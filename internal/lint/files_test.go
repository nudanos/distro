package lint

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

// writeTree creates files (relative path -> content) under a temporary port.
func writeTree(t *testing.T, files map[string]string) string {
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

func classify(t *testing.T, files map[string]string) map[string][]string {
	t.Helper()
	dir := writeTree(t, files)
	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Classify(dir, cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range got {
		sort.Strings(l)
	}
	return got
}

// Most vyatta scripts have no extension: the shebang decides (Review Focus 1).
func TestClassifyByExtensionAndShebang(t *testing.T) {
	got := classify(t, map[string]string{
		"main.go":                     "package main\n",
		"lib/Vyatta/Mod.pm":           "package Vyatta::Mod;\n",
		"scripts/tool.pl":             "print 1;\n",
		"t/basic.t":                   "use Test::More;\n",
		"scripts/vyatta-show-version": "#!/usr/bin/perl\nuse strict;\n",
		"scripts/run.sh":              "echo hi\n",
		"scripts/vyatta-bash-thing":   "#!/bin/bash\necho hi\n",
		"scripts/vbash-thing":         "#!/bin/vbash\necho hi\n",
		"debian/postinst":             "#!/bin/sh\nset -e\n",
		"debian/rules":                "#!/usr/bin/make -f\n%:\n\tdh $@\n",
		"py/mod.py":                   "x = 1\n",
		"scripts/py-tool":             "#!/usr/bin/python3\nprint(1)\n",
		"src/a.c":                     "int a;\n",
		"src/a.h":                     "int a;\n",
		"src/b.cc":                    "int b;\n",
		"src/c.cpp":                   "int c;\n",
		"src/d.hpp":                   "int d;\n",
		"README":                      "text\n",
	})
	want := map[string][]string{
		"go":     {"main.go"},
		"perl":   {"lib/Vyatta/Mod.pm", "scripts/tool.pl", "scripts/vyatta-show-version", "t/basic.t"},
		"shell":  {"debian/postinst", "scripts/run.sh", "scripts/vbash-thing", "scripts/vyatta-bash-thing"},
		"python": {"py/mod.py", "scripts/py-tool"},
		"c":      {"src/a.c", "src/a.h", "src/b.cc", "src/c.cpp", "src/d.hpp"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %v\nwant %v", got, want)
	}
}

// Version control, vendored third-party code, patches and build staging
// under debian/ are never linted; debian/tests is.
func TestClassifySkipsByDefault(t *testing.T) {
	got := classify(t, map[string]string{
		".git/hooks/pre-commit":          "#!/bin/sh\n",
		"vendor/github.com/x/y.go":       "package y\n",
		"debian/patches/fix.c":           "int a;\n",
		"debian/tmp/usr/bin/x.sh":        "echo\n",
		"debian/vyatta-foo/usr/bin/x.sh": "echo\n",
		"debian/.debhelper/x.sh":         "echo\n",
		"debian/tests/smoke":             "#!/bin/sh\necho\n",
		"keep.sh":                        "echo\n",
	})
	want := map[string][]string{"shell": {"debian/tests/smoke", "keep.sh"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %v\nwant %v", got, want)
	}
}

func TestConfigSkipGlobs(t *testing.T) {
	got := classify(t, map[string]string{
		"nudanos-lint.yaml": "skip:\n  - \"generated/*\"\n",
		"generated/x.go":    "package g\n",
		"keep.go":           "package k\n",
	})
	if want := map[string][]string{"go": {"keep.go"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	dir := writeTree(t, map[string]string{"nudanos-lint.yaml": "rules:\n  - off\n"})
	if _, err := LoadConfig(dir); err == nil {
		t.Error("a key other than skip was accepted")
	}
}
