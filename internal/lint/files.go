package lint

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	yaml "go.yaml.in/yaml/v3"
)

// ConfigFile is the per-port lint configuration, at the port's root.
const ConfigFile = "nudanos-lint.yaml"

// Config holds a port's only permitted lint exceptions: paths not to check.
type Config struct {
	// path.Match globs relative to the port root; an entry ending in "/"
	// skips that directory's whole tree
	Skip []string `yaml:"skip"`
}

// skipped reports whether cfg leaves rel (a path relative to the port root)
// out.
func (cfg Config) skipped(rel string, dir bool) bool {
	for _, g := range cfg.Skip {
		if tree, ok := strings.CutSuffix(g, "/"); ok {
			if rel == tree || strings.HasPrefix(rel, tree+"/") {
				return true
			}
			continue
		}
		if ok, _ := path.Match(g, rel); ok && !dir {
			return true
		}
	}
	return false
}

// LoadConfig reads portDir's nudanos-lint.yaml; a port without one gets the
// empty Config. Any key other than skip is an error.
func LoadConfig(portDir string) (Config, error) {
	var c Config
	b, err := os.ReadFile(filepath.Join(portDir, ConfigFile))
	if errors.Is(err, fs.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&c); err != nil && !errors.Is(err, io.EOF) {
		return c, err
	}
	for _, g := range c.Skip {
		if _, err := path.Match(g, ""); err != nil {
			return c, err
		}
	}
	return c, nil
}

var byExt = map[string]string{
	".go": "go",
	".pl": "perl", ".pm": "perl", ".t": "perl",
	".sh": "shell", ".bash": "shell",
	".py": "python",
	".c":  "c", ".h": "c", ".cc": "c", ".cpp": "c", ".cxx": "c", ".hpp": "c", ".hh": "c",
}

// byShebang classifies a file with no known extension by its first line.
func byShebang(first string) string {
	if !strings.HasPrefix(first, "#!") {
		return ""
	}
	interp := strings.Fields(strings.TrimPrefix(first, "#!"))
	if len(interp) == 0 {
		return ""
	}
	prog := path.Base(interp[0])
	if prog == "env" && len(interp) > 1 {
		prog = path.Base(interp[1])
	}
	switch {
	case strings.HasPrefix(prog, "perl"):
		return "perl"
	case strings.HasPrefix(prog, "python"):
		return "python"
	case prog == "sh" || prog == "bash" || prog == "dash" || prog == "vbash":
		return "shell"
	}
	return ""
}

func firstLine(p string) string {
	f, err := os.Open(p)
	if err != nil {
		return ""
	}
	defer f.Close()
	l, _ := bufio.NewReader(f).ReadString('\n')
	return strings.TrimRight(l, "\r\n")
}

// skipDir reports directories that are never linted: version control,
// vendored third-party code, and everything under debian/ except debian/tests
// (patches, build staging, debhelper state).
func skipDir(rel string) bool {
	switch {
	case rel == ".git", rel == "vendor", strings.HasSuffix(rel, "/vendor"):
		return true
	case strings.HasPrefix(rel, "debian/"):
		return rel != "debian/tests" && !strings.HasPrefix(rel, "debian/tests/")
	}
	return false
}

// Classify lists portDir's files to check, by language (go, perl, shell,
// python, c), as paths relative to portDir.
func Classify(portDir string, cfg Config) (map[string][]string, error) {
	out := map[string][]string{}
	err := filepath.WalkDir(portDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(portDir, p)
		if err != nil || rel == "." {
			return err
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if skipDir(rel) || cfg.skipped(rel, true) {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() || cfg.skipped(rel, false) {
			return nil
		}
		lang, ok := byExt[path.Ext(rel)]
		if !ok {
			lang = byShebang(firstLine(p))
		}
		if lang != "" {
			out[lang] = append(out[lang], rel)
		}
		return nil
	})
	return out, err
}
