# Local Linux test environment

Prepared 2026-11-03 inside `test-runtime`, without host software installation, WSL, Docker, or Windows feature changes.

## Runtime and access

- Portable QEMU 11.1.0, extracted rather than installed, running headlessly with TCG, 4 virtual CPUs and 4 GiB RAM.
- Official Debian 13 genericcloud amd64 disk; guest reports Debian 13.7 and kernel `6.12.115+deb13.1-cloud-amd64`.
- 24 GiB writable QCOW2 overlay, backed by the unmodified official image. No host filesystem share.
- SSH listens only at host `127.0.0.1:2222`. Dedicated user `builder` has passwordless sudo and key authentication. Password/root login is disabled.
- NoCloud initialization completed; actual SSH `id`, Debian release, kernel, and disk checks succeeded.

Run from PowerShell (Codex execution requires escalation because sandbox blocks loopback sockets):

```powershell
& 'C:\Users\Jon\Documents\Projects\nudanos-revival\test-runtime\ssh-debian.ps1' 'uname -a'
```

File transfer:

```powershell
$r = 'C:\Users\Jon\Documents\Projects\nudanos-revival\test-runtime'
scp.exe -i "$r\debian\id_ed25519" -P 2222 -o BatchMode=yes -o "UserKnownHostsFile=$r\debian\known_hosts" <local-file> builder@127.0.0.1:/home/builder/
```

The SSH wrapper forwards command arguments; use SCP for input files rather than piping content through the wrapper.

## Lifecycle and files

- Start: `test-runtime\start-debian.ps1` (uses `Start-Process -WindowStyle Hidden`).
- Graceful stop: `test-runtime\stop-debian.ps1` (`sudo poweroff`).
- PID: `test-runtime\debian\qemu.pid`; boot log: `debian\serial.log`; QEMU logs: `debian\qemu.{stdout,stderr}.log`.
- Guest disk: `debian\system.qcow2`; base disk: `downloads\debian-13-genericcloud-amd64.qcow2`.
- SSH key/known hosts: `debian\id_ed25519`, `debian\id_ed25519.pub`, `debian\known_hosts`. Treat the private key as local test credentials; do not commit it.
- NoCloud seed: `debian\seed.iso`; regeneration helper: `test-runtime\make-seed.py` with workspace-local pycdlib.
- Guest build directory `/srv/nudanos` is owned by `builder`.

## Provenance and verification

- QEMU installer and Debian image matched published SHA512 checksums before execution/boot.
- 7-Zip portable bootstrap, Extra archive, and full package matched SHA256 digests from the official GitHub release assets. The full package supplies NSIS archive extraction; its installer was never run.
- Download sources: [QEMU Windows binaries](https://qemu.weilnetz.de/w64/), [Debian cloud images](https://cloud.debian.org/images/cloud/trixie/latest/), [7-Zip release](https://github.com/ip7z/7zip/releases/tag/26.05).
- VM setup follows [QEMU invocation](https://www.qemu.org/docs/master/system/invocation.html) and [cloud-init NoCloud](https://docs.cloud-init.io/en/latest/reference/datasources/nocloud.html).

## Package status

All requested package batches completed with exit status 0. `sudo dpkg --audit` is clean. Installed base build tools include GCC/G++ 14.2 and Git 2.47.3; Python `crypt_r`/`passlib` and Perl IPC::Run3/File::Slurp/JSON/Template/NetAddr::IP imports were verified. REST SSL/PAM/audit/Jansson/curl development headers and Go dbus/fsnotify/systemd/crypto/ini libraries are installed. `sudo unshare -m --propagation private true` passed.

Go is explicitly the Debian backport `golang-1.26-go` version `1.26.10-1~bpo13+1`, verified as `go version go1.26.10 linux/amd64`. `/usr/local/bin/go` and `gofmt` link to `/usr/lib/go-1.26/bin/`, so ordinary remote commands select this version. The official image already configured the trixie-backports apt source.

Reproduction installers: `test-runtime\install-base.sh` and `install-go126.sh`. Host copies of apt transaction logs and the installed-package check are in `test-runtime\debian\{base-install,go126-install,extra-install,package-verification}.log`; guest originals are under `/home/builder`. Apt installation ownership is released after these transactions; coordinate any later package changes to avoid concurrent apt writers.
