package main

import (
	"bytes"
	"io"
	"net"
	"strings"
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
