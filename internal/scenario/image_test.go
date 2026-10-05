package scenario

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReference2105RefusesWrongHash(t *testing.T) {
	iso := filepath.Join(t.TempDir(), "danos-2105-base-amd64.iso")
	os.WriteFile(iso, []byte("not the 2105 ISO"), 0o644)
	if _, err := Reference2105(iso); err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Errorf("err = %v, want a SHA-256 mismatch", err)
	}
}

func TestBaseConfig(t *testing.T) {
	want := []string{
		"set system host-name r1",
		"set interfaces dataplane dp0s10 address 10.0.2.15/24",
		"set service ssh",
	}
	if got := BaseConfig("R1"); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("BaseConfig(R1) = %q", got)
	}
}

// An installed base disk for this ISO already exists: no QEMU may run.
func TestEnsureBaseReusesExistingDisk(t *testing.T) {
	dir := t.TempDir()
	iso := filepath.Join(dir, "nudanos.iso")
	os.WriteFile(iso, []byte("an iso"), 0o644)
	sum := sha256.Sum256([]byte("an iso"))
	base := filepath.Join(dir, "base-"+hex.EncodeToString(sum[:])[:12]+".qcow2")
	os.WriteFile(base, []byte("qcow2"), 0o644)
	bin := t.TempDir()
	marker := filepath.Join(dir, "qemu-ran")
	os.WriteFile(filepath.Join(bin, "qemu-system-x86_64"), []byte("#!/bin/sh\ntouch "+marker+"\nexit 1\n"), 0o755)
	os.WriteFile(filepath.Join(bin, "qemu-img"), []byte("#!/bin/sh\ntouch "+marker+"\nexit 1\n"), 0o755)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	got, err := EnsureBase(NuDanOS(iso), dir, false, &bytes.Buffer{}, func(d time.Duration) time.Duration { return d })
	if err != nil || got != base {
		t.Fatalf("EnsureBase = %q, %v; want %q", got, err, base)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Error("EnsureBase ran QEMU although the base disk exists")
	}
}

func TestImageProfiles(t *testing.T) {
	n := NuDanOS("/iso/n.iso")
	if n.MemMB != 1024 || n.User != "nudanos" || n.Password != "NuDanOS-test-1" || n.Live {
		t.Errorf("NuDanOS profile = %+v", n)
	}
}
