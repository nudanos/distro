# Networking review — 2026-11-03

## Scope and confidence

Reviewed the actual `trixie` sources selected by `distro/manifest.yaml` at distro commit `864370dc3ff5a6086ce298b147f79ac2c4de315e`, plus preserved defaults of deferred repositories with no `trixie` branch. All clones are depth 1; no product edits, privileged scripts, or GitHub writes. This is a source review, not a Linux runtime validation. This Windows host has no installed Go/Linux/container environment; Git's Perl failed with an MSYS named-object permission error. The parent found the bundled Python runtime and passed the distro's 50 Python tests. Do not describe the findings below as failures of the working positive-path routing scenarios. The parent review separately verified today's nightly Linux run: all seven scenarios passed, including non-gating MPLS. These findings concern migration, invalid input, failure propagation, cleanup, native-host integration, and deferred dataplane work.

Current strategy is materially better than the old default-branch inventory suggests: kernel forwarding deliberately bypasses DPDK, modern FRR is consumed from upstream packages, and only protobuf libraries are built from the old dataplane. Source evidence: [manifest FRR selection](https://github.com/nudanos/distro/blob/864370dc3ff5a6086ce298b147f79ac2c4de315e/manifest.yaml#L109), [protobuf-only profile](https://github.com/nudanos/distro/blob/864370dc3ff5a6086ce298b147f79ac2c4de315e/manifest.yaml#L901), [kernel FRR daemon options](https://github.com/nudanos/vyatta-protocols-frr/blob/7b3c841394f68e01dba37b88053f1a20971ae4e7/etc/frr/daemons.danos#L14). Therefore the preserved FRR 6.0 fork is not the current routing engine, and the DPDK 18.11 default is not a present kernel-mode dependency.

## Prioritized findings

### N1 — P1: firewall-bearing migration boots with the policy removed (confirmed behavior / security design defect)

`vyatta-kernel-forwarding/dpdk-only-paths:9-15` lists `security firewall`, interface firewall bindings, `service nat`, and `policy qos`. `boot-config.d/50-dpdk-set-aside:105-120` removes matching blocks/leaves; `158-180` backs up the original, saves removed settings, and replaces the boot configuration. It exits successfully at `183`. The package explicitly implements no kernel firewall/NAT/QoS (`README.md:9-10`, `debian/control:16-19`). Existing tests assert that the firewall and interface binding disappear (`tests/set-aside.t:38-43,79-87`). A backup and login MOTD are helpful diagnostics, but they occur alongside forwarding with the original perimeter policy absent.

Evidence: [patterns](https://github.com/nudanos/vyatta-kernel-forwarding/blob/471ce7e62166d3966b6a648dcff870ebba955229/dpdk-only-paths#L9), [successful replacement](https://github.com/nudanos/vyatta-kernel-forwarding/blob/471ce7e62166d3966b6a648dcff870ebba955229/boot-config.d/50-dpdk-set-aside#L158), [existing regression expectation](https://github.com/nudanos/vyatta-kernel-forwarding/blob/471ce7e62166d3966b6a648dcff870ebba955229/tests/set-aside.t#L38).

Recommendation: distinguish unsupported performance settings from security policy. When firewall/NAT policy is present, do not enable transit forwarding until an explicit reviewed migration decision or an implemented kernel-policy translation exists. Keep management recovery available. Add a VM regression fixture with deny rules and packet probes that verifies denied transit remains denied through migration, failed migration, reboot, and recovery. This does not mean a fresh kernel-mode router with no promised firewall is itself broken; it is the automatic conversion of a previously protected configuration that is unsafe.

### N2 — P1/P2: ifmgrd advances its running state after interface actions fail (confirmed static bug)

`ifmgrd/intf_state_machine.go:182-189` receives commit errors, prints them, and still returns `true`. `applyconfig:239-244` unconditionally stores the candidate as running and emits `configuration-updated`; `doneApplying:356-367` then regards matching candidate/running pointers as complete. `dispatcher.go:41-49` already acknowledged Apply before those asynchronous actions completed. Failed IP/MTU/VRF/address scripts can leave the kernel state different from both the acknowledged config and ifmgrd's running cache; repeating the same config need not retry the failed change.

Evidence: [discarded errors](https://github.com/nudanos/ifmgrd/blob/40e22ccdfd43ab7f408c9fca459625a4d25cc195/intf_state_machine.go#L182), [unconditional running update](https://github.com/nudanos/ifmgrd/blob/40e22ccdfd43ab7f408c9fca459625a4d25cc195/intf_state_machine.go#L239), [completion](https://github.com/nudanos/ifmgrd/blob/40e22ccdfd43ab7f408c9fca459625a4d25cc195/intf_state_machine.go#L356).

Recommendation: return success/error separately from whether a diff existed; advance running state only after successful actions. Publish failure state to the management plane, preserve last applied config, and provide bounded retry/reconcile. Regression: inject an action failure (e.g. rejected MTU or link enslaving), assert no success notification or false running state, then clear the failure and reapply the same config and verify the kernel converges. Hot-unplug during failure should also be exercised. This is especially important when native NetworkManager/networkd or another privileged agent can reject or overwrite a kernel change.

### N3 — P2: accepted VRF names of length 13–16 are silently not provisioned (confirmed static bug)

`routing-instance/yang/vyatta-routing-v1.yang:79-83` accepts names of length `1..16`. `vrf-manager/lib/perl/Vyatta/VrfManager.pm:21-22` limits names to 12 because the kernel master is `vrf` plus the name. `scripts/vrf-manager:205-208` detects longer names, prints a refusal, and returns `$SUCCESS`. A routing component depending on the nonexistent device then defers FRR reload and returns nil (`vyatta-protocols-frr/cmd/frr/main.go:135-149`), waiting for a provisioning event which will not arrive. This can leave a committed routing change unapplied.

Evidence: [YANG range](https://github.com/nudanos/routing-instance/blob/a5982b5731bd351982f9d2f250dcec39a4e5e6db/yang/vyatta-routing-v1.yang#L79), [silent success](https://github.com/nudanos/vrf-manager/blob/a9be87ae5d0ea261df034fffee9cab087f301f41/scripts/vrf-manager#L205), [FRR wait](https://github.com/nudanos/vyatta-protocols-frr/blob/7b3c841394f68e01dba37b88053f1a20971ae4e7/cmd/frr/main.go#L135).

Recommendation: align schema and implementation limits (12 with the current prefix), reject invalid names before commit, and make provisioning refusal an error. Test names of length 12, 13, and 16, with a BGP/static/OSPF config in that instance. Add a timeout/failure path for deferred FRR provisioning, including failed `ip link add` and missed notifications.

### N4 — P2: MPLS setup errors are ignored and removed interfaces retain MPLS input (confirmed static bugs; positive MPLS operation can still pass)

`vyatta-protocols-frr/cmd/frr/main.go:119-120` ignores the error returned by `Mpls(frontend_map)`. `mpls.go:55-93` can fail to open/write `platform_labels` or per-interface `input`, but the caller writes the new config and reloads FRR anyway. The function only enables: it writes `1` to each configured input, and the early return when LDP is absent (`15-21`) never resets removed interfaces to `0`. It is passed no previous configuration, despite Set already holding the previous map. Thus removing LDP or one LDP interface can leave kernel MPLS receive state enabled until link teardown/reboot, and a missing module/read-only sysctl can produce a successful management commit with a nonfunctional datapath.

Evidence: [ignored return](https://github.com/nudanos/vyatta-protocols-frr/blob/7b3c841394f68e01dba37b88053f1a20971ae4e7/cmd/frr/main.go#L119), [early return](https://github.com/nudanos/vyatta-protocols-frr/blob/7b3c841394f68e01dba37b88053f1a20971ae4e7/cmd/frr/mpls.go#L14), [enable-only writes](https://github.com/nudanos/vyatta-protocols-frr/blob/7b3c841394f68e01dba37b88053f1a20971ae4e7/cmd/frr/mpls.go#L78).

Recommendation: reconcile old/new per-interface state, reset removed inputs, propagate all failures, and make the transition recoverable. Regression: enable on A/B, remove A, remove LDP entirely, then probe sysctls and packet behavior; inject open/write failure and assert failed commit rather than false success. The existing positive scenario cannot catch cleanup or error propagation.

### N5 — P2: null RPC arguments panic ifmgrd (confirmed static bug; socket-local access required)

`ifmgrd/conn.go:202-207` applies `reflect.TypeOf(v)` and calls `t1.ConvertibleTo(t2)` without checking nil. JSON `null` decodes as a nil interface. A well-shaped request with e.g. Register's one argument set to null reaches a nil dereference; there is no recovery around reflective dispatch (`220-221` or `Handle`). The daemon accepts the Unix socket as `configd:vyattacfg`, mode `0770` (`debian/ifmgrd.ifmgrd.socket:5-8`), so this is an authenticated/local availability concern, not an unauthenticated network vulnerability.

Evidence: [nil dereference](https://github.com/nudanos/ifmgrd/blob/40e22ccdfd43ab7f408c9fca459625a4d25cc195/conn.go#L202), [socket permissions](https://github.com/nudanos/ifmgrd/blob/40e22ccdfd43ab7f408c9fca459625a4d25cc195/debian/ifmgrd.ifmgrd.socket#L5).

Recommendation: reject null/type mismatches, guard the dispatch boundary, and fuzz JSON RPC requests. Regression: null/scalars/arrays/wrong count/unknown methods must return errors without losing the process or interrupting interface application.

### N6 — P1 before milestone 2: preserved route-broker accepts FPM on all IPv4 addresses (confirmed dormant security defect)

`vyatta-route-broker/daemon/broker_main.c:63` zero-initializes `sockaddr_in`; `81-83` sets only family and port, so bind uses `0.0.0.0:2620`. `94-100` accepts the first peer with no authentication/address check and closes the listener. A reachable peer winning that connection can supply route messages or hold the connection. `broker_process.c:226-237` publishes received route add/delete messages. This is not installed in the current kernel-mode package set: distro manifest assigns route-broker to milestone 2; current zebra's FPM module is deliberately disabled.

Evidence: [wildcard bind/first peer](https://github.com/nudanos/vyatta-route-broker/blob/8ee9162d55e8e0e02672acfceb44f562b4b34662/daemon/broker_main.c#L63), [route dispatch](https://github.com/nudanos/vyatta-route-broker/blob/8ee9162d55e8e0e02672acfceb44f562b4b34662/daemon/broker_process.c#L226), [deferred scope](https://github.com/nudanos/distro/blob/864370dc3ff5a6086ce298b147f79ac2c4de315e/manifest.yaml#L1078).

Recommendation: use loopback-only FPM or a credential-capable local transport, explicit peer policy, bounded reads/timeouts, and reconnect/resync tests. Test external connection rejection before restoring FPM. The preserved receiver also only switches on RTM_NEWROUTE/DELROUTE; [FRR's current FPM documentation](https://docs.frrouting.org/en/stable-10.7/zebra.html#fpm-commands) documents optional RTM_NEWNEXTHOP/DELNEXTHOP. Pin and test the actual FPM mode, VRFs, ECMP/nexthop groups, MPLS, route replacement, reconnect and full resync. This latter protocol issue is a compatibility risk until the chosen FRR configuration is tested.

## Native Debian and Arch ownership requirements

The current code is an appliance-style owner of global network state. Native installation into an existing host requires a separate explicit ownership contract:

* Interfaces: udev `81-vyatta-dp-names.rules:4` renames matching NICs after `80-net-setup-link`; `dp-name:9-16` accepts PCI `enp...` patterns only, preserving other names. [Naming source](https://github.com/nudanos/vyatta-kernel-forwarding/blob/471ce7e62166d3966b6a648dcff870ebba955229/dp-name#L9). Existing native NetworkManager/networkd/ifupdown profiles can depend on the original name; onboard/USB/domain-qualified NICs need explicit coverage. Preserve native names through an adapter/mapping or make rename a per-NIC opt-in. Avoid allowing two managers to own one NIC. Acceptance: onboard, USB, multi-PCI-domain and multiport cards; install/upgrade/uninstall; hotplug; reboot; no change to excluded management NICs.
* VRFs: routing-instance invokes vrf-manager on routing begin/end (`yang/vyatta-routing-v1.yang:135-137`); vrf-manager allocates `/run/route-domain.conf`, creates `vrf<name>` masters/tables (`scripts/vrf-manager:145`), sets default unreachable routes and loopback addresses (`232-240`); interface code binds devices to those masters. It also globally changes local policy rules from pref 0 to pref 2000 (`160-168`) and does not restore them on VRF delete. This behavior can be intentional in an appliance, but native installation must detect and coordinate existing VRFs/rules/table allocations. [Kernel VRF documentation](https://docs.kernel.org/networking/vrf.html) describes the default l3mdev rule and warns that enslaving a device drops additional routes depending on it. Validate cross-VRF isolation, addresses/routes after rebinding, DNS per VRF, and restore behavior.
* Routing: zebra owns protocol-derived kernel FIB state in kernel mode; no route-broker/controller/FPM should compete. `vyatta-protocols-frr` owns FRR global daemon/config paths (`etc/frr/daemons.danos`, `debian/*.displace*`, `debian/vyatta-protocols-frr.tmpfile`) and runs `/usr/lib/frr/frr-reload.py` (`scripts/frr/parser.py:32,364-367`). Use a dedicated FRR instance/path or explicitly acquire full host FRR ownership; do not overwrite an existing native FRR installation silently. Validate reload failures, daemon restarts, package upgrades, and concurrent config rejection.
* Service integration: DNS is an active trixie port using native dnsmasq/ddclient and a renamed `vyatta-dnsmasq@.service`; VRF DNS starts via `chvrf` (`debian/vyatta-dnsmasq@.service:7-12`). That helper uses native `ip vrf exec` (`vrf-manager/scripts/chvrf:19-25`). Arch acceptance must include cgroup-v2/BPF `ip vrf exec` behavior, systemd ordering, service paths/users, DNS resolver ownership, and security-policy profiles. These are test obligations, not observed failures.

Debian appliance acceptance should keep the proven ISO/boot/routing scenarios and add negative-path/state-reconciliation/security-migration checks. Native Debian and native Arch acceptance must start from an already-networked distribution install, preserve excluded NICs, DNS and existing firewall rules, acquire only declared resources, and survive package upgrade/reboot/uninstall. Arch must be tested against a documented moving-version matrix with rebuilds on Go/protobuf/FRR/DPDK ABI changes; copying Debian binaries or simply renaming packages is insufficient.

## Deferred services / dataplane scope

* DHCP: master still depends on `isc-dhcp-server` and `isc-dhcp-client` (`debian/control:26,57-59`); distro defers it to 1.1 and no trixie branch exists. Interfaces retain `address dhcp` support but the client is only Recommended (`vyatta-interfaces/debian/changelog:9-12` says 1.0 static). Need a native backend adapter and semantics/lease/VRF tests, including Kea/client replacement decision; do not promise DHCP now.
* VPN: the trixie package deliberately builds only the VTI YANG model in 1.0. Full VPN packages are excluded by build profiles (`vyatta-security-vpn/debian/control:45,91`); runtime still needs strongSwan/VICI, python3-vplaned, xtables-addons-bypass and dataplane protobuf support (`57-78`). This is a planned 1.1 rewrite, not a present installed service. Native kernel XFRM/route/VRF and firewall integration must be specified before unmasking these models.
* DPDK/NPF/QoS: full dataplane requires fork-specific `libdpdk-dev >=20.11-2vyatta12`, extra `librte-...` development feature packages and `libvyatta-dpdk-swport-dev` (`vyatta-dataplane/debian/control:16,28-32`). NPF/QoS packages depend on versioned dataplane command virtuals (`vplane-config-npf/debian/control:15-17`, `vplane-config-qos/debian/control:17-18`), not Linux nftables/tc. Retain the current protobuf-only build isolation; model this whole pipeline as a separate provider with its own kernel/PMD/protobuf/FPM compatibility matrix and security validation. [DPDK ABI/deprecation policy](https://doc.dpdk.org/guides/rel_notes/deprecation.html) confirms that newer releases change APIs/ABIs. Do not turn off hardening for untrusted packet parsing without measured justification (`vyatta-dataplane/debian/rules:51-58` presently strips fortify/stack protector and relaxes symbol checks).
* Preserved FRR fork: default is 6.0 (`frr/debian/changelog:1`); active manifest consumes upstream FRR 10.7.1 packages. Old available branches include `danos/7.5`, `/7.5.1`, `/7.6`; none is trixie. Audit any essential custom commands/patches against current upstream rather than rebasing the complete old daemon fork. In particular `vrf-manager/scripts/rt_add_del_vrf:56-60` still emits legacy `ip vrf <name> <id> <table>` VTY commands; determine whether this path executes/is needed with modern kernel-discovered VRFs. This is a compatibility audit item, not a claim that today's successful routing paths fail.

## Pinned networking inventory

All paths below are under `C:/Users/Jon/Documents/Projects/nudanos-review/repos/`. URL pattern: `https://github.com/nudanos/<repo>/tree/<SHA>`.

| Repository | Inspected branch | HEAD |
| --- | --- | --- |
| vyatta-kernel-forwarding | trixie | 471ce7e62166d3966b6a648dcff870ebba955229 |
| nudanos-router | trixie | e45627a7eb7b70a8bcffab5757064c285e59f595 |
| vyatta-interfaces | trixie | 1e47e9e05928d6fef58f7fb512c22d941f0511ae |
| ifmgrd | trixie | 40e22ccdfd43ab7f408c9fca459625a4d25cc195 |
| vrf-manager | trixie | a9be87ae5d0ea261df034fffee9cab087f301f41 |
| routing-instance | trixie | a5982b5731bd351982f9d2f250dcec39a4e5e6db |
| vyatta-protocols-frr | trixie | 7b3c841394f68e01dba37b88053f1a20971ae4e7 |
| vyatta-route-broker | master (no trixie) | 8ee9162d55e8e0e02672acfceb44f562b4b34662 |
| vyatta-cfg-dataplane | trixie | 513f14364ce2c6f94e5255fda33b324c83828d68 |
| vplane-config-npf | master (no trixie) | 2103ac7e19ee77eacff30a3d11cf487dfbefee26 |
| vplane-config-qos | master (no trixie) | 19fdc56e29d36b92975843cb30df28541fdaf256 |
| vyatta-service-dhcp | master (no trixie) | af6e2c1d7fc45b73694a90365cfdff469385e878 |
| vyatta-service-dns | trixie | aeba9e317112e1afbbd50efbc87651dabb708def |
| vyatta-security-vpn | trixie | 40415591d0cbdd68309a22eb24ad324ebd2275e0 |
| vyatta-dataplane | trixie | 5f927126ea7f4d724f1663f823e9346baf627367 |
| frr | frr-vyatta-6.0 (preserved, not runtime) | a9d205efd8ac2490b85d4dd59ff169b7fb80aa08 |
| dpdk | master (18.11; no trixie) | 381827f42e00247c47e68073c11a266791542514 |

`git ls-remote` additionally confirmed dpdk `20.11.x` at `54a7f67eaa06957825b774b869e8a5a3fe937faa`, FRR `danos/7.5` at `415dfc8d15fb8a8c9a366c89de029f3c0d1ad8cc`, `danos/7.5.1` at `494cf46dfcc0a023523729d766318728075f08c4`, and `danos/7.6` at `8ea2235f20d5098efeffc517408d6cc2afdf6441`. Those extra branch trees were not cloned/reviewed. Future milestone refs pointing to nonexistent trixie branches are roadmap incompleteness, not a failure of the current 1.0 manifest build scope.
