# Serial terminal fix acceptance — 2026-11-09

Status: source and native-package tests PASS; actual serial/SSH/cross-user reuse
and teardown PASS on the independent old-root fixture. Root completed final
review and the signed-off commit is pushed. Corrected authentication-package
upgrade and live in-session legacy-root password renewal subsequently passed;
details are in `debian-legacy-sandbox-report.md`. That renewal test did not verify
credentials after reboot; root later found a separate blocking2.45 persistence
failure. The terminal-fix results remain valid, while final durable-renewal and
Debian acceptance gates remain pending corrected artifacts.

## Source and scope

Worktree: `cli-sandbox`, branch `revival/serial-terminal`, based on
`c6a637f7dfc28f37993aafa2f1bd65fca4d5a674`. Version0.33 commit
`0c1040ce9437ef9d4f5b6131848c374892911450`, tree
`8ae52d5bbec58c17a0d6bb625b90c3c76c74732d`, carries Jon Fernandez's sign-off
and is pushed to `origin/revival/serial-terminal`. Worktree is clean.
Final source archive SHA256:
`536a1b707e8433a49e31b13b3ad83c6d271aec29a4314518a27f0293db975b0e`.
`test-runtime/acceptance-legacy-sandbox/source-manifest.json` records per-file
hashes; `prepare-source.py` exports tracked/untracked source, normalizes Windows
line endings, and preserves Git executable modes. No installed product source
was patched inside the guest.

The module captures the actual controlling fd0 once before the existing retry
loop. The direct `/dev` source path must be a character device matching fd0's
filesystem, inode and device number. Canonical numeric PTYs are skipped. No
PAM_TTY input is used. After chroot, a no-follow root-owned non-writable `/dev`
descriptor anchors `mknodat` of the single basename. The alias is root:root000;
existing entries must have matching type/device and equally restrictive metadata.
Foreign/symlink/broader aliases fail closed. Explicit administrator terminal binds
may be rejected. The original host inode can change owners without transferring
access through an old sandbox's alias.

## Fixture and reproducibility

All guest modifications/build tools are in `build-test.qcow2`, a new overlay of
read-only `legacy-working-old.qcow2` (SHA256
`260c9dbce124194afa0978b520ca7430336acb5ebc25dda0bb352f4d34bf8ad0`). The latter
is the cleanly shut down, real SSH-working0.31 fixture described in
`debian-legacy-sandbox-report.md`. The baseline master remains separate/read-only.
QEMU uses WHPX, q35, qemu64,-svm, serial stdio, SSH2226 and QMP18768.
HTTP18767 serves this fixture's scripts/source only. Other VMs were untouched.

Scripts/logs below are relative to `test-runtime/acceptance-legacy-sandbox/`:

- `build-prereqs.sh` / `build-prereqs.log`: signed baseline and Debian sources,
  build dependencies installed in disposable overlay; audit clean.
- `terminal-red.log`: compiled C regression against no-op helper fails at
  `lstat(path)` because alias is absent. The separate actual old login trace
  already proved `/dev/ttyS0` ENOENT immediately before login child exit.
- `terminal-green.log`: real-root creation/reopen, uid65534 denial, strict
  existing-node checks, path identity and controlling PTY exclusion pass.
- `package-final-build.log`: `make check`, real-root `./tests/session-terminal`
  (private mount namespace and temporary `/dev`), then stock
  `dpkg-buildpackage -us -uc -b` pass. Existing4 Python tests and shellcheck pass;
  nonroot package build explicitly skips privileged device cases.
- `install-serial.sh` / `install-serial.log`: normal APT local-package upgrade
  of four sandbox binaries0.31→0.33, explicit normal isolation activation,
  unchanged old-root package inventory. Native module path is used.
- `build-serial.log`: real password getty sessions, probes, second-user normal
  configuration and successful return to the original user's surviving sandbox.
- `prepare-reuse.sh`, `serial-fixed-trace-proof.log`: bounded strace attached to
  the actual serial agetty; records successful device creation/reopen order.
