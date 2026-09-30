package upstream

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nudanos/distro/internal/build"
	"github.com/nudanos/distro/internal/manifest"
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@e", "GIT_COMMITTER_NAME=t",
		"GIT_COMMITTER_EMAIL=t@e", "GIT_COMMITTER_DATE=2026-03-30T07:55:19+00:00", "GIT_AUTHOR_DATE=2026-03-30T07:55:19+00:00")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func write(t *testing.T, path, body string) {
	t.Helper()
	os.MkdirAll(filepath.Dir(path), 0o755)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// fixture: an upstream repo tagged v2.4.3 (which ships its own stale debian/),
// a packaging repo whose changelog carries epoch 1, and a DANOS patch.
func fixture(t *testing.T) (manifest.Entry, string, string) {
	root := t.TempDir()
	up := filepath.Join(root, "up")
	os.MkdirAll(up, 0o755)
	git(t, up, "init", "-q", "-b", "master")
	write(t, filepath.Join(up, "configure.ac"), "AC_INIT([keepalived], [2.4.3])\n")
	write(t, filepath.Join(up, "debian", "control"), "Source: stale\n")
	git(t, up, "add", ".")
	git(t, up, "commit", "-qm", "release")
	git(t, up, "tag", "v2.4.3")

	pk := filepath.Join(root, "pkg")
	os.MkdirAll(pk, 0o755)
	git(t, pk, "init", "-q", "-b", "master")
	write(t, filepath.Join(pk, "debian", "changelog"), "keepalived (1:2.3.3-1) unstable; urgency=medium\n\n  * Upload.\n\n -- Debian <d@d>  Mon, 01 Jan 2024 00:00:00 +0000\n")
	write(t, filepath.Join(pk, "debian", "control"), "Source: keepalived\n")
	write(t, filepath.Join(pk, "debian", "source", "format"), "3.0 (quilt)\n")
	write(t, filepath.Join(pk, "debian", "patches", "series"), "debian-fix.patch\n")
	write(t, filepath.Join(pk, "debian", "patches", "debian-fix.patch"), "--- a\n+++ b\n")
	write(t, filepath.Join(pk, "README.packaging"), "not copied\n")
	git(t, pk, "add", ".")
	git(t, pk, "commit", "-qm", "packaging")

	patches := filepath.Join(root, "patches")
	write(t, filepath.Join(patches, "keepalived", "0001-vyatta-vrrp-hook.patch"), "--- a\n+++ b\n")

	e := manifest.Entry{Name: "keepalived", Kind: manifest.Upstream, Upstream: up, Tag: "v2.4.3", Version: "2.4.3",
		TagPattern: `^v(\d+\.\d+\.\d+)$`, Packaging: pk, PackagingRef: "master", Track: "latest"}
	return e, patches, filepath.Join(root, "work")
}

func TestPrepare(t *testing.T) {
	e, patches, work := fixture(t)
	dir, err := Prepare(context.Background(), e, patches, work, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "configure.ac")); !strings.Contains(string(b), "2.4.3") {
		t.Error("upstream source missing")
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "debian", "control")); string(b) != "Source: keepalived\n" {
		t.Errorf("debian/control = %q, want packaging's (upstream's debian/ replaced)", b)
	}
	if _, err := os.Stat(filepath.Join(dir, "README.packaging")); !os.IsNotExist(err) {
		t.Error("files outside packaging's debian/ were copied")
	}
	series, _ := os.ReadFile(filepath.Join(dir, "debian", "patches", "series"))
	if string(series) != "debian-fix.patch\n0001-vyatta-vrrp-hook.patch\n" {
		t.Errorf("series = %q", series)
	}
	cl, _ := os.ReadFile(filepath.Join(dir, "debian", "changelog"))
	first := strings.SplitN(string(cl), "\n", 2)[0]
	if first != "keepalived (1:2.4.3-0nudanos1) trixie; urgency=medium" {
		t.Errorf("changelog top = %q (epoch must be kept)", first)
	}
	if !strings.Contains(string(cl), " -- NuDanOS Maintainers <jon@fernandez.tech>  Mon, 30 Mar 2026 07:55:19 +0000") {
		t.Errorf("changelog trailer must carry the tag's commit date:\n%s", cl)
	}
	if !strings.Contains(string(cl), "keepalived (1:2.3.3-1) unstable") {
		t.Error("previous changelog entries lost")
	}
}

