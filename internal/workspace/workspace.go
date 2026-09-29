// Package workspace checks properties of the build work directory.
package workspace

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// CaseSensitive reports whether dir's filesystem distinguishes names by case.
// It creates dir if missing.
func CaseSensitive(dir string) (bool, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false, err
	}
	probe, err := os.MkdirTemp(dir, ".casecheck-")
	if err != nil {
		return false, err
	}
	defer os.RemoveAll(probe)
	if err := os.WriteFile(filepath.Join(probe, "a"), nil, 0o644); err != nil {
		return false, err
	}
	_, err = os.Stat(filepath.Join(probe, "A"))
	switch {
	case err == nil:
		return false, nil
	case errors.Is(err, fs.ErrNotExist):
		return true, nil
	default:
		return false, err
	}
}
