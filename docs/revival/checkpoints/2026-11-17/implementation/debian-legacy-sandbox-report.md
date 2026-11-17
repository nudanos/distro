# Legacy sandbox appliance upgrade — in progress

This independent gate uses `test-runtime/acceptance-legacy-sandbox/legacy.qcow2`, a new qcow2 overlay of the preserved installed baseline. The read-only backing file remains `test-runtime/acceptance-baseline/baseline-preserved.qcow2`, SHA256 `823195ac55359ea2b0f1e11433c6d2a3d8953811c1c44b70f1be813e1295a9dd` (reverified before setup). The primary acceptance VM and its controller are untouched.

The copied controller is `test-runtime/acceptance-legacy-sandbox/boot.py`; it boots existing QEMU using q35, WHPX, qemu64,-svm, 2 CPUs and 2048 MiB. Serial input is paced at 40 ms per byte. The serial transcript is `serial.log` in the same directory. Host Python is `C:/Users/Jon/.cache/codex-runtimes/codex-primary-runtime/dependencies/python/python.exe`. The local read-only HTTP server uses port 18767, rooted at this independent directory. No SSH forwarding or host settings/installations were added.

The retained baseline repository ZIP SHA256 reverified as `76a4aeb2d24532aebcb71ce4dec8e71cf6bc5fe281b6e12344c462e8032f4836`; it was extracted into this gate's `baseline-repo`. A separate copy of the distro-pinned public key is served as `pinned-baseline.asc`, SHA256 `e1fb2ecc48d4ec614103efacc2b9a54242b7edf00f3ac7eedb51410b77e4bfc1`; expected fingerprint from the distro key record is `44235C5FF45218554D862EAABA18191EBBD0A857`. Actual APT signature validation is still pending.

Installed baseline admin login succeeded. After the boot configuration's getty restart, admin login was repeated and the normal configuration CLI created disposable superuser `legacy`, configured dataplane `dp0s16` as `10.0.2.15/24`, a default route via `10.0.2.2` and DNS `10.0.2.3`, then committed successfully. Fixture passwords are disposable and appear only in local controller/transcript artifacts.

Next: use `setup-old.sh` served over HTTP for signed baseline APT installation of cli-sandbox/pam-sandbox/shared-storage/optional isolation feature 0.31, retaining renewal 2.43. Establish a genuinely working old isolated session and root inventory before host upgrade. No legacy isolation, renewal, or upgrade pass is claimed yet.

Baseline setup completed: APT accepted the pinned signed baseline index and installed cli-sandbox/pam-sandbox/shared-storage/optional feature 0.31 with Debian mmdebstrap 1.5.7-1+deb13u1 and systemd-container 257.13-1~deb13u1. The package's normal postinst successfully created `/var/lib/sandbox` in 360.715 seconds. `dpkg --audit` was empty; renewal remained 2.43 and common-session contained a required pam_sandbox rule. This is setup evidence, not a session pass.

The old root inventory (`inventory-old.sh`, serial transcript) confirms renewal 2.43, libvyatta-config2 1.34, libjansson4 2.14-2+b3 and no internal password module. Its dpkg status SHA256 is `5be07cf6d81faa3da9a9aae9accd8d7ec5a03d0309b77014908a20809c952edf`; the old host password module SHA256 is `14924df91968f2ca591a20f05cef24fc846a87fc192690963dfba308c6148e5c`. The 0.31 nspawn template binds `/lib/x86_64-linux-gnu/security/pam_vyatta_passwd.so` from the host. Baseline pam_sandbox is installed at `/usr/lib/security/pam_sandbox.so`. A root inventory command via sudo succeeded after installation.

`prepare-session.sh` recorded host mount/PID/network namespace identities (`4026531841`, `4026531836`, `4026531840`) and root device/inode (`26:2`), created a harmless host-only marker in `/var/tmp`, and staged a session probe in the admin's shared home. Actual admin login is the next check.

## Actual old-root session results

Normal serial password authentication as `nudanos` reached the welcome banner, then returned to getty without an interactive shell. Host journal confirmed that pam_sandbox started `cli-sandbox@nudanos`, entered `cli-1016`, then the unit deactivated after the session exited. This is a real baseline console-session failure, not a successful console gate. The old sandbox lacks `/dev/ttyS0`; causality is not yet proven.

A separate PAM `su - nudanos` command successfully executed the session probe inside the sandbox. More importantly, after enabling SSH through the normal CLI, a real password-authenticated interactive SSH login from the guest host to `nudanos@127.0.0.1` succeeded. The CLI SSH-enable commit emitted an unrelated existing `/boot/grub` missing-directory warning and reported committed with nonfatal failures; SSH nevertheless started. The operational CLI's `ssh` alias does not accept standard OpenSSH options, so the actual probe used `/usr/bin/ssh`.

