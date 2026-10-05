package topology

import (
	"context"
	"fmt"
	"io"
	"net"
	"os/exec"
	"time"

	"github.com/nudanos/distro/internal/boottest"
)

// VM is a running router.
type VM struct {
	Spec    VMSpec
	Console *boottest.Console
	conn    net.Conn
	exited  chan struct{}
	cancel  context.CancelFunc
}

// Start launches QEMU for spec and connects to its serial console. QEMU's
// output and the console transcript go to log. If QEMU exits before the
// console answers, Start says so instead of waiting out a timeout.
func Start(spec VMSpec, log io.Writer) (*VM, error) {
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, "qemu-system-x86_64", spec.QEMUArgs()...)
	cmd.Stdout, cmd.Stderr = log, log
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("%s: qemu: %w", spec.Name, err)
	}
	vm := &VM{Spec: spec, exited: make(chan struct{}), cancel: cancel}
	var waitErr error
	go func() { waitErr = cmd.Wait(); close(vm.exited) }()
	deadline := time.Now().Add(30 * time.Second)
	for {
		select {
		case <-vm.exited:
			cancel()
			return nil, fmt.Errorf("%s: qemu exited before its console answered: %v", spec.Name, waitErr)
		default:
		}
		conn, err := net.Dial("unix", spec.SerialSock)
		if err == nil {
			vm.conn = conn
			vm.Console = boottest.NewConsole(conn, log)
			return vm, nil
		}
		if time.Now().After(deadline) {
			vm.Stop(0)
			return nil, fmt.Errorf("%s: serial console %s: %w", spec.Name, spec.SerialSock, err)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// Stop waits up to grace for QEMU to exit by itself (after a poweroff), then
// kills it. It is safe to call more than once.
func (vm *VM) Stop(grace time.Duration) {
	select {
	case <-vm.exited:
	case <-time.After(grace):
		vm.cancel()
		<-vm.exited
	}
	vm.cancel()
	if vm.conn != nil {
		vm.conn.Close()
	}
}

// Overlay creates a copy-on-write disk at path backed by base.
func Overlay(base, path string) error {
	out, err := exec.Command("qemu-img", "create", "-q", "-f", "qcow2", "-b", base, "-F", "qcow2", path).CombinedOutput()
	if err != nil {
		return fmt.Errorf("qemu-img create %s: %v: %s", path, err, out)
	}
	return nil
}
