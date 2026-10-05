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

// User is a test account (not a secret: scenario files commit them).
type User struct {
	Name, Password string
	Priv           int
}

type handler struct{ users map[string]User }

// Serve answers TACACS+ on l until l is closed. Authentication accepts PAP
// and interactive ASCII login; authorization of a known user passes with
// priv-lvl set; accounting always succeeds.
func Serve(l net.Listener, secret string, users []User) error {
	h := handler{users: map[string]User{}}
	for _, u := range users {
		h.users[u.Name] = u
	}
	conn := &tacplus.ServerConnHandler{Handler: h, ConnConfig: tacplus.ConnConfig{Secret: []byte(secret), Mux: true}}
	srv := &tacplus.Server{ServeConn: conn.Serve}
	return srv.Serve(l)
}

func (h handler) check(user, password string) *tacplus.AuthenReply {
	if u, ok := h.users[user]; ok && u.Password == password {
		return &tacplus.AuthenReply{Status: tacplus.AuthenStatusPass}
	}
	return &tacplus.AuthenReply{Status: tacplus.AuthenStatusFail, ServerMsg: "authentication failed"}
}

func (h handler) HandleAuthenStart(ctx context.Context, a *tacplus.AuthenStart, s *tacplus.ServerSession) *tacplus.AuthenReply {
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
	if !ok {
		return &tacplus.AuthorResponse{Status: tacplus.AuthorStatusFail}
	}
	return &tacplus.AuthorResponse{Status: tacplus.AuthorStatusPassAdd, Arg: []string{fmt.Sprintf("priv-lvl=%d", u.Priv)}}
}

func (h handler) HandleAcctRequest(ctx context.Context, a *tacplus.AcctRequest, s *tacplus.ServerSession) *tacplus.AcctReply {
	return &tacplus.AcctReply{Status: tacplus.AcctStatusSuccess}
}
