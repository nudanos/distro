# Arch Linux test environment

Prepared 2026-11-06 as a disposable local test VM, using portable QEMU without host installation, WSL, host filesystem sharing, or host credentials.

## Provenance

The official cloud image and its SHA-256 file were downloaded over HTTPS from [Arch's package mirror](https://fastly.mirror.pkgbuild.com/images/latest/Arch-Linux-x86_64-cloudimg.qcow2), as identified by the [ArchWiki cloud image instructions](https://wiki.archlinux.org/title/Arch_Linux_on_a_VPS). The downloaded image SHA-256 matched the published checksum: `794eb4889b39e1f0ed7d5ae998b2a1ed2137db8cf8c3476cf2962c7437ecb003`. This checks integrity against the mirror's HTTPS checksum; no detached signature was verified.

## Configuration and access

Portable QEMU 11.1.0 uses TCG, four virtual CPUs and 4 GiB RAM. The 24 GiB QCOW2 overlay is backed by the downloaded image, with UEFI firmware and a separate writable variable store. A NoCloud seed creates a dedicated `builder` user with passwordless sudo and a VM-only SSH key; SSH password and root login are disabled. TCP SSH is explicitly enabled by cloud-init. The replacement guest listens only on host `127.0.0.1:2224`:

```powershell
& 'C:\Users\Jon\Documents\Projects\nudanos-revival\test-runtime\ssh-arch-rescue.ps1' 'uname -a'
```

The wrapper uses `test-runtime\arch\id_ed25519` and `rescue-known_hosts`. Start with `test-runtime\start-arch-rescue.ps1`; stop with the wrapper command `sudo poweroff`. The VM files are in `test-runtime\arch`. Local socket access required sandbox escalation. Do not commit the private key or runtime disk.

The original guest hit a kernel NULL-pointer oops after a long clock pause during a kernel upgrade, leaving its boot image missing. It was replaced with the clean `rescue.qcow2` overlay; the original `system.qcow2` is not used by the replacement. This replacement completed package installation, generated its new kernel image, and rebooted successfully.

## Versions and package state

`/etc/os-release` reports Arch Linux, `BUILD_ID=rolling`. The downloaded image booted `7.2.11-arch1-1`; the replacement now runs the installed `7.2.14-arch1-1` kernel. Cloud-init completed and the guest's built-in `pacman-init.service` finished before installation. The first guest's earlier attempt raced keyring initialization and failed package signature checks without upgrading anything. The clean guest retained signature checking and completed:

```sh
sudo pacman -Syu --noconfirm --needed base-devel go audit git pam libxcrypt jansson python python-pip python-setuptools openssh systemd perl perl-ipc-run3
sudo pacman -S --noconfirm --needed python-crypt_r
```

Both transactions exited 0. `pacman -Dk` reported no database errors, and the lock was absent afterward. Package versions: base-devel 1-2, Go 2:1.27.3-1 (`go1.27.3-X:nodwarf5 linux/amd64`), GCC 16.2.1+r23+gd564253eb6c8-1, make 4.4.1-3, pkgconf 3.0.10-1, audit 4.2.1-1, git 2.56.0-1, PAM 1.7.3-1, libxcrypt 4.5.2-1, Jansson 2.15.1-1, Python 3.14.9-1, python-pip 26.2.2-1, python-setuptools 1:84.2.1-1, OpenSSH 10.6p1-2, systemd 262.1-1, Perl 5.44.0-1, perl-ipc-run3 0.049-4 and python-crypt_r 3.13.1-1. `pkg-config --modversion libsystemd` returned 262; Perl IPC::Run3 and Python crypt_r imports succeeded. Guest package logs are `/home/builder/base-install.log` and `/home/builder/crypt-install.log`. Coordinate any later pacman changes to avoid concurrent package transactions.
