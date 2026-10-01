# NuDanOS: DANOS revival on Debian 13 (design)

Date: 2026-09-28
Status: draft for review
Background: [`REVIEW.md`](../../../REVIEW.md) (code review, preservation status, Debian 13 gap analysis)

## 1. Goals

**NuDanOS** revives DANOS, AT&T's open-source router OS (abandoned 2021), as:

- **B. A public community project** that others can install, use and contribute to.
- **C. A source of reusable parts.** The YANG config system (configd, VCI) and
  later the DPDK dataplane should be usable on their own.

The first target is DANOS on **Debian 13 "trixie"**, built entirely from source, with
networking components at their **latest upstream releases**.

### Non-goals (for this design)

- White-box switch support (Broadcom OpenNSL is a missing binary blob)
- Migrating the ~700 legacy Perl/script hooks to VCI (a later stage, see §9)
- Secure Boot signing (§7.5)
- Targets other than Debian 13 amd64 (§9)
- Renaming the `vyatta-*` packages. They keep their names so 2105 configs and
  scripts keep working

### Success criteria

1. A Debian 13 based ISO, built from source by public CI, boots in a VM. On it,
   a user can log in, configure interfaces, commit and save config, and run
   BGP/OSPF/VRRP.
2. Each rewrite (Kea, libnetconf2, strongswan 6, Go modules) passes its scenario test.
3. Every DANOS 2105 reference config loads on the new build without edits.
4. `install.sh` on a stock Debian 13 VM produces a system that passes the same
   smoke test (milestone 1.5).

## 2. Decisions taken

| Topic | Decision |
|---|---|
| Milestone 1 scope | Management plane, FRR, kernel forwarding. DPDK dataplane is milestone 2 |
| Repo structure | One repo per package (forks of all 189, full history) plus a `distro` repo |
| Build hosts | GitHub Actions CI. Local builds in Docker Desktop or a Debian 13 VM, same container |
| Script install | Designed for now, built as milestone 1.5 |
| Versions | Tier 1 (networking) = latest upstream. Tier 2 (base OS) = Debian 13 plus security updates |
| Rewrites | Kea, libnetconf2/libyang, strongswan 6/VICI and Go modules are in milestone 1 (checkpoint 1.1) |
| Firewall/NAT/QoS | Milestone 2, with the dataplane (they exist only in NPF) |

## 3. Architecture

### 3.1 Repositories

- A new GitHub org with login **`nudanos`** (lowercase) and display name **NuDanOS**.
  Keep the login lowercase: Go module paths are case-sensitive, and a mixed-case path
  (`github.com/NuDanOS/...`) gets escaped by the module proxy and is easy to mistype.
- All 189 `github.com/danos` repos are pushed from the local `mirrors/` copies with
  every branch and tag. They are standalone repos, not GitHub forks, so they don't
  depend on `github.com/danos` surviving. Each README credits the original project.
- Porting happens on a new **`trixie`** branch in each repo. Original branches and
  tags are never rewritten.
- jsouthworth's `danos-buildpackage`, `danos-bootstrap` and `vci-dhcpv6-pd` are
  imported with history (the first two are merged into `distro`; see §7).
- Repos whose manifest policy is `debian` or `drop` are archived on GitHub.

### 3.2 The `distro` repo

| Path | Contents |
|---|---|
| `manifest.yaml` | Every package: kind, source, ref, tracking rule, milestone |
| `builder/Dockerfile` | `debian:trixie` build image |
| `cmd/distro-build/` | Build tool (Go) |
| `image/` | live-build config (from `build-iso`), updated for Debian 13 |
| `installer/install.sh` | Script installer (milestone 1.5) |
| `patches/<pkg>/` | DANOS patches kept after audit, for `upstream` entries |
| `tests/` | Boot tests, topologies, Robot suites, 2105 reference data |
| `.github/workflows/` | Shared package workflow, nightly, weekly update check |

### 3.3 Build flow

