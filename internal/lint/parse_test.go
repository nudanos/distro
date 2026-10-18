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
	if _, err := Parse("pylint", sample(t, "pylint-fatal")); err == nil || !strings.Contains(err.Error(), "syntax") {
		t.Errorf("pylint: err = %v, want an error naming the file", err)
	}
	// cppcheck itself failing is an error
	if _, err := Parse("cppcheck", []byte("x.c:0:internalError:Internal error. Token::Match called with varid 0.\n")); err == nil {
		t.Error("cppcheck internalError was taken for a finding")
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

// pylint's duplicate-code message (R0801) runs over several lines: the
// files and line ranges ("==name:[a:b]") and the duplicated code itself.
// They belong to the finding, not to unrecognised output (a real block from
// vyatta-platform).
func TestParsePylintDuplicateCode(t *testing.T) {
	got, err := Parse("pylint", sample(t, "pylint-duplicate"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d findings, want the one R0801 block: %+v", len(got), got)
	}
	for _, f := range got {
		if f.Rule != "R0801" || f.Message != "Similar lines in 2 files" {
			t.Errorf("finding %+v", f)
		}
	}
}

// cppcheck's syntaxError and unknownMacro mostly mean it lacks the
// preprocessor configuration (macros it cannot expand), not broken code. The
// file was not analysed, so it is a finding that keeps the port from zero,
// not a run error that stops lint for the port (real output: cppcheck-error,
// and libnss-vrfdns's unknownMacro).
func TestParseCppcheckUnanalysedFileIsAFinding(t *testing.T) {
	out := string(sample(t, "cppcheck-error")) +
		"src/vrffiles-XXX.c:144:unknownMacro:There is an unknown macro here somewhere. Configuration is required. If CONCAT is a macro then please configure it.\n"
	got, err := Parse("cppcheck", []byte(out))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Rule != "syntaxError" || got[1].Rule != "unknownMacro" {
		t.Errorf("got %+v", got)
	}
}
