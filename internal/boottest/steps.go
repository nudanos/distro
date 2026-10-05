package boottest

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Console prompts of a NuDanOS (and DANOS) router.
var (
	LoginPrompt = regexp.MustCompile(`login: $`)
	PassPrompt  = regexp.MustCompile(`Password: $`)
	OpPrompt    = regexp.MustCompile(`:~\$ $`)
	CfgPrompt   = regexp.MustCompile(`# $`)
	// a configuration command has finished when "[edit]" and the prompt follow
	CfgDone = regexp.MustCompile(`\[edit\]\r*\n[^\r\n]*# $`)

	untilOpPrompt = regexp.MustCompile(`(?s)^.*?:~\$ $`)
	escapes       = regexp.MustCompile(`\x1b\[[0-9;?]*[a-zA-Z]|\x1b\[!p|\x1b\[\?[0-9]+[hl]`)
)

func errorf(format string, a ...any) error { return fmt.Errorf(format, a...) }

// InstallImage installs to the virtual disk as administrator admin,
// refusing the default password.
func InstallImage(c *Console, admin, password string, t func(time.Duration) time.Duration) error {
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
	rules := []Rule{
		{Prompt: regexp.MustCompile(`Continue[^\r\n]*\(Yes/No\) \[No\]: ?$`), Reply: "Yes"},
		{Prompt: regexp.MustCompile(`Enter username for administrator account: $`), Reply: admin},
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
			_, err := c.Expect(OpPrompt, t(time.Minute))
			return err
		case i == 1:
			sawRefusal = true
		case i == 2:
			reply := password
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

// Halt powers the VM off cleanly, so QEMU flushes the qcow2 disk before it
// exits (killing QEMU can lose cached writes from install image).
func Halt(c *Console, t func(time.Duration) time.Duration) error {
	if err := c.Send("poweroff"); err != nil {
		return err
	}
	if _, err := c.Expect(regexp.MustCompile(`Proceed with poweroff\? \(Yes/No\) \[No\] ?$`), t(time.Minute)); err != nil {
		return err
	}
	return c.Send("y")
}

// Login logs in at the console. Until the boot configuration has loaded the
// user does not exist, so the console answers "Login incorrect"; applying the
// configuration also restarts the getty, which can end an attempt with a
// fresh login prompt and no answer at all. Either way it tries again, until
// total has passed.
func Login(c *Console, user, password string, prompt *regexp.Regexp, total time.Duration) error {
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
		i, _, err := c.FirstNudging([]*regexp.Regexp{PassPrompt, prompt}, quiet, left)
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
		i, _, err = c.FirstNudging([]*regexp.Regexp{OpPrompt, incorrect, prompt}, quiet, left)
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

// Op runs one operational command, waits for want in its output and then for
// the prompt, so nothing typed next reaches a pager still reading the keyboard.
func Op(c *Console, cmd string, want *regexp.Regexp, timeout time.Duration) error {
	c.Discard()
	if err := c.Send(cmd); err != nil {
		return err
	}
	if _, err := c.Expect(want, timeout); err != nil {
		return err
	}
	_, err := c.Expect(OpPrompt, timeout)
	return err
}

// Configure runs one configuration-mode command and waits for its own
// "[edit]" prompt: output already read (type-ahead echoes, an earlier prompt)
// is discarded first, so a stale prompt cannot end the wait.
func Configure(c *Console, cmd string, timeout time.Duration) error {
	c.Discard()
	if err := c.Send(cmd); err != nil {
		return err
	}
	_, err := c.Expect(CfgDone, timeout)
	return err
}

// Commit commits in configuration mode. want, if set, is the expected outcome
// (a refusal); otherwise the commit must succeed. The console accepts logins
// while the boot configuration is still being committed, so "Commit already
// in progress" means wait pause and try again, until timeout.
func Commit(c *Console, want *regexp.Regexp, pause, timeout time.Duration) error {
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
		res := []*regexp.Regexp{inProgress, failed, CfgDone}
		if want != nil {
			res = append(res, want)
		}
		i, _, err := c.First(res, left)
		if err != nil {
			return err
		}
		switch {
		case i == 0: // the boot commit is still running
			if _, err := c.Expect(CfgDone, left); err != nil {
				return err
			}
			time.Sleep(pause)
		case i == 3: // the expected refusal
			_, err := c.Expect(CfgDone, left)
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

func prompts(rules []Rule) []*regexp.Regexp {
	out := make([]*regexp.Regexp, len(rules))
	for i, r := range rules {
		out[i] = r.Prompt
	}
	return out
}

func firstOf(c *Console, res []*regexp.Regexp, d time.Duration) (int, string, error) {
	return c.First(res, d)
}

// LoginRefused checks that user/password cannot log in at the console.
func LoginRefused(c *Console, user, password string, prompt *regexp.Regexp, total time.Duration) error {
	quiet := total / 20
	if _, _, err := c.FirstNudging([]*regexp.Regexp{prompt}, quiet, total); err != nil {
		return err
	}
	if err := c.Send(user); err != nil {
		return err
	}
	if _, _, err := c.FirstNudging([]*regexp.Regexp{PassPrompt}, quiet, total); err != nil {
		return err
	}
	if err := c.Send(password); err != nil {
		return err
	}
	i, _, err := c.First([]*regexp.Regexp{regexp.MustCompile(`Login incorrect`), OpPrompt}, total)
	if err != nil {
		return err
	}
	if i == 1 {
		return errorf("%s/%s logged in on the installed system", user, password)
	}
	return nil
}

// OpOutput runs one operational command and returns what it printed: the
// lines between the echoed command and the next prompt, without escape
// sequences or carriage returns.
func OpOutput(c *Console, cmd string, timeout time.Duration) (string, error) {
	c.Discard()
	if err := c.Send(cmd); err != nil {
		return "", err
	}
	m, err := c.Expect(untilOpPrompt, timeout)
	if err != nil {
		return "", err
	}
	lines := strings.Split(strings.ReplaceAll(escapes.ReplaceAllString(m, ""), "\r", ""), "\n")
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == strings.TrimSpace(cmd) {
		lines = lines[1:]
	}
	if len(lines) > 0 {
		lines = lines[:len(lines)-1] // the prompt
	}
	if len(lines) == 0 {
		return "", nil
	}
	return strings.Join(lines, "\n") + "\n", nil
}
