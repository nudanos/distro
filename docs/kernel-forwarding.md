# Kernel forwarding (milestone 1)

NuDanOS 1.0 forwards in the Linux kernel. `vyatta-kernel-forwarding` provides
`vyatta-forwarding` (the DPDK `vyatta-dataplane` is milestone 2) and renames NICs
to DANOS dataplane names (`enp0s3` → `dp0s3`), so the `interfaces dataplane`
configuration model drives kernel netdevs.

`tests/integration/installable.sh` simulates installing every binary package of
the repo together with `vyatta-kernel-forwarding`. Every package it cannot
install is listed below with its reason; `tools/router_deps.py` leaves exactly
these out of `nudanos-router`.

## Dataplane interface model: configd actions

| Action (vyatta-interfaces-dataplane-v1.yang) | Under kernel forwarding |
|---|---|
| `vyatta-interfaces.pl --create-dev/--delete-dev` | works: VRF bind, link down, stats files (the dpid 0 paths; dp1+ remote dataplanes are DPDK) |
| `vyatta-address add/delete`, `ip li set … alias`, `sysctl …/forwarding` | works: kernel netdev |
| `vyatta-interfaces.pl --set-dev-mtu/--set-mac` | works: kernel netdev |
| `vplane-affinity` (cpu-affinity, receive-, transmit-cpu-affinity) | refused at commit (`vyatta-kernel-forwarding-deviations-v1-yang`) |
| `breakout`, `breakout-reserved-for` | absent: a YANG feature only switch platforms enable |
| `vyatta-intf-end`, `vyatta-update-vifs` | works for addresses, MTU and VIFs; its speed, duplex and pause settings go to the DPDK process (a no-op here), so they are refused at commit (see below) |
| `speed`, `duplex` other than `auto` | refused at commit (`vyatta-kernel-forwarding-deviations-v1-yang`) |
| `pause-frame` | absent: a YANG feature only the SIAD platform package enables |
| `ip gratuitous-arp request/reply` (interface and VIF) | refused at commit; `vyatta-interfaces-garp` programs only the DPDK process |
| `ip gratuitous-arp-count`, IPv6 redirects | accepted, no effect (defaulted leaves cannot be refused without refusing every configuration) |

Op commands backed by `vplane-config`'s scripts (`show dataplane …`,
`monitor dataplane …`, `show arp` via `vplane-arp`, `reset ip arp` via
`nbr-res-flush.pl`, `show ipv6 neighbors` via `vplane-nd.pl`, `show interfaces
bonding … detail` via `vplane-ifconfig.pl`) print "requires the DPDK
dataplane; this system forwards in the Linux kernel": `vyatta-kernel-forwarding`
installs `dpdk-required` under those script names. Kernel-native `show arp` and
`show ipv6 neighbors` are not in 1.0; `ip neigh` shows the same table.

## DPDK-only or hardware-specific binaries (left out of nudanos-router)

