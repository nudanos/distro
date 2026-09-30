package main

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

// Editing builder/ without re-running `builder` must not build with the old
// image under the new cache key.
func TestCheckBuilder(t *testing.T) {
	if err := checkBuilder("img", "h1", "h1"); err != nil {
		t.Errorf("matching hash: %v", err)
	}
	for _, label := range []string{"", "h0"} {
		err := checkBuilder("img", label, "h1")
		if err == nil || !strings.Contains(err.Error(), "distro-build builder") {
			t.Errorf("label %q: err = %v, want advice to run distro-build builder", label, err)
		}
	}
}

// Parallel builds interleave on stderr, so each build also gets its own log.
func TestPackageLogWritesPerPackageFile(t *testing.T) {
	dir := t.TempDir()
	w, err := packageLog(dir, "vyatta-cfg")
	if err != nil {
		t.Fatal(err)
	}
	w.Write([]byte("hello\n"))
	w.Close()
	b, err := os.ReadFile(dir + "/logs/vyatta-cfg.log")
	if err != nil || string(b) != "hello\n" {
		t.Errorf("log = %q, %v", b, err)
	}
}

func TestBuildEnvCarriesProfiles(t *testing.T) {
	got := buildEnv("vyatta-dataplane", 501, 20, 4, []string{"pkg.a", "pkg.b"})
	want := map[string]string{"PKG": "vyatta-dataplane", "HOST_UID": "501", "HOST_GID": "20", "JOBS": "4",
		"DEB_BUILD_PROFILES": "pkg.a pkg.b"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildEnv = %v, want %v", got, want)
	}
	if _, ok := buildEnv("x", 0, 0, 1, nil)["DEB_BUILD_PROFILES"]; ok {
		t.Error("no profiles must not set DEB_BUILD_PROFILES")
	}
}
