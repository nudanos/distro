package build

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Output of entries that are no longer ready would otherwise stay in the pool,
// win at pin priority in later builds, and be published.
func TestPruneRemovesOutputOfEntriesNoLongerReady(t *testing.T) {
	out := t.TempDir()
	for _, n := range []string{"keep", "frr", "renamed-away"} {
		write(t, filepath.Join(out, n, n+"_1_all.deb"), n)
	}
	write(t, filepath.Join(out, "stray-file"), "x")
	var log bytes.Buffer
	if err := Prune(out, map[string]bool{"keep": true, "frr": true}, &log); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"keep", "frr", "stray-file"} {
		if _, err := os.Stat(filepath.Join(out, n)); err != nil {
			t.Errorf("%s was removed: %v", n, err)
		}
	}
	if _, err := os.Stat(filepath.Join(out, "renamed-away")); !os.IsNotExist(err) {
		t.Error("renamed-away survived")
	}
	if !strings.Contains(log.String(), "renamed-away") {
		t.Errorf("log = %q, want it to name the pruned entry", log.String())
	}
}

func TestPruneMissingOutRootIsFine(t *testing.T) {
	if err := Prune(filepath.Join(t.TempDir(), "absent"), nil, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
}
