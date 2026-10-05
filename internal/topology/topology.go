// Package topology plans and starts the QEMU routers of a layer 4 scenario
// (plan 4): up to four VMs joined by point-to-point socket links, each with
// a serial console and a management NIC whose SSH, HTTPS and SNMP ports are
// forwarded to the host.
package topology

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
)

// MaxRouters keeps a topology inside the development Mac's 8 GB.
const MaxRouters = 4

// MgmtPCI is the management NIC's PCI slot: dp0s10 on both NuDanOS and 2105.
const MgmtPCI = 0x0a

// firstDataPCI is the first data NIC's slot (dp0s3).
const firstDataPCI = 3

// Link joins two routers.
type Link struct{ A, B string }

// Ports are host ports forwarded to the management address 10.0.2.15.
type Ports struct{ SSH, HTTPS, SNMP int }

// DataNIC is one end of a link. The side that listens is the router started
// first: QEMU waits for its serial console before it creates networks.
type DataNIC struct {
	PCI    int
	Peer   string
	Listen bool
	Port   int
}

// VMSpec is everything needed to start one router.
type VMSpec struct {
	Name       string
	Index      int // position in the topology; part of every NIC's MAC
	MemMB      int
	Disk       string
	ISO        string
	KVM        bool
	Data       []DataNIC
	Mgmt       Ports
	SerialSock string
}

var itoa = strconv.Itoa

// AllocatePorts returns n distinct free TCP ports on 127.0.0.1. Each is
// probed by listening on it; ports are held until all are found, so the
// same port is never returned twice.
func AllocatePorts(n int) ([]int, error) {
	var held []net.Listener
	defer func() {
		for _, l := range held {
			l.Close()
		}
	}()
	ports := make([]int, 0, n)
	for len(ports) < n {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return nil, fmt.Errorf("allocating ports: %w", err)
		}
		held = append(held, l)
		ports = append(ports, l.Addr().(*net.TCPAddr).Port)
	}
	return ports, nil
}

// Plan lays out routers and links: NIC slots in link order, one port per
// link, three management ports per router, and a serial socket per router.
func Plan(routers []string, links []Link, memMB int, kvm bool, alloc func(n int) ([]int, error)) ([]VMSpec, error) {
	if len(routers) > MaxRouters {
		return nil, fmt.Errorf("a topology has at most %d routers, got %d", MaxRouters, len(routers))
	}
	index := map[string]int{}
	for i, r := range routers {
		index[r] = i
	}
	seen := map[[2]string]bool{}
	for _, l := range links {
		for _, end := range []string{l.A, l.B} {
			if _, ok := index[end]; !ok {
				return nil, fmt.Errorf("link %s-%s names unknown router %s", l.A, l.B, end)
			}
		}
		if l.A == l.B {
			return nil, fmt.Errorf("link from %s to itself", l.A)
		}
		key := [2]string{l.A, l.B}
		if index[l.A] > index[l.B] {
			key = [2]string{l.B, l.A}
		}
		if seen[key] {
			return nil, fmt.Errorf("duplicate link %s-%s", l.A, l.B)
		}
		seen[key] = true
	}
	ports, err := alloc(len(links) + 3*len(routers))
	if err != nil {
		return nil, err
	}
	specs := make([]VMSpec, len(routers))
	for i, r := range routers {
		specs[i] = VMSpec{Name: r, Index: i, MemMB: memMB, KVM: kvm,
			Mgmt:       Ports{SSH: ports[len(links)+3*i], HTTPS: ports[len(links)+3*i+1], SNMP: ports[len(links)+3*i+2]},
			SerialSock: filepath.Join(os.TempDir(), fmt.Sprintf("nudanos-scenario-%d-%s.sock", os.Getpid(), r))}
	}
	for li, l := range links {
		a, b := index[l.A], index[l.B]
		first := a
		if b < a {
			first = b
		}
		for _, end := range []struct{ me, peer int }{{a, b}, {b, a}} {
			s := &specs[end.me]
			s.Data = append(s.Data, DataNIC{PCI: firstDataPCI + len(s.Data), Peer: routers[end.peer],
				Listen: end.me == first, Port: ports[li]})
		}
	}
	return specs, nil
}

func (v VMSpec) mac(pci int) string {
	return fmt.Sprintf("52:54:00:00:%02x:%02x", v.Index, pci)
}

// QEMUArgs is the qemu-system-x86_64 command line for this router.
func (v VMSpec) QEMUArgs() []string {
	args := []string{"-machine", "pc", "-m", itoa(v.MemMB), "-smp", "2", "-display", "none",
		"-serial", "unix:" + v.SerialSock + ",server=on,wait=on"}
	if v.KVM {
		args = append(args, "-enable-kvm", "-cpu", "host")
	} else {
		args = append(args, "-cpu", "max")
	}
	if v.Disk != "" {
		args = append(args, "-drive", "file="+v.Disk+",if=virtio,format=qcow2")
	}
	for _, d := range v.Data {
		mode := "connect"
		if d.Listen {
			mode = "listen"
		}
		id := "d" + itoa(d.PCI)
		args = append(args, "-netdev", fmt.Sprintf("socket,id=%s,%s=127.0.0.1:%d", id, mode, d.Port),
			"-device", fmt.Sprintf("virtio-net-pci,netdev=%s,addr=%02x,mac=%s", id, d.PCI, v.mac(d.PCI)))
	}
	args = append(args, "-netdev", fmt.Sprintf("user,id=m,hostfwd=tcp:127.0.0.1:%d-10.0.2.15:22,hostfwd=tcp:127.0.0.1:%d-10.0.2.15:443,hostfwd=udp:127.0.0.1:%d-10.0.2.15:161",
		v.Mgmt.SSH, v.Mgmt.HTTPS, v.Mgmt.SNMP),
		"-device", fmt.Sprintf("virtio-net-pci,netdev=m,addr=%02x,mac=%s", MgmtPCI, v.mac(MgmtPCI)))
	if v.ISO != "" {
		args = append(args, "-cdrom", v.ISO, "-boot", "d")
	}
	return args
}

// Interface names router's NIC toward peer as both NuDanOS and 2105 do
// (PCI slot 3 on bus 0 is dp0s3).
func Interface(specs []VMSpec, router, peer string) (string, error) {
	for _, s := range specs {
		if s.Name != router {
			continue
		}
		for _, d := range s.Data {
			if d.Peer == peer {
				return "dp0s" + itoa(d.PCI), nil
			}
		}
		return "", fmt.Errorf("%s has no link to %s", router, peer)
	}
	return "", fmt.Errorf("no router %s", router)
}
