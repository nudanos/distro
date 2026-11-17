# Live configd upgrade correction

The authentic baseline APT transition installed configd 2.16 and login/renewal
2.44, but retained the loaded 2.15 executable. Real expired-account renewal
then failed with `RPC failure: Authentication failure` and `Authentication token
manipulation error`. Recovery login succeeded and root, recovery and target
shadow-entry hashes did not change. The failed overlay and console transcript
are preserved by Debian acceptance. Reboot loaded the new executable; the
post-reboot real PAM renewal subsequently succeeded. Password refusal, unrelated
account checks and reboot persistence are still being completed by acceptance.

Ordinary debhelper restarts are suppressed. A separate YANG trigger can request
restarts, but this transaction did not trigger a successful refresh. The new
PAM helper requires corrected server peer identity, so an installed package
dependency alone does not establish a working live upgrade.

The focused correction in configd 2.17 invokes Debian's `deb-systemd-invoke
try-restart configd.service` after generated maintainer-script configuration,
only for an upgrade on a booted systemd system. This refreshes an active
process, preserves inactive services and honors `policy-rc.d`. A restart error
is propagated. An explicit administrator policy denial remains respected and
therefore can require a later administrator restart/reboot. Login 2.45 requires
configd >= 2.17. No shared daemon or PAM code changes are included.

## Regression evidence

`configd/tests/upgrade-restart.sh` ran in the disposable Debian helper against
real systemd and its installed Debian service-policy helper. It refuses an
existing configd unit and cleans up its temporary unit. Only unrelated
account/capability setup is stubbed.

- RED: unchanged source postinst exited 1 at the assertion `upgrading an active
  daemon kept the old PID`.
- GREEN: corrected source postinst exited 0; all five cases passed: active
  process replacement, stopped service preservation, fresh-configuration
  preservation, explicit policy 101 for `try-restart`, and restart failure
  propagation. The deliberately failing service emitted the expected systemd
  diagnostic during the last case.

The fixture's safe file creation and signal cleanup were independently reviewed
and corrected before the final five-case run, which again passed with exit 0.
Independent source review approved the final adjustment.

Archives `configd-upgrade-red.tar.gz` and `configd-upgrade-green.tar.gz` were
exported with LF-normalized Git worktree files and retained under
implementation. Source commits are `741a5815777ffe59845f21e4f7b4f0944873fe8d`
(configd) and `02edfdc07fab943717730e236f62942ad5224642` (login), both signed
off by Jon Fernandez. PRs are https://github.com/nudanos/configd/pull/2 and
https://github.com/nudanos/vyatta-login/pull/2; package CI runs are 42671494349
and 42671508054. Fresh package CI, actual baseline APT re-run and complete final
image acceptance remain required. These fixture
results are not a claim that appliance upgrade acceptance has passed.

Both initial CI attempts failed before creating any jobs or check runs. The
configd run's public annotation reports `Internal server error. Correlation ID:
bec4ae06-98c3-47c4-940f-96a0c56b8611`; no source-build failure was observed.
Both workflows were retried through `gh run rerun`, which returned success.
Passing package CI is still required before merge or final candidate assembly.

Update: CI run 42671494349 attempt 3 passed on reviewed head 741a581. Actual
interim-v2 APT from the preserved baseline returned 0, replaced the old running
daemon before reboot and passed real expired-password renewal. PR2 merged as
`4a10c5ad094f740545e00d1cbdf04b2fa934ebd1`; fetched source tree
`b139c3eb86e9003646d3f076b0c0be6939b69d70` exactly matches the reviewed head.
Final full-image checks remain pending.

Premerge CI annotation review found existing C audit-return/pointer-signedness
compiler warnings and a setup-go cache lookup warning (checkout is in a
subdirectory). GitHub's matcher labels some compiler-context annotations as
failures, but the actual package check concluded success. No Go/C source changed
in this focused packaging correction. Cache configuration and audit-warning
cleanup remain separate follow-ups.
