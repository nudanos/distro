package boottest

import (
	"bytes"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

// A router accepts logins before its boot configuration is committed
// (system-configure runs after getty). Configuring then raced the boot
// commit: "Commit already in progress", and a configd session opened
// mid-boot-commit never got the lock (mpls-ldp, two hours). The boot
// commit also ends the admin's console session on NuDanOS. The runner
// waits for system-configure, logging in again when the session ends.
func bootConsole(t *testing.T, answers []string) (*Console, *int) {
	t.Helper()
	WaitInterval = 10 * time.Millisecond
	a, vm := net.Pipe()
	t.Cleanup(func() { a.Close(); vm.Close() })
	asked := 0
	go func() {
		b := make([]byte, 256)
		for _, answer := range answers {
			n, err := vm.Read(b)
			if err != nil {
				return
			}
			asked++
			io.WriteString(vm, string(b[:n])+"\n"+strings.ReplaceAll(answer, "\n", "\r\n"))
		}
	}()
	return NewConsole(a, &bytes.Buffer{}), &asked
}

func TestWaitBootedUntilSystemConfigureDone(t *testing.T) {
	// systemctl ends its answer with a terminal escape on the console
	c, asked := bootConsole(t, []string{"activating\x1b[m\nvyatta@r1:~$ ", "activating\nvyatta@r1:~$ ", "inactive\x1b[m\nvyatta@r1:~$ "})
	if err := WaitBooted(c, "vyatta", "pw", 5*time.Second); err != nil {
		t.Fatal(err)
	}
	if *asked != 3 {
		t.Errorf("asked %d times, want 3 (until system-configure finished)", *asked)
	}
}

func TestWaitBootedLogsInAgainWhenSessionEnds(t *testing.T) {
	// the boot commit ends the session: the next command lands at a login prompt
	c, asked := bootConsole(t, []string{
		"\nnode login: ",          // the command was typed after the session ended
		"\nnode login: ",          // Login nudges for a fresh prompt
		"Password: ",              // user name
		"\nvyatta@r1:~$ ",         // password
		"inactive\nvyatta@r1:~$ ", // asked again in the new session
	})
	if err := WaitBooted(c, "vyatta", "pw", 5*time.Second); err != nil {
		t.Fatal(err)
	}
	if *asked < 4 {
		t.Errorf("console exchanges = %d; want a fresh login and a second question", *asked)
	}
}

func TestWaitBootedWithoutSystemctl(t *testing.T) {
	// 2105's admin shell is vbash-sandbox, which has no systemctl
	c, asked := bootConsole(t, []string{"vbash-sandbox: systemctl: command not found\ntmpuser@r1:~$ "})
	if err := WaitBooted(c, "tmpuser", "pw", 5*time.Second); err != nil {
		t.Fatal(err)
	}
	if *asked != 1 {
		t.Errorf("asked %d times, want 1", *asked)
	}
}
