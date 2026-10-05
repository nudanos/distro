package scenario

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nudanos/distro/internal/topology"
)

// fakeSNMP answers ifDescr with dp0s10 at index 4 and ifInOctets.4 with a
// value that grows by 1000 on every call.
func fakeSNMP(t *testing.T, grow bool) {
	t.Helper()
	bin := t.TempDir()
	count := filepath.Join(bin, "count")
	os.WriteFile(count, []byte("5000"), 0o644)
	step := "1000"
	if !grow {
		step = "0"
	}
	script := `#!/bin/sh
for a; do last="$a"; done
case "$last" in
  *1.3.6.1.2.1.2.2.1.2) echo 'iso.3.6.1.2.1.2.2.1.2.1 = STRING: "lo"'; echo 'iso.3.6.1.2.1.2.2.1.2.4 = STRING: "dp0s10"' ;;
  *1.3.6.1.2.1.2.2.1.10.4) n=$(cat ` + count + `); echo "iso.3.6.1.2.1.2.2.1.10.4 = Counter32: $n"; echo $((n + ` + step + `)) > ` + count + ` ;;
esac
`
	os.WriteFile(filepath.Join(bin, "snmpwalk"), []byte(script), 0o755)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestSNMPCounterMoves(t *testing.T) {
	fakeSNMP(t, true)
	rs := routers(topology.Ports{SSH: sshServer(t), SNMP: 16161})
	if err := snmpCounterMoves(context.Background(), rs); err != nil {
		t.Fatal(err)
	}
}

func TestSNMPCounterMovesFailsWhenStuck(t *testing.T) {
	fakeSNMP(t, false)
	opInterval = 10 * time.Millisecond
	rs := routers(topology.Ports{SSH: sshServer(t), SNMP: 16161})
	rs.T = func(d time.Duration) time.Duration { return d / 300 }
	if err := snmpCounterMoves(context.Background(), rs); err == nil {
		t.Error("a counter that never moves passed")
	}
}
