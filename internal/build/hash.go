package build

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// TreeHash is a sha256 over every non-.git file under dir: relative path, mode
// and contents (symlink target for links), in sorted path order.
func TreeHash(dir string) (string, error) {
	var paths []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		if !d.IsDir() {
			paths = append(paths, p)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(paths)
	h := sha256.New()
	for _, p := range paths {
		rel, _ := filepath.Rel(dir, p)
		info, err := os.Lstat(p)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(h, "%s\x00%o\x00", filepath.ToSlash(rel), info.Mode())
		if info.Mode()&fs.ModeSymlink != 0 {
			target, err := os.Readlink(p)
			if err != nil {
				return "", err
			}
			io.WriteString(h, target)
		} else {
			f, err := os.Open(p)
			if err != nil {
				return "", err
			}
			_, err = io.Copy(h, f)
			f.Close()
			if err != nil {
				return "", err
			}
		}
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
