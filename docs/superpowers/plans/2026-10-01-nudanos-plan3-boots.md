# NuDanOS Plan 3: Boots

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The nightly builds a NuDanOS ISO from source, proves it installs (layer 2) and boots, configures, saves, installs to disk and boots again in QEMU (layer 3), and only then publishes it.

**Architecture:** First make the milestone-1 package set installable without DPDK. Then turn it into a system:
- Kernel NICs keep DANOS dataplane names (`dp0sN`), so `interfaces dataplane` works over the kernel.
- A `nudanos-router` meta-package names the package set.
- ISO hooks move into maintainer scripts.
- `distro-build` gains `image` (live-build in a privileged container), `test install` (layer 2) and `test boot` (a Go serial-console driver for QEMU).
- The nightly chains them and releases the ISO.

**Tech Stack:** Go 1.26 (`github.com/nudanos/distro`), Docker, Debian 13 live-build, QEMU (`qemu-system-x86`, KVM on GitHub runners, TCG elsewhere), bash, Perl/Python in the port repos, GitHub Actions.

**Spec:** `docs/superpowers/specs/2026-10-01-plan3-boots-design.md` (all four sections approved), an addendum to `docs/superpowers/specs/2026-09-28-debian13-revival-design.md`. Plan 2's ledger, `docs/superpowers/ledgers/2026-09-29-plan2-ledger.md`, carries the "Plan 3 note" lines this plan resolves.

**Scope:**
- **In:** spec addendum §1–§4 and base spec §4.5 (meta-package, hook moves), §7.1 `image`, §8.2 and §8.3.
- **Plan 4:** multi-VM scenarios (§8.4), 2105 fixtures (§8.5), the kernel vendor-commit audit, and the 8 deferred tooling minors from plan 2.

## Global Constraints

- GitHub org `nudanos`; ports live on `trixie` branches; original branches are never rewritten.
- Debian packaging on `trixie`:
  - `debhelper-compat (= 13)`, `Standards-Version: 4.7.2`, `Rules-Requires-Root: no`
  - `Maintainer: NuDanOS Maintainers <jon@fernandez.tech>`
  - `Vcs-Git`/`Vcs-Browser` pointing at `github.com/nudanos/<repo>`
- Changelog entries are signed `NuDanOS Maintainers <jon@fernandez.tech>`. Port versions get `+nudanos1` (bumped to `+nudanos2`… on later changes); upstream-kind builds use `0nudanos1`, or `<rev>+nudanos1` when Debian already packages that upstream version.
- Package builds run their test suites; a failing test fails the build. `lintian --fail-on error --profile vyatta` gates every build.
- Physical NICs are named `dp0<rest>` from their predictable PCI path name (`enp0s3` → `dp0s3`, `enp2s0f1` → `dp0p2s0f1`). Names that cannot be mapped keep their kernel name and appear under `interfaces system`.
- Live ISO login is `vyatta`/`vyatta`; `install image` refuses the password `vyatta`.
- Milestone 1 has no firewall, NAT or QoS (base spec §4.4) and is not for internet-facing use.
- The image config in `distro/image/` is ported from DANOS `build-iso` (GPL-2.0-only). Every ported file keeps a `SPDX-License-Identifier: GPL-2.0-only` line and the original copyright.
- Kernel: Debian `linux-image-amd64` from `trixie-backports` (base spec §4.3).
- The work directory is case-sensitive (`/Volumes/nudanos/work` on the Mac).
- The agent never types or handles secrets. Creating repos, archiving, and anything else outward-facing needs the user's go-ahead. Creating `nudanos/nudanos-router` was approved in brainstorming (Task 7). Pushing `trixie` branches to existing `nudanos/*` repos and committing to `main` of `nudanos/distro` are approved.
- Commits use `git commit -s` and end with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- Never republish or copy `jsouthworth/danos-bootstrap` (unlicensed).
- Wave procedure (`docs/porting.md`): manifest first, then branches. Run multi-repo loops under `bash`, because zsh does not word-split `$LIST`.

## Review Focus

1. **NIC names off the happy path:**
   - onboard NICs (`enp0s31f6`), multifunction (`enp2s0f1`) and high bus numbers (`enp175s0f1np1`);
   - USB/MAC names (`enx…`) and PCI-domain names (`enP1p2s0`);
   - a mapped name longer than 15 characters (`IFNAMSIZ`) must be left alone, not truncated.

   Test: `dp-name` cases in Task 3.
2. **Maintainer scripts run twice:** postinst rerun and upgrade must not duplicate the loopback stanza or stack diversions; purge must undo the `/etc/os-release` diversion. Test: the double-run checks in Task 4, plus layer 2's purge diff in Task 8.
3. **A DPDK-only setting on a kernel-forwarding box** (`cpu-affinity`) must fail the commit with a message that names the cause, not an obscure script error. Test: boot-test step `refuse-affinity` in Task 10.
4. **UEFI boot.** The layer 3 run boots BIOS (SeaBIOS); the ISO also claims UEFI (GRUB EFI). Test: the `-firmware efi` smoke boot to the login prompt in Task 10, run by the nightly in Task 11.
5. **A slow or noisy console:**
   - kernel messages interleaved with prompts;
   - TCG being 5–20× slower than KVM;
   - a step that never matches must fail at its timeout, showing the last console lines, and never hang the job.

   Test: `boottest` unit tests in Task 10, and the timeout scaling in `cmd/boottest`.

---

## File Structure

| File | Responsibility |
|---|---|
| `tests/integration/installable.sh` | Simulate installing every milestone-1 binary with kernel forwarding; report each failure's first unmet dependency |
| `docs/kernel-forwarding.md` | Inventory: what each dataplane-model configd action does under kernel forwarding; packages excluded as DPDK-only, with reasons |
| `internal/engine/engine.go` | + `Privileged`, `Devices` on `RunSpec` |
| `internal/installtest/installtest.go` | Layer 2 container spec |
| `tests/integration/install-purge.sh`, `tests/integration/install-allowlist.txt` | Layer 2 script and its leftover-file allowlist |
| `internal/imagebuild/imagebuild.go`, `builder/make-image.sh` | `image` container spec and the script that runs live-build |
| `image/` | live-build config ported from DANOS `build-iso` |
| `internal/boottest/console.go` | Serial-console expect library (`Expect`, `Send`, `Dialog`, `Tail`) |
| `cmd/boottest/main.go`, `cmd/boottest/steps.go` | QEMU runner and the layer 3 script |
| `tester/Dockerfile` | Image with QEMU and OVMF for `test boot` |
| `cmd/distro-build/main.go` | + `image`, `test install`, `test boot` |
| `tools/router_deps.py`, `tools/test_router_deps.py` | Generate and check `nudanos-router`'s dependency list |
| `.github/workflows/nightly.yml` | + image, layer 2, layer 3, release |
| Port repos | `vyatta-interfaces`, `vyatta-debian-system-config` (re-port from `master`); `vyatta-kernel-forwarding`; `vyatta-cfg-dataplane`; `base-files`; `vyatta-cfg-system`; `cli-sandbox`; `vyatta-service-ntp`; `vyatta-service-bridge`; `vyatta-vrrp`; `vyatta-image-tools`; new `nudanos-router` |

---

### Task 1: Re-port `vyatta-interfaces` and `vyatta-debian-system-config` from `master`

Plan 2 branched every port from its repo's default branch. For 98 repos that is DANOS `master` (late 2021). For these two it is a 2020 release branch: `vyatta-interfaces` `1.84.x` (1.84.4; `master` is 2.8) and `vyatta-debian-system-config` `1.16.x`. The newer packages built from `master` expect the newer `vyatta-interfaces`. For example, dh-yang turns their YANG imports into a dependency on `vyatta-interfaces-v1-yang`, which only `vyatta-interfaces` 2.x produces, so the dataplane interface model cannot install today.

**Files:**
- Modify (port repos): `/Volumes/nudanos/port-vyatta-interfaces`, `/Volumes/nudanos/port-vyatta-debian-system-config` (new `trixie` history).

**Interfaces:**
- Consumes: `tools/port.py` (plan 2); the plan 2 commits on each repo's current `trixie` branch.
- Produces: `trixie` branches based on `origin/master` with every plan 2 change carried over; `vyatta-interfaces-v1-yang` exists in the pool.

- [ ] **Step 1: Record the current state**

```bash
cd /Volumes/nudanos
for r in vyatta-interfaces vyatta-debian-system-config; do
  git -C port-$r fetch -q origin
  echo "== $r"; git -C port-$r log --oneline "$(git -C port-$r merge-base origin/trixie origin/HEAD)"..origin/trixie | cat
done
```
Expected: the port commits for each repo (for `vyatta-interfaces`: the port, `drop dh_install --fail-missing`, the CI caller, `depend on vyatta-forwarding`; for `vyatta-debian-system-config`: the port, the transform fix, the CI caller, `depend on ntpsec`). Save this list in the ledger; Step 3 reapplies each of these commits.

- [ ] **Step 2: Branch from `master` and re-run the mechanical port**

```bash
cd /Volumes/nudanos
for r in vyatta-interfaces vyatta-debian-system-config; do
  git -C port-$r branch -f trixie-1.x origin/trixie   # keep the old port for reference
  git -C port-$r checkout -q -B trixie origin/master
  python3 distro/tools/port.py port-$r
  git -C port-$r add -A && git -C port-$r commit -qs -m "Port to Debian 13 (trixie)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
done
```
Expected: one commit per repo. port.py prints NOTE lines; resolve each before Step 3.

- [ ] **Step 3: Carry the hand-made plan 2 changes**

For each non-mechanical commit listed in Step 1 (everything except "Port to Debian 13" and the CI caller), run `git cherry-pick -x <sha>` on the new `trixie`. When a pick conflicts because `master` changed the same lines, re-make the change by hand against the `master` file, keeping the intent stated in the original commit message.

The changes to carry:
- **`vyatta-interfaces`:** `debian/rules` without `dh_install --fail-missing`; `Depends: vyatta-forwarding` instead of `vyatta-dataplane`; and `breakout-0 | vyatta-kernel-forwarding` in the interfaces package.
- **`vyatta-debian-system-config`:** the `.transform` `${Dollar}` escapes; `ntpsec` instead of `ntp` in `debian/control`; and `ntpsec` in the postinst init list.

Then add the CI caller with `bash wave-push.sh` later in Step 5.

- [ ] **Step 4: Build both against the full pool**

```bash
cd /Volumes/nudanos/distro && export PATH="$HOME/.local/go/bin:$HOME/.local/bin:$PATH"
go build -o distro-build ./cmd/distro-build
./distro-build -work /Volumes/nudanos/work -jobs 4 \
  -local vyatta-interfaces=../port-vyatta-interfaces \
  -local vyatta-debian-system-config=../port-vyatta-debian-system-config \
  build vyatta-interfaces vyatta-debian-system-config 2>&1 | tail -5
ls /Volumes/nudanos/work/out/vyatta-interfaces/ | grep -c '^vyatta-interfaces-v1-yang_'
```
Expected: both `built`; `1`. On a build failure, fix it on the port branch the way plan 2 did (dh_missing → `debian/not-installed`; merged-usr paths → `usr/lib`; and so on). Ledger each non-mechanical fix as a `Ruling:`.

- [ ] **Step 5: Rebuild their dependents, then push**

```bash
cd /Volumes/nudanos/distro
./distro-build -work /Volumes/nudanos/work -jobs 4 \
  -local vyatta-interfaces=../port-vyatta-interfaces \
  -local vyatta-debian-system-config=../port-vyatta-debian-system-config build 2>&1 | tail -4
cd /Volumes/nudanos && printf 'vyatta-interfaces vyatta-debian-system-config\n' > wave-p3-report.txt
bash wave-push.sh wave-p3-report.txt
```
Expected: `build exit=0` (dependents rebuild because their dependency keys changed); both branches pushed. Their CI goes green (check with `bash ci-wait.sh wave-p3-report.txt`). The default branch on GitHub stays as it is; only `trixie` moves. The ledger records the replaced history (`trixie-1.x`, kept locally).

---

### Task 2: Installability audit and untangling (the inventory)

Spec addendum §2: "The plan's first task is an inventory." Run it after Task 1, because the stale `vyatta-interfaces` caused part of the breakage.

