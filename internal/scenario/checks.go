package scenario

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/nudanos/distro/internal/boottest"
	"github.com/nudanos/distro/internal/topology"
)

// Routers is what a check can reach: each router's console and forwarded
// management ports, and the administrator the HTTP checks authenticate as.
type Routers struct {
	VMs             map[string]*topology.VM
	Ports           map[string]topology.Ports
	Admin, Password string
	T               func(time.Duration) time.Duration
	Log             io.Writer // failed attempts of retried checks (nil: none)
}

var noSession = regexp.MustCompile(`Failed to set up config session`)

// opInterval is how long a retrying check waits between attempts.
var opInterval = 10 * time.Second

// pollInterval is how often an HTTP step re-asks while the router answers
// 202 Accepted (an operational command still running).
var pollInterval = 2 * time.Second

var (
	goMu     sync.Mutex
	goChecks = map[string]func(ctx context.Context, r *Routers) error{}
)

// RegisterGo makes a named Go check available to scenario files ("go: name").
func RegisterGo(name string, f func(ctx context.Context, r *Routers) error) {
	goMu.Lock()
	defer goMu.Unlock()
	goChecks[name] = f
}

// RunCheck runs one check. Checks that observe the network retry until they
// pass or their (scaled) timeout passes; the error describes the last try.
func RunCheck(ctx context.Context, c Check, r *Routers) error {
	timeout := r.T(c.Timeout)
	// a retry returns only its last error; earlier ones go to the run log
	logged := func(try func() error) func() error {
		return func() error {
			err := try()
			if err != nil && r.Log != nil {
				fmt.Fprintf(r.Log, "  attempt failed: %v\n", err)
			}
			return err
		}
	}
	switch {
	case c.Op != nil:
		return retry(ctx, timeout, logged(func() error { return opOnce(r, c) }))
	case c.Action != nil:
		vm, err := r.vm(c.Router)
		if err != nil {
			return err
		}
		return ConfigureSession(vm.Console, c.Action.Configure, c.Action.Commit, r.T)
	case c.HTTP != nil:
		return retry(ctx, timeout, logged(func() error { return httpOnce(r, c) }))
	case c.SNMP != nil:
		return retry(ctx, timeout, logged(func() error { return snmpOnce(r, c) }))
	case c.Login != nil:
		return retry(ctx, timeout, logged(func() error { return loginOnce(r, c) }))
	case c.Go != "":
		goMu.Lock()
		f := goChecks[c.Go]
		goMu.Unlock()
		if f == nil {
			return fmt.Errorf("no Go check named %q", c.Go)
		}
		return f(ctx, r)
	}
	return fmt.Errorf("%s: no check kind", c.Name)
}

func (r *Routers) vm(name string) (*topology.VM, error) {
	vm := r.VMs[name]
	if vm == nil {
		return nil, fmt.Errorf("no running router %s", name)
	}
	return vm, nil
}

func retry(ctx context.Context, timeout time.Duration, try func() error) error {
	deadline := time.Now().Add(timeout)
	for {
		err := try()
		if err == nil {
			return nil
		}
		if time.Now().Add(opInterval).After(deadline) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(opInterval):
		}
	}
}

func opOnce(r *Routers, c Check) error {
	vm, err := r.vm(c.Router)
	if err != nil {
		return err
	}
	out, err := boottest.OpOutput(vm.Console, c.Op.Command, r.T(2*time.Minute))
	if err != nil {
		return err
	}
	re, err := regexp.Compile(c.Op.Want)
	if err != nil {
		return err
	}
	if re.MatchString(out) == c.Op.Absent {
		verb := "lacks"
		if c.Op.Absent {
			verb = "still contains"
		}
		return fmt.Errorf("%s: %q %s %q:\n%s", c.Router, c.Op.Command, verb, c.Op.Want, out)
	}
	return nil
}

// ConfigureSession enters configuration mode, runs lines, commits when asked
// and returns to operational mode.
func ConfigureSession(con *boottest.Console, lines []string, commit bool, t func(time.Duration) time.Duration) error {
	// Until configd is ready after boot, "configure" answers "Failed to set
	// up config session" and returns to the shell: ask again.
	err := retry(context.Background(), t(5*time.Minute), func() error {
		con.Discard()
		if err := con.Send("configure"); err != nil {
			return err
		}
		i, _, err := con.First([]*regexp.Regexp{boottest.CfgPrompt, noSession}, t(time.Minute))
		if err != nil {
			return err
		}
		if i == 1 {
			con.Expect(boottest.OpPrompt, t(time.Minute))
			return fmt.Errorf("configure: failed to set up config session")
		}
		return nil
	})
	if err != nil {
		return err
	}
	for _, l := range lines {
		if err := boottest.Configure(con, l, t(time.Minute)); err != nil {
			return fmt.Errorf("%q: %w", l, err)
		}
	}
	exit := "exit discard"
	if commit {
		if err := boottest.Commit(con, nil, 30*time.Second, t(20*time.Minute)); err != nil {
			return err
		}
		exit = "exit"
	}
	con.Discard()
	if err := con.Send(exit); err != nil {
		return err
	}
	_, err = con.Expect(boottest.OpPrompt, t(time.Minute))
	return err
}

