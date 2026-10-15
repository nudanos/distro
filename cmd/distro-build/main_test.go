package main

import (
	"errors"
	"github.com/nudanos/distro/internal/engine"
	"github.com/nudanos/distro/internal/manifest"
	"os"
	"reflect"
	"strings"
	"testing"
)

// Editing builder/ without re-running `builder` must not build with the old
// image under the new cache key.
func TestCheckBuilder(t *testing.T) {
	if err := checkBuilder("img", "h1", "h1"); err != nil {
		t.Errorf("matching hash: %v", err)
	}
	for _, label := range []string{"", "h0"} {
		err := checkBuilder("img", label, "h1")
		if err == nil || !strings.Contains(err.Error(), "distro-build builder") {
			t.Errorf("label %q: err = %v, want advice to run distro-build builder", label, err)
		}
	}
}

// Parallel builds interleave on stderr, so each build also gets its own log.
func TestPackageLogWritesPerPackageFile(t *testing.T) {
	dir := t.TempDir()
	w, err := packageLog(dir, "vyatta-cfg", false)
	if err != nil {
		t.Fatal(err)
	}
	w.Write([]byte("hello\n"))
	w.Close()
	b, err := os.ReadFile(dir + "/logs/vyatta-cfg.log")
	if err != nil || string(b) != "hello\n" {
		t.Errorf("log = %q, %v", b, err)
	}
}

func TestBuildEnvCarriesProfiles(t *testing.T) {
	got := buildEnv("vyatta-dataplane", 501, 20, 4, []string{"pkg.a", "pkg.b"}, false)
	want := map[string]string{"PKG": "vyatta-dataplane", "HOST_UID": "501", "HOST_GID": "20", "JOBS": "4",
		"DEB_BUILD_PROFILES": "pkg.a pkg.b"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildEnv = %v, want %v", got, want)
	}
	if _, ok := buildEnv("x", 0, 0, 1, nil, false)["DEB_BUILD_PROFILES"]; ok {
		t.Error("no profiles must not set DEB_BUILD_PROFILES")
	}
}

func TestKeyExtrasCarryProfilesOrderIndependently(t *testing.T) {
	m := &manifest.Manifest{Packages: []manifest.Entry{
		{Name: "dp", Profiles: []string{"pkg.b", "pkg.a"}},
		{Name: "plain"},
	}}
	got := keyExtras(m)
	if got["dp"] != "profiles=pkg.a,pkg.b" {
		t.Errorf("dp = %q", got["dp"])
	}
	if _, ok := got["plain"]; ok {
		t.Errorf("plain has a key extra: %q", got["plain"])
	}
}

// A closure build (per-package CI) must not fail because an unrelated entry
// could not be fetched, but must fail, naming it, when the closure needs it.
func TestFetchFailuresBlockOnlyTheirClosure(t *testing.T) {
	failed := map[string]error{"unrelated": errors.New("host down"), "dep": errors.New("no branch trixie")}
	if err := fetchBlocks(map[string]bool{"pkg": true}, failed); err != nil {
		t.Errorf("unrelated failure blocked the build: %v", err)
	}
	err := fetchBlocks(map[string]bool{"pkg": true, "dep": true}, failed)
	if err == nil || !strings.Contains(err.Error(), "dep") || !strings.Contains(err.Error(), "no branch trixie") ||
		strings.Contains(err.Error(), "unrelated") {
		t.Errorf("err = %v, want one naming dep and its fetch error only", err)
	}
}

func TestScenarioSpecMounts(t *testing.T) {
	r := scenarioRun{ISO: "/w/image/nudanos-1.0~20261005-amd64.iso", Tests: "/d/tests", Work: "/w/scenarios", Names: []string{"bgp"}}
	spec, err := scenarioSpec(r)
	if err != nil {
		t.Fatal(err)
	}
	mounts := map[string]engine.Mount{}
	for _, m := range spec.Mounts {
		mounts[m.Container] = m
	}
	if m := mounts["/iso"]; m.Host != "/w/image" || !m.ReadOnly {
		t.Errorf("/iso mount = %+v", m)
	}
	if m := mounts["/tests"]; m.Host != "/d/tests" || !m.ReadOnly {
		t.Errorf("/tests must be read-only without -capture: %+v", m)
	}
	if m := mounts["/work"]; m.Host != "/w/scenarios" || m.ReadOnly {
		t.Errorf("/work mount = %+v", m)
	}
	cmd := strings.Join(spec.Cmd, " ")
	for _, want := range []string{"/work/scenario", "-iso /iso/nudanos-1.0~20261005-amd64.iso", "-tests /tests", "-work /work", "-scenario bgp"} {
		if !strings.Contains(cmd, want) {
			t.Errorf("command %q lacks %q", cmd, want)
		}
	}
	r.Reference, r.Capture, r.Names, r.All = true, true, nil, true
	spec, err = scenarioSpec(r)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range spec.Mounts {
		if m.Container == "/tests" && m.ReadOnly {
			t.Error("/tests must be writable with -capture")
		}
	}
	cmd = strings.Join(spec.Cmd, " ")
	for _, want := range []string{"-reference", "-capture", "-all"} {
		if !strings.Contains(cmd, want) {
			t.Errorf("command %q lacks %q", cmd, want)
		}
	}
	if _, err := scenarioSpec(scenarioRun{ISO: "/x.iso", Capture: true, All: true}); err == nil {
		t.Error("-capture without -reference-iso must be an error")
	}
}

