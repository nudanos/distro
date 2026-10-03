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
	// a configuration command has finished when "[edit]" and the prompt follow
	cfgDone = regexp.MustCompile(`\[edit\]\r*\n[^\r\n]*# $`)
	// "save" either saves or says that commit already saved
	saved = regexp.MustCompile(`Saving configuration|Done|'commit' saves configuration`)
)

// testPassword is the administrator password the test sets on install; it is
// not a secret (the VM is thrown away) and must differ from "vyatta".
const testPassword = "NuDanOS-test-1"

// liveSteps runs on the ISO: log in, configure, commit, save. (spec addendum §1)
func liveSteps(c *boottest.Console, t func(time.Duration) time.Duration) error {
	steps := []func() error{
		func() error { return login(c, "vyatta", "vyatta", loginPrompt, t(30*time.Minute)) },
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
		func() error { return configure(c, "set interfaces dataplane dp0s3 cpu-affinity 1", t(time.Minute)) },
		func() error {
			return commit(c, regexp.MustCompile(`requires the DPDK dataplane`), 30*time.Second, t(20*time.Minute))
		},
		func() error { return configure(c, "discard", t(time.Minute)) },
		func() error {
			return configure(c, "set interfaces dataplane dp0s3 address 192.0.2.1/24", t(time.Minute))
		},
		func() error { return configure(c, "set system host-name nudanos-test", t(time.Minute)) },
		func() error {
			return configure(c, "set system login user tester authentication plaintext-password "+testPassword, t(time.Minute))
		},
		func() error { return configure(c, "set system login user tester level admin", t(time.Minute)) },
		func() error { return commit(c, nil, 30*time.Second, t(20*time.Minute)) },
		func() error {
			c.Discard()
			if err := c.Send("save"); err != nil {
				return err
			}
			if _, err := c.Expect(saved, t(5*time.Minute)); err != nil {
				return err
			}
			_, err := c.Expect(cfgDone, t(5*time.Minute))
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
	done := regexp.MustCompile(`Done\.\r*\n`)
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
			return login(c, "tester", testPassword, regexp.MustCompile(`nudanos-test login: $`), t(30*time.Minute))
		},
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

// login logs in at the console. Until the boot configuration has loaded the
// user does not exist, so the console answers "Login incorrect"; applying the
// configuration also restarts the getty, which can end an attempt with a
// fresh login prompt and no answer at all. Either way it tries again, until
// total has passed.
func login(c *boottest.Console, user, password string, prompt *regexp.Regexp, total time.Duration) error {
	incorrect := regexp.MustCompile(`Login incorrect`)
	deadline := time.Now().Add(total)
	atPrompt := false
	for {
		left := time.Until(deadline)
		if left <= 0 {
			return errorf("could not log in as %s within %s", user, total)
		}
		if !atPrompt {
			if _, err := c.Expect(prompt, left); err != nil {
				return err
			}
		}
		atPrompt = false
		if err := c.Send(user); err != nil {
			return err
		}
		i, _, err := c.First([]*regexp.Regexp{passPrompt, prompt}, left)
		if err != nil {
			return err
		}
		if i == 1 { // the getty restarted before asking for the password
			atPrompt = true
			continue
		}
		if err := c.Send(password); err != nil {
			return err
		}
		i, _, err = c.First([]*regexp.Regexp{opPrompt, incorrect, prompt}, left)
		if err != nil {
			return err
		}
		switch i {
		case 0:
			return nil
		case 2: // the getty restarted: a fresh prompt is already here
			atPrompt = true
		}
	}
}

// configure runs one configuration-mode command and waits for its own
// "[edit]" prompt: output already read (type-ahead echoes, an earlier prompt)
// is discarded first, so a stale prompt cannot end the wait.
func configure(c *boottest.Console, cmd string, timeout time.Duration) error {
	c.Discard()
	if err := c.Send(cmd); err != nil {
		return err
	}
	_, err := c.Expect(cfgDone, timeout)
	return err
}

// commit commits in configuration mode. want, if set, is the expected outcome
// (a refusal); otherwise the commit must succeed. The console accepts logins
// while the boot configuration is still being committed, so "Commit already
// in progress" means wait pause and try again, until timeout.
func commit(c *boottest.Console, want *regexp.Regexp, pause, timeout time.Duration) error {
	inProgress := regexp.MustCompile(`Commit already in progress`)
	failed := regexp.MustCompile(`Commit failed`)
	deadline := time.Now().Add(timeout)
	for {
		left := time.Until(deadline)
		if left <= 0 {
			return errorf("commit still in progress after %s", timeout)
		}
		c.Discard()
		if err := c.Send("commit"); err != nil {
			return err
		}
		res := []*regexp.Regexp{inProgress, failed, cfgDone}
		if want != nil {
			res = append(res, want)
		}
		i, _, err := c.First(res, left)
		if err != nil {
			return err
		}
		switch {
		case i == 0: // the boot commit is still running
			if _, err := c.Expect(cfgDone, left); err != nil {
				return err
			}
			time.Sleep(pause)
		case i == 3: // the expected refusal
			_, err := c.Expect(cfgDone, left)
			return err
		case want != nil:
			return errorf("commit: wanted %v, got another outcome:\n%s", want, c.Tail(15))
		case i == 1:
			return errorf("commit failed:\n%s", c.Tail(15))
		default:
			return nil
		}
	}
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
