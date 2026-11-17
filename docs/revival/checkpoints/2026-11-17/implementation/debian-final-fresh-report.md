# Final candidate fresh appliance acceptance

Owner: integration_review. Current status: preparation only; final builder ISO
and exact SHA256 are pending. Root identified a blocking password-persistence
failure in the independent interim-v2 upgrade test. The pending1.0~20261109.1
image is diagnostic only until that issue is resolved; no provisional ISO is
accepted as final evidence. No fresh VM has been booted.

Before the corrected handoff, refresh the explicit expected versions in
`http/stock-state.sh`, `http/activate.sh` and `http/session-probe.sh`; their current
2.17/2.45 values describe the earlier prepared candidate, not approval of a
future artifact. Preserve the manifest guard and record corrected versions.

## Scope and fixtures

- New directory: `test-runtime/acceptance-final-fresh/`.
- New independent empty `fresh.qcow2` will be created only after final artifact
  handoff. Existing baseline/legacy disks remain read-only and untouched.
- Owned ports: SSH2226, QMP18768, probe HTTP18771. Acceptance owns QMP18770;
  Arch2224 is reserved. VM identity: `nudanos-final-fresh`.
- `console.py` uses the proven q35/WHPX/qemu64,-svm combination,40ms/byte pacing,
  persistent stdin-EOF lifetime and complete-line queue forwarding. It is a
  passive console: no init/kernel overrides, injected password or rescue action.
- `final-artifact.json` must explicitly designate the final ISO and its exact
  SHA256; every launch verifies bytes against that manifest. Transcript files
  cannot be overwritten. QMP and SSH management allow bounded independent
  recovery after a documented failure.

## Acceptance sequence

1. Record exact final ISO/repository/signature/package provenance from builder.
2. Stock live boot/default vyatta login, allowing normal getty restart during
   config application. A login is provisional until actual system-configure
   ActiveState/SubState/Result and completion timestamp are observed. No rescue
   without first recording a genuine stock-path failure.
3. Run real installer, preserve prompt transcript, clean shutdown/installed boot
   and second clean reboot. Verify installed admin authentication, final package
   versions and stock PAM/isolation state before optional activation.
4. Create excluded recovery superuser through normal CLI, configure network/SSH,
   verify signed final repository, install optional isolation model0.33 and
   create a fresh root through normal package/runtime paths.
5. Verify fresh root renewal2.45, native password module/Jansson dependencies,
   absence of configd daemon package, actual mounts/binds and root provenance.
6. Real serial and SSH sessions: namespace/root separation, hidden host marker,
   terminal access bounds, in-session password renewal and fresh credential test.
   Immediately clean-reboot and test old/new credentials again, BEFORE any
   unrelated configuration commit or isolation disable/re-enable action. Such a
   commit could accidentally save a password that renewal itself failed to save.
7. Disable/re-enable isolation through normal CLI; verify actual behavior in each
   state, final teardown and clean shutdown. Record limitations explicitly.

Main acceptance separately owns APT/daemon guard, normal image upgrade, lockout
and recovery. Stage3/Arch waits for all final Debian gates.

## Prepared provenance and harness checks

Builder supplied the final CI-based signed repository at
`test-runtime/final-ci-repository/repo`. Its InRelease SHA256 was independently
rechecked as `f879c3ee91a13489aa2695eed36076fab8613913120cf966bba3ce55ba5d5d82`;
public-key SHA256 `e608bec258289850f1416e05809123b7b5740e3536ef68bb4599f197eef73479`,
fingerprint `632DE3028B010AD7971CF88FC8FF6AE6F7B0295F`. Builder reports signature
and all648 payload hashes verified. Shared loopback HTTP18772 serves this exact
directory for both acceptance owners; fresh probe HTTP18771 serves only `http/`.

Controller checks passed for40ms-per-byte pacing, partial queue-line retention,
provisional artifact rejection and incorrect final ISO hash rejection; Python
compilation passed. Initial temporary test directory outside the workspace was
blocked by the host sandbox; rerunning with the temporary directory inside this
fixture passed. No guest or product behavior was changed by that harness check.
