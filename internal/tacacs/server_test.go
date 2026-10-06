package tacacs

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nwaples/tacplus"
)

const secret = "nudanos-tac-1"

func serve(t *testing.T) *tacplus.Client {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	go Serve(l, secret, []User{{"tacadmin", "tac-admin-1", 15}, {"tacop", "tac-op-1", 1}})
	return &tacplus.Client{Addr: l.Addr().String(), ConnConfig: tacplus.ConnConfig{Secret: []byte(secret)}}
}

func pap(t *testing.T, c *tacplus.Client, user, pass string) uint8 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	r, _, err := c.SendAuthenStart(ctx, &tacplus.AuthenStart{Action: tacplus.AuthenActionLogin, AuthenType: tacplus.AuthenTypePAP,
		AuthenService: tacplus.AuthenServiceLogin, User: user, Data: []byte(pass)})
	if err != nil {
		t.Fatal(err)
	}
	return r.Status
}

func TestServeAuthenticatesKnownUser(t *testing.T) {
	c := serve(t)
	if s := pap(t, c, "tacadmin", "tac-admin-1"); s != tacplus.AuthenStatusPass {
		t.Fatalf("authen status = %d, want pass", s)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	r, err := c.SendAuthorRequest(ctx, &tacplus.AuthorRequest{AuthenMethod: tacplus.AuthenMethodTACACSPlus,
		AuthenType: tacplus.AuthenTypePAP, AuthenService: tacplus.AuthenServiceLogin, User: "tacadmin",
		Arg: []string{"service=shell", "cmd="}})
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != tacplus.AuthorStatusPassAdd || !strings.Contains(strings.Join(r.Arg, " "), "priv-lvl=15") {
		t.Errorf("author = %d %v, want pass-add with priv-lvl=15", r.Status, r.Arg)
	}
}

func TestServeRefusesWrongPassword(t *testing.T) {
	if s := pap(t, serve(t), "tacadmin", "wrong"); s != tacplus.AuthenStatusFail {
		t.Errorf("authen status = %d, want fail", s)
	}
}

func TestServeRefusesUnknownUser(t *testing.T) {
	if s := pap(t, serve(t), "nobody", "x"); s != tacplus.AuthenStatusFail {
		t.Errorf("authen status = %d, want fail", s)
	}
}

// pam_tacplus logs in with interactive ASCII authentication by default.
func TestServeASCIILogin(t *testing.T) {
	c := serve(t)
	for _, tc := range []struct {
		pass string
		want uint8
	}{{"tac-op-1", tacplus.AuthenStatusPass}, {"wrong", tacplus.AuthenStatusFail}} {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		r, s, err := c.SendAuthenStart(ctx, &tacplus.AuthenStart{Action: tacplus.AuthenActionLogin, AuthenType: tacplus.AuthenTypeASCII,
			AuthenService: tacplus.AuthenServiceLogin, User: "tacop"})
		if err != nil {
			t.Fatal(err)
		}
		if r.Status != tacplus.AuthenStatusGetPass {
			t.Fatalf("first reply = %d, want a password prompt", r.Status)
		}
		r, err = s.Continue(ctx, tc.pass)
		if err != nil {
			t.Fatal(err)
		}
		if r.Status != tc.want {
			t.Errorf("password %q: status = %d, want %d", tc.pass, r.Status, tc.want)
		}
		s.Close()
		cancel()
	}
}

func TestServeLogLogsEachRequest(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	var mu sync.Mutex
	var lines []string
	go ServeLog(l, secret, []User{{"tacadmin", "tac-admin-1", 15}}, func(format string, a ...any) {
		mu.Lock()
		lines = append(lines, fmt.Sprintf(format, a...))
		mu.Unlock()
	})
	c := &tacplus.Client{Addr: l.Addr().String(), ConnConfig: tacplus.ConnConfig{Secret: []byte(secret)}}
	pap(t, c, "tacadmin", "wrong")
	mu.Lock()
	defer mu.Unlock()
	got := strings.Join(lines, "\n")
	if !strings.Contains(got, "tacadmin") || !strings.Contains(got, "fail") {
		t.Errorf("log = %q, want the user and the outcome", got)
	}
}
