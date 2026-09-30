// Package upstream prepares source trees for packages built from an upstream
// release with Debian's (or our) packaging on top.
package upstream

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/nudanos/distro/internal/control"
	"github.com/nudanos/distro/internal/fetch"
	"github.com/nudanos/distro/internal/manifest"
)

const maintainer = "NuDanOS Maintainers <jon@fernandez.tech>"

// repackSuffix matches the marker Debian adds to a repacked upstream version.
var repackSuffix = regexp.MustCompile(`\+(dfsg|ds)\d*`)

// DebianVersion is the version for an upstream build from prevTop, the
// packaging's latest version: its epoch (if any), the upstream version with
// the packaging's repack suffix (we apply the same Files-Excluded), and
// revision 0nudanos1 -- or, when the packaging already has this upstream
// version, its own revision plus nudanos1, so ours sorts after it.
func DebianVersion(prevTop, upstreamVersion string) string {
	epoch, rest := "", prevTop
	if i := strings.Index(rest, ":"); i > 0 {
		epoch, rest = rest[:i+1], rest[i+1:]
	}
	prevUp, prevRev := rest, ""
	if i := strings.LastIndex(rest, "-"); i > 0 {
		prevUp, prevRev = rest[:i], rest[i+1:]
	}
	up := upstreamVersion
	if s := repackSuffix.FindString(prevUp); s != "" && !strings.Contains(up, s) {
		up += s
	}
	if up == prevUp && prevRev != "" {
		return epoch + up + "-" + prevRev + "nudanos1"
	}
	return epoch + up + "-0nudanos1"
}

// rfc2822 parses RFC 2822 dates with or without a zero-padded day (git's %cD
// writes "Tue, 5 Sep 2023").
const rfc2822 = "Mon, 2 Jan 2006 15:04:05 -0700"

// changelogDate dates the new entry: the tag's commit date, unless the
// packaging's latest entry is newer (Debian packaged the release after it was
// tagged), then one second after that entry. Both are RFC 2822 dates.
func changelogDate(tagDate, prevDate string) (string, error) {
	tag, err := time.Parse(rfc2822, tagDate)
	if err != nil {
		return "", fmt.Errorf("tag date %q: %w", tagDate, err)
	}
	prev, err := time.Parse(rfc2822, prevDate)
	if err != nil {
		return "", fmt.Errorf("changelog date %q: %w", prevDate, err)
	}
	if tag.After(prev) {
		return tagDate, nil
	}
	return prev.Add(time.Second).Format(time.RFC1123Z), nil
}

// Prepare checks out e.Upstream at e.Tag and e.Packaging at e.PackagingRef, and
// assembles work/src/<name>: the upstream tree (from e.Subdir, minus .git and
// any debian/), the packaging's debian/ (or e.PackagingDir of the upstream
// tree, for upstreams that ship their own), our patches from patchesDir/<name> appended to the
// quilt series, and a changelog entry dated with the tag's commit date so the
// result is reproducible.
func Prepare(ctx context.Context, e manifest.Entry, patchesDir, work string, log io.Writer) (string, error) {
	upDir := filepath.Join(work, "upstream", e.Name)
	pkDir := filepath.Join(work, "packaging", e.Name)
	if err := fetch.Git(ctx, e.Upstream, e.Tag, upDir, log); err != nil {
		return "", err
	}
	debSrc := filepath.Join(upDir, filepath.FromSlash(e.PackagingDir))
	if e.PackagingDir == "" {
		if err := fetch.Git(ctx, e.Packaging, e.PackagingRef, pkDir, log); err != nil {
			return "", err
		}
		debSrc = filepath.Join(pkDir, "debian")
	}
	dir := filepath.Join(work, "src", e.Name)
	if err := os.RemoveAll(dir); err != nil {
		return "", err
	}
	if err := copyTree(filepath.Join(upDir, filepath.FromSlash(e.Subdir)), dir, func(rel string) bool {
		return rel == ".git" || rel == "debian" || strings.HasPrefix(rel, ".git/") || strings.HasPrefix(rel, "debian/")
	}); err != nil {
		return "", err
	}
	if err := copyTree(debSrc, filepath.Join(dir, "debian"), func(string) bool { return false }); err != nil {
		return "", fmt.Errorf("%s: no packaging at %s: %w", e.Name, debSrc, err)
	}
	if err := dropExcluded(dir, append(filesExcluded(filepath.Join(dir, "debian", "copyright")), e.Exclude...)); err != nil {
		return "", err
	}
	if err := addPatches(filepath.Join(patchesDir, e.Name), filepath.Join(dir, "debian", "patches")); err != nil {
		return "", err
	}
	date, err := gitOutput(ctx, upDir, "log", "-1", "--format=%cD", e.Tag)
	if err != nil {
		return "", err
	}
	if err := bumpChangelog(filepath.Join(dir, "debian", "changelog"), e, date); err != nil {
		return "", err
	}
	return dir, nil
}

