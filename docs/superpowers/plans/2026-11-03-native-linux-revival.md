# Native Linux Revival — Security Baseline Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans to implement task by task. Track verification and remaining work honestly.

**Goal:** Correct the reviewed security/correctness defects, then establish a reproducible Debian baseline and native Debian/Arch build/integration.
**Architecture:** Retain the shared configd/YANG/VCI runtime and kernel forwarding baseline. Debian and Arch receive separate native package/OS integration; the existing Debian image is a separate appliance profile. Restore optional providers only after their own acceptance gates.
**Tech Stack:** Go, C/C++, Perl, Python, POSIX shell, systemd, Debian packaging, Arch PKGBUILD, QEMU test VMs.
**Spec:** C:/Users/Jon/Documents/Projects/nudanos-review/REVIVAL-REVIEW.md (accepted by the user on 2026-11-03: “go head with that sequence”).

## Global Constraints

- Native execution on both Debian and Arch; building Debian containers on Arch is insufficient.
- Debian first; preserve existing passing behavior while adding negative tests.
- Keep preserved review checkouts unchanged. Work in linked revival/native-linux worktrees under C:/Users/Jon/Documents/Projects/nudanos-revival.
- Do not merge, publish releases, modify host authentication/networking, or expose test services.
- Reproduce each meaningful defect with behavioral tests before its implementation. Do not claim Linux validation from source inspection.
- Preserve configuration safety, authorization boundaries, supported import/API identities, package ownership, and a management recovery path.
- Stage and commit only each task’s files. Record unverified work and environment limitations.
- Independent repositories may be implemented in parallel; mutations within one repository remain owned by one worker.
- Later-phase specifications are refined from actual first-phase results; do not treat this plan’s later milestones as completed deliverables.

## Review Focus

- Malformed/empty/long authentication and IPC input must reject safely without crashing or logging secrets.
- Unprivileged peer identities and attacker-controlled runtime paths must never acquire administrator authority.
- Failed/partial configuration actions must not advance applied state or silently remove network protection.
- Authentication and package lifecycle changes must preserve recovery, administrator edits and unsupported-platform failures.
- Release verification, cache hits and source provenance must not silently accept absent, corrupt or mismatched inputs.

### Task 1: REST authentication boundary

**Owner/Files:** vyatta-rest only; scripts/vyatta-webgui-chunker-aux, src/server/authbasic.cc, src/server/authsession.cc, existing test/build integration and minimal new security helpers/tests as needed.
**Consumes:** existing Basic and session auth interfaces.
**Produces:** same supported auth API with explicit-length decoding, no credential logging, protected session authority and Linux regression tests.
- [ ] Trace decoder/parser/session issuer/startup ownership and all supported users.
- [ ] Add behavioral regressions for empty/invalid/max-length Basic headers, embedded NUL, colon/password behavior; demonstrate current failure under sanitizers.
- [ ] Add Linux tests proving an unprivileged UID cannot replace session authority and legacy valid session behavior still works.
- [ ] Implement bounded decoding and restrictive root/service-owned runtime state; reject unsafe/symlink state rather than trusting it.
- [ ] Run focused and existing tests, inspect diff, commit and write task-1-report.md.

### Task 2: Management daemon identity/input/recovery

**Owner/Files:** configd and vci only: configd/server/conn.go and tests; configd/session/commitmgr.go and tests; vci/dbus_transport.go, policy and tests.
**Consumes:** existing peer credentials, audit attribution, RPC method maps and VCI bus.
**Produces:** invalid-argument errors rather than panic; peer identity retained on unset attribution; safe signal delivery; observable atomic recovery persistence.
- [ ] Pin behavioral null/type/signature tests before changes, preserving permitted RPC conversion behavior.
- [ ] Reproduce unset-loginuid mapping with injected/pure credential resolution tests; trusted internal callers retain documented behavior.
- [ ] Reject malformed signal bodies and constrain producer authority where the actual bus/model ownership supports it; do not invent an unchecked trust list.
- [ ] Add recovery persistence failure/restart tests, atomically replace state and propagate errors using the existing response contract.
- [ ] Run focused/existing tests and package checks, commit per repo and write task-2-report.md.

### Task 3: Native authentication/package safety

**Owner/Files:** vyatta-login, cli-sandbox, vyatta-debian-pam-configs-config only.
**Consumes:** current password RPC/PAM and Debian package lifecycle.
**Produces:** supported password verification, serialized RPC input, supported lockout integration, failure-aware isolation activation and reversible owned PAM changes.
- [ ] Reproduce password RPC crypt import failure and quote/backslash payload defect with actual helper tests.
- [ ] Implement a declared supported hash-verification dependency/API and proper JSON serialization; cover incorrect/expired/long/unicode passwords.
- [ ] Replace obsolete pam_tally integration with supported lockout semantics and reject missing prerequisites before activation; preserve recovery.
- [ ] Make sandbox activation/deactivation fail on command/prerequisite errors; explicitly reject unsupported OS integration until native adapter exists.
- [ ] Correct diversion paths and remove only owned PAM edits; test install/remove with administrator edits.
- [ ] Parameterize PAM module destination for target packaging; run tests, commit and write task-3-report.md.

