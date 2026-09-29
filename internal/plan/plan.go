// Package plan orders source packages by their build dependencies.
package plan

import (
	"fmt"
	"sort"
	"strings"

	"github.com/nudanos/distro/internal/control"
)

// Graph holds build-order edges: Deps[a] lists the nodes a build-depends on.
type Graph struct {
	Deps map[string][]string
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Build derives edges from Build-Depends. A dependency counts only if another
// source in the set produces it (as a binary, or failing that as a Provides).
// For alternatives, the first alternative produced inside the set wins.
// Two sources producing the same binary package is an error.
func Build(sources map[string]*control.Source) (*Graph, error) {
	names := sortedKeys(sources)
	producer := map[string]string{}
	var dups []string
	for _, n := range names {
		for _, b := range sources[n].Binaries {
			if prev, taken := producer[b]; taken {
				dups = append(dups, fmt.Sprintf("%s (from %s and %s)", b, prev, n))
				continue
			}
			producer[b] = n
		}
	}
	if len(dups) > 0 {
		return nil, fmt.Errorf("binary packages produced by more than one source: %s", strings.Join(dups, "; "))
	}
	for _, n := range names {
		for _, p := range sources[n].Provides {
			if _, taken := producer[p]; !taken {
				producer[p] = n
			}
		}
	}
	g := &Graph{Deps: map[string][]string{}}
	for _, n := range names {
		set := map[string]bool{}
		for _, alts := range sources[n].BuildDepends {
			for _, a := range alts {
				if p, ok := producer[a]; ok {
					if p != n {
						set[p] = true
					}
					break
				}
			}
		}
		var deps []string
		if len(set) > 0 {
			deps = sortedKeys(set)
		}
		g.Deps[n] = deps
	}
	return g, nil
}

// Tiers groups nodes so each tier depends only on earlier tiers. On a cycle it
// returns the tiers resolved so far and an error naming the unresolvable nodes.
func (g *Graph) Tiers() ([][]string, error) {
	done := map[string]bool{}
	remaining := sortedKeys(g.Deps)
	var tiers [][]string
	for len(remaining) > 0 {
		var tier, rest []string
		for _, n := range remaining {
			ready := true
			for _, d := range g.Deps[n] {
				if !done[d] {
					ready = false
					break
				}
			}
			if ready {
				tier = append(tier, n)
			} else {
				rest = append(rest, n)
			}
		}
		if len(tier) == 0 {
			return tiers, fmt.Errorf("dependency cycle (or blocked by one) among: %s", strings.Join(rest, ", "))
		}
		for _, n := range tier {
			done[n] = true
		}
		tiers = append(tiers, tier)
		remaining = rest
	}
	return tiers, nil
}

// Closure returns names plus everything they transitively build-depend on.
func (g *Graph) Closure(names []string) (map[string]bool, error) {
	out := map[string]bool{}
	stack := append([]string(nil), names...)
	for _, n := range names {
		if _, ok := g.Deps[n]; !ok {
			return nil, fmt.Errorf("%s is not a ready package in the manifest", n)
		}
	}
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if out[n] {
			continue
		}
		out[n] = true
		stack = append(stack, g.Deps[n]...)
	}
	return out, nil
}

// Filter restricts tiers to the names in keep, preserving order.
func Filter(tiers [][]string, keep map[string]bool) [][]string {
	var out [][]string
	for _, t := range tiers {
		var kept []string
		for _, n := range t {
			if keep[n] {
				kept = append(kept, n)
			}
		}
		if len(kept) > 0 {
			out = append(out, kept)
		}
	}
	return out
}