// lintScopeManifest has one entry of each kind/milestone/ready combination
// that matters for scope.
func lintScopeManifest() *manifest.Manifest {
	return &manifest.Manifest{Packages: []manifest.Entry{
		{Name: "vyatta-cfg", Kind: manifest.Danos, Milestone: "1.0", Ready: true},
		{Name: "dp", Kind: manifest.Danos, Milestone: "1.0", Ready: true, Profiles: []string{"pkg.b", "pkg.a"}},
		{Name: "kea-port", Kind: manifest.Danos, Milestone: "1.1", Ready: true},
		{Name: "not-ready", Kind: manifest.Danos, Milestone: "1.0"},
		{Name: "frr", Kind: manifest.Upstream, Milestone: "1.0", Ready: true},
		{Name: "dpdk", Kind: manifest.Danos, Milestone: "2"},
	}}
}

// Scope is spec section 1: ready danos entries of milestone 1.0.
func TestInScopeIsReadyDanos10(t *testing.T) {
	if got, want := inScope(lintScopeManifest()), []string{"dp", "vyatta-cfg"}; !reflect.DeepEqual(got, want) {
		t.Errorf("inScope = %v, want %v", got, want)
	}
}

func TestBuildEnvAddsWarningFlagsInScope(t *testing.T) {
	env := buildEnv("vyatta-cfg", 0, 0, 1, nil, true)
	if env["DEB_CFLAGS_MAINT_APPEND"] != "-Wall -Wextra" || env["DEB_CXXFLAGS_MAINT_APPEND"] != "-Wall -Wextra" {
		t.Errorf("in scope: %v", env)
	}
	if _, ok := buildEnv("frr", 0, 0, 1, nil, false)["DEB_CFLAGS_MAINT_APPEND"]; ok {
		t.Error("an out-of-scope package got the lint flags")
	}
}

// The flags are a build input outside the tree: in-scope packages must
// rebuild with them, others keep their cache.
func TestKeyExtrasIncludeFlagsInScope(t *testing.T) {
	got := keyExtras(lintScopeManifest())
	if got["vyatta-cfg"] != "flags=wall-wextra" {
		t.Errorf("vyatta-cfg = %q", got["vyatta-cfg"])
	}
	if got["dp"] != "profiles=pkg.a,pkg.b;flags=wall-wextra" {
		t.Errorf("dp = %q", got["dp"])
	}
	if _, ok := got["frr"]; ok {
		t.Errorf("frr has a key extra: %q", got["frr"])
	}
}

func TestPackageLogWritesFlagsLine(t *testing.T) {
	dir := t.TempDir()
	for name, in := range map[string]bool{"vyatta-cfg": true, "frr": false} {
		w, err := packageLog(dir, name, in)
		if err != nil {
			t.Fatal(err)
		}
		w.Close()
		b, _ := os.ReadFile(dir + "/logs/" + name + ".log")
		if has := strings.HasPrefix(string(b), "nudanos-lint-flags: wall-wextra\n"); has != in {
			t.Errorf("%s: log %q", name, b)
		}
	}
}

// The port is mounted read-only and the analyzers get no network.
func TestLintSpecMounts(t *testing.T) {
	spec := lintSpec("vyatta-cfg", "/w/src/vyatta-cfg", "/tmp/bin")
	mounts := map[string]engine.Mount{}
	for _, m := range spec.Mounts {
		mounts[m.Container] = m
	}
	if m := mounts["/src"]; m.Host != "/w/src/vyatta-cfg" || !m.ReadOnly {
		t.Errorf("/src mount = %+v", m)
	}
	if m := mounts["/usr/local/bin/lint"]; m.Host != "/tmp/bin/lint" || !m.ReadOnly {
		t.Errorf("binary mount = %+v", m)
	}
	if spec.Image != "nudanos/lint:trixie" || spec.Network != "none" {
		t.Errorf("image %q network %q", spec.Image, spec.Network)
	}
	if got := strings.Join(spec.Cmd, " "); got != "lint -port vyatta-cfg /src" {
		t.Errorf("cmd %q", got)
	}
}

func TestLintNamedOutOfScopeIsAnError(t *testing.T) {
	m := lintScopeManifest()
	if got, err := lintTargets(m, nil); err != nil || !reflect.DeepEqual(got, []string{"dp", "vyatta-cfg"}) {
		t.Errorf("all: %v %v", got, err)
	}
	if got, err := lintTargets(m, []string{"vyatta-cfg"}); err != nil || !reflect.DeepEqual(got, []string{"vyatta-cfg"}) {
		t.Errorf("named: %v %v", got, err)
	}
	if _, err := lintTargets(m, []string{"frr"}); err == nil || !strings.Contains(err.Error(), "frr") {
		t.Errorf("out of scope accepted: %v", err)
	}
}
