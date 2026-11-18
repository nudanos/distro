# NuDanOS revival checkpoint

Updated 2026-11-17. This directory backs up the Debian-first, native Debian/Arch
revival work, its source checkpoints, and curated test evidence.

## Current state

The initial login, PAM, configd and sandbox fixes, followed by the service-refresh,
lockout-validation and serial-terminal corrections, have been merged. Complete
Debian image acceptance remains pending. Native Arch implementation has not
started; its source research and proposed task graph are preserved here.

The blocking password test is explicit: renewal changes live credentials, but a
clean reboot without an intervening CLI commit restores the old password. The
target-only persistence repair is a design awaiting implementation. The attempted
`1.0~20261109.1` image was also rejected because its copied bootstrap cache lost
nonzero privileged-file groups. Future image builds must use a fresh bootstrap;
package-download caching remains usable. The focused operational-provider fix is
checkpointed and has historical unit/appliance evidence.

The [checkpoint evidence](checkpoints/2026-11-17/README.md) includes the authoritative
execution ledger, review findings, plans, scripts and historical source snapshots.
Older reports record their original observations; the current ledger qualifies
the password-persistence result and rejected image. See the included manifest for
publication exclusions and sanitization.

## Source backups

| Repository | Backup branch | Checkpoint |
| --- | --- | --- |
| distro | revival/native-linux | [9375a36](https://github.com/nudanos/distro/commit/9375a365237f5fe06a3d03278f3f13c1633d6901): safety fixtures and revival plan; later commits on this branch add this evidence backup |
| vci | revival/native-linux | [da9a6ad](https://github.com/nudanos/vci/commit/da9a6ad99ba8c06b2a7221b888c8bbea764e7e3c): notification validation |
| vyatta-cfg | revival/native-linux | [f71d97a](https://github.com/nudanos/vyatta-cfg/commit/f71d97aa7583f415baaaeb1dab2869cc01c136dd): boot-hook refusal |
| vyatta-kernel-forwarding | revival/native-linux | [5897ef6](https://github.com/nudanos/vyatta-kernel-forwarding/commit/5897ef6532b930e62d9e92e6e472aa466181998d): unsupported policy migration refusal |
| vyatta-rest | revival/native-linux | [84143ff](https://github.com/nudanos/vyatta-rest/commit/84143ff4163c7ae9a4eaa6cafdddf4133580a4f6): REST credential/session hardening |
| vyatta-login | revival/renewal-persistence | [b08af22](https://github.com/nudanos/vyatta-login/commit/b08af22967fad622e3e54f2ce03ea2e52e4c8f15): state-provider fix and tests; persistence repair still pending |

These feature branches preserve unfinished work and may require integration with
newer upstream changes before merge. Their existence does not indicate a passing
full-image or Arch acceptance gate. Already merged component commits and CI
artifact identities are recorded in the execution ledger.

## Continue in this order

1. Reconcile the actual password renewal path and configd's automatic save;
   correct the design and verify immediate reboot without another commit.
   [November18 correction and draft source checkpoints](2026-11-18-status.md)
   supersede the target-only persistence assumption in the historical plan.
2. Complete corrected Debian fresh installation, APT and normal image upgrade,
   reboot, lockout/recovery and fresh/legacy sandbox acceptance.
3. Implement and test native Arch packages and PAM integration against that
   validated Debian behavior.
