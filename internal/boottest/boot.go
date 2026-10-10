package boottest

import (
	"fmt"
	"regexp"
	"time"
)

// WaitInterval is how long WaitBooted waits between questions.
var WaitInterval = 10 * time.Second

var (
	stillBooting = regexp.MustCompile(`\bactivating\b`) // the console may append escape sequences
	// output through the next shell or login prompt (First returns the match)
	throughOpPrompt    = regexp.MustCompile(`(?s)^.*?:~\$ $`)
	throughLoginPrompt = regexp.MustCompile(`(?s)^.*?login: $`)
)

// WaitBooted waits until system-configure, which commits the boot
// configuration after getty has started, is no longer running: configuring
// before then races the boot commit ("Commit already in progress"). The
// boot commit ends the admin's session on NuDanOS; a login prompt where the
// shell was means log in again. 2105's admin shell has no systemctl, and
// that counts as booted.
func WaitBooted(c *Console, user, password string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		err := waitBootedOnce(c, user, password, deadline)
		if err == nil {
			return nil
		}
		if time.Now().Add(WaitInterval).After(deadline) {
			return err
		}
		time.Sleep(WaitInterval)
	}
}

func waitBootedOnce(c *Console, user, password string, deadline time.Time) error {
	c.Discard()
	if err := c.Send("systemctl show -p ActiveState --value system-configure.service"); err != nil {
		return err
	}
	i, out, err := c.First([]*regexp.Regexp{throughOpPrompt, throughLoginPrompt}, time.Minute)
	if err != nil {
		return err
	}
	if i == 1 {
		if err := Login(c, user, password, LoginPrompt, time.Until(deadline)); err != nil {
			return err
		}
		return fmt.Errorf("the boot commit ended the session; logged in again")
	}
	if stillBooting.MatchString(out) {
		return fmt.Errorf("boot configuration still being committed (system-configure activating)")
	}
	return nil
}
