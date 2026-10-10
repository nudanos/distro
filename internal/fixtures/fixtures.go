// Package fixtures checks that DANOS 2105 configurations load on NuDanOS
// (plan 4): each captured 2105 config.boot is copied into a NuDanOS router,
// passed through the DPDK set-aside hook, loaded and committed, and what
// the router then shows is compared with what 2105 showed.
package fixtures

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/nudanos/distro/internal/boottest"
	"github.com/nudanos/distro/internal/topology"
)

// Ref is one captured 2105 configuration: its config.boot and the "show
// configuration commands" 2105 printed for it.
type Ref struct{ Name, ConfigBoot, Commands string }

// Failure is a configuration that did not load as 2105 loaded it.
type Failure struct{ Name, Detail string }

const (
	hook      = "/opt/vyatta/etc/boot-config.d/50-dpdk-set-aside"
	pathsFile = "/usr/share/vyatta-kernel-forwarding/dpdk-only-paths"
	refFile   = "/tmp/ref.boot"
)

// The base configuration (management address, host name, console users)
// differs between the 2105 and NuDanOS images and is not compared.
var baseLine = regexp.MustCompile(fmt.Sprintf(`^set (system host-name|system login user|interfaces dataplane dp0s%d) `, topology.MgmtPCI))

// Check loads every ref on vm and reports each that does not show what 2105
// showed, apart from the lines the set-aside hook removes by design.
func Check(ctx context.Context, vm *topology.VM, refs []Ref, t func(time.Duration) time.Duration) []Failure {
	c := vm.Console
	list, err := boottest.OpOutput(c, "cat "+pathsFile, t(time.Minute))
	if err != nil {
		return []Failure{{Name: pathsFile, Detail: err.Error()}}
	}
	pats := parsePatterns(list)
	if len(pats) == 0 {
		return []Failure{{Name: pathsFile, Detail: "no set-aside paths on the router:\n" + list}}
	}
	var failures []Failure
	for _, r := range refs {
		if ctx.Err() != nil {
			failures = append(failures, Failure{Name: r.Name, Detail: ctx.Err().Error()})
			continue
		}
		if detail := checkOne(c, r, pats, t); detail != "" {
			failures = append(failures, Failure{Name: r.Name, Detail: detail})
		}
	}
	return failures
}

func checkOne(c *boottest.Console, r Ref, pats []pattern, t func(time.Duration) time.Duration) string {
	// the hook writes its side files next to the file it is given
	if _, err := boottest.OpOutput(c, "rm -f "+refFile+" /tmp/config.boot.dpdk-only /tmp/config.boot.2105-original", t(time.Minute)); err != nil {
		return err.Error()
	}
	if err := copyFile(c, refFile, r.ConfigBoot, t); err != nil {
		return "copying config.boot: " + err.Error()
	}
	if out, err := boottest.OpOutput(c, hook+" "+refFile, t(2*time.Minute)); err != nil {
		return "set-aside hook: " + err.Error() + "\n" + out
	}
	if err := load(c, refFile, t); err != nil {
		return err.Error()
	}
	got, err := boottest.OpOutput(c, "show configuration commands", t(2*time.Minute))
	if err != nil {
		return err.Error()
	}
	return compare(r.Commands, got, pats)
}

// load loads file in configuration mode and commits it.
func load(c *boottest.Console, file string, t func(time.Duration) time.Duration) error {
	c.Discard()
	if err := c.Send("configure"); err != nil {
		return err
	}
	if _, err := c.Expect(boottest.CfgDone, t(time.Minute)); err != nil {
		return fmt.Errorf("configure: %w", err)
	}
	c.Discard()
	if err := c.Send("load " + file); err != nil {
		return err
	}
	out, err := c.Expect(boottest.CfgDone, t(5*time.Minute))
	if err != nil {
		return fmt.Errorf("load: %w", err)
	}
	var problem error
	if regexp.MustCompile(`(?i)\b(error|fail(ed|ure)?|invalid)\b`).MatchString(out) {
		problem = fmt.Errorf("load reported:\n%s", strings.TrimSpace(out))
	} else if err := boottest.Commit(c, nil, 30*time.Second, t(20*time.Minute)); err != nil {
		problem = fmt.Errorf("commit: %w\n%s", err, c.Tail(40))
	}
	exit := "exit"
	if problem != nil {
		exit = "exit discard"
	}
	c.Discard()
	if err := c.Send(exit); err != nil {
		return err
	}
	if _, err := c.Expect(boottest.OpPrompt, t(time.Minute)); err != nil {
		return err
	}
	return problem
}

