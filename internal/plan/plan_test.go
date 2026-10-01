package plan

import (
	"reflect"
	"strings"
	"testing"

	"github.com/nudanos/distro/internal/control"
)

func src(bins []string, provides []string, bd ...[]string) *control.Source {
	return &control.Source{Binaries: bins, Provides: provides, BuildDepends: bd}
}

func TestBuildResolvesBinariesProvidesAndAlternatives(t *testing.T) {
	g, err := Build(map[string]*control.Source{
		"dh-yang": src([]string{"dh-yang"}, nil, []string{"debhelper-compat"}),
		"yang":    src([]string{"golang-github-danos-yang-dev"}, []string{"yang-virtual"}, []string{"dh-yang"}),
		"configd": src([]string{"configd"}, nil,
			[]string{"missing-in-set", "yang-virtual"}, // first in-set alternative wins
			[]string{"configd"},                        // self-dependency ignored
			[]string{"libc6-dev"}),                     // external dependency ignored
	})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{"dh-yang": nil, "yang": {"dh-yang"}, "configd": {"yang"}}
	for k, v := range want {
		if !reflect.DeepEqual(g.Deps[k], v) {
			t.Errorf("Deps[%s] = %v, want %v", k, g.Deps[k], v)
		}
	}
}

func TestTiers(t *testing.T) {
	g := &Graph{Deps: map[string][]string{"a": nil, "b": {"a"}, "c": {"a"}, "d": {"b", "c"}}}
	tiers, err := g.Tiers()
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{{"a"}, {"b", "c"}, {"d"}}
	if !reflect.DeepEqual(tiers, want) {
		t.Errorf("Tiers = %v, want %v", tiers, want)
	}
}

func TestTiersReportsCycle(t *testing.T) {
	g := &Graph{Deps: map[string][]string{"a": {"b"}, "b": {"a"}, "c": nil}}
	tiers, err := g.Tiers()
	if err == nil || !strings.Contains(err.Error(), "a, b") {
		t.Fatalf("err = %v, want cycle naming a, b", err)
	}
	if !reflect.DeepEqual(tiers, [][]string{{"c"}}) {
		t.Errorf("tiers before cycle = %v", tiers)
	}
}

// Two sources producing the same binary make the order ambiguous: which one a
// dependent gets depends on map iteration. encoding and
// golang-github-danos-encoding-rfc7951 both build the rfc7951 -dev package.
func TestBuildRejectsDuplicateProducers(t *testing.T) {
	_, err := Build(map[string]*control.Source{
		"encoding":                             src([]string{"golang-github-danos-encoding-rfc7951-dev"}, nil),
		"golang-github-danos-encoding-rfc7951": src([]string{"golang-github-danos-encoding-rfc7951-dev"}, nil),
	})
	if err == nil || !strings.Contains(err.Error(), "golang-github-danos-encoding-rfc7951-dev") ||
		!strings.Contains(err.Error(), "encoding") {
		t.Fatalf("err = %v, want duplicate producer error naming the binary and sources", err)
	}
}
func TestClosureIncludesTransitiveDependenciesOnly(t *testing.T) {
	g := &Graph{Deps: map[string][]string{"a": nil, "b": {"a"}, "c": {"b"}, "d": {"a"}, "e": nil}}
	got, err := g.Closure([]string{"c"})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"a": true, "b": true, "c": true}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Closure(c) = %v, want %v", got, want)
	}
	if _, err := g.Closure([]string{"nope"}); err == nil || !strings.Contains(err.Error(), "nope") {
		t.Errorf("unknown name: err = %v", err)
	}
}

