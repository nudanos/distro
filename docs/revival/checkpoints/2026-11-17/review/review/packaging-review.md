# NuDanOS packaging, operating-system integration and portability review

Review date: 2026-11-03. Distribution orchestration reference: `nudanos/distro` commit `864370dc3ff5a6086ce298b147f79ac2c4de315e`. This is a read-only review; no source changes, package installation, privileged scripts, image boot, or GitHub writes were performed by this reviewer.

## Result

The selected `trixie` packaging is substantially newer than the preserved DANOS default branches. All 14 actively selected packaging/OS repositories inspected here use debhelper compatibility 13, Standards-Version 4.7.2, and Rules-Requires-Root: no. Repeating the historical `docs/REVIEW.md` claim that current ports all use compat 9 would be wrong. The two inspected legacy image-builder repositories retain old packaging but are explicitly `reference` in the current manifest.

Debian 13 remains an appropriate first native target. The official release page identifies trixie/13.7 as current stable and specifies full support through 2028-08-09, followed by LTS through 2030-06-30; architecture coverage narrows during LTS. Use amd64 as the release baseline already represented by the image and test machinery, and consume Debian's supported kernel/system libraries instead of resurrecting vendor forks. [Debian release information](https://www.debian.org/releases/trixie/)

Native Arch is a separate delivery target, not supplied by running the existing Debian builder on an Arch host. The inspected trees contain Debian-only installation transactions, PAM composition, package metadata, initramfs hooks, and image management. No PKGBUILD or `.SRCINFO` was found in the locally inspected source set. That search is not a proof about every uninspected org tree, but the active manifest and build system select `.deb` packaging throughout.

Three immediate release gates from this part of the review are password-change compatibility, login lockout compatibility, and image-upgrade authentication. Native package installation also needs an explicit separation from appliance-wide takeover of identity, login, services and boot configuration.

## Scope and evidence quality

`repository-coverage.csv` accounts for all **195** repositories in the current organization inventory, and `coverage-summary.json` records the reconciliation. The manifest contains **197** entries: **192** correspond to org repositories, **5** are external additions, and **3** org repositories are tooling outside the package manifest. Those three are `distro`, `danos-buildimage`, and `danos-buildpackage`. The external five are `golang-github-leanovate-gopter`, `kea`, `libnetconf2`, `libnss-tacplus`, and `libtacplus-map`.

| Treatment of org repositories | Count | Review consequence |
|---|---:|---|
| `danos` source ports | 124 | 96 marked ready; remaining ports explicitly deferred |
| Debian replacements | 32 | Review supported Debian package plus any retained vendor delta; do not rebuild preserved fork by default |
| Selected maintained upstream builds | 10 | Review pinned upstream, selected packaging and patch audit |
| External apt repository | 1 | FRR package set; review keys, version pinning and behavior delta |
| Dropped | 15 | Excluded current product scope, including hardware/proprietary dependencies |
| Reference only | 10 | Historical/documentation/empty/template repositories |
| Tooling outside manifest | 3 | Current orchestration and older builder tools |
| **Total** | **195** | **148 active; 47 archived** |

No archived org repository remains selected as `kind: danos`. Historical `docs/data/inventory.json` describes the old 189-repository DANOS snapshot and its old defaults. It is useful preservation metadata, not evidence that the current trixie branches retain those old versions or build flags.

The CSV records inventory/manifest coverage for every repo, focused source inspection for this review's 16 repositories, and separate-reviewer areas. It does **not** claim exhaustive source/security audit of all 195 trees or the large archived vendor trees. Runtime validation on Linux was unavailable to this reviewer. Parent review has separate build-engine, core, forwarding and CI results; those should be read alongside these notes.

## Ranked findings

### PKG-01 — P1: auto-disable can insert an unavailable required PAM module

Selected source: `vyatta-login@64c15c933f59ae552e8044dbd70a6f2d13e6c99b`.

When both `system login auto-disable attempts` and `duration` are set, `scripts/system/vyatta_update_login.pl:85-95` writes `/usr/share/pam-configs/vyatta-pam-tally`, then invokes `pam-auth-update` at line 197. Its installed template contains `required pam_tally.so` at lines 6 and 8. Debian 13's official `libpam-modules` file list contains `pam_faillock.so` and no `pam_tally.so`; Arch's current PAM package similarly supplies faillock, not tally. Thus this supported configuration writes a required module that cannot be loaded on the modern base. The expected failure is loss of PAM login for affected stacks, not merely a missing lockout statistic. The same repository still runs the removed `pam_tally` command when deleting users, displaying login information and resetting counters.

