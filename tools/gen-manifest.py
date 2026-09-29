#!/usr/bin/env python3
"""Generate manifest.yaml from the review data (docs/data) and spec §4.3.

Hand edits to manifest.yaml are expected after generation (ready flags, audit
notes); re-run only to rebuild from scratch, then review the diff.
"""
from __future__ import annotations

import json
import os
import sys

TOP = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..")
ORG = "https://github.com/nudanos"
PILOT = {"dh-yang", "dh-vci", "vyatta-util"}

SALSA = "https://salsa.debian.org/debian"
UPSTREAM = {  # spec §4.3 Tier 1; packaging URLs are the Vcs-Git fields of the trixie sources
    "strongswan": ("https://github.com/strongswan/strongswan", f"{SALSA}/strongswan.git", "1.1"),
    "keepalived": ("https://github.com/acassen/keepalived", f"{SALSA}/pkg-keepalived.git", "1.0"),
    "libteam": ("https://github.com/jpirko/libteam", f"{SALSA}/libteam.git", "1.0"),
    "net-snmp": ("https://github.com/net-snmp/net-snmp", f"{SALSA}/net-snmp.git", "1.0"),
    "ntp": ("https://gitlab.com/NTPsec/ntpsec", f"{SALSA}/ntpsec.git", "1.0"),
    "owamp": ("https://github.com/perfsonar/owamp", f"{ORG}/owamp", "1.0"),
    "i2util": ("https://github.com/perfsonar/i2util", f"{ORG}/i2util", "1.0"),
    "pam_tacplus": ("https://github.com/kravietz/pam_tacplus", f"{ORG}/pam_tacplus", "1.0"),
    "mstpd": ("https://github.com/mstpd/mstpd", f"{ORG}/mstpd", "1.0"),
    "libyang": ("https://github.com/CESNET/libyang", "https://forgejo.debian.net/frr/libyang", "1.1"),
}
DEBIAN = {  # fork archived, Debian 13 package used; value is a note
    "check": "", "cloud-init": "audit 5 DANOS patches", "dh-golang": "", "golang": "trixie-backports Go 1.26",
    "golang-defaults": "trixie-backports", "grub": "", "shim": "", "iperf": "", "iputils": "audit 1 patch",
    "jitterentropy": "", "libpcap": "audit 1 patch", "libvirt": "", "linux-firmware": "Debian firmware-* packages",
    "makedumpfile": "", "openssh": "", "pygobject": "", "ppp": "audit 1 patch", "radvd": "audit 1 patch",
    "rdma-core": "", "rsyslog": "", "smartmontools": "audit 1 patch", "sssd": "audit 5 patches", "valgrind": "",
    "netkit-telnet": "replaced by Debian telnetd",
    "linux-vyatta": "Debian kernel (trixie-backports); audit 88 vendor commits",
    "isc-dhcp": "server -> Kea, client -> dhcpcd (M1.1); relay stays on Debian isc-dhcp-relay",
}
GO_LIBS = {"golang-dbus", "golang-github-jsouthworth-objtree", "golang-github-zeromq-goczmq",
           "golang-github-mdlayher-netlink", "golang-github-mdlayher-genetlink", "golang-github-josharian-native",
           "golang-golang-x-sys", "golang-github-youmark-pkcs8"}
M2 = {"dpdk", "dpdk-kmods", "ndpi", "vermont", "host-sflow", "libzmq-libzmq3-perl", "libzmq-constants-perl",
      "vyatta-controller", "vyatta-route-broker", "vplane-config-npf", "vplane-config-qos",
      "vplane-config-npf-alg-scripts", "vyatta-service-dpi", "vyatta-dpdk-swport", "vyatta-cfg-sflow",
      "vyatta-service-export", "vyatta-cpu-shield", "vyatta-debug"}
DROP = {"bcm-kbp-linux-modules": "Broadcom hardware", "bcm-linux-bde-modules": "Broadcom hardware",
        "ufispace-apollo-linux-modules": "switch hardware", "ufispace-bsp-utils": "switch hardware",
        "opennsl-binary": "missing Broadcom blob", "libfal-opennsl": "needs OpenNSL",
        "accton-hwdiag": "switch hardware", "vyatta-hwdiag": "switch hardware", "vyatta-poe": "switch hardware",
        "vyatta-ipmi": "switch hardware", "fluent-bit": "unused",
        "pytest-lazy-fixture": "tests adapted instead", "grub2-signed": "until Secure Boot",
        "libnetconf": "replaced by libnetconf2 (M1.1)", "pyang": "replaced by libnetconf2 (M1.1)"}
LATER = {"libre": "PCP service deferred (re 1.1 -> 4.x)", "repcpd": "PCP service deferred",
         "vyatta-service-pcp": "PCP service deferred"}
REWRITE_1_1 = {"vyatta-service-dhcp", "netconfd", "vyatta-security-vpn", "vyatta-ipsec-trapd"}
NEW_UPSTREAM = [  # Tier 1 packages DANOS never carried (spec §4.2, §5)
    {"name": "libnetconf2", "kind": "upstream", "milestone": "1.1", "upstream": "https://github.com/CESNET/libnetconf2",
     "track": "latest", "packaging": "https://forgejo.debian.net/frr/libnetconf2", "note": "replaces libnetconf 0.10"},
    {"name": "kea", "kind": "upstream", "milestone": "1.1", "upstream": "https://gitlab.isc.org/isc-projects/kea",
     "track": "latest", "packaging": "https://salsa.debian.org/debian/isc-kea.git",
     "note": "replaces the ISC DHCP server; Debian source name isc-kea"},
]
REFERENCE = {"build-iso": "moved to distro/image", "tests": "moved to distro/tests",
             "live-build-desc": "OBS hooks", "danos-service-flowstat": "no packaging",
             "vyatta-fs-monitor": "no packaging", "vci-template-go": "template",
             "foobartest": "empty", "iproute2": "empty", "dataplane-flowstat-plugin": "empty",
             "vyatta-dataplane-flow": "empty"}


