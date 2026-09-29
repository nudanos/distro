# SONiC and VyOS: what helps NuDanOS

Reviewed 2026-09-29: https://github.com/orgs/sonic-net/repositories (50 repos)
and https://github.com/vyos (43 repos). Both are active, Debian-based network
OS projects. Unlike 6WIND, their code is open, so there is real material to use.

## Context

| | SONiC | VyOS | NuDanOS |
|---|---|---|---|
| What | Switch NOS (hardware ASICs via SAI), Linux Foundation | Router OS, the other Vyatta descendant | Router OS (DANOS revival) |
| Debian base | builds **trixie** (and bookworm) | **bookworm** (rolling, 2026-09) | trixie |
| State/config | Redis DBs + YANG mgmt framework | Python config backend (vyos-1x), OCaml vyconf | configd (Go, YANG) |
| Dataplane | ASIC via SAI; VPP as virtual platform | VPP (vyos-vpp-patches) | DPDK (M2) / kernel (M1) |
| FRR | 10.5.4 + 89 patches | upstream-based | 10.7.1 (FRR's packages) |

## SONiC: worth using

| Repo | Use for NuDanOS | When |
|---|---|---|
| **sonic-buildimage** (`src/sonic-frr/patch`, 89 patches on FRR 10.5.4) | SONiC pushes routes through FRR's FPM dataplane plugin, the same model as DANOS's route broker. Its FPM/dplane patches (e.g. `0012 dplane-fpm-sonic module`, `0013 do not send local routes to fpm`, `0026 translate tableid for dplane route notify`) are the reference for auditing DANOS's 27 FRR commits and for the M2 route broker | Plan 3 FRR audit, M2 |
| **sonic-buildimage** (trixie build) | Trixie-era fixes for packages we also build (kernel packaging via `sonic-linux-kernel`, FRR, net-snmp, lldpd). Check here first when a wave hits a Debian 13 build failure | Waves B-E |
| **sonic-dhcp-relay** (`dhcp4relay`, `dhcp6relay`) | Candidate successor for `isc-dhcp-relay` (spec §11 open decision; Kea has no relay). Caveat: SONiC's relays read config from its Redis CONFIG_DB and would need decoupling | M1.1 decision |
| **sonic-gnmi** (Go, gNMI server + gNOI) | Streaming telemetry and gNOI operations: the gap noted in the 6WIND review. Go, like configd; a gNMI northbound on configd's YANG model is a natural fit | After M1 |
| **sonic-mgmt-framework / sonic-mgmt-common** (Go, YANG translib → REST/gNMI) | Architecture reference for serving one YANG model over several northbound APIs (DANOS does this with configd + REST + NETCONF) | Reference |
| **sonic-netconf-server** (Go) | A Go NETCONF server. Reference for the M1.1 NETCONF rewrite (currently: port netconfd to libnetconf2) | M1.1 decision |
| **sonic-platform-vpp** | VPP as a SONiC platform: evidence for VPP as a dataplane option alongside DANOS's own DPDK dataplane | M2 decision |
| sonic-ztp, sonic-snmpagent, sonic-stp, sonic-bmp | Zero-touch provisioning; AgentX subagent; STP daemon; BMP collector. Feature references | Later |

Not useful: SAI/ASIC repos (sonic-sairedis, saibcm-modules, sonic-pins, DASH): switch-silicon specific, and NuDanOS targets software routing.

## VyOS: worth using

| Repo | Use for NuDanOS | When |
|---|---|---|
| **vyatta-bash, vyatta-cfg, vyatta-cfg-system, live-boot** | Maintained forks of the *same* Vyatta ancestors as DANOS's packages, building on bookworm. Compare when our ports of these fail. Their fixes are one Debian release behind ours but share lineage | Waves B-C |
| **vyos-build** (Python + Docker + live-build, `data/defaults.toml`) | A mature image build for a Debian router OS: flavours, architectures, signing. Reference for plan 3's `distro-build image` | Plan 3 |
| **vyos-nightly-build** | Scheduled ISO builds published from GitHub Actions: the model for spec §7.3's nightly ISO release | Plan 3 |
| **shim-signed / efi-boot-shim** | How a community Debian-based router OS handles Secure Boot. Reference for the deferred Secure Boot item (spec §7.5) | Later |
| **libpam-tacplus, libtacplus-map, libnss-mapuser, libpam-radius-auth** | TACACS+/RADIUS user mapping (Cumulus-derived, `-cl5.1.0`). NuDanOS builds kravietz pam_tacplus 1.7.0 plus DANOS's tacplusd. Compare the user-mapping approach with DANOS's sandbox/AAA | Wave D reference |
| **vyos-vpp-patches** | VyOS's patches to VPP: the second data point (with SONiC) for evaluating VPP for M2 | M2 decision |
| ipaddrcheck | IPv4/IPv6 validation CLI; overlaps DANOS's `vyatta-validate-type` | Reference |

Not useful: vyos-1x itself (a different config system, Python), vyconf (OCaml), and the Ansible collections (VyOS CLI-specific).

## Licensing

SONiC is mostly Apache-2.0. VyOS is GPL-2.0/LGPL-2.1. DANOS-authored code is LGPL-2.1/MPL-2.0. Reuse as **separate packages** is fine either way. Copying GPL code *into* an LGPL/MPL DANOS repo would change that repo's licence, so port ideas and fixes, and vendor code only when the licences match.

## Recommended follow-ups

1. Plan 3: consult `sonic-buildimage/src/sonic-frr/patch` when auditing DANOS's FRR commits.
2. Spec §11 open decision (DHCP relay): evaluate `sonic-dhcp-relay` decoupled from Redis, against keeping Debian's `isc-dhcp-relay`.
3. M1.1 NETCONF: compare porting netconfd to libnetconf2 with a Go server (sonic-netconf-server as reference) talking to configd directly.
4. M2 dataplane: evaluate VPP (SONiC and VyOS both use it) against porting DANOS's DPDK dataplane to 25.11 before committing M2's design.
5. After M1: gNMI telemetry (sonic-gnmi) and a monitoring stack.
