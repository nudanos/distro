# Password RPC automatic-save correction â€” November 18, 2026

The selective-writer design is superseded. Source inspection and a real
dispatcher regression established that configd's public Commit calls Save on
the normal success path. The initial Python outer writer lock would cause a
nested lock timeout after the runtime password had already changed. Its fake
peer omitted automatic save, so its passing tests did not establish integration
correctness. That checkpoint remains in Git history, with no release or merge.

There is also a distinct ordinary-nonfatal-error branch in
server/dispatcher.go confirmedCommitInternal: runtime can apply, Save is skipped,
and a warning string is returned without an API exception. The original password
RPC discards that string. This is a concrete possible explanation of the preserved
reboot failure; actual appliance attribution is still being investigated and is
not claimed by the source test.

Reviewed source checkpoint 9a622e2afbc6b44254b849dccb6aab111a3fe92b is published
on login revival/renewal-persistence, draft PR3. It preserves automatic-save
semantics and holds no startup writer lock across Commit. It verifies the
substituted candidate, rejects a nonfatal response with an explicit partial-change
diagnostic, verifies the applied RUNNING target, then verifies and syncs the saved
target using a read-only no-follow filesystem helper. It performs no compensating
Save or rollback. Unique private sessions and interrupted-setup cleanup prevent
stale PID-based candidates from being reused. Backend errors are not echoed with
potential credentials. The prior target-only/unrelated-unsaved-state promise is
withdrawn; normal successful Commit already saves the full running configuration.

The read-only verifier parses a single unambiguous active saved target, rejects
unsafe ownership/type/link count/modes, compares the hash without logging it,
fsyncs the saved file and its parent, rereads and checks metadata/content and final
path identities, and closes all descriptors with safe errors. It does not provide
a global CAS boundary with concurrent administrative commits or manual writes.
The package no longer requires experimental configd 2.18/cfg 1.36 versions; the
existing configd>=2.17 minimum remains for daemon/target identity support.

Verification: on the disposable Debian helper, 6 existing RPC tests, 10 new
automatic-save-aware RPC tests, 14 read-only verifier tests, and the normal Perl
suite (3 files/13 assertions) passed. The verifier's root ownership case separately
passed (14/14 as root); ordinary builder runs skip only that case. The new peer
actually takes a writer flock inside Commit and saves startup, reproducing the
outer-lock failure before the correction. It also models nonfatal warning/no-Save,
save exception after runtime mutation, saved mismatch and intervening runtime
change. The corrected native 2.46 package build completed with its normal tests;
the helper is installed beside the RPC as 0644 and RPC as 0755. Exact-head GitHub package run 44314453839 passed for 9a622e2. Actual
PAM/VCI normal and expired SSH renewal checks have since passed across reboot; the warning-path negative control remains pending. The earlier failure cause has not been established.

Normalized production source hashes:

- scripts/vyatta-password-rpc:
  00df845f5f52e2b336a686d70e9744b278192bf5939112da6e05022e9e58c7bd
- scripts/vyatta_password_persistence.py:
  316940c19f571c2553f5f4baae7463bf54bea637c7b900a585a768e812335d64
- Diagnostic source-only tar login-autosave-source.tar:
  08450427398a6ccd8e67b9a18dcb8db5dd340782d8d4fb82c2d7c52f27be48cf

This report contains no passwords, actual appliance password hashes, signing
private keys or raw credential-bearing transcripts.
