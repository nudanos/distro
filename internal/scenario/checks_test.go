package scenario

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/nudanos/distro/internal/boottest"
	"github.com/nudanos/distro/internal/topology"
)

func identity(d time.Duration) time.Duration { return d }

func port(t *testing.T, rawURL string) int {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := strconv.Atoi(u.Port())
	return p
}

func routers(ports topology.Ports) *Routers {
	return &Routers{Ports: map[string]topology.Ports{"R1": ports}, Admin: "nudanos", Password: "NuDanOS-test-1", T: identity}
}

// The REST API hands out a configuration session in a Location header; the
// next request of the flow goes to that session.
func TestHTTPCheckFollowsLocation(t *testing.T) {
	var seen []string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.Path)
		if r.URL.Path == "/rest/conf" {
			w.Header().Set("Location", "/rest/conf/ABC")
			w.WriteHeader(201)
			return
		}
		fmt.Fprint(w, "ok")
	}))
	defer srv.Close()
	c := Check{Name: "flow", Router: "R1", Timeout: 5 * time.Second, HTTP: &HTTPCheck{Steps: []HTTPCheck{
		{Method: "POST", Path: "/rest/conf", Status: 201, Auth: true},
		{Method: "PUT", Path: "{location}/set/system/host-name/r1", Status: 200, Want: "ok", Auth: true},
	}}}
	if err := RunCheck(context.Background(), c, routers(topology.Ports{HTTPS: port(t, srv.URL)})); err != nil {
		t.Fatal(err)
	}
	if strings.Join(seen, ",") != "POST /rest/conf,PUT /rest/conf/ABC/set/system/host-name/r1" {
		t.Errorf("requests = %v", seen)
	}
}

func TestHTTPCheckRequiresAuthWhenAsked(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if u, p, ok := r.BasicAuth(); !ok || u != "nudanos" || p != "NuDanOS-test-1" {
			w.WriteHeader(401)
			return
		}
		fmt.Fprint(w, "Version: 1.0")
	}))
	defer srv.Close()
	rs := routers(topology.Ports{HTTPS: port(t, srv.URL)})
	authed := Check{Name: "a", Router: "R1", Timeout: 5 * time.Second, HTTP: &HTTPCheck{Method: "GET", Path: "/rest/op", Status: 200, Want: "Version", Auth: true}}
	anon := Check{Name: "b", Router: "R1", Timeout: 5 * time.Second, HTTP: &HTTPCheck{Method: "GET", Path: "/rest/op", Status: 401}}
	for _, c := range []Check{authed, anon} {
		if err := RunCheck(context.Background(), c, rs); err != nil {
			t.Errorf("%s: %v", c.Name, err)
		}
	}
	wrong := Check{Name: "c", Router: "R1", Timeout: time.Second, HTTP: &HTTPCheck{Method: "GET", Path: "/rest/op", Status: 200}}
	if err := RunCheck(context.Background(), wrong, rs); err == nil {
		t.Error("an unauthenticated request expecting 200 passed")
	}
}

func TestSNMPCheckRunsSnmpwalk(t *testing.T) {
	bin := t.TempDir()
	args := filepath.Join(bin, "args")
	os.WriteFile(filepath.Join(bin, "snmpwalk"), []byte("#!/bin/sh\necho \"$@\" > "+args+"\necho 'IF-MIB::ifDescr.3 = STRING: dp0s3'\n"), 0o755)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	c := Check{Name: "v3", Router: "R1", Timeout: 5 * time.Second, SNMP: &SNMPCheck{Version: "3", User: "snmpv3",
		AuthKey: "snmpv3-auth-1", PrivKey: "snmpv3-priv-1", OID: "IF-MIB::ifDescr", Want: "dp0s3"}}
	if err := RunCheck(context.Background(), c, routers(topology.Ports{SNMP: 16161})); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(args)
	for _, want := range []string{"-v3", "-l authPriv", "-u snmpv3", "-a SHA", "-A snmpv3-auth-1", "-x AES", "-X snmpv3-priv-1", "udp:127.0.0.1:16161", "IF-MIB::ifDescr"} {
		if !strings.Contains(string(got), want) {
			t.Errorf("snmpwalk args %q lack %q", got, want)
		}
	}
}

// sshServer accepts nudanos/NuDanOS-test-1 and answers the command "id".
func sshServer(t *testing.T) int {
	t.Helper()
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &ssh.ServerConfig{PasswordCallback: func(c ssh.ConnMetadata, p []byte) (*ssh.Permissions, error) {
		if c.User() == "nudanos" && string(p) == "NuDanOS-test-1" {
			return nil, nil
		}
		return nil, fmt.Errorf("denied")
	}}
	cfg.AddHostKey(signer)
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	go func() {
		for {
			nc, err := l.Accept()
			if err != nil {
				return
			}
			go func() {
				_, chans, reqs, err := ssh.NewServerConn(nc, cfg)
				if err != nil {
					return
				}
				go ssh.DiscardRequests(reqs)
				for ch := range chans {
					c, creqs, _ := ch.Accept()
					go func() {
						for r := range creqs {
							r.Reply(r.Type == "exec", nil)
							if r.Type == "exec" {
								io.WriteString(c, "uid=1000(nudanos) gid=100(users) groups=100(users),27(sudo),112(vyattaadm)\n")
								c.SendRequest("exit-status", false, []byte{0, 0, 0, 0})
								c.Close()
							}
						}
					}()
				}
			}()
		}
	}()
	return l.Addr().(*net.TCPAddr).Port
}

