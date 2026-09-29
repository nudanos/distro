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
	SrcDirs   map[string]string // manifest name -> source checkout
	OutRoot   string
	StateFile string // JSON: name -> cache key of the last successful build
	KeySalt   string // part of every cache key; change it to invalidate all
	Build     Func
	Log       io.Writer
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

// Run builds every package in tiers order. A package whose dependency failed or
// was skipped is skipped. The error return is reserved for state-file I/O.
func (b *Builder) Run(ctx context.Context, tiers [][]string, deps map[string][]string) ([]Result, error) {
	state, err := loadState(b.StateFile)
	if err != nil {
		return nil, err
	}
	bad := map[string]bool{}
	var results []Result
	record := func(r Result) {
		if r.Status == Failed || r.Status == Skipped {
			bad[r.Name] = true
		}
		results = append(results, r)
		fmt.Fprintf(b.Log, "==> %-40s %s %s\n", r.Name, r.Status, r.Detail)
	}
	for _, tier := range tiers {
		for _, name := range tier {
			if err := ctx.Err(); err != nil {
				return results, err
			}
			blocker := ""
			for _, d := range deps[name] {
				if bad[d] {
					blocker = d
					break
				}
			}
			if blocker != "" {
				record(Result{name, Skipped, "dependency " + blocker + " did not build"})
				continue
			}
			src, out := b.SrcDirs[name], filepath.Join(b.OutRoot, name)
			hash, err := TreeHash(src)
			if err != nil {
				record(Result{name, Failed, err.Error()})
				continue
			}
			key := hash + ":" + b.KeySalt
			if state[name] == key && HasArtifacts(out) {
				record(Result{name, Cached, ""})
				continue
			}
			if err := os.RemoveAll(out); err != nil {
				return results, err
			}
			if err := os.MkdirAll(out, 0o755); err != nil {
				return results, err
			}
			err = b.Build(ctx, name, src, out)
			if err == nil && !HasArtifacts(out) {
				err = errors.New("build produced no .deb files")
			}
			if err != nil {
				delete(state, name)
				record(Result{name, Failed, err.Error()})
			} else {
				state[name] = key
				record(Result{name, Built, ""})
			}
			if err := saveState(b.StateFile, state); err != nil {
				return results, err
			}
		}
	}
	return results, nil
}
