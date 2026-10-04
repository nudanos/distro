package boottest

import (
	"bytes"
	"io"
	"net"
	"regexp"
	"strings"
	"testing"
	"time"
)

// pipe returns a Console whose "VM" side is the returned net.Conn.
func pipe(t *testing.T) (*Console, net.Conn, *bytes.Buffer) {
	t.Helper()
	a, b := net.Pipe()
	var tr bytes.Buffer
	t.Cleanup(func() { a.Close(); b.Close() })
	return NewConsole(a, &tr), b, &tr
}

func TestExpectMatchesAcrossChunksAndNoise(t *testing.T) {
	c, vm, _ := pipe(t)
	go func() {
		io.WriteString(vm, "[   12.3] random: crng init done\r\nvyat")
		time.Sleep(20 * time.Millisecond)
		io.WriteString(vm, "ta login: ")
	}()
	if _, err := c.Expect(regexp.MustCompile(`login: $`), time.Second); err != nil {
		t.Fatal(err)
	}
}

func TestExpectConsumesSoTheNextStartsAfterTheMatch(t *testing.T) {
	c, vm, _ := pipe(t)
	go io.WriteString(vm, "$ one\r\n$ two\r\n")
	if _, err := c.Expect(regexp.MustCompile(`one`), time.Second); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Expect(regexp.MustCompile(`one`), 100*time.Millisecond); err == nil {
		t.Fatal("matched consumed output twice")
	}
}

func TestExpectTimeoutShowsTheLastLines(t *testing.T) {
	c, vm, _ := pipe(t)
	go io.WriteString(vm, "line1\r\nKernel panic - not syncing\r\n")
	_, err := c.Expect(regexp.MustCompile(`login:`), 100*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "Kernel panic") || !strings.Contains(err.Error(), "login:") {
		t.Fatalf("err = %v, want the awaited pattern and the last console lines", err)
	}
}

func TestSendAppendsCarriageReturnAndTranscriptRecordsBoth(t *testing.T) {
	c, vm, tr := pipe(t)
	got := make(chan string, 1)
	go func() {
		b := make([]byte, 64)
		n, _ := vm.Read(b)
		got <- string(b[:n])
		io.WriteString(vm, "ok\r\n")
	}()
	if err := c.Send("show version"); err != nil {
		t.Fatal(err)
	}
	if g := <-got; g != "show version\r" {
		t.Errorf("sent %q", g)
	}
	c.Expect(regexp.MustCompile(`ok`), time.Second)
	if !strings.Contains(tr.String(), "ok") {
		t.Errorf("transcript = %q", tr.String())
	}
}

func TestDialogAnswersPromptsUntilDone(t *testing.T) {
	c, vm, _ := pipe(t)
	replies := make(chan string, 4)
	go func() {
		buf := make([]byte, 128)
		for _, p := range []string{"Continue? (Yes/No) [No]: ", "Enter password for user 'vyatta':"} {
			io.WriteString(vm, p)
			n, _ := vm.Read(buf)
			replies <- string(buf[:n])
		}
		io.WriteString(vm, "\r\nDone!\r\n")
	}()
	err := c.Dialog([]Rule{
		{Prompt: regexp.MustCompile(`Continue\? \(Yes/No\) \[No\]: $`), Reply: "Yes"},
		{Prompt: regexp.MustCompile(`Enter password for user '[^']+':$`), Reply: "s3cret"},
	}, regexp.MustCompile(`Done!`), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if a, b := <-replies, <-replies; a != "Yes\r" || b != "s3cret\r" {
		t.Errorf("replies %q %q", a, b)
	}
}

// Output after a prompt ("node login: [  OK  ] Stopped ...") means an
// end-anchored pattern can never match it; after a quiet spell a bare
// carriage return makes the getty or shell print a fresh prompt.
func TestFirstNudgingSendsCarriageReturnWhenStalled(t *testing.T) {
	c, vm, _ := pipe(t)
	go func() {
		io.WriteString(vm, "node login: [  OK  ] Stopped serial-getty@ttyS0.service.\r\n")
		b := make([]byte, 16)
		n, _ := vm.Read(b)
		if string(b[:n]) == "\r" {
			io.WriteString(vm, "\r\nnode login: ")
		}
	}()
	if _, _, err := c.FirstNudging([]*regexp.Regexp{regexp.MustCompile(`login: $`)}, 50*time.Millisecond, time.Second); err != nil {
		t.Fatal(err)
	}
}