```
manifest.yaml
  → fetch (clone repos at refs; download upstream tags; mirror apt entries)
  → plan  (build order from debian/control Build-Depends)
  → build (fresh debian:trixie container per package, source + binary,
           earlier outputs served as a local apt repo)
  → repo  (signed apt repo with deb-src)
  → image (live-build ISO)
  → test  (QEMU)
```

## 4. Package policy

### 4.1 Rule

Use Debian 13's package by default. Carry a patch only if a milestone-1 DANOS
package needs the behaviour and the upstream release lacks it. Record every patch
audit (keep / upstreamed / dropped, with reason) in the manifest entry.

### 4.2 Version tiers

- **Tier 1: networking.** Track the latest upstream release (or latest LTS where
  marked). Built by us from upstream, reusing Debian's packaging from
  salsa.debian.org, unless upstream publishes Debian 13 packages.
- **Tier 2: base OS** (openssh, openssl, glibc, systemd, grub, shim, rsyslog, sudo,
  …). Debian 13 plus security updates. A trixie-backports version may be used where
  newer, since Debian still maintains it.

Versions at time of writing (checked 2026-09-28):

| Component | DANOS | Target | Source |
|---|---|---|---|
| FRR | 7.6 | 10.7.1 | deb.frrouting.org `trixie frr-stable` (`kind: apt`) |
| Kernel | 5.10.69 | 7.1.x | trixie-backports `linux-image-amd64`. Fallback: Debian 6.12 or self-built 6.18 LTS |
| Go | 1.15 | 1.26 | trixie-backports `golang-go` |
| strongswan | 5.9.0 | 6.1.0 | upstream |
| keepalived | 2.2.0 | 2.4.3 | upstream |
| libteam | 1.11 | 1.32 | upstream |
| net-snmp | 5.7.3 | 5.9.5.2 | upstream |
| ntp → ntpsec | 4.2.8p15 | NTPsec 1.2.5 | upstream |
| owamp + i2util | 4.2.1 | 5.2.6 | upstream (perfSONAR) |
| pam_tacplus | 1.6.1 | 1.7.0 | upstream |
| mstpd | 0.0.9 | latest tag | upstream |
| Kea (replaces ISC DHCP server) | – | 3.2.x | upstream |
| libyang / libnetconf2 | 1.0 / libnetconf 0.10 | latest mutually compatible releases (libyang 5.8.x / libnetconf2 4.4.x today) | upstream |
| DPDK (M2) | 20.11.3 | 25.11 LTS | upstream |
| nDPI (M2) | 3.4 | 6.0 | upstream |

### 4.3 Disposition of the 66 upstream forks

| Disposition | Forks |
|---|---|
| **Tier 1, build from upstream (audit DANOS patches)** | strongswan (20 DANOS patches), keepalived (18), libteam (10), net-snmp (5), ntp→ntpsec (3), owamp + i2util (28), pam_tacplus, mstpd (5), libyang (M1.1) |
| **Tier 1, upstream's own packages** | frr (27 DANOS commits audited against 10.7) |
| **Debian source plus kept patches** | netplug (12; upstream is dead, Debian carries the same release) |
| **Debian 13 package (fork archived)** | check, cloud-init (audit 5 patches), dh-golang, golang, golang-defaults, grub, shim, iperf, iputils (audit 1), jitterentropy, libpcap (audit 1), libvirt, linux-firmware (→ Debian firmware packages), makedumpfile, openssh, pygobject, ppp (audit 1), radvd (audit 1), rdma-core, rsyslog, smartmontools (audit 1), sssd (audit 5), valgrind, netkit-telnet (→ `telnetd`) |
| **Kernel** | linux-vyatta → Debian kernel. The 88 vendor commits are audited: already upstream / needed by M1 / dataplane-only. Anything needed is carried as a patch set on Debian's kernel packaging, not a kernel fork |
| **Go libraries** | golang-dbus, objtree, goczmq, mdlayher-*, josharian-native, x-sys, youmark-pkcs8, jsouthworth-*: built as today for M1.0; become vendored Go module dependencies in M1.1 and are retired as Debian packages |
| **Replaced by a rewrite (M1.1)** | isc-dhcp (server → Kea; client → dhcpcd; relay → Debian `isc-dhcp-relay` for now), libnetconf + pyang (→ libnetconf2) |
| **Kept as-is** | vyatta-bash (thin wrapper over Debian bash) |
| **Milestone 2** | dpdk, dpdk-kmods, ndpi, vermont, host-sflow, libzmq-libzmq3-perl, libzmq-constants-perl |
| **Deferred** | libre + repcpd (PCP service: upstream `re` moved from 1.1 to 4.x; not in M1) |
| **Dropped** | bcm-kbp-linux-modules, bcm-linux-bde-modules, ufispace-apollo-linux-modules, ufispace-bsp-utils, opennsl-binary, fluent-bit (unused), pytest-lazy-fixture (tests adapted instead), grub2-signed (until Secure Boot, §7.5) |

