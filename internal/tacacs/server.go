// Package tacacs is a minimal TACACS+ server for the layer 4 tacacs
// scenario. Debian 13 packages no TACACS+ server; this one knows a fixed
// table of test users and grants each its privilege level.
package tacacs

import (
	"context"
	"fmt"
	"net"

	"github.com/nwaples/tacplus"
)

// User is a test account (not a secret: scenario files commit them). Level
// is what DANOS's SSSD TACACS+ provider reads: operator, admin or superuser.
type User struct {
	Name, Password string
	Priv           int
	Level          string
}

type handler struct {
	users map[string]User
	logf  func(format string, a ...any)
}

// Serve answers TACACS+ on l until l is closed. Authentication accepts PAP
// and interactive ASCII login; authorization of a known user passes with
// priv-lvl set; accounting always succeeds.
func Serve(l net.Listener, secret string, users []User) error {
	return ServeLog(l, secret, users, nil)
}

// ServeLog is Serve, reporting every request and its outcome to logf (when
// set), so a scenario run shows what a router asked.
func ServeLog(l net.Listener, secret string, users []User, logf func(format string, a ...any)) error {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	h := handler{users: map[string]User{}, logf: logf}
	for _, u := range users {
		h.users[u.Name] = u
	}
	cfg := tacplus.ConnConfig{Secret: []byte(secret), Mux: true, Log: func(v ...any) { logf("tacacs: %s", fmt.Sprint(v...)) }}
	conn := &tacplus.ServerConnHandler{Handler: h, ConnConfig: cfg}
	srv := &tacplus.Server{ServeConn: conn.Serve, Log: cfg.Log}
	return srv.Serve(l)
}

func (h handler) check(user, password string) *tacplus.AuthenReply {
	if u, ok := h.users[user]; ok && u.Password == password {
		h.logf("tacacs: authen %s: pass", user)
		return &tacplus.AuthenReply{Status: tacplus.AuthenStatusPass}
	}
	h.logf("tacacs: authen %s: fail", user)
	return &tacplus.AuthenReply{Status: tacplus.AuthenStatusFail, ServerMsg: "authentication failed"}
}

func (h handler) HandleAuthenStart(ctx context.Context, a *tacplus.AuthenStart, s *tacplus.ServerSession) *tacplus.AuthenReply {
	h.logf("tacacs: authen start from %s: user %q type %d service %d", s.RemoteAddr(), a.User, a.AuthenType, a.AuthenService)
	if a.AuthenType == tacplus.AuthenTypePAP {
		return h.check(a.User, string(a.Data))
	}
	user := a.User
	if user == "" {
		c, err := s.GetUser(ctx, "Username: ")
		if err != nil || c == nil {
			return nil
		}
		user = c.Message
	}
	c, err := s.GetPass(ctx, "Password: ")
	if err != nil || c == nil {
		return nil
	}
	return h.check(user, c.Message)
}

func (h handler) HandleAuthorRequest(ctx context.Context, a *tacplus.AuthorRequest, s *tacplus.ServerSession) *tacplus.AuthorResponse {
	u, ok := h.users[a.User]
	h.logf("tacacs: author %q args %v: known=%v", a.User, a.Arg, ok)
	if !ok {
		return &tacplus.AuthorResponse{Status: tacplus.AuthorStatusFail}
	}
	args := []string{fmt.Sprintf("priv-lvl=%d", u.Priv)}
	if u.Level != "" {
		args = append(args, "level="+u.Level)
	}
	return &tacplus.AuthorResponse{Status: tacplus.AuthorStatusPassAdd, Arg: args}
}

func (h handler) HandleAcctRequest(ctx context.Context, a *tacplus.AcctRequest, s *tacplus.ServerSession) *tacplus.AcctReply {
	return &tacplus.AcctReply{Status: tacplus.AcctStatusSuccess}
}
