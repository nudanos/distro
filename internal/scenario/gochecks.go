package scenario

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"time"
)

func init() {
	RegisterGo("snmp-counter-moves", snmpCounterMoves)
}

// Numeric OIDs: Debian's snmp package ships no MIB files.
const (
	oidIfDescr    = ".1.3.6.1.2.1.2.2.1.2"
	oidIfInOctets = ".1.3.6.1.2.1.2.2.1.10"
)

var (
	ifDescrLine = regexp.MustCompile(`\.(\d+) = STRING: "?dp0s10"?`)
	counter     = regexp.MustCompile(`= Counter(?:32|64): (\d+)`)
)

func snmpGet(r *Routers, router, oid string) (string, error) {
	out, err := exec.Command("snmpwalk", "-v2c", "-c", "nudanos-ro",
		fmt.Sprintf("udp:127.0.0.1:%d", r.Ports[router].SNMP), oid).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("snmpwalk %s: %v: %s", oid, err, out)
	}
	return string(out), nil
}

// snmpCounterMoves reads R1's management interface (dp0s10) inbound octet
// counter over SNMPv2c, makes traffic (an SSH login), and requires the
// counter to have grown.
func snmpCounterMoves(ctx context.Context, r *Routers) error {
	descr, err := snmpGet(r, "R1", oidIfDescr)
	if err != nil {
		return err
	}
	m := ifDescrLine.FindStringSubmatch(descr)
	if m == nil {
		return fmt.Errorf("ifDescr has no dp0s10:\n%s", descr)
	}
	read := func() (uint64, error) {
		out, err := snmpGet(r, "R1", oidIfInOctets+"."+m[1])
		if err != nil {
			return 0, err
		}
		c := counter.FindStringSubmatch(out)
		if c == nil {
			return 0, fmt.Errorf("no counter in %q", out)
		}
		return strconv.ParseUint(c[1], 10, 64)
	}
	before, err := read()
	if err != nil {
		return err
	}
	return retry(ctx, r.T(time.Minute), func() error {
		loginOnce(r, Check{Router: "R1", Login: &LoginCheck{User: r.Admin, Password: r.Password, Expect: "ok"}})
		after, err := read()
		if err != nil {
			return err
		}
		if after <= before {
			return fmt.Errorf("dp0s10 ifInOctets did not grow: %d then %d", before, after)
		}
		return nil
	})
}
