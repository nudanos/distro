package boottest

import (
	"bytes"
	"io"
	"net"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// The layer 3 test values: the administrator install image creates and its
// password (not secrets; the VM is thrown away).
const (
	adminUser    = "nudanos"
	testPassword = "NuDanOS-test-1"
)

// installer replays vyatta-install-image's prompts in the order a live-ISO
// install asks them (layer 3 transcript, 2026-10-03) and records what the
// test typed after each.
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
	io.WriteString(vm, "\r\nWelcome to the NuDanOS image installer.\r\n")
	ask("What would you like to name this image? [1.0-20261003.1532]: ")
	io.WriteString(vm, "This image will be named: 1.0-20261003.1532\r\nChecking drives for previous configurations...\r\n")
	ask("I found the following configuration files:\r\n    /config/config.boot\r\n    /opt/vyatta/etc/config.boot.default\r\nWhich one should I copy? [/config/config.boot]: ")
	ask("\r\nEnter username for administrator account: ")
	ask("\r\nEnter password for administrator account\r\nEnter password for user 'nudanos':")
	io.WriteString(vm, "\r\n'vyatta' is the published live-ISO default; choose another password\r\n")
	ask("Enter password for user 'nudanos':")
	ask("Retype password for user 'nudanos':")
	ask("\r\nEnter the desired system console [ttyS0]: ")
	ask("Enter the console speed [115200]: ")
	ask("Would you like to setup a grub password? (Yes/No) [No]: ")
	ask("Would you like to enable a reduced grub layout? (Yes/No) [No]: ")
	io.WriteString(vm, "Probing drives: OK\r\nThe following drives were detected on your system:\r\n vda\t8589MB\r\n")
	ask("Install the image on? [vda]:")
	ask("Disk label type (msdos/gpt) [gpt]: ")
	ask("Partition (Auto/Parted) [Auto]: ")
	io.WriteString(vm, "[  300.123]") // a kernel message split across reads
	time.Sleep(20 * time.Millisecond)
	io.WriteString(vm, " virtio_blk virtio1: [vda] 16777216 512-byte logical blocks\r\n")
	ask("Size of BIOS_BOOT partition? [256]: ")
	ask("How much space would you like to allocate for the vRouter partition? [8308]: ")
	ask("Size of the log partition? [0]: ")
	ask("Print final partition sizes? (Yes/No) [No]: ")
	ask("Ready to write partitions to disk\r\nContinue (Yes/No) [No]: ")
	// progress, not a question: nothing may be typed here
	io.WriteString(vm, "Creating new disk_label on [vda]: ")
	time.Sleep(50 * time.Millisecond)
	io.WriteString(vm, "OK\r\nSettling...Done!\r\n") // partitioning, not the end
	io.WriteString(vm, "Creating admin account for user [nudanos]\r\nRunning post-install script...\r\nDone.\r\nvyatta@node:~$ ")
	// anything typed now answered a line that was not a question
	vm.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	b := make([]byte, 256)
	if n, _ := vm.Read(b); n > 0 {
		replies = append(replies, "extra:"+strings.TrimSuffix(string(b[:n]), "\r"))
	}
	vm.SetReadDeadline(time.Time{})
	got <- replies
	io.Copy(io.Discard, vm) // anything typed after the end is a wrong answer, not a hang
}

func TestInstallStepsAnswersTheRealInstaller(t *testing.T) {
	a, vm := net.Pipe()
	defer a.Close()
	defer vm.Close()
	c := NewConsole(a, &bytes.Buffer{})
	got := make(chan []string, 1)
	go installer(t, vm, got)
	if err := InstallImage(c, adminUser, testPassword, func(d time.Duration) time.Duration { return d / 600 }); err != nil {
		t.Fatal(err)
	}
	want := []string{"", "", adminUser, "vyatta", testPassword, testPassword, "", "", "", "", "", "", "", "", "", "", "", "Yes"}
	select {
	case r := <-got:
		if strings.Join(r, "|") != strings.Join(want, "|") {
			t.Errorf("replies\n got %q\nwant %q", r, want)
		}
	case <-time.After(time.Second):
		t.Error("the installer replay did not finish: a non-prompt was answered")
	}
}

