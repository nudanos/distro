package build

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestTreeHashIgnoresGitButNotContentOrMode(t *testing.T) {
	d := t.TempDir()
	write(t, filepath.Join(d, "debian", "control"), "Source: a\n")
	write(t, filepath.Join(d, ".git", "HEAD"), "ref: x\n")
	h1, _ := TreeHash(d)
	write(t, filepath.Join(d, ".git", "HEAD"), "ref: y\n")
	if h2, _ := TreeHash(d); h2 != h1 {
		t.Error(".git change altered hash")
	}
	os.Chmod(filepath.Join(d, "debian", "control"), 0o755)
	h3, _ := TreeHash(d)
	if h3 == h1 {
		t.Error("mode change did not alter hash")
	}
	write(t, filepath.Join(d, "debian", "control"), "Source: b\n")
	if h4, _ := TreeHash(d); h4 == h3 {
		t.Error("content change did not alter hash")
	}
}

type fixture struct {
	mu    sync.Mutex
	b     *Builder
	built []string
	fail  map[string]bool
}

func newFixture(t *testing.T, names ...string) *fixture {
	root := t.TempDir()
	f := &fixture{fail: map[string]bool{}}
	dirs := map[string]string{}
	for _, n := range names {
		dirs[n] = filepath.Join(root, "src", n)
		write(t, filepath.Join(dirs[n], "debian", "control"), "Source: "+n+"\n")
	}
	f.b = &Builder{SrcDirs: dirs, OutRoot: filepath.Join(root, "out"), StateFile: filepath.Join(root, "state.json"),
		KeySalt: "builder-v1", Log: io.Discard,
		Build: func(ctx context.Context, name, src, out string) error {
			f.mu.Lock()
			f.built = append(f.built, name)
			f.mu.Unlock()
			if f.fail[name] {
				return errors.New("boom")
			}
			return os.WriteFile(filepath.Join(out, name+"_1_all.deb"), []byte(name), 0o644)
		}}
	return f
}

func statuses(rs []Result) map[string]Result {
	m := map[string]Result{}
	for _, r := range rs {
		m[r.Name] = r
	}
	return m
}

func TestRunSkipsDependentsOfAFailureAndBuildsTheRest(t *testing.T) {
	f := newFixture(t, "a", "b", "c", "d")
	f.fail["a"] = true
	rs, err := f.b.Run(context.Background(), [][]string{{"a", "c"}, {"b"}, {"d"}},
		map[string][]string{"b": {"a"}, "d": {"b", "c"}})
	if err != nil {
		t.Fatal(err)
	}
	s := statuses(rs)
	if s["a"].Status != Failed || s["c"].Status != Built {
		t.Errorf("a=%v c=%v", s["a"], s["c"])
	}
	if s["b"].Status != Skipped || !strings.Contains(s["b"].Detail, "a") {
		t.Errorf("b = %+v, want skipped naming a", s["b"])
	}
	if s["d"].Status != Skipped || !strings.Contains(s["d"].Detail, "b") {
		t.Errorf("d = %+v, want skipped naming b", s["d"])
	}
	if strings.Join(f.built, ",") != "a,c" {
		t.Errorf("built = %v, want a,c", f.built)
	}
}

