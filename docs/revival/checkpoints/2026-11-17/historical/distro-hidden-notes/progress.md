# SDD ledger — plan: docs/superpowers/plans/2026-11-03-native-linux-revival.md

User authorized the reviewed sequence on 2026-11-03 and native Debian + Arch in the prior turn.
Workspace: C:/Users/Jon/Documents/Projects/nudanos-revival. Branch in each linked worktree: revival/native-linux.

Ruling: execute the accepted sequence without another planning approval — explicit “go ahead” plus the developer autonomy rule authorizes the concrete implementation.
Ruling: use git linked worktrees in a sibling multi-repository workspace — native create_worktree rejected nested sourceRoot because the chat workspace is Projects; all worktrees remain inside the authorized writable root.
Ruling: implement independent repositories in parallel — no shared files/working trees; developer proactive parallel delegation takes precedence over the generic skill’s sequential-worker default.
Ruling: do not push/merge/publish or change host networking/authentication during this local implementation. A portable isolated VM supplies Linux tests; no host WSL/virtualization changes.

## Preflight conflict scan

| Task/pair | Check | Ruling |
|---|---|---|
| 1 | Bounded decode/session issuer/startup and credential tests agree | Preserve supported schemes; do not silently break valid clients |
| 2 | Peer identity/input/recovery contracts | Attribution cannot elevate authority; atomic persistence errors observable |
| 3 | Password/PAM/lockout lifecycle | Common semantics, supported distro adapters; preserve recovery |
| 4 | Migration error vs loader ignores errors | Transit must actually be blocked, not only failed hook exit |
| 5 | Publisher/installer trust agreement | Must produce and consume same mandatory signed asset |
| 6 | Development refs vs immutable release | Resolve once; don't refetch a different lint source |
| 7 | Shared builds and existing Debian package transforms | Move behavior before translating packaging |
| 8 | Arch build host vs native runtime | VM/package lifecycle evidence required |
| 9 | Optional provider claims | Remain explicit pending gates |
| 1/2/3/4 | Separate component repositories | Independent mutations may proceed in parallel |
| 4/6 | Kernel package selection + source lock | Lock must select corrected component commits |
| 5/6 | distro CI/release files shared | Root owns both; sequential edits |
| 6/7/8 | Build metadata and native targets | Phase order preserved; refine later interface spec from verified changes |

## Progress

- [x] Reviewed source baseline and accepted spec loaded.
- [x] Thirteen clean isolated linked worktrees created.
- [x] Local Debian runtime prepared (Debian 13.7 QEMU, Go 1.26.10, loopback SSH; see linux-environment-report.md).
- [ ] Task 1 REST: local signed-off 84143ff; focused suite passed, sanitizer/full package checks tracked in task-1-report.md.
- [ ] Task 2 management daemons. Configd fixes are in local commit `b63160d`; VCI producer authorization remains outside the November 6 scope.
- [x] Task 3 authentication and package safety for the selected Debian scope.
- [ ] Task 4 network safety.
- [ ] Task 5 image trust.
- [ ] Task 6 reproducible Debian.
- [ ] Task 7 shared runtime.
- [ ] Task 8 native Arch.
- [ ] Task 9 service/provider gates.

## 2026-11-05 checkpoint

