// Package scenario runs layer 4 scenarios (plan 4): a topology of routers,
// their configuration, and checks, judged on NuDanOS or on DANOS 2105.
package scenario

import (
	"fmt"
	"os"
	"regexp"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/nudanos/distro/internal/tacacs"
	"github.com/nudanos/distro/internal/topology"
)

// File is one tests/scenarios/<name>/scenario.yaml.
type File struct {
	Name        string
	Gating      bool
	Routers     map[string]Router
	RouterOrder []string // file order; routers start in this order
	Links       []topology.Link
	Show        []string
	Checks      []Check
	TACACS      *TACACSServer
}

// Router is one router's scenario configuration, as `set` lines.
type Router struct {
	Config []string `yaml:"config"`
}

// Check is one judgement; exactly one kind is set.
type Check struct {
	Name    string
	Router  string
	Timeout time.Duration
	Op      *OpCheck
	Action  *Action
	HTTP    *HTTPCheck
	SNMP    *SNMPCheck
	Login   *LoginCheck
	Go      string
}

// OpCheck: the command's output must match Want (or, with Absent, stop matching).
type OpCheck struct {
	Command string `yaml:"command"`
	Want    string `yaml:"want"`
	Absent  bool   `yaml:"absent"`
}

// Action changes the topology before later checks.
type Action struct {
	Configure []string `yaml:"configure"`
	Commit    bool     `yaml:"commit"`
}

// HTTPCheck is a request to the router's HTTPS service (REST API).
type HTTPCheck struct {
	Method string      `yaml:"method"`
	Path   string      `yaml:"path"`
	Body   string      `yaml:"body"`
	Status int         `yaml:"status"`
	Want   string      `yaml:"want"`
	Auth   bool        `yaml:"auth"`
	Steps  []HTTPCheck `yaml:"steps"`
}

// SNMPCheck is an snmpwalk from the runner.
type SNMPCheck struct {
	Version   string `yaml:"version"`
	Community string `yaml:"community"`
	User      string `yaml:"user"`
	AuthKey   string `yaml:"auth_key"`
	PrivKey   string `yaml:"priv_key"`
	OID       string `yaml:"oid"`
	Want      string `yaml:"want"`
}

// LoginCheck is an SSH login; Expect is "ok" or "refused".
type LoginCheck struct {
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Expect   string `yaml:"expect"`
	Want     string `yaml:"want"`
	NotWant  string `yaml:"not_want"`
}

// TACACSServer is the test server the runner starts on 127.0.0.1:49.
type TACACSServer struct {
	Secret string        `yaml:"secret"`
	Users  []tacacs.User `yaml:"users"`
}

type rawCheck struct {
	Name    string      `yaml:"name"`
	Router  string      `yaml:"router"`
	Timeout string      `yaml:"timeout"`
	Op      *OpCheck    `yaml:"op"`
	Action  *Action     `yaml:"action"`
	HTTP    *HTTPCheck  `yaml:"http"`
	SNMP    *SNMPCheck  `yaml:"snmp"`
	Login   *LoginCheck `yaml:"login"`
	Go      string      `yaml:"go"`
}

type rawFile struct {
	Name    string        `yaml:"name"`
	Gating  bool          `yaml:"gating"`
	Routers yaml.Node     `yaml:"routers"`
	Links   [][]string    `yaml:"links"`
	Show    []string      `yaml:"show"`
	Checks  []rawCheck    `yaml:"checks"`
	TACACS  *TACACSServer `yaml:"tacacs"`
}

const defaultTimeout = 2 * time.Minute

// Load reads and validates a scenario file.
func Load(path string) (*File, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var raw rawFile
	if err := yaml.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	f := &File{Name: raw.Name, Gating: raw.Gating, Routers: map[string]Router{}, Show: raw.Show, TACACS: raw.TACACS}
	if raw.Routers.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%s: routers must be a mapping", path)
	}
	for i := 0; i+1 < len(raw.Routers.Content); i += 2 {
		name := raw.Routers.Content[i].Value
		var r Router
		if err := raw.Routers.Content[i+1].Decode(&r); err != nil {
			return nil, fmt.Errorf("%s: router %s: %w", path, name, err)
		}
		f.Routers[name] = r
		f.RouterOrder = append(f.RouterOrder, name)
	}
	for _, l := range raw.Links {
		if len(l) != 2 {
			return nil, fmt.Errorf("%s: a link joins exactly two routers: %v", path, l)
		}
		f.Links = append(f.Links, topology.Link{A: l[0], B: l[1]})
	}
	for i, rc := range raw.Checks {
		c := Check{Name: rc.Name, Router: rc.Router, Timeout: defaultTimeout,
			Op: rc.Op, Action: rc.Action, HTTP: rc.HTTP, SNMP: rc.SNMP, Login: rc.Login, Go: rc.Go}
		if c.Name == "" {
			c.Name = fmt.Sprintf("check %d", i+1)
		}
		if rc.Timeout != "" {
			if c.Timeout, err = time.ParseDuration(rc.Timeout); err != nil {
				return nil, fmt.Errorf("%s: %s: timeout: %w", path, c.Name, err)
			}
		}
		kinds := 0
		for _, set := range []bool{c.Op != nil, c.Action != nil, c.HTTP != nil, c.SNMP != nil, c.Login != nil, c.Go != ""} {
			if set {
				kinds++
			}
		}
		if kinds != 1 {
			return nil, fmt.Errorf("%s: %s: a check has exactly one of op, action, http, snmp, login, go (found %d)", path, c.Name, kinds)
		}
		if c.Router == "" && c.Go == "" {
			return nil, fmt.Errorf("%s: %s: no router", path, c.Name)
		}
		if _, ok := f.Routers[c.Router]; c.Router != "" && !ok {
			return nil, fmt.Errorf("%s: %s: unknown router %s", path, c.Name, c.Router)
		}
		f.Checks = append(f.Checks, c)
	}
	return f, nil
}

var placeholder = regexp.MustCompile(`\$\{(\w+):(\w+)\}`)

// Expand replaces ${R1:R2} with R1's interface toward R2 in configuration
// lines, op commands and action lines.
func (f *File) Expand(specs []topology.VMSpec) error {
	var firstErr error
	sub := func(s string) string {
		return placeholder.ReplaceAllStringFunc(s, func(m string) string {
			p := placeholder.FindStringSubmatch(m)
			ifc, err := topology.Interface(specs, p[1], p[2])
			if err != nil {
				if firstErr == nil {
					firstErr = fmt.Errorf("%s: %s: %w", f.Name, m, err)
				}
				return m
			}
			return ifc
		})
	}
	for name, r := range f.Routers {
		for i := range r.Config {
			r.Config[i] = sub(r.Config[i])
		}
		f.Routers[name] = r
	}
	for i := range f.Checks {
		c := &f.Checks[i]
		if c.Op != nil {
			c.Op.Command = sub(c.Op.Command)
		}
		if c.Action != nil {
			for j := range c.Action.Configure {
				c.Action.Configure[j] = sub(c.Action.Configure[j])
			}
		}
	}
	return firstErr
}
