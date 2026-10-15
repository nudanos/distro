package main

import (
	"reflect"
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
		"cppcheck": {"cppcheck", "-q", "--enable=warning,style,performance,portability",
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