func TestFilterKeepsOrderAndDropsEmptyTiers(t *testing.T) {
	tiers := [][]string{{"a", "e"}, {"b", "d"}, {"c"}}
	got := Filter(tiers, map[string]bool{"a": true, "c": true})
	want := [][]string{{"a"}, {"c"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Filter = %v, want %v", got, want)
	}
}

// Every package is linted with lintian-profile-vyatta, but nothing
// build-depends on it: without an implicit edge a package's closure left it
// out, and CI linted every package with the bootstrap rules instead.
func TestAddImplicitDep(t *testing.T) {
	g := &Graph{Deps: map[string][]string{"a": nil, "b": {"a"}, "p": nil}}
	g.AddImplicitDep("p")
	want := map[string][]string{"a": {"p"}, "b": {"a", "p"}, "p": nil}
	if !reflect.DeepEqual(g.Deps, want) {
		t.Errorf("Deps = %v, want %v", g.Deps, want)
	}
	keep, err := g.Closure([]string{"a"})
	if err != nil || !keep["p"] {
		t.Errorf("Closure(a) = %v, %v; want it to include p", keep, err)
	}
	tiers, err := g.Tiers()
	if err != nil || !reflect.DeepEqual(tiers[0], []string{"p"}) {
		t.Errorf("Tiers = %v, %v; want p alone in tier 0", tiers, err)
	}
}

func TestAddImplicitDepAbsentIsNoop(t *testing.T) {
	g := &Graph{Deps: map[string][]string{"a": nil, "b": {"a"}}}
	g.AddImplicitDep("p")
	want := map[string][]string{"a": nil, "b": {"a"}}
	if !reflect.DeepEqual(g.Deps, want) {
		t.Errorf("Deps = %v, want %v", g.Deps, want)
	}
}

// A build-dependency is installed with its runtime dependencies, so the
// sources producing those must be built first and be in the pool: vyatta-system
// (build-dep of bonding) depends on vyatta-login, which depends on vyatta-cfg.
func TestBuildAddsRuntimeDependsOfBuildDeps(t *testing.T) {
	system := src([]string{"vyatta-system", "vyatta-cfg-system"}, nil)
	system.Depends = map[string][][]string{"vyatta-system": {{"vyatta-login"}, {"libc6"}}}
	login := src([]string{"vyatta-login"}, nil)
	login.Depends = map[string][][]string{"vyatta-login": {{"vyatta-cfg"}}}
	cfg := src([]string{"vyatta-cfg"}, nil)
	cfg.Depends = map[string][][]string{"vyatta-cfg": {{"vyatta-login"}}} // runtime cycles are normal
	bonding := src([]string{"vyatta-cfg-bonding"}, nil, []string{"vyatta-system"})
	g, err := Build(map[string]*control.Source{
		"vyatta-cfg-system": system, "vyatta-login": login, "vyatta-cfg": cfg, "vyatta-cfg-bonding": bonding})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"vyatta-cfg", "vyatta-cfg-system", "vyatta-login"}
	if !reflect.DeepEqual(g.Deps["vyatta-cfg-bonding"], want) {
		t.Errorf("bonding deps = %v, want %v", g.Deps["vyatta-cfg-bonding"], want)
	}
	if g.Deps["vyatta-cfg"] != nil || g.Deps["vyatta-login"] != nil {
		t.Errorf("runtime deps alone must not order builds: cfg=%v login=%v", g.Deps["vyatta-cfg"], g.Deps["vyatta-login"])
	}
	if _, err := g.Tiers(); err != nil {
		t.Errorf("tiers: %v", err)
	}
}

// Only the installed binary's dependencies count, not its siblings': vci
// builds the -dev library notifyd builds against and a daemon that depends on
// notifyd at runtime. Aggregating per source would make notifyd depend on itself.
func TestRuntimeDependsArePerBinary(t *testing.T) {
	vci := src([]string{"golang-github-danos-vci-dev", "vci"}, nil)
	vci.Depends = map[string][][]string{"vci": {{"notifyd"}}}
	notifyd := src([]string{"notifyd"}, nil, []string{"golang-github-danos-vci-dev"})
	g, err := Build(map[string]*control.Source{"vci": vci, "notifyd": notifyd})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(g.Deps["notifyd"], []string{"vci"}) {
		t.Errorf("notifyd deps = %v", g.Deps["notifyd"])
	}
	if _, err := g.Tiers(); err != nil {
		t.Errorf("tiers: %v", err)
	}
}
