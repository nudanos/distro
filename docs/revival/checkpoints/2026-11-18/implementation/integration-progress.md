# Integration execution ledger

## Current corrected source â€” 2026-11-18

The rejected outer-lock design below has been replaced, not merged. Login
`9a622e2afbc6b44254b849dccb6aab111a3fe92b` is pushed and its exact-head package
CI 44314453839 passed. It retains configd's automatic Commit save, holds no writer
lock, checks nonfatal commit warnings, verifies candidate/RUNNING/saved agreement,
and syncs the saved file through a read-only verifier. It issues no extra Save or
rollback. Native package build and 30 Python tests plus Perl 3 files/13 assertions
passed; independent RPC and verifier reviews found no remaining source blocker.
See password-autosave-correction-report.md. Draft PR 3 remains unmerged until
packaged normal/expired renewal and the warning-path negative control are verified.

Ordinary nonfatal end-hook failures are a specific exception to automatic save:
configd can apply RUNNING, skip Save and return a warning without an API error.
The original RPC ignored it. This is a source-proven possible cause of the old
failure, not yet an actual-appliance attribution. A live original-RPC observer
logs only constant boundaries, return classification and lengths, never values.
Controlled original 2.45 normal and expired pre-session SSH renewal both saved
successfully, returned no nonfatal warning, and survived guest shutdown plus
same-disk cold boot (WHPX faults at restart). Prior password was rejected and new
accepted in both cases. The historical failure is NOT REPRODUCED in those tests;
its prior causal attribution is withdrawn. Do not claim a proven historical fix.

Exact CI login 2.46 package is installed on that disposable child, audit clean,
with source-matching RPC/helper hashes. Its normal in-session renewal passed saved
byte comparison and same-disk cold-boot credentials; expired pre-session renewal
passed the pre-reboot saved-byte check and is awaiting its reboot verification.

Writer components are merged: cfg f665e20 CI 44311837662 passed and merge
5ad5a8649b3fd02a304483c50cd6afed6542b23e tree231c142cd2b1d93f790ce8aa2682b99df093adc2
matches the reviewed head. Configd final826248c fixed the destination-validation
race, passed independent review and CI44316942009, and merged as
a58fbcb3afe23aa5867dcca2d1b34e2c4064a51e; tree965e1a9758f1b93c6a982603438643d02740de1f
matches the reviewed head. Login does not require those newer writer versions.
No final Debian
acceptance, corrected image or native Arch implementation is complete.

## Authoritative correction â€” 2026-11-18

The GitHub-first requirement is complete: all 14 source worktrees were published
and clean, and distro commit98ec670 preserves the reviewed 400-file evidence
checkpoint with verified manifest. New tested WIP source is also published:
login e725440 (PR 3), configd 1bd489e (PR 3), cfg f665e20 (PR 1). All three PRs are
drafts and MUST NOT be merged as the current persistence design.

The proposed selective startup editor passed isolated filesystem/RPC tests and
a native Debian2.46 package build, but integration exposed a false assumption in
the design and its fake configd peer: successful public configd Commit already
calls Save in server/dispatcher.go confirmedCommitInternal (including merged
2.17 and its predecessors). A live API domain-name commit also persisted startup.
Holding the proposed Python startup lock across that Commit nests the lock;
the stronger real-dispatch test applied RUNNING then timed out in automatic Save.
The earlier approval is superseded. Do not remove existing auto-save semantics
merely to make the new helper work. The actual original PAM path and historical
reboot failure are being reconciled before selecting a smaller correction.

The prior claims below that ordinary raw API Commit leaves unrelated unsaved
state are incorrect for this source. The historical reboot failure overlay is
preserved, but its causal attribution to a missing RPC Save is not established.
No final Debian acceptance, corrected ISO, or native Arch implementation is done.

Verified local component checks: login6RPC+24helper(also24/24asroot)+10integration
and Perl 3 files/13 assertions; native 2.46 package build includes RPC755/helper644.
Configd full Go tests and legacy46save assertions pass; all six ordered
Python/Go/Perl private-fixture flock pairings pass. These do not substitute for
the newly failing commit/save integration. Package CI is running on GitHub.

## Authoritative resume checkpoint â€” 2026-11-17

The user requested that all work be backed up to GitHub before further
implementation. Source checkpoints and a curated evidence backup are being
published first; no new persistence implementation has started.

The four original PR1s and three corrective PR2s are merged as recorded below.
Those component fixes passed review and CI. Complete Debian image acceptance
has **not** passed, and native Arch implementation has **not** started.