func TestPrepareIsReproducible(t *testing.T) {
	e, patches, work := fixture(t)
	d1, err := Prepare(context.Background(), e, patches, work, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	h1, _ := build.TreeHash(d1)
	d2, err := Prepare(context.Background(), e, patches, work, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if h2, _ := build.TreeHash(d2); h2 != h1 {
		t.Error("preparing the same tag twice produced different trees")
	}
}

func TestDebianVersion(t *testing.T) {
	cases := map[[2]string]string{
		{"1:2.3.3-1", "2.4.3"}: "1:2.4.3-0nudanos1",
		// Debian repacks (Files-Excluded) and so do we: carry the suffix.
		{"5.9.4+dfsg-2+deb13u1", "5.9.5.2"}: "5.9.5.2+dfsg-0nudanos1",
		// Debian already packages this upstream version: sort after its
		// revision, not before it (ntpsec 1.2.5+dfsg-1).
		{"1.2.5+dfsg-1", "1.2.5"}: "1.2.5+dfsg-1nudanos1",
		{"1.2.9.2-5", "1.2.9.2"}:  "1.2.9.2-5nudanos1",
	}
	for in, want := range cases {
		if got := DebianVersion(in[0], in[1]); got != want {
			t.Errorf("DebianVersion(%q, %q) = %q, want %q", in[0], in[1], got, want)
		}
	}
}

// Debian repacks some upstream releases: DEP-5 Files-Excluded names what
// uscan strips. Git tags can also carry files a release tarball leaves out
// (keepalived's www/, with a non-free RFC draft), listed in the entry's Exclude.
func TestPrepareDropsExcludedFiles(t *testing.T) {
	e, patches, work := fixture(t)
	up := e.Upstream
	write(t, filepath.Join(up, "doc", "manual.pdf"), "x")
	write(t, filepath.Join(up, "mibs", "HOST-RESOURCES-MIB.txt"), "x")
	write(t, filepath.Join(up, "mibs", "NET-SNMP-MIB.txt"), "keep")
	write(t, filepath.Join(up, "www", "docs", "draft.txt"), "x")
	git(t, up, "add", ".")
	git(t, up, "commit", "-qm", "more")
	git(t, up, "tag", "-f", "v2.4.3")
	pk := e.Packaging
	write(t, filepath.Join(pk, "debian", "copyright"),
		"Format: https://www.debian.org/doc/packaging-manuals/copyright-format/1.0/\nFiles-Excluded: doc\n mibs/H*.txt\n\nFiles: *\nLicense: BSD\n")
	git(t, pk, "add", ".")
	git(t, pk, "commit", "-qm", "dep5")
	e.Exclude = []string{"www"}
	dir, err := Prepare(context.Background(), e, patches, work, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	for _, gone := range []string{"doc", "mibs/HOST-RESOURCES-MIB.txt", "www"} {
		if _, err := os.Stat(filepath.Join(dir, gone)); !os.IsNotExist(err) {
			t.Errorf("%s should have been excluded", gone)
		}
	}
	for _, kept := range []string{"mibs/NET-SNMP-MIB.txt", "configure.ac", "debian/copyright"} {
		if _, err := os.Stat(filepath.Join(dir, kept)); err != nil {
			t.Errorf("%s should remain: %v", kept, err)
		}
	}
}

func TestChangelogDate(t *testing.T) {
	const tag = "Fri, 31 Jul 2026 00:27:11 -0400"
	// Tag newer than the packaging's last entry: use the tag's date.
	if got, err := changelogDate(tag, "Mon, 01 Jun 2026 10:00:00 +0000"); err != nil || got != tag {
		t.Errorf("changelogDate = %q, %v; want %q", got, err, tag)
	}
	// Debian packaged the release after its tag (ntpsec): the new entry must
	// still be dated after the previous one (lintian
	// latest-changelog-entry-without-new-date).
	want := "Sat, 01 Aug 2026 12:00:01 +0000"
	if got, err := changelogDate(tag, "Sat, 01 Aug 2026 12:00:00 +0000"); err != nil || got != want {
		t.Errorf("changelogDate = %q, %v; want %q", got, err, want)
	}
	// git's %cD does not zero-pad the day (libteam v1.32).
	const short = "Tue, 5 Sep 2023 16:29:17 +0200"
	if got, err := changelogDate(short, "Mon, 01 Jan 2024 00:00:00 +0000"); err != nil || got != "Mon, 01 Jan 2024 00:00:01 +0000" {
		t.Errorf("changelogDate(%q) = %q, %v", short, got, err)
	}
}

// perfSONAR releases keep the source two levels down (owamp/owamp/) and ship
// their own Debian packaging inside it (unibuild-packaging/deb).
func TestPrepareSubdirAndInTreePackaging(t *testing.T) {
	root := t.TempDir()
	up := filepath.Join(root, "up")
	os.MkdirAll(up, 0o755)
	git(t, up, "init", "-q", "-b", "master")
	write(t, filepath.Join(up, "Makefile"), "# unibuild top level\n")
	write(t, filepath.Join(up, "owamp", "owamp", "configure.ac"), "AC_INIT([owamp], [5.2.6])\n")
	deb := filepath.Join(up, "owamp", "owamp", "unibuild-packaging", "deb")
	write(t, filepath.Join(deb, "changelog"), "owamp (5.2.6-1) perfsonar-5.2; urgency=low\n\n  * New upstream version.\n\n -- perfSONAR <d@p>  Mon, 01 Jan 2024 00:00:00 +0000\n")
	write(t, filepath.Join(deb, "control"), "Source: owamp\n")
	write(t, filepath.Join(deb, "source", "format"), "3.0 (quilt)\n")
	git(t, up, "add", ".")
	git(t, up, "commit", "-qm", "release")
	git(t, up, "tag", "v5.2.6")

	e := manifest.Entry{Name: "owamp", Kind: manifest.Upstream, Upstream: up, Tag: "v5.2.6", Version: "5.2.6",
		TagPattern: `^v(\d+\.\d+\.\d+)$`, Track: "latest",
		Subdir: "owamp/owamp", PackagingDir: "owamp/owamp/unibuild-packaging/deb"}
	dir, err := Prepare(context.Background(), e, filepath.Join(root, "patches"), filepath.Join(root, "work"), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "configure.ac")); err != nil {
		t.Error("subdir is not the source root")
	}
	if _, err := os.Stat(filepath.Join(dir, "Makefile")); !os.IsNotExist(err) {
		t.Error("files above subdir were copied")
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "debian", "control")); string(b) != "Source: owamp\n" {
		t.Errorf("debian/control = %q, want the in-tree packaging's", b)
	}
	cl, _ := os.ReadFile(filepath.Join(dir, "debian", "changelog"))
	if first := strings.SplitN(string(cl), "\n", 2)[0]; first != "owamp (5.2.6-1nudanos1) trixie; urgency=medium" {
		t.Errorf("changelog top = %q", first)
	}
}

// Packaging we do not maintain (salsa, perfSONAR) is changed only through
// patches/<name>/debian/*.patch, applied to the assembled tree. They are not
// quilt patches: quilt patches may not touch debian/.
func TestPreparePackagingPatches(t *testing.T) {
	e, patches, work := fixture(t)
	write(t, filepath.Join(patches, "keepalived", "debian", "0001-control.patch"),
		"--- a/debian/control\n+++ b/debian/control\n@@ -1 +1,2 @@\n Source: keepalived\n+Section: net\n")
	dir, err := Prepare(context.Background(), e, patches, work, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "debian", "control")); string(b) != "Source: keepalived\nSection: net\n" {
		t.Errorf("debian/control = %q, want the packaging patch applied", b)
	}
	series, _ := os.ReadFile(filepath.Join(dir, "debian", "patches", "series"))
	if strings.Contains(string(series), "0001-control.patch") {
		t.Errorf("packaging patch added to the quilt series: %q", series)
	}
}

