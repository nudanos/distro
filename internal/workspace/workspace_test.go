package workspace

import (
	"runtime"
	"testing"
)

func TestCaseSensitiveOnLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux filesystems are case-sensitive; macOS default volumes are not")
	}
	ok, err := CaseSensitive(t.TempDir())
	if err != nil || !ok {
		t.Fatalf("CaseSensitive = %v, %v; want true, nil", ok, err)
	}
}

func TestCaseSensitiveCreatesMissingDir(t *testing.T) {
	dir := t.TempDir() + "/new/work"
	if _, err := CaseSensitive(dir); err != nil {
		t.Fatal(err)
	}
}