Blocking findings supersede the earlier optimistic persistence statements:

- Password renewal updates RUNNING/shadow but does not update config.boot.
  With no intervening CLI commit/save, a clean reboot restores the old password
  and rejects the replacement. Earlier persistence evidence was masked by a
  subsequent CLI commit, which autosaves the full configuration. The failure
  overlay and transcript are preserved; see password-persistence-plan.md and
  password-persistence-design-review.md. A target-only repair is designed but
  not implemented or validated.
- The attempted 1.0~20261109.1 image is rejected: Docker cp -a lost nonzero
  file groups in its bootstrap cache, producing root-group setgid utilities.
  No such image was accepted or released. All filesystem bootstrap-cache reuse
  is abandoned. A fresh package-download-cache-only bootstrap passed the full
  root/shadow/crontab group, mode and setid preflight. Future images must start
  with a fresh bootstrap and pass independent final ownership audit.
- The blank login failure-counter display was traced to BusyBox last usage
  on inherited stderr corrupting the provider's JSON. The focused provider
  fix and tests passed and were verified on a disposable appliance. They were
  uncommitted before this resume and are now checkpointed separately; the
  eventual package/image still needs verification.

Historical evidence below remains useful but is not an assertion that current
image, reboot-persistence or Arch gates are complete. The next authorized work
after GitHub backup is the persistence repair, independent review/tests/CI,
fresh corrected image acceptance, then native Arch work in that order.

The user approved the sequence on November 8, 2026. Complete the four PRs through CI, review and merge, build and exercise a Debian image containing those commits, then implement native Arch packaging and PAM integration.

## Execution constraints

- Preserve existing dirty distro work. Build the candidate from a separate checkout and fresh guest work directory.
- Use exact merged component commits and record resolved source refs and artifact hashes.
- Use disposable Linux guests for rootful package, image and PAM checks.
- Merge only the reviewed head after its applicable GitHub checks pass. Preserve signed-off commits with a merge commit.
- Complete Debian image acceptance before implementing the Arch port.
- Candidate image validation includes install, upgrade, reboot, password renewal, lockout recovery, and a real sandboxed login.
- The user authorized opening and merging the four PRs and the implementation needed for the three steps. Release publication is outside this execution.

## Step 1

- [x] Four PRs opened against trixie.
- [x] Independent source review approved final heads, including the host dependency correction in `c7212c2`.
- [x] GitHub package builds pass on all final heads.
- [x] Merge reviewed passing heads and verify resulting trixie commits and source-tree equality.

| Repository | PR | Reviewed head | Initial CI run |
| --- | --- | --- | --- |
| vyatta-login | https://github.com/nudanos/vyatta-login/pull/1 | c7212c2ea6ea293bfcb1ead48c2d5687c1c64d22 | 42535159479 |
| vyatta-debian-pam-configs-config | https://github.com/nudanos/vyatta-debian-pam-configs-config/pull/1 | 541f763b0bd19f20c8eaa470ff60bd975cb27a49 | 42534693937 |
| cli-sandbox | https://github.com/nudanos/cli-sandbox/pull/1 | 8c71637cdd49227b394deadfbf477b7c20a5d65f | 42534710912 |
| configd | https://github.com/nudanos/configd/pull/1 | b63160dd18e750102c7f04f655de3d16865cab23 | 42534721985 |

All four are merged. Final trixie revisions are `ea6eb3f0342b342eaf374219772be45081c4aea0` for PAM configuration, `c6a637f7dfc28f37993aafa2f1bd65fca4d5a674` for sandbox, `390d7bf6df5023fce63c9dc9878ba1d66e83ea29` for configd, and `783ff2120a3b7894275cff4fe160565e2fdb4bc1` for login. Remote source-tree hashes were compared with the reviewed and CI-tested heads and matched in every repository.

The upgrade review added host dependency `configd (>= 2.16)` in `c7212c2` and its fresh CI run passed before merge. Baseline configd maps unset audit login identity to root, so the new pre-session password helper requires the corrected server. Renewal retains its daemon-free dependency closure for sandbox-only installation.

## Step 2

Current checkpoint: all three corrective PR2s are reviewed, CI passing, merged and remotely fetched with exact reviewed-tree equality. Real interim-v2 baseline APT, pre-reboot loaded-daemon refresh and expired-password renewal passed. Both solver directions selected paired login/schema 2.45. Actual mixed-stack CLI commit was rejected before effects, preserving running/boot/PAM/shadow state and leaving the staged OS user absent. Legacy old-root real serial/SSH isolation, cross-user denial, existing-alias reuse, teardown and in-session password change through the new host-bound module passed. Final CI-input repository and ISO `1.0~20261109.1` are now being built. Final full-image gates are still pending; Arch implementation has not started.

