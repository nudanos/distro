## Task 3: Native authentication/package safety

**Owner/Files:** vyatta-login, cli-sandbox, vyatta-debian-pam-configs-config only.
**Consumes:** current password RPC/PAM and Debian package lifecycle.
**Produces:** supported password verification, serialized RPC input, supported lockout integration, failure-aware isolation activation and reversible owned PAM changes.
- [ ] Reproduce password RPC crypt import failure and quote/backslash payload defect with actual helper tests.
- [ ] Implement a declared supported hash-verification dependency/API and proper JSON serialization; cover incorrect/expired/long/unicode passwords.
- [ ] Replace obsolete pam_tally integration with supported lockout semantics and reject missing prerequisites before activation; preserve recovery.
- [ ] Make sandbox activation/deactivation fail on command/prerequisite errors; explicitly reject unsupported OS integration until native adapter exists.
- [ ] Correct diversion paths and remove only owned PAM edits; test install/remove with administrator edits.
- [ ] Parameterize PAM module destination for target packaging; run tests, commit and write task-3-report.md.

