# NuDanOS plan 4: checkpoint 1.0 (design)

Addendum to `2026-09-28-debian13-revival-design.md` (the spec). Plan 3 made the
1.0 ISO boot and install (layers 1–3 pass in the nightly; the pre-release
`nightly-20261004` is published). Plan 4 closes **checkpoint 1.0** as spec §8.6
defines it: layers 1–3, plus the 1.0 gating scenarios, plus "all 2105 configs
load".

**Status (2026-10-05):** all four sections approved by the user in
brainstorming. Next: the user reviews this written spec; then the implementation
plan (`superpowers:writing-plans`) and the choice of execution method. No
implementation before the plan is approved.

Decisions the user took while scoping (2026-10-05):

- **Where scenarios run:** gate in the CI nightly (KVM, 16 GB, 4 CPUs) **and**
  keep every topology runnable on the development Mac (Docker Desktop: 8 GB,
  QEMU emulation, no KVM).
- **Which 2105 configs:** the configs every scenario produces on 2105, **plus a
  feature sampler** of single-router configs covering the 1.0 feature set and
  deliberately DPDK-only settings.
- **DPDK-only settings in a 2105 config:** **set aside at boot, load the rest.**
  Today one refused line fails the whole boot commit and the router comes up
  unconfigured (the plan 3 final review's deferred item).
- **Harness:** a **Go-native scenario runner** in `distro-build` (approach A).
  The DANOS Robot suites (`nudanos/tests`) are the source of test cases, not
  something the runner executes. This amends spec §8.4 (see §6).

Not in this plan: the kernel vendor-commit audit and the deferred tooling
minors, which the plan 3 addendum had also assigned to plan 4. They move to
plan 5.

## 1. Facts this design rests on (measured 2026-10-05)

- **2105 runs under emulation.** `danos-2105-base-amd64.iso`
  (SHA-256 `6d500d5d7ea69ebca0b7ada2bd74cec40c87780f41cefd9b9cc0e14fb81d9b51`,
  kept outside the repo) boots in QEMU with `-cpu max`, 2 GB, 2 CPUs and
  virtio NICs. Its default boot entry has a serial console; live login is
  `tmpuser`/`tmppwd`. The DPDK dataplane comes up: `show dataplane` answers,
  `dp0s3` goes up/up with an address after commit, a BGP neighbour commits.
  Memory in use: 1.1 GB of 2 GB. On the live ISO `/config/config.boot` does not
  exist; the configuration is read with `show configuration` or a `save` to a
  file.
- **The NuDanOS boot test** runs one VM at 2 GB and 2 CPUs. Four routers at
  2 GB do not fit the Mac's 8 GB. **A NuDanOS router fits in 1 GB:** booted
  emulated at 1 GB with OSPF and BGP committed, it used 574 MB with 414 MB
  available.
- **Known bug the scenarios will hit:** committing OSPF or BGP makes
  `frr-reload.py` fail. `vyatta-protocols-frr` still writes `fpm address
  127.0.0.1` and `no fpm use-next-hop-groups` into `frr.conf`, but plan 3 stopped
  zebra loading `dplane_fpm_nl`, so zebra rejects those lines ("Unknown
  command") and the reload reports failure. Plan 4 fixes it test-first before
  the routing scenarios.
- **Robot suites:** `nudanos/tests` holds BGP, MPLS-LDP and REST suites (plus
  firewall and IPsec, not 1.0). They assume a hand-built four-router lab,
  hand-edited test data and `vymgmt` (abandoned 2016). Robot Framework is not
  packaged in Debian 13. OSPF, VRRP, SNMP and TACACS+ have no suites.
- **No TACACS+ server in Debian 13** (only `libauthen-tacacsplus-perl`, a client).
- **Boot-time loading:** `vyatta-boot-config-loader` (port `vyatta-cfg`) runs
  `loadFile` then `commit`; nothing at boot calls the old
  `vyatta-config-migrate` framework.

## 2. Architecture

Everything new lives in `nudanos/distro`, except the set-aside step (§4.4),
which changes two ports.

```
distro/
  cmd/scenario/                  runner executed inside the tester container
  internal/topology/             VMs, links, management access, boot, teardown
  internal/scenario/             scenario files, steps, checks, show capture/diff
  internal/boottest/             reused: Console, login, configure, commit, op
  tests/scenarios/<name>/        scenario.yaml + README.md
  tests/reference/2105/<name>/   captured 2105 output per scenario and router
  tests/reference/2105/sampler/  <feature>.set inputs and their 2105 captures
```

- **`distro-build test scenario <name>`** (and `test scenarios` for all; `-image
  2105` for the reference ISO; `-capture` to write references) starts the
  tester container as `test boot` does and runs `cmd/scenario` in it.
- **`internal/topology`** starts up to four QEMU VMs from one image. Each VM has:
  - a serial console on a socket, driven by the existing `Console`
  - one virtio NIC per data link, linked with QEMU socket networks (one link =
    one `-netdev socket` pair), so no root, bridges or taps are needed
  - a management NIC (QEMU user networking) with forwarded ports, so the runner
    reaches each router's SSH, HTTPS and SNMP on localhost. DHCP is a 1.1
    feature, so every router first gets a static base config over the
    console: the management interface at QEMU's fixed guest address
    `10.0.2.15/24` with gateway `10.0.2.2` (the runner's side, where the
    TACACS+ server listens), `service ssh`, and the scenario's own services
  - a copy-on-write overlay of one **installed** disk: the runner installs the
    ISO to a base qcow2 once per image (the layer 3 install steps), and every
    scenario boots overlays of it, so routers start as installed systems
    without repeating the install. 2105 VMs boot the live ISO instead (its
    captures are taken live).
- **Interface names.** NIC PCI addresses are fixed per link order (data links
  from `addr=03` up, management last), so both NuDanOS and 2105 name them
  `dp0s3`, `dp0s4`, … and one scenario config serves both images.
- **Scenario files** (`scenario.yaml`, parsed with `go.yaml.in/yaml/v3`, already
  a dependency) declare routers, links, each router's configuration as `set`
  lines, the `show` commands to capture, and the checks. A check is one of:
  - `op`: an operational command whose output must match a pattern within a
    timeout (e.g. a BGP neighbour `Established`)
  - `http`: a request to a router's REST API with an expected status and body
    pattern
  - `snmp`: an `snmpwalk` (v2c or v3) with an expected OID/value pattern
  - `login`: an SSH login with given credentials that must succeed (with a
    privilege check) or be refused
  - `action`: a step that changes the topology (commit a change on a router,
    take a link or interface down) before later checks
  - `go`: a named Go function for checks that need logic
- **Timeouts** are written for KVM and scaled by the boot test's emulation
  factor (×6) when `/dev/kvm` is absent.
- **On failure** the runner keeps going with the next scenario, and saves each
  router's console transcript and a final `show` dump. It names the failed
  check and ends with a pass/fail table.

## 3. Scenarios

Prefixes come from loopback addresses, not extra host VMs, so no topology needs
more than four routers.

| Scenario | Topology | Checks | Gates 1.0 |
|---|---|---|---|
| `bgp` | R1–R2–R3 in a line, R4 linked to R1, R2 and R3 (the Robot BGP lab without its LAN hosts) | every goal of `BGP_DANOS.robot`: eBGP direct and multihop, iBGP direct and multihop between loopbacks, route reflector (all three reflection rules), next-hop handling, iBGP vs eBGP selection, local-preference, confederation, dampening | yes |
| `ospf` | R1–R2–R3; R2 is the area border (area 0 / area 1) | adjacencies Full; intra- and inter-area routes in the kernel table; loopback-to-loopback ping across R2; a cost change moves the route | yes |
| `vrrp` | R1, R2 on a shared link | master/backup by priority; failover when the master's interface goes down; preemption when it returns | yes |
| `rest` | R1 | `danos_restapi.robot` translated: HTTPS service up, authenticated GET of configuration, set and commit through the API, unauthenticated request refused | yes |
| `snmp` | R1 | SNMPv2c and SNMPv3 (authPriv) walks from the runner: system MIB; IF-MIB lists `dp0s3`; its counters move after traffic | yes |
| `tacacs` | R1 plus a TACACS+ server in the runner | SSH login as a TACACS+ user succeeds at the configured privilege level; a wrong password is refused; local fallback works when the server is unreachable | yes |
| `mpls-ldp` | R1–R2–R3 | LDP sessions up, label bindings exchanged, an MPLS-forwarded ping across R2 | no (reported) |

- **2105 first.** Every scenario must pass on the 2105 ISO, same configs and
  checks, before it judges NuDanOS. A check that 2105 itself fails is fixed or
  dropped. This is how the harness's own correctness is checked against a
  known-good system.
- **TACACS+ server.** A minimal test server built into the runner with a Go
  TACACS+ library (`github.com/nwaples/tacplus`, BSD), a test-only dependency
  of distro. Routers reach it at the management network's host address. Per the
  project rule, the library is fetched and its licence and server API checked
  before any code depends on it; the fallback is building the `tac_plus` daemon
  from source in the tester image.
- **Tester image additions:** `snmp` (net-snmp tools) and `openssh-client`.

## 4. 2105 fixtures

### 4.1 Capture (manual, committed)

- **`distro-build test scenario -image 2105 -capture <name>`** runs the scenario
  on 2105 and writes, per router, to `tests/reference/2105/<name>/<router>/`:
  - `config.boot`: saved by 2105 itself (`save` to a file, then read back),
    including its version footer
  - `commands.txt`: `show configuration commands`
  - `show/<command>.txt`: the scenario's `show` list
- **Sampler.** `tests/reference/2105/sampler/<feature>.set` are single-router
  inputs, committed on 2105 and captured the same way. 1.0 features:
  - interfaces and VIFs, loopback, bridge, bonding
  - static routes, prefix-lists and route-maps
  - BGP and OSPF options, VRRP
  - SNMP, TACACS+, NTP, syslog, DNS, SSH
  - local users, time zone (including `US/Pacific`), LLDP

  plus deliberately DPDK-only inputs:
  - firewall, NAT, QoS
  - cpu-affinity, forced speed and duplex, gratuitous-ARP control
- **The ISO stays off CI.** Captures are committed; `tests/reference/2105/README.md`
  records the ISO's SHA-256 and how to recapture. CI never needs the ISO.

### 4.2 Load check (`distro-build test fixtures`, nightly, gating)

- One NuDanOS VM (installed overlay) takes every captured `config.boot` in
  turn: the set-aside step (§4.4) runs on it, then `load` and `commit` in
  configuration mode.
- **Pass:** the commit succeeds, and NuDanOS's `show configuration commands`
  equals 2105's `commands.txt` minus exactly the set-aside lines. Any other
  difference fails, naming the lines.
- **The boot path itself** is tested once: a sampler config with DPDK-only lines
  is written to `/config/config.boot` and the router rebooted. It must come up
  with everything but the set-aside parts applied, and with the set-aside file
  and login notice present (§4.4). One reboot, not one per config, keeps the
  emulated Mac run practical.

### 4.3 `show` differences

- Scenarios on NuDanOS capture the same `show` commands. Volatile fields
  (uptimes, counters, timers, ages) are normalised by per-command filters.
- Each difference from the 2105 capture must appear in that scenario's reviewed
  `tests/reference/2105/<name>/accepted.diff` (FRR 10.7 formatting, for
  example). An unlisted difference fails the scenario.
- Writing `accepted.diff` is review work during the plan: each accepted
  difference is a ledgered ruling; a difference that reveals a NuDanOS bug is
  fixed instead.

### 4.4 Set-aside at boot (product change)

- **`vyatta-cfg`:** `vyatta-boot-config-loader` runs each executable in
  `/opt/vyatta/etc/boot-config.d/` (sorted) with the boot file's path before
  `loadFile`. A hook that fails is logged and skipped, never fatal: the load
  must still happen.
- **`vyatta-kernel-forwarding`** ships the hook and a list of DPDK-only
  configuration paths:
  - the firewall, NAT, QoS and policy trees of the DPDK-only packages
  - `cpu-affinity`, `receive-cpu-affinity`, `transmit-cpu-affinity`
  - `speed` and `duplex` other than `auto`
  - `ip gratuitous-arp` request/reply control (interface and VIF)

  The hook moves matching nodes out of the boot file and:
  - keeps the untouched original as `/config/config.boot.2105-original` (first
    run only)
  - writes the removed nodes to `/config/config.boot.dpdk-only`
  - logs each removed path to syslog
  - adds a login notice naming what was set aside and where it is

  A config with nothing to set aside is left byte-for-byte unchanged.
- **Consistency tests** (in the port): every refusal in
  `vyatta-kernel-forwarding-deviations-v1.yang` has a list entry; every
  DPDK-only package in `docs/kernel-forwarding.md` has its top-level tree
  covered. The two cannot drift apart silently.
- **Open question the fixtures answer:** what configd's `loadFile` does at boot
  with nodes whose YANG is not installed at all (firewall on NuDanOS): drop
  them or fail. The list covers those trees either way.

## 5. CI, resources, tests

- **Nightly:** after layer 3 and the UEFI smoke boot, `distro-build test
  fixtures` then `distro-build test scenarios`. The release job requires
  fixtures and every gating scenario; `mpls-ldp` is reported in the run summary
  and never blocks. One artifact holds every transcript, `show` dump and diff.
  Expected addition: 40–60 minutes (the nightly is about 50 minutes today;
  GitHub's job limit is 6 hours).
- **Router memory:** NuDanOS routers get 1 GB (measured, §1); 2105 VMs get
  1.5 GB (measured use: 1.1 GB). A four-router NuDanOS topology needs 4 GB.
- **Testing the harness:** unit tests for topology QEMU arguments, scenario-file
  parsing, every check type against recorded output, `show` normalisation and
  diffing, and console replays copied from real transcripts (the layer 3
  pattern). The set-aside hook is tested in its port against the sampler
  configs, plus the consistency tests above.
- **Bugs found** in NuDanOS by scenarios or fixtures are in scope: each is fixed
  test-first and ledgered as in plan 3.

## 6. Spec amendments

- **§8.4:** scenarios run under a Go runner in `distro-build`; the DANOS Robot
  suites are the source of the BGP and REST test cases. The "kernel interface
  names" sentence was already superseded by plan 3 (`dp0sN` names stay).
- **§8.5:** sampler configs in addition to scenario configs; captures are
  committed and the 2105 ISO is not needed in CI; DPDK-only settings are set
  aside at boot (§4.4) rather than failing the load.
- **§9:** the 1.0 row is marked done when the nightly passes fixtures and every
  gating scenario.

## 7. Done means

- The nightly passes `test fixtures` and all six gating scenarios, on the ISO it
  builds, and publishes.
- Every 2105 capture (scenarios and sampler) loads on NuDanOS with only the
  set-aside lines missing.
- Every `show` difference from 2105 is either fixed or in a reviewed
  `accepted.diff`.
- The spec's checkpoint 1.0 row is updated.