// httpTimeout bounds one HTTP request before scaling.
var httpTimeout = 60 * time.Second

var insecureTransport = &http.Transport{
	TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // routers serve self-signed certificates
}

func httpOnce(r *Routers, c Check) error {
	steps := c.HTTP.Steps
	if len(steps) == 0 {
		steps = []HTTPCheck{*c.HTTP}
	}
	// scaled: under emulation a commit can take minutes, and a request
	// abandoned early may still succeed on the router
	insecure := &http.Client{Timeout: r.T(httpTimeout), Transport: insecureTransport}
	location := ""
	for i, s := range steps {
		path := strings.ReplaceAll(s.Path, "{location}", location)
		req, err := http.NewRequest(s.Method, fmt.Sprintf("https://127.0.0.1:%d%s", r.Ports[c.Router].HTTPS, path), strings.NewReader(s.Body))
		if err != nil {
			return err
		}
		if s.Auth {
			req.SetBasicAuth(r.Admin, r.Password)
		}
		var resp *http.Response
		var body []byte
		for polls := 0; ; polls++ {
			if resp, err = insecure.Do(req); err != nil {
				return fmt.Errorf("step %d %s %s: %w", i+1, s.Method, path, err)
			}
			body, _ = io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode != http.StatusAccepted || s.Status == http.StatusAccepted || polls >= 60 {
				break
			}
			time.Sleep(pollInterval)
			if req, err = http.NewRequest(s.Method, req.URL.String(), strings.NewReader(s.Body)); err != nil {
				return err
			}
			if s.Auth {
				req.SetBasicAuth(r.Admin, r.Password)
			}
		}
		if s.Status != 0 && resp.StatusCode != s.Status {
			return fmt.Errorf("step %d %s %s: status %d, want %d: %s", i+1, s.Method, path, resp.StatusCode, s.Status, body)
		}
		if s.Want != "" && !regexp.MustCompile(s.Want).Match(body) {
			return fmt.Errorf("step %d %s %s: body lacks %q: %s", i+1, s.Method, path, s.Want, body)
		}
		if loc := resp.Header.Get("Location"); loc != "" {
			if u, err := url.Parse(loc); err == nil {
				location = "/" + strings.TrimPrefix(u.Path, "/") // 2105 sends "rest/conf/ID"
			}
		}
	}
	return nil
}

func snmpOnce(r *Routers, c Check) error {
	s := c.SNMP
	args := []string{"-v2c", "-c", s.Community}
	if s.Version == "3" {
		args = []string{"-v3", "-l", "authPriv", "-u", s.User, "-a", "SHA", "-A", s.AuthKey, "-x", "AES", "-X", s.PrivKey}
	}
	args = append(args, fmt.Sprintf("udp:127.0.0.1:%d", r.Ports[c.Router].SNMP), s.OID)
	out, err := exec.Command("snmpwalk", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("snmpwalk %s: %v: %s", strings.Join(args, " "), err, out)
	}
	if !regexp.MustCompile(s.Want).Match(out) {
		return fmt.Errorf("snmpwalk %s lacks %q:\n%s", s.OID, s.Want, out)
	}
	return nil
}

func loginOnce(r *Routers, c Check) error {
	l := *c.Login
	for _, f := range []*string{&l.User, &l.Password} {
		*f = strings.NewReplacer("${admin}", r.Admin, "${password}", r.Password).Replace(*f)
	}
	cfg := &ssh.ClientConfig{User: l.User, Auth: []ssh.AuthMethod{ssh.Password(l.Password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), Timeout: 30 * time.Second} // test routers' host keys are new every run
	client, err := ssh.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", r.Ports[c.Router].SSH), cfg)
	if l.Expect == "refused" {
		if err == nil {
			client.Close()
			return fmt.Errorf("%s logged in; want refused", l.User)
		}
		if strings.Contains(err.Error(), "unable to authenticate") {
			return nil
		}
		return fmt.Errorf("login as %s: %w (want an authentication failure)", l.User, err)
	}
	if err != nil {
		return fmt.Errorf("login as %s: %w", l.User, err)
	}
	defer client.Close()
	sess, err := client.NewSession()
	if err != nil {
		return err
	}
	defer sess.Close()
	out, err := sess.CombinedOutput("id")
	if err != nil {
		return fmt.Errorf("id as %s: %w: %s", l.User, err, out)
	}
	if l.Want != "" && !regexp.MustCompile(l.Want).Match(out) {
		return fmt.Errorf("id as %s lacks %q: %s", l.User, l.Want, out)
	}
	if l.NotWant != "" && regexp.MustCompile(l.NotWant).Match(out) {
		return fmt.Errorf("id as %s contains %q: %s", l.User, l.NotWant, out)
	}
	return nil
}
