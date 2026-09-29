// Command distro-build builds the NuDanOS package set.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/nudanos/distro/internal/aptmirror"
	"github.com/nudanos/distro/internal/aptrepo"
	"github.com/nudanos/distro/internal/build"
	"github.com/nudanos/distro/internal/control"
	"github.com/nudanos/distro/internal/engine"
	"github.com/nudanos/distro/internal/fetch"
	"github.com/nudanos/distro/internal/manifest"
	"github.com/nudanos/distro/internal/plan"
	"github.com/nudanos/distro/internal/upstream"
	"github.com/nudanos/distro/internal/workspace"
)

type localFlags map[string]string

func (l localFlags) String() string {
	var s []string
	for k, v := range l {
		s = append(s, k+"="+v)
	}
	sort.Strings(s)
	return strings.Join(s, ",")
}

func (l localFlags) Set(v string) error {
	name, dir, ok := strings.Cut(v, "=")
	if !ok || name == "" || dir == "" {
		return fmt.Errorf("want name=dir, got %q", v)
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	l[name] = abs
	return nil
}

// builderLabel records the hash of builder/ an image was built from.
const builderLabel = "org.nudanos.builder-hash"

// checkBuilder fails when the image was not built from the current builder/.
func checkBuilder(image, label, want string) error {
	if label != want {
		return fmt.Errorf("builder image %s is out of date with builder/ (image %q, directory %q); "+
			"run 'distro-build builder' first", image, label, want)
	}
	return nil
}

type app struct {
	manifest, work, image, builderDir, gnupg, key string
	eng                                           engine.Engine
	local                                         localFlags
	jobs                                          int
}

func (a *app) outRoot() string { return filepath.Join(a.work, "out") }

func (a *app) checkWork() error {
	ok, err := workspace.CaseSensitive(a.work)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("work directory %s is on a case-insensitive filesystem; DANOS sources contain "+
			"names that differ only by case. Use a case-sensitive volume (see docs/workspace.md)", a.work)
	}
	return nil
}

// fetch mirrors every ready apt entry, then checks out every ready danos entry
// (or uses its -local override) and returns the source directory for each.
func (a *app) fetch(ctx context.Context, m *manifest.Manifest) (map[string]string, error) {
	if err := a.checkWork(); err != nil {
		return nil, err
	}
	if err := build.Prune(a.outRoot(), readyNames(m), os.Stderr); err != nil {
		return nil, err
	}
	if len(m.Ready(manifest.Apt)) > 0 {
		if _, err := a.builderSalt(ctx); err != nil {
			return nil, err
		}
	}
	if err := a.mirror(ctx, m); err != nil {
		return nil, err
	}
	dirs := map[string]string{}
	for _, e := range m.Ready(manifest.Danos) {
		if d, ok := a.local[e.Name]; ok {
			dirs[e.Name] = d
			continue
		}
		d := filepath.Join(a.work, "src", e.Name)
		fmt.Fprintf(os.Stderr, "==> fetch %s@%s\n", e.Name, e.Ref)
		if err := fetch.Git(ctx, e.Repo, e.Ref, d, os.Stderr); err != nil {
			return nil, err
		}
		dirs[e.Name] = d
	}
	for _, e := range m.Ready(manifest.Upstream) {
		fmt.Fprintf(os.Stderr, "==> prepare %s %s (%s)\n", e.Name, e.Version, e.Tag)
		d, err := upstream.Prepare(ctx, e, filepath.Join(filepath.Dir(a.manifest), "patches"), a.work, os.Stderr)
		if err != nil {
			return nil, err
		}
		dirs[e.Name] = d
	}

	for name := range a.local {
		if _, ok := dirs[name]; !ok {
			return nil, fmt.Errorf("-local %s: no ready danos entry with that name", name)
		}
	}
	return dirs, nil
}

// readyNames is every entry whose output belongs in the pool.
func readyNames(m *manifest.Manifest) map[string]bool {
	keep := map[string]bool{}
	for _, k := range []manifest.Kind{manifest.Danos, manifest.Apt, manifest.Upstream} {
		for _, e := range m.Ready(k) {
			keep[e.Name] = true
		}
	}
	return keep
}

func graph(dirs map[string]string) (*plan.Graph, [][]string, error) {
	srcs := map[string]*control.Source{}
	for name, d := range dirs {
		s, err := control.ReadSource(filepath.Join(d, "debian", "control"))
		if err != nil {
			return nil, nil, err
		}
		srcs[name] = s
	}
	g, err := plan.Build(srcs)
	if err != nil {
		return nil, nil, err
	}
	tiers, err := g.Tiers()
	return g, tiers, err
}

func (a *app) containerBuild() build.Func {
	u, g := a.eng.OwnerIDs(os.Getuid(), os.Getgid())
	uid, gid := strconv.Itoa(u), strconv.Itoa(g)
	return func(ctx context.Context, name, src, out string) error {
		return a.eng.Run(ctx, engine.RunSpec{
			Image: a.image,
			Mounts: []engine.Mount{
				{Host: src, Container: "/src", ReadOnly: true},
				{Host: a.outRoot(), Container: "/pool", ReadOnly: true},
				{Host: out, Container: "/out"},
			},
			Env: map[string]string{"PKG": name, "HOST_UID": uid, "HOST_GID": gid,
				"JOBS": strconv.Itoa(max(1, runtime.NumCPU()/max(1, a.jobs)))},
			Cmd: []string{"/usr/local/bin/build-package"},
		}, os.Stderr, os.Stderr)
	}
}

