package main

import (
	"regexp"
	"time"

	"github.com/nudanos/distro/internal/boottest"
)

var (
	// show version's first line follows the bracketed-paste escape and two
	// carriage returns, not a bare line start; the value is the image name 95-build.txt stamps
	versionLine = regexp.MustCompile(`(?m)^(?:\x1b\[\?2004l)?\r*Version:\s+1\.0-\d{8}\.\d{4}`)
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
		func() error { return boottest.Login(c, "vyatta", "vyatta", boottest.LoginPrompt, t(30*time.Minute)) },
		func() error { return c.Send("show version") },
		// The image carries its NuDanOS version (95-build.txt), not UNKNOWN.
		func() error { _, err := c.Expect(versionLine, t(time.Minute)); return err },
		func() error {
			_, err := c.Expect(regexp.MustCompile(`(?m)^Base:\s+Debian GNU/Linux 13`), t(time.Minute))
			return err
		},
		func() error { _, err := c.Expect(regexp.MustCompile(`(?m)^FRR:\s+10\.7`), t(time.Minute)); return err },
		func() error { _, err := c.Expect(boottest.OpPrompt, t(time.Minute)); return err },
		func() error { return c.Send("configure") },
		func() error { _, err := c.Expect(boottest.CfgPrompt, t(time.Minute)); return err },
		// Review Focus 3: a DPDK-only setting is refused with its reason.
		func() error {
			return boottest.Configure(c, "set interfaces dataplane dp0s3 cpu-affinity 1", t(time.Minute))
		},
		func() error {
			return boottest.Commit(c, regexp.MustCompile(`requires the DPDK dataplane`), 30*time.Second, t(20*time.Minute))
		},
		func() error { return boottest.Configure(c, "discard", t(time.Minute)) },
		// A forced link speed is refused too, rather than silently ignored.
		func() error {
			return boottest.Configure(c, "set interfaces dataplane dp0s3 speed 100m", t(time.Minute))
		},
		func() error {
			return boottest.Commit(c, regexp.MustCompile(`forced link speed requires the DPDK dataplane`), 30*time.Second, t(20*time.Minute))
		},
		func() error { return boottest.Configure(c, "discard", t(time.Minute)) },
		func() error {
			return boottest.Configure(c, "set interfaces dataplane dp0s3 address 192.0.2.1/24", t(time.Minute))
		},
		func() error { return boottest.Configure(c, "set system host-name nudanos-test", t(time.Minute)) },
		func() error {
			return boottest.Configure(c, "set system login user tester authentication plaintext-password "+testPassword, t(time.Minute))
		},
		func() error { return boottest.Configure(c, "set system login user tester level admin", t(time.Minute)) },
		func() error { return boottest.Commit(c, nil, 30*time.Second, t(20*time.Minute)) },
		func() error {
			c.Discard()
			if err := c.Send("save"); err != nil {
				return err
			}
			if _, err := c.Expect(saved, t(5*time.Minute)); err != nil {
				return err
			}
			_, err := c.Expect(boottest.CfgDone, t(5*time.Minute))
			return err
		},
		func() error { return c.Send("exit") },
		func() error { _, err := c.Expect(boottest.OpPrompt, t(time.Minute)); return err },
	}
	return run(steps)
}

// diskSteps runs after booting the installed system: the config survived.
func diskSteps(c *boottest.Console, t func(time.Duration) time.Duration) error {
	steps := []func() error{
		func() error {
			return boottest.Login(c, "tester", testPassword, regexp.MustCompile(`nudanos-test login: $`), t(30*time.Minute))
		},
		func() error {
			return boottest.Op(c, "show interfaces", regexp.MustCompile(`dp0s3\s+192\.0\.2\.1/24`), t(time.Minute))
		},
		func() error {
			return boottest.Op(c, "show version", regexp.MustCompile(`(?m)^Kernel:\s+\S+`), t(time.Minute))
		},
		// The administrator named at install works; the live ISO's
		// vyatta/vyatta was dropped (review: Critical #1).
		func() error { return c.Send("exit") },
		func() error {
			return boottest.LoginRefused(c, "vyatta", "vyatta", regexp.MustCompile(`nudanos-test login: $`), t(5*time.Minute))
		},
		func() error {
			return boottest.Login(c, adminUser, testPassword, regexp.MustCompile(`nudanos-test login: $`), t(5*time.Minute))
		},
	}
	return run(steps)
}

func run(steps []func() error) error {
	for i, s := range steps {
		if err := s(); err != nil {
			return errorf("step %d: %v", i+1, err)
		}
	}
	return nil
}
