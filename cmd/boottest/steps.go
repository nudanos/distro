package main

import (
	"regexp"
	"time"

	"github.com/nudanos/distro/internal/boottest"
)

var (
	loginPrompt = regexp.MustCompile(`login: $`)
	passPrompt  = regexp.MustCompile(`Password: $`)
	opPrompt    = regexp.MustCompile(`:~\$ $`)
	cfgPrompt   = regexp.MustCompile(`# $`)
)

// testPassword is the administrator password the test sets on install; it is
// not a secret (the VM is thrown away) and must differ from "vyatta".
const testPassword = "NuDanOS-test-1"

// liveSteps runs on the ISO: log in, configure, commit, save. (spec addendum §1)
func liveSteps(c *boottest.Console, t func(time.Duration) time.Duration) error {
	steps := []func() error{
		func() error { _, err := c.Expect(loginPrompt, t(20*time.Minute)); return err },
		func() error { return c.Send("vyatta") },
		func() error { _, err := c.Expect(passPrompt, t(time.Minute)); return err },
		func() error { return c.Send("vyatta") },
		func() error { _, err := c.Expect(opPrompt, t(2*time.Minute)); return err },
		func() error { return c.Send("show version") },
		func() error {
			_, err := c.Expect(regexp.MustCompile(`(?m)^Base:\s+Debian GNU/Linux 13`), t(time.Minute))
			return err
		},
		func() error { _, err := c.Expect(regexp.MustCompile(`(?m)^FRR:\s+10\.7`), t(time.Minute)); return err },
		func() error { _, err := c.Expect(opPrompt, t(time.Minute)); return err },
		func() error { return c.Send("configure") },
		func() error { _, err := c.Expect(cfgPrompt, t(time.Minute)); return err },
		// Review Focus 3: a DPDK-only setting is refused with its reason.
		func() error { return c.Send("set interfaces dataplane dp0s3 cpu-affinity 1") },
		func() error { return c.Send("commit") },
		func() error {
			_, err := c.Expect(regexp.MustCompile(`requires the DPDK dataplane`), t(2*time.Minute))
			return err
		},
		func() error { return c.Send("discard") },
		func() error { _, err := c.Expect(cfgPrompt, t(time.Minute)); return err },
		func() error { return c.Send("set interfaces dataplane dp0s3 address 192.0.2.1/24") },
		func() error { return c.Send("set system host-name nudanos-test") },
		func() error {
			return c.Send("set system login user tester authentication plaintext-password " + testPassword)
		},
		func() error { return c.Send("commit") },
		func() error { _, err := c.Expect(cfgPrompt, t(5*time.Minute)); return err },
		func() error { return c.Send("save") },
		func() error {
			_, err := c.Expect(regexp.MustCompile(`Saving configuration|Done`), t(2*time.Minute))
			return err
		},
		func() error { return c.Send("exit") },
		func() error { _, err := c.Expect(opPrompt, t(time.Minute)); return err },
	}
	return run(steps)
}

// installSteps installs to the virtual disk, refusing the default password.
func installSteps(c *boottest.Console, t func(time.Duration) time.Duration) error {
	if err := c.Send("install image"); err != nil {
		return err
	}
	sawRefusal := false
	refusal := regexp.MustCompile(`published live-ISO default`)
	pw := 0
	// Only the disk-overwrite confirmations default to No ("Continue with
	// installation? (Yes/No) [No]", "Continue (Yes/No) [No]"); every other
	// question (grub password, reduced grub layout, console, partition sizes,
	// saving the live configuration) keeps its default. A blanket "Yes"
	// would also accept "set up a grub password?". The default-answer rule
	// needs a word, "?" or ")" before the bracket, so a kernel timestamp
	// ("[  300.1]") that happens to end a read is not taken for a prompt.
	rules := []boottest.Rule{
		{Prompt: regexp.MustCompile(`Continue[^\r\n]*\(Yes/No\) \[No\]: ?$`), Reply: "Yes"},
		{Prompt: regexp.MustCompile(`Enter username for administrator account: $`), Reply: "vyatta"},
		{Prompt: regexp.MustCompile(`[A-Za-z?)] \[[^\]\r\n]*\]:? ?$`), Reply: ""}, // accept the default
	}
	// Password prompts: first answer the published default (must be refused),
	// then the test password twice.
	pwPrompt := regexp.MustCompile(`(Enter|Retype) password for user '[^']+':$`)
	// The installer ends with "Done."; partitioning prints "Done!" earlier.
	done := regexp.MustCompile(`Done\.\r?\n`)
	for {
		i, _, err := firstOf(c, append([]*regexp.Regexp{done, refusal, pwPrompt}, prompts(rules)...), t(30*time.Minute))
		if err != nil {
			return err
		}
		switch {
		case i == 0:
			if !sawRefusal {
				return errorf("install image accepted the default password 'vyatta'")
			}
			_, err := c.Expect(opPrompt, t(time.Minute))
			return err
		case i == 1:
			sawRefusal = true
		case i == 2:
			reply := testPassword
			if pw == 0 {
				reply = "vyatta"
			}
			pw++
			if err := c.Send(reply); err != nil {
				return err
			}
		default:
			if err := c.Send(rules[i-3].Reply); err != nil {
				return err
			}
		}
	}
}

// diskSteps runs after booting the installed system: the config survived.
func diskSteps(c *boottest.Console, t func(time.Duration) time.Duration) error {
	steps := []func() error{
		func() error {
			_, err := c.Expect(regexp.MustCompile(`nudanos-test login: $`), t(20*time.Minute))
			return err
		},
		func() error { return c.Send("tester") },
		func() error { _, err := c.Expect(passPrompt, t(time.Minute)); return err },
		func() error { return c.Send(testPassword) },
		func() error { _, err := c.Expect(opPrompt, t(2*time.Minute)); return err },
		func() error { return c.Send("show interfaces") },
		func() error {
			_, err := c.Expect(regexp.MustCompile(`dp0s3\s+192\.0\.2\.1/24`), t(time.Minute))
			return err
		},
		func() error { return c.Send("show version") },
		func() error { _, err := c.Expect(regexp.MustCompile(`(?m)^Kernel:\s+\S+`), t(time.Minute)); return err },
	}
	return run(steps)
}

// halt powers the VM off cleanly, so QEMU flushes the qcow2 disk before it
// exits (killing QEMU can lose cached writes from install image).
func halt(c *boottest.Console, t func(time.Duration) time.Duration) error {
	if err := c.Send("poweroff"); err != nil {
		return err
	}
	if _, err := c.Expect(regexp.MustCompile(`Proceed with poweroff\? \(Yes/No\) \[No\] ?$`), t(time.Minute)); err != nil {
		return err
	}
	return c.Send("y")
}

func run(steps []func() error) error {
	for i, s := range steps {
		if err := s(); err != nil {
			return errorf("step %d: %v", i+1, err)
		}
	}
	return nil
}

func prompts(rules []boottest.Rule) []*regexp.Regexp {
	out := make([]*regexp.Regexp, len(rules))
	for i, r := range rules {
		out[i] = r.Prompt
	}
	return out
}

func firstOf(c *boottest.Console, res []*regexp.Regexp, d time.Duration) (int, string, error) {
	return c.First(res, d)
}