// builderSalt returns the builder hash after checking the image matches builder/.
func (a *app) builderSalt(ctx context.Context) (string, error) {
	want, err := build.TreeHash(a.builderDir)
	if err != nil {
		return "", fmt.Errorf("hashing builder dir: %w", err)
	}
	label, err := a.eng.ImageLabel(ctx, a.image, builderLabel)
	if err != nil {
		return "", err
	}
	return want, checkBuilder(a.image, label, want)
}

func (a *app) run(ctx context.Context, cmd string, names []string) error {
	switch cmd {
	case "builder":
		hash, err := build.TreeHash(a.builderDir)
		if err != nil {
			return fmt.Errorf("hashing builder dir: %w", err)
		}
		return a.eng.BuildImage(ctx, a.builderDir, a.image, map[string]string{builderLabel: hash}, os.Stderr, os.Stderr)
	case "fetch", "plan", "build":
	case "repo":
		return a.repo(ctx)
	default:
		return fmt.Errorf("unknown command %q", cmd)
	}
	m, err := manifest.Load(a.manifest)
	if err != nil {
		return err
	}
	dirs, err := a.fetch(ctx, m)
	if err != nil || cmd == "fetch" {
		return err
	}
	g, tiers, err := graph(dirs)
	if err != nil {
		return err
	}
	if len(names) > 0 {
		keep, err := g.Closure(names)
		if err != nil {
			return err
		}
		tiers = plan.Filter(tiers, keep)
	}
	if cmd == "plan" {
		for i, t := range tiers {
			fmt.Printf("tier %d (%d): %s\n", i, len(t), strings.Join(t, " "))
		}
		return nil
	}
	salt, err := a.builderSalt(ctx)
	if err != nil {
		return err
	}
	b := &build.Builder{SrcDirs: dirs, OutRoot: a.outRoot(), StateFile: filepath.Join(a.work, "state.json"),
		KeySalt: a.image + ":" + salt, Build: a.containerBuild(), Log: os.Stderr, Workers: a.jobs, BeforeTier: a.indexPool}
	results, err := b.Run(ctx, tiers, g.Deps)
	if err != nil {
		return err
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	failed := 0
	for _, r := range results {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", r.Name, r.Status, r.Detail)
		if r.Status == build.Failed || r.Status == build.Skipped {
			failed++
		}
	}
	tw.Flush()
	if failed > 0 {
		return fmt.Errorf("%d of %d packages failed or were skipped", failed, len(results))
	}
	return nil
}

func (a *app) mirror(ctx context.Context, m *manifest.Manifest) error {
	for _, e := range m.Ready(manifest.Apt) {
		cached, err := aptmirror.Mirror(ctx, a.eng, a.image, e, a.outRoot(),
			filepath.Join(a.work, "mirror-state.json"), os.Stderr)
		if err != nil {
			return err
		}
		status := "downloaded"
		if cached {
			status = "cached"
		}
		fmt.Fprintf(os.Stderr, "==> mirror %s %s\n", e.Name, status)
	}
	return nil
}

func (a *app) repo(ctx context.Context) error {
	if a.key == "" {
		return fmt.Errorf("repo: -key <fingerprint> is required")
	}
	if err := a.checkWork(); err != nil {
		return err
	}
	m, err := manifest.Load(a.manifest)
	if err != nil {
		return err
	}
	if err := build.Prune(a.outRoot(), readyNames(m), os.Stderr); err != nil {
		return err
	}
	if _, err := a.builderSalt(ctx); err != nil {
		return err
	}
	dir := filepath.Join(a.work, "repo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	uid, gid := a.eng.OwnerIDs(os.Getuid(), os.Getgid())
	return a.eng.Run(ctx, aptrepo.Spec(a.image, a.outRoot(), dir, a.gnupg, a.key, uid, gid),
		os.Stderr, os.Stderr)
}

func main() {
	home, _ := os.UserHomeDir()
	a := &app{local: localFlags{}}
	flag.StringVar(&a.manifest, "manifest", "manifest.yaml", "path to manifest.yaml")
	flag.StringVar(&a.work, "work", "work", "work directory (must be case-sensitive)")
	engineBin := flag.String("engine", "docker", "container engine: docker or podman")
	flag.StringVar(&a.image, "image", "nudanos/builder:trixie", "builder image tag")
	flag.StringVar(&a.builderDir, "builder-dir", "builder", "directory holding the builder Dockerfile")
	flag.StringVar(&a.gnupg, "gnupg", filepath.Join(home, ".nudanos", "gnupg"), "GnuPG home with the archive signing key (repo)")
	flag.StringVar(&a.key, "key", "", "archive signing key fingerprint (repo)")
	flag.IntVar(&a.jobs, "jobs", 1, "packages built at once within a tier")
	flag.Var(a.local, "local", "use a local checkout for a ready danos entry: name=dir (repeatable)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: distro-build [flags] builder|fetch|plan|build|repo|check-updates [name…]\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(2)
	}
	if *engineBin != "docker" && *engineBin != "podman" {
		fmt.Fprintln(os.Stderr, "error: -engine must be docker or podman")
		os.Exit(2)
	}
	a.eng = engine.Engine{Bin: *engineBin}
	abs, err := filepath.Abs(a.work)
	if err == nil {
		a.work = abs
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := a.run(ctx, flag.Arg(0), flag.Args()[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// indexPool writes the pool's apt index once, before a tier's builds start.
func (a *app) indexPool(ctx context.Context) error {
	if err := os.MkdirAll(a.outRoot(), 0o755); err != nil {
		return err
	}
	return a.eng.Run(ctx, engine.RunSpec{
		Image:   a.image,
		Mounts:  []engine.Mount{{Host: a.outRoot(), Container: "/pool"}},
		Network: "none",
		Cmd:     []string{"/usr/local/bin/index-pool"},
	}, os.Stderr, os.Stderr)
}