- `terminal-reuse.log`: simultaneous SSH/console probes, package/module hashes.
- `terminal-teardown.sh` / `terminal-teardown.log`: both units inactive/dead,
  Result=success, no `/run/cli-sandbox` mounts, empty `dpkg --audit`.

Final package SHA256 values:

All packages and native `.buildinfo`/`.changes` are exported under
`test-runtime/acceptance-legacy-sandbox/artifacts/`.

| File | SHA256 |
|---|---|
| cli-sandbox_0.33_all.deb | b03c57b697817bee8dd99e25c6f154a6a7383391cc75870bd93f4abf2af3b792 |
| pam-sandbox_0.33_amd64.deb | d2c828beb97fc3fe07b8cb72ea9bcc1f7abc19eef5519c1949c495912d72fba8 |
| python3-shared-storage_0.33_all.deb | e664d86398c5710df315c8a0beeee04aeb196a8cc10ef9fe2a20f14dc318b371 |
| vyatta-system-login-user-isolation-v1-yang_0.33_all.deb | da10e1035e6474216078df7ff7fe5ee39ac2335a68b68ba0b4a57ef6635970be |

Final rebuild differs in test containment/docs only from the installed build.
Both installed and final package `pam_sandbox.so` SHA256 are identical:
`c8aaf36244ef7fb037556f6485d4eac7a3e259f63bfc7a3911f9e1ae19f2ad8b`.

## Observed runtime behavior

1. Real serial password login for nudanos(uid1016) now reaches interactive vbash.
   tty is `/dev/ttyS0`, alias is char4:64 root:root000. An ordinary user
   `open(O_RDWR|O_NONBLOCK)` returns EACCES. Host-only marker stays hidden;
   mnt/pid/net namespaces differ from host. Actual process remains in the serial
   getty cgroup and successfully reopens the device, exercising actual access
   policy rather than only node creation.
2. Real password SSH login (`ssh -tt`, SSH2226) succeeds in the same sandbox using
   `/dev/pts/0`; it stays open while the console logs out and serialtwo(uid1018)
   takes over. Host tty becomes1018:tty0600. Original nudanos SSH still sees its
   own root000 alias and gets EACCES, with unchanged sandbox namespaces.
3. Actual Debian login trace: original fd0 `fchown(0,1018,5)` and `fchmod(0,0600)`;
   forked child `setsid()`, `openat("/dev/ttyS0",O_RDWR|O_NONBLOCK)=3`, then
   `setuid(1018)=0`. Unprivileged probe subsequently gets EACCES. This confirms
   the privileged reopen assumption against this appliance's patched binary.
4. serialtwo's sandbox has distinct namespaces/root. After its logout, nudanos
   serial login returns to the still-live original sandbox (same mnt4026532278,
   pid4026532281, net4026532277 and root52:2); its existing root000 alias is
   accepted. Direct user open remains denied. SSH and console then exit and
   both sandbox units finish inactive/dead/success without remaining mounts.
5. Oldroot `/var/lib/sandbox/var/lib/dpkg/status` remains SHA256
   `5be07cf6d81faa3da9a9aae9accd8d7ec5a03d0309b77014908a20809c952edf`
   before upgrade, after upgrade and after all sessions. This proves the package
   inventory was retained, not byte-for-byte immutability of runtime files.

## Limits and remaining gates

The old root contains no `ps`; the optional process-list probe prints that error,
while namespace/cgroup/device/denial assertions and actual shell interactivity
provide the relevant evidence. Login also prints existing missing-locale warnings.
Normal config commit from the isolated admin shell created serialtwo successfully
but reports an existing nonfatal missing `/boot/grub` hook failure; this fix does
not address that independent behavior.

Corrected host configd2.17 and login/renewal2.45 were subsequently installed by
normal signed APT and the running daemon hash changed as required. Password
renewal from an **existing isolated session** against the retained old root
passed through the verified read-only host-module fallback bind; fresh SSH
rejected the old password and accepted the new one. See the legacy report and
`legacy-final-gate.log`. Fresh-root candidate/full-image acceptance remains owned
by the main acceptance task. Arch remains blocked on the complete Debian gates.