DANOS-authored repos with hardware-only purpose (libfal-opennsl, accton-hwdiag,
vyatta-hwdiag, vyatta-poe, ipmi/bmc packages) are dropped from M1 too.

### 4.4 Untangling the dataplane for milestone 1

`no-dataplane` is only a boot flag. 46 milestone-1 packages depend on the dataplane
stack at runtime (`m1-scope.json`). Changes:

1. **`vyatta-dataplane` gains a Debian build profile `pkg.vyatta-dataplane.protobuf-only`**
   and a matching meson option. It builds only the protobuf packages
   (`libvyatta-dataplane-proto*`, `libvyatta-dataplane-proto-support`,
   `golang-github-danos-vyatta-dataplane-protobuf-dev`, Perl/Python bindings) with no
   DPDK dependency.
2. **Relax hard dependencies.** `vyatta-system`, `vyatta-system-network-v1-yang`,
   `vyatta-hotplug` and `vyatta-interfaces-bonding` change
   `Depends: vyatta-dataplane` to a virtual package `vyatta-forwarding`, provided
   by `vyatta-dataplane` (M2) and by a new `vyatta-kernel-forwarding` package (M1).
   The latter sets the `no-dataplane` behaviour permanently rather than via a kernel
   command-line flag. `vyatta-service-bridge` depends on `vplane-config-npf`. Check
   what it uses: relax the dependency if only bridge-firewall features need it,
   otherwise keep kernel bridging on the system-interfaces models and move the
   package to M2.
3. **Firewall, NAT, PBR, QoS, session and ALG models** (the 34 `*-v1-yang` packages
   that depend on `vplane-config-npf`/`vplane-config-qos`) leave the M1 package set.

Milestone 1 therefore has **no firewall or NAT** and is not for internet-facing use.

### 4.5 Enablers for the script install

- A meta-package, **`nudanos-router`**, whose
  dependencies are the package set. The ISO installs it too, so the ISO and the
  script share one list.
- System setup moves from ISO hooks into package maintainer scripts:
  - the user-isolation sandbox (`0990-create-chroot-fs`) → `cli-sandbox` postinst
  - iptables/ebtables legacy alternatives → `vyatta-kernel-forwarding` postinst
  - `os-release` branding → `base-files-vyatta` postinst
  - the loopback entry in `/etc/network/interfaces` → `vyatta-cfg-system` postinst
  - YANG validation (`98-yangcheck`) → a build-time CI check

  Only live-boot/ISO-specific hooks stay in `image/`.
- The apt repo is signed and served over HTTPS (§7.4).

## 5. Rewrites (checkpoint 1.1)

Each lands as its own PR series and must pass its scenario (§8.4) before the next starts.

### 5.1 DHCP → Kea 3.2 / dhcpcd

Scope: `vyatta-service-dhcp` (~7.3k lines).

- **Server (v4/v6):** generate Kea JSON config from the unchanged YANG model.
  Reload through Kea's control socket. Lease show/clear use Kea lease commands.
