package main

import (
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
