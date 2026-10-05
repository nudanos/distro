package topology

import (
	"net"
	"strings"
	"testing"
)

// seq hands out ports 40000, 40001, ... so tests can predict them.
func seq() func(int) ([]int, error) {
	next := 40000
	return func(n int) ([]int, error) {
		out := make([]int, n)
		for i := range out {
			out[i] = next
			next++
		}
		return out, nil
	}
}

func plan3(t *testing.T) []VMSpec {
	t.Helper()
	specs, err := Plan([]string{"R1", "R2", "R3"}, []Link{{"R1", "R2"}, {"R2", "R3"}}, 1024, false, seq())
	if err != nil {
		t.Fatal(err)
	}
	return specs
}

func TestPlanAssignsInterfacesInLinkOrder(t *testing.T) {
	specs := plan3(t)
	for _, c := range []struct{ router, peer, want string }{
		{"R1", "R2", "dp0s3"}, {"R2", "R1", "dp0s3"}, {"R2", "R3", "dp0s4"}, {"R3", "R2", "dp0s3"},
	} {
		got, err := Interface(specs, c.router, c.peer)
		if err != nil || got != c.want {
			t.Errorf("Interface(%s, %s) = %q, %v; want %q", c.router, c.peer, got, err, c.want)
		}
	}
	if _, err := Interface(specs, "R1", "R3"); err == nil {
		t.Error("Interface(R1, R3) with no link: want an error")
	}
}

// QEMU waits for its serial console before creating networks, so the side
// of a link that listens must be the router started first (list order).
func TestPlanListenSideIsTheEarlierRouter(t *testing.T) {
	specs, err := Plan([]string{"R1", "R2"}, []Link{{"R2", "R1"}}, 1024, false, seq())
	if err != nil {
		t.Fatal(err)
	}
	if !specs[0].Data[0].Listen || specs[1].Data[0].Listen {
		t.Errorf("R1 must listen and R2 connect: %+v / %+v", specs[0].Data[0], specs[1].Data[0])
	}
	if specs[0].Data[0].Port != specs[1].Data[0].Port {
		t.Error("both ends of a link must use the same port")
	}
}

func TestPlanRejects(t *testing.T) {
	five := []string{"R1", "R2", "R3", "R4", "R5"}
	for name, c := range map[string]struct {
		routers []string
		links   []Link
		want    string
	}{
		"too many routers": {five, nil, "at most 4"},
		"unknown router":   {[]string{"R1"}, []Link{{"R1", "R9"}}, "R9"},
		"self link":        {[]string{"R1"}, []Link{{"R1", "R1"}}, "R1"},
		"duplicate link":   {[]string{"R1", "R2"}, []Link{{"R1", "R2"}, {"R2", "R1"}}, "duplicate"},
	} {
		_, err := Plan(c.routers, c.links, 1024, false, seq())
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want it to mention %q", name, err, c.want)
		}
	}
}

func TestQEMUArgs(t *testing.T) {
	specs := plan3(t)
	specs[0].Disk = "/work/r1.qcow2"
	a := strings.Join(specs[0].QEMUArgs(), " ")
	b := strings.Join(specs[1].QEMUArgs(), " ")
	link := specs[0].Data[0].Port
	m := specs[0].Mgmt
	for _, want := range []string{
		"-m 1024", "-smp 2", "-cpu max", "-display none",
		"-serial unix:" + specs[0].SerialSock + ",server=on,wait=on",
		"-drive file=/work/r1.qcow2,if=virtio,format=qcow2",
		"socket,id=d3,listen=127.0.0.1:" + itoa(link),
		"virtio-net-pci,netdev=d3,addr=03,mac=52:54:00:00:00:03",
		"user,id=m,hostfwd=tcp:127.0.0.1:" + itoa(m.SSH) + "-10.0.2.15:22,hostfwd=tcp:127.0.0.1:" + itoa(m.HTTPS) + "-10.0.2.15:443,hostfwd=udp:127.0.0.1:" + itoa(m.SNMP) + "-10.0.2.15:161",
		"virtio-net-pci,netdev=m,addr=0a,mac=52:54:00:00:00:0a",
	} {
		if !strings.Contains(a, want) {
			t.Errorf("R1 args lack %q:\n%s", want, a)
		}
	}
	for _, want := range []string{"socket,id=d3,connect=127.0.0.1:" + itoa(link), "mac=52:54:00:00:01:03"} {
		if !strings.Contains(b, want) {
			t.Errorf("R2 args lack %q:\n%s", want, b)
		}
	}
	for _, absent := range []string{"-enable-kvm", "-cdrom"} {
		if strings.Contains(a, absent) {
			t.Errorf("R1 args contain %q", absent)
		}
	}
	specs[0].KVM, specs[0].ISO = true, "/iso/x.iso"
	a = strings.Join(specs[0].QEMUArgs(), " ")
	for _, want := range []string{"-enable-kvm -cpu host", "-cdrom /iso/x.iso -boot d"} {
		if !strings.Contains(a, want) {
			t.Errorf("KVM/ISO args lack %q:\n%s", want, a)
		}
	}
}

func TestAllocatePortsSkipsBusyPort(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	busy := l.Addr().(*net.TCPAddr).Port
	ports, err := AllocatePorts(20)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[int]bool{}
	for _, p := range ports {
		if p == busy {
			t.Errorf("allocated the busy port %d", p)
		}
		if seen[p] {
			t.Errorf("port %d allocated twice", p)
		}
		seen[p] = true
	}
}