| Package | Why |
|---|---|
| libvplaned-client1 | DPDK controller client library, built against libprotobuf17 (milestone 2) |
| vplane-config | the DPDK dataplane's config backend (needs the dataplane protocol virtuals and iproute -vyatta) |
| vplane-config-backend | the DPDK dataplane's config backend daemon |
| vyatta-interfaces-dataplane-rpc-v1-yang | RPCs served by vplane-config-backend |
| vyatta-system-dataplane-v1-yang | power profile and CPU mask of the DPDK process |
| vyatta-system-hugepages-v1-yang | DPDK hugepages |
| vyatta-interfaces-backplane-v1-yang | switch backplane (vyatta-dataplane-cfg-backplane) |
| vyatta-interfaces-backplane-deviation-s9500-30xs-v1-yang | switch backplane, UfiSpace S9500 |
| vyatta-op-interfaces-backplane-v1-yang | switch backplane |
| vyatta-interfaces-bonding-qos-v1-yang | QoS is in the DPDK dataplane |
| vyatta-interfaces-bonding-storm-control-v1-yang | storm control is in the DPDK dataplane |
| vyatta-security-storm-control-v1-yang | storm control is in the DPDK dataplane |
| vyatta-op-storm-control-v1-yang | storm control is in the DPDK dataplane |
| vyatta-interfaces-dataplane-transceiver-v1-yang | transceiver data from the DPDK dataplane |
| vyatta-interfaces-tcp-mss-v1-yang | TCP MSS clamping in the DPDK dataplane |
| vyatta-interfaces-switch-vif-tcp-mss-v1-yang | TCP MSS clamping in the DPDK dataplane |
| vyatta-interfaces-vfp-v1-yang | virtual feature points are DPDK (needs TCP MSS) |
| vyatta-interfaces-vfp-unnumbered-v1-yang | virtual feature points are DPDK |
| vyatta-interfaces-policy-v1-yang | interface policy (PBR/QoS) is in the DPDK dataplane |
| vyatta-interfaces-switch-portmonitor-v1-yang | port monitoring needs the firewall model (DPDK) |
| vyatta-service-portmonitor-v1-yang | port monitoring needs the firewall model (DPDK) |
| vyatta-service-portmonitor-deviation-broadcom-stratadnx-v1-yang | port monitoring, Broadcom switch hardware |
| vyatta-interfaces-vhost-xconnect-v1-yang | vhost interfaces are DPDK virtio |
| vyatta-op-dataplane-mpls | MPLS state from the DPDK dataplane |
| vyatta-op-show-platform-dataplane-v1-yang | platform state from the DPDK dataplane |
| vyatta-system-sfp-v1-yang | SFP permit list in the DPDK dataplane |
| vyatta-op-system-sfp-v1-yang | SFP permit list in the DPDK dataplane |
| vyatta-policy-vlan-modify-v1-yang | VLAN modification in the DPDK dataplane |
| vyatta-security-mac-limit-v1-yang | MAC limits in the DPDK dataplane |
| vyatta-service-gnss | UfiSpace switch hardware (python3-ufispace-bsp-utils) |
| vyatta-service-gnss-v1-yang | UfiSpace switch hardware |
| vyatta-op-show-gnss-v1-yang | UfiSpace switch hardware |
| vyatta-op-start-gnss-v1-yang | UfiSpace switch hardware |
| vyatta-op-stop-gnss-v1-yang | UfiSpace switch hardware |
| vyatta-gnss-plugins-ublox | UfiSpace switch hardware |

## 2105 configs at boot

A DANOS 2105 configuration can hold settings only the DPDK dataplane
implements (above, and the deviations in
`vyatta-kernel-forwarding-deviations-v1`). One such node makes the whole
boot configuration fail to load, so `vyatta-kernel-forwarding` sets them
aside first:

- `vyatta-boot-config-loader` (vyatta-cfg) runs every executable in
  `/opt/vyatta/etc/boot-config.d/` as `<hook> <boot file>` before loading,
  and logs `boot-config hook <name>: ok|failed (<status>)`. A failing hook
  never stops the boot.
