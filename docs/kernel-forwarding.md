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
| `vyatta-intf-end`, `vyatta-update-vifs` | works |

Op commands that ask the DPDK process report failure under kernel forwarding:
`show dataplane …`, and, in `vyatta-op`, `reset ip arp …` and
`show ipv6 neighbors` (`nbr-res-flush.pl`, `vplane-nd.pl`).

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
| vyatta-cfg-default-minimal | alternative default configuration; the variants conflict (each ships /etc/vyatta/yang.conf) and the router installs vyatta-cfg-default-vr, as DANOS's VR image did |
| vyatta-cfg-default-vcpe | alternative default configuration (see vyatta-cfg-default-minimal) |
| vyatta-cfg-default-vdr | alternative default configuration (see vyatta-cfg-default-minimal) |
| vyatta-cfg-default-vdr-dp | alternative default configuration for the DPDK dataplane (see vyatta-cfg-default-minimal) |
