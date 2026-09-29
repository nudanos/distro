#!/usr/bin/env python3
"""Work out the milestone-1 package set: everything the ISO installs, minus the
DPDK dataplane stack, plus whatever those packages need to build and run.

Reports which ISO packages runtime-depend on the dataplane stack (they either
need that dependency relaxed, or move to milestone 2)."""
from __future__ import annotations

import glob
import json
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
TOP = os.path.join(HERE, "..")
ROOT = os.path.join(TOP, "repos")
sys.path.insert(0, HERE)
from depgraph import names, parse_control  # noqa: E402

DATAPLANE = {
    "dpdk", "dpdk-kmods", "vyatta-dataplane", "vyatta-dpdk-swport", "vyatta-controller",
    "vyatta-route-broker", "vplane-config-npf", "vplane-config-qos", "vplane-config-npf-alg-scripts",
    "ndpi", "vyatta-service-dpi", "vermont", "host-sflow", "libfal-opennsl", "opennsl-binary",
}
# package lists that only make sense with the DPDK dataplane or on switch hardware
DATAPLANE_LISTS = {
    "vyatta-dataplane", "vyatta-debug-dataplane", "vyatta-yang-dpi", "vyatta-yang-ndpi",
    "vyatta-yang-user-dpi", "vyatta-policy-qos", "vyatta-yang-flowmon", "vyatta-yang-sflow",
    "danos-switch-opennsl", "vyatta-yang-port-monitor", "vyatta-poe", "vyatta-systemtap",
    "vyatta-ipmi", "vyatta-yang-bmc", "bootloaders-signed.list.chroot", "danos-no-dataplane",
}
RUNTIME_FIELDS = ("Depends", "Pre-Depends", "Recommends")


def main() -> None:
    produces: dict[str, str] = {}
    runtime: dict[str, set[str]] = {}
    for ctl in glob.glob(os.path.join(ROOT, "*", "debian", "control")):
        repo = ctl.split(os.sep)[-3]
        paras = parse_control(open(ctl, errors="replace").read())
        for p in paras[1:]:
            for n in names(p.get("Package", "")):
                produces[n] = repo
                runtime[n] = set().union(*(names(p.get(f, "")) for f in RUNTIME_FIELDS))
            for n in names(p.get("Provides", "")):
                produces.setdefault(n, repo)

    lists_dir = os.path.join(ROOT, "build-iso", "config", "package-lists")
    iso_pkgs, m1_pkgs = set(), set()
    for fn in os.listdir(lists_dir):
        pk = {l.strip() for l in open(os.path.join(lists_dir, fn)) if l.strip() and not l.startswith("#")}
        iso_pkgs |= pk
        if fn not in DATAPLANE_LISTS:
            m1_pkgs |= pk

    # runtime closure over DANOS-built packages
    seen, stack, blocked = set(), [p for p in m1_pkgs if p in produces], {}
    while stack:
        p = stack.pop()
        if p in seen:
            continue
        seen.add(p)
        for d in runtime.get(p, ()):
            if d not in produces:
                continue
            if produces[d] in DATAPLANE:
                blocked.setdefault(p, set()).add(d)
                continue
            stack.append(d)
    m1_repos = sorted({produces[p] for p in seen})

    out = {"iso_packages": len(iso_pkgs), "m1_packages": len(m1_pkgs),
           "m1_repos": m1_repos, "blocked_by_dataplane": {k: sorted(v) for k, v in sorted(blocked.items())},
           "unknown_packages": sorted(p for p in m1_pkgs if p not in produces)}
    json.dump(out, open(os.path.join(TOP, "m1-scope.json"), "w"), indent=1)
    print(f"ISO lists: {len(iso_pkgs)} packages; milestone-1 lists: {len(m1_pkgs)}")
    print(f"milestone-1 runtime closure: {len(seen)} DANOS binaries from {len(m1_repos)} repos")
    print(f"\nruntime deps on the dataplane stack ({len(blocked)}):")
    for k, v in sorted(blocked.items()):
        print(f"  {k:55} -> {', '.join(sorted(v))}")
    print(f"\nlisted packages not built by DANOS (expected from Debian): {len(out['unknown_packages'])}")
    print("  " + " ".join(out["unknown_packages"]))


if __name__ == "__main__":
    main()