var (
	continuation = regexp.MustCompile(`> $`)
	md5Line      = regexp.MustCompile(`\b([0-9a-f]{32})\b`)
)

// copyFile writes content to path through the console: base64 in a here
// document, one line at a time (a serial console drops a burst), then
// checks the md5 sum.
func copyFile(c *boottest.Console, path, content string, t func(time.Duration) time.Duration) error {
	enc := base64.StdEncoding.EncodeToString([]byte(content))
	c.Discard()
	if err := c.Send("base64 -d > " + path + " <<'NUDANOS_EOF'"); err != nil {
		return err
	}
	if _, err := c.Expect(continuation, t(time.Minute)); err != nil {
		return err
	}
	for len(enc) > 0 {
		n := min(76, len(enc))
		c.Discard()
		if err := c.Send(enc[:n]); err != nil {
			return err
		}
		if _, err := c.Expect(continuation, t(time.Minute)); err != nil {
			return err
		}
		enc = enc[n:]
	}
	c.Discard()
	if err := c.Send("NUDANOS_EOF"); err != nil {
		return err
	}
	if _, err := c.Expect(boottest.OpPrompt, t(time.Minute)); err != nil {
		return err
	}
	out, err := boottest.OpOutput(c, "md5sum "+path, t(time.Minute))
	if err != nil {
		return err
	}
	m := md5Line.FindStringSubmatch(out)
	if want := fmt.Sprintf("%x", md5.Sum([]byte(content))); m == nil || m[1] != want {
		return fmt.Errorf("md5sum %s: %q, want %s", path, strings.TrimSpace(out), want)
	}
	return nil
}

// pattern is a dpdk-only-paths entry: names ("*" matches one) and an
// optional value that a matching leaf must differ from ("!auto").
type pattern struct {
	names []string
	not   string
}

func parsePatterns(list string) []pattern {
	var pats []pattern
	for _, l := range strings.Split(list, "\n") {
		f := strings.Fields(l)
		if len(f) == 0 || strings.HasPrefix(f[0], "#") {
			continue
		}
		p := pattern{names: f}
		if last := f[len(f)-1]; strings.HasPrefix(last, "!") {
			p = pattern{names: f[:len(f)-1], not: last[1:]}
		}
		pats = append(pats, p)
	}
	return pats
}

var word = regexp.MustCompile(`'(?:[^']*)'|\S+`)

// setAside reports whether a "set ..." line falls under a pattern.
func setAside(line string, pats []pattern) bool {
	w := word.FindAllString(strings.TrimPrefix(line, "set "), -1)
	for i := range w {
		w[i] = strings.Trim(w[i], "'")
	}
	for _, p := range pats {
		if len(w) < len(p.names) {
			continue
		}
		ok := true
		for i, n := range p.names {
			if n != "*" && n != w[i] {
				ok = false
				break
			}
		}
		if !ok {
			continue
		}
		if p.not == "" || (len(w) == len(p.names)+1 && w[len(p.names)] != p.not) {
			return true
		}
	}
	return false
}

func lines(s string, drop func(string) bool) map[string]bool {
	m := map[string]bool{}
	for _, l := range strings.Split(strings.ReplaceAll(s, "\r", ""), "\n") {
		l = strings.Join(strings.Fields(l), " ")
		if strings.HasPrefix(l, "set ") && !baseLine.MatchString(l) && !drop(l) {
			m[l] = true
		}
	}
	return m
}

// compare reports what NuDanOS shows differently from 2105: lines missing
// (other than the set-aside ones) and lines 2105 did not have.
func compare(want, got string, pats []pattern) string {
	w := lines(want, func(l string) bool { return setAside(l, pats) })
	g := lines(got, func(string) bool { return false })
	var missing, extra []string
	for l := range w {
		if !g[l] {
			missing = append(missing, l)
		}
	}
	for l := range g {
		if !w[l] {
			extra = append(extra, l)
		}
	}
	if len(missing) == 0 && len(extra) == 0 {
		return ""
	}
	sort.Strings(missing)
	sort.Strings(extra)
	var b strings.Builder
	for _, l := range missing {
		fmt.Fprintf(&b, "missing on NuDanOS: %s\n", l)
	}
	for _, l := range extra {
		fmt.Fprintf(&b, "only on NuDanOS:    %s\n", l)
	}
	return b.String()
}