// An installer that asks the same thing again and again (a refused user
// name, a password-quality rule) must fail the test, not loop until the CI
// job is killed.
func TestInstallStepsGivesUpOnARepeatedPrompt(t *testing.T) {
	a, vm := net.Pipe()
	defer a.Close()
	defer vm.Close()
	c := NewConsole(a, &bytes.Buffer{})
	go func() {
		b := make([]byte, 64)
		vm.Read(b)
		for {
			if _, err := io.WriteString(vm, "\r\nEnter username for administrator account: "); err != nil {
				return
			}
			if _, err := vm.Read(b); err != nil {
				return
			}
		}
	}()
	done := make(chan error, 1)
	go func() {
		done <- InstallImage(c, adminUser, testPassword, func(d time.Duration) time.Duration { return d / 600 })
	}()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "times") {
			t.Fatalf("want a repeated-prompt error, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("installSteps kept answering a repeated prompt")
	}
}

// A systemd status line printed after "login: " leaves no prompt at the end
// of the output (layer 3 transcript: "node login: [  OK  ] Stopped
// serial-getty@ttyS0.service"); login must get a fresh one.
func TestLoginRecoversWhenNoiseFollowsThePrompt(t *testing.T) {
	a, vm := net.Pipe()
	defer a.Close()
	defer vm.Close()
	c := NewConsole(a, &bytes.Buffer{})
	go func() {
		b := make([]byte, 64)
		io.WriteString(vm, "\r\nnode login: [  OK  ] Stopped serial-getty@ttyS0.service.\r\n")
		askedPassword := false
		for {
			n, err := vm.Read(b)
			if err != nil {
				return
			}
			switch {
			case askedPassword:
				io.WriteString(vm, "\r\nvyatta@node:~$ ")
				return
			case string(b[:n]) == "\r":
				io.WriteString(vm, "\r\nnode login: ")
			default:
				io.WriteString(vm, string(b[:n])+"\nPassword: ")
				askedPassword = true
			}
		}
	}()
	if err := Login(c, "vyatta", "vyatta", regexp.MustCompile(`login: $`), 2*time.Second); err != nil {
		t.Fatal(err)
	}
}

