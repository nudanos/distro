### Task 1: REST authentication boundary

**Owner/Files:** vyatta-rest only; scripts/vyatta-webgui-chunker-aux, src/server/authbasic.cc, src/server/authsession.cc, existing test/build integration and minimal new security helpers/tests as needed.
**Consumes:** existing Basic and session auth interfaces.
**Produces:** same supported auth API with explicit-length decoding, no credential logging, protected session authority and Linux regression tests.
- [ ] Trace decoder/parser/session issuer/startup ownership and all supported users.
- [ ] Add behavioral regressions for empty/invalid/max-length Basic headers, embedded NUL, colon/password behavior; demonstrate current failure under sanitizers.
- [ ] Add Linux tests proving an unprivileged UID cannot replace session authority and legacy valid session behavior still works.
- [ ] Implement bounded decoding and restrictive root/service-owned runtime state; reject unsafe/symlink state rather than trusting it.
- [ ] Run focused and existing tests, inspect diff, commit and write task-1-report.md.