func gitOutput(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %v in %s: %w", args, dir, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// copyTree copies src to dst, skipping paths (relative to src) where skip is true.
func copyTree(src, dst string, skip func(rel string) bool) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		rel = filepath.ToSlash(rel)
		if rel != "." && skip(rel) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(dst, rel)
		info, err := os.Lstat(p)
		if err != nil {
			return err
		}
		switch {
		case d.IsDir():
			return os.MkdirAll(target, 0o755)
		case info.Mode()&fs.ModeSymlink != 0:
			link, err := os.Readlink(p)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		default:
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			return os.WriteFile(target, b, info.Mode().Perm())
		}
	})
}

// addPatches copies from/*.patch (sorted) into to/ and appends them to to/series.
func addPatches(from, to string) error {
	names, err := filepath.Glob(filepath.Join(from, "*.patch"))
	if err != nil || len(names) == 0 {
		return err
	}
	if err := os.MkdirAll(to, 0o755); err != nil {
		return err
	}
	series, _ := os.ReadFile(filepath.Join(to, "series"))
	if len(series) > 0 && !bytes.HasSuffix(series, []byte("\n")) {
		series = append(series, '\n')
	}
	for _, n := range names {
		b, err := os.ReadFile(n)
		if err != nil {
			return err
		}
		base := filepath.Base(n)
		if err := os.WriteFile(filepath.Join(to, base), b, 0o644); err != nil {
			return err
		}
		series = append(series, []byte(base+"\n")...)
	}
	return os.WriteFile(filepath.Join(to, "series"), series, 0o644)
}

// bumpChangelog prepends an entry for e.Version, keeping the packaging's epoch.
// trailerDate is the date on the first (latest) changelog trailer line.
var trailerDate = regexp.MustCompile(`(?m)^ -- .*?  (.+)$`)

func bumpChangelog(path string, e manifest.Entry, date string) error {
	old, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	first := strings.SplitN(string(old), "\n", 2)[0]
	fields := strings.Fields(first)
	if len(fields) < 2 {
		return fmt.Errorf("%s: unparseable first line %q", path, first)
	}
	src, prev := fields[0], strings.Trim(fields[1], "()")
	if m := trailerDate.FindStringSubmatch(string(old)); m != nil {
		if date, err = changelogDate(date, m[1]); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	}
	entry := fmt.Sprintf("%s (%s) trixie; urgency=medium\n\n  * New upstream release %s (tag %s), built by NuDanOS.\n\n -- %s  %s\n\n",
		src, DebianVersion(prev, e.Version), e.Version, e.Tag, maintainer, date)
	return os.WriteFile(path, append([]byte(entry), old...), 0o644)
}

// filesExcluded returns the DEP-5 Files-Excluded globs of a copyright file
// (none if the file is missing or not machine-readable).
func filesExcluded(copyright string) []string {
	b, err := os.ReadFile(copyright)
	if err != nil {
		return nil
	}
	ps := control.Parse(string(b))
	if len(ps) == 0 {
		return nil
	}
	return strings.Fields(ps[0]["Files-Excluded"])
}

// dropExcluded removes paths under dir (outside debian/) matching any glob.
// A glob names a file or a whole directory; '*' does not cross '/'.
func dropExcluded(dir string, globs []string) error {
	if len(globs) == 0 {
		return nil
	}
	var doomed []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		rel = filepath.ToSlash(rel)
		if rel == "." || rel == "debian" || strings.HasPrefix(rel, "debian/") {
			if rel == "debian" {
				return filepath.SkipDir
			}
			return nil
		}
		for _, g := range globs {
			if ok, _ := filepath.Match(strings.TrimSuffix(g, "/"), rel); ok {
				doomed = append(doomed, p)
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	for _, p := range doomed {
		if err := os.RemoveAll(p); err != nil {
			return err
		}
	}
	return nil
}