### Task 4: Network migration and applied state

**Owner/Files:** vyatta-kernel-forwarding, ifmgrd, routing-instance, vrf-manager, vyatta-protocols-frr.
**Consumes:** current boot hook/ifmgrd transitions, VRF naming and MPLS state.
**Produces:** fail-closed migration of unsupported security policy; truthful applied state/error reporting; consistent VRF limits; MPLS cleanup.
- [ ] Reproduce imported deny-policy removal using existing fixture/parser tests and isolated packet-level VM test.
- [ ] Add a boot-wide transit guard/recovery contract before removing unsupported security policy; do not merely return a hook error that loader ignores.
- [ ] Make ifmgrd distinguish changed/success/failure, retain previous applied state and notify/retry accurately after action failure.
- [ ] Align VRF schema/runtime limits and return failure on provisioning refusal.
- [ ] Reconcile previous/new MPLS inputs, disable removed interfaces, propagate sysctl errors.
- [ ] Run boundary/deletion/failure tests plus existing scenarios; commit and write task-4-report.md.

### Task 5: Authenticated image update

**Owner/Files:** vyatta-image-tools and distro release scripts/workflows.
**Consumes:** shipped NuDanOS public key and existing image-fetch/install interface.
**Produces:** signed image/release manifest and required verification before mounting/install; explicit recorded local-development mode only if needed.
- [ ] Reproduce absent/wrong-key/corrupt signature behavior with current helper tests.
- [ ] Require dedicated-key verification, remove silent signature downgrade, define explicit development override and compatibility migration.
- [ ] Publish matching signed assets without exposing private keys to untrusted build code.
- [ ] Test valid/missing/corrupt/wrong-key/expired inputs and release/install compatibility.
- [ ] Run tests, commit and write task-5-report.md.

### Task 6: Reproducible Debian release baseline

**Owner/Files:** distro manifest/fetch/build/aptmirror/updates and CI.
**Consumes:** completed security changes and existing build graph.
**Produces:** immutable resolved source lock used by build/lint/test; cache verification and source/package/build provenance.
- [ ] Add regressions for source movement, actual builder identity change, partial/corrupt mirrors, missing upstream packages.
- [ ] Resolve all source/packaging/tag inputs once; preserve useful development manifests but bind release to lock identity.
- [ ] Include effective builder/dependency inputs in cache identity and validate every expected artifact.
- [ ] Fail/report vanished APT package pins; separate new security/correctness lint findings from style debt.
- [ ] Verify clean package/image build and existing Debian tests with the locked source set.

### Task 7: Shared native build/install contract

**Owner/Files:** core build/install entrypoints and OS adapter assets, selected after prior changes.
**Consumes:** stable corrected runtime APIs.
**Produces:** independently staged runtime install, generated model/service metadata and explicit capability/path/user/auth/network ownership contracts.
- [ ] Document exact per-component outputs and distribution boundary from actual Debian build behavior.
- [ ] Extract packaging-only transformations/version logic to ordinary build/check/install interfaces.
- [ ] Pin shared Go dependencies while keeping plugin/daemon toolchain graph consistent.
- [ ] Stage without host mutation; compare Debian install manifests and rerun Debian tests.

### Task 8: Native Arch kernel-mode target

**Owner/Files:** native Arch packaging/integration and test harness.
**Consumes:** shared staged runtime and OS contracts.
**Produces:** native packages/rootfs/VM covering CLI/configd/VCI/interfaces/VRFs/static routes/FRR with package lifecycle and recovery tests.
- [ ] Native source builds in clean Arch chroots; no converted Debian binaries.
- [ ] Arch PAM/NSS/systemd/sysusers/tmpfiles and module paths; explicit network/FRR/DNS ownership.
- [ ] Install/configure/save/reboot/upgrade/remove/reinstall on native Arch and Debian VMs.
- [ ] Publish no support claim without retained runtime test results.

### Task 9: Service/provider completion

- [ ] Add DNS/SNMP/AAA and other declared service tests to both native targets.
- [ ] Add DHCP/Kea and VPN/VICI only with explicit backend/schema/lease/XFRM tests.
- [ ] Choose tested kernel policy versus optional DPDK provider work from user’s accepted architecture; no implicit NPF-to-nftables translation.
- [ ] Before DPDK restoration, correct dormant FPM listener trust, validate FPM/VRF/ECMP/MPLS/resync, driver/kernel/protobuf/PMD versions and packet hardening.

## Delivery record

Each task report records baseline SHA, changed commits/files, RED/GREEN command/output, independent review findings and remaining runtime checks. A task with missing runtime evidence remains pending verification. Update the durable ledger after each meaningful state change. Keep an explicit list of phases not yet delivered.