- `50-dpdk-set-aside` removes the nodes listed in
  `/usr/share/vyatta-kernel-forwarding/dpdk-only-paths` (one path per line,
  `*` for a list key, a trailing `!auto` for "only when the value is not
  auto", e.g. `interfaces dataplane * speed !auto`). When it removes
  anything it keeps the untouched file once as `config.boot.2105-original`,
  writes the removed nodes in config syntax to `config.boot.dpdk-only`, and
  logs each path with tag `dpdk-set-aside`. With nothing to remove the boot
  file keeps its bytes and its modification time.
- While `/config/config.boot.dpdk-only` exists, `/etc/update-motd.d/60-dpdk-set-aside`
  prints a login notice naming it.

The path list is checked by `tests/set-aside-consistency.t` in the port:
every deviation must be covered, and every package in the DPDK-only table
above that ships a configuration YANG module needs a `# <package>` block in
the list (run with `NUDANOS_KF_DOC` pointing at this file). Add a package
here and its paths there together.

## Deferred to a later milestone

| Package | Milestone | Why |
|---|---|---|
| vyatta-security-vpn | 1.1 | VPN: needs python3-vici (strongswan rewrite); only vyatta-interfaces-vti-v1-yang is built in 1.0 |
| vyatta-interfaces-switch-vif-sflow-v1-yang | 2 | sFlow (vyatta-cfg-sflow, host-sflow) is milestone 2 |
| vyatta-cfg-journalbeat | later | journalbeat (Elastic Beats) is not packaged in Debian |
| vyatta-system-journal-export-logstash-v1-yang | later | needs vyatta-cfg-journalbeat |
| vyatta-system-journal-export-logstash-routing-instance-v1-yang | later | needs vyatta-cfg-journalbeat |
| vyatta-interfaces-vif-v1-yang | later | needs vyatta-interfaces-bridge-yang, which no DANOS source provides (superseded by vif-v2) |
| vyatta-vrrp-path-monitor-track-v1-yang | later | needs monitord-feature-dbus: the path monitor daemon was never published in DANOS sources |
| vyatta-vrrp-path-monitor-track-interfaces-bonding-v1-yang | later | needs vyatta-vrrp-path-monitor-track-v1-yang |
| vyatta-vrrp-path-monitor-track-interfaces-dataplane-v1-yang | later | needs vyatta-vrrp-path-monitor-track-v1-yang |
| vyatta-vrrp-path-monitor-track-interfaces-switch-v1-yang | later | needs vyatta-vrrp-path-monitor-track-v1-yang |

## Optional (installable, left out of nudanos-router)

| Package | Why |
|---|---|
| vyatta-system-login-user-isolation-v1-yang | its postinst builds the user sandbox root with mmdebstrap, which needs the network at install time; install it on a running system (optional in DANOS too) |
| pam-sandbox | user isolation's PAM module: enabled at install, it sandboxes every non-root login and fails the session when no sandbox root exists (the router leaves user isolation out) |
| cli-sandbox | user isolation's sandbox service (see pam-sandbox) |
| vyatta-sssd-cli-sandbox | SSSD support inside user isolation's sandbox (see pam-sandbox) |
| python3-shared-storage | used only by cli-sandbox |
| vyatta-cfg-default-minimal | alternative default configuration; the variants conflict (each ships /etc/vyatta/yang.conf) and the router installs vyatta-cfg-default-vr, as DANOS's VR image did |
| vyatta-cfg-default-vcpe | alternative default configuration (see vyatta-cfg-default-minimal) |
| vyatta-cfg-default-vdr | alternative default configuration (see vyatta-cfg-default-minimal) |
| vyatta-cfg-default-vdr-dp | alternative default configuration for the DPDK dataplane (see vyatta-cfg-default-minimal) |

## Platform-specific (hardware deviations, left out of nudanos-router)

Deviation modules that restrict the model to one switch platform's hardware.
Installed on a general-purpose box they refuse valid configuration (a keyed GRE
tunnel, pause frames on dp0s3). `tools/router_deps.py` fails on any hardware
platform deviation module not listed in this document.

| Package | Platform and effect |
|---|---|
| vyatta-interfaces-bonding-deviation-broadcom-stratadnx-v1-yang | Broadcom StrataDNX: at most 32 bonds |
| vyatta-interfaces-dataplane-deviation-ufi-apollo-ncp1-1-v1-yang | UfiSpace Apollo NCP1-1 |
| vyatta-interfaces-dataplane-pause-deviations-siad-v1-yang | SIAD: pause frames only on dp0xe/dp0ce |
| vyatta-interfaces-dataplane-speed-deviations-siad-v1-yang | SIAD: speed validation script on every interfaces commit |
| vyatta-interfaces-switch-deviations-siad-v1-yang | SIAD: switch MAC ignored |
| vyatta-interfaces-tunnel-deviations-broadcom-dpp-v1-yang | Broadcom DPP: tunnel `parameters ip key` not supported |
