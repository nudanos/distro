package manifest

import (
	"strings"
	"testing"
)

const good = `packages:
  - name: dh-yang
    kind: danos
    milestone: "1.0"
    ready: true
    repo: https://github.com/nudanos/dh-yang
    ref: trixie
  - name: frr
    kind: apt
    milestone: "1.0"
    ready: true
    source: "https://deb.frrouting.org/frr trixie frr-stable"
    key: https://deb.frrouting.org/frr/keys.gpg
    packages:
      frr: 10.7.1-0~deb13u1
      libyang3: 3.13.6-1~deb13u1
  - name: strongswan
    kind: upstream
    milestone: "1.1"
    upstream: https://github.com/strongswan/strongswan
    track: latest
    packaging: https://salsa.debian.org/debian/strongswan.git
  - name: openssh
    kind: debian
    milestone: "1.0"
  - name: tests
    kind: reference
    milestone: none
`

func TestParseGood(t *testing.T) {
	m, err := Parse([]byte(good))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Packages) != 5 {
		t.Fatalf("got %d entries", len(m.Packages))
	}
	if r := m.Ready(Danos); len(r) != 1 || r[0].Name != "dh-yang" {
		t.Errorf("Ready(Danos) = %v", r)
	}
	if r := m.Ready(Apt); len(r) != 1 || r[0].Packages["libyang3"] != "3.13.6-1~deb13u1" {
		t.Errorf("Ready(Apt) = %v", r)
	}
}

func TestParseRejects(t *testing.T) {
	cases := map[string]struct{ yaml, want string }{
		"unknown field":  {"packages:\n  - name: a\n    kind: danos\n    milestone: \"1.0\"\n    repo: r\n    ref: x\n    reff: typo\n", "reff"},
		"duplicate":      {"packages:\n  - {name: a, kind: debian, milestone: \"1.0\"}\n  - {name: a, kind: debian, milestone: \"1.0\"}\n", "duplicate"},
		"danos no ref":   {"packages:\n  - {name: a, kind: danos, milestone: \"1.0\", repo: r}\n", "repo and ref"},
		"upstream ready": {"packages:\n  - {name: a, kind: upstream, milestone: \"1.1\", ready: true, upstream: u, packaging: p, track: latest}\n", "not implemented"},
		"bad track":      {"packages:\n  - {name: a, kind: upstream, milestone: \"1.1\", upstream: u, packaging: p, track: newest}\n", "track"},
		"apt source":     {"packages:\n  - {name: a, kind: apt, milestone: \"1.0\", source: \"https://x trixie\", key: k, packages: {p: \"1\"}}\n", "URL SUITE COMPONENT"},
		"bad kind":       {"packages:\n  - {name: a, kind: rpm, milestone: \"1.0\"}\n", "unknown kind"},
		"bad milestone":  {"packages:\n  - {name: a, kind: debian, milestone: \"3\"}\n", "milestone"},
		"debian ready":   {"packages:\n  - {name: a, kind: debian, milestone: \"1.0\", ready: true}\n", "cannot be ready"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Parse([]byte(c.yaml))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("err = %v, want mention of %q", err, c.want)
			}
		})
	}
}
