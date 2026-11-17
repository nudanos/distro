# Login operational-state provider fix

Status: focused test-first regression and actual appliance provider/configd/CLI
verification PASS. Acceptance owner performed the live check on its disposable
lockout overlay; final packaged-image validation remains a later gate.
Root owns branch, metadata, commit and publication. This agent edited only
`scripts/system-login-information`, `t/system-login-information.t`, and new
`t/lib/LoginProviderFixture.pm` in the shared `vyatta-login` worktree.

The appliance's BusyBox `/usr/bin/last` does not implement the requested
`-R --time-format iso` interface. Its usage diagnostics previously reached the
provider's stderr; configd collected provider output with stderr, invalidating
the JSON document and hiding otherwise valid faillock state.

Actual command characterization from acceptance's verified `last-inspect.sh`:
`/usr/bin/last` points through `/etc/alternatives/last` to `/usr/bin/busybox`;
BusyBox v1.37.0 returns exit1, empty stdout and usage text on stderr for
`last -R --time-format iso`. Recorded in
`test-runtime/acceptance-baseline/installed-boot.log`.

The fix captures only the optional history command's stderr, catches its spawn
failure and checks its exit status. Unsupported/unavailable history returns the
existing YANG-compatible `never logged in` fallback. Successful full-tool output
retains the existing ISO timestamp and exact-account selection. There is no
catch around the whole provider or tally reader; tally failures remain fatal.

`t/system-login-information.t` executes the complete actual provider script in
subprocesses with focused command/account fixtures. Original code fails on
history-warning stderr, failed/signaled history output and missing history
command; the changed code passes those cases plus correct ISO/no-record behavior.
The fixture keeps one failure counter present and verifies it survives fallback.
A separate tally failure remains nonzero with its diagnostic and no success JSON.
The normal package `prove t/` command discovers the new test automatically.

Evidence: `test-runtime/acceptance-final-fresh/provider-test-red.log` and
`provider-test-green.log`. Tests ran through the already-installed Git Perl under
the required MSYS execution permission, with no host dependency installation.
`git diff --check` passed. The same focused suite also passed on the appliance's
Linux Perl runtime (Files1, Tests9). Actual Linux IPC::Run3 behavior was separately
verified by the real operational provider, as described below.

Normalized candidate provider SHA256:
`d666da81d785548c1ed92a9444013d898dd6a6119b2980771c479469ea398b50`.
The three-file source/test archive is served at fresh-probe HTTP18771 as
`provider-source.tar`, SHA256
`f55e9dc7eab22a70055d7941e8db8542dce5bacb848d0759407a63d4bf55fc85`.
Acceptance preserved the installed original under `/tmp`, then installed this
exact source only into its disposable overlay. No service restart was performed.

## Actual appliance before/after

With one fresh valid faillock record for `locktest`, the original provider exited0
and its stdout JSON contained tally-counter1, but stderr contained7,991 bytes of
BusyBox usage. Configd's TreeGetFull omitted the user state and
`show system login users account-disabled` printed no entry.

After only the provider file replacement, the same valid failure record remained
count1. The provider exited0, stderr was exactly0 bytes, configd TreeGetFull
returned `locktest` with tally-counter1, and the actual operational command
printed `User locktest has 1`. The complete before/after transcript is
`test-runtime/acceptance-baseline/installed-boot.log`. This verifies the original
failure path through the real command, provider, configd and CLI rather than
only the mocked tests. Final package/image gates must still use the subsequently
built artifact; the temporary overlay script replacement is diagnostic proof.
