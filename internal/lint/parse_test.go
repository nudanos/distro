package lint

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func sample(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "out", name+".txt"))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// The samples are real output of the trixie tools, run with the arguments
// cmd/lint passes them.
func TestParseEachTool(t *testing.T) {
	cases := map[string][]Finding{
		"gofmt":    {{Tool: "gofmt", File: "bad.go", Rule: "gofmt", Message: "not gofmt-formatted"}},
		"perltidy": {{Tool: "perltidy", File: "bad.pl", Rule: "perltidy", Message: "not perltidy-formatted"}},
		"perlcritic": {
			{Tool: "perlcritic", File: "bad.pl", Line: 5, Rule: "InputOutput::ProhibitBarewordFileHandles", Message: "Bareword file handle opened"},
			{Tool: "perlcritic", File: "bad.pl", Line: 5, Rule: "InputOutput::RequireBriefOpen", Message: "Close filehandles as soon as possible after opening them"},
			{Tool: "perlcritic", File: "bad.pl", Line: 5, Rule: "InputOutput::RequireCheckedOpen", Message: `Return value of "open" ignored`},
		},
		"shellcheck": {{Tool: "shellcheck", File: "bad.sh", Line: 2, Rule: "SC2086", Message: "Double quote to prevent globbing and word splitting."}},
		"flake8": {
			{Tool: "flake8", File: "bad.py", Line: 1, Rule: "F401", Message: "'os' imported but unused"},
			{Tool: "flake8", File: "bad.py", Line: 2, Rule: "E225", Message: "missing whitespace around operator"},
		},
		"pylint": {
			{Tool: "pylint", File: "bad.py", Line: 2, Rule: "C0103", Message: `Constant name "x" doesn't conform to UPPER_CASE naming style`},
			{Tool: "pylint", File: "bad.py", Line: 1, Rule: "W0611", Message: "Unused import os"},
		},
		"cppcheck": {
			{Tool: "cppcheck", File: "bad.c", Line: 1, Rule: "arrayIndexOutOfBounds", Message: "Array 'a[2]' accessed at index 2, which is out of bounds."},
			{Tool: "cppcheck", File: "bad.c", Line: 1, Rule: "constVariable", Message: "Variable 'a' can be declared as const array"},
			{Tool: "cppcheck", File: "bad.c", Line: 1, Rule: "uninitvar", Message: "Uninitialized variable: a"},
			{Tool: "cppcheck", File: "bad.c", Line: 1, Rule: "unassignedVariable", Message: "Variable 'a' is not assigned a value."},
		},
	}
	for tool, want := range cases {
		got, err := Parse(tool, sample(t, tool))
		if err != nil {
			t.Errorf("%s: %v", tool, err)
			continue
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s:\n got %+v\nwant %+v", tool, got, want)
		}
	}
}

// A tool that could not read a file has not checked it: that is an error,
// never "no findings" (Review Focus 2).
func TestParseToolErrorIsAnError(t *testing.T) {
	for tool, sampleName := range map[string]string{"pylint": "pylint-fatal", "cppcheck": "cppcheck-error"} {
		_, err := Parse(tool, sample(t, sampleName))
		if err == nil || !strings.Contains(err.Error(), "syntax") {
			t.Errorf("%s: err = %v, want an error naming the file", tool, err)
		}
	}
	// flake8 reports a file it could not parse as E999
	if _, err := Parse("flake8", []byte("syntax.py:1:8: E999 SyntaxError: invalid syntax\n")); err == nil {
		t.Error("flake8 E999 was taken for a finding")
	}
	// output in no known form is an error too, not silently dropped
	if _, err := Parse("shellcheck", []byte("shellcheck: bad.sh: openBinaryFile: does not exist\n")); err == nil {
		t.Error("unrecognised shellcheck output was ignored")
	}
}

func TestParseUnknownTool(t *testing.T) {
	if _, err := Parse("jslint", nil); err == nil {
		t.Error("unknown tool accepted")
	}
}

func TestFindingString(t *testing.T) {
	f := Finding{Port: "aaa", Tool: "shellcheck", File: "x.sh", Line: 2, Rule: "SC2086", Message: "quote"}
	if got, want := f.String(), "aaa: x.sh:2: shellcheck SC2086: quote"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