def closure(roots: set[str], edges: dict[str, list[str]]) -> set[str]:
    seen, stack = set(), list(roots)
    while stack:
        n = stack.pop()
        if n in seen:
            continue
        seen.add(n)
        stack.extend(edges.get(n, []))
    return seen


def classify(name: str, data: dict) -> dict | None:
    if name == "frr":
        return None  # the DANOS fork is preserved at nudanos/frr; the manifest entry is the apt mirror
    m1 = closure(set(data["m1"]) | PILOT | {"vyatta-dataplane"}, data["edges"])
    e: dict = {"name": name}
    if name in UPSTREAM:
        up, pkg, ms = UPSTREAM[name]
        e.update(kind="upstream", milestone=ms, upstream=up, track="latest", packaging=pkg)
    elif name == "netplug":
        e.update(kind="upstream", milestone="1.0", upstream="debian-source:netplug", track="debian",
                 packaging="debian-source:netplug", note="Debian has no Vcs; carry 12 DANOS patches")
    elif name in DEBIAN:
        e.update(kind="debian", milestone="1.0")
        if DEBIAN[name]:
            e["note"] = DEBIAN[name]
    elif name in DROP:
        e.update(kind="drop", milestone="none", note=DROP[name])
    elif name in REFERENCE:
        e.update(kind="reference", milestone="none", note=REFERENCE[name])
    else:
        e.update(kind="danos", repo=f"{ORG}/{name}", ref="trixie")
        if name in M2:
            e["milestone"] = "2"
        elif name in LATER:
            e.update(milestone="later", note=LATER[name])
        elif name in REWRITE_1_1:
            e["milestone"] = "1.1"
        elif name in GO_LIBS or name in m1 or name == "vyatta-bash":
            e["milestone"] = "1.0"
        else:
            e["milestone"] = "later"
        if name == "vyatta-dataplane":
            e["note"] = "1.0 builds the pkg.vyatta-dataplane.protobuf-only profile; full dataplane is M2"
        if name in GO_LIBS:
            e["note"] = "vendored Go module dependency from M1.1"
        if name in PILOT:
            e["ready"] = True
    return e


def q(v: str) -> str:
    return json.dumps(v)  # JSON strings are valid YAML double-quoted scalars


def emit(entries: list[dict]) -> str:
    order = ["name", "kind", "milestone", "ready", "repo", "ref", "upstream", "track", "packaging",
             "patches", "source", "key", "packages", "note"]
    lines = ["# Generated by tools/gen-manifest.py from docs/data and spec §4.3; then edited by hand.",
             "packages:"]
    for e in entries:
        first = True
        for k in order:
            if k not in e:
                continue
            prefix = "  - " if first else "    "
            first = False
            v = e[k]
            if isinstance(v, bool):
                lines.append(f"{prefix}{k}: {'true' if v else 'false'}")
            elif isinstance(v, dict):
                lines.append(f"{prefix}{k}:")
                lines.extend(f"      {pk}: {q(pv)}" for pk, pv in sorted(v.items()))
            else:
                lines.append(f"{prefix}{k}: {q(v)}")
    return "\n".join(lines) + "\n"


FRR = {"name": "frr", "kind": "apt", "milestone": "1.0", "ready": True,
       "source": "https://deb.frrouting.org/frr trixie frr-stable",
       "key": "https://deb.frrouting.org/frr/keys.gpg",
       "packages": {"frr": "10.7.1-0~deb13u1", "frr-pythontools": "10.7.1-0~deb13u1",
                    "frr-snmp": "10.7.1-0~deb13u1", "libyang3": "3.13.6-1~deb13u1"},
       "note": "FRR's own trixie packages; audit the 27 DANOS commits against 10.7"}


def main() -> int:
    d = os.path.join(TOP, "docs", "data")
    inv = {r["repo"]: r for r in json.load(open(os.path.join(d, "inventory.json")))}
    data = {"inventory": inv,
            "m1": set(json.load(open(os.path.join(d, "m1-scope.json")))["m1_repos"]),
            "edges": json.load(open(os.path.join(d, "depgraph.json")))["edges"]}
    names = sorted(m["name"] for m in json.load(open(os.path.join(d, "github-metadata.json"))))
    entries = [e for e in (classify(n, data) for n in names) if e is not None]
    entries.append({"name": "vci-dhcpv6-pd", "kind": "danos", "milestone": "later",
                    "repo": f"{ORG}/vci-dhcpv6-pd", "ref": "trixie",
                    "note": "from jsouthworth; DHCPv6 prefix delegation component"})
    entries.append(FRR)
    entries.extend(NEW_UPSTREAM)
    entries.sort(key=lambda e: e["name"])
    sys.stdout.write(emit(entries))
    return 0


if __name__ == "__main__":
    sys.exit(main())
