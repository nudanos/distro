# Password persistence design audit — 2026-11-09

Read-only source audit; no implementation. Confirmed runtime finding supplied by
main acceptance:2.45 password renewal changes live credentials, but startup config
is unchanged and the old password returns after a clean reboot without CLI save.

## Existing paths and limitations

- `vyatta-login/scripts/vyatta-password-rpc`: ChangePassword.setup/set/commit/
  teardown does not save startup configuration. Identity comes exclusively from
  VCI metadata; current password is verified against RUNNING.
- `configd/cmd/cfgcli/runfns.go:206-229`: normal CLI commit explicitly invokes
  `saveRunInternal` after `Client.Commit`; lines585-598 call Client.Save and mark
  the session saved. Bare `save` is a no-op at575-579 because CLI commit saves.
  Calling generic Save from the password RPC would save ALL committed running
  configuration, including unrelated API-committed/unsaved changes.
- `configd/server/dispatcher.go:1331-1342`: Save raises logical configd privileges
  (unredacted whole config) then calls SaveTo(`/config/config.boot`).
- `configd/server/config_mgmt.go:159-188,221-229,295-342`: SaveTo snapshots all
  RUNNING state into a synced temporary file, then local copy uses `cp -T` as
  caller/privileged daemon. There is no startup-file serialization lock in these
  methods; destination replacement is not the same-directory fsync+rename design
  of the legacy Perl save utility.
- `vyatta-cfg/scripts/vyatta-save-config.pl:93-151`: legacy save writes full active
  config, restrictive umask, same-directory tempfile, file fsync, rename and
  directory fsync. It also has no shared startup-file lock.
- `vyatta-login/scripts/vyatta-change-password`: recovery utility parses saved
  configuration with XorpConfigParser, replaces/adds a user password, emits the
  whole tree into `/tmp`, then File::Copy::move to config.boot. No lock, checked
  fsync, same-filesystem atomicity, or robust duplicate/missing-leaf validation.
  Do not call it unchanged for online renewal.
- Actual parser is in `nudanos-review/repos/vyatta-config-migrate/scripts/
  XorpConfigParser.pm`, not the test stub under vyatta-cfg. `get_node`290 and
  `create_node`142 expose mutable children/name nodes. Full output423-479 rewrites
  formatting/comments; more seriously `! ` is parsed into `disable` at624-631,
  but output never emits that flag. EOF validation also does not reject all
  unclosed constructs. Whole-file round-trip is unsuitable for a promise that
  unrelated saved settings remain unchanged.
- `configd/session/session.go:335` session locks protect one candidate; they do
  not lock startup-file writers. `session/commitmgr.go:209-241` serializes/rejects
  overlapping commits only for the duration of a commit. SaveTo is outside that
  channel. No reusable startup-config file lock was found in the audited paths or
  repository-wide flock/LOCK_EX search; do not treat candidate locks as one.
- `provisiond/debian/provisiond.component`: launches through `lu -user configd`.
  `provisiond/provisiond.go:267-315` executes legacy RPC helpers with metadata in
  the environment and no further credential drop. The configd service similarly
  uses lu; `configd/configd.go:56-61` RaisePrivileges only changes an authorization
  flag, not Unix UIDs. Actual runtime real/effective/saved UIDs and config.boot
  ownership were requested from acceptance and should be confirmed before file
  ownership/privilege assumptions are implemented.

## Smallest defensible target-only contract

1. Edit the SAVED configuration, never serialize RUNNING wholesale. Require an
   unambiguous existing saved target user before changing runtime; reject an
   unsaved newly-created account with an explicit instruction to persist its
   account configuration first. Do not silently create its level, keys or other
   unsaved account attributes. Preserve every unrelated saved byte/semantic.
   Use a validated targeted representation/edit, not whole Xorp re-output.
2. All online password writers and normal startup Save/SaveTo writers need one
   stable shared lock acquired BEFORE reading startup or snapshotting RUNNING.
   Lock a dedicated never-renamed file, not config.boot's replaceable inode.
   Use no-follow/root-controlled directory/file checks and bounded contention.
   Legacy online save writers need the same contract; an advisory lock used by
   only the new helper cannot prevent lost updates. Offline recovery is separate.
3. While holding that lock: verify the current credential again, read/validate
   startup target, generate a candidate with the new target hash, stage the saved
   target-only edit on the same filesystem and fsync it before runtime mutation.
   Publish with atomic rename and directory fsync, preserving/restricting normal
   owner/mode and rejecting symlink/nonregular/unexpected ownership entries.
4. The file lock alone does NOT make runtime commit+startup publication atomic:
   ordinary commits remain independently serialized. Do not hold a new external
   file lock while calling a server operation that also takes that lock (deadlock).
   A correct rollback needs a server-side compare-and-set/commit coordination
   boundary so it cannot revert another user's intervening password change.
5. Preflight/staging failure must leave both states unchanged. Do not return PAM
   success until runtime and durable startup updates succeeded. If publication
   fails after runtime commit, either perform coordinated target-only rollback
   (checking the applied generation/hash), or explicitly report partial failure
   and retain durable recovery information. Blindly restoring the whole old
   config file/tree can erase concurrent administrative changes. Crash-atomic
   guarantees across both resources require a recovery protocol; do not claim
   those guarantees merely from fsync+rename of one file.

## Focused tests

- Saved A / unrelated running-unsaved B: renewal changes only saved target hash;
  B remains unsaved, comments/disabled/quoted/IPv6/other-user content preserved.
- Missing saved user, malformed/ambiguous target, symlink/nonregular startup and
  permission/stage-write failure: refuse before runtime mutation.
- Same-user concurrent renewals verify inside serialization; different-user
  renewals preserve both changes; concurrent normal save cannot publish a stale
  pre-renewal snapshot. Exercise ordinary same-user admin commit too.
- Inject commit, rename and directory-fsync failures; verify documented rollback
  or partial-failure behavior, no false success, no unrelated state restoration.
- Actual expired pre-session and in-session renewal, then immediate clean reboot
  with NO intervening CLI/config commit: old password rejected/new accepted,
  unrelated unsaved state still absent after reboot.

Root owns final runtime/startup transaction design and implementation. This audit
does not approve a helper-only lock or full Xorp startup rewrite.

Runtime identity follow-up from acceptance (read-only probe
`acceptance-baseline/probes/provisiond-identity.sh`, `installed-boot.log`):
provisiond PID1512 has real/effective/saved UID/GID all0 and
CapEff000001ffffffffff. `/config` is root:vyattacfg2770; `/config/config.boot` is
root:vyattacfg0600 regular. Thus a lock within `/config` must account for that
group-writable directory; place a stable lock in a verified root-controlled
directory or reject hostile/symlink lock entries and ensure normal group members
cannot unlink/replace the lock inode. The RPC script executes root despite the
`lu -user configd` service command text.
