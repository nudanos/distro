# DANOS code review and preservation status

Reviewed 2026-09-28 against every repository in <https://github.com/danos>.
Raw data: `inventory.json` (per-repo metrics), `depgraph.json` (build graph),
`github-metadata.json` (GitHub API snapshot). Scripts: `tools/`.

## 1. What DANOS is, and what happened to it

DANOS (Disaggregated Network Operating System) is AT&T's router OS, descended from
Vyatta (Vyatta → Brocade → AT&T in 2017). The Linux Foundation announced the project
in March 2018, and the code went public in November 2019.

| Date | Event |
|---|---|
| 2019-11 | First public release, 1908 |
| 2020–21 | Releases 2005, 2009, 2012, **2105** (June 2021, the last announced release) |
| 2021-10-31 | Ciena closes its purchase of AT&T's Vyatta business |
| 2021-11-30 | Last mass push to GitHub. Release **2111** exists in code but was never announced |
| 2023-12 | Last commit to any repo (cloud-init). No activity since |

**The binary side is gone.** The project wiki says of the apt repository on S3:
*"The above AWS service is no longer operational, there is currently no replacement."*
The ISOs, the livebuild tarballs and the release signing key all return 404. The Jenkins
CI and the Open Build Service instance that built the packages were internal to
AT&T. **GitHub source is the only surviving copy.**

## 2. Preservation done so far

| Copy | Location | Contents |
|---|---|---|
| Full mirrors | `mirrors/*.git` | All 189 repos, every branch and tag, full history (`git clone --mirror`) |
| Working trees | `repos/*` | Shallow checkouts of default branches, used for this review |

Default branches alone would **not** have been a complete copy. The code that
built the last releases sits on other branches:

- `dpdk`: default branch is 18.11, but the dataplane needs 20.11 (`20.11.x` branch)
- `linux-vyatta`: branches for 4.19.199, 5.4.149 and **5.10.69**
- `frr`: `danos/7.5`, `danos/7.5.1`, `danos/7.6`
- `build-iso`: one branch per release (danos-1908, fleetwood=2005, glasgow=2009,
  halifax=2012, inverness=2105, jarrow=2109, kington=2111=master)

macOS has a case-insensitive filesystem, so a few kernel and netfilter files that
differ only in case collide in the `repos/` working trees. The bare mirrors are
unaffected. Do any builds on Linux.

| Wiki | `wiki/` | All 100 Confluence pages (storage HTML) plus 75 attachments, `index.json` |
| ISO | `binaries/` | `danos-2105-base-amd64.iso`, SHA-256 matches the wiki's published hash |
| jsouthworth tooling | `mirrors-jsouthworth/` | 17 repos, full mirrors (see 2.1) |

### 2.1 John Southworth's personal repos

<https://github.com/jsouthworth> wrote much of the Go management plane. His
personal account holds the one piece the org lacks: **tooling for building DANOS
outside AT&T.**

- **danos-buildpackage** (Go): builds any DANOS source package inside a Docker image
  (`debian:buster` plus build deps). Takes `-pkg DIR`, a local directory of
  already-built debs to satisfy dependencies.
- **danos-buildimage** (Go): builds the ISO with live-build in a container.
- **danos-bootstrap** (Go, Dec 2020): **a full from-source rebuild of the whole
  distro.** Clones every danos repo, topologically sorts `debian/control`, builds
  each package with danos-buildpackage, and feeds the results back as build deps.
  Its TODO records an end-to-end run with only 8 failures: grub (gcc-6), the
  Broadcom OpenNSL pair, ntp, rsyslog (aclocal-1.15), strongswan and vyatta-vrrp
  (unit tests), vyatta-cfg-sflow, and vyatta-security-vpn (knock-on from
  strongswan). The logs are in `failure-logs/`.
- **vci-dhcpv6-pd**: a DHCPv6 prefix-delegation VCI component that never made it
  into the org. **vyrest**: a Go REST client.
- Upstreams of the vendored Go libraries (immutable, seq, transduce, try, dyn, etm,
  hash, objtree, dbus fork) and **go-import-redirector**, the server behind the
  `jsouthworth.net/go/*` vanity import paths.

Docker Hub still serves the prebuilt images: `jsouthworth/danos-buildpackage` and
`jsouthworth/danos-buildimage`, tags 1908, 2005, 2009, **debian10-bootstrap**,
2012, 2105 and latest (~311 MB and ~85 MB). Only the `debian10-bootstrap` tag
works without the dead S3 apt repo. The release tags' Dockerfiles add that
repo, so rebuilding those images from their Dockerfiles now fails.

Still to preserve:
- The Docker Hub images above (needs docker/podman, or a registry pull by hand)
- Release assets on repos other than build-iso (the scan hit GitHub's anonymous
  API rate limit partway through)

## 3. Inventory

189 repos, 4 of them empty. 185 have content.

| Group | Repos | Lines | Notes |
|---|---|---|---|
| DANOS-authored | 119 | ~1.05M | C 528k, Go 204k, YANG 104k, Perl 95k, Python 74k, C++ 21k |
| Patched upstream imports | 66 | ~30M | Kernel (21.7M), DPDK, FRR, libvirt, strongswan, openssh, … |

