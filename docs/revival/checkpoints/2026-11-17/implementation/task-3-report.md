# Task 3 — Native authentication and PAM package safety

## Scope and decisions

- Password RPC verification uses the declared Debian `python3-crypt-r` package and its `crypt_r.crypt` API. It uses the complete stored libxcrypt setting, rejects locked markers and unusable credentials, and compares successful hashes with `compare_digest`. PAM builds the RPC object with Jansson and sends a dynamically sized UTF-8 JSON document. The Debian package exports the host multiarch PAM directory. Native make builds can override `PAM_MODULE_DIR`.
- Login auto-disable uses Linux-PAM's `authfail` and `authsucc` with Debian's managed local-only Unix password stack. A Unix password success skips failure recording and reaches a required faillock success check. An unsuccessful password goes through failure recording and terminates. It tracks consecutive attempts in the native 900-second window, retains the requested unlock duration within the supported seven-day range, and exempts root. Tally reads and reset use the same private `/run/vyatta-faillock` directory. Mixed or remote auth configurations and unrecognized effective auth stacks are rejected before activation. Removing the feature through the config hook removes the generated profile rules.
- Sandbox activation is explicit for fresh installations. The Debian/DANOS hook checks the sandbox assets and systemd units, native PAM module directory, and required session rule. It rejects other operating systems, command failures, and an updater that leaves the effective PAM stack unchanged. Disable and package removal verify that PAM stopped referencing the module and preserve administrator-modified profile files. The native Debian module path is also used for sandbox packaging.
- The PAM configuration package checks the diversion registry rather than inferring ownership from backup file existence. It restores the original service configuration only when the canonical path is vacant or held by the package's exact link. It preserves canonical administrator replacements and retains the displaced `.debian` files alongside them. Fresh installations preserve administrator audit profiles. Configure and removal release a legacy owned auditd diversion at `/usr/share/pam-configs/auditd`. No maintainer-script edits are made to passwd or umask rules. Stock Debian 13 auditd has no PAM profile. [Debian auditd package file list](https://packages.debian.org/trixie/amd64/auditd/filelist).

## Reproduction evidence

The signed-off candidates were exercised on Debian 13.7 in the isolated QEMU VM. PAM, shadow, runtime and dpkg-diversion mutations were confined to private mount namespaces. All test passwords and hashes were fixtures.

| Baseline check | Result before the fix |
| --- | --- |
| `python3 -S scripts/vyatta-password-rpc verify-password` with configd fixture | Exit 1, `ModuleNotFoundError: No module named 'crypt'`. The separately installed guest copy of Debian's `python3-crypt-r` actually ships both `crypt.py` and `crypt_r.py`. A baseline run with site packages enabled therefore resolved the undeclared compatibility shim. This corrects the review's narrower package-content statement. |
| `python3 tests/password-rpc.py` with guest crypt provider installed | Three invalid-current-password subcases failed through Python exceptions for missing, NUL-containing or invalid Unicode input. |
| C password payload harness against original module | Exit 1, quote/backslash and Unicode/control round trips failed and long input truncated. |
| Login hook plus Debian PAM test application | Exit 1, correct password still accepted after the failure threshold. |
| Sandbox hook | Exit 1 from the harness, the hook reported successful activation with a missing profile. |
| PAM package maintainer scripts plus real `dpkg-divert` registry | Exit 1 from the harness, original configure deleted an administrator's `pam_succeed_if.so` password rule. |

## Verification commands

Run from the corresponding worktree after transferring source with Linux line endings. Privileged integration tests require a disposable Debian host and a private mount namespace.

```sh
# vyatta-login
python3 tests/password-rpc.py
cc -Wall -Wextra -Werror -Itests/include -o /tmp/nudanos-password-payload \
    tests/password-payload.c -ljansson -lpam
/tmp/nudanos-password-payload
prove t/
shellcheck tests/lockout-in-namespace.sh
sudo unshare -m --propagation private bash tests/lockout-in-namespace.sh

# cli-sandbox
make -f debian/rules override_dh_auto_test
shellcheck debian/pam-sandbox.prerm debian/pam-sandbox.postinst tests/user-isolation-in-namespace.sh
sudo unshare -m --propagation private bash tests/user-isolation-in-namespace.sh
make PAM_MODULE_DIR=/usr/lib/"$(dpkg-architecture -qDEB_HOST_MULTIARCH)"/security \
    DESTDIR=/tmp/nudanos-sandbox-stage install

# vyatta-debian-pam-configs-config
shellcheck debian/preinst debian/postinst debian/postrm tests/diversions-in-namespace.sh
sudo unshare -m --propagation private bash tests/diversions-in-namespace.sh
```

