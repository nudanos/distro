package topology

import (
	"bytes"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nudanos/distro/internal/boottest"
)

// fakeQEMU puts a qemu-system-x86_64 running script on PATH.
func fakeQEMU(t *testing.T, script string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "qemu-system-x86_64"), []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func spec(t *testing.T) VMSpec {
	t.Helper()
	return VMSpec{Name: "R1", MemMB: 1024, SerialSock: filepath.Join(t.TempDir(), "s.sock")}
}

// QEMU is the console server; the fake only sleeps, so the test plays the
// server and checks Start connects and hands back a working Console.
func TestStartConnectsConsole(t *testing.T) {
	fakeQEMU(t, "sleep 30")
	s := spec(t)
	l, err := net.Listen("unix", s.SerialSock)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	go func() {
		c, err := l.Accept()
		if err == nil {
			io.WriteString(c, "\r\nnode login: ")
			time.Sleep(2 * time.Second)
			c.Close()
		}
	}()
	vm, err := Start(s, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	defer vm.Stop(0)
	if _, err := vm.Console.Expect(boottest.LoginPrompt, time.Second); err != nil {
		t.Fatal(err)
	}
}

func TestStartFailsWhenQEMUExits(t *testing.T) {
	fakeQEMU(t, "echo 'could not open disk' >&2; exit 1")
	start := time.Now()
	_, err := Start(spec(t), &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "qemu exited") {
		t.Fatalf("err = %v, want 'qemu exited'", err)
	}
	if time.Since(start) > 5*time.Second {
		t.Errorf("Start took %s to notice QEMU had exited", time.Since(start))
	}
}
