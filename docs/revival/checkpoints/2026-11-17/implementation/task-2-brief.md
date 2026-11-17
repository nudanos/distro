## Task 2: Management daemon identity/input/recovery

**Owner/Files:** configd and vci only: configd/server/conn.go and tests; configd/session/commitmgr.go and tests; vci/dbus_transport.go, policy and tests.
**Consumes:** existing peer credentials, audit attribution, RPC method maps and VCI bus.
**Produces:** invalid-argument errors rather than panic; peer identity retained on unset attribution; safe signal delivery; observable atomic recovery persistence.
- [ ] Pin behavioral null/type/signature tests before changes, preserving permitted RPC conversion behavior.
- [ ] Reproduce unset-loginuid mapping with injected/pure credential resolution tests; trusted internal callers retain documented behavior.
- [ ] Reject malformed signal bodies and constrain producer authority where the actual bus/model ownership supports it; do not invent an unchecked trust list.
- [ ] Add recovery persistence failure/restart tests, atomically replace state and propagate errors using the existing response contract.
- [ ] Run focused/existing tests and package checks, commit per repo and write task-2-report.md.

