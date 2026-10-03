package main

import (
	"bytes"
	"io"
	"net"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nudanos/distro/internal/boottest"
)

// installer replays vyatta-install-image's prompts in order (texts copied from
// port-vyatta-image-tools) and records what the test typed after each.
func installer(t *testing.T, vm net.Conn, got chan<- []string) {
	t.Helper()
	var replies []string
	read := func() string {
		b := make([]byte, 256)
		n, _ := vm.Read(b)
		return strings.TrimSuffix(string(b[:n]), "\r")
	}
	ask := func(prompt string) {
		io.WriteString(vm, prompt)
		replies = append(replies, read())
	}
	read() // "install image"
	ask("Continue with installation? (Yes/No) [No]: ")
	ask("What would you like to name this image? [1.0~20261002]: ")
	ask("Partition (Auto/Parted/Skip) [Auto]: ")
	io.WriteString(vm, "[  300.123]") // a kernel message split across reads
	time.Sleep(20 * time.Millisecond)
	io.WriteString(vm, " virtio_blk virtio1: [vda] 16777216 512-byte logical blocks\r\n")
	ask("Install the image on? [vda]:")
	ask("Continue (Yes/No) [No]: ")
	ask("How much space would you like to allocate for the vRouter partition? [8192]: ")
	io.WriteString(vm, "Done!\r\n") // install-get-partition, not the end
	ask("Would you like to save the current configuration \r\ndirectory and config file? (Yes/No) [Yes]: ")
	ask("\r\nEnter username for administrator account: ")
	ask("Enter password for user 'vyatta':")
	io.WriteString(vm, "\r\n'vyatta' is the published live-ISO default; choose another password\r\n")
	ask("Enter password for user 'vyatta':")
	ask("Retype password for user 'vyatta':")
	ask("Enter the desired system console [ttyS0]: ")
	ask("Would you like to setup a grub password? (Yes/No) [No]: ")
	ask("Would you like to enable a reduced grub layout? (Yes/No) [No]: ")
	io.WriteString(vm, "Setting up grub on /dev/vda: OK\r\nDone.\r\nvyatta@vyatta:~$ ")
	got <- replies
}

func TestInstallStepsAnswersTheRealInstaller(t *testing.T) {
	a, vm := net.Pipe()
	defer a.Close()
	defer vm.Close()
	c := boottest.NewConsole(a, &bytes.Buffer{})
	got := make(chan []string, 1)
	go installer(t, vm, got)
	if err := installSteps(c, func(d time.Duration) time.Duration { return d / 600 }); err != nil {
		t.Fatal(err)
	}
	want := []string{"Yes", "", "", "", "Yes", "", "", "vyatta", "vyatta", testPassword, testPassword, "", "", ""}
	if r := <-got; strings.Join(r, "|") != strings.Join(want, "|") {
		t.Errorf("replies\n got %q\nwant %q", r, want)
	}
}

// Until the boot configuration has loaded the user does not exist: the
// console answers "Login incorrect" and the getty restarts. login retries.
func TestLoginRetriesUntilTheConfigurationHasLoaded(t *testing.T) {
	a, vm := net.Pipe()
	defer a.Close()
	defer vm.Close()
	c := boottest.NewConsole(a, &bytes.Buffer{})
	go func() {
		b := make([]byte, 64)
		for i := 0; i < 2; i++ {
			io.WriteString(vm, "\r\nnode login: ")
			vm.Read(b)
			io.WriteString(vm, "vyatta\r\nPassword: ")
			vm.Read(b)
			if i == 0 {
				io.WriteString(vm, "\r\nLogin incorrect\r\n")
			}
		}
		io.WriteString(vm, "\r\nvyatta@node:~$ ")
	}()
	if err := login(c, "vyatta", "vyatta", regexp.MustCompile(`login: $`), time.Second); err != nil {
		t.Fatal(err)
	}
}

// A configuration command is done only when its own "[edit]" and prompt come
// back; a prompt already in the buffer (type-ahead echoes, an earlier
// command) must not count.
func TestConfigureWaitsForEachCommandsOwnPrompt(t *testing.T) {
	a, vm := net.Pipe()
	defer a.Close()
	defer vm.Close()
	c := boottest.NewConsole(a, &bytes.Buffer{})
	var answered atomic.Bool
	go func() {
		b := make([]byte, 256)
		io.WriteString(vm, "[edit]\r\nvyatta@node# ") // stale prompt
		n, _ := vm.Read(b)
		time.Sleep(50 * time.Millisecond) // the commit takes a while
		answered.Store(true)
		io.WriteString(vm, string(b[:n])+"\n[edit]\r\nvyatta@node# ")
	}()
	time.Sleep(20 * time.Millisecond) // the stale prompt is in the buffer
	if err := configure(c, "commit", time.Second); err != nil {
		t.Fatal(err)
	}
	if !answered.Load() {
		t.Error("configure returned on a stale prompt, before the command answered")
	}
}

// Applying the boot configuration restarts the getty: an attempt can end in
// a fresh login prompt with neither a shell nor "Login incorrect".
func TestLoginRetriesWhenTheGettyRestartsMidAttempt(t *testing.T) {
	a, vm := net.Pipe()
	defer a.Close()
	defer vm.Close()
	c := boottest.NewConsole(a, &bytes.Buffer{})
	go func() {
		b := make([]byte, 64)
		io.WriteString(vm, "\r\nnode login: ")
		vm.Read(b)
		io.WriteString(vm, "vyatta\r\nPassword: ")
		vm.Read(b)
		io.WriteString(vm, "\r\n[  OK  ] Started serial-getty@ttyS0.service\r\nWelcome to NuDanOS - ttyS0\r\n\r\nnode login: ")
		vm.Read(b)
		io.WriteString(vm, "vyatta\r\nPassword: ")
		vm.Read(b)
		io.WriteString(vm, "\r\nvyatta@node:~$ ")
	}()
	if err := login(c, "vyatta", "vyatta", regexp.MustCompile(`login: $`), time.Second); err != nil {
		t.Fatal(err)
	}
}

// The real console ends "[edit]" with "\r\r\n" and wraps the prompt in
// bracketed-paste escapes (bytes copied from a layer 3 transcript).
func TestConfigDoneMatchesTheRealConsole(t *testing.T) {
	out := "set interfaces dataplane dp0s3 cpu-affinity 1\r\n\x1b[?2004l\r\x1b[?2004h[edit]\r\r\nvyatta@node# "
	if !cfgDone.MatchString(out) {
		t.Errorf("cfgDone does not match %q", out)
	}
}
