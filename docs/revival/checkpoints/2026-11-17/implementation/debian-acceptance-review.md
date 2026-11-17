# Debian acceptance audit — 2026-11-09

Disposition: candidate acceptance remains pending. Do not begin Arch implementation on the strength of package builds, namespace tests, or baseline installation alone. This audit reviewed source and reports; it did not run appliance tests or change product/test scripts.

## Observed upgrade state and outstanding renewal probe

The acceptance owner reported configd 2.16, login/renewal 2.44 and PAM configuration 1.13 installed with a clean `dpkg --audit`. The running configd remained baseline PID 1464 with executable hash beginning `4e02`, while the installed binary hash began `8001`. Full hashes and actual commands belong in the acceptance transcript. Thus installed package version does not establish that the identity fix is active.

The pre-reboot expired-password probe was still unresolved when this audit completed. Preserve its actual outcome without manually restarting configd first. Repeat the same managed fixture's renewal after reboot, verifying that only its configuration/shadow entry changes, the previous password is rejected, and the replacement password logs in after reboot. Keep an independent recovery session available.

Source behavior explains the risk: baseline configd `b63160d^:server/conn.go` assigns `getLoginUid()` directly to the socket credential; unset audit identity returns UID zero with an error that `Handle()` permits. The new PAM child sets real/effective/saved target IDs before connecting (`vyatta-login/src/pam_vyatta_passwd/pam_vyatta_passwd.c:48-59`), but that cannot correct the old server's identity selection. Configd exports `ctx.User` as RPC metadata (`configd/server/dispatcher.go:2004`); the password RPC takes that metadata user and selects its hash and change target (`vyatta-login/scripts/vyatta-password-rpc:59`, `:79`, `:117`). Login and SSH set audit login identity in the session phase (`vyatta-debian-pam-configs-config/etc/pam.d/login.vyatta:105`, `sshd.vyatta:26`), so pre-session forced renewal can reach the old behavior. A wrong-user verification failure is a concrete source-supported hazard; the actual appliance result must still be recorded. This is retained old-server behavior, not evidence of a newly demonstrated authentication bypass.

## Restart-policy qualification

`configd/debian/rules:45` suppresses ordinary systemd upgrade restart, and `:49` suppresses init stop/start behavior. However, `configd/debian/configd.triggers:1` declares interest in `/usr/share/configd/yang`, and `debian/configd.postinst:40-43` restarts configd, provisiond and vyatta-opd on the trigger, ignoring restart failure. Therefore a blanket claim that this package never restarts during upgrades is unsupported. Observe the actual process/executable before APT, after APT, and after reboot.

No documented rationale establishing a general reboot-only upgrade policy was found in the inspected packaging, README or changelog. Available local history is shallow at `240e958`. No reboot-required marker or runtime version handshake was found in the inspected configd/login packaging and client path: `vyatta-cfg/src/client/connect.c:79-126` opens the connection without negotiating a server version; `configd/rpc/rpc.go:19-26` defines requests with method, parameters and ID. The host `configd (>= 2.16)` dependency constrains installed packages, not the running process.

## Legacy sandbox coverage gap

The installed baseline described in `debian-acceptance-report.md` had no sandbox package. Upgrading that fixture and subsequently enabling 0.32 tests fresh-root activation, not an old sandbox-root upgrade.

Use a separate preserved-baseline overlay to install the retained signed 0.31 sandbox packages and old optional isolation feature/renewal 2.43. Establish a working isolated session before upgrading and record old-root package inventory and identity. Upgrade the host while preserving that root, then test a real isolated login and forced password renewal. This exercises the changed fallback host-module bind in `cli-sandbox/scripts/cli-sandbox:343-349`, including whether the old root has the module's required libraries. It must not be replaced with a freshly generated root and called a legacy-root pass. Fresh activation remains a separate gate (`scripts/cli-sandbox-create:8` preserves an existing root; the optional feature's postinst creates/enables only on first configuration).

## Passive hook and remaining gates

The local `implementation/12-test-passive-sandbox.chroot` SHA256 was independently verified as `c1c7007e3b8b4ac33354204856a3901de475f8e12efdb9c952896a7c3f62f4c5`. Its line 5 installs only cli-sandbox, pam-sandbox and python3-shared-storage 0.32. It does not request the optional isolation feature. Record the builder's copied-hook hash and verify final ISO package versions, absence of optional activation and effective PAM inactivity. This is an explicitly modified acceptance image, not evidence that the stock router metapackage selects isolation.

Fresh candidate installation/installed boot, complete APT transition and reboot, normal image upgrade on a separate baseline overlay, deployed renewal, lockout/reset/root recovery, and actual isolation boundaries/teardown remain individual acceptance gates. APT success does not prove the normal image-upgrade path. Retain configuration and credential checks after both upgrade routes. The current report appropriately leaves candidate gates pending; no new pass is asserted here.
