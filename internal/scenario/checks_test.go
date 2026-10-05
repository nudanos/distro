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