func TestLoginCheckOKAndRefused(t *testing.T) {
	rs := routers(topology.Ports{SSH: sshServer(t)})
	ok := Check{Name: "ok", Router: "R1", Timeout: 5 * time.Second, Login: &LoginCheck{User: "nudanos", Password: "NuDanOS-test-1", Expect: "ok", Want: "vyattaadm", NotWant: "root"}}
	refused := Check{Name: "refused", Router: "R1", Timeout: 5 * time.Second, Login: &LoginCheck{User: "nudanos", Password: "wrong", Expect: "refused"}}
	notAdmin := Check{Name: "notwant", Router: "R1", Timeout: 5 * time.Second, Login: &LoginCheck{User: "nudanos", Password: "NuDanOS-test-1", Expect: "ok", NotWant: "vyattaadm"}}
	for _, c := range []Check{ok, refused} {
		if err := RunCheck(context.Background(), c, rs); err != nil {
			t.Errorf("%s: %v", c.Name, err)
		}
	}
	if err := RunCheck(context.Background(), notAdmin, rs); err == nil {
		t.Error("NotWant matched but the check passed")
	}
}

// The first answer lacks the wanted text; the check asks again.
func TestOpCheckRetriesUntilMatch(t *testing.T) {
	opInterval = 10 * time.Millisecond
	a, vm := net.Pipe()
	defer a.Close()
	defer vm.Close()
	go func() {
		b := make([]byte, 128)
		for _, answer := range []string{"Neighbor 10.0.12.2 Idle", "Neighbor 10.0.12.2 Established"} {
			n, err := vm.Read(b)
			if err != nil {
				return
			}
			io.WriteString(vm, string(b[:n])+"\n"+answer+"\r\nvyatta@r1:~$ ")
		}
	}()
	rs := routers(topology.Ports{})
	rs.VMs = map[string]*topology.VM{"R1": topology.NewVM(topology.VMSpec{Name: "R1"}, boottest.NewConsole(a, &bytes.Buffer{}), func(time.Duration) {})}
	c := Check{Name: "bgp up", Router: "R1", Timeout: 5 * time.Second, Op: &OpCheck{Command: "show ip bgp summary", Want: "Established"}}
	if err := RunCheck(context.Background(), c, rs); err != nil {
		t.Fatal(err)
	}
}

// The local administrator differs per image (tmpuser on 2105, nudanos on
// NuDanOS); scenario files name it as ${admin} / ${password}.
func TestLoginCheckUsesAdminPlaceholders(t *testing.T) {
	rs := routers(topology.Ports{SSH: sshServer(t)})
	c := Check{Name: "local", Router: "R1", Timeout: 5 * time.Second, Login: &LoginCheck{User: "${admin}", Password: "${password}", Expect: "ok"}}
	if err := RunCheck(context.Background(), c, rs); err != nil {
		t.Fatal(err)
	}
}

// 2105 answers with a Location that has no leading slash ("rest/conf/ID").
func TestHTTPCheckRelativeLocation(t *testing.T) {
	var seen []string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.URL.Path)
		if r.URL.Path == "/rest/conf" {
			w.Header().Set("Location", "rest/conf/C605455D3A48BC3A")
			w.WriteHeader(201)
		}
	}))
	defer srv.Close()
	c := Check{Name: "relative", Router: "R1", Timeout: 5 * time.Second, HTTP: &HTTPCheck{Steps: []HTTPCheck{
		{Method: "POST", Path: "/rest/conf", Status: 201},
		{Method: "POST", Path: "{location}/commit", Status: 200},
	}}}
	if err := RunCheck(context.Background(), c, routers(topology.Ports{HTTPS: port(t, srv.URL)})); err != nil {
		t.Fatal(err)
	}
	if strings.Join(seen, ",") != "/rest/conf,/rest/conf/C605455D3A48BC3A/commit" {
		t.Errorf("requests = %v", seen)
	}
}

// An operational command's result answers 202 while it still runs (2105).
func TestHTTPCheckPollsWhileAccepted(t *testing.T) {
	gets := 0
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			w.Header().Set("Location", "rest/op/CA88")
			w.WriteHeader(201)
			return
		}
		gets++
		if gets < 3 {
			w.WriteHeader(202)
			return
		}
		fmt.Fprint(w, "Version: 2105")
	}))
	defer srv.Close()
	old := pollInterval
	pollInterval = 10 * time.Millisecond
	defer func() { pollInterval = old }()
	c := Check{Name: "op", Router: "R1", Timeout: 5 * time.Second, HTTP: &HTTPCheck{Steps: []HTTPCheck{
		{Method: "POST", Path: "/rest/op/show/version", Status: 201},
		{Method: "GET", Path: "{location}", Status: 200, Want: "Version"},
	}}}
	if err := RunCheck(context.Background(), c, routers(topology.Ports{HTTPS: port(t, srv.URL)})); err != nil {
		t.Fatal(err)
	}
	if gets != 3 {
		t.Errorf("GET ran %d times, want 3 (two 202s, then 200)", gets)
	}
}

