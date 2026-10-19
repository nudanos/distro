package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"github.com/nudanos/distro/internal/engine"
	"github.com/nudanos/distro/internal/lint"
	"github.com/nudanos/distro/internal/manifest"
)

const lintImage = "nudanos/lint:trixie"

// lintTargets is every in-scope port, or the named ones, each of which must
// be in scope.
func lintTargets(m *manifest.Manifest, names []string) ([]string, error) {
	scope := inScope(m)
	if len(names) == 0 {
		return scope, nil
	}
	in := map[string]bool{}
	for _, n := range scope {
		in[n] = true
	}
	for _, n := range names {
		if !in[n] {
			return nil, fmt.Errorf("%s is not in lint scope (ready danos entries of milestone 1.0)", n)
		}
	}
	out := append([]string(nil), names...)
	sort.Strings(out)
	return out, nil
}

// lintSpec runs the lint binary from binDir over one port's source, read-only
// and without network.
func lintSpec(port, src, binDir string) engine.RunSpec {
	return engine.RunSpec{Image: lintImage, Network: "none",
		Mounts: []engine.Mount{
			{Host: src, Container: "/src", ReadOnly: true},
			{Host: filepath.Join(binDir, "lint"), Container: "/usr/local/bin/lint", ReadOnly: true},
		},
		Cmd: []string{"lint", "-port", port, "/src"}}
}

// lintPort returns one port's findings: the analyzers in the lint image,
// then compiler warnings and staticcheck from its build log.
func (a *app) lintPort(ctx context.Context, port, src, binDir string) ([]lint.Finding, error) {
	var out, errb bytes.Buffer
	if err := a.eng.Run(ctx, lintSpec(port, src, binDir), &out, &errb); err != nil {
		return nil, fmt.Errorf("%s: %v: %s", port, err, bytes.TrimSpace(errb.Bytes()))
	}
	var found []lint.Finding
	if err := json.Unmarshal(out.Bytes(), &found); err != nil {
		return nil, fmt.Errorf("%s: reading lint output: %w", port, err)
	}
	cfg, err := lint.LoadConfig(src)
	if err != nil {
		return nil, err
	}
	byLang, err := lint.Classify(src, cfg)
	if err != nil {
		return nil, err
	}
	langs := map[string]bool{}
	for l := range byLang {
		langs[l] = true
	}
	if langs["c"] || langs["go"] {
		log, err := os.ReadFile(filepath.Join(a.work, "logs", port+".log"))
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("%s: no build log, build %s", port, port)
		}
		if err != nil {
			return nil, err
		}
		more, err := lint.FromBuildLog(port, langs, log)
		if err != nil {
			return nil, err
		}
		found = append(found, more...)
	}
	return found, nil
}

// lintCmd checks the in-scope ports (or the named ones) and applies the
// baseline ratchet; -init-baseline and -update-baseline write it instead.
func (a *app) lintCmd(ctx context.Context, names []string) error {
	m, err := manifest.Load(a.manifest)
	if err != nil {
		return err
	}
	ports, err := lintTargets(m, names)
	if err != nil {
		return err
	}
	// Linting named ports, like building them, tolerates fetch failures of
	// unrelated entries; a named port that could not be fetched still fails.
	dirs, fetchFailed, err := a.fetch(ctx, m, len(names) > 0)
	if err != nil {
		return err
	}
	keep := map[string]bool{}
	for _, p := range ports {
		keep[p] = true
	}
	if err := fetchBlocks(keep, fetchFailed); err != nil {
		return err
	}
	dir := filepath.Dir(a.manifest)
	if err := a.eng.BuildImage(ctx, filepath.Join(dir, "lint"), lintImage, nil, os.Stderr, os.Stderr); err != nil {
		return err
	}
	bin, err := os.MkdirTemp("", "distro-build-lint")
	if err != nil {
		return err
	}
	defer os.RemoveAll(bin)
	build := exec.CommandContext(ctx, "go", "build", "-o", filepath.Join(bin, "lint"), "./cmd/lint")
	build.Dir, build.Env = dir, append(os.Environ(), "CGO_ENABLED=0", "GOOS=linux", "GOARCH=amd64")
	build.Stdout, build.Stderr = os.Stderr, os.Stderr
	if err := build.Run(); err != nil {
		return fmt.Errorf("lint: building the runner: %w", err)
	}

	var all []lint.Finding
	var failed []error
	counts := map[string]int{}
	for _, p := range ports {
		found, err := a.lintPort(ctx, p, dirs[p], bin)
		if err != nil {
			failed = append(failed, err)
			continue
		}
		counts[p] = len(found)
		all = append(all, found...)
	}
	sort.SliceStable(all, func(i, j int) bool {
		x, y := all[i], all[j]
		if x.Port != y.Port {
			return x.Port < y.Port
		}
		if x.File != y.File {
			return x.File < y.File
		}
		return x.Line < y.Line
	})
	withFindings := 0
	for _, n := range counts {
		if n > 0 {
			withFindings++
		}
	}
	for _, f := range all {
		fmt.Println(f)
	}
	fmt.Printf("lint: %d findings in %d ports (%d checked)\n", len(all), withFindings, len(counts))
	if len(failed) > 0 {
		return fmt.Errorf("%d ports could not be checked: %w", len(failed), errors.Join(failed...))
	}

	path := filepath.Join(dir, lint.BaselineFile)
	b, exists, err := lint.LoadBaseline(path)
	if err != nil {
		return err
	}
	switch {
	case a.initBaseline:
		if exists {
			return fmt.Errorf("%s exists; use -update-baseline to lower it", path)
		}
		return lint.NewBaseline(counts).Save(path)
	case a.updateBaseline:
		nb, err := b.Update(counts, ports)
		if err != nil {
			return err
		}
		return nb.Save(path)
	}
	if over := b.Check(counts); len(over) > 0 {
		for _, o := range over {
			fmt.Println("lint: worse than baseline:", o)
		}
		return fmt.Errorf("%d ports have more findings than %s allows", len(over), lint.BaselineFile)
	}
	return nil
}
