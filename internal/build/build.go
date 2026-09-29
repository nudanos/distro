// Package build runs ordered, cached package builds.
package build

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

// Status is the outcome of one package in a run.
type Status string

const (
	Built   Status = "built"
	Cached  Status = "cached"
	Failed  Status = "failed"
	Skipped Status = "skipped"
)

// Result reports one package.
type Result struct {
	Name   string
	Status Status
	Detail string
}

// Func builds the source in srcDir, writing .deb and source artifacts into outDir.
type Func func(ctx context.Context, name, srcDir, outDir string) error

// Builder builds packages tier by tier. Artifacts for package N go to
// OutRoot/N; the whole OutRoot is what later builds install dependencies from.
type Builder struct {
	SrcDirs    map[string]string // manifest name -> source checkout
	OutRoot    string
	StateFile  string // JSON: name -> cache key of the last successful build
	KeySalt    string // part of every cache key; change it to invalidate all
	Build      Func
	Log        io.Writer
	Workers    int                             // builds run at once within a tier; <=1 means serial
	BeforeTier func(ctx context.Context) error // e.g. index the pool; nil means none

}

// EmptyDir makes dir an empty directory without deleting dir itself. Engines
// that share host folders through a VM (Docker Desktop) keep a stale view of a
// directory that is deleted and recreated while its parent is mounted by
// another container, so output directories are emptied in place instead.
func EmptyDir(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := os.RemoveAll(filepath.Join(dir, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

// HasArtifacts reports whether dir holds at least one .deb.
func HasArtifacts(dir string) bool {
	m, _ := filepath.Glob(filepath.Join(dir, "*.deb"))
	return len(m) > 0
}

func loadState(path string) (map[string]string, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	st := map[string]string{}
	if err := json.Unmarshal(b, &st); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return st, nil
}

func saveState(path string, st map[string]string) error {
	b, err := json.MarshalIndent(st, "", " ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Run builds every package in tiers order, up to Workers at once within a
// tier. A package whose dependency failed or was skipped is skipped. Results
// come back in tier order. The error return is reserved for state-file I/O,
// the BeforeTier hook and cancellation.
func (b *Builder) Run(ctx context.Context, tiers [][]string, deps map[string][]string) ([]Result, error) {
	state, err := loadState(b.StateFile)
	if err != nil {
		return nil, err
	}
	workers := b.Workers
	if workers < 1 {
		workers = 1
	}
	var mu sync.Mutex // guards state, bad, and state-file writes
	bad := map[string]bool{}
	var results []Result
	for _, tier := range tiers {
		if err := ctx.Err(); err != nil {
			return results, err
		}
		if b.BeforeTier != nil {
			if err := b.BeforeTier(ctx); err != nil {
				return results, err
			}
		}
		tierResults := make([]Result, len(tier))
		errs := make([]error, len(tier))
		sem := make(chan struct{}, workers)
		var wg sync.WaitGroup
		for i, name := range tier {
			blocker := ""
			for _, d := range deps[name] {
				if bad[d] {
					blocker = d
					break
				}
			}
			if blocker != "" {
				tierResults[i] = Result{name, Skipped, "dependency " + blocker + " did not build"}
				continue
			}
			wg.Add(1)
			sem <- struct{}{}
			go func(i int, name string) {
				defer wg.Done()
				defer func() { <-sem }()
				tierResults[i], errs[i] = b.one(ctx, name, deps[name], state, &mu)
			}(i, name)
		}
		wg.Wait()
		for i, r := range tierResults {
			if errs[i] != nil {
				return results, errs[i]
			}
			if r.Status == Failed || r.Status == Skipped {
				bad[r.Name] = true
			}
			results = append(results, r)
			fmt.Fprintf(b.Log, "==> %-40s %s %s\n", r.Name, r.Status, r.Detail)
		}
	}
	return results, nil
}

// one builds a single package. It reads dependency keys from state (those
// packages finished in earlier tiers) and records its own key under mu.
func (b *Builder) one(ctx context.Context, name string, deps []string, state map[string]string, mu *sync.Mutex) (Result, error) {
	src, out := b.SrcDirs[name], filepath.Join(b.OutRoot, name)
	hash, err := TreeHash(src)
	if err != nil {
		return Result{name, Failed, err.Error()}, nil
	}
	mu.Lock()
	// Dependencies' keys are part of ours, so a rebuilt dependency rebuilds
	// its dependents (transitively, since their keys change in turn).
	key := hash + ":" + b.KeySalt
	for _, d := range deps {
		key += ":" + d + "=" + state[d]
	}
	cached := state[name] == key && HasArtifacts(out)
	mu.Unlock()
	if cached {
		return Result{name, Cached, ""}, nil
	}
	if err := EmptyDir(out); err != nil {
		return Result{}, err
	}
	err = b.Build(ctx, name, src, out)
	if err == nil && !HasArtifacts(out) {
		err = errors.New("build produced no .deb files")
	}
	mu.Lock()
	defer mu.Unlock()
	r := Result{name, Built, ""}
	if err != nil {
		delete(state, name)
		r = Result{name, Failed, err.Error()}
	} else {
		state[name] = key
	}
	return r, saveState(b.StateFile, state)
}