Most important DANOS-authored components:

| Component | Lang | Size | Role |
|---|---|---|---|
| vyatta-dataplane | C | 422k | DPDK forwarding pipeline, NPF firewall/NAT, QoS, FAL hardware abstraction |
| configd | Go | 44k | Transactional YANG config datastore, sessions, AAA |
| yang | Go | 42k | YANG compiler and schema library |
| config | Go | 24k | Config tree / diff / union |
| vci | Go | 9k | Vyatta Component Infrastructure (component bus over D-Bus) |
| vyatta-controller (vplaned) | C | 17k | Netlink/config relay to the dataplane over ZMQ |
| vyatta-route-broker | C | 5k | FRR → kernel/dataplane FIB sync |
| vplane-config-npf / -qos | Python+Perl | 22k / 14k | Firewall and QoS config into the dataplane |
| vyatta-cfg, vyatta-cfg-system, vyatta-interfaces | C/Perl | ~45k | Legacy Vyatta config layer |
| build-iso | live-build | – | ISO / ONIE image definition |

## 4. Architecture

```
  CLI (vbash)   NETCONF (netconfd)   REST (vyatta-rest)
        \              |              /
         +------- configd (Go) ------+        opd (op-mode commands)
          YANG schema, sessions, AAA, commit
                 |                  \
        VCI bus (D-Bus)          provisiond -> legacy Perl/shell scripts
         VCI components (Go/Python/Perl)        (node.def / configd:end hooks)
                 |
   FRR --zebra FPM--> route-broker --> dataplane
   kernel netlink ---> vplaned (vyatta-controller) --ZMQ/protobuf--> dataplane
                                                   DPDK pipeline + FAL plugins
```

Base: Debian with live-boot. Squashfs images on overlayfs let several images be
installed side by side (image management in vyatta-image-tools).

## 5. Build graph

`tools/depgraph.py` reads every `debian/control`: **177 source packages produce
1,284 binary packages** in **11 dependency tiers with no cycles**. A from-source
rebuild is feasible in topological order.

- Tier 0 (74): toolchain helpers (dh-yang, dh-vci, dh-golang, golang-defaults),
  most patched upstreams, simple vyatta-* packages
- Tier 4: vyatta-dataplane (after dpdk and vyatta-dpdk-swport)
- Tiers 5–8: Go stack. encoding → vci/yang → config → configd/opd/provisiond
- Tier 10: vyatta-rest

Most depended-on: dh-yang (55 repos), golang-defaults (39), dh-golang (38),
dh-vci (18), vci (11).

## 6. Findings

### 6.1 Everything underneath is end-of-life

| Layer | DANOS 2111 | Status |
|---|---|---|
| Base OS | Debian 10 buster | Standard LTS ended June 2024 |
| Kernel | 4.19 shipped; 5.10.69 branch in progress | 4.19 EOL; 5.10 nearing the end of upstream LTS |
| DPDK | 20.11 plus 2 DANOS patches | EOL. 4 LTS releases behind |
| FRR | 7.5 / 7.6 | Several major versions behind (10.x) |
| Go | Forked toolchain, 1.15 | Unsupported for years |
| libyang | 1.0.184 | Upstream moved to 2.x/3.x with API breaks |
| ISC DHCP | 4.4.1 | ISC ended ISC DHCP maintenance in 2022. Successor is Kea |
| openssh | 7.9p1 | Old; many CVEs since |
| strongswan | 5.7.2 (5.9 branch) | Old; CVEs since |
| sssd | 1.12.5 (2015) | Very old |
| ntp | 4.2.8p15 | Maintained, but chrony/ntpsec are the modern choice |

Treat any 2105/2111 image as unfit for an internet-facing network. The ~30M lines of
patched upstream code are the largest liability. Most of those forks exist only to
backport features to buster, and a modern Debian base makes many of them unnecessary.

### 6.2 Build and CI infrastructure is missing

- Built on Open Build Service plus internal Jenkins (8 repos have Jenkinsfiles that
  call an internal `dram` tool). None of it is public. jsouthworth's
  danos-buildpackage / danos-bootstrap (section 2.1) is the public substitute and
  the natural starting point for a new build system.
- `debhelper` compat 9 in 91 of 119 repos, `Standards-Version` 3.9.x. Current Debian
  deprecates compat 9 and expects 13+.
- Go is packaged the Debian GOPATH way. **32 of 33 Go repos have no `go.mod`.**
  Imports are already `github.com/danos/...`, so adding modules is mechanical.
  One stray `brocade.com/vyatta/cmdclient` import, in `opd/cmd/opstress` only.
- Some Go deps use vanity paths on a personal domain (`jsouthworth.net/go/*`). It
  resolves today but should be pinned or vendored.

### 6.3 The management-plane migration was left half done

The architecture docs describe moving features off legacy scripts and onto VCI
components. At shutdown:

- **18** native VCI components
- **~700** legacy YANG script hooks (`configd:end/create/update/delete/begin`), plus
  44 `call-rpc` and 35 `get-state` scripts, all bridged through provisiond
