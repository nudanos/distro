# Login configd dependency correction — 2026-11-08

Repository: `vyatta-login`, branch `revival/native-linux`.
Base: `7e3b45e08fb999c44e6ee119ef05894acda02933`.
Correction commit: `c7212c2ea6ea293bfcb1ead48c2d5687c1c64d22` (`Require configd peer identity handling for login`).
Author and sign-off: Jon Fernandez <jon@fernandez.tech>.

The host `vyatta-login` binary package now requires `configd (>= 2.16)` in `debian/control`. The current unreleased 2.44 changelog has one additional bullet explaining the target-user password RPC identity requirement; its version was not bumped. No renewal-package dependency or source implementation changed.

The dependency ties the native PAM module's target-user privilege drop to the server's corrected peer identity handling. Baseline configd can map an unset audit login UID to root; the new PAM child sets the target UID before the password-management RPC, while pam_loginuid runs only in the session phase. Requiring the corrected server on the host prevents that unsupported combination. The sandbox renewal package intentionally does not depend on the configd daemon.

Verification completed before commit:

- Parsed all 10 Debian control paragraphs and unfolded fields; compared every field with the base revision. The only changed field was `vyatta-login/Depends`.
- Verified exactly one host dependency `configd (>= 2.16)`, preservation of every preceding host dependency and its ordering, and the existing `vyatta-password-renewal (>= 2.44)` minimum.
- Verified the complete `vyatta-password-renewal` paragraph remained unchanged, contained no configd dependency, and retained `Breaks`/`Replaces: vyatta-login (<< 2.44)`.
- Verified the changelog still starts with version 2.44.
- Ran `git diff --check` successfully and inspected the complete diff: two files, two insertions. Git emitted only its configured LF/CRLF conversion notices.
- Confirmed the committed author, committer, requested `Signed-off-by`, and clean repository working tree.

No mirrored source test was added. Prior 2.44 package tests are recorded separately; this metadata correction did not rerun or extend them. Candidate-package dependency generation and real apt resolution remain for CI/full-image integration, so these checks do not claim that acceptance result. No push was performed by the implementation agent.
