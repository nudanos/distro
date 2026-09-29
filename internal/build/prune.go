package build

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// Prune removes output directories under outRoot whose names are not in keep,
// so packages from entries that are no longer ready (or were renamed) cannot
// leak into later builds or the published repository. Plain files are left alone.
func Prune(outRoot string, keep map[string]bool, log io.Writer) error {
	entries, err := os.ReadDir(outRoot)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !e.IsDir() || keep[e.Name()] {
			continue
		}
		fmt.Fprintf(log, "==> prune %s (no longer a ready manifest entry)\n", e.Name())
		if err := os.RemoveAll(filepath.Join(outRoot, e.Name())); err != nil {
			return err
		}
	}
	return nil
}