- **835** legacy `node.def` op-mode templates
- **95k lines of Perl**, mostly on these legacy paths (vyatta-interfaces,
  vyatta-service-snmp, vyatta-security-vpn, vyatta-service-dhcp, …)

Python is already Python 3. The `danos-no-python2` package exists, and only 3 of
318 Python files look like Python 2.

### 6.4 Dataplane

The best-engineered part of the system: meson build, `-Werror`, LTO, unit tests with
`check`, and whole-dataplane test harnesses. Moving past DPDK 20.11 requires:

- ~250 references to identifiers DPDK renamed in 21.11 and removed in 22.11
  (`PKT_RX_*`, `ETH_RSS_*`, `DEV_*_OFFLOAD_*`, `ether_addr`, …). Mechanical;
  DPDK ships Coccinelle scripts for these
- Two DANOS-specific DPDK patches (`librte-acl-rcu-qsbr-dq-support`,
  `librte-crytpodev-session-sym-pool-empty`). Check whether upstream DPDK
  absorbed them or they have to be carried
- A patched kernel-headers package (`bvnos-linux-libc-dev-vyatta`)

### 6.5 Hardware (white-box switch) support can't be rebuilt

`opennsl-binary` packages a Broadcom blob that is not in the repo, and
`libfal-opennsl` depends on it. The UfiSpace and Accton BSPs target 2019–2020
platforms. A revival should aim at the **software router** (x86 VM or bare metal
with DPDK NICs) and leave FAL as an extension point.

### 6.6 Tests

- vyatta-dataplane: unit tests plus whole-dataplane harness (~200 test files)
- configd, yang, config, vci, encoding: Go unit tests
- `tests` repo: a small Robot Framework suite
- Most Perl/Python feature packages: few or no tests

### 6.7 Licensing and naming

DANOS-authored code is LGPL-2.1 (88 repos), MPL-2.0 (12), MIT (7) or BSD (3).
9 small config-only repos have no LICENSE file. A community fork is legally fine.
"DANOS" (Linux Foundation) and "Vyatta" (Ciena) are trademarks, so a fork needs
its own name.

### 6.8 Relationship to VyOS

VyOS is the other living Vyatta descendant. It forked from Vyatta Core in 2013 and
went a different way: Python config backend, VPP dataplane. DANOS's distinctive
assets are the **YANG-first configd with transactional sessions**, **VCI**, and
the **DPDK dataplane with NPF**. A revival should keep those.

## 7. Gap to Debian 13 "trixie" (current stable, 13.7)

`tools/trixie_gap.py` produces `trixie-gap.json`. Inputs: trixie Sources and amd64
Packages indexes (`research/`).

**Build-deps:** 34 repos need something that is neither in trixie nor built by
DANOS. Nearly all of them are patched upstream forks (Python 2 era deps, gcc-6,
`dh-systemd`, `libpcre3-dev`). Among DANOS-authored repos the gaps are few:

- `bvnos-linux-libc-dev(-vyatta)`: headers from the DANOS kernel (8 repos)
- `dh-signobs`: OBS secure-boot signing helper, never published (7 repos)
- `pylint3`, `python3-pytest-pep8`: renamed/removed Python tooling
- `librte-acl-rcu-qsbr-dq-support-dev`, `librte-crytpodev-session-sym-pool-empty-dev`:
  feature flags from DANOS's patched DPDK
- `vyatta-dataplane-flow-plugin-protobuf`: from `vyatta-dataplane-flow`, which is
  **empty on GitHub**. Only vyatta-cfg-sflow needs it

**Forks against trixie** (release-branch figures, not default branch):

| Fork | DANOS | trixie | DANOS delta |
|---|---|---|---|
| kernel | 5.10.69 | 6.12 | ~88 vendor commits: net, ipv6, mpls, bridge, vlan, team, vrf |
| DPDK | 20.11.3 (+52 patches) | 24.11.4 | **Forked librte_sched (8 queues/TC, per-DSCP WRED, per-subport WRED, 64-bit counters) and librte_acl (hash, rcu, copy)**, cryptodev mempool, driver fixes. The hardest part of the port |
| FRR | 7.6 | 10.3 | 27 commits, mostly zebra dplane-FPM fixes the route broker relies on |
| strongswan | 5.9.0 (+48 patches) | 6.0.1 | 20 DANOS-authored patches |
| keepalived | 2.2.0 | 2.3.3 | 18 DANOS patches (VRRP integration) |
| owamp, vermont, repcpd, mstpd | – | not in trixie | Must stay forks (or be dropped) |

Most other forks exist only to backport to buster. Trixie's own versions can
replace them outright: openssh, grub, shim, libvirt, rsyslog, net-snmp, rdma-core,
valgrind, cloud-init, libyang, golang, and more.

**Kernel-forwarding mode exists.** `vyatta-dataplane.service` has
`ConditionKernelCommandLine=!no-dataplane`, and `vyatta-interfaces-system-*` YANG
models kernel-native interfaces. So the management plane, FRR and the kernel data
path can come up on trixie before the DPDK dataplane is ported.
