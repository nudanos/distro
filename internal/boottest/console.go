// Package boottest drives a VM's serial console: wait for output, type
// commands, answer installer prompts (the layer 3 boot test, spec 8.3).
package boottest

import (
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Console reads everything the VM prints into a buffer and matches patterns
// against the part not yet consumed by an earlier Expect.
type Console struct {
	w       io.Writer
	tr      io.Writer
	mu      sync.Mutex
	buf     []byte
	pos     int
	err     error
	changed chan struct{}
}

// Rule answers one installer prompt.
type Rule struct {
	Prompt *regexp.Regexp
	Reply  string
}

// NewConsole starts reading rw; everything read and sent is copied to transcript.
func NewConsole(rw io.ReadWriter, transcript io.Writer) *Console {
	c := &Console{w: rw, tr: transcript, changed: make(chan struct{}, 1)}
	go c.read(rw)
	return c
}

func (c *Console) read(r io.Reader) {
	b := make([]byte, 4096)
	for {
		n, err := r.Read(b)
		c.mu.Lock()
		if n > 0 {
			c.buf = append(c.buf, b[:n]...)
			c.tr.Write(b[:n])
		}
		if err != nil {
			c.err = err
		}
		c.mu.Unlock()
		select {
		case c.changed <- struct{}{}:
		default:
		}
		if err != nil {
			return
		}
	}
}

// Expect waits until re matches the unconsumed output, consumes through the
// match and returns it. On timeout the error names re and shows the last lines.
func (c *Console) Expect(re *regexp.Regexp, timeout time.Duration) (string, error) {
	_, m, err := c.first([]*regexp.Regexp{re}, timeout)
	return m, err
}

// First waits for the earliest match among res, consumes it and returns its index.
func (c *Console) First(res []*regexp.Regexp, timeout time.Duration) (int, string, error) {
	return c.first(res, timeout)
}

// first waits for the earliest match among res and returns its index.
func (c *Console) first(res []*regexp.Regexp, timeout time.Duration) (int, string, error) {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	for {
		c.mu.Lock()
		best, bestStart, bestEnd := -1, 0, 0
		for i, re := range res {
			if loc := re.FindIndex(c.buf[c.pos:]); loc != nil && (best < 0 || loc[0] < bestStart) {
				best, bestStart, bestEnd = i, loc[0], loc[1]
			}
		}
		if best >= 0 {
			m := string(c.buf[c.pos+bestStart : c.pos+bestEnd])
			c.pos += bestEnd
			c.mu.Unlock()
			return best, m, nil
		}
		err := c.err
		c.mu.Unlock()
		if err != nil {
			return -1, "", fmt.Errorf("console closed while waiting for %v: %v\nlast output:\n%s", res, err, c.Tail(15))
		}
		select {
		case <-c.changed:
		case <-deadline.C:
			return -1, "", fmt.Errorf("timed out after %s waiting for %v\nlast output:\n%s", timeout, res, c.Tail(15))
		}
	}
}

// Discard consumes everything read so far, so the next Expect only sees
// output that arrives after this call.
func (c *Console) Discard() {
	c.mu.Lock()
	c.pos = len(c.buf)
	c.mu.Unlock()
}

// Send types line followed by a carriage return.
func (c *Console) Send(line string) error {
	c.mu.Lock()
	c.tr.Write([]byte("<<" + line + ">>\n"))
	c.mu.Unlock()
	_, err := io.WriteString(c.w, line+"\r")
	return err
}

// Dialog answers prompts by rules until done matches. Each wait has timeout.
func (c *Console) Dialog(rules []Rule, done *regexp.Regexp, timeout time.Duration) error {
	res := []*regexp.Regexp{done}
	for _, r := range rules {
		res = append(res, r.Prompt)
	}
	for {
		i, _, err := c.first(res, timeout)
		if err != nil {
			return err
		}
		if i == 0 {
			return nil
		}
		if err := c.Send(rules[i-1].Reply); err != nil {
			return err
		}
	}
}

// Tail returns the last n lines of everything the VM printed.
func (c *Console) Tail(n int) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	lines := strings.Split(strings.ReplaceAll(string(c.buf), "\r", ""), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