// distro-build passes -patches as a relative path, and git apply runs inside
// the assembled tree: the patch path must not depend on the working directory.
func TestPreparePackagingPatchesRelativeDir(t *testing.T) {
	e, patches, work := fixture(t)
	write(t, filepath.Join(patches, "keepalived", "debian", "0001-control.patch"),
		"--- a/debian/control\n+++ b/debian/control\n@@ -1 +1,2 @@\n Source: keepalived\n+Section: net\n")
	t.Chdir(filepath.Dir(patches))
	dir, err := Prepare(context.Background(), e, filepath.Base(patches), work, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "debian", "control")); string(b) != "Source: keepalived\nSection: net\n" {
		t.Errorf("debian/control = %q, want the packaging patch applied", b)
	}
}

func TestPreparePackagingPatchMustApply(t *testing.T) {
	e, patches, work := fixture(t)
	write(t, filepath.Join(patches, "keepalived", "debian", "0001-stale.patch"),
		"--- a/debian/control\n+++ b/debian/control\n@@ -1 +1 @@\n-Source: something-else\n+Source: x\n")
	if _, err := Prepare(context.Background(), e, patches, work, io.Discard); err == nil {
		t.Error("a packaging patch that does not apply was accepted")
	}
}

// ntpsec ships waf as a self-extracting blob; Debian unpacks it into source
// (debian/repack-waf) and lintian rejects the blob (source-contains-waf-binary).
func TestUnpackWaf(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "waf"), "#!/usr/bin/env python3\nimport os\nos.makedirs('.waf3-2.1.4-abc/waflib', exist_ok=True)\nopen('.waf3-2.1.4-abc/waflib/__init__.py', 'w').write('# waflib\\n')\n#==>\n#BZh91AY&SY binary tail\n")
	if err := unpackWaf(context.Background(), dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "waflib", "__init__.py")); err != nil {
		t.Error("waflib not unpacked into the tree")
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "waf")); strings.Contains(string(b), "#==>") || strings.Contains(string(b), "binary tail") {
		t.Errorf("waf still carries its blob:\n%s", b)
	}
	if m, _ := filepath.Glob(filepath.Join(dir, ".waf3-*")); len(m) != 0 {
		t.Errorf("extraction directory left behind: %v", m)
	}
}