Baseline, measured while writing this plan against the repo built at the end of plan 2: **402 of 637 binaries install with `vyatta-kernel-forwarding`; 235 do not.** Many of the 235 fail only because they depend on a few root causes (the stale `vyatta-interfaces`, and `vyatta-interfaces-base`'s `vplane-config`/DHCP dependencies).

Findings so far, from simulating installs against the current repo:
- `vyatta-interfaces-dataplane-v1-yang` needs `vplane-config` and `vyatta-dataplane-op-ifconfig-1`.
- `vyatta-interfaces-base` needs `vplane-config` and `vyatta-service-dhcp-client`; DHCP is milestone 1.1, and 1.0 interfaces are statically addressed.
- `vyatta-interfaces-switch-v1-yang` needs `vyatta-policy-qos-vci` (QoS is DPDK).

Actions in the dataplane interface model that would touch DPDK:
- `vplane-affinity` on `cpu-affinity`, `receive-cpu-affinity` and `transmit-cpu-affinity`.
- Breakout is a YANG feature enabled only on switch platforms, so it is absent on x86.

Everything else the model runs (`vyatta-interfaces.pl --create-dev/--delete-dev`, `vyatta-address`, `ip link`, `sysctl`) acts on the kernel netdev by name. For `dp0*` (dpid 0) the create/delete paths are kernel-only.

**Files:**
- Create: `tests/integration/installable.sh`, `docs/kernel-forwarding.md`
- Modify (port repos, as the audit finds): `debian/control` of the packages named below

**Interfaces:**
- Consumes: the signed repo (`distro-build repo`).
- Produces:
  - `installable.sh` prints one line per binary package: `OK <pkg>`, or `NO <pkg> <first unmet dependency line>`;
  - `docs/kernel-forwarding.md` holds the inventory table and the list of DPDK-only binaries with reasons (Task 7's meta-package excludes exactly these).

- [ ] **Step 1: Write the audit script**

`tests/integration/installable.sh`:
```bash
#!/bin/bash
# Simulate installing each binary package of the repo together with
# vyatta-kernel-forwarding, in a clean Debian 13 container, and report the
# first unmet dependency of each one that cannot be installed.
# Usage: WORK=/Volumes/nudanos/work tests/integration/installable.sh [pkg…]
set -euo pipefail
: "${WORK:?set WORK to the distro-build work directory}"
ENGINE=${ENGINE:-docker}
$ENGINE run --rm -v "$WORK/repo":/repo:ro -e "PKGS=$*" debian:trixie bash -euc '
  apt-get update -qq >/dev/null && apt-get install -y -qq ca-certificates gpg >/dev/null
  gpg --dearmor < /repo/nudanos-archive-keyring.asc > /usr/share/keyrings/nudanos.gpg
  echo "deb [signed-by=/usr/share/keyrings/nudanos.gpg] file:/repo trixie main" > /etc/apt/sources.list.d/nudanos.list
  apt-get update -qq >/dev/null
  pkgs=${PKGS:-$(awk "/^Package: /{print \$2}" /repo/dists/trixie/main/binary-amd64/Packages | sort -u)}
  for p in $pkgs; do
    if apt-get install -s --no-install-recommends vyatta-kernel-forwarding "$p" >/tmp/o 2>&1; then
      echo "OK $p"
    else
      echo "NO $p $(grep -m1 -E "Depends:|Conflicts:|Breaks:" /tmp/o | tr -s " " | cut -c1-200)"
    fi
  done'
```

- [ ] **Step 2: Run it and keep the baseline**

```bash
cd /Volumes/nudanos/distro && chmod +x tests/integration/installable.sh
./distro-build -work /Volumes/nudanos/work -key "$(cat keys/FINGERPRINT)" repo >/dev/null
WORK=/Volumes/nudanos/work tests/integration/installable.sh > /Volumes/nudanos/installable-before.txt
grep -c '^OK' /Volumes/nudanos/installable-before.txt; grep -c '^NO' /Volumes/nudanos/installable-before.txt
```
Expected: two counts. Record them in the ledger (this run takes 15–20 minutes for ~500 binaries).

- [ ] **Step 3: Classify each `NO` line, by these rules, in order**

1. **The unmet dependency is a DPDK protocol virtual** (`vyatta-dataplane-cfg-*`, `vyatta-dataplane-op-*`) **or `vplane-config` / `vplane-config-*`.**
   - **Base system or interface model**, i.e. something the dataplane interface model, VRRP, FRR, bridge or bonding needs at runtime: change the dependency to `<dep> | vyatta-kernel-forwarding`. This is plan 2's ruling, extended.
   - **A feature that exists only in the DPDK dataplane** (storm control, port monitor, sFlow, affinity, power profile, `vyatta-system-dataplane-v1-yang`): it stays strict and is listed as DPDK-only.
2. **The unmet dependency is a milestone 1.1/2/later package** (`vyatta-service-dhcp-client`, `vyatta-policy-qos-vci`, …).
   - **The dependent works without it:** move it to `Recommends`. Example: `vyatta-interfaces-base` → `vyatta-service-dhcp-client`, because 1.0 interfaces are static.
   - **Otherwise:** list the dependent as excluded until that milestone.
3. **Nothing provides the dependency:** find the DANOS source that should, with this command, and fix the producer (a missing `Provides:`, or a stale port):
   ```bash
   M="$HOME/Documents/Claude/Projects/danOS Project/mirrors"
   for d in "$M"/*.git; do git -C "$d" show HEAD:debian/control 2>/dev/null | grep -qE "^Package: <dep>$|Provides:.*<dep>" && basename "$d" .git; done
   ```
4. **A Debian package is missing in trixie** (like `iproute` in plan 2): use its trixie successor.

Apply each change on the port's `trixie` branch, one commit per repo, explaining the reason in the commit message. Write each decision into `docs/kernel-forwarding.md` (template below).

- [ ] **Step 4: Write `docs/kernel-forwarding.md`**

```markdown
# Kernel forwarding (milestone 1)

NuDanOS 1.0 forwards in the Linux kernel. `vyatta-kernel-forwarding` provides
`vyatta-forwarding` and renames NICs to DANOS dataplane names, so the
`interfaces dataplane` model drives kernel netdevs.

## Dataplane interface model: configd actions

| Action (vyatta-interfaces-dataplane-v1.yang) | Under kernel forwarding |
|---|---|
| `vyatta-interfaces.pl --create-dev/--delete-dev` | works: VRF bind, link down, stats files (dpid 0 paths) |
| `vyatta-address add/delete`, `ip li set … alias`, `sysctl …/forwarding` | works: kernel netdev |
| `vyatta-interfaces.pl --set-dev-mtu/--set-mac` | works: kernel netdev |
| `vplane-affinity` (cpu-affinity, receive-, transmit-cpu-affinity) | refused at commit (deviation in vyatta-kernel-forwarding) |
| `breakout`, `breakout-reserved-for` | absent: YANG feature enabled only on switch platforms |
| `vyatta-intf-end`, `vyatta-update-vifs` | works |

## DPDK-only binaries (left out of nudanos-router)

| Package | Why |
|---|---|
| vyatta-security-storm-control-v1-yang | needs vyatta-dataplane-cfg-storm-ctl-3 |
| vyatta-system-dataplane-v1-yang | power profile / cpumask of the DPDK process |
| vplane-config | the DPDK dataplane's config backend |
| … | (one row per DPDK-only package from Step 3) |

## Deferred to a later milestone

| Package | Milestone | Why |
|---|---|---|
| … | 1.1 | (one row per rule-2 exclusion from Step 3) |
```
Fill the last two tables from Step 3; every row needs a reason.

- [ ] **Step 5: Rebuild, re-run, iterate to a fixed point**

```bash
cd /Volumes/nudanos/distro
./distro-build -work /Volumes/nudanos/work -jobs 4 $(for r in <changed repos>; do printf -- '-local %s=../port-%s ' $r $r; done) build 2>&1 | tail -3
./distro-build -work /Volumes/nudanos/work -key "$(cat keys/FINGERPRINT)" repo >/dev/null
WORK=/Volumes/nudanos/work tests/integration/installable.sh > /Volumes/nudanos/installable-after.txt
grep '^NO' /Volumes/nudanos/installable-after.txt | awk '{print $2}' | sort > /tmp/no.txt
sed -n '/^## DPDK-only/,/^## Deferred/p;/^## Deferred/,$p' docs/kernel-forwarding.md | awk -F'|' 'NF>2{gsub(/ /,"",$2); print $2}' | grep -v '^Package$\|^-' | sort > /tmp/excluded.txt
comm -23 /tmp/no.txt /tmp/excluded.txt
```
Expected: the last command prints nothing. Every package that still cannot be installed is one the document excludes, with a reason. Repeat Steps 3–5 until it does.

- [ ] **Step 6: Commit and push**

```bash
cd /Volumes/nudanos/distro
git add tests/integration/installable.sh docs/kernel-forwarding.md
git commit -s -m "kernel forwarding: installability audit and inventory

tests/integration/installable.sh simulates installing every binary with
vyatta-kernel-forwarding; docs/kernel-forwarding.md classifies the
dataplane interface model's actions and lists the DPDK-only and deferred
packages, each with its reason.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
git push origin main
cd /Volumes/nudanos && printf '<changed repos>\n' > wave-p3-audit.txt && bash wave-push.sh wave-p3-audit.txt && bash ci-wait.sh wave-p3-audit.txt
```
Expected: CI green for every changed repo.

---

### Task 3: `vyatta-kernel-forwarding`: dataplane NIC names, type table, refusals, iptables

**Files (in `/Volumes/nudanos/port-vyatta-kernel-forwarding`):**
- Create: `dp-name`, `81-vyatta-dp-names.rules`, `netdevice.d/kernel-forwarding`, `yang/vyatta-kernel-forwarding-deviations-v1.yang`, `tests/dp-name.t`, `Makefile`, `debian/vyatta-kernel-forwarding.install`, `debian/vyatta-kernel-forwarding.postinst`, `debian/vyatta-kernel-forwarding-deviations-v1-yang.install`
- Modify: `debian/control`, `debian/rules`, `debian/changelog`

**Interfaces:**
- Produces:
  - `/usr/lib/vyatta-kernel-forwarding/dp-name <ID_NET_NAME_PATH>` prints the dataplane name or nothing;
  - the udev rule names NICs;
  - `/opt/vyatta/etc/netdevice.d/kernel-forwarding` maps `dp` → `dataplane`;
  - the marker file `/opt/vyatta/etc/kernel-forwarding` (Task 5 tests for it);
  - binary `vyatta-kernel-forwarding-deviations-v1-yang`, which refuses the DPDK-only affinity nodes.

- [ ] **Step 1: Write the failing name tests**

`tests/dp-name.t`:
```perl
#!/usr/bin/perl
# dp-name maps a predictable PCI path name to the DANOS dataplane name.
use strict;
use warnings;
use Test::More;

my %cases = (
    'enp0s3'        => 'dp0s3',          # bus 0 drops the "p0"
    'enp0s31f6'     => 'dp0s31f6',       # onboard, function 6
    'enp2s0f1'      => 'dp0p2s0f1',      # multifunction on bus 2
    'enp175s0f1np1' => 'dp0p175s0f1np1', # 14 characters: fits IFNAMSIZ
    'enx001122334455' => '',             # MAC-based (USB): no PCI path
    'enP1p2s0'      => '',               # PCI domain prefix: not mapped
    'enp255s31f7np123' => '',            # would be 17 characters: not truncated
    'wlp3s0'        => '',               # not ethernet
    ''              => '',
);
for my $in ( sort keys %cases ) {
    my $out = `./dp-name '$in'`;
    is( $?, 0, "dp-name '$in' exits 0" );
    chomp $out;
    is( $out, $cases{$in}, "dp-name '$in'" );
}
done_testing();
```

- [ ] **Step 2: Run it to verify it fails**

Run: `cd /Volumes/nudanos/port-vyatta-kernel-forwarding && prove tests/dp-name.t`
Expected: FAIL (`./dp-name: No such file or directory`).

- [ ] **Step 3: Write `dp-name`, the rule and the type table**

`dp-name`:
```sh
#!/bin/sh
# Print the DANOS dataplane name for a predictable PCI path name
# (ID_NET_NAME_PATH), or nothing when it has none: enp0s3 -> dp0s3,
# enp2s0f1 -> dp0p2s0f1. Names over 15 characters are not mapped.
set -eu
p=${1-}
case "$p" in
    enp0s[0-9]*) n="dp0${p#enp0}" ;;
    enp[0-9]*s[0-9]*) n="dp0${p#en}" ;;
    *) exit 0 ;;
esac
[ "${#n}" -le 15 ] && printf '%s\n' "$n"
exit 0
```

`81-vyatta-dp-names.rules`:
```
# Name NICs with DANOS dataplane names (vyatta-kernel-forwarding), so the
# "interfaces dataplane" configuration drives kernel netdevs. Runs after
# 80-net-setup-link.rules; NICs without a PCI path name keep their name.
SUBSYSTEM=="net", ACTION=="add", ENV{ID_NET_NAME_PATH}=="en*", PROGRAM="/usr/lib/vyatta-kernel-forwarding/dp-name $env{ID_NET_NAME_PATH}", RESULT=="?*", NAME="%c"
```

`netdevice.d/kernel-forwarding` (a literal tab between the fields):
```
# Kernel NICs named by vyatta-kernel-forwarding
dp	dataplane
```

`Makefile`:
```make
check:
	prove tests/dp-name.t
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `chmod +x dp-name && prove tests/dp-name.t`
Expected: PASS (18 tests).

- [ ] **Step 5: The DPDK-only refusal**

`yang/vyatta-kernel-forwarding-deviations-v1.yang`:
```yang
module vyatta-kernel-forwarding-deviations-v1 {
	namespace "urn:vyatta.com:mgmt:vyatta-kernel-forwarding-deviations:1";
	prefix vyatta-kernel-forwarding-deviations-v1;

	import vyatta-interfaces-v1 { prefix if; }
	import vyatta-interfaces-dataplane-v1 { prefix interfaces-dataplane; }

	organization "NuDanOS";
	contact "NuDanOS Maintainers <jon@fernandez.tech>";
	description
		"Refuse dataplane interface settings that only the DPDK dataplane
		 implements, on systems that forward in the Linux kernel.

		 SPDX-License-Identifier: LGPL-2.1-only";

	revision 2026-10-01 { description "Initial revision"; }

	deviation /if:interfaces/interfaces-dataplane:dataplane/interfaces-dataplane:cpu-affinity {
		deviate add {
			must "false()" {
				error-message "cpu-affinity requires the DPDK dataplane; this system forwards in the Linux kernel";
			}
		}
	}
	deviation /if:interfaces/interfaces-dataplane:dataplane/interfaces-dataplane:receive-cpu-affinity {
		deviate add {
			must "false()" {
				error-message "receive-cpu-affinity requires the DPDK dataplane; this system forwards in the Linux kernel";
			}
		}
	}
	deviation /if:interfaces/interfaces-dataplane:dataplane/interfaces-dataplane:transmit-cpu-affinity {
		deviate add {
			must "false()" {
				error-message "transmit-cpu-affinity requires the DPDK dataplane; this system forwards in the Linux kernel";
			}
		}
	}
}
```
Before committing, confirm the module and prefix names against the target module:
```bash
grep -nE '^module|prefix|leaf (cpu|receive-cpu|transmit-cpu)-affinity' /Volumes/nudanos/port-vyatta-cfg-dataplane/yang/vyatta-interfaces-dataplane-v1.yang | head
```
Expected: `module vyatta-interfaces-dataplane-v1`, its `import vyatta-interfaces-v1`, and the three leaves directly under the `dataplane` list. Adjust the deviation paths if a leaf sits deeper.

- [ ] **Step 6: Packaging**

`debian/control` adds `Build-Depends: debhelper-compat (= 13), dh-yang` and this binary package:
```
Package: vyatta-kernel-forwarding-deviations-v1-yang
Architecture: all
Depends: vyatta-kernel-forwarding (= ${binary:Version}), vyatta-interfaces-dataplane-v1-yang, ${misc:Depends}, ${yang:Depends}
Description: NuDanOS kernel forwarding: refuse DPDK-only interface settings
 YANG deviations that refuse dataplane interface settings only the DPDK
 dataplane implements (CPU affinity) with a clear message.
```
`vyatta-kernel-forwarding` gains `Depends: ${misc:Depends}, udev`. `debian/rules`:
```make
#!/usr/bin/make -f
%:
	dh $@ --with yang
```
`debian/vyatta-kernel-forwarding.install`:
```
dp-name usr/lib/vyatta-kernel-forwarding
81-vyatta-dp-names.rules usr/lib/udev/rules.d
netdevice.d/kernel-forwarding opt/vyatta/etc/netdevice.d
```
`debian/vyatta-kernel-forwarding-deviations-v1-yang.install`:
```
yang/vyatta-kernel-forwarding-deviations-v1.yang usr/share/configd/yang
```
`debian/vyatta-kernel-forwarding.postinst`. It moves DANOS `14-iptables-legacy` here (base spec §4.5) and writes the marker file:
```sh
#!/bin/sh
set -e
case "$1" in
    configure)
        # DANOS used the legacy iptables/ebtables back ends (build-iso hook
        # 14-iptables-legacy); keep that for its scripts.
        for t in iptables ip6tables ebtables; do
            if [ -x /usr/sbin/$t-legacy ]; then
                update-alternatives --quiet --set $t /usr/sbin/$t-legacy
            fi
        done
        # Marker for scripts that behave differently without the DPDK
        # dataplane (vyatta-bridge.pl keeps kernel flooding on).
        mkdir -p /opt/vyatta/etc
        : > /opt/vyatta/etc/kernel-forwarding
        ;;
esac
#DEBHELPER#
exit 0
```
`debian/vyatta-kernel-forwarding.postrm`:
```sh
#!/bin/sh
set -e
case "$1" in
    remove|purge) rm -f /opt/vyatta/etc/kernel-forwarding ;;
