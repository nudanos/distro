#!/bin/bash
# SPDX-License-Identifier: GPL-2.0-only
# Run as root in a disposable Linux VM. All links, forwarding and firewall
# changes are confined to a new network and mount namespace.
set -euo pipefail
if [[ ${1:-} != --isolated ]]; then
    exec unshare --mount --net --propagation private /bin/bash "$0" --isolated
fi
root=$(cd "$(dirname "$0")/.." && pwd)
mount -t tmpfs tmpfs /run
tmp=$(mktemp -d)
trap 'ip netns del left; ip netns del right; rm -f "$tmp/config.boot"; rmdir "$tmp"' EXIT

ip netns add left
ip netns add right
ip link add left-r type veth peer name host netns left
ip link add right-r type veth peer name host netns right
ip addr add 192.0.2.1/24 dev left-r
ip addr add 198.51.100.1/24 dev right-r
ip -6 addr add 2001:db8:1::1/64 dev left-r nodad
ip -6 addr add 2001:db8:2::1/64 dev right-r nodad
ip link set left-r up
ip link set right-r up
ip -n left addr add 192.0.2.2/24 dev host
ip -n right addr add 198.51.100.2/24 dev host
ip -n left -6 addr add 2001:db8:1::2/64 dev host nodad
ip -n right -6 addr add 2001:db8:2::2/64 dev host nodad
ip -n left link set host up
ip -n right link set host up
ip -n left route add default via 192.0.2.1
ip -n right route add default via 198.51.100.1
ip -n left -6 route add default via 2001:db8:1::1
ip -n right -6 route add default via 2001:db8:2::1
sysctl -qw net.ipv4.ip_forward=1 net.ipv6.conf.all.forwarding=1

probe() { ip netns exec left ping -c 2 -W 2 "$1" >/dev/null; }
probe 198.51.100.2
probe 2001:db8:2::2
nft add table inet existing_policy

cp "$root/tests/set-aside/firewall.boot" "$tmp/config.boot"
export SET_ASIDE_PATHS="$root/dpdk-only-paths"
export POLICY_GUARD_HOOK="$root/boot-config.d/50-dpdk-set-aside"
guard() {
    local rc=0
    sh "$root/policy-guard" "$tmp/config.boot" || rc=$?
    test "$rc" = 78
}
guard
guard # applying twice must be safe
cmp "$tmp/config.boot" "$root/tests/set-aside/firewall.boot"
nft list table inet existing_policy >/dev/null
nft list table bridge nudanos_migration_guard >/dev/null
probe 192.0.2.1 # local traffic (including recovery) is not filtered
probe 2001:db8:1::1
if probe 198.51.100.2 || probe 2001:db8:2::2; then
    echo 'transit remained open after migration refusal' >&2
    exit 1
fi
echo 'policy-guard: IPv4 and IPv6 transit blocked; local traffic and existing rules preserved'
