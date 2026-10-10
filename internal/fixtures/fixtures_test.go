package fixtures

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/nudanos/distro/internal/boottest"
	"github.com/nudanos/distro/internal/topology"
)

func identity(d time.Duration) time.Duration { return d }

const paths = "# comment\nsecurity firewall\ninterfaces dataplane * speed !auto\n"

// fakeRouter answers like a NuDanOS console: a base64 heredoc lands in a
// file whose md5sum it reports, configure/load/commit/exit, the path list,
// and "show configuration commands" from show.
func fakeRouter(t *testing.T, show string) *topology.VM {
	t.Helper()
	a, b := net.Pipe()
	t.Cleanup(func() { a.Close(); b.Close() })
	go func() {
		op, cfg := "\r\nvyatta@r1:~$ ", "\r\n[edit]\r\nvyatta@r1# "
		var heredoc *strings.Builder
		var file []byte
		mode := "op"
		buf := make([]byte, 0, 4096)
		rd := make([]byte, 4096)
		for {
			n, err := b.Read(rd)
			if err != nil {
				return
			}
			buf = append(buf, rd[:n]...)
			for {
				i := bytes.IndexByte(buf, '\r')
				if i < 0 {
					break
				}
				line := string(buf[:i])
				buf = buf[i+1:]
				reply := line
				switch {
				case heredoc != nil && line == "NUDANOS_EOF":
					file, _ = base64.StdEncoding.DecodeString(heredoc.String())
					heredoc = nil
					reply += op
				case heredoc != nil:
					heredoc.WriteString(line)
					reply += "\r\n> "
				case strings.HasPrefix(line, "base64 -d"):
					heredoc = &strings.Builder{}
					reply += "\r\n> "
				case strings.HasPrefix(line, "md5sum"):
					reply += fmt.Sprintf("\r\n%x  /tmp/ref.boot", md5.Sum(file)) + op
				case strings.HasPrefix(line, "cat /usr/share/vyatta-kernel-forwarding/dpdk-only-paths"):
					reply += "\r\n" + strings.ReplaceAll(paths, "\n", "\r\n") + strings.TrimPrefix(op, "\r\n")
				case line == "show configuration commands":
					reply += "\r\n" + strings.ReplaceAll(show, "\n", "\r\n") + strings.TrimPrefix(op, "\r\n")
				case line == "configure":
					mode = "cfg"
					reply += cfg
				case mode == "cfg" && strings.HasPrefix(line, "exit"):
					mode = "op"
					reply += op
				case mode == "cfg":
					reply += cfg
				default:
					reply += op
				}
				io.WriteString(b, reply)
			}
		}
	}()
	return topology.NewVM(topology.VMSpec{Name: "R1"}, boottest.NewConsole(a, &bytes.Buffer{}), func(time.Duration) {})
}

const refCommands = `set interfaces dataplane dp0s10 address '10.0.2.15/24'
set interfaces dataplane dp0s3 address '10.0.0.1/24'
set interfaces dataplane dp0s3 speed '100m'
set interfaces dataplane dp0s4 speed 'auto'
set protocols static route 10.9.0.0/16 next-hop '10.0.0.2'
set security firewall name OUT default-action 'drop'
set system host-name 'r1'
set system login user tmpuser level 'admin'
`

var ref = Ref{Name: "sampler/firewall", ConfigBoot: "system {\n\thost-name r1\n}\n", Commands: refCommands}

// The set-aside lines (firewall, a speed that is not auto) are absent on
// NuDanOS by design.
func TestCheckPassesWhenOnlySetAsideLinesDiffer(t *testing.T) {
	vm := fakeRouter(t, `set interfaces dataplane dp0s10 address '10.0.2.15/24'
set interfaces dataplane dp0s3 address '10.0.0.1/24'
set interfaces dataplane dp0s4 speed 'auto'
set protocols static route 10.9.0.0/16 next-hop '10.0.0.2'
set system host-name 'r1'
set system login user tmpuser level 'admin'
`)
	if f := Check(context.Background(), vm, []Ref{ref}, identity); len(f) != 0 {
		t.Fatalf("failures: %+v", f)
	}
}

func TestCheckReportsUnexpectedDifference(t *testing.T) {
	vm := fakeRouter(t, `set interfaces dataplane dp0s10 address '10.0.2.15/24'
set interfaces dataplane dp0s3 address '10.0.0.1/24'
set interfaces dataplane dp0s4 speed 'auto'
set system host-name 'r1'
set system login user tmpuser level 'admin'
`)
	f := Check(context.Background(), vm, []Ref{ref}, identity)
	if len(f) != 1 || f[0].Name != "sampler/firewall" || !strings.Contains(f[0].Detail, "set protocols static route 10.9.0.0/16 next-hop '10.0.0.2'") {
		t.Fatalf("failures = %+v; want the missing static route named", f)
	}
}

// The management address, host name and console users are the base
// configuration, which differs between the 2105 and NuDanOS images.
func TestCheckIgnoresBaseConfigLines(t *testing.T) {
	vm := fakeRouter(t, `set interfaces dataplane dp0s10 address dhcp
set interfaces dataplane dp0s3 address '10.0.0.1/24'
set interfaces dataplane dp0s4 speed 'auto'
set protocols static route 10.9.0.0/16 next-hop '10.0.0.2'
set system host-name 'nudanos'
set system login user nudanos level 'admin'
`)
	if f := Check(context.Background(), vm, []Ref{ref}, identity); len(f) != 0 {
		t.Fatalf("failures: %+v", f)
	}
}

// The reboot test boots a 2105 configuration with the administrator added,
// so it can log in afterwards.
func TestMergeBootAddsAdministrator(t *testing.T) {
	boot := "interfaces {\n\tdataplane dp0s3 {\n\t\tcpu-affinity 1\n\t}\n}\nsystem {\n\thost-name r1\n\tlogin {\n\t\tuser tmpuser {\n\t\t\tlevel admin\n\t\t}\n\t}\n}\n"
	got := mergeBoot(boot, adminBoot("nudanos", "pw"))
	want := "interfaces {\n\tdataplane dp0s3 {\n\t\tcpu-affinity 1\n\t}\n}\nsystem {\n\thost-name r1\n\tlogin {\n\t\tuser tmpuser {\n\t\t\tlevel admin\n\t\t}\n\t\tuser nudanos {\n\t\t\tauthentication {\n\t\t\t\tplaintext-password \"pw\"\n\t\t\t}\n\t\t\tlevel admin\n\t\t}\n\t}\n}\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}