The authenticated SSH session executed `/home/nudanos/legacy-session.sh`: UID/GID 1016, mount/PID/network namespaces `4026532278`, `4026532281`, `4026532277`, root device/inode `52:2`, and `HOST_MARKER_HIDDEN`. It saw renewal 2.43 and the host-bound password module. Its terminal was `/dev/pts/0`; `/dev/console` pointed to pts/0, and `/dev/ttyS0` was absent. Exiting returned to the host recovery shell. Thus an actual old sandbox SSH session is established; this does not waive the separate serial defect or establish renewal/upgrade success.

## Current diagnostic state

`trace-login.sh` downloaded and extracted, without dpkg installation, Debian strace 6.13+ds-1 (deb SHA256 `cae290b67cf835350a2cde58ca332256fba475784b3e02a9a8677d73eaf47511`) and its libunwind8 1.8.1-0.1 dependency into `/var/tmp/legacy-trace`. It invokes `strace -ff /bin/login -f nudanos` strictly as a diagnostic; forced login is not acceptance evidence. Login package version is `1:4.16.0-2+really2.41.5-0+deb13u1`.

This diagnostic currently holds the serial foreground without returning. Newline, `exit`, and guest EOF did not release it. The running controller has no QMP or host SSH forwarding, so the process/disk are being preserved while recovery is determined. No candidate packages have been installed and no product code/PAM rules were edited. The next controller must provide independent management access before retrying tracing. Package and old-root hashes above remain the pre-upgrade provenance.

Recovery action: guest Ctrl-\\ also failed to release the diagnostic. Root authorized bounded recovery of this disposable VM. The exact QEMU identity was checked (PID 34588, name `nudanos-legacy-sandbox`, disk `acceptance-legacy-sandbox/legacy.qcow2`) before stopping it. Other VM processes were untouched. The offline overlay was copied to `legacy-pre-recovery.qcow2`, SHA256 `808383429a0a631e827170c876ae794337da0ed23ecb867837910be3c140fd4c`; `qemu-img check legacy.qcow2` reported no errors. This copy is crash-consistent, not a clean-shutdown snapshot. Baseline master SHA256 was reverified unchanged.

`recovery.py` now boots the same working overlay with QMP on loopback port 18768 and SSH forwarding on loopback port 2224, logging separately to `recovery-serial.log`. It logs in as excluded recovery superuser `legacy`. Guest filesystem/journal recovery must be checked before continuing tests. Product code and PAM rules remain unchanged.

Port correction: 2224 is reserved for the existing Arch fixture. Before any SSH use, the identity-checked QMP interface removed this VM's 2224 forward and added **2226**. The saved controller now uses 2226. Arch scripts/known-hosts and other VMs were untouched. A disposable management key lives outside the HTTP root at `test-runtime/legacy-keys`; only its public key was placed in the fixture account.

Recovery checks passed: system-configure returned success/active/exited; `dpkg --audit` was empty; old-root dpkg status hash stayed identical. EXT4 mounted/remounted without reported filesystem errors. Journald detected the unclean prior journal and renamed it, explicitly recorded in `recovery-check.log`. The saved failed diagnostic trace explains its hang: invoking login as a traced subprocess changed its foreground process-group context; TCSETS raised SIGTTOU before PAM. It was a harness diagnostic failure, not the sandbox root cause.

## Confirmed baseline serial defect and clean preservation

A separate 90-second bounded strace attached to the actual serial getty PID2508 through independent SSH management. Real password login reproduced the console failure. After sandbox entry, login's child PID2778 called `setsid()`, then `openat("/dev/ttyS0", O_RDWR|O_NONBLOCK)` returned ENOENT. It slept five seconds and exited 1 before executing the user shell. This proves the missing terminal in the sandbox causes the actual serial login failure. Evidence: `serial-trace-tail.log` and `recovery-serial.log`. No product/PAM modifications were made. Candidate behavior must be tested separately; the successful old SSH session remains valid evidence for the legacy-root upgrade path.

`preserve-old.sh` verified cli-sandbox@nudanos inactive/dead with Result=success, no remaining runtime sandbox mounts, unchanged old-root status hash, original package versions, and empty `dpkg --audit`, then synced and powered off. `old-clean-teardown.log` records this. The serial log reached `reboot: Power down`; inherited loop-device cleanup warnings were recorded. The controller exited 0. An offline `qemu-img check` found no errors and a read-only `legacy-working-old.qcow2` copy was made, SHA256 `260c9dbce124194afa0978b520ca7430336acb5ebc25dda0bb352f4d34bf8ad0`. Candidate upgrade remains pending corrected packages from the root task.

## Serial fix validation on an independent overlay