esac
#DEBHELPER#
exit 0
```
Add a changelog entry `0.2` describing the names, the type table, the deviations and the iptables/marker postinst. Then remove the now-obsolete `empty-binary-package` override:

```bash
git rm debian/vyatta-kernel-forwarding.lintian-overrides
```

- [ ] **Step 7: Build, check the package contents, push**

```bash
cd /Volumes/nudanos/distro
./distro-build -work /Volumes/nudanos/work -local vyatta-kernel-forwarding=../port-vyatta-kernel-forwarding build vyatta-kernel-forwarding 2>&1 | tail -3
dpkg-deb -c /Volumes/nudanos/work/out/vyatta-kernel-forwarding/vyatta-kernel-forwarding_0.2_all.deb | grep -E 'dp-name|rules.d|netdevice.d'
```
Expected: `built`, and three paths listed (run `dpkg-deb` inside the builder image if it is not on the Mac: `docker run --rm -v /Volumes/nudanos/work/out:/o nudanos/builder:trixie dpkg-deb -c /o/vyatta-kernel-forwarding/vyatta-kernel-forwarding_0.2_all.deb`). Commit the port repo, then run `printf 'vyatta-kernel-forwarding\n' > /Volumes/nudanos/wave-p3-kf.txt && bash /Volumes/nudanos/wave-push.sh /Volumes/nudanos/wave-p3-kf.txt` and wait for its CI.

---

### Task 4: Image hooks move into packages (base spec §4.5)

**Files:**
- Modify: `port-base-files/debian/preinst`, `port-base-files/debian/postrm`, `port-base-files/etc/os-release.vyatta`, `port-vyatta-cfg-system/debian/vyatta-system.postinst`
- Create: `port-cli-sandbox/scripts/cli-sandbox-create`, `port-cli-sandbox/debian/vyatta-system-login-user-isolation-v1-yang.postinst`
- Modify: `port-cli-sandbox/debian/control`, `port-cli-sandbox/debian/vyatta-system-login-user-isolation-v1-yang.install` (or the `.install` that ships `scripts/`)
- Create (distro): `tools/yangcheck.sh`, a step in `.github/workflows/ci.yml`

**Interfaces:**
- Produces: `/etc/os-release` → NuDanOS branding; a loopback stanza in `/etc/network/interfaces`; `/usr/sbin/cli-sandbox-create [ROOT]`; `tools/yangcheck.sh WORK` (build-time YANG compile check, replacing `98-yangcheck`).

- [ ] **Step 1: Write the double-run test (Review Focus 2)**

`tests/integration/hooks-idempotent.sh` (in distro):
```bash
#!/bin/bash
# Install the packages that took over DANOS image hooks, run their maintainer
# scripts a second time, and check nothing is duplicated; purge base-files-vyatta
# and check the os-release diversion is gone.
set -euo pipefail
: "${WORK:?}"
ENGINE=${ENGINE:-docker}
$ENGINE run --rm -v "$WORK/repo":/repo:ro debian:trixie bash -euxc '
  apt-get update -qq >/dev/null && apt-get install -y -qq ca-certificates gpg >/dev/null
  gpg --dearmor < /repo/nudanos-archive-keyring.asc > /usr/share/keyrings/nudanos.gpg
  echo "deb [signed-by=/usr/share/keyrings/nudanos.gpg] file:/repo trixie main" > /etc/apt/sources.list.d/nudanos.list
  apt-get update -qq >/dev/null
  apt-get install -y -qq --no-install-recommends vyatta-kernel-forwarding base-files-vyatta vyatta-system >/dev/null
  grep -q "NuDanOS" /etc/os-release
  dpkg-reconfigure base-files-vyatta vyatta-system
  test "$(grep -c "^auto lo" /etc/network/interfaces)" = 1
  test "$(dpkg-divert --list /etc/os-release | wc -l)" = 1
  apt-get purge -y -qq base-files-vyatta >/dev/null
  test -z "$(dpkg-divert --list /etc/os-release)"
  grep -q "Debian" /etc/os-release
  echo hooks-idempotent: OK'
```
Run: `WORK=/Volumes/nudanos/work tests/integration/hooks-idempotent.sh`
Expected: FAIL at `grep -q "NuDanOS" /etc/os-release`.

- [ ] **Step 2: `os-release` → `base-files-vyatta` (from `96-os-release`)**

`etc/os-release.vyatta`:
```
PRETTY_NAME="NuDanOS"
NAME="NuDanOS"
ID=vyatta
ID_LIKE=debian
HOME_URL="https://github.com/nudanos"
SUPPORT_URL="https://github.com/nudanos/distro/issues"
BUG_REPORT_URL="https://github.com/nudanos/distro/issues"
```
`ID` stays `vyatta`: DANOS scripts test it. In `debian/preinst` and `debian/postrm`, change both loops from `for file in /etc/issue /etc/issue.net; do` to `for file in /etc/issue /etc/issue.net /etc/os-release; do`. The existing `[ -e ${file}.debian ]` guard makes the preinst idempotent. Changelog entry.

- [ ] **Step 3: Loopback → `vyatta-system` postinst (from `01-interfaces`)**

In `port-vyatta-cfg-system/debian/vyatta-system.postinst`, inside its `configure)` branch:
```sh
        # The loopback interface (DANOS build-iso hook 01-interfaces).
        if ! grep -q '^auto lo' /etc/network/interfaces 2>/dev/null; then
            mkdir -p -m 0755 /etc/network
            printf '# The loopback network interface\nauto lo\niface lo inet loopback\n' >> /etc/network/interfaces
        fi