- **Client:** dhcpcd (Debian 13's default ifupdown client) replaces dhclient.
  Update the `vyatta-service-dhcp-client@` units and hooks.
- **Relay:** Kea has no relay agent. Keep Debian 13's `isc-dhcp-relay` (4.4.3,
  Debian-maintained), and track the successor choice as an open item (§11).
- The YANG model does not change (compatibility promise).

### 5.2 NETCONF → libnetconf2 + libyang (latest compatible pair)

Scope: `netconfd` (782 lines of C, 59 libnetconf calls). Port to the libnetconf2
server API with SSH transport. Datastore operations still go through configd.
Retires the libnetconf 0.10 and pyang forks.

### 5.3 VPN → strongswan 6.1, VICI only

Scope: `vyatta-security-vpn`. ~20 files still use `ipsec.conf`/`ipsec.secrets`/
`stroke`/the `ipsec` starter, and ~26 already use VICI. Move everything to VICI
(`swanctl`-style config pushed over the VICI socket). Remove
`vyatta-strongswan-starter.service`. Carry only DANOS strongswan patches that
survive audit.

### 5.4 Go → modules, Go 1.26

Scope: 33 Go repos. Add `go.mod` (module path `github.com/nudanos/<repo>`) and
`go mod vendor`, and build with `-mod=vendor` inside `debian/rules`. Rewrite
`github.com/danos/...` imports to the new org. Pin `jsouthworth.net/go/*` through
`go.mod` replace directives pointing at the imported copies. Fix vet failures that
newer Go surfaces in tests. Remove the `brocade.com/vyatta/cmdclient` import from
`opd/cmd/opstress`.

## 6. Debian 13 port checklist (every DANOS package, `trixie` branch)

- `debhelper-compat (= 13)`, current `Standards-Version`, `Rules-Requires-Root: no`
- Drop `dh-systemd` (merged into debhelper) and `dh-signobs`
- `pylint3` → `pylint`. `python3-pytest-pep8` (removed from Debian) → run
  `pycodestyle`/`flake8` directly from the test target
- `bvnos-linux-libc-dev(-vyatta)` → `linux-libc-dev` (fail loudly if a DANOS-only
  header is actually used, then decide per case)
- Maintainer and Vcs-* fields → the new org
- lintian clean against `lintian-profile-vyatta`, updated for trixie
- A new `debian/changelog` entry per package

## 7. Build tooling

### 7.1 `distro-build`

Go. Merges `danos-bootstrap` (ordering, bulk build) and `danos-buildpackage`
(per-package container build). Uses the current Docker Engine API client and works
against Docker or Podman.

| Command | Behaviour |
|---|---|
| `fetch` | Clone/update `danos` repos at `ref`. Fetch `upstream` release tags plus salsa packaging. Mirror `apt` entries at a pinned version |
| `plan` | Print the build order and tiers from Build-Depends. Error on cycles |
| `build [pkg…]` | Fresh `debian:trixie` container per package: install build-deps with `mk-build-deps` from Debian 13, trixie-backports, deb.frrouting.org and the local output repo, then `dpkg-buildpackage` (source + binary). Skips a package whose source-tree hash matches its last successful build |
| `repo` | Build a signed apt repo with `apt-ftparchive` and GPG, including `deb-src` |
| `image` | Run live-build in a container with `image/`. Output: hybrid ISO (the `--onie` output is supported by stock live-build but unused in M1) |
| `test` | Run §8 layers 2–4 |
| `check-updates` | Compare each entry's version with its upstream per `track` (`latest`, `lts`, `debian`). Print drift. Exit non-zero when drift exists |

### 7.2 Manifest