`build-test.qcow2` is a new overlay of the preserved old-working fixture; build
tools and candidate sandbox0.33 were installed there only. The real baseline
serial defect is fixed by capturing fd0's validated physical terminal before
namespace entry and creating a root:root000 alias after chroot. Native package
build, C regression/security tests, actual serial/SSH sessions, cross-user console
reuse while the first user's SSH sandbox survives, and clean teardown passed.
The oldroot package inventory SHA256 remains unchanged. Exact source/package
hashes, actual Debian login reopen trace, logs and limits are recorded in
`serial-terminal-report.md`. Source awaits root final review/commit approval.

The corrected host upgrade and in-session compatibility gate are now complete,
as recorded below. This is old-root compatibility evidence, not a substitute for
the separate fresh-image/install/normal-image-upgrade acceptance gates.

## Corrected host upgrade and actual old-root live renewal — PASS

Scope qualification added after the independent persistence finding: this gate
proved live in-session renewal and fresh SSH authentication before shutdown. It
did NOT reboot and authenticate afterward. Root subsequently found that2.45
renewal failed to persist across reboot in the separate APT fixture; therefore
this result does not establish durable password renewal or complete final Debian
acceptance. A corrected artifact must repeat renewal followed by immediate clean
reboot, with no intervening unrelated config commit that could mask the defect.

`upgrade-auth.sh` used the signed interim-v2 repository at host HTTP18769. It
verified signing fingerprint `632DE3028B010AD7971CF88FC8FF6AE6F7B0295F`, then used
normal APT to install configd2.17, vyatta-login/renewal2.45, login schema2.45 and
PAM config1.13. Sandbox packages0.33 remained installed. The solver upgraded the
related configd/login schema and utility packages and installed python3-crypt-r;
no packages were removed. `upgrade-auth.log` records the full transaction.

Before the upgrade, configd PID3956 had SHA256
`4e02c50e16e1ee71ae33d19d6bc281ef8e0029b60d9ddebcfaa51b923a8a84d5`.
After normal package configuration, running PID7598 and `/usr/sbin/configd` both
had SHA256 `8001be84bb8a7280d8e91fbf74c46e948c885005e987c08d5a18fea19007ec76`.
Thus this transition actually started the corrected daemon before testing;
package versions alone were not treated as runtime proof. Audit was empty.

A real password-authenticated serial login entered the retained old sandbox.
`renewal-probe.sh` inventoried renewal2.43, libvyatta-config2 1.34 and
libjansson4 2.14-2+b3 inside that root. The internal module remained absent in
`/var/lib/sandbox`, while mountinfo in the actual user session showed the
read-only fallback host bind at the native PAM path. Its SHA256 was
`04280dc30b2a8769c0735dc5eec01645f2bc75c0d2cad4b7637fb98990b690b5`, exactly the
new host2.45 module. The root's passwd PAM stack loaded that module.

The user ran `/usr/bin/passwd` from the existing isolated interactive session,
entered its current password and a new password twice, and received
`passwd: password updated successfully`. The configd journal recorded the
`nudanos` password update and successful commit. A fresh actual SSH connection
then explicitly rejected the old password and accepted the new password; its
interactive shell ran the same provenance probe through `/dev/pts/0`.
`legacy-final-gate.log` records the corresponding Failed/Accepted password
journal events at18:48:50/18:49:04 UTC on2026-11-09. This exercises the bound
module inside the old root, not just pre-session host PAM.

Evidence files under `test-runtime/acceptance-legacy-sandbox/`:
`upgrade-auth.log`, `renewal-bound-module.log`, `legacy-final-gate.log`, and the
real conversation in `build-serial.log`. Reproducible scripts are
`upgrade-auth.sh`, `renewal-probe.sh`, `terminal-teardown.sh` and
`finish-legacy.sh`.

Oldroot dpkg status SHA256 stayed
`5be07cf6d81faa3da9a9aae9accd8d7ec5a03d0309b77014908a20809c952edf` before and
after host upgrade, renewal and teardown. This asserts retained package
inventory, not that every runtime file in the root is byte-for-byte unchanged.
Both sandbox units finished inactive/dead/Result=success with no runtime mounts.
The guest synced and powered off; controller exit0 and serial `reboot: Power
down` were observed. Existing live-image loop-unmount warnings remain recorded.
Offline qemu-img check found no errors.

Final overlay `build-test.qcow2` SHA256:
`69eb171ba91a13db89a95dd769a3dcaacfc3e1abc35c8abdc0016cdeddcb25b7`.
Reverified immutable working-old fixture SHA256:
`260c9dbce124194afa0978b520ca7430336acb5ebc25dda0bb352f4d34bf8ad0`.
Reverified baseline master SHA256:
`823195ac55359ea2b0f1e11433c6d2a3d8953811c1c44b70f1be813e1295a9dd`.