| Final component | Version | Merged trixie SHA | Verified tree | CI run / artifact |
| --- | --- | --- | --- | --- |
| configd | 2.17 | 4a10c5ad094f740545e00d1cbdf04b2fa934ebd1 | b139c3eb86e9003646d3f076b0c0be6939b69d70 | 42671494349 attempt 3 / 13699582406 |
| vyatta-login | 2.45 | e37397bbbab364f66724e0267a50f91c22954ea0 | 6d779be638695942ed87166cf0ef06759543443e | 42674775264 / 13699842259 |
| cli-sandbox | 0.33 | 7d693f126f834bb8d733008b4e2d545a014ce26f | 8ae52d5bbec58c17a0d6bb625b90c3c76c74732d | 42676853188 / 13701535311 |
| Debian PAM configuration | 1.13 | ea6eb3f0342b342eaf374219772be45081c4aea0 | e318851fc7f52607b37e8a34fb38e6a9b00b6e5e | 42534693937 / 13638448199 |

Final image ownership: debian_image_prepare builds and audits ISO/repository; integration_review owns a new final fresh-install/installed reboot/fresh sandbox-root fixture; debian_acceptance owns normal image upgrade, package upgrade/guard and lockout/recovery. Final fresh SSH2226/QMP18768/HTTP18771; interim SSH2228/QMP18770/repo HTTP18769; builder SSH2222; Arch SSH2224 remains reserved. No host features/settings were changed.

Sandbox EEXIST reuse and teardown subsequently passed; both units inactive/dead/success, no runtime mounts, audit clean and old-root package inventory unchanged. Root completed final source/test/docs review and authorized the agent's signed-off feature-branch commit/push. GitHub PR/CI and in-session old-root renewal with corrected host packages remain pending. Its helper creates only the validated controlling terminal's root-owned mode-000 alias; actual unprivileged opens are denied even after the host console changes users. See serial-terminal-report.md for traces, hashes and explicit limits. Full final-image install, normal image upgrade and fresh-root acceptance still follow package-level tests.

- [x] Prepare candidate inputs, verified signed repository and preserved baseline ISO for upgrade setup.
- [ ] Build Debian image with merged component sources.
- [ ] Verify install, upgrade and reboot.
- [ ] Verify target-account password renewal, lockout/reset/root recovery, and real isolated login.
- [ ] Independent review of image evidence and source fixes discovered by acceptance.

Image preparation is owned by debian_image_prepare. The build needs a rootful container engine and fresh work tree. The existing nightly workflow publishes a prerelease and will not be used for a candidate build.

The candidate repository uses test key fingerprint `632DE3028B010AD7971CF88FC8FF6AE6F7B0295F`. Its metadata, all 648 binary payloads, and source payloads were verified after assembly. It is handed off at `test-runtime/candidate-repository/repo`. The image uses the stock script, recorded Debian trixie base digest, and isolated output. The earlier TCG attempt and logs were preserved. Existing WHPX was confirmed with a diskless QMP startup and clean exit, then real guest boots. No host features were changed. The helper restarted with WHPX and a clean image attempt using explicit version `1.0~20261109` plus saved verified package cache.

Baseline controller defects were corrected and independently reviewed, including UART pacing, locked buffer handling, true output markers, command lifetime, suspension-aware waits, and actual systemctl polling. In the rescue-booted live baseline, system-configure completed successfully, created vyatta, and normal vyatta/vyatta login succeeded. The only rescue change was a temporary root hash; no target account repair was made. Earlier refusals preceded account creation. This is useful fixture evidence, not an unmodified stock-boot pass. The genuine installer completed, powered off, and its preserved qcow2 passed image checks. A separate installed-baseline overlay booted and accepted its configured account. The installed baseline is retained read-only; candidate acceptance remains pending. Acceptance details and transcripts are in debian-acceptance-report.md.

