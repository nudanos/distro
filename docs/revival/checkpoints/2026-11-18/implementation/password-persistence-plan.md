# Target-only password persistence repair

**SUPERSEDED — 2026-11-18. Do not execute this design.** Integration proved
successful public configd Commit automatically calls Save. Holding the proposed
startup lock across Commit nests the lock and times out after RUNNING changes.
The raw-Commit/unsaved-state assumption below was wrong. Draft source checkpoints
are preserved on GitHub; original PAM renewal/reboot behavior is being traced
before choosing a correction that preserves existing commit/save semantics.

This is an in-scope repair of the reproduced reboot acceptance failure, under
the user's existing instruction to finish login/PAM/configd and the approved
Debian-before-Arch sequence. Implementation choices below are the agent's calls.

## Evidence and approach

The password RPC commits only RUNNING. Normal cfgcli commit separately saves,
which explains why subsequent unrelated CLI commits masked the first renewal
test. Whole-running Save is unsuitable for a user password RPC: another API
client can have unrelated committed, unsaved configuration. Full Xorp parser
round-trip is also unsuitable: its output drops disabled-node markers and
rewrites unrelated text. Use a strict lexical byte-span edit of the existing
saved target's encrypted-password value, preserving every other byte.

## Contract

- A password change requires an existing, unambiguous saved target account and
  password. Reject missing/unsaved target, disabled/duplicate/ambiguous target,
  malformed config, unsafe file metadata or staging failure before commit.
- Use one stable advisory lock at `/run/nudanos-startup/startup.lock`, in a
  no-follow directory owned by the executing privileged service and mode0700;
  lock file regular, single-link, same owner, mode0600. Bound contention to10s.
  Production services execute as root. Tests use owned private temporary paths.
- Python renewal holds this lock across re-verification, candidate setup/set,
  staging, commit and publication. Normal configd local SaveTo acquires the same
  lock BEFORE taking its RUNNING snapshot. The supported legacy save utility
  also acquires it for the default startup destination/aliases. Do not lock
  ordinary runtime Commit in this protocol: renewal calls Commit while holding
  the file lock. Thus supported startup writers cannot publish stale snapshots;
  runtime commit semantics remain those of existing configd.
- Obtain the substituted encrypted hash from the password candidate, stage a
  same-directory temporary file containing only the target replacement, preserve
  or restrict original owner/group/mode, fsync and close it before committing.
  After commit, confirm RUNNING contains that candidate hash and the saved
  source inode/content has not changed, then atomic rename and directory fsync.
- Return success only after durable publication. Precommit errors leave both
  states unchanged and discard the candidate. A postcommit publication/check
  failure returns an explicit partial-change error and logs recovery guidance;
  do not blindly roll back a concurrent administrative change. Do not claim
  crash-atomicity across runtime and startup or global CAS for concurrent admin
  commits. Manual/uncooperative root writers cannot be serialized by advisory
  locking; source-change detection provides additional refusal, not a universal
  CAS guarantee.
- Saved plaintext-password, duplicate target paths, disabled target blocks,
  unknown syntax and symlink/nonregular/multilink startup files are refused.
  The normal CLI commit can persist a newly configured account before renewal.
  No plaintext password is written to startup or logs.

## Work split and versions

1. Root: login RPC selective editor/lock/staging integration and real filesystem
   tests. Login2.46 will require configd>=2.18 and vyatta-cfg>=1.35 so installed
   supported startup writers use the same lock. Existing schema/helper minimum
   2.45 remains sufficient because the validation interface is unchanged.
2. debian_image_prepare: shared-protocol configd2.18 SaveTo and legacy cfg1.35
   save locking, isolated clean branches, focused concurrency/error tests and
   ordinary suites. Preserve the older cfg safety branch and dirty distro.
3. integration_review: independently fix the observed state provider's unsupported
   BusyBox last stderr pollution; only its script/tests, disjoint from root's
   edits. Preserve tally errors and valid full-last ISO results.
4. debian_acceptance: actual provider/configd/CLI proof and renewed-password
   immediate reboot with no intervening CLI commit, plus target-only startup
   preservation and new failure/metadata tests on preserved-baseline overlays.

## Verification

First reproduce missing durable target update. Test strict grammar/path selection
with comments, escaped/quoted values, disabled unrelated settings, IPv6, missing
and duplicate targets. Assert all non-target bytes remain exact. Use real files
for atomic replacement, permissions, locks, contention and failed staging; inject
commit/publication failures without claiming rollback. Exercise different-user
renewals and a normal save waiting until after the password update before taking
its snapshot. Keep RPC serialization/Unicode/legacy-hash tests and PAM regressions.

Independently review changes, run package CI, verify metadata/artifacts and merge
exact passing heads. Build a new ISO using package-download cache only; discard
all filesystem bootstrap caches. Complete ownership/group/setid audit and all
fresh/install/upgrade/password/lockout/isolation gates before Arch implementation.
