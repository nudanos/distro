# Lockout commit validation correction

Real upgraded-appliance acceptance reproduced a committed but unenforced policy:
`auto-disable attempts 2 duration 300` and a new user appeared in running and
saved configuration after the updater refused the mixed PAM stack. No faillock
rules or profiles were installed and the new OS user did not exist. The refusal
ran from `configd:end`, whose failure is nonfatal. The failed overlay and
transcript are preserved in the primary acceptance report.

Login 2.45 now invokes the existing updater with `--system --validate` through
the login container's `configd:validate` hook. This mode performs the existing
prerequisite checks and exits before account or PAM-profile mutation. The end
hook retains its checks for runtime safety. Unsupported configuration must be
rejected before commit effects; supported local-only PAM prerequisites remain
required. No unsafe mixed-stack support is added.

Evidence:

- RED: invoking validation on the original updater printed `Unknown option:
  validate`, continued into activation and changed common-auth, failing the
  nonmutation assertion.
- GREEN: the complete private-namespace real-PAM regression exited 0, including
  nonmutating validation, remote/custom-stack refusal, migration/rollback,
  expiry, consecutive-failure enforcement, reset and root exemption.
- Both Perl tests passed (4 assertions), and all 6 password-RPC tests passed.
- Independent source review approved the diff, conditional on the actual
  appliance commit rejection gate proving the YANG hook is invoked.

The validation commit is `e3fd54a588e241c4641c5ed5ddf1a8788b511e28`. A subsequent
review caught the split-package pairing hazard: a new schema requires the new
validation-only helper, and a new helper must not retain the old unguarded
schema. Commit `591f71e90316394bca2cc045804bca42cf284143`, tree
`6d779be638695942ed87166cf0ef06759543443e`, adds YANG Depends login >= 2.45 and
host Breaks YANG << 2.45, without a circular dependency. These constraints were
independently reviewed. It also documents the tested supported Debian PAM
profile-manager prerequisite. Both commits are signed off and published in
https://github.com/nudanos/vyatta-login/pull/2. The local stock-builder package
build and GitHub CI 42673382763 passed on the earlier e3fd54a head; its binary
declares configd >= 2.17 and extracted YANG contains the validation hook. That
pass is not a pass for the new head. A separate interim-v2 repository and fresh
baseline tests must verify the revised package solver behavior in both upgrade
directions, actual commit validation, and the complete APT transition. Latest
head CI, merge and final image acceptance remain pending.

Update: latest-head CI 42674775264 passed. On a fresh preserved-baseline overlay,
both host-only and schema-only APT simulations selected compatible host/schema
2.45 and configd 2.17. The real minimal host upgrade returned 0. Actual mixed-PAM
CLI commit returned `Commit failed!`; after discard, running config and saved
config/PAM hashes were unchanged and the staged user was absent from passwd.
Unrelated shadow hashes were unchanged. PR2 merged as
`e37397bbbab364f66724e0267a50f91c22954ea0`, verified tree
`6d779be638695942ed87166cf0ef06759543443e`. Final image acceptance remains pending.
