package fetch

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@e", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@e")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestGitClonesBranchThenFollowsUpdates(t *testing.T) {
	origin := t.TempDir()
	git(t, origin, "init", "-q", "-b", "master")
	os.WriteFile(filepath.Join(origin, "f"), []byte("master"), 0o644)
	git(t, origin, "add", "f")
	git(t, origin, "commit", "-qm", "m")
	git(t, origin, "tag", "danos/2105")
	git(t, origin, "checkout", "-qb", "trixie")
	os.WriteFile(filepath.Join(origin, "f"), []byte("trixie-1"), 0o644)
	git(t, origin, "commit", "-qam", "t1")

	dir := filepath.Join(t.TempDir(), "src", "pkg")
	if err := Git(context.Background(), origin, "trixie", dir, io.Discard); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "f")); string(b) != "trixie-1" {
		t.Fatalf("after clone f = %q", b)
	}

	os.WriteFile(filepath.Join(origin, "f"), []byte("trixie-2"), 0o644)
	git(t, origin, "commit", "-qam", "t2")
	os.WriteFile(filepath.Join(dir, "junk"), []byte("x"), 0o644) // untracked files are cleaned
	if err := Git(context.Background(), origin, "trixie", dir, io.Discard); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "f")); string(b) != "trixie-2" {
		t.Fatalf("after update f = %q", b)
	}
	if _, err := os.Stat(filepath.Join(dir, "junk")); !os.IsNotExist(err) {
		t.Error("untracked file survived update")
	}

	if err := Git(context.Background(), origin, "danos/2105", dir, io.Discard); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "f")); string(b) != "master" {
		t.Fatalf("at tag f = %q", b)
	}
}

func TestGitUnknownRefFails(t *testing.T) {
	origin := t.TempDir()
	git(t, origin, "init", "-q", "-b", "master")
	git(t, origin, "commit", "-q", "--allow-empty", "-m", "m")
	if err := Git(context.Background(), origin, "trixie", filepath.Join(t.TempDir(), "p"), io.Discard); err == nil {
		t.Error("want error for missing ref")
	}
}

// A manifest entry can move to another repository (mstpd's packaging moved
// from the DANOS fork to Debian's). The existing checkout must follow the new
// URL instead of fetching the old one.
func TestGitFollowsChangedRepository(t *testing.T) {
	repo := func(branch, content string) string {
		r := t.TempDir()
		git(t, r, "init", "-q", "-b", branch)
		os.WriteFile(filepath.Join(r, "f"), []byte(content), 0o644)
		git(t, r, "add", "f")
		git(t, r, "commit", "-qm", "c")
		return r
	}
	oldRepo, newRepo := repo("master", "fork"), repo("debian/latest", "debian")
	dir := filepath.Join(t.TempDir(), "packaging", "mstpd")
	if err := Git(context.Background(), oldRepo, "master", dir, io.Discard); err != nil {
		t.Fatal(err)
	}
	if err := Git(context.Background(), newRepo, "debian/latest", dir, io.Discard); err != nil {
		t.Fatalf("after the repository changed: %v", err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "f")); string(b) != "debian" {
		t.Errorf("f = %q, want the new repository's content", b)
	}
}

// Packaging is pinned to a commit so a push to the followed branch cannot
// change a prepared tree: checking out the pin stays put after the branch moves.
func TestGitChecksOutAPinnedCommit(t *testing.T) {
	origin := t.TempDir()
	git(t, origin, "init", "-q", "-b", "master")
	os.WriteFile(filepath.Join(origin, "f"), []byte("one"), 0o644)
	git(t, origin, "add", "f")
	git(t, origin, "commit", "-qm", "one")
	pin := git(t, origin, "rev-parse", "HEAD")
	os.WriteFile(filepath.Join(origin, "f"), []byte("two"), 0o644)
	git(t, origin, "commit", "-qam", "two")

	dir := filepath.Join(t.TempDir(), "p")
	for i := 0; i < 2; i++ { // fresh clone, then an existing checkout
		if err := Git(context.Background(), origin, pin, dir, io.Discard); err != nil {
			t.Fatal(err)
		}
		if b, _ := os.ReadFile(filepath.Join(dir, "f")); string(b) != "one" {
			t.Fatalf("run %d: f = %q, want the pinned commit's", i, b)
		}
	}
}
