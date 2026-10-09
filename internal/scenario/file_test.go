package scenario

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nudanos/distro/internal/topology"
)

func write(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "scenario.yaml")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

const ospfLike = `
name: two
gating: true
routers:
  R2:
    config:
      - set interfaces dataplane ${R2:R1} address 10.0.12.2/24
  R1:
    config:
      - set interfaces dataplane ${R1:R2} address 10.0.12.1/24
links:
  - [R1, R2]
show:
  - show ip route
checks:
  - name: R1 sees R2
    router: R1
    op: {command: "ping 10.0.12.2 count 1 interface ${R1:R2}", want: " 0% packet loss"}
  - router: R2
    timeout: 5m
    action: {configure: ["set interfaces dataplane ${R2:R1} disable"], commit: true}
`

func TestLoadOrdersRoutersAndLinks(t *testing.T) {
	f, err := Load(write(t, ospfLike))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(f.RouterOrder, ",") != "R2,R1" {
		t.Errorf("RouterOrder = %v, want file order R2,R1", f.RouterOrder)
	}
	if len(f.Links) != 1 || f.Links[0] != (topology.Link{A: "R1", B: "R2"}) {
		t.Errorf("Links = %v", f.Links)
	}
	if !f.Gating || f.Name != "two" || len(f.Show) != 1 || len(f.Checks) != 2 {
		t.Errorf("parsed %+v", f)
	}
}

func TestLoadDefaultsTimeoutTo2m(t *testing.T) {
	f, err := Load(write(t, ospfLike))
	if err != nil {
		t.Fatal(err)
	}
	if f.Checks[0].Timeout != 2*time.Minute || f.Checks[1].Timeout != 5*time.Minute {
		t.Errorf("timeouts = %s, %s", f.Checks[0].Timeout, f.Checks[1].Timeout)
	}
}

func TestLoadRejectsCheckWithTwoKinds(t *testing.T) {
	_, err := Load(write(t, strings.Replace(ospfLike, `    timeout: 5m`, "    timeout: 5m\n    go: snmp-counter-moves", 1)))
	if err == nil || !strings.Contains(err.Error(), "exactly one") {
		t.Errorf("err = %v, want 'exactly one'", err)
	}
}

func TestLoadRejectsCheckWithNoKind(t *testing.T) {
	_, err := Load(write(t, ospfLike+"  - name: empty\n    router: R1\n"))
	if err == nil || !strings.Contains(err.Error(), "exactly one") {
		t.Errorf("err = %v, want 'exactly one'", err)
	}
}

func TestLoadRejectsUnknownRouterInCheck(t *testing.T) {
	_, err := Load(write(t, strings.Replace(ospfLike, "  - router: R2\n", "  - router: R9\n", 1)))
	if err == nil || !strings.Contains(err.Error(), "R9") {
		t.Errorf("err = %v, want it to name R9", err)
	}
}

func TestExpandReplacesInterfacePlaceholders(t *testing.T) {
	f, err := Load(write(t, ospfLike))
	if err != nil {
		t.Fatal(err)
	}
	specs, err := topology.Plan(f.RouterOrder, f.Links, 1024, false, topology.AllocatePorts)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Expand(specs); err != nil {
		t.Fatal(err)
	}
	if got := f.Routers["R2"].Config[0]; got != "set interfaces dataplane dp0s3 address 10.0.12.2/24" {
		t.Errorf("R2 config = %q", got)
	}
	if got := f.Checks[0].Op.Command; got != "ping 10.0.12.2 count 1 interface dp0s3" {
		t.Errorf("op command = %q", got)
	}
	if got := f.Checks[1].Action.Configure[0]; got != "set interfaces dataplane dp0s3 disable" {
		t.Errorf("action = %q", got)
	}
}

func TestExpandRejectsPlaceholderWithoutLink(t *testing.T) {
	f, err := Load(write(t, strings.Replace(ospfLike, "${R2:R1} address", "${R2:R3} address", 1)))
	if err != nil {
		t.Fatal(err)
	}
	specs, _ := topology.Plan(f.RouterOrder, f.Links, 1024, false, topology.AllocatePorts)
	if err := f.Expand(specs); err == nil || !strings.Contains(err.Error(), "R3") {
		t.Errorf("err = %v, want it to name R3", err)
	}
}

// Every committed scenario parses, its links are a valid topology and its
// interface placeholders resolve: a typo fails here, not after a VM run.
func TestCommittedScenariosLoad(t *testing.T) {
	files, _ := filepath.Glob("../../tests/scenarios/*/scenario.yaml")
	if len(files) == 0 {
		t.Fatal("no scenario files found")
	}
	for _, p := range files {
		f, err := Load(p)
		if err != nil {
			t.Errorf("%s: %v", p, err)
			continue
		}
		if f.Name != filepath.Base(filepath.Dir(p)) {
			t.Errorf("%s: name %q does not match its directory", p, f.Name)
		}
		specs, err := topology.Plan(f.RouterOrder, f.Links, 1024, false, topology.AllocatePorts)
		if err != nil {
			t.Errorf("%s: %v", p, err)
			continue
		}
		if err := f.Expand(specs); err != nil {
			t.Errorf("%s: %v", p, err)
		}
	}
}

func TestLoadRejectsSecondCaptureShow(t *testing.T) {
	_, err := Load(write(t, ospfLike+"  - capture_show: true\n  - capture_show: true\n"))
	if err == nil || !strings.Contains(err.Error(), "captured once") {
		t.Errorf("err = %v, want 'captured once'", err)
	}
}
