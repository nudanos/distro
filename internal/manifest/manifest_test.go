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
		"unknown field": {"packages:\n  - name: a\n    kind: danos\n    milestone: \"1.0\"\n    repo: r\n    ref: x\n    reff: typo\n", "reff"},
		"duplicate":     {"packages:\n  - {name: a, kind: debian, milestone: \"1.0\"}\n  - {name: a, kind: debian, milestone: \"1.0\"}\n", "duplicate"},
		"danos no ref":  {"packages:\n  - {name: a, kind: danos, milestone: \"1.0\", repo: r}\n", "repo and ref"},
		"bad track":     {"packages:\n  - {name: a, kind: upstream, milestone: \"1.1\", upstream: u, packaging: p, track: newest}\n", "track"},
		"apt source":    {"packages:\n  - {name: a, kind: apt, milestone: \"1.0\", source: \"https://x trixie\", key: k, packages: {p: \"1\"}}\n", "URL SUITE COMPONENT"},
		"bad kind":      {"packages:\n  - {name: a, kind: rpm, milestone: \"1.0\"}\n", "unknown kind"},
		"bad milestone": {"packages:\n  - {name: a, kind: debian, milestone: \"3\"}\n", "milestone"},
		"debian ready":  {"packages:\n  - {name: a, kind: debian, milestone: \"1.0\", ready: true}\n", "cannot be ready"},
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

func TestUpstreamReadyNeedsPins(t *testing.T) {
	base := "packages:\n  - {name: k, kind: upstream, milestone: \"1.0\", ready: true, upstream: u, packaging: p, track: latest"
	cases := map[string]struct{ tail, want string }{
		"no version": {", tag: v1, tag_pattern: '^v(.*)$', packaging_ref: master}\n", "version"},
		"no tag":     {", version: \"1\", tag_pattern: '^v(.*)$', packaging_ref: master}\n", "tag"},
		"no ref":     {", version: \"1\", tag: v1, tag_pattern: '^v(.*)$'}\n", "packaging_ref"},
		"bad regexp": {", version: \"1\", tag: v1, tag_pattern: '^v(', packaging_ref: master}\n", "tag_pattern"},
		"no group":   {", version: \"1\", tag: v1, tag_pattern: '^v.*$', packaging_ref: master}\n", "capture group"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Parse([]byte(base + c.tail))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("err = %v, want mention of %q", err, c.want)
			}
		})
	}
	ok := base + ", version: \"2.4.3\", tag: v2.4.3, tag_pattern: '^v(\\d+\\.\\d+\\.\\d+)$', packaging_ref: master}\n"
	if _, err := Parse([]byte(ok)); err != nil {
		t.Errorf("complete upstream entry rejected: %v", err)
	}
}

func TestAuditVerdicts(t *testing.T) {
	y := "packages:\n  - name: k\n    kind: upstream\n    milestone: \"1.0\"\n    upstream: u\n    packaging: p\n    track: latest\n    audit:\n      - {patch: a.patch, verdict: maybe, note: x}\n"
	if _, err := Parse([]byte(y)); err == nil || !strings.Contains(err.Error(), "verdict") {
		t.Errorf("err = %v, want verdict error", err)
	}
}

// Upstreams that ship their own Debian packaging (perfSONAR) need no separate
// packaging repository or ref.
func TestUpstreamInTreePackaging(t *testing.T) {
	y := "packages:\n  - name: owamp\n    kind: upstream\n    milestone: \"1.0\"\n    ready: true\n    upstream: u\n    track: latest\n    version: 5.2.6\n    tag: v5.2.6\n    tag_pattern: '^v(.+)$'\n    subdir: owamp/owamp\n    packaging_dir: owamp/owamp/unibuild-packaging/deb\n"
	if _, err := Parse([]byte(y)); err != nil {
		t.Fatalf("in-tree packaging rejected: %v", err)
	}
	both := strings.Replace(y, "    subdir:", "    packaging: p\n    subdir:", 1)
	if _, err := Parse([]byte(both)); err == nil {
		t.Error("packaging and packaging_dir together accepted")
	}
}

func TestProfiles(t *testing.T) {
	m, err := Parse([]byte("packages:\n  - {name: d, kind: danos, milestone: \"1.0\", repo: r, ref: trixie, profiles: [pkg.vyatta-dataplane.protobuf-only]}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got := m.Packages[0].Profiles; len(got) != 1 || got[0] != "pkg.vyatta-dataplane.protobuf-only" {
		t.Errorf("Profiles = %v", got)
	}
}

func TestHold(t *testing.T) {
	m, err := Parse([]byte("packages:\n  - {name: p, kind: upstream, milestone: \"1.0\", upstream: u, packaging: p, track: latest, hold: \"reason\"}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if m.Packages[0].Hold != "reason" {
		t.Errorf("Hold = %q", m.Packages[0].Hold)
	}
}
