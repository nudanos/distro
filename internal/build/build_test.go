package build

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
			f.built = append(f.built, name)
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