Evidence: [configuration hook](https://github.com/nudanos/vyatta-login/blob/64c15c933f59ae552e8044dbd70a6f2d13e6c99b/scripts/system/vyatta_update_login.pl#L85), [required module template](https://github.com/nudanos/vyatta-login/blob/64c15c933f59ae552e8044dbd70a6f2d13e6c99b/usr/share/pam-configs/vyatta-pam-tally.template#L6), [user deletion command](https://github.com/nudanos/vyatta-login/blob/64c15c933f59ae552e8044dbd70a6f2d13e6c99b/lib/Vyatta/Login/User.pm#L138), [Debian PAM file list](https://packages.debian.org/trixie/amd64/libpam-modules/filelist), [Arch PAM file list](https://archlinux.org/packages/core/x86_64/pam/files/).

Fix direction: implement and test an auth/account ordering using `pam_faillock`, with matching counter display/reset operations and a migration strategy. PAM lockout cannot be fixed by simply renaming a module: preauth, authfail/authsucc or account operations have ordering semantics. Until then reject the feature before writing PAM configuration. Require serial-console recovery plus SSH/local, local/remote auth-chain, success-reset, expiry and removal tests on both distros. Finding established by source and official package contents; not reproduced as a live lockout here.

### PKG-02 — P1: password RPC imports Python's removed `crypt` module

Selected source: same login SHA.

`scripts/vyatta-password-rpc:11` unconditionally imports `crypt`; line 61 calls `crypt.crypt`. Both verify-password and change-password execute the script, and `pam_vyatta_passwd.c:72-82` routes normal password verification/changes through those RPCs. Python removed the standard-library module in 3.13, and Debian trixie's default Python is 3.13. No compat module or dependency was found in `vyatta-login/debian/control`, the inspected router metapackage, image packages or related scripts. Debian's available `python3-crypt-r` provides a **renamed `crypt_r` module**; installing it alone does not satisfy `import crypt`.

Evidence: [import and call](https://github.com/nudanos/vyatta-login/blob/64c15c933f59ae552e8044dbd70a6f2d13e6c99b/scripts/vyatta-password-rpc#L11), [PAM RPC caller](https://github.com/nudanos/vyatta-login/blob/64c15c933f59ae552e8044dbd70a6f2d13e6c99b/src/pam_vyatta_passwd/pam_vyatta_passwd.c#L72), [runtime dependencies](https://github.com/nudanos/vyatta-login/blob/64c15c933f59ae552e8044dbd70a6f2d13e6c99b/debian/control#L47), [Python removal notice](https://docs.python.org/3/library/crypt.html), [Debian Python package](https://packages.debian.org/trixie/python3), [Debian crypt-r package](https://packages.debian.org/trixie/python3-crypt-r).

Fix direction: use an explicitly packaged supported verification implementation that understands the system's password hashes, e.g. a suitable libcrypt binding, and declare it on both package targets. Test verify/change RPC and an actual unprivileged `passwd` operation, including expired-password renewal and legacy hashes. Ordinary boot and authentication of an existing account need not execute this path, so a passing boot scenario does not disprove this defect. No native Python 3.13/Linux RPC execution was available here.

### PKG-03 — P1: missing image signatures silently bypass upgrade authenticity

Selected source: `vyatta-image-tools@0405dbdeb2818a4f19136730610cb4ba87058c75`.

`fetch_iso_by_url` downloads the ISO, attempts its `.asc` at lines 1890-1895, deletes the signature after any download failure, and returns success at line 1902. `prepare_install_source:1974-1976` verifies **only if the signature exists**, then mounts the ISO. This means a network attacker on supported HTTP/FTP/TFTP delivery, or a compromised mirror, can omit the signature and supply an accepted image. A failed signature prompts to continue, whereas an absent signature has no equivalent gate. The verifier also defaults to legacy `/etc/apt/trusted.gpg`, rather than an explicitly scoped image-release keyring; this does not naturally follow the package repository's modern Signed-By trust configuration.

Evidence: [download fallback](https://github.com/nudanos/vyatta-image-tools/blob/0405dbdeb2818a4f19136730610cb4ba87058c75/scripts/vyatta-install-image.functions#L1890), [conditional verification](https://github.com/nudanos/vyatta-image-tools/blob/0405dbdeb2818a4f19136730610cb4ba87058c75/scripts/vyatta-install-image.functions#L1974), [legacy keyring and override](https://github.com/nudanos/vyatta-image-tools/blob/0405dbdeb2818a4f19136730610cb4ba87058c75/scripts/vyatta-install-image.functions#L1795), [modern Debian trust configuration](https://manpages.debian.org/trixie/apt/apt-secure.8.en.html).

Fix direction: publish a release signature or signed manifest over the ISO hash, require verification before mounting, and ship a dedicated pinned NuDanOS release keyring with a rotation/revocation policy. Keep any local development bypass explicit, off by default, and visible in audit output. Do not mistake signed apt metadata used while building an image for a signature authenticating a downloaded finished ISO. Test absent, truncated, wrong-key, expired/revoked, valid and locally bypassed cases.

### PKG-04 — P2: PAM package removal does not restore the diverted audit profile and edits unrelated rules

Selected source: `vyatta-debian-pam-configs-config@e8502d8d0e8b59bc95cbb77a79258886d20fea52`.

`debian/preinst:35-39` diverts `/usr/share/pam-configs/auditd` to `/usr/share/pam-configs/auditd.debian`, but its guard checks `/etc/pam.d/auditd.debian`. `debian/postrm:41-45` repeats that wrong guard, so normal removal skips undoing the diversion. In that same remove/purge/failed-upgrade/abort block, line 48 deletes **all lines beginning `password`** from Debian/admin-owned `/etc/pam.d/passwd`, even though the package's configure operation only removes `pam_succeed_if.so` lines. This can erase an administrator's password rules. Stock Debian may only contain an `@include common-password` line, so do not claim that every stock removal immediately breaks `passwd`; the destructive edit and stale diversion are the concrete defects.

Evidence: [preinst diversion](https://github.com/nudanos/vyatta-debian-pam-configs-config/blob/e8502d8d0e8b59bc95cbb77a79258886d20fea52/debian/preinst#L35), [postrm cleanup](https://github.com/nudanos/vyatta-debian-pam-configs-config/blob/e8502d8d0e8b59bc95cbb77a79258886d20fea52/debian/postrm#L41), [configure edit](https://github.com/nudanos/vyatta-debian-pam-configs-config/blob/e8502d8d0e8b59bc95cbb77a79258886d20fea52/debian/postinst#L32).

Fix direction: test diversion ownership/registered destination, restore the exact managed resource, and avoid deleting unowned configuration. Add install → reinstall → upgrade → remove → purge → reinstall tests that compare diversion state and PAM content, including administrator edits and interrupted transactions. These package-lifecycle operations are required before advertising native installation on existing hosts.

### PKG-05 — P2: removing SysV links is not a durable systemd service policy

`vyatta-debian-system-config@13cb55fae181a4ad39366185bcc6f555ff2d32e2/debian/vyatta-debian-system-config.postinst:22-31` removes SysV links for numerous services supposedly controlled by Vyatta configuration, and explicitly disables the native unit only for dnsmasq. `vyatta-debian-ssh-server-config@8c337c88d03c2f538f7de3cc74ed8ec6f5cf7822/debian/postinst:22-23` only removes ssh's SysV links. The systemd configuration package installs a journald drop-in; it does not supply missing native service lifecycle policy. Native unit enablement, socket activation, already-running daemons and subsequent upstream package upgrades need explicit reconciliation.

Debian's own update-rc.d manual warns that deleting links makes the next package upgrade recreate factory-default links. The code therefore cannot reliably implement its stated policy, even without assuming exactly which daemon is exposed on a particular image. [system configuration script](https://github.com/nudanos/vyatta-debian-system-config/blob/13cb55fae181a4ad39366185bcc6f555ff2d32e2/debian/vyatta-debian-system-config.postinst#L22), [SSH script](https://github.com/nudanos/vyatta-debian-ssh-server-config/blob/8c337c88d03c2f538f7de3cc74ed8ec6f5cf7822/debian/postinst#L22), [Debian helper manual](https://manpages.debian.org/trixie/init-system-helpers/update-rc.d.8.en.html).

Fix direction: define the systemd units/sockets that NuDanOS owns, the conditions under which each starts, and the state preserved/restored on package lifecycle operations. Use package-manager-aware service helpers for installation and normal systemd calls for runtime changes. Verify clean boot without a configured service, enabling/disabling by configuration, upstream daemon package upgrade, restart and removal. Keep image-build policy from silently becoming policy for a normal host installation.

### PKG-06 — P2: the password PAM bridge builds JSON without escaping

`vyatta-login`'s `src/pam_vyatta_passwd/pam_vyatta_passwd.c:74-80` interpolates raw current/new passwords into a JSON string using `snprintf`. A quote creates malformed JSON and a backslash can change the represented string or create an invalid escape. Long combined passwords also silently truncate the fixed 4096-byte buffer. This is a concrete password usability/correctness issue independently of the Python import defect; no privilege-escalation claim is made here.

Evidence: [raw JSON construction](https://github.com/nudanos/vyatta-login/blob/64c15c933f59ae552e8044dbd70a6f2d13e6c99b/src/pam_vyatta_passwd/pam_vyatta_passwd.c#L74).

Fix direction: use the project's JSON serializer/library and handle length/allocation errors. Test quotes, backslashes, non-ASCII, empty values and boundary-length passwords through the actual PAM/RPC round trip.

### PKG-07 — P2 portability requirement: PAM module path is fixed to Debian amd64

`vyatta-login/src/Makefile:27-28` installs its module only into `/usr/lib/x86_64-linux-gnu/security` although `debian/control` declares Architecture: any. Arch's official PAM package loads modules from `/usr/lib/security`. A native Arch build that reused the current install rule would place the password module outside that layout. The path also prevents genuine Debian non-amd64 packaging without overrides.

Evidence: [install rule](https://github.com/nudanos/vyatta-login/blob/64c15c933f59ae552e8044dbd70a6f2d13e6c99b/src/Makefile#L27), [Arch module files](https://archlinux.org/packages/core/x86_64/pam/files/).

Fix direction: expose an install-time PAM module directory and derive it from the target toolchain/distribution packaging. Keep `DESTDIR` separate from the runtime prefix. This is one example of why native Arch packaging needs tested install manifests, not just converted Debian package names.

### PKG-08 — P2 latent legacy compatibility: shipped SSSD wrapper requests a vendor-only option

`vyatta-sssd@1b21e8402077d21de96b9bfbbd875ced0cfdeb0f/Vyatta/SSSD.py:112-114` always initializes its SSSD service, and `_setup_sssd:217` calls `set_option("switch_user", "true")`. This option is absent from Debian SSSD 2.10.1's source schema; `SSSDConfig.set_option` explicitly rejects unknown schema options with `NoOptionError`. The package builds without executing this configuration path. The old wrapper also retains local/TACACS+ provider assumptions.

Evidence: [wrapper initialization](https://github.com/nudanos/vyatta-sssd/blob/1b21e8402077d21de96b9bfbbd875ced0cfdeb0f/Vyatta/SSSD.py#L105), [unsupported option](https://github.com/nudanos/vyatta-sssd/blob/1b21e8402077d21de96b9bfbbd875ced0cfdeb0f/Vyatta/SSSD.py#L216), [official Debian source tarball](https://deb.debian.org/debian/pool/main/s/sssd/sssd_2.10.1.orig.tar.gz), [Debian SSSD package](https://packages.debian.org/trixie/python3-sss).

Verification performed: downloaded the official upstream tarball used by Debian (SHA-256 `ea6a690047cea1ecd50016aa30946f9348da37b46daa984f34bc72ddb767539f`), inspected the schema and rejection code, and reproduced `SSSDConfig.NoOptionError: Section [sssd] has no option [switch_user]` in a pure schema-API test. On Windows only the import of `fcntl` was stubbed to permit this read-only API test; no locking, configuration writes, Linux service start or full wrapper execution was exercised. Treat this as a **legacy shipped API defect**, not proof that current TACACS+ login fails: NuDanOS has already migrated the main TACACS+ path to pam_tacplus/libnss-tacplus/tacplusd. Trace remaining consumers before retaining, adapting or deleting the old wrapper.

## Native Debian + Arch architecture proposal

Preserve a common management/router implementation and make distribution policy explicit at the boundary. Avoid creating a second fork of the configuration engine for Arch. The initial native package target should install management/CLI, YANG/VCI, kernel forwarding, FRR integration and explicitly enabled services into a normal root filesystem. The existing Debian appliance image remains a distinct product profile. Arch ISO, multi-image boot and Secure Boot are additional deliverables after normal installation is reliable; if full appliance parity is required at first release, they are additional release gates rather than implicit existing support.

| Boundary | Common implementation | Debian adapter | Arch adapter |
|---|---|---|---|
| Build/install | Versioned source inputs; staged `DESTDIR`; runtime/config/state dirs; schema and service assets | Native `.deb` split packages, debhelper/dh-yang/dh-vci, shlibs/symbols and Debian dependency names | Native PKGBUILD split packages, build/check/package stages, explicit depends/makedepends/checkdepends, checksums/signatures |
| YANG and VCI | Compiler/validator and generated dependency metadata available as ordinary commands | Keep dh-yang/dh-vci wrapper integrations | Call same compiler/installer; package-owned hooks update aggregate schema/service metadata once per transaction |
| Authentication | Common auth-chain/lockout semantics, password RPC and hash handling | PAM common-* and pam-auth-update adapter; properly owned diversions only where unavoidable | Arch PAM include stack (`system-auth` etc.) and module locations; no dependency on Debian PAM profile generator |
| Users/files | Named service users/groups, tmpfiles/sysusers descriptions, restrictive state permissions | Debian maintainer helpers and tested reversible transitions | sysusers/tmpfiles integration and native package hooks; preserve administrator config via `backup=()` |
| Services | systemd units and runtime lifecycle contracts, explicit capabilities/sockets | deb-systemd-helper/deb-systemd-invoke and Debian daemon names | Native systemd lifecycle and Arch daemon names; no update-rc.d dependency |
| Network ownership | Configd manages only opted-in interfaces/services and router policy | Explicit handoff from Debian's active network manager | Explicit handoff from Arch's active network manager; do not assume one particular network manager is installed |
| Package upgrades | Config schema/state migration, ABI/version compatibility and rollback rules | apt repository with Signed-By, supported package upgrades | signed pacman repository and supported full-system upgrade compatibility window |
| Boot/image | Image version/health/rollback API, only if appliance profile selected | Existing live-build/live-boot/initramfs-tools/GRUB implementation | Separate Arch boot/initramfs adapter (e.g. mkinitcpio hooks), Arch package database and bootloader integration |

The package format/transaction distinction is documented in Arch's [PKGBUILD manual](https://man.archlinux.org/man/PKGBUILD.5.en) and [ALPM hooks manual](https://man.archlinux.org/man/alpm-hooks.5.en). Arch's [mkinitcpio manual](https://man.archlinux.org/man/mkinitcpio.8.en) describes its hook/preset/UKI machinery. Debian initramfs-tools hooks cannot simply be copied into that pipeline. Arch's [PAM manual](https://man.archlinux.org/man/pam.8.en) also describes the machine/vendor configuration precedence that an adapter must preserve.

Specific code that must move behind those boundaries:

- `base-files/debian/{preinst,postinst,postrm}` diverts `/etc/os-release` and issues, switches dpkg origins, and uses dpkg-statoverride. `base-files-vyatta` is Essential: yes. These are Debian appliance identity operations, not prerequisites for installing the common runtime on an existing Debian or Arch host.
- `nudanos-router/debian/control` pulls the appliance identity, login/PAM/password defaults, initramfs and image tools into one metapackage. Split minimal runtime, router functionality, distribution integration, optional services and appliance-image profiles with explicit package lists.
- `vyatta-debian-system-config` edits `/etc/sudoers`, `/etc/adduser.conf`, `/etc/login.defs`, ctrl-alt-del state and removes another package's PPP hook. Package lifecycle must preserve ownership/admin changes; native Arch should not emulate dpkg diversions or wholesale replace these files.
- `vyatta-login` generates Debian PAM profiles and executes pam-auth-update. This is security policy requiring two tested adapters, not a package-name mapping exercise.
- `vyatta-image-tools/lib/Vyatta/Live.pm` reads `/var/lib/dpkg` for image versions, while `vyatta-squashfs-chroot` performs apt/dpkg operations. Image/rootfs management cannot support native Arch until these operations are abstracted or replaced.
- `live-boot-vyatta` sources `/usr/share/initramfs-tools/hook-functions` and overrides live-boot internals; it is not generic initramfs code. Maintain it as a Debian adapter with upstream compatibility tests.
- `vyatta-hotplug` already installs its udev rule under `/usr/lib/udev/rules.d`, and `vyatta-debian-systemd-config` uses `/usr/lib/systemd/journald.conf.d`; retain these merged-/usr locations in new target packages.
- `vyatta-platform` still includes vendor hardware/SFP facilities; keep those optional for an initial software-router native installation and test missing-hardware behavior. Dropped Broadcom/UfiSpace/Accton stacks are not automatically revived by adding Arch packaging.

## Acceptance gates for native support

Run these against **both** a clean Debian 13 amd64 VM and a clean, date/version-recorded Arch x86_64 VM; retain separate results for appliance images. Do not claim Arch runtime support based on an Arch build-host smoke test.

1. Build all selected runtime packages in each distro's clean build environment, install into a clean rootfs, verify dynamic library/PAM/module/schema paths and package ownership, and retain exact source/package versions.
2. Install minimal/router/service profiles on an existing OS; boot; start/stop/restart; verify configd/VCI/D-Bus authorization; commit/save/reboot; prove persistent configuration and kernel forwarding/FRR routing.
3. Exercise local users, SSH keys, current/wrong passwords, quoted/backslash/unicode password changes, expiration/renewal, local/remote auth chains, lockout/unlock and recovery. Test the Linux PAM path, not only config tree transformation.
4. Run same-version reinstall, upgrade with changed dependency versions, remove, purge where applicable, failed/aborted transaction, reinstall and reboot. Compare auth files, diversions, service enablement, users, ownership and administrator changes before/after.
5. Upgrade supported host packages (Python, PAM, systemd, OpenSSL, FRR/libyang, Perl/C ABI dependencies), then rerun the router/auth suite. Record an Arch snapshot compatibility window and rebuild native packages when ABI changes require it.
6. For appliance profile parity, validate BIOS/UEFI boot, install to disk, multi-image upgrade/rollback, persistence, kernel update/initramfs regeneration, signature rejection cases and secure boot policy separately on both targets.

## Inspected repository revisions

All selected active repositories below were inspected on `trixie`; the two reference repositories use their preserved default branches.

| Repository | Commit |
|---|---|
| base-files | `2274cbfbcbb96bedfcaa4878cd59e4901de99178` |
| vyatta-base | `c850111cadd5cf150a79543aa4838d23afa28bc1` |
| vyatta-login | `64c15c933f59ae552e8044dbd70a6f2d13e6c99b` |
| vyatta-debian-system-config | `13cb55fae181a4ad39366185bcc6f555ff2d32e2` |
| vyatta-debian-systemd-config | `9cf2be239e99adcd65f61c1a3874f600f17b3d96` |
| vyatta-debian-ssh-server-config | `8c337c88d03c2f538f7de3cc74ed8ec6f5cf7822` |
| vyatta-debian-passwd-config | `b1979acf7e155386b15817ff34f44cd905fade5c` |
| vyatta-debian-pam-configs-config | `e8502d8d0e8b59bc95cbb77a79258886d20fea52` |
| vyatta-debian-lldpd-config | `405908fd3384cf8f8b20ea61bb41bf40aaf0dd1a` |
| vyatta-image-tools | `0405dbdeb2818a4f19136730610cb4ba87058c75` |
| live-boot-vyatta | `2551a43376ff660867a3fcb6a4153c7b6e48a933` |
| vyatta-platform | `51e7b65044084e9d01223c471b4e02fe352c7130` |
| vyatta-hotplug | `b50efce4714adff73ef49ea0f08534a4a11f8e13` |
| vyatta-sssd | `1b21e8402077d21de96b9bfbbd875ced0cfdeb0f` |
| build-iso (reference; master) | `7f3544386650a877bf90801d4c6c2f365934d100` |
| live-build-desc (reference; master) | `619a40797c6ba0fdeda8a819921dd8092c93e1a2` |

Legacy `build-iso/auto/config` still carries OBS configuration discovery, old live-build flags, floating stable and insecure historical apt flags; `live-build-desc/livebuild_pre_run` edits installed debootstrap/live-build scripts. These are reasons to keep the historical repositories as references and use `distro/image`; they are not asserted to be defects in the current active build path.
