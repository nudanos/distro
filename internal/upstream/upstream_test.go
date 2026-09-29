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
		{"1:2.3.3-1", "2.4.3"}:              "1:2.4.3-0nudanos1",
		{"5.9.4+dfsg-2+deb13u1", "5.9.5.2"}: "5.9.5.2-0nudanos1",
		{"1.2.9.2-5", "1.2.9.2"}:            "1.2.9.2-0nudanos1",
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