The baseline administrator role intentionally lacks sudo; the test uses a separately CLI-configured superuser for package and recovery actions. Candidate repository signatures verify on the installed baseline. Its first APT simulation required the ordinary signed Debian trixie source for python3-crypt-r, which is not part of the NuDanOS-only repository. With that signed source enabled, the actual transaction configured configd 2.16, login/renewal 2.44 and PAM configuration 1.13; dpkg audit is clean, module ownership transferred to renewal, and the administrator's PAM comment and file hash were preserved. Console input loss prevented reliable capture of the APT process status, so this proves configured package state rather than a recorded successful APT exit. The old daemon remained PID 1464 with its original loaded executable while the installed binary changed. A real pre-reboot expired-account test is pending. Ordinary debhelper upgrade restart is suppressed, but YANG triggers can request a restart and ignore failure; actual process evidence is required for each transaction. Also test the normal image upgrade on a separate baseline overlay.

The pre-reboot expired-account renewal test failed with an RPC authentication error while the old daemon remained loaded. Recovery login and unchanged root/recovery/target shadow-entry hashes were verified; a read-only failure overlay was preserved. Reboot loaded the new binary. Root is implementing a focused configd 2.17 active-service upgrade refresh and matching login 2.45 dependency; see configd-upgrade-fix.md for red/green real-systemd regression evidence. New package CI/review/merge and the authentic APT re-run remain pending. No final image acceptance pass is claimed.

The provisional ISO finished building and its package audit passed. Inspection traced six 0775 directories to the isolated checkout's 0002 umask; the baseline uses 0755. Final build inputs will use umask 0022 and normalized include-directory modes, preserving file permissions. The final ISO must incorporate the upgrade correction and pass independent ownership/mode, capability, signature, package and live appliance checks. A separate legacy 0.31 isolation overlay is owned by integration_review; its root was built by the normal old package hook and first actual session is under investigation.

Post-reboot provisional password renewal, old-password refusal and changed-password persistence through another reboot passed, with unrelated account hashes unchanged. Two more real integration defects were found: lockout could be committed/saved despite a nonfatal end-hook refusal (and a simultaneous user was not created); isolated serial login fails when login reopens absent /dev/ttyS0. The login correction adds nonmutating prerequisite validation in configd's validate phase; red/green real-PAM namespace tests, Perl and RPC tests passed and independent source review approved. Login PR2 head is now e3fd54a588e241c4641c5ed5ddf1a8788b511e28 (2.45). Its actual appliance commit gate remains pending. integration_review owns a narrowly bounded serial-terminal fix and its tests; old-root password SSH isolation already passed and its clean baseline was preserved.

Configd PR2 head 741a5815777ffe59845f21e4f7b4f0944873fe8d (2.17) has passed a local stock-builder package build and generated-postinst placement inspection. GitHub Actions failed internally before creating jobs on both initial PR2 runs and their retries; the official status page confirmed an Actions partial outage (incident tp8cghg7tlc4). Do not mistake this for a source-test failure or bypass the passing-CI merge requirement. Local exact-source package builds and provisional appliance checks continue while CI is unavailable. No final candidate has been assembled or accepted.

CI subsequently resumed: both current configd and login heads passed. Their CI artifact digests, `.changes` payloads and critical metadata were verified; CI and local package bytes differ and are not claimed bit-identical. The final image still waits for actual corrected upgrade/commit checks, reviewed sandbox 0.33, CI and merged source-tree verification. Live lockout, private tally records, correct-password refusal after two failures, recovery-user access, operational reset and subsequent login passed on the provisional upgraded appliance after the explicit supported `pam-auth-update --disable tacplus sss` prerequisite on a confirmed local-only configuration. CLI local auth-chain selection alone did not remove those profiles. An operational display discrepancy is being bounded separately and is not a reset failure.

Two scratch image attempts were rejected. The first inherited an excessive container file-descriptor limit that made APT cleanup scan invalid descriptors; a per-container nofile limit corrects that environment issue. The second exposed a recursively chowned bootstrap cache with incorrect root filesystem ownership. Its successful package checks do not make it a valid image. Rebuild debootstrap using only cached package files, with a hard ownership/mode and dpkg-audit gate before further image construction, followed by independent final squashfs checks. The local acceptance hook includes passive sandbox packages 0.32 without activating user isolation in the build chroot; activation belongs to live appliance acceptance.

## Step 3

- [ ] Finalize native package source closure and adapter design using the Debian acceptance result.
- [ ] Implement and test native Arch packages and PAM adapter while retaining Debian behavior.
- [ ] Exercise native package install, upgrade, removal and real login with production client libraries.

Read-only source discovery is complete. See arch-integration-task-graph.md for the seven-task implementation graph, source archive comparisons and unresolved model dependency boundary. Implementation waits for Debian acceptance.
