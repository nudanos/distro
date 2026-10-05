// Command boottest boots the NuDanOS ISO in QEMU and runs the layer 3 test
// over the serial console (spec 8.3). It runs inside nudanos/tester:trixie.
package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/nudanos/distro/internal/boottest"
)

func errorf(format string, a ...any) error { return fmt.Errorf(format, a...) }

func main() {
	iso := flag.String("iso", "", "ISO to boot")
	disk := flag.String("disk", "/work/disk.qcow2", "virtual disk to install to")
	kvm := flag.Bool("kvm", false, "use KVM (otherwise TCG emulation)")
	firmware := flag.String("firmware", "bios", "bios or efi")
	smoke := flag.Bool("smoke", false, "only boot to the login prompt (UEFI check)")
	logPath := flag.String("log", "/work/serial.log", "serial transcript")
	flag.Parse()
	scale := 1.0
	if !*kvm {
		scale = 6 // TCG is 5-20x slower; timeouts scale with it
	}
	t := func(d time.Duration) time.Duration { return time.Duration(float64(d) * scale) }
	if err := runAll(*iso, *disk, *kvm, *firmware, *smoke, *logPath, t); err != nil {
		fmt.Fprintln(os.Stderr, "boottest: FAIL:", err)
		os.Exit(1)
	}
	fmt.Println("boottest: OK")
}

func runAll(iso, disk string, kvm bool, firmware string, smoke bool, logPath string, t func(time.Duration) time.Duration) error {
	log, err := os.Create(logPath)
	if err != nil {
		return err
	}
	defer log.Close()
	if err := exec.Command("qemu-img", "create", "-q", "-f", "qcow2", disk, "8G").Run(); err != nil {
		return fmt.Errorf("qemu-img: %w", err)
	}
	// Phase 1: the ISO.
	c, stop, err := boot(iso, disk, kvm, firmware, log)
	if err != nil {
		return err
	}
	if smoke {
		_, err := c.Expect(boottest.LoginPrompt, t(20*time.Minute))
		stop(0)
		return err
	}
	if err := liveSteps(c, t); err != nil {
		stop(0)
		return fmt.Errorf("live: %w", err)
	}
	if err := boottest.InstallImage(c, adminUser, testPassword, t); err != nil {
		stop(0)
		return fmt.Errorf("install image: %w", err)
	}
	if err := boottest.Halt(c, t); err != nil {
		stop(0)
		return fmt.Errorf("poweroff after install: %w", err)
	}
	stop(t(5 * time.Minute)) // let QEMU exit by itself and flush the disk
	// Phase 2: the installed disk only.
	c, stop, err = boot("", disk, kvm, firmware, log)
	if err != nil {
		return err
	}
	if err := diskSteps(c, t); err != nil {
		stop(0)
		return fmt.Errorf("installed: %w", err)
	}
	boottest.Halt(c, t)
	stop(t(5 * time.Minute))
	return nil
}

// boot starts QEMU with its serial console on a unix socket and connects to it.
// stop(grace) waits up to grace for QEMU to exit on its own, then kills it; it
// is always called, so a hung boot never outlives the test.
func boot(iso, disk string, kvm bool, firmware string, log *os.File) (*boottest.Console, func(time.Duration), error) {
	sock := filepath.Join(os.TempDir(), fmt.Sprintf("serial-%d.sock", time.Now().UnixNano()))
	args := []string{"-machine", "pc", "-m", "2048", "-smp", "2", "-display", "none",
		"-serial", "unix:" + sock + ",server=on,wait=on",
		"-drive", "file=" + disk + ",if=virtio,format=qcow2",
		"-netdev", "user,id=n0", "-device", "virtio-net-pci,netdev=n0,addr=03"}
	if kvm {
		args = append(args, "-enable-kvm", "-cpu", "host")
	}
	if firmware == "efi" {
		args = append(args, "-bios", "/usr/share/ovmf/OVMF.fd")
	}
	if iso != "" {
		args = append(args, "-cdrom", iso, "-boot", "d")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, "qemu-system-x86_64", args...)
	cmd.Stdout, cmd.Stderr = log, log
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, nil, fmt.Errorf("qemu: %w", err)
	}
	exited := make(chan struct{})
	go func() { cmd.Wait(); close(exited) }()
	kill := func(grace time.Duration) {
		select {
		case <-exited:
		case <-time.After(grace):
			cancel()
			<-exited
		}
		cancel()
	}
	var conn net.Conn
	var err error
	for i := 0; i < 100; i++ {
		if conn, err = net.Dial("unix", sock); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err != nil {
		kill(0)
		return nil, nil, fmt.Errorf("serial socket: %w", err)
	}
	return boottest.NewConsole(conn, log), func(grace time.Duration) { kill(grace); conn.Close() }, nil
}
