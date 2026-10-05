package topology

import (
	"context"
	"fmt"
	"io"
	"net"
	"os/exec"
	"sync"
	"time"

	"github.com/nudanos/distro/internal/boottest"
)

// VM is a running router.
type VM struct {
	Spec    VMSpec
	Console *boottest.Console
	stop    func(grace time.Duration)
	once    sync.Once
}

// NewVM wraps a console and a stop function as a VM (for tests and for
// routers started some other way).
func NewVM(spec VMSpec, c *boottest.Console, stop func(grace time.Duration)) *VM {
	return &VM{Spec: spec, Console: c, stop: stop}
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
	exited := make(chan struct{})
	var waitErr error
	go func() { waitErr = cmd.Wait(); close(exited) }()
	var conn net.Conn
	stop := func(grace time.Duration) {
		select {
		case <-exited:
		case <-time.After(grace):
			cancel()
			<-exited
		}
		cancel()
		if conn != nil {
			conn.Close()
		}
	}
	deadline := time.Now().Add(30 * time.Second)
	for {
		select {
		case <-exited:
			cancel()
			return nil, fmt.Errorf("%s: qemu exited before its console answered: %v", spec.Name, waitErr)
		default:
		}
		c, err := net.Dial("unix", spec.SerialSock)
		if err == nil {
			conn = c
			return NewVM(spec, boottest.NewConsole(c, log), stop), nil
		}
		if time.Now().After(deadline) {
			stop(0)
			return nil, fmt.Errorf("%s: serial console %s: %w", spec.Name, spec.SerialSock, err)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// Stop waits up to grace for QEMU to exit by itself (after a poweroff), then
// kills it. Only the first call does anything.
func (vm *VM) Stop(grace time.Duration) {
	vm.once.Do(func() {
		if vm.stop != nil {
			vm.stop(grace)
		}
	})
}

// Overlay creates a copy-on-write disk at path backed by base.
func Overlay(base, path string) error {
	out, err := exec.Command("qemu-img", "create", "-q", "-f", "qcow2", "-b", base, "-F", "qcow2", path).CombinedOutput()
	if err != nil {
		return fmt.Errorf("qemu-img create %s: %v: %s", path, err, out)
	}
	return nil
}