```yaml
- name: configd
  kind: danos                 # our repo, own debian/ packaging
  repo: https://github.com/nudanos/configd
  ref: trixie
  milestone: "1.0"
- name: strongswan
  kind: upstream              # upstream release + Debian packaging
  upstream: https://github.com/strongswan/strongswan
  track: latest
  packaging: https://salsa.debian.org/debian/strongswan
  patches: patches/strongswan/
  milestone: "1.1"
  audit:                      # one entry per DANOS patch considered
    - {patch: <file>, verdict: kept | upstreamed | dropped, note: <reason>}
- name: netplug
  kind: upstream
  track: debian               # Debian's source version, plus our kept patches
  packaging: https://salsa.debian.org/debian/netplug
  patches: patches/netplug/
  milestone: "1.0"
- name: frr
  kind: apt
  source: "https://deb.frrouting.org/frr trixie frr-stable"
  pin: 10.7.1-0~deb13u1
  milestone: "1.0"
- name: openssh
  kind: debian
  milestone: "1.0"
```

### 7.3 CI (GitHub Actions)

- **Package repos:** a three-line workflow calls `distro`'s reusable
  `package.yml`. It builds against the latest nightly repo, runs tests and
  lintian, and uploads `.deb` artifacts. It is required for merge.
- **`distro` nightly:** `fetch → build → repo → image → test` with an
  `actions/cache` keyed on source hashes. Publishes the repo to GitHub Pages and
  the ISO to a GitHub Release. If a cold build exceeds the 6-hour job limit, split
  into one job per build tier, passing outputs as artifacts.
- **`distro` weekly:** `check-updates` opens or updates one issue per drifting package.

### 7.4 Hosting and signing

- apt repo: GitHub Pages (1 GB soft limit). Move to object storage (e.g. Cloudflare
  R2) if exceeded.
- ISOs: GitHub Releases (2 GB per file).
- A new repo signing key (Ed25519/RSA-4096 GPG), stored as a CI secret. The public
  key is committed to `distro` and published with the repo.

### 7.5 Boot security

No Secure Boot in milestone 1. DANOS signed through OBS (`dh-signobs`), which is
gone. The ISO boots with Secure Boot disabled. Adding shim review and a signing
setup is a later item.

## 8. Testing

### 8.1 Layer 1: package tests (every PR)

Each package's own tests run inside `dpkg-buildpackage` (`check`, `go test` with
vet, `prove`, `pytest`). A failure fails the build. Known 2020 failures (strongswan,
vyatta-vrrp) get fixed, not skipped. lintian runs with the updated profile.

### 8.2 Layer 2: install tests (distro PRs, nightly)

In a clean `debian:trixie` container: install `nudanos-router`, remove, purge.
Fail on maintainer-script errors or leftover files outside documented state
directories.

### 8.3 Layer 3: boot test (nightly)

One QEMU/KVM VM. Boot the ISO → serial-console login → `configure`, set an interface
address, hostname and user → `commit`, `save` → reboot → verify config →
`install image` to a virtual disk → boot from disk → verify → `show version`
reports the Debian 13 base and the expected FRR and kernel versions.

### 8.4 Layer 4: scenarios (nightly, release gate)

`distro-build test --topology <name>` starts 2–4 QEMU VMs connected by socket
networks. The existing Robot suites (`tests` repo) are adapted: kernel interface
names instead of `dp0sX`, and `vymgmt` (abandoned 2016) replaced by a small SSH
helper. Config goes through REST/NETCONF where possible.

| Scenario | Checkpoint |
|---|---|
| BGP (existing suite), OSPF, VRRP, REST API (existing suite) | 1.0 |
| MPLS-LDP (existing suite, if kernel MPLS covers it) | 1.0, non-gating |
| SNMP walk, TACACS+ login (tacplus server container) | 1.0 |
| Kea DHCP server + dhcpcd client | 1.1 (DHCP) |
| NETCONF get-config / edit-config | 1.1 (NETCONF) |
| strongswan site-to-site IPsec (existing suite) | 1.1 (VPN) |
| Firewall (existing suite) | 2 |

### 8.5 2105 compatibility fixtures