// Until the boot configuration has loaded the user does not exist: the
// console answers "Login incorrect" and the getty restarts. login retries.
func TestLoginRetriesUntilTheConfigurationHasLoaded(t *testing.T) {
	a, vm := net.Pipe()
	defer a.Close()
	defer vm.Close()
	c := NewConsole(a, &bytes.Buffer{})
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
	if err := Login(c, "vyatta", "vyatta", regexp.MustCompile(`login: $`), time.Second); err != nil {
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
	c := NewConsole(a, &bytes.Buffer{})
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
	if err := Configure(c, "commit", time.Second); err != nil {
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
	c := NewConsole(a, &bytes.Buffer{})
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
	if err := Login(c, "vyatta", "vyatta", regexp.MustCompile(`login: $`), time.Second); err != nil {
		t.Fatal(err)
	}
}

// The real console ends "[edit]" with "\r\r\n" and wraps the prompt in
// bracketed-paste escapes (bytes copied from a layer 3 transcript).
func TestConfigDoneMatchesTheRealConsole(t *testing.T) {
	out := "set interfaces dataplane dp0s3 cpu-affinity 1\r\n\x1b[?2004l\r\x1b[?2004h[edit]\r\r\nvyatta@node# "
	if !CfgDone.MatchString(out) {
		t.Errorf("CfgDone does not match %q", out)
	}
}

// The console accepts logins while the boot configuration is still being
// committed: a commit then fails with "Commit already in progress" and must be
// retried, not taken as the answer.
func TestCommitRetriesWhileTheBootCommitRuns(t *testing.T) {
	a, vm := net.Pipe()
	defer a.Close()
	defer vm.Close()
	c := NewConsole(a, &bytes.Buffer{})
	go func() {
		b := make([]byte, 64)
		vm.Read(b)
		io.WriteString(vm, "commit\r\n[]\r\n\r\nCommit already in progress\r\n\r\nCommit failed!\r\n\r\n[edit]\r\r\nvyatta@node# ")
		vm.Read(b)
		io.WriteString(vm, "commit\r\ncpu-affinity requires the DPDK dataplane; this system forwards in the Linux kernel\r\nCommit failed!\r\n[edit]\r\r\nvyatta@node# ")
	}()
	if err := Commit(c, regexp.MustCompile(`requires the DPDK dataplane`), 10*time.Millisecond, time.Second); err != nil {
		t.Fatal(err)
	}
}

// "show interfaces" runs in a pager that reads the keyboard until the output
// is complete; the next command must wait for the prompt, or the pager eats
// its first keystrokes (layer 3 run 16: "-vbash: ow: command not found").
func TestOpWaitsForThePromptBeforeReturning(t *testing.T) {
	a, vm := net.Pipe()
	defer a.Close()
	defer vm.Close()
	c := NewConsole(a, &bytes.Buffer{})
	var prompted atomic.Bool
	go func() {
		b := make([]byte, 64)
		vm.Read(b)
		io.WriteString(vm, "show interfaces\r\nWaiting for data... (^X or interrupt to abort)dp0s3  192.0.2.1/24  u/u\r\n")
		time.Sleep(50 * time.Millisecond)
		prompted.Store(true)
		io.WriteString(vm, "tester@nudanos-test:~$ ")
	}()
	if err := Op(c, "show interfaces", regexp.MustCompile(`dp0s3\s+192\.0\.2\.1/24`), time.Second); err != nil {
		t.Fatal(err)
	}
	if !prompted.Load() {
		t.Error("op returned before the prompt came back")
	}
}

// On the installed system the live ISO's vyatta/vyatta must be refused
// (the administrator was named differently at install).
func TestLoginRefusedFailsWhenTheLoginWorks(t *testing.T) {
	for _, tc := range []struct {
		answer string
		ok     bool
	}{
		{"\r\nLogin incorrect\r\n\r\nnudanos-test login: ", true},
		{"\r\nvyatta@nudanos-test:~$ ", false},
	} {
		a, vm := net.Pipe()
		c := NewConsole(a, &bytes.Buffer{})
		go func(answer string) {
			b := make([]byte, 64)
			io.WriteString(vm, "\r\nnudanos-test login: ")
			vm.Read(b)
			io.WriteString(vm, "vyatta\r\nPassword: ")
			vm.Read(b)
			io.WriteString(vm, answer)
		}(tc.answer)
		err := LoginRefused(c, "vyatta", "vyatta", regexp.MustCompile(`login: $`), time.Second)
		if (err == nil) != tc.ok {
			t.Errorf("answer %q: err = %v", tc.answer, err)
		}
		a.Close()
		vm.Close()
	}
}

// OpOutput returns what a command printed, without the shell's escape
// sequences, carriage returns or the prompt that follows it (bytes from a
// layer 3 transcript).
func TestOpOutputStripsEscapesAndPrompt(t *testing.T) {
	a, vm := net.Pipe()
	defer a.Close()
	defer vm.Close()
	c := NewConsole(a, &bytes.Buffer{})
	go func() {
		b := make([]byte, 64)
		vm.Read(b)
		io.WriteString(vm, "show version\r\n\x1b[?2004l\r\rVersion:      1.0-20261004.0334\x1b[m\r\n\x1b[?2004hvyatta@node:~$ ")
	}()
	out, err := OpOutput(c, "show version", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if out != "Version:      1.0-20261004.0334\n" {
		t.Errorf("OpOutput = %q", out)
	}
}
