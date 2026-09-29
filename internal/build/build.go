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
//
// Each tier runs in three phases: decide what is cached and empty the output
// of everything that will rebuild; run BeforeTier (which indexes the pool, so
// the index cannot list files about to disappear); then build.
func (b *Builder) Run(ctx context.Context, tiers [][]string, deps map[string][]string) ([]Result, error) {
	state, err := loadState(b.StateFile)
	if err != nil {
		return nil, err
	}
	workers := b.Workers
	if workers < 1 {
		workers = 1
	}
	var mu sync.Mutex // guards state and state-file writes during a tier's builds
	bad := map[string]bool{}
	var results []Result
	type job struct {
		i         int
		name, key string
	}
	for _, tier := range tiers {
		if err := ctx.Err(); err != nil {
			return results, err
		}
		tierResults := make([]Result, len(tier))
		var jobs []job
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
			key, err := b.key(name, deps[name], state)
			if err != nil {
				tierResults[i] = Result{name, Failed, err.Error()}
				continue
			}
			out := filepath.Join(b.OutRoot, name)
			if state[name] == key && HasArtifacts(out) {
				tierResults[i] = Result{name, Cached, ""}
				continue
			}
			if err := EmptyDir(out); err != nil {
				return results, err
			}
			jobs = append(jobs, job{i, name, key})
		}
		if len(jobs) > 0 && b.BeforeTier != nil {
			if err := b.BeforeTier(ctx); err != nil {
				return results, err
			}
		}
		errs := make([]error, len(tier))
		sem := make(chan struct{}, workers)
		var wg sync.WaitGroup
		for _, j := range jobs {
			wg.Add(1)
			sem <- struct{}{}
			go func(j job) {
				defer wg.Done()
				defer func() { <-sem }()
				tierResults[j.i], errs[j.i] = b.build(ctx, j.name, j.key, state, &mu)
			}(j)
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

// key is the cache key of name: its source hash, the builder salt, and its
// dependencies' keys (so a rebuilt dependency rebuilds its dependents,
// transitively). Dependencies finished in earlier tiers, so state is stable.
func (b *Builder) key(name string, deps []string, state map[string]string) (string, error) {
	hash, err := TreeHash(b.SrcDirs[name])
	if err != nil {
		return "", err
	}
	key := hash + ":" + b.KeySalt
	for _, d := range deps {
		key += ":" + d + "=" + state[d]
	}
	return key, nil
}

// build runs one package's build into its (already emptied) output directory
// and records the outcome under mu.
func (b *Builder) build(ctx context.Context, name, key string, state map[string]string, mu *sync.Mutex) (Result, error) {
	out := filepath.Join(b.OutRoot, name)
	err := b.Build(ctx, name, b.SrcDirs[name], out)
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
