package lint

import (
	"reflect"
	"strings"
	"testing"
)

const flagsLine = "nudanos-lint-flags: wall-wextra\n"

var cOnly = map[string]bool{"c": true}

func TestFromBuildLogWarnings(t *testing.T) {
	log := flagsLine +
		"gcc -Wall -Wextra -c src/vci.c\n" +
		"src/vci.c:42:9: warning: unused variable 'n' [-Wunused-variable]\n" +
		"/build/pkg/src/vci.c:42:9: warning: unused variable 'n' [-Wunused-variable]\n" + // the same one, absolute
		"/usr/include/x86_64-linux-gnu/bits/stdio2.h:3:1: warning: ignoring return value [-Wunused-result]\n" +
		"lib/x.cc:7:3: warning: comparison of integer expressions of different signedness [-Wsign-compare]\n"
	got, err := FromBuildLog("libvci", cOnly, []byte(log))
	if err != nil {
		t.Fatal(err)
	}
	want := []Finding{
		{Port: "libvci", Tool: "cc", File: "src/vci.c", Line: 42, Rule: "-Wunused-variable", Message: "unused variable 'n'"},
		{Port: "libvci", Tool: "cc", File: "lib/x.cc", Line: 7, Rule: "-Wsign-compare", Message: "comparison of integer expressions of different signedness"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
}

func TestFromBuildLogStaticcheck(t *testing.T) {
	log := "nudanos-staticcheck: services/x.go:12:2: this value of err is never used (SA4006)\n" +
		"nudanos-staticcheck: done\n"
	got, err := FromBuildLog("vci", map[string]bool{"go": true}, []byte(log))
	if err != nil {
		t.Fatal(err)
	}
	want := []Finding{{Port: "vci", Tool: "staticcheck", File: "services/x.go", Line: 12, Rule: "SA4006", Message: "this value of err is never used"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
}

// A cached package's log can predate the flags: that is not "no warnings"
// (Review Focus 5).
func TestFromBuildLogStaleIsAnError(t *testing.T) {
	_, err := FromBuildLog("libvci", cOnly, []byte("gcc -c src/vci.c\n"))
	if err == nil || !strings.Contains(err.Error(), "rebuild libvci") {
		t.Errorf("err = %v", err)
	}
}

func TestFromBuildLogGoWithoutStaticcheckIsAFinding(t *testing.T) {
	got, err := FromBuildLog("vci", map[string]bool{"go": true}, []byte("go vet ./...\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Tool != "lint" || got[0].Rule != "staticcheck-not-run" {
		t.Errorf("got %+v", got)
	}
}

// A port with neither C nor Go needs nothing from its log.
func TestFromBuildLogPerlOnlyNeedsNoLog(t *testing.T) {
	got, err := FromBuildLog("vyatta-op", map[string]bool{"perl": true}, nil)
	if err != nil || len(got) != 0 {
		t.Errorf("got %v, %v", got, err)
	}
}

// A staticcheck that could not run or load the packages prints something
// other than findings before the "done" marker: that port was not checked
// (Review Focus 2), even though the marker is there.
func TestFromBuildLogStaticcheckFailureIsAnError(t *testing.T) {
	for _, line := range []string{
		"nudanos-staticcheck: /bin/sh: 1: staticcheck: not found",
		"nudanos-staticcheck: -: could not load package github.com/x/y: no Go files",
	} {
		_, err := FromBuildLog("vci", map[string]bool{"go": true}, []byte(line+"\nnudanos-staticcheck: done\n"))
		if err == nil || !strings.Contains(err.Error(), "vci") {
			t.Errorf("%q: err = %v", line, err)
		}
	}
}
