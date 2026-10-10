package fixtures

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/nudanos/distro/internal/boottest"
	"github.com/nudanos/distro/internal/topology"
)

// RebootTest boots vm from ref's config.boot (the cpu-affinity sampler:
// a 2105 configuration holding a DPDK-only setting) and requires that the
// set-aside hook removed the setting at boot, the rest loaded, the removed
// node was kept in /config/config.boot.dpdk-only, and the login banner says
// so. user/password are added to the configuration (2105's captures mask
// passwords) so the router can be logged into afterwards.
func RebootTest(ctx context.Context, vm *topology.VM, ref Ref, user, password string, t func(time.Duration) time.Duration) error {
	c := vm.Console
	sudo := "echo '" + password + "' | sudo -S "
	if err := copyFile(c, "/tmp/reboot.boot", mergeBoot(ref.ConfigBoot, adminBoot(user, password)), t); err != nil {
		return fmt.Errorf("copying config.boot: %w", err)
	}
	if _, err := boottest.OpOutput(c, sudo+"rm -f /config/config.boot.dpdk-only /config/config.boot.2105-original", t(time.Minute)); err != nil {
		return err
	}
	if _, err := boottest.OpOutput(c, sudo+"cp /tmp/reboot.boot /config/config.boot", t(time.Minute)); err != nil {
		return err
	}
	c.Discard()
	if err := c.Send(sudo + "systemctl reboot"); err != nil {
		return err
	}
	if err := boottest.Login(c, user, password, boottest.LoginPrompt, t(30*time.Minute)); err != nil {
		return fmt.Errorf("after the reboot: %w", err)
	}
	if err := boottest.WaitBooted(c, user, password, t(20*time.Minute)); err != nil {
		return fmt.Errorf("after the reboot: %w", err)
	}
	// the notice depends on the boot commit's side file: log in once more
	c.Discard()
	if err := c.Send("exit"); err != nil {
		return err
	}
	banner, err := loginBanner(c, user, password, t)
	if err != nil {
		return err
	}
	list, err := boottest.OpOutput(c, "cat "+pathsFile, t(time.Minute))
	if err != nil {
		return err
	}
	got, err := boottest.OpOutput(c, "show configuration commands", t(2*time.Minute))
	if err != nil {
		return err
	}
	var problems []string
	pats := parsePatterns(list)
	g := lines(got, func(string) bool { return false })
	for l := range lines(ref.Commands, func(l string) bool { return setAside(l, pats) }) {
		if !g[l] {
			problems = append(problems, "missing after the reboot: "+l)
		}
	}
	if strings.Contains(got, "cpu-affinity") {
		problems = append(problems, "cpu-affinity is still configured")
	}
	if out, _ := boottest.OpOutput(c, "ls /config/config.boot.dpdk-only", t(time.Minute)); strings.Contains(out, "No such file") || !strings.Contains(out, "config.boot.dpdk-only") {
		problems = append(problems, "no /config/config.boot.dpdk-only: "+strings.TrimSpace(out))
	}
	if !strings.Contains(banner, "dpdk-only") {
		problems = append(problems, "the login banner does not mention dpdk-only:\n"+banner)
	}
	if len(problems) > 0 {
		return fmt.Errorf("%s", strings.Join(problems, "\n"))
	}
	return nil
}

var throughShell = regexp.MustCompile(`(?s)^.*?:~\$ $`)

// loginBanner logs in and returns what the router printed between the
// password and the shell prompt.
func loginBanner(c *boottest.Console, user, password string, t func(time.Duration) time.Duration) (string, error) {
	if _, _, err := c.FirstNudging([]*regexp.Regexp{boottest.LoginPrompt}, t(time.Minute), t(5*time.Minute)); err != nil {
		return "", err
	}
	if err := c.Send(user); err != nil {
		return "", err
	}
	if _, err := c.Expect(boottest.PassPrompt, t(time.Minute)); err != nil {
		return "", err
	}
	if err := c.Send(password); err != nil {
		return "", err
	}
	return c.Expect(throughShell, t(5*time.Minute))
}

// adminBoot is a configuration fragment adding an administrator.
func adminBoot(user, password string) string {
	return fmt.Sprintf("system {\n\tlogin {\n\t\tuser %s {\n\t\t\tauthentication {\n\t\t\t\tplaintext-password %q\n\t\t\t}\n\t\t\tlevel admin\n\t\t}\n\t}\n}\n", user, password)
}

// node is a config.boot node: a block (children) or a leaf/comment line.
type node struct {
	head     string
	block    bool
	children []*node
}

var quotedString = regexp.MustCompile(`"(?:[^"\\]|\\.)*"`)

func parseBoot(s string) *node {
	root := &node{block: true}
	stack := []*node{root}
	for _, l := range strings.Split(s, "\n") {
		t := strings.TrimSpace(l)
		if t == "" {
			continue
		}
		u := quotedString.ReplaceAllString(t, "Q")
		cur := stack[len(stack)-1]
		switch {
		case strings.HasPrefix(t, "/*"):
			cur.children = append(cur.children, &node{head: t})
		case strings.HasSuffix(u, "{"):
			n := &node{head: strings.TrimSpace(strings.TrimSuffix(t, "{")), block: true}
			cur.children = append(cur.children, n)
			stack = append(stack, n)
		case u == "}":
			if len(stack) > 1 {
				stack = stack[:len(stack)-1]
			}
		default:
			cur.children = append(cur.children, &node{head: t})
		}
	}
	return root
}

func merge(into, from *node) {
	for _, f := range from.children {
		var same *node
		if f.block {
			for _, n := range into.children {
				if n.block && n.head == f.head {
					same = n
					break
				}
			}
		}
		if same != nil {
			merge(same, f)
		} else {
			into.children = append(into.children, f)
		}
	}
}

func (n *node) write(b *strings.Builder, depth int) {
	for _, c := range n.children {
		ind := strings.Repeat("\t", depth)
		if c.block {
			b.WriteString(ind + c.head + " {\n")
			c.write(b, depth+1)
			b.WriteString(ind + "}\n")
		} else {
			b.WriteString(ind + c.head + "\n")
		}
	}
}

// mergeBoot merges the config.boot fragment add into boot.
func mergeBoot(boot, add string) string {
	root := parseBoot(boot)
	merge(root, parseBoot(add))
	var b strings.Builder
	root.write(&b, 0)
	return b.String()
}