- Windows shell/POSIX tests require the disposable Debian VM for accurate permissions. Use implementation/run-debian.ps1 (preserves Git file modes and LF when exporting) through exec_command require_escalated for loopback SSH. Do not install or change Windows services/WSL. VM apt owner is root agent.
- `vyatta-cfg` f71d97a, signed off: hook exit 78 prevents load/commit and tears down the session. Debian shell regression first failed on ignored EX_CONFIG and then passed; ordinary hook failure still continues.
- `vyatta-kernel-forwarding` 5897ef6, signed off: critical protected/ambiguous boot config is preserved; privileged systemd ExecStartPre refuses it and installs scoped nftables drop-forward tables. Existing legacy `.dpdk-only` firewall/NAT is also refused. Linux parser RED observed original stripping and inline fail-open, GREEN 86 tests plus actual dual-stack routed probes. Local traffic and unrelated table preserved; bridge table existence verified (no bridge data-plane probe yet).
- Distro fixture harness changes pending commit: recognize exact hook exit status and confirm firewall/NAT cases remain byte-identical without load; the two focused RED tests failed against old harness and focused GREEN passed. Full Go/Python suite running. Docs updated. Remaining task 4: ifmgrd, VRF and MPLS work; do not mark whole task complete yet.
- VCI worker committed da9a6ad with test-first signal body validation. Full D-Bus-backed suite passed; producer authorization remains a separate contract problem.
- REST original agent was stopped by a generic cybersecurity filter during attack-oriented regression development. A fresh defensive-only worker used benign tests and implemented/restored scoped checks without rerunning the flagged approach. User's Git author identity is configured per local repository; do not modify global Git config.
- Prior CI package archive `apt-repo` id 13239336290 (208660895 bytes) downloaded read-only and staged for Debian package builds. Verify APT InRelease against distro/keys/nudanos-archive.asc using Signed-By. No remote changes.

## 2026-11-06 login, PAM, and configd checkpoint

- User narrowed this checkpoint to login, PAM, and configd, with GitHub publication deferred. Four clean `revival/native-linux` worktrees contain signed-off local commits. No remote writes occurred.
- `vyatta-login` `7e3b45e` (2.44), `vyatta-debian-pam-configs-config` `541f763` (1.13), and `cli-sandbox` `8c71637` (0.32) built on Debian 13.7. Tests covered native hashing, recognized legacy lockout migration, renewal module ownership transfer, PAM diversion preservation, sandbox activation rollback, and real PAM profile removal. See task-3-report.md.
- `configd` `b63160d` (2.16) passed the full Go test suite and vet, a real unprivileged Unix peer with unset login UID, recovery restart and failure checks, and confirmed-commit rollback scheduling. The final Debian binary build and its coverage-enabled server and session suites passed. See task-2-report.md.
- The VM verified the prior CI archive against the repository-pinned key fingerprint and installed its signed build dependencies using APT Signed-By. The Debian tests did not cover a full appliance upgrade, deployed AAA policy, or a real nspawn login session. Native Arch remains a later stage.

## GitHub publication and Arch verification

- User authorized publishing the four login, PAM, sandbox, and configd commits first, then testing on native Arch.
- GitHub verified `refs/heads/revival/native-linux` in `nudanos/vyatta-login` at `7e3b45e`, `nudanos/vyatta-debian-pam-configs-config` at `541f763`, `nudanos/cli-sandbox` at `8c71637`, and `nudanos/configd` at `b63160d`. The push targeted only these feature branches.
- Arch checks use a separate disposable x86_64 QEMU guest. The four published branches contain Debian packaging and no PKGBUILDs. Tests distinguish native source execution from pacman installation and service integration.
- Arch verification completed after the client resumed November 8. Configd's native build, full Go suite, vet, and real unset-login-UID peer regression passed. Official Arch crypt_r passed six password RPC tests, the sandbox passed four Python tests and native module build, and stock Arch PAM passed real lockout/reset/root-recovery checks. The Debian-specific hooks require an Arch adapter, and the password module's compilation used fixture configd bindings. Full native packaging and appliance integration remain pending. See arch-verification.md.

## Baselines

distro 864370dc3ff5a6086ce298b147f79ac2c4de315e
vyatta-rest a520840d1bc51a9bef4d86bb61bd2105068fb2a7
configd 240e958c481b5024620a5599a665f5b31c222ec4
vci 0499b9e2b4c5ee7883e0f0c397685cfd26d762c4
ifmgrd 40e22ccdfd43ab7f408c9fca459625a4d25cc195
vyatta-login 64c15c933f59ae552e8044dbd70a6f2d13e6c99b
vyatta-kernel-forwarding 471ce7e62166d3966b6a648dcff870ebba955229
vyatta-image-tools 0405dbdeb2818a4f19136730610cb4ba87058c75