func TestRunCachesUntilSourceOrBuilderChanges(t *testing.T) {
	f := newFixture(t, "a")
	run := func() Status {
		rs, err := f.b.Run(context.Background(), [][]string{{"a"}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		return rs[0].Status
	}
	if s := run(); s != Built {
		t.Fatalf("first run = %s", s)
	}
	if s := run(); s != Cached {
		t.Fatalf("second run = %s", s)
	}
	write(t, filepath.Join(f.b.SrcDirs["a"], ".git", "FETCH_HEAD"), "x")
	if s := run(); s != Cached {
		t.Fatalf("after .git change = %s", s)
	}
	write(t, filepath.Join(f.b.SrcDirs["a"], "debian", "rules"), "#!/usr/bin/make -f\n")
	if s := run(); s != Built {
		t.Fatalf("after source change = %s", s)
	}
	f.b.KeySalt = "builder-v2"
	if s := run(); s != Built {
		t.Fatalf("after builder change = %s", s)
	}
	os.RemoveAll(filepath.Join(f.b.OutRoot, "a"))
	if s := run(); s != Built {
		t.Fatalf("after artifacts removed = %s", s)
	}
}

func TestRunFailsWhenBuildProducesNoDebs(t *testing.T) {
	f := newFixture(t, "a")
	f.b.Build = func(ctx context.Context, name, src, out string) error { return nil }
	rs, _ := f.b.Run(context.Background(), [][]string{{"a"}}, nil)
	if rs[0].Status != Failed {
		t.Errorf("status = %s, want failed", rs[0].Status)
	}
}

// A rebuilt dependency must invalidate its dependents: Go -dev libraries are
// compiled statically into their consumers.
func TestRunRebuildsDependentsWhenADependencyChanges(t *testing.T) {
	f := newFixture(t, "a", "b")
	tiers, deps := [][]string{{"a"}, {"b"}}, map[string][]string{"b": {"a"}}
	run := func() map[string]Result {
		rs, err := f.b.Run(context.Background(), tiers, deps)
		if err != nil {
			t.Fatal(err)
		}
		return statuses(rs)
	}
	run()
	if s := run(); s["a"].Status != Cached || s["b"].Status != Cached {
		t.Fatalf("second run a=%s b=%s, want cached", s["a"].Status, s["b"].Status)
	}
	write(t, filepath.Join(f.b.SrcDirs["a"], "debian", "rules"), "#!/usr/bin/make -f\n")
	if s := run(); s["a"].Status != Built || s["b"].Status != Built {
		t.Fatalf("after changing a: a=%s b=%s, want both built", s["a"].Status, s["b"].Status)
	}
}

func TestRunBuildsATierConcurrently(t *testing.T) {
	f := newFixture(t, "a", "b", "c")
	var mu sync.Mutex
	started, release := 0, make(chan struct{})
	inner := f.b.Build
	f.b.Workers = 3
	f.b.Build = func(ctx context.Context, name, src, out string) error {
		mu.Lock()
		started++
		if started == 3 {
			close(release)
		}
		mu.Unlock()
		select {
		case <-release:
		case <-time.After(5 * time.Second):
			return errors.New("builds in one tier did not run concurrently")
		}
		return inner(ctx, name, src, out)
	}
	rs, err := f.b.Run(context.Background(), [][]string{{"a", "b", "c"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, r := range rs {
		if r.Status != Built {
			t.Errorf("%s = %s %s", r.Name, r.Status, r.Detail)
		}
		names = append(names, r.Name)
	}
	if strings.Join(names, ",") != "a,b,c" {
		t.Errorf("result order = %v, want tier order a,b,c", names)
	}
}

func TestParallelFailureStillSkipsDependents(t *testing.T) {
	f := newFixture(t, "a", "b", "c", "d")
	f.b.Workers = 4
	f.fail["b"] = true
	rs, err := f.b.Run(context.Background(), [][]string{{"a", "b", "c"}, {"d"}}, map[string][]string{"d": {"b"}})
	if err != nil {
		t.Fatal(err)
	}
	s := statuses(rs)
	if s["b"].Status != Failed || s["d"].Status != Skipped || s["a"].Status != Built || s["c"].Status != Built {
		t.Errorf("statuses = %+v", s)
	}
	st, err := loadState(f.b.StateFile)
	if err != nil {
		t.Fatalf("state file unreadable after parallel run: %v", err)
	}
	if st["a"] == "" || st["c"] == "" || st["b"] != "" {
		t.Errorf("state = %v, want a and c recorded, b absent", st)
	}
}

func TestBeforeTierRunsOncePerTierBeforeItsBuilds(t *testing.T) {
	f := newFixture(t, "a", "b")
	var events []string
	f.b.BeforeTier = func(ctx context.Context) error { events = append(events, "index"); return nil }
	inner := f.b.Build
	f.b.Build = func(ctx context.Context, name, src, out string) error {
		events = append(events, name)
		return inner(ctx, name, src, out)
	}
	if _, err := f.b.Run(context.Background(), [][]string{{"a"}, {"b"}}, map[string][]string{"b": {"a"}}); err != nil {
		t.Fatal(err)
	}
	if strings.Join(events, ",") != "index,a,index,b" {
		t.Errorf("events = %v", events)
	}
}

// Rebuilding must not delete and recreate the output directory: container
// engines that share host folders through a VM (Docker Desktop) keep a stale
// view of a recreated directory while its parent is mounted elsewhere.
func TestRebuildEmptiesOutputDirInPlace(t *testing.T) {
	f := newFixture(t, "a")
	if _, err := f.b.Run(context.Background(), [][]string{{"a"}}, nil); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(f.b.OutRoot, "a")
	before, err := os.Stat(out)
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(out, "stale_0_all.deb"), "old")
	f.b.KeySalt = "builder-v2"
	if _, err := f.b.Run(context.Background(), [][]string{{"a"}}, nil); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(out)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(before, after) {
		t.Error("output directory was recreated instead of emptied in place")
	}
	if _, err := os.Stat(filepath.Join(out, "stale_0_all.deb")); !os.IsNotExist(err) {
		t.Error("stale artifact survived the rebuild")
	}
}

// The pool index must not list a package that is about to be rebuilt: its old
// files are removed before the build, so an index taken earlier points at
// files that no longer exist (lintian-profile-vyatta installs itself).
func TestIndexRunsAfterRebuildingOutputsAreCleared(t *testing.T) {
	f := newFixture(t, "a")
	if _, err := f.b.Run(context.Background(), [][]string{{"a"}}, nil); err != nil {
		t.Fatal(err)
	}
	f.b.KeySalt = "builder-v2" // force a rebuild
	sawOld := false
	f.b.BeforeTier = func(ctx context.Context) error {
		sawOld = HasArtifacts(filepath.Join(f.b.OutRoot, "a"))
		return nil
	}
	if _, err := f.b.Run(context.Background(), [][]string{{"a"}}, nil); err != nil {
		t.Fatal(err)
	}
	if sawOld {
		t.Error("index hook ran while a rebuilding package's old artifacts were still in the pool")
	}
}

// Build inputs that live outside the source tree (an entry's Debian build
// profiles in the manifest) must be part of the key, or changing them keeps
// serving the old build.
func TestRunRebuildsWhenKeyExtraChanges(t *testing.T) {
	f := newFixture(t, "a")
	run := func() Status {
		rs, err := f.b.Run(context.Background(), [][]string{{"a"}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		return statuses(rs)["a"].Status
	}
	f.b.KeyExtra = map[string]string{"a": "profiles=pkg.a.small"}
	if s := run(); s != Built {
		t.Fatalf("first run = %s", s)
	}
	if s := run(); s != Cached {
		t.Fatalf("unchanged = %s", s)
	}
	f.b.KeyExtra = map[string]string{}
	if s := run(); s != Built {
		t.Fatalf("after profiles dropped = %s", s)
	}
}
