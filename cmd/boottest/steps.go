package main

import (
	"regexp"
	"strings"
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

// adminUser is the administrator install image creates.
const adminUser = "nudanos"

// liveSteps runs on the ISO: log in, configure, commit, save. (spec addendum §1)
func liveSteps(c *boottest.Console, t func(time.Duration) time.Duration) error {
	steps := []func() error{
		func() error { return login(c, "vyatta", "vyatta", loginPrompt, t(30*time.Minute)) },
		func() error { return c.Send("show version") },
		// The image carries its NuDanOS version (95-build.txt), not UNKNOWN.
		func() error {
			_, err := c.Expect(regexp.MustCompile(`(?m)^Version:\s+1\.0-\d{8}\.\d{4}`), t(time.Minute))
			return err
		},
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
		// A forced link speed is refused too, rather than silently ignored.
		func() error { return configure(c, "set interfaces dataplane dp0s3 speed 100m", t(time.Minute)) },
		func() error {
			return commit(c, regexp.MustCompile(`forced link speed requires the DPDK dataplane`), 30*time.Second, t(20*time.Minute))
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

// installSteps installs to the virtual disk as administrator adminUser,
// refusing the default password.
func installSteps(c *boottest.Console, t func(time.Duration) time.Duration) error {
	if err := c.Send("install image"); err != nil {
		return err
	}
	sawRefusal := false
	refusal := regexp.MustCompile(`published live-ISO default`)
	pw := 0
	// Only the disk-overwrite confirmations default to No ("Continue with
	// installation? (Yes/No) [No]", "Continue (Yes/No) [No]"); every other
	// question (grub password, reduced grub layout, console, partition sizes)
	// keeps its default. A blanket "Yes" would also accept "set up a grub
	// password?". A default is accepted only after "?", ")" or the console
	// questions' last word: progress lines end in brackets too ("Creating new
	// disk_label on [vda]: ", a kernel timestamp "[  300.1]").
	rules := []boottest.Rule{
		{Prompt: regexp.MustCompile(`Continue[^\r\n]*\(Yes/No\) \[No\]: ?$`), Reply: "Yes"},
		{Prompt: regexp.MustCompile(`Enter username for administrator account: $`), Reply: adminUser},
		{Prompt: regexp.MustCompile(`(?:\?|\)|console|speed) \[[^\]\r\n]*\]:? ?$`), Reply: ""}, // accept the default
	}
	// Password prompts: first answer the published default (must be refused),
	// then the test password twice.
	pwPrompt := regexp.MustCompile(`(Enter|Retype) password for user '[^']+':$`)
	// The installer ends with "Done."; partitioning prints "Done!" earlier.
	done := regexp.MustCompile(`Done\.\r*\n`)
	// A question asked again and again (a refused name, a password rule) has
	// no answer here: fail instead of looping until the job is killed.
	const maxAsked = 3
	asked := map[string]int{}
	deadline := time.Now().Add(t(90 * time.Minute))
	for {
		left := time.Until(deadline)
		if left <= 0 {
			return errorf("install image did not finish within %s\nlast output:\n%s", t(90*time.Minute), c.Tail(15))
		}
		if w := t(30 * time.Minute); left > w {
			left = w
		}
		i, m, err := firstOf(c, append([]*regexp.Regexp{done, refusal, pwPrompt}, prompts(rules)...), left)
		if err != nil {
			return err
		}
		if i >= 2 {
			q := strings.TrimSpace(m)
			if asked[q]++; asked[q] > maxAsked {
				return errorf("install image asked %q %d times\nlast output:\n%s", q, asked[q], c.Tail(15))
			}
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
		func() error {
			return op(c, "show interfaces", regexp.MustCompile(`dp0s3\s+192\.0\.2\.1/24`), t(time.Minute))
		},
		func() error { return op(c, "show version", regexp.MustCompile(`(?m)^Kernel:\s+\S+`), t(time.Minute)) },
		// The administrator named at install works; the live ISO's
		// vyatta/vyatta was dropped (review: Critical #1).
		func() error { return c.Send("exit") },
		func() error {
			return loginRefused(c, "vyatta", "vyatta", regexp.MustCompile(`nudanos-test login: $`), t(5*time.Minute))
		},
		func() error {
			return login(c, adminUser, testPassword, regexp.MustCompile(`nudanos-test login: $`), t(5*time.Minute))
		},
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
	// Console output after a prompt (a systemd status line) buries it; after
	// a quiet spell a bare carriage return gets a fresh one.
	quiet := total / 20
	atPrompt := false
	for {
		left := time.Until(deadline)
		if left <= 0 {
			return errorf("could not log in as %s within %s", user, total)
		}
		if !atPrompt {
			if _, _, err := c.FirstNudging([]*regexp.Regexp{prompt}, quiet, left); err != nil {
				return err
			}
		}
		atPrompt = false
		if err := c.Send(user); err != nil {
			return err
		}
		i, _, err := c.FirstNudging([]*regexp.Regexp{passPrompt, prompt}, quiet, left)
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
		i, _, err = c.FirstNudging([]*regexp.Regexp{opPrompt, incorrect, prompt}, quiet, left)
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

// op runs one operational command, waits for want in its output and then for
// the prompt, so nothing typed next reaches a pager still reading the keyboard.
func op(c *boottest.Console, cmd string, want *regexp.Regexp, timeout time.Duration) error {
	c.Discard()
	if err := c.Send(cmd); err != nil {
		return err
	}
	if _, err := c.Expect(want, timeout); err != nil {
		return err
	}
	_, err := c.Expect(opPrompt, timeout)
	return err
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

// loginRefused checks that user/password cannot log in at the console.
func loginRefused(c *boottest.Console, user, password string, prompt *regexp.Regexp, total time.Duration) error {
	quiet := total / 20
	if _, _, err := c.FirstNudging([]*regexp.Regexp{prompt}, quiet, total); err != nil {
		return err
	}
	if err := c.Send(user); err != nil {
		return err
	}
	if _, _, err := c.FirstNudging([]*regexp.Regexp{passPrompt}, quiet, total); err != nil {
		return err
	}
	if err := c.Send(password); err != nil {
		return err
	}
	i, _, err := c.First([]*regexp.Regexp{regexp.MustCompile(`Login incorrect`), opPrompt}, total)
	if err != nil {
		return err
	}
	if i == 1 {
		return errorf("%s/%s logged in on the installed system", user, password)
	}
	return nil
}