Boot `binaries/danos-2105-base-amd64.iso` (SHA-256 verified) in QEMU once per
scenario to capture reference `config.boot` files and `show` outputs, stored in
`distro/tests/reference/2105/`. Every 2105 config must load unchanged on the new
build. `show` differences are reviewed and either accepted (e.g. FRR 10.7
formatting) or fixed.

### 8.6 Pass bars

| Stage | Required |
|---|---|
| Package PR | Layer 1 |
| `distro` PR / nightly publish | Layers 1–3 |
| Checkpoint 1.0 | Layers 1–3 + the 1.0 gating scenarios + all 2105 configs load |
| Checkpoint 1.1 | Each rewrite's scenario, with all earlier scenarios still passing |
| Milestone 1.5 | `install.sh` on stock Debian 13 passes layer 3 (minus ISO steps) |

## 9. Roadmap

| Stage | Content |
|---|---|
| **0: Launch** | Create org, push mirrors, archive `debian`/`drop` repos, create `distro` — done 2026-09-28 (plan 1); `debian`/`drop` repos archived 2026-10-01 (plan 2: 47 — 32 Debian-replaced forks, 15 drops) |
| **1.0: Boots** | Build tooling, port checklist on the M1 repos, dataplane untangling, Tier 1 packages, ISO, layers 1–4 (1.0 scenarios). `vyatta-service-dhcp`, `netconfd` and `vyatta-security-vpn` are left out of the 1.0 package set (and the ISO) until their 1.1 rewrites; 1.0 interfaces are statically addressed |
| **1.1: Rewrites** | Kea/dhcpcd, libnetconf2, strongswan 6/VICI, Go modules (one at a time) |
| **1.5: Script install** | `installer/install.sh`, CI test on stock Debian 13 |
| **2: Dataplane** | DPDK 25.11 LTS port (forked librte_sched/librte_acl, ~250 renamed identifiers), vyatta-controller, route broker, NPF firewall/NAT/QoS, nDPI 6.0, flow monitoring |
| **Later** | Perl-hook → VCI migration, Secure Boot, DHCP relay successor, other targets (e.g. Ubuntu 24.04), package renaming/branding |

Each stage gets its own implementation plan.

## 10. Risks

| Risk | Mitigation |
|---|---|
| Backports kernel moves major versions every ~2–3 months and breaks netlink/MPLS/VRF behaviour | Kernel is a manifest entry. Pin, and fall back to Debian 6.12 or self-built 6.18 LTS. Boot and scenario tests catch it nightly |
| FRR 10.7 dropped or changed a behaviour DANOS relied on (FPM, CLI output parsed by op scripts) | Audit the 27 commits first. Op-mode scripts are covered by the 2105 `show` fixtures |
| DANOS code depends on kernel UAPI only in `bvnos-linux-libc-dev` | Port checklist fails loudly. Carry a minimal header patch only where needed |
| Hidden dataplane coupling beyond the 46 known packages | `m1-scope.json` is regenerated in CI. The install test fails on unmet dependencies |
| Cold nightly build exceeds GitHub runner limits | Incremental cache. Per-tier job split. Self-hosted runner on the Debian VM as fallback |
| Single-person dependencies (`jsouthworth.net` vanity domain) | Imported copies plus go.mod replace directives (M1.1) |
| Trademark: "NuDanOS" contains "DANOS", the Linux Foundation project's name | Before public launch, check whether the Linux Foundation holds a registered DANOS mark, and ask it for a no-objection if it does. The project is dormant and the code is LGPL/MPL, so a courteous request is likely to succeed. `vyatta-*` package names stay until the later branding stage |

## 11. Open decisions

Decided: project name **NuDanOS**, GitHub org `nudanos` (2026-09-28).

| Decision | Needed by | Default if undecided |
|---|---|---|
| DHCP relay successor to `isc-dhcp-relay` | Before `isc-dhcp-relay` leaves Debian | Stay on Debian's package |
| apt hosting beyond GitHub Pages | When the repo exceeds ~1 GB | Cloudflare R2 |