```
Changelog entry.

- [ ] **Step 4: Sandbox → `cli-sandbox` (from `0990-create-chroot-fs`)**

`scripts/cli-sandbox-create`:
```sh
#!/bin/sh
# Create the user-isolation sandbox root (DANOS build-iso hook
# 0990-create-chroot-fs) from this system's apt sources.
# SPDX-License-Identifier: GPL-2.0-only
set -eu
ROOT=${1:-/var/lib/sandbox}
INCLUDES="vyatta-op-shell,vyatta-config-shell,vyatta-bash,bash-completion,openssh-client,vcli,locales,vyatta-password-renewal"
[ -e "$ROOT/etc/os-release" ] && exit 0
mkdir -p "$ROOT"
cat /etc/apt/sources.list /etc/apt/sources.list.d/*.list /etc/apt/sources.list.d/*.sources 2>/dev/null \
    | mmdebstrap --variant=minbase --include="$INCLUDES" trixie "$ROOT" -
chroot "$ROOT" apt-get clean
```
`debian/vyatta-system-login-user-isolation-v1-yang.postinst`:
```sh
#!/bin/sh
set -e
case "$1" in
    configure) /usr/sbin/cli-sandbox-create || echo "warning: could not create the sandbox root; run cli-sandbox-create" >&2 ;;
esac
#DEBHELPER#
exit 0
```
Add `mmdebstrap` to that package's `Depends` and install the script under `usr/sbin`. The sandbox needs the network at install time, so `nudanos-router` (Task 7) does **not** include user isolation. Record that in the ledger as `Ruling:` (it is optional in DANOS too: the hook skipped itself unless the package was present).

- [ ] **Step 5: YANG check → CI (from `98-yangcheck`)**

`tools/yangcheck.sh`:
```bash
#!/bin/bash
# Compile every YANG module the built packages ship, as the DANOS image hook
# 98-yangcheck did at ISO build time. Usage: tools/yangcheck.sh WORK
set -euo pipefail
WORK=${1:?usage: yangcheck.sh WORK}
docker run --rm -v "$WORK/out":/out:ro nudanos/builder:trixie bash -euc '
  mkdir -p /tmp/y && cd /tmp/y
  for d in $(find /out -name "*.deb"); do dpkg-deb -x "$d" . ; done
  apt-get update -qq >/dev/null; apt-get install -y -qq /out/configd/yang-utils_*.deb /out/configd/configd_*.deb >/dev/null 2>&1 || true
  yangc -check -yangdir usr/share/configd/yang'
```
Before relying on it, confirm the `yangc` binary name and flags:
```bash
docker run --rm -v /Volumes/nudanos/work/out:/out:ro nudanos/builder:trixie bash -c 'dpkg-deb -c /out/configd/yang-utils_*.deb | grep -E "bin/(yangc|yang)"'
```
Adjust the last line of the script to the real invocation, which DANOS's hook used as `yangc -check`. Add a nightly step after the build that runs `tools/yangcheck.sh "$RUNNER_TEMP/work"` (Task 11 wires it).

- [ ] **Step 6: Build, run the double-run test, push**

```bash
cd /Volumes/nudanos/distro
./distro-build -work /Volumes/nudanos/work -jobs 4 -local base-files=../port-base-files -local vyatta-cfg-system=../port-vyatta-cfg-system -local cli-sandbox=../port-cli-sandbox build base-files vyatta-cfg-system cli-sandbox 2>&1 | tail -4
./distro-build -work /Volumes/nudanos/work -key "$(cat keys/FINGERPRINT)" repo >/dev/null
WORK=/Volumes/nudanos/work tests/integration/hooks-idempotent.sh | tail -1
```
Expected: `hooks-idempotent: OK`. Commit `tests/integration/hooks-idempotent.sh` and `tools/yangcheck.sh` to distro, push the three port branches with `wave-push.sh`, and wait for CI.

---

### Task 5: Runtime fixes for kernel forwarding

**Files:**
- `port-vyatta-service-ntp`: `debian/control`, `debian/ntpd@.service`, `debian/vyatta-service-ntp.install`, `debian/vyatta-service-ntp.tmpfile`, `scripts/vyatta_configure_ntp.pl`, `scripts/vyatta_update_ntpkeys`, `sysconf/ntp.service.d/` → `sysconf/ntpsec.service.d/`, `templates/show/ntp/**`
- `port-vyatta-service-bridge`: `scripts/vyatta-bridge.pl`, new `tests/bridge-flood.t`
- `port-vyatta-vrrp`: `vyatta/vrrp_vci/keepalived/vrrp.py`, `vyatta/vrrp_vci/keepalived/config_file.py`, `tests/` (fixtures and a new test)
- `port-vyatta-interfaces`: `debian/control` (`libvyatta-interface-perl` declares `Vyatta::ioctl`)
- `port-vyatta-cfg-system`: `scripts/vyatta-show-version`, `tests/vyatta-show-version.t`

**Interfaces:**
- Consumes: the marker `/opt/vyatta/etc/kernel-forwarding` (Task 3).

- [ ] **Step 1: VRRP: failing test first**

In `port-vyatta-vrrp/tests/`, add to the config-file tests:
```python
def test_start_delay_is_a_global_startup_delay(simple_config):
    """keepalived 2.4 has no per-instance start_delay; the delay becomes
    global_defs vrrp_startup_delay (all groups share one delay)."""
    from vyatta.vrrp_vci.keepalived.config_file import KeepalivedConfig
    cfg = KeepalivedConfig()
    simple_config["vyatta-interfaces-v1:interfaces"]["vyatta-interfaces-dataplane-v1:dataplane"][0]["vyatta-vrrp-v1:vrrp"]["start-delay"] = 30
    cfg.update(simple_config)
    out = cfg.config_string()
    assert "vrrp_startup_delay 30" in out
    assert "start_delay" not in out.replace("vrrp_startup_delay", "")
```
Match the fixture name and update API to the existing tests: `grep -n "def test_.*config\|KeepalivedConfig(" tests/*.py | head`. Run: `cd /Volumes/nudanos/port-vyatta-vrrp && python3 -m pytest -q -k start_delay_is_a_global` (inside the builder image if pytest's deps are missing on the Mac). Expected: FAIL.

- [ ] **Step 2: VRRP: implement**

In `vrrp.py`, delete the line `    start_delay {delay}` from the instance template, along with its `.format()` argument. In `config_file.py`, where `global_defs` is rendered, add after `dynamic_interfaces allow_if_changes`:
```python
        startup_delay = max((g.start_delay for g in self._vrrp_instances), default=0)
        if startup_delay:
            config_string = config_string.replace(
                "dynamic_interfaces allow_if_changes",
                f"dynamic_interfaces allow_if_changes\n        vrrp_startup_delay {startup_delay}")
```
Adapt the attribute names to the code: `VrrpGroup` stores the delay passed as its second constructor argument. Update the fixtures in `tests/conftest.py` that contain `start_delay 0` (delete those lines). Run the whole suite: `python3 -m pytest -q`. Expected: all pass.

- [ ] **Step 3: Bridge: failing test first**

`port-vyatta-service-bridge/tests/bridge-flood.t`:
```perl
#!/usr/bin/perl
# With kernel forwarding there is no DPDK dataplane to flood; bridge ports must
# keep kernel broadcast/multicast flooding on.
use strict; use warnings;
use Test::More;
use File::Temp qw(tempdir);
use lib 'lib';
require 'scripts/vyatta-bridge.pl';
my $root = tempdir( CLEANUP => 1 );
for my $kf ( 0, 1 ) {
    my $d = "$root/sys/devices/virtual/net/br0/brif/dp0s3";
    system("mkdir -p $d && echo 1 > $d/multicast_flood && echo 1 > $d/broadcast_flood");
    set_port_flooding( 'br0', 'dp0s3', $kf, "$root/sys" );
    my $want = $kf ? '1' : '0';
    is( `cat $d/multicast_flood` =~ s/\s+//r, $want, "multicast_flood with kernel_forwarding=$kf" );
    is( `cat $d/broadcast_flood` =~ s/\s+//r, $want, "broadcast_flood with kernel_forwarding=$kf" );
}
done_testing();
```
Run: `prove tests/bridge-flood.t`. Expected: FAIL (`Undefined subroutine &main::set_port_flooding`). If the script runs its main body when `require`d, guard the body with `main() unless caller;` as part of Step 4.

- [ ] **Step 4: Bridge: implement**

In `scripts/vyatta-bridge.pl`, replace the two `write_file(… multicast_flood …)` and `write_file(… broadcast_flood …)` blocks in `add_bridge_port` with:
```perl
    set_port_flooding( $bridge, $port, -e '/opt/vyatta/etc/kernel-forwarding', '/sys' )
      or exit 1;
```
and add:
```perl
# The DPDK dataplane floods, so DANOS turned kernel flooding off on bridge
# ports. With kernel forwarding the kernel must flood (ARP broadcasts).
sub set_port_flooding {
    my ( $bridge, $port, $kernel_forwarding, $sys ) = @_;
    my $val = $kernel_forwarding ? 1 : 0;
    for my $f (qw(multicast_flood broadcast_flood)) {
        write_file( "$sys/devices/virtual/net/$bridge/brif/$port/$f", $val ) == 1
          or return 0;
    }
    return 1;
}
```
Make sure `debian/rules` or the Makefile runs `prove tests/` in `dh_auto_test`; add it if not. Run: `prove tests/bridge-flood.t`. Expected: PASS (4 tests).

- [ ] **Step 5: ntpsec**

In `port-vyatta-service-ntp`, make these replacements. Afterwards the check command below must print nothing.

| From | To |
|---|---|
| `debian/control`: `ntp (>= 1:4.2.7p22), ntpdate` | `ntpsec, ntpsec-ntpdate` |
| `/etc/ntp.conf` (scripts, tmpfile, templates) | `/etc/ntpsec/ntp.conf` |
| `systemctl … ntp` / `ntp.service` | `ntpsec` / `ntpsec.service` |
| `sysconf/ntp.service.d/configuration-exists.conf` → `/usr/lib/systemd/system/ntp.service.d` | `sysconf/ntpsec.service.d/…` → `/usr/lib/systemd/system/ntpsec.service.d` |
| `templates/show/ntp/packets/node.def` (`ntpq -c privatestat` and the private stat lines) | only `ntpq --wide -n -c iostats` |
| `debian/ntpd@.service` `ExecStart` | keep `/usr/sbin/ntpd` (ntpsec's daemon has the same name and `-p`, `-g`, `-n`, `-c` options) |

`scripts/vyatta_update_ntpkeys` keeps its key file under `/etc/ntpsec/`. Check what it writes first: `grep -n "keys" scripts/vyatta_update_ntpkeys`.
```bash
cd /Volumes/nudanos/port-vyatta-service-ntp
git grep -n -E '/etc/ntp\.conf|systemctl [a-z-]+ ntp($|[^s@d])|ntp\.service|privatestat|\bntpdate\b' -- . ':!debian/changelog'
```
Expected: no output, except `ntpdate` inside `templates/set/date/ntp/*`, which ntpsec-ntpdate still provides as a command.

- [ ] **Step 6: `Vyatta::ioctl` dependency**

In `port-vyatta-interfaces/debian/control`, add `libvyatta-ioctl-perl` to `libvyatta-interface-perl`'s `Depends`. `vyatta-system` provides it (`Provides: libvyatta-ioctl-perl`).

- [ ] **Step 7: `show version` reports the base, kernel and FRR (spec addendum §1)**

Add to `port-vyatta-cfg-system/tests/vyatta-show-version.t` a test that runs `vyatta-show-version` in a fixture root and expects lines matching:
```perl
like( $out, qr/^Base:\s+Debian GNU\/Linux \d+/m,  'Base line' );
like( $out, qr/^Kernel:\s+\S+/m,                  'Kernel line' );
like( $out, qr/^FRR:\s+\S+/m,                     'FRR line' );
```
Follow the file's existing fixture style (`grep -n "sub \|local \$ENV\|/etc/os-release" tests/vyatta-show-version.t | head`). Run `prove tests/vyatta-show-version.t`; expected: FAIL. Then add to `scripts/vyatta-show-version`, after the `Description:` line:
```perl
    my $debver = -r '/etc/debian_version' ? read_file('/etc/debian_version') : '';
    chomp $debver;
    print "Base:         Debian GNU/Linux $debver\n" if $debver ne '';
    print "Kernel:       " . ( POSIX::uname() )[2] . "\n";
    my $frr = `dpkg-query -W -f='\${Version}' frr 2>/dev/null`;
    print "FRR:          $frr\n" if $frr ne '';
```
Expected after the change: PASS.

- [ ] **Step 8: Build all five, push**

```bash
cd /Volumes/nudanos/distro
L=""; for r in vyatta-service-ntp vyatta-service-bridge vyatta-vrrp vyatta-interfaces vyatta-cfg-system; do L="$L -local $r=../port-$r"; done
./distro-build -work /Volumes/nudanos/work -jobs 4 $L build vyatta-service-ntp vyatta-service-bridge vyatta-vrrp vyatta-interfaces vyatta-cfg-system 2>&1 | tail -6
```
Expected: all five `built`. Push with `wave-push.sh` and wait for CI.

---

### Task 6: `install image` refuses the default password

**Files (in `/Volumes/nudanos/port-vyatta-image-tools`):**
- Modify: `scripts/vyatta-install-image.functions` (`_dialog_enter_password`), `tests/vyatta-install-image.functions.sh`

**Interfaces:**
- Produces: `_password_acceptable PASSWORD` returns 0, or prints the reason to stderr and returns 1.

- [ ] **Step 1: Failing test**

Append to `tests/vyatta-install-image.functions.sh` (shunit2):
```sh
test_password_acceptable ()
{
    _password_acceptable "s3cret-Pass" 2>/dev/null
    assertEquals "a real password" 0 $?
    _password_acceptable "" 2>/dev/null
    assertNotEquals "empty" 0 $?
    _password_acceptable "vyatta" 2>/dev/null
    assertNotEquals "the published live-ISO default" 0 $?
    msg=$(_password_acceptable "vyatta" 2>&1)
    assertContains "names the reason" "$msg" "default"
}
```
Run: `cd tests && sh vyatta-install-image.functions.sh` (inside the builder image: `docker run --rm -v $PWD:/s -w /s/tests nudanos/builder:trixie bash -c 'apt-get install -y -qq shunit2 >/dev/null; sh vyatta-install-image.functions.sh'`). Expected: FAIL (`_password_acceptable: command not found`).

- [ ] **Step 2: Implement**

In `scripts/vyatta-install-image.functions`, add before `_dialog_enter_password`:
```sh
# Returns 0 if $1 may be the administrator password of an installed system.
# The live ISO logs in as vyatta/vyatta; no installed system may keep it.
_password_acceptable ()
{
    case "$1" in
        "") echo "'' is not a valid password" >&2; return 1 ;;
        vyatta) echo "'vyatta' is the published live-ISO default; choose another password" >&2; return 1 ;;
    esac
    return 0
}
```
In `_dialog_enter_password`, replace the `if [[ "$pwd1" == "" ]]; then … continue; fi` block with:
```sh
    if ! _password_acceptable "$pwd1" 2>/dev/tty; then
        pwd1="1"
        pwd2="2"
        continue
    fi
```
Run the test again. Expected: PASS. Build `vyatta-image-tools`, push with `wave-push.sh`, wait for CI.

---

### Task 7: `nudanos-router` meta-package (new repo)

**Files:**
- Create (distro): `tools/router_deps.py`, `tools/test_router_deps.py`
- Create (new repo `nudanos/nudanos-router`): `debian/control`, `debian/rules`, `debian/changelog`, `debian/copyright`, `debian/source/format`, `README.md`, `.github/workflows/package.yml`
- Modify (distro): `manifest.yaml` (new entry), `.github/workflows/ci.yml` (drift check)

**Interfaces:**
- Consumes: the repo's `Packages` index; `docs/kernel-forwarding.md` exclusion tables (Task 2).
- Produces: `router_deps.py PACKAGES_FILE KF_DOC` prints the sorted dependency list; `router_deps.py --check CONTROL PACKAGES_FILE KF_DOC` exits 1, listing the difference, when `nudanos-router`'s `Depends` drift from it.

- [ ] **Step 1: Failing tests**

`tools/test_router_deps.py`:
```python
from __future__ import annotations

import importlib.util
import os
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
spec = importlib.util.spec_from_file_location("router_deps", os.path.join(HERE, "router_deps.py"))
rd = importlib.util.module_from_spec(spec)
spec.loader.exec_module(rd)

PACKAGES = """Package: vyatta-system
Source: vyatta-cfg-system
Architecture: amd64

Package: vplane-config
Source: vyatta-cfg-dataplane
Architecture: all

Package: libfoo-dev
Source: foo
Architecture: amd64

Package: vyatta-kernel-forwarding
Architecture: all

Package: vyatta-dataplane
Architecture: amd64
"""

DOC = """## DPDK-only binaries (left out of nudanos-router)

| Package | Why |
|---|---|
| vplane-config | the DPDK dataplane's config backend |

## Deferred to a later milestone

| Package | Milestone | Why |
|---|---|---|
"""


class RouterDepsTest(unittest.TestCase):
    def test_excludes_dpdk_only_dev_and_dataplane(self):
        self.assertEqual(rd.router_deps(PACKAGES, DOC), ["vyatta-kernel-forwarding", "vyatta-system"])

    def test_check_reports_drift(self):
        control = "Package: nudanos-router\nDepends: vyatta-system, ${misc:Depends}\n"
        missing, extra = rd.drift(control, PACKAGES, DOC)
        self.assertEqual((missing, extra), (["vyatta-kernel-forwarding"], []))


if __name__ == "__main__":
    unittest.main()
```
Run: `cd /Volumes/nudanos/distro && python3 -m unittest tools/test_router_deps.py`. Expected: FAIL (`router_deps.py` does not exist).

- [ ] **Step 2: Implement `tools/router_deps.py`**

```python
#!/usr/bin/env python3
"""nudanos-router's dependency list: every binary in the repo except development
packages, debug symbols, the DPDK dataplane itself, and the packages
docs/kernel-forwarding.md excludes (DPDK-only or deferred), each with a reason.

  router_deps.py PACKAGES_FILE KF_DOC              print the list
  router_deps.py --check CONTROL PACKAGES_FILE KF_DOC   exit 1 on drift
"""
from __future__ import annotations

import re
import sys

ALWAYS_OUT = {"vyatta-dataplane", "nudanos-router"}
SUFFIX_OUT = ("-dev", "-dbgsym", "-dbg", "-doc", "-tests", "-test")


def excluded(doc: str) -> set[str]:
    out, on = set(), False
    for line in doc.splitlines():
        if line.startswith("## "):
            on = line.startswith("## DPDK-only") or line.startswith("## Deferred")
            continue
        m = re.match(r"\|\s*([a-z0-9][a-z0-9.+-]+)\s*\|", line)
        if on and m and m.group(1) != "Package":
            out.add(m.group(1))
    return out


def router_deps(packages: str, doc: str) -> list[str]:
    names = set(re.findall(r"^Package: (\S+)$", packages, re.M))
    skip = excluded(doc) | ALWAYS_OUT
    return sorted(n for n in names if n not in skip and not n.endswith(SUFFIX_OUT))


def drift(control: str, packages: str, doc: str) -> tuple[list[str], list[str]]:
    m = re.search(r"^Package: nudanos-router$.*?^Depends:(.*?)(?=^\S|\Z)", control, re.M | re.S)
    have = {d.strip().split()[0] for d in (m.group(1) if m else "").split(",") if d.strip() and not d.strip().startswith("$")}
    want = set(router_deps(packages, doc))
    return sorted(want - have), sorted(have - want)


def main(argv: list[str]) -> int:
    if argv and argv[0] == "--check":
        control, packages, doc = (open(p).read() for p in argv[1:4])
        missing, extra = drift(control, packages, doc)
        for n in missing:
            print(f"missing from nudanos-router: {n}")
        for n in extra:
            print(f"in nudanos-router but not in the set: {n}")
        return 1 if missing or extra else 0
    packages, doc = (open(p).read() for p in argv[0:2])
    print("\n".join(router_deps(packages, doc)))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
```
Run: `python3 -m unittest tools/test_router_deps.py`. Expected: PASS (2 tests).

- [ ] **Step 3: Create the repo** (approved by the user in brainstorming)

```bash
gh repo create nudanos/nudanos-router --public --description "NuDanOS router meta-package: the milestone-1 package set" 
mkdir -p /Volumes/nudanos/port-nudanos-router && cd /Volumes/nudanos/port-nudanos-router && git init -q -b trixie
git remote add origin https://github.com/nudanos/nudanos-router.git
```

- [ ] **Step 4: Write the package**

`debian/control`:
```
Source: nudanos-router
Section: metapackages
Priority: optional
Maintainer: NuDanOS Maintainers <jon@fernandez.tech>
Build-Depends: debhelper-compat (= 13)
Standards-Version: 4.7.2
Rules-Requires-Root: no
Vcs-Git: https://github.com/nudanos/nudanos-router.git
Vcs-Browser: https://github.com/nudanos/nudanos-router

Package: nudanos-router
Architecture: all
Depends: ${misc:Depends},
 <one line per package from: python3 /Volumes/nudanos/distro/tools/router_deps.py /Volumes/nudanos/work/repo/dists/trixie/main/binary-amd64/Packages /Volumes/nudanos/distro/docs/kernel-forwarding.md>
Description: NuDanOS router (milestone 1: kernel forwarding)
 Installs the NuDanOS router package set: the Vyatta configuration system,
 FRR routing, VRRP and the services of milestone 1, forwarding in the Linux
 kernel. No firewall, NAT or QoS (those need the DPDK dataplane).
```
Generate the `Depends` lines with the command shown in the placeholder line (`sed 's/^/ /;s/$/,/'`), and drop the trailing comma on the last one. `debian/rules` is `dh $@`; `debian/source/format` is `3.0 (native)`; `debian/changelog` has `nudanos-router (1.0~1) trixie; urgency=medium` with `* Initial release.`; `debian/copyright` is DEP-5 with `License: LGPL-2.1-only`. Add the CI caller exactly as `wave-push.sh` writes it, with `package: nudanos-router`.

- [ ] **Step 5: Manifest entry and the drift check**

`manifest.yaml`, in name order:
```yaml
  - name: "nudanos-router"
    kind: "danos"
    milestone: "1.0"
    ready: true
    repo: "https://github.com/nudanos/nudanos-router"
    ref: "trixie"
    note: "meta-package: the milestone-1 package set (spec 4.5); its Depends are checked by tools/router_deps.py"
```
Add to `.github/workflows/nightly.yml`, after the build step (Task 11 places it):
```yaml
      - name: nudanos-router covers the package set
        run: |
          ./distro-build -work "$RUNNER_TEMP/work" -key "$(cat keys/FINGERPRINT)" repo
          python3 tools/router_deps.py --check "$RUNNER_TEMP/work/src/nudanos-router/debian/control" \
            "$RUNNER_TEMP/work/repo/dists/trixie/main/binary-amd64/Packages" docs/kernel-forwarding.md
```

- [ ] **Step 6: Build, check it installs, push**

```bash
cd /Volumes/nudanos/distro
./distro-build -work /Volumes/nudanos/work -local nudanos-router=../port-nudanos-router build nudanos-router 2>&1 | tail -2
./distro-build -work /Volumes/nudanos/work -key "$(cat keys/FINGERPRINT)" repo >/dev/null
WORK=/Volumes/nudanos/work tests/integration/installable.sh nudanos-router
```
Expected: `built`, then `OK nudanos-router`. Commit distro (`manifest.yaml`, `tools/router_deps.py`, its test) and push `main` first. Then commit the new repo and push it: `git -C ../port-nudanos-router push -u origin trixie`. Set the default branch: `gh repo edit nudanos/nudanos-router --default-branch trixie`. Wait for its CI.

---

### Task 8: Layer 2: `distro-build test install`

**Files:**
- Modify: `internal/engine/engine.go`, `internal/engine/engine_test.go`, `cmd/distro-build/main.go`
- Create: `internal/installtest/installtest.go`, `internal/installtest/installtest_test.go`, `tests/integration/install-purge.sh`, `tests/integration/install-allowlist.txt`

**Interfaces:**
- Produces:
  - `engine.RunSpec{Privileged bool; Devices []string}`;
  - `installtest.Spec(repoDir, testsDir string) engine.RunSpec`;
  - the command `distro-build test install`.

- [ ] **Step 1: Failing engine test**

Append to `internal/engine/engine_test.go`:
```go
func TestRunArgsPrivilegedAndDevices(t *testing.T) {
	got := Engine{Bin: "docker"}.RunArgs(RunSpec{Image: "img", Privileged: true, Devices: []string{"/dev/kvm"}})
	want := []string{"run", "--rm", "--privileged", "--device", "/dev/kvm", "img"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("RunArgs = %v, want %v", got, want)
	}
}
```
Run: `go test ./internal/engine/ -run Privileged`. Expected: FAIL (unknown fields).

- [ ] **Step 2: Implement**

In `RunSpec` add:
```go
	Privileged bool     // live-build needs mounts and loop devices
	Devices    []string // host devices passed through, e.g. /dev/kvm
```
In `RunArgs`, after the `--network` block:
```go
	if s.Privileged {
		args = append(args, "--privileged")
	}
	for _, d := range s.Devices {
		args = append(args, "--device", d)
	}
```
Run: `go test ./internal/engine/`. Expected: PASS.

- [ ] **Step 3: The layer 2 script and allowlist**

`tests/integration/install-purge.sh`:
```bash
#!/bin/bash
# Layer 2 (spec 8.2): in a clean Debian 13 container, install nudanos-router
# from /repo, remove and purge everything it brought, and fail on any
# maintainer-script error or on files left outside the allowlist.
set -euo pipefail
snapshot() {
    find / -xdev \( -path /proc -o -path /sys -o -path /dev -o -path /run -o -path /tmp \
        -o -path /var/cache -o -path /var/lib/apt -o -path /var/lib/dpkg -o -path /var/log \
        -o -path /repo -o -path /tests \) -prune -o -print | sort
}
apt-get update -qq && apt-get install -y -qq ca-certificates gpg >/dev/null
gpg --dearmor < /repo/nudanos-archive-keyring.asc > /usr/share/keyrings/nudanos.gpg
echo "deb [signed-by=/usr/share/keyrings/nudanos.gpg] file:/repo trixie main" > /etc/apt/sources.list.d/nudanos.list
apt-get update -qq
dpkg-query -W -f='${Package}\n' | sort > /tmp/pkgs-before
snapshot > /tmp/files-before
DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends nudanos-router
dpkg-query -W -f='${Package}\n' | sort > /tmp/pkgs-after
comm -13 /tmp/pkgs-before /tmp/pkgs-after > /tmp/pkgs-added
DEBIAN_FRONTEND=noninteractive apt-get purge -y $(cat /tmp/pkgs-added)
snapshot > /tmp/files-after
comm -13 /tmp/files-before /tmp/files-after > /tmp/left
grep -v -E -f <(grep -v '^#' /tests/install-allowlist.txt | awk 'NF{print $1}') /tmp/left > /tmp/unexplained || true
if [ -s /tmp/unexplained ]; then
    echo "files left after purge:"; cat /tmp/unexplained; exit 1
fi
echo "install-purge: OK ($(wc -l < /tmp/pkgs-added) packages)"
```
`tests/integration/install-allowlist.txt` holds one regex per line, then the reason. The first run shows what is left. Add an entry only with a reason it is documented state, and fix the package for anything else. Start with:
```
^/config(/|$)      Vyatta configuration directory: user data, kept on purge by design
^/opt/vyatta/etc/config(/|$)   compat path for /config
```

- [ ] **Step 4: Failing spec test, then `installtest.Spec`**

`internal/installtest/installtest_test.go`:
```go
package installtest

import "testing"

func TestSpecMountsRepoAndTestsReadOnly(t *testing.T) {
	s := Spec("/w/repo", "/d/tests/integration")
	if s.Image != "debian:trixie" || len(s.Mounts) != 2 || !s.Mounts[0].ReadOnly || !s.Mounts[1].ReadOnly {
		t.Fatalf("spec = %+v", s)
	}
	if s.Cmd[len(s.Cmd)-1] != "/tests/install-purge.sh" {
		t.Errorf("cmd = %v", s.Cmd)
	}
}
```
`internal/installtest/installtest.go`:
```go
// Package installtest runs the layer 2 install/remove/purge test (spec 8.2).
package installtest

import "github.com/nudanos/distro/internal/engine"

// Spec runs tests/integration/install-purge.sh in a clean debian:trixie
// container against the signed repo in repoDir.
func Spec(repoDir, testsDir string) engine.RunSpec {
	return engine.RunSpec{Image: "debian:trixie",
		Mounts: []engine.Mount{
			{Host: repoDir, Container: "/repo", ReadOnly: true},
			{Host: testsDir, Container: "/tests", ReadOnly: true},
		},
		Cmd: []string{"bash", "/tests/install-purge.sh"}}
}
```
Run: `go test ./internal/installtest/`. Expected: FAIL first (no package), then PASS.

- [ ] **Step 5: Wire `test install` into `distro-build`**

In `run()`'s `switch`:
```go
	case "test":
		return a.test(ctx, names)
```
New method:
```go
// test runs the image test layers: "install" (layer 2) and "boot" (layer 3).
func (a *app) test(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: distro-build test install|boot")
	}
	repo := filepath.Join(a.work, "repo")
	if _, err := os.Stat(filepath.Join(repo, "dists", "trixie", "InRelease")); err != nil {
		return fmt.Errorf("test: no signed repo in %s; run 'distro-build repo' first", repo)
	}
	switch args[0] {
	case "install":
		tests, err := filepath.Abs(filepath.Join(filepath.Dir(a.manifest), "tests", "integration"))
		if err != nil {
			return err
		}
		return a.eng.Run(ctx, installtest.Spec(repo, tests), os.Stdout, os.Stderr)
	case "boot":
		return a.testBoot(ctx)
	}
	return fmt.Errorf("unknown test %q (want install or boot)", args[0])
}
```
Add a temporary `func (a *app) testBoot(ctx context.Context) error { return fmt.Errorf("test boot: not implemented yet") }`; Task 10 replaces it. Update the usage line to `builder|fetch|plan|build|repo|image|test install|test boot|check-updates`. Run: `go vet ./... && go test ./...`. Expected: PASS.

- [ ] **Step 6: Run layer 2 for real**

```bash
cd /Volumes/nudanos/distro && go build -o distro-build ./cmd/distro-build
./distro-build -work /Volumes/nudanos/work -key "$(cat keys/FINGERPRINT)" repo >/dev/null
./distro-build -work /Volumes/nudanos/work test install 2>&1 | tail -15
```
Expected: `install-purge: OK (N packages)`.
- **Maintainer-script errors:** fix them in the package, on its `trixie` branch, and rebuild.
- **Leftover files:** fix the package's `postrm`, or allowlist the path with a reason if it is documented state.

Record each fix as a `Ruling:` line. Commit and push distro.

---

### Task 9: `distro-build image` and `distro/image/`

**Files:**
- Create: `image/auto/config`, `image/config/package-lists/nudanos.list.chroot`, `image/config/archives/nudanos.list.chroot`, `image/config/archives/backports.list.chroot`, `image/config/archives/backports.pref.chroot`, `image/config/hooks/live/*` (ported), `image/config/includes.chroot/etc/…` (ported), `image/config/bootloaders/grub-pc/grub.cfg`, `image/config/bootloaders/grub-efi/grub.cfg`, `image/LICENSE`, `builder/make-image.sh`, `internal/imagebuild/imagebuild.go`, `internal/imagebuild/imagebuild_test.go`
- Modify: `builder/Dockerfile` (install `make-image`), `cmd/distro-build/main.go`

**Interfaces:**
- Consumes: the signed repo in `work/repo` (`nudanos-archive-keyring.asc`, `dists/trixie/InRelease`).
- Produces:
  - `imagebuild.Spec(image, repoDir, configDir, outDir, version, epoch string, uid, gid int) engine.RunSpec`;
  - the command `distro-build image`, writing `work/image/nudanos-<version>-amd64.iso`.

- [ ] **Step 1: Port `build-iso` into `image/`**

```bash
M="$HOME/Documents/Claude/Projects/danOS Project/mirrors/build-iso.git"
cd /Volumes/nudanos/distro && mkdir -p image && git -C "$M" archive HEAD config auto LICENSE | tar -x -C image
cd image
git rm -rq --cached . 2>/dev/null || true
# hooks that moved into packages (Task 4) or are obsolete
rm config/hooks/live/01-interfaces.chroot config/hooks/live/14-iptables-legacy.chroot \
   config/hooks/live/96-os-release.chroot config/hooks/live/98-yangcheck.chroot \
   config/hooks/normal/0990-create-chroot-fs.chroot
# 07-apt used apt-key (removed in apt 3); the keyring is installed through config/archives instead
rm config/hooks/live/07-apt.chroot
# DANOS package lists are replaced by one list
rm config/package-lists/*
rm -r config/bootloaders/isolinux
```
Keep the remaining hooks with their DANOS copyright, adding `# SPDX-License-Identifier: GPL-2.0-only` after the shebang where missing: `00-manifest`, `00-mk_buildid`, `01-boot_live`, `02-live-vmlinuz`, `03-live-config` (both), `03-root_bash_completion`, `04-locale`, `05-event_tty`, `11-busybox`, `13-sources_list`, `15-mandb`, `95-build.txt`, `97-generate_deb-versions`, `99-adjust-chroot-gids`, `99-cleanup`. Read each one and remove any reference to DANOS infrastructure (S3 URLs, `danos-` package names); note each such edit in the commit message.

- [ ] **Step 2: Write the live-build config**

`image/auto/config` (replaces DANOS's OBS-specific script):
```sh
#!/bin/sh
# NuDanOS live image (live-build). Ported from DANOS build-iso.
# SPDX-License-Identifier: GPL-2.0-only
set -e
lb config noauto \
    --mode debian --distribution trixie \
    --archive-areas "main non-free-firmware" \
    --binary-images iso-hybrid \
    --bootloaders "grub-pc grub-efi" \
    --debian-installer none --apt-recommends false \
    --linux-packages linux-image --linux-flavours amd64 \
    --iso-application NuDanOS --iso-publisher "NuDanOS https://github.com/nudanos" \
    --iso-volume "NuDanOS ${NUDANOS_VERSION}" \
    --bootappend-live "boot=live components console=tty0 console=ttyS0,115200" \
    --firmware-binary false --firmware-chroot false --source false \
    --mirror-bootstrap http://deb.debian.org/debian \
    --mirror-chroot http://deb.debian.org/debian \
    --mirror-binary http://deb.debian.org/debian \
    "${@}"
```
`image/config/package-lists/nudanos.list.chroot`:
```
nudanos-router
live-boot
live-config
live-config-systemd
linux-image-amd64
```
`image/config/archives/nudanos.list.chroot` (the repo is served on 127.0.0.1 by `make-image`):
```
deb http://127.0.0.1:8080/ trixie main
```
`image/config/archives/backports.list.chroot`:
```
deb http://deb.debian.org/debian trixie-backports main non-free-firmware
```
`image/config/archives/backports.pref.chroot`:
```
Package: linux-image-* linux-headers-* firmware-*
Pin: release n=trixie-backports
Pin-Priority: 900
```
`image/config/bootloaders/grub-pc/grub.cfg` and `grub-efi/grub.cfg` both enable the serial console so the boot test can drive the ISO:
```
serial --unit=0 --speed=115200
terminal_input console serial
terminal_output gfxterm serial
set default=0
set timeout=5
menuentry "NuDanOS (live)" {
	linux @KERNEL_LIVE@ @APPEND_LIVE@
	initrd @INITRD_LIVE@
}
```
`builder/make-image.sh`:
```bash
#!/bin/bash
# Build the NuDanOS ISO with live-build. /repo is the signed NuDanOS repo
# (read-only), /image the live-build config (read-only), /out the output.
# Env: NUDANOS_VERSION, SOURCE_DATE_EPOCH, HOST_UID, HOST_GID
set -euo pipefail
: "${NUDANOS_VERSION:?}" "${SOURCE_DATE_EPOCH:?}" "${HOST_UID:=0}" "${HOST_GID:=0}"
export SOURCE_DATE_EPOCH
apt-get update --error-on=any -qq -o Acquire::Retries=3
apt-get install -y -qq --no-install-recommends live-build python3 >/dev/null
# Serve the repo to the image chroot (it shares this network namespace).
( cd /repo && exec python3 -m http.server 8080 --bind 127.0.0.1 >/dev/null 2>&1 ) &
cp -a /image /tmp/lb && cd /tmp/lb
cp /repo/nudanos-archive-keyring.asc config/archives/nudanos.key.chroot
NUDANOS_VERSION="$NUDANOS_VERSION" lb config
lb build
iso=$(ls -1 /tmp/lb/*.hybrid.iso | head -1)
cp "$iso" "/out/nudanos-${NUDANOS_VERSION}-amd64.iso"
chown "$HOST_UID:$HOST_GID" "/out/nudanos-${NUDANOS_VERSION}-amd64.iso"
echo "image: /out/nudanos-${NUDANOS_VERSION}-amd64.iso"
```
`builder/Dockerfile`, after the `make-repo` lines:
```dockerfile
COPY make-image.sh /usr/local/bin/make-image
RUN chmod 0755 /usr/local/bin/make-image
```
(This changes the builder hash, so the next build rebuilds every package once.)

- [ ] **Step 3: Failing spec test**

`internal/imagebuild/imagebuild_test.go`:
```go
package imagebuild

import "testing"

func TestSpecIsPrivilegedWithReadOnlyInputs(t *testing.T) {
	s := Spec("nudanos/builder:trixie", "/w/repo", "/d/image", "/w/image", "1.0~20261001", "1790000000", 501, 20)
	if !s.Privileged {
		t.Error("live-build needs a privileged container")
	}
	ro := map[string]bool{}
	for _, m := range s.Mounts {
		ro[m.Container] = m.ReadOnly
	}
	if !ro["/repo"] || !ro["/image"] || ro["/out"] {
		t.Errorf("mounts = %+v (want /repo and /image read-only, /out writable)", s.Mounts)
	}
	if s.Env["NUDANOS_VERSION"] != "1.0~20261001" || s.Env["SOURCE_DATE_EPOCH"] != "1790000000" {
		t.Errorf("env = %v", s.Env)
	}
}
```
Run: `go test ./internal/imagebuild/`. Expected: FAIL (no package).

- [ ] **Step 4: Implement `imagebuild.Spec`**

```go
// Package imagebuild runs live-build to produce the NuDanOS ISO (spec 7.1 image).
package imagebuild

import (
	"strconv"

	"github.com/nudanos/distro/internal/engine"
)

// Spec builds the ISO from the signed repo in repoDir with the live-build
// config in configDir, writing nudanos-<version>-amd64.iso to outDir.
func Spec(image, repoDir, configDir, outDir, version, epoch string, uid, gid int) engine.RunSpec {
	return engine.RunSpec{Image: image, Privileged: true,
		Mounts: []engine.Mount{
			{Host: repoDir, Container: "/repo", ReadOnly: true},
			{Host: configDir, Container: "/image", ReadOnly: true},
			{Host: outDir, Container: "/out"},
		},
		Env: map[string]string{"NUDANOS_VERSION": version, "SOURCE_DATE_EPOCH": epoch,
			"HOST_UID": strconv.Itoa(uid), "HOST_GID": strconv.Itoa(gid)},
		Cmd: []string{"/usr/local/bin/make-image"}}
}
```
Run: `go test ./internal/imagebuild/`. Expected: PASS.

- [ ] **Step 5: Wire `image` into `distro-build`**

In `run()`'s `switch`:
```go
	case "image":
		return a.buildImage(ctx)
```
```go
// buildImage builds the ISO from work/repo. Its version and timestamp come from
// the distro commit, so the same commit builds the same image name.
func (a *app) buildImage(ctx context.Context) error {
	repo := filepath.Join(a.work, "repo")
	if _, err := os.Stat(filepath.Join(repo, "dists", "trixie", "InRelease")); err != nil {
		return fmt.Errorf("image: no signed repo in %s; run 'distro-build repo' first", repo)
	}
	if _, err := a.builderSalt(ctx); err != nil {
		return err
	}
	dir := filepath.Dir(a.manifest)
	out, err := exec.CommandContext(ctx, "git", "-C", dir, "log", "-1", "--format=%ct").Output()
	if err != nil {
		return fmt.Errorf("image: reading the commit time: %w", err)
	}
	epoch := strings.TrimSpace(string(out))
	sec, err := strconv.ParseInt(epoch, 10, 64)
	if err != nil {
		return fmt.Errorf("image: commit time %q: %w", epoch, err)
	}
	version := "1.0~" + time.Unix(sec, 0).UTC().Format("20060102")
	config, err := filepath.Abs(filepath.Join(dir, "image"))
	if err != nil {
		return err
	}
	outDir := filepath.Join(a.work, "image")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	uid, gid := a.eng.OwnerIDs(os.Getuid(), os.Getgid())
	return a.eng.Run(ctx, imagebuild.Spec(a.image, repo, config, outDir, version, epoch, uid, gid), os.Stderr, os.Stderr)
}
```
Run: `go vet ./... && go test ./...`. Expected: PASS.

- [ ] **Step 6: Build an ISO**

```bash
cd /Volumes/nudanos/distro && go build -o distro-build ./cmd/distro-build
./distro-build -work /Volumes/nudanos/work builder
./distro-build -work /Volumes/nudanos/work -jobs 4 build 2>&1 | tail -3
./distro-build -work /Volumes/nudanos/work -key "$(cat keys/FINGERPRINT)" repo >/dev/null
./distro-build -work /Volumes/nudanos/work image 2>&1 | tail -5
ls -la /Volumes/nudanos/work/image/
```
Expected: `image: /out/nudanos-1.0~<date>-amd64.iso` and an ISO of 600 MB–1.5 GB. Fix live-build errors in `image/` (each fix noted in the ledger). Commit `image/`, `builder/`, `internal/imagebuild`, `cmd/distro-build` and push.

---

### Task 10: Layer 3: `distro-build test boot`

**Files:**
- Create: `internal/boottest/console.go`, `internal/boottest/console_test.go`, `cmd/boottest/main.go`, `cmd/boottest/steps.go`, `tester/Dockerfile`
- Modify: `cmd/distro-build/main.go` (replace the `testBoot` stub)

**Interfaces:**
- Produces:
  - `boottest.NewConsole(rw io.ReadWriter, transcript io.Writer) *Console`
  - `(*Console).Expect(re *regexp.Regexp, timeout time.Duration) (string, error)`
  - `(*Console).Send(line string) error`
  - `(*Console).Dialog(rules []Rule, done *regexp.Regexp, timeout time.Duration) error`
  - `(*Console).Tail(n int) string`
  - `boottest.Rule{Prompt *regexp.Regexp; Reply string}`
  - the `cmd/boottest` binary (flags `-iso -disk -kvm -firmware bios|efi -smoke -log FILE`; timeouts scale by 6 without KVM)
  - the image `nudanos/tester:trixie`

- [ ] **Step 1: Failing console tests (Review Focus 5)**

`internal/boottest/console_test.go`:
```go
package boottest

import (
	"bytes"
	"io"
	"net"
	"regexp"
	"strings"
	"testing"
	"time"
)

// pipe returns a Console whose "VM" side is the returned net.Conn.
func pipe(t *testing.T) (*Console, net.Conn, *bytes.Buffer) {
	t.Helper()
	a, b := net.Pipe()
	var tr bytes.Buffer
	t.Cleanup(func() { a.Close(); b.Close() })
	return NewConsole(a, &tr), b, &tr
}

func TestExpectMatchesAcrossChunksAndNoise(t *testing.T) {
	c, vm, _ := pipe(t)
	go func() {
		io.WriteString(vm, "[   12.3] random: crng init done\r\nvyat")
		time.Sleep(20 * time.Millisecond)
		io.WriteString(vm, "ta login: ")
	}()
	if _, err := c.Expect(regexp.MustCompile(`login: $`), time.Second); err != nil {
		t.Fatal(err)
	}
}

func TestExpectConsumesSoTheNextStartsAfterTheMatch(t *testing.T) {
	c, vm, _ := pipe(t)
	go io.WriteString(vm, "$ one\r\n$ two\r\n")
	if _, err := c.Expect(regexp.MustCompile(`one`), time.Second); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Expect(regexp.MustCompile(`one`), 100*time.Millisecond); err == nil {
		t.Fatal("matched consumed output twice")
	}
}

func TestExpectTimeoutShowsTheLastLines(t *testing.T) {
	c, vm, _ := pipe(t)
	go io.WriteString(vm, "line1\r\nKernel panic - not syncing\r\n")
	_, err := c.Expect(regexp.MustCompile(`login:`), 100*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "Kernel panic") || !strings.Contains(err.Error(), "login:") {
		t.Fatalf("err = %v, want the awaited pattern and the last console lines", err)
	}
}

func TestSendAppendsCarriageReturnAndTranscriptRecordsBoth(t *testing.T) {
	c, vm, tr := pipe(t)
	got := make(chan string, 1)
	go func() {
		b := make([]byte, 64)
		n, _ := vm.Read(b)
		got <- string(b[:n])
		io.WriteString(vm, "ok\r\n")
	}()
	if err := c.Send("show version"); err != nil {
		t.Fatal(err)
	}
	if g := <-got; g != "show version\r" {
		t.Errorf("sent %q", g)
	}
	c.Expect(regexp.MustCompile(`ok`), time.Second)
	if !strings.Contains(tr.String(), "ok") {
		t.Errorf("transcript = %q", tr.String())
	}
}

func TestDialogAnswersPromptsUntilDone(t *testing.T) {
	c, vm, _ := pipe(t)
	replies := make(chan string, 4)
	go func() {
		buf := make([]byte, 128)
		for _, p := range []string{"Continue? (Yes/No) [No]: ", "Enter password for user 'vyatta':"} {
			io.WriteString(vm, p)
			n, _ := vm.Read(buf)
			replies <- string(buf[:n])
		}
		io.WriteString(vm, "\r\nDone!\r\n")
	}()
	err := c.Dialog([]Rule{
		{Prompt: regexp.MustCompile(`Continue\? \(Yes/No\) \[No\]: $`), Reply: "Yes"},
		{Prompt: regexp.MustCompile(`Enter password for user '[^']+':$`), Reply: "s3cret"},
	}, regexp.MustCompile(`Done!`), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if a, b := <-replies, <-replies; a != "Yes\r" || b != "s3cret\r" {
		t.Errorf("replies %q %q", a, b)
	}
}
```
Run: `go test ./internal/boottest/`. Expected: FAIL (no package).

- [ ] **Step 2: Implement `internal/boottest/console.go`**

```go
// Package boottest drives a VM's serial console: wait for output, type
// commands, answer installer prompts (the layer 3 boot test, spec 8.3).
package boottest

import (
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Console reads everything the VM prints into a buffer and matches patterns
// against the part not yet consumed by an earlier Expect.
type Console struct {
	w       io.Writer
	tr      io.Writer
	mu      sync.Mutex
	buf     []byte
	pos     int
	err     error
	changed chan struct{}
}

// Rule answers one installer prompt.
type Rule struct {
	Prompt *regexp.Regexp
	Reply  string
}

// NewConsole starts reading rw; everything read and sent is copied to transcript.
func NewConsole(rw io.ReadWriter, transcript io.Writer) *Console {
	c := &Console{w: rw, tr: transcript, changed: make(chan struct{}, 1)}
	go c.read(rw)
	return c
}

func (c *Console) read(r io.Reader) {
	b := make([]byte, 4096)
	for {
		n, err := r.Read(b)
		c.mu.Lock()
		if n > 0 {
			c.buf = append(c.buf, b[:n]...)
			c.tr.Write(b[:n])
		}
		if err != nil {
			c.err = err
		}
		c.mu.Unlock()
		select {
		case c.changed <- struct{}{}:
		default:
		}
		if err != nil {
			return
		}
	}
}

// Expect waits until re matches the unconsumed output, consumes through the
// match and returns it. On timeout the error names re and shows the last lines.
func (c *Console) Expect(re *regexp.Regexp, timeout time.Duration) (string, error) {
	_, m, err := c.first([]*regexp.Regexp{re}, timeout)
	return m, err
}

// first waits for the earliest match among res and returns its index.
func (c *Console) first(res []*regexp.Regexp, timeout time.Duration) (int, string, error) {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	for {
		c.mu.Lock()
		best, bestStart, bestEnd := -1, 0, 0
		for i, re := range res {
			if loc := re.FindIndex(c.buf[c.pos:]); loc != nil && (best < 0 || loc[0] < bestStart) {
				best, bestStart, bestEnd = i, loc[0], loc[1]
			}
		}
		if best >= 0 {
			m := string(c.buf[c.pos+bestStart : c.pos+bestEnd])
			c.pos += bestEnd
			c.mu.Unlock()
			return best, m, nil
		}
		err := c.err
		c.mu.Unlock()
		if err != nil {
			return -1, "", fmt.Errorf("console closed while waiting for %v: %v\nlast output:\n%s", res, err, c.Tail(15))
		}
		select {
		case <-c.changed:
		case <-deadline.C:
			return -1, "", fmt.Errorf("timed out after %s waiting for %v\nlast output:\n%s", timeout, res, c.Tail(15))
		}
	}
}

// Send types line followed by a carriage return.
func (c *Console) Send(line string) error {
	c.mu.Lock()
	c.tr.Write([]byte("<<" + line + ">>\n"))
	c.mu.Unlock()
	_, err := io.WriteString(c.w, line+"\r")
	return err
}

// Dialog answers prompts by rules until done matches. Each wait has timeout.
func (c *Console) Dialog(rules []Rule, done *regexp.Regexp, timeout time.Duration) error {
	res := []*regexp.Regexp{done}
	for _, r := range rules {
		res = append(res, r.Prompt)
	}
	for {
		i, _, err := c.first(res, timeout)
		if err != nil {
			return err
		}
		if i == 0 {
			return nil
		}
		if err := c.Send(rules[i-1].Reply); err != nil {
			return err
		}
	}
}

// Tail returns the last n lines of everything the VM printed.
func (c *Console) Tail(n int) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	lines := strings.Split(strings.ReplaceAll(string(c.buf), "\r", ""), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
```
Run: `go test -race ./internal/boottest/`. Expected: PASS (5 tests).

- [ ] **Step 3: The layer 3 script (`cmd/boottest/steps.go`)**

```go
package main

import (
	"regexp"
	"time"

	"github.com/nudanos/distro/internal/boottest"
)

var (
	loginPrompt = regexp.MustCompile(`login: $`)
	passPrompt  = regexp.MustCompile(`Password: $`)
	opPrompt    = regexp.MustCompile(`:~\$ $`)
	cfgPrompt   = regexp.MustCompile(`# $`)
)

// testPassword is the administrator password the test sets on install; it is
// not a secret (the VM is thrown away) and must differ from "vyatta".
const testPassword = "NuDanOS-test-1"

// liveSteps runs on the ISO: log in, configure, commit, save. (spec addendum §1)
func liveSteps(c *boottest.Console, t func(time.Duration) time.Duration) error {
	steps := []func() error{
		func() error { _, err := c.Expect(loginPrompt, t(20*time.Minute)); return err },
		func() error { return c.Send("vyatta") },
		func() error { _, err := c.Expect(passPrompt, t(time.Minute)); return err },
		func() error { return c.Send("vyatta") },
		func() error { _, err := c.Expect(opPrompt, t(2*time.Minute)); return err },
		func() error { return c.Send("show version") },
		func() error { _, err := c.Expect(regexp.MustCompile(`(?m)^Base:\s+Debian GNU/Linux 13`), t(time.Minute)); return err },
		func() error { _, err := c.Expect(regexp.MustCompile(`(?m)^FRR:\s+10\.7`), t(time.Minute)); return err },
		func() error { _, err := c.Expect(opPrompt, t(time.Minute)); return err },
		func() error { return c.Send("configure") },
		func() error { _, err := c.Expect(cfgPrompt, t(time.Minute)); return err },
		// Review Focus 3: a DPDK-only setting is refused with its reason.
		func() error { return c.Send("set interfaces dataplane dp0s3 cpu-affinity 1") },
		func() error { return c.Send("commit") },
		func() error {
			_, err := c.Expect(regexp.MustCompile(`requires the DPDK dataplane`), t(2*time.Minute))
			return err
		},
		func() error { return c.Send("discard") },
		func() error { _, err := c.Expect(cfgPrompt, t(time.Minute)); return err },
		func() error { return c.Send("set interfaces dataplane dp0s3 address 192.0.2.1/24") },
		func() error { return c.Send("set system host-name nudanos-test") },
		func() error { return c.Send("set system login user tester authentication plaintext-password " + testPassword) },
		func() error { return c.Send("commit") },
		func() error { _, err := c.Expect(cfgPrompt, t(5*time.Minute)); return err },
		func() error { return c.Send("save") },
		func() error { _, err := c.Expect(regexp.MustCompile(`Saving configuration|Done`), t(2*time.Minute)); return err },
		func() error { return c.Send("exit") },
		func() error { _, err := c.Expect(opPrompt, t(time.Minute)); return err },
	}
	return run(steps)
}

// installSteps installs to the virtual disk, refusing the default password.
func installSteps(c *boottest.Console, t func(time.Duration) time.Duration) error {
	if err := c.Send("install image"); err != nil {
		return err
	}
	sawRefusal := false
	refusal := regexp.MustCompile(`published live-ISO default`)
	pw := 0
	// Only the disk-overwrite confirmation defaults to No; every other
	// question (grub password, reduced grub layout, console, partition
	// sizes, which config to copy) keeps its default. A blanket "Yes" would
	// also accept "set up a grub password?".
	rules := []boottest.Rule{
		{Prompt: regexp.MustCompile(`Continue\? \(Yes/No\) \[No\]: ?$`), Reply: "Yes"},
		{Prompt: regexp.MustCompile(`Enter username for administrator account: $`), Reply: "vyatta"},
		{Prompt: regexp.MustCompile(`\[[^\]]*\]:? ?$`), Reply: ""}, // accept the default
	}
	// Password prompts: first answer the published default (must be refused),
	// then the test password twice.
	pwPrompt := regexp.MustCompile(`(Enter|Retype) password for user '[^']+':$`)
	done := regexp.MustCompile(`Done!|installation (is )?complete`)
	for {
		i, _, err := firstOf(c, append([]*regexp.Regexp{done, refusal, pwPrompt}, prompts(rules)...), t(30*time.Minute))
		if err != nil {
			return err
		}
		switch {
		case i == 0:
			if !sawRefusal {
				return errorf("install image accepted the default password 'vyatta'")
			}
			return nil
		case i == 1:
			sawRefusal = true
		case i == 2:
			reply := testPassword
			if pw == 0 {
				reply = "vyatta"
			}
			pw++
			if err := c.Send(reply); err != nil {
				return err
			}
		default:
			if err := c.Send(rules[i-3].Reply); err != nil {
				return err
			}
		}
	}
}

// diskSteps runs after booting the installed system: the config survived.
func diskSteps(c *boottest.Console, t func(time.Duration) time.Duration) error {
	steps := []func() error{
		func() error {
			_, err := c.Expect(regexp.MustCompile(`nudanos-test login: $`), t(20*time.Minute))
			return err
		},
		func() error { return c.Send("tester") },
		func() error { _, err := c.Expect(passPrompt, t(time.Minute)); return err },
		func() error { return c.Send(testPassword) },
		func() error { _, err := c.Expect(opPrompt, t(2*time.Minute)); return err },
		func() error { return c.Send("show interfaces") },
		func() error { _, err := c.Expect(regexp.MustCompile(`dp0s3\s+192\.0\.2\.1/24`), t(time.Minute)); return err },
		func() error { return c.Send("show version") },
		func() error { _, err := c.Expect(regexp.MustCompile(`(?m)^Kernel:\s+\S+`), t(time.Minute)); return err },
	}
	return run(steps)
}

// halt powers the VM off cleanly, so QEMU flushes the qcow2 disk before it
// exits (killing QEMU can lose cached writes from install image).
func halt(c *boottest.Console, t func(time.Duration) time.Duration) error {
	if err := c.Send("poweroff"); err != nil {
		return err
	}
	if _, err := c.Expect(regexp.MustCompile(`\[confirm\]`), t(time.Minute)); err != nil {
		return err
	}
	return c.Send("y")
}
```
In the same file, the helpers:
```go
func run(steps []func() error) error {
	for i, s := range steps {
		if err := s(); err != nil {
			return errorf("step %d: %v", i+1, err)
		}
	}
	return nil
}

func prompts(rules []boottest.Rule) []*regexp.Regexp {
	out := make([]*regexp.Regexp, len(rules))
	for i, r := range rules {
		out[i] = r.Prompt
	}
	return out
}
```
`firstOf` needs `Console.first`, which is unexported. Add an exported wrapper in `console.go`, then define `firstOf` as a one-line call to it:
```go
// First waits for the earliest match among res, consumes it and returns its index.
func (c *Console) First(res []*regexp.Regexp, timeout time.Duration) (int, string, error) {
	return c.first(res, timeout)
}
```
```go
func firstOf(c *boottest.Console, res []*regexp.Regexp, d time.Duration) (int, string, error) {
	return c.First(res, d)
}
```
The installer's prompt texts (`vyatta-install-image.functions`) are matched by the rules above. The live ISO has no saved config, so the "save config from" questions do not appear. At Task 12's first run, compare the transcript with these patterns and adjust any pattern the real installer phrases differently, recording the change in the ledger.

- [ ] **Step 4: The QEMU runner (`cmd/boottest/main.go`)**

```go
// Command boottest boots the NuDanOS ISO in QEMU and runs the layer 3 test
// over the serial console (spec 8.3). It runs inside nudanos/tester:trixie.
package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/nudanos/distro/internal/boottest"
)

func errorf(format string, a ...any) error { return fmt.Errorf(format, a...) }

func main() {
	iso := flag.String("iso", "", "ISO to boot")
	disk := flag.String("disk", "/work/disk.qcow2", "virtual disk to install to")
	kvm := flag.Bool("kvm", false, "use KVM (otherwise TCG emulation)")
	firmware := flag.String("firmware", "bios", "bios or efi")
	smoke := flag.Bool("smoke", false, "only boot to the login prompt (UEFI check)")
	logPath := flag.String("log", "/work/serial.log", "serial transcript")
	flag.Parse()
	scale := 1.0
	if !*kvm {
		scale = 6 // TCG is 5-20x slower; timeouts scale with it
	}
	t := func(d time.Duration) time.Duration { return time.Duration(float64(d) * scale) }
	if err := runAll(*iso, *disk, *kvm, *firmware, *smoke, *logPath, t); err != nil {
		fmt.Fprintln(os.Stderr, "boottest: FAIL:", err)
		os.Exit(1)
	}
	fmt.Println("boottest: OK")
}

func runAll(iso, disk string, kvm bool, firmware string, smoke bool, logPath string, t func(time.Duration) time.Duration) error {
	log, err := os.Create(logPath)
	if err != nil {
		return err
	}
	defer log.Close()
	if err := exec.Command("qemu-img", "create", "-q", "-f", "qcow2", disk, "8G").Run(); err != nil {
		return fmt.Errorf("qemu-img: %w", err)
	}
	// Phase 1: the ISO.
	c, stop, err := boot(iso, disk, kvm, firmware, log)
	if err != nil {
		return err
	}
	if smoke {
		_, err := c.Expect(loginPrompt, t(20*time.Minute))
		stop(0)
		return err
	}
	if err := liveSteps(c, t); err != nil {
		stop(0)
		return fmt.Errorf("live: %w", err)
	}
	if err := installSteps(c, t); err != nil {
		stop(0)
		return fmt.Errorf("install image: %w", err)
	}
	if err := halt(c, t); err != nil {
		stop(0)
		return fmt.Errorf("poweroff after install: %w", err)
	}
	stop(t(5 * time.Minute)) // let QEMU exit by itself and flush the disk
	// Phase 2: the installed disk only.
	c, stop, err = boot("", disk, kvm, firmware, log)
	if err != nil {
		return err
	}
	if err := diskSteps(c, t); err != nil {
		stop(0)
		return fmt.Errorf("installed: %w", err)
	}
	halt(c, t)
	stop(t(5 * time.Minute))
	return nil
}

// boot starts QEMU with its serial console on a unix socket and connects to it.
// stop(grace) waits up to grace for QEMU to exit on its own, then kills it; it
// is always called, so a hung boot never outlives the test.
func boot(iso, disk string, kvm bool, firmware string, log *os.File) (*boottest.Console, func(time.Duration), error) {
	sock := filepath.Join(os.TempDir(), fmt.Sprintf("serial-%d.sock", time.Now().UnixNano()))
	args := []string{"-machine", "pc", "-m", "2048", "-smp", "2", "-display", "none",
		"-serial", "unix:" + sock + ",server=on,wait=on",
		"-drive", "file=" + disk + ",if=virtio,format=qcow2",
		"-netdev", "user,id=n0", "-device", "virtio-net-pci,netdev=n0,addr=03"}
	if kvm {
		args = append(args, "-enable-kvm", "-cpu", "host")
	}
	if firmware == "efi" {
		args = append(args, "-bios", "/usr/share/ovmf/OVMF.fd")
	}
	if iso != "" {
		args = append(args, "-cdrom", iso, "-boot", "d")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, "qemu-system-x86_64", args...)
	cmd.Stdout, cmd.Stderr = log, log
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, nil, fmt.Errorf("qemu: %w", err)
	}
	exited := make(chan struct{})
	go func() { cmd.Wait(); close(exited) }()
	kill := func(grace time.Duration) {
		select {
		case <-exited:
		case <-time.After(grace):
			cancel()
			<-exited
		}
		cancel()
	}
	var conn net.Conn
	var err error
	for i := 0; i < 100; i++ {
		if conn, err = net.Dial("unix", sock); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err != nil {
		kill(0)
		return nil, nil, fmt.Errorf("serial socket: %w", err)
	}
	return boottest.NewConsole(conn, log), func(grace time.Duration) { kill(grace); conn.Close() }, nil
}
```
Run: `go vet ./... && go build ./cmd/boottest`. Expected: no output.

- [ ] **Step 5: The tester image and `distro-build test boot`**

`tester/Dockerfile`:
```dockerfile
# NuDanOS boot tester: QEMU with BIOS and UEFI firmware.
FROM debian:trixie
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y --no-install-recommends \
      qemu-system-x86 qemu-utils ovmf ca-certificates \
 && rm -rf /var/lib/apt/lists/*
```
Replace the `testBoot` stub in `cmd/distro-build/main.go`:
```go
// testBoot cross-compiles cmd/boottest, builds the tester image and boots the
// newest ISO in work/image under QEMU (KVM when /dev/kvm exists).
func (a *app) testBoot(ctx context.Context) error {
	isos, _ := filepath.Glob(filepath.Join(a.work, "image", "nudanos-*-amd64.iso"))
	if len(isos) == 0 {
		return fmt.Errorf("test boot: no ISO in %s; run 'distro-build image' first", filepath.Join(a.work, "image"))
	}
	sort.Strings(isos)
	iso := isos[len(isos)-1]
	dir := filepath.Dir(a.manifest)
	bt := filepath.Join(a.work, "boottest")
	if err := os.MkdirAll(bt, 0o755); err != nil {
		return err
	}
	build := exec.CommandContext(ctx, "go", "build", "-o", filepath.Join(bt, "boottest"), "./cmd/boottest")
	build.Dir, build.Env = dir, append(os.Environ(), "CGO_ENABLED=0", "GOOS=linux", "GOARCH=amd64")
	build.Stdout, build.Stderr = os.Stderr, os.Stderr
	if err := build.Run(); err != nil {
		return fmt.Errorf("test boot: building boottest: %w", err)
	}
	if err := a.eng.BuildImage(ctx, filepath.Join(dir, "tester"), "nudanos/tester:trixie", nil, os.Stderr, os.Stderr); err != nil {
		return err
	}
	spec := engine.RunSpec{Image: "nudanos/tester:trixie",
		Mounts: []engine.Mount{
			{Host: filepath.Dir(iso), Container: "/iso", ReadOnly: true},
			{Host: bt, Container: "/work"},
		},
		Cmd: []string{"/work/boottest", "-iso", "/iso/" + filepath.Base(iso), "-firmware", a.firmware}}
	if a.smoke {
		spec.Cmd = append(spec.Cmd, "-smoke")
	}
	if _, err := os.Stat("/dev/kvm"); err == nil {
		spec.Devices = []string{"/dev/kvm"}
		spec.Cmd = append(spec.Cmd, "-kvm")
	}
	err := a.eng.Run(ctx, spec, os.Stdout, os.Stderr)
	fmt.Fprintf(os.Stderr, "serial transcript: %s\n", filepath.Join(bt, "serial.log"))
	return err
}
```
Add `firmware string` and `smoke bool` to `app`, with the flags `flag.StringVar(&a.firmware, "firmware", "bios", "test boot: bios or efi")` and `flag.BoolVar(&a.smoke, "smoke", false, "test boot: only boot to the login prompt")`. Check the signature of `engine.BuildImage` first (`grep -n "func (e Engine) BuildImage" internal/engine/engine.go`); if a nil label map is not accepted, pass `map[string]string{}`. Run: `go vet ./... && go test ./...`. Expected: PASS.

- [ ] **Step 6: Format and commit**

```bash
gofmt -w internal/boottest cmd/boottest cmd/distro-build && go vet ./... && go test -race ./...
git add internal/boottest cmd/boottest tester cmd/distro-build
git commit -s -m "test boot: layer 3 boot test over the serial console

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
git push origin main
```

---

### Task 11: Nightly: image → layer 2 → layer 3 → release

**Files:**
- Modify: `.github/workflows/nightly.yml`

**Interfaces:**
- Consumes: `distro-build image`, `test install`, `test boot`, `tools/router_deps.py --check`, `tools/yangcheck.sh`.

- [ ] **Step 1: Extend the `pilot` job and add `release`**

Rename `pilot` to `build` and replace its last two steps with:
```yaml
      - name: Build, sign, check the package set
        run: |
          go build -o distro-build ./cmd/distro-build
          ./distro-build -work "$RUNNER_TEMP/work" builder
          ./distro-build -work "$RUNNER_TEMP/work" -jobs 2 build
          WORK="$RUNNER_TEMP/work" KEY="$(cat keys/FINGERPRINT)" tests/integration/pilot.sh
          tools/yangcheck.sh "$RUNNER_TEMP/work"
          python3 tools/router_deps.py --check "$RUNNER_TEMP/work/src/nudanos-router/debian/control" \
            "$RUNNER_TEMP/work/repo/dists/trixie/main/binary-amd64/Packages" docs/kernel-forwarding.md
      - name: Image
        run: ./distro-build -work "$RUNNER_TEMP/work" image
      - name: Layer 2 (install, remove, purge)
        run: ./distro-build -work "$RUNNER_TEMP/work" test install
      - name: Enable KVM
        run: |
          echo 'KERNEL=="kvm", GROUP="kvm", MODE="0666", OPTIONS+="static_node=kvm"' | sudo tee /etc/udev/rules.d/99-kvm.rules
          sudo udevadm control --reload-rules && sudo udevadm trigger --name-match=kvm
      - name: Layer 3 (boot, configure, install, reboot)
        run: ./distro-build -work "$RUNNER_TEMP/work" test boot
      - name: UEFI smoke boot
        run: ./distro-build -work "$RUNNER_TEMP/work" -firmware efi -smoke test boot
      - if: always()
        uses: actions/upload-artifact@v7
        with:
          name: boot-transcript
          path: ${{ runner.temp }}/work/boottest/serial.log
      - uses: actions/upload-artifact@v7
        with:
          name: iso
          path: ${{ runner.temp }}/work/image/*.iso
          retention-days: 7
```
Add a second job:
```yaml
  release:
    needs: build
    runs-on: ubuntu-24.04
    environment: release
    permissions:
      contents: write
    steps:
      - uses: actions/download-artifact@v7
        with:
          name: iso
      - name: Publish the ISO
        env:
          GH_TOKEN: ${{ github.token }}
        run: |
          iso=$(ls nudanos-*-amd64.iso)
          tag="nightly-$(date -u +%Y%m%d)"
          sha256sum "$iso" > "$iso.sha256"
          gh release create "$tag" "$iso" "$iso.sha256" -R "$GITHUB_REPOSITORY" \
            --prerelease --title "NuDanOS nightly $(date -u +%Y-%m-%d)" \
            --notes "Built from ${GITHUB_SHA}. Passed layers 1-3. Milestone 1: no firewall, NAT or QoS; not for internet-facing use."
```
The job-level `permissions: contents: write` overrides the workflow's `contents: read` for `release` only. If a cold build plus image and boot exceeds the 6-hour limit, split `build` at the image step into `build` (uploading `work/out` and `work/repo` as an artifact) and `image` (downloading them). Record the split in the ledger if made.

- [ ] **Step 2: Validate and commit**

```bash
cd /Volumes/nudanos/distro
docker run --rm -v "$PWD":/r -w /r rhysd/actionlint:latest .github/workflows/nightly.yml
git add .github/workflows/nightly.yml && git commit -s -m "nightly: image, layers 2 and 3, UEFI smoke, release

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>" && git push origin main
```
Expected: actionlint prints nothing.

---

### Task 12: End to end: run every layer, fix what breaks

**Files:** whatever the failures point at, each fix on its own repo, with a ledger line.

- [ ] **Step 1: Local layer 3 under TCG**

```bash
cd /Volumes/nudanos/distro && go build -o distro-build ./cmd/distro-build
./distro-build -work /Volumes/nudanos/work test boot 2>&1 | tail -20
```
Expected: `boottest: OK` (TCG on the Mac: expect 1–2 hours). On failure, read `work/boottest/serial.log`, fix the cause (package, image config, or a prompt pattern in `steps.go`), rebuild what changed, and rerun. Each fix gets a `Ruling:` line when it departs from this plan.

- [ ] **Step 2: Nightly on GitHub**

```bash
gh workflow run nightly -R nudanos/distro --ref main
```
Wait for the run, tolerating network drops:
```bash
id=$(gh run list -R nudanos/distro -w nightly -L 1 --json databaseId -q '.[0].databaseId')
until s=$(gh run view "$id" -R nudanos/distro --json status,conclusion -q '"\(.status) \(.conclusion)"' 2>/dev/null) && [ "${s%% *}" = completed ]; do sleep 60; done; echo "$s"
gh release list -R nudanos/distro -L 1
```
Expected: `completed success`, and a `nightly-<date>` pre-release with the ISO and its `.sha256`.

- [ ] **Step 3: Record and finish**

Update base spec §9's `1.0: Boots` row to say the ISO boots and installs (date), commit `spec: plan 3 boots`, and push. Then run the executing-plans final review.