// Right after boot NuDanOS answers "configure" with "Failed to set up config
// session" until configd is ready; the session is retried.
func TestConfigureSessionRetriesUntilConfigdIsReady(t *testing.T) {
	opInterval = 10 * time.Millisecond
	a, vm := net.Pipe()
	defer a.Close()
	defer vm.Close()
	go func() {
		b := make([]byte, 128)
		answers := []string{
			"configure\r\nFailed to set up config session\r\nnudanos@r1:~$ ",
			"configure\r\n[edit]\r\nnudanos@r1# ",
			"set system host-name r1\r\n[edit]\r\nnudanos@r1# ",
			"exit discard\r\nnudanos@r1:~$ ",
		}
		for _, answer := range answers {
			if _, err := vm.Read(b); err != nil {
				return
			}
			io.WriteString(vm, answer)
		}
	}()
	c := boottest.NewConsole(a, &bytes.Buffer{})
	if err := ConfigureSession(c, []string{"set system host-name r1"}, false, func(d time.Duration) time.Duration { return d / 60 }); err != nil {
		t.Fatal(err)
	}
}

// A retried check reports only its last error; the first failure (often
// the informative one, before a non-idempotent step makes every retry
// fail differently) goes to the run log.
func TestRetriedCheckLogsEveryFailedAttempt(t *testing.T) {
	opInterval = 10 * time.Millisecond
	defer func() { opInterval = 10 * time.Second }()
	tries := 0
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tries++
		if tries == 1 {
			w.WriteHeader(500)
			fmt.Fprint(w, "first failure")
			return
		}
		fmt.Fprint(w, "ok")
	}))
	defer srv.Close()
	var log bytes.Buffer
	r := routers(topology.Ports{HTTPS: port(t, srv.URL)})
	r.Log = &log
	c := Check{Name: "flaky", Router: "R1", Timeout: 5 * time.Second, HTTP: &HTTPCheck{Method: "GET", Path: "/rest/op", Status: 200}}
	if err := RunCheck(context.Background(), c, r); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(log.String(), "first failure") {
		t.Errorf("run log lacks the failed attempt:\n%s", log.String())
	}
}

// Under emulation a REST commit can outlast an unscaled client timeout,
// succeed on the router anyway, and leave every retry failing ("Node
// exists"). The HTTP client's timeout scales like every other timeout.
func TestHTTPTimeoutScales(t *testing.T) {
	old := httpTimeout
	httpTimeout = 100 * time.Millisecond
	defer func() { httpTimeout = old }()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(250 * time.Millisecond)
		fmt.Fprint(w, "ok")
	}))
	defer srv.Close()
	r := routers(topology.Ports{HTTPS: port(t, srv.URL)})
	c := Check{Name: "slow commit", Router: "R1", Timeout: time.Millisecond, HTTP: &HTTPCheck{Method: "POST", Path: "/rest/conf/X/commit", Status: 200}}
	if err := RunCheck(context.Background(), c, r); err == nil {
		t.Fatal("unscaled: a request slower than httpTimeout succeeded")
	}
	r.T = func(d time.Duration) time.Duration { return 6 * d }
	if err := RunCheck(context.Background(), c, r); err != nil {
		t.Fatalf("scaled x6: %v", err)
	}
}

// A failed commit leaves the router in configuration mode with the changes
// pending; ConfigureSession discards them and returns to operational mode,
// or every later step fails ("Cannot load: configuration modified").
func TestConfigureSessionDiscardsAfterFailedCommit(t *testing.T) {
	a, vm := net.Pipe()
	defer a.Close()
	defer vm.Close()
	var sent []string
	go func() {
		b := make([]byte, 256)
		answers := []string{
			"configure\r\n[edit]\r\nnudanos@r1# ",
			"set protocols ospf passive-interface dp0s4\r\n[edit]\r\nnudanos@r1# ",
			"commit\r\nInvalid interface name\r\nCommit failed!\r\n[edit]\r\nnudanos@r1# ",
			"exit discard\r\nnudanos@r1:~$ ",
		}
		for _, answer := range answers {
			n, err := vm.Read(b)
			if err != nil {
				return
			}
			sent = append(sent, strings.TrimSpace(string(b[:n])))
			io.WriteString(vm, answer)
		}
	}()
	c := boottest.NewConsole(a, &bytes.Buffer{})
	err := ConfigureSession(c, []string{"set protocols ospf passive-interface dp0s4"}, true, func(d time.Duration) time.Duration { return d / 60 })
	if err == nil {
		t.Fatal("a failed commit returned no error")
	}
	if len(sent) != 4 || sent[3] != "exit discard" {
		t.Errorf("sent %q; want the changes discarded with exit discard", sent)
	}
}
