package scenario

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/nudanos/distro/internal/boottest"
	"github.com/nudanos/distro/internal/topology"
)

// SHA2105 is the only DANOS 2105 ISO the runner accepts (spec addendum §1).
const SHA2105 = "6d500d5d7ea69ebca0b7ada2bd74cec40c87780f41cefd9b9cc0e14fb81d9b51"

// The installed routers' administrator, as layer 3 creates it (test values).
const (
	adminUser     = "nudanos"
	adminPassword = "NuDanOS-test-1"
)

// installMemMB is the install VM's memory; installed routers run in 1 GB.
const installMemMB = 2048

// Image is what a scenario boots.
type Image struct {
	Name           string
	ISO            string
	MemMB          int
	User, Password string
	Live           bool // boot the ISO itself (2105) instead of an installed overlay
}

// NuDanOS boots overlays of a disk installed from iso.
func NuDanOS(iso string) Image {
	return Image{Name: "nudanos", ISO: iso, MemMB: 1024, User: adminUser, Password: adminPassword}
}

// Reference2105 boots the DANOS 2105 live ISO, which must be the one whose
// SHA-256 is SHA2105. It needs 1792 MB.
func Reference2105(iso string) (Image, error) {
	sum, err := fileSHA256(iso)
	if err != nil {
		return Image{}, err
	}
	if sum != SHA2105 {
		return Image{}, fmt.Errorf("%s: SHA-256 %s is not the DANOS 2105 ISO (%s)", iso, sum, SHA2105)
	}
	// Below about 1.75 GB the 2105 dataplane cannot reserve its memory and
	// leaves the NICs as kernel devices (ens10), measured 2026-10-05.
	return Image{Name: "2105", ISO: iso, MemMB: 1792, User: "tmpuser", Password: "tmppwd", Live: true}, nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// EnsureBase returns the installed base disk for a NuDanOS image, installing
// it first when this ISO has none yet ("" for a live image). The disk is
// named after the ISO's hash, so a new ISO gets a fresh install.
func EnsureBase(img Image, workDir string, kvm bool, log io.Writer, t func(time.Duration) time.Duration) (string, error) {
	if img.Live {
		return "", nil
	}
	sum, err := fileSHA256(img.ISO)
	if err != nil {
		return "", err
	}
	base := filepath.Join(workDir, "base-"+sum[:12]+".qcow2")
	if _, err := os.Stat(base); err == nil {
		return base, nil
	}
	tmp := base + ".installing"
	os.Remove(tmp)
	if out, err := exec.Command("qemu-img", "create", "-q", "-f", "qcow2", tmp, "8G").CombinedOutput(); err != nil {
		return "", fmt.Errorf("qemu-img create %s: %v: %s", tmp, err, out)
	}
	specs, err := topology.Plan([]string{"install"}, nil, installMemMB, kvm, topology.AllocatePorts)
	if err != nil {
		return "", err
	}
	specs[0].Disk, specs[0].ISO = tmp, img.ISO
	vm, err := topology.Start(specs[0], log)
	if err != nil {
		return "", err
	}
	defer vm.Stop(0)
	if err := boottest.Login(vm.Console, "vyatta", "vyatta", boottest.LoginPrompt, t(30*time.Minute)); err != nil {
		return "", fmt.Errorf("installing the base disk: live login: %w", err)
	}
	if err := boottest.InstallImage(vm.Console, adminUser, adminPassword, t); err != nil {
		return "", fmt.Errorf("installing the base disk: %w", err)
	}
	if err := boottest.Halt(vm.Console, t); err != nil {
		return "", fmt.Errorf("installing the base disk: poweroff: %w", err)
	}
	vm.Stop(t(5 * time.Minute)) // let QEMU flush the disk and exit by itself
	return base, os.Rename(tmp, base)
}

// BaseConfig is applied to every router before its scenario configuration:
// a host name, the management address QEMU user networking expects (DHCP is
// a 1.1 feature), and SSH.
func BaseConfig(router string) []string {
	return []string{
		"set system host-name " + strings.ToLower(router),
		"set interfaces dataplane dp0s10 address 10.0.2.15/24",
		"set service ssh",
	}
}