## Coverage and limits

- The password RPC test uses the production script and Debian libxcrypt through the documented configd client boundary. The C test uses production serialization and a transport stub. It verifies both RPC methods, Unicode and escape sequences, and a 6,000-byte token without JSON truncation. Native libxcrypt can reject overlength input according to its own limits. End-to-end configd and deployed password policy were outside this VM test.
- The PAM test runs the real Debian profile manager, pam_unix, pam_faillock and faillock tool. It verifies prerequisite rejection, refusal when the administrator's PAM stack stops regeneration, idempotent enable, threshold rejection, success reset, tally/reset operations, root recovery, and deactivation. The test also exercises the PAM account result for a forced-expired fixture separately from the password change RPC.
- The initial sandbox tests used the real shell hook and package prerm with controlled systemd and PAM update command boundaries. The final package and lifecycle checks are listed below.
- The earlier candidate did not change the inherited renewal scripts; the follow-up below describes their subsequent changes and outstanding verification.


## 2026-11-06 review and follow-up

Independent review found upgrade and lifecycle gaps in the earlier candidate. The PAM configuration repository now has local signed-off commit `541f763` (1.13): its private-namespace lifecycle test also covers a regular administrator replacement followed by removal and reinstall. A fresh Debian binary build and ShellCheck completed successfully.

The login candidate remains under verification. It now rejects new passwords that exceed libxcrypt's 511-byte UTF-8 limit or contain NUL, CR, LF, or invalid UTF-8; its hashing pipeline reports command failures. The PAM helper drops supplementary groups and assumes the target real, effective, and saved IDs in its child before opening configd. A privileged transport harness reproduced the incorrect effective identity before that change. The package candidate moves the PAM module into the architecture-specific renewal package and adds a versioned relationship for upgrades. The renewal maintainer scripts avoid forced regeneration and guard administrator profiles; removal stages the Unix profile before regeneration and refuses removal if the old module remains active. A configure hook attempts migration of a recognized legacy tally stack and refuses unknown rules. The namespace test simulates a previous profile-manager cache; a complete real dpkg upgrade from an old appliance and standalone sandbox-root upgrade remain outstanding until explicitly verified.

The final login and renewal package build passed on Debian with the direct `whois` runtime dependency. The signed-off local commit is `7e3b45e` (2.44). A disposable dpkg database unpacked simulated old 2.43 packages followed by 2.44 and recorded both the PAM module and `Faillock.pm` under renewal. This fixture covers file ownership transfer. Full appliance package configuration and apt dependency resolution remain untested. Fresh tests passed for the target effective identity, password RPC, renewal administrator changes and direct service references, and known and unknown legacy stacks. Debian's real `mkpasswd` verified 511-byte ASCII and UTF-8 fixtures and rejected a 512-byte fixture with pipeline failure propagated.

Configd uses a mode 0777 socket and maps a mismatched loginuid to the peer UID, according to the configd owner. An end-to-end deployed AAA policy check for the target user’s password RPC remains outstanding.

## Final Debian sandbox verification

Local signed-off commit `8c71637` builds cli-sandbox 0.32. The standalone PAM module stages its profile. The optional user-isolation feature activates it on first configuration after root preparation and runtime checks. Upgrades preserve activation state. Legacy roots keep a native host module bind when they need it. Fresh roots use the module provided by the renewal package.

- A clean Debian 13.7 amd64 `dpkg-buildpackage -us -uc -b -j2` emitted five binary packages. ShellCheck and all four Python tests passed. The pam-sandbox package contains its executable check helper and the module under `/usr/lib/x86_64-linux-gnu/security`.
- The private-mount-namespace test passed for blocked login/SSH stacks, failed regeneration, rollback, first configure, disabled upgrades, unsupported OS detection, and local administrator module references during removal.
- The final built pam-sandbox 0.32 package passed inactive and active removal against the real Debian `pam-auth-update` in a private mount namespace. The staged PAM profile was removed only after its active rule was gone.
- A complete nspawn login session and a full appliance upgrade remain outside these checks.
