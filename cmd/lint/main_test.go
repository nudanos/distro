package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// The arguments must produce the output formats internal/lint parses.
func TestToolArgs(t *testing.T) {
	files := []string{"a", "b"}
	cases := map[string][]string{
		"gofmt":      {"gofmt", "-l", "a", "b"},
		"perltidy":   {"perltidy", "--profile=/lint/perltidyrc", "-st", "-se", "a", "b"},
		"perlcritic": {"perlcritic", "--profile", "/lint/perlcriticrc", "--verbose", `%f:%l:%c:%p:%m\n`, "a", "b"},
		"shellcheck": {"shellcheck", "-f", "gcc", "-S", "style", "a", "b"},
		"flake8":     {"flake8", "a", "b"},
		"pylint":     {"pylint", "--rcfile", "/lint/pylintrc", "--output-format=parseable", "a", "b"},
		// --check-level=exhaustive: at the normal level cppcheck stops
		// following branches in long functions and says so
		// (normalCheckLevelMaxBranches); that file was not fully checked
		"cppcheck": {"cppcheck", "-q", "--enable=warning,style,performance,portability", "--check-level=exhaustive",
			"--suppress=missingIncludeSystem", "--inline-suppr", "--template={file}:{line}:{id}:{message}", "a", "b"},
	}
	for tool, want := range cases {
		if got := toolArgs(tool, files); !reflect.DeepEqual(got, want) {
			t.Errorf("%s:\n got %q\nwant %q", tool, got, want)
		}
	}
}

// Each tool's "findings present" status is not an error; anything else is
// (Review Focus 2).
func TestExitOK(t *testing.T) {
	cases := []struct {
		tool string
		code int
		ok   bool
	}{
		{"gofmt", 0, true}, {"gofmt", 2, false},
		{"perlcritic", 2, true}, {"perlcritic", 1, false},
		{"shellcheck", 1, true}, {"shellcheck", 2, false},
		{"flake8", 1, true}, {"flake8", 2, false},
		{"pylint", 4 | 16, true}, {"pylint", 1, false}, {"pylint", 32, false},
		{"cppcheck", 0, true}, {"cppcheck", 1, false},
	}
	for _, c := range cases {
		if got := exitOK(c.tool, c.code); got != c.ok {
			t.Errorf("exitOK(%s, %d) = %v, want %v", c.tool, c.code, got, c.ok)
		}
	}
}

// Large ports are checked in chunks so no command line gets too long.
func TestChunks(t *testing.T) {
	files := make([]string, 450)
	got := chunks(files, 200)
	if len(got) != 3 || len(got[0]) != 200 || len(got[2]) != 50 {
		t.Errorf("chunk sizes %d", len(got))
	}
}

func writePort(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for p, c := range files {
		full := filepath.Join(dir, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// A 3.0 (quilt) port is built with debian/patches applied: lint must check
// that tree, not the bare checkout.
func TestPreparedTreeAppliesQuiltSeries(t *testing.T) {
	dir := writePort(t, map[string]string{
		"debian/source/format":       "3.0 (quilt)\n",
		"debian/patches/series":      "# a comment\nfix.patch\n\nother.patch -p0\n",
		"script.sh":                  "#!/bin/sh\necho hello\n",
		"debian/patches/fix.patch":   "--- a/script.sh\n+++ b/script.sh\n@@ -1,2 +1,3 @@\n #!/bin/sh\n echo hello\n+echo $1\n",
		"debian/patches/other.patch": "--- other.txt\n+++ other.txt\n@@ -0,0 +1 @@\n+added\n",
	})
	tree, cleanup, err := preparedTree(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if tree == dir {
		t.Fatal("the checkout itself was returned")
	}
	if b, _ := os.ReadFile(filepath.Join(tree, "script.sh")); !strings.Contains(string(b), "echo $1") {
		t.Errorf("patched script.sh = %q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(tree, "other.txt")); string(b) != "added\n" {
		t.Errorf("-p0 patch: other.txt = %q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "script.sh")); strings.Contains(string(b), "echo $1") {
		t.Error("the checkout was modified")
	}
}

func TestPreparedTreeLeavesOtherPortsAlone(t *testing.T) {
	dir := writePort(t, map[string]string{"debian/source/format": "3.0 (native)\n", "a.sh": "echo\n"})
	tree, cleanup, err := preparedTree(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if tree != dir {
		t.Errorf("tree = %s, want the checkout", tree)
	}
}

// A patch that does not apply is an error, never a silently unpatched tree.
func TestPreparedTreeFailsOnABadPatch(t *testing.T) {
	dir := writePort(t, map[string]string{
		"debian/source/format":     "3.0 (quilt)\n",
		"debian/patches/series":    "bad.patch\n",
		"script.sh":                "echo hello\n",
		"debian/patches/bad.patch": "--- a/script.sh\n+++ b/script.sh\n@@ -1 +1 @@\n-echo goodbye\n+echo bye\n",
	})
	if _, cleanup, err := preparedTree(dir); err == nil {
		cleanup()
		t.Error("a patch that does not apply was accepted")
	}
}
