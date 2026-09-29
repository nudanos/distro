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
