#!/usr/bin/env python3
"""Compare DANOS against a Debian release.

1. For every upstream fork: the version DANOS carried, the version in the target
   release, and how many patches DANOS layered on top.
2. For every repo: Build-Depends satisfied neither by the target release nor by
   another DANOS repo.
"""
from __future__ import annotations

import json
import lzma
import os
import re
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
TOP = os.path.join(HERE, "..")
ROOT = os.path.join(TOP, "repos")
RESEARCH = os.path.join(TOP, "research")
SUITE = sys.argv[1] if len(sys.argv) > 1 else "trixie"

sys.path.insert(0, HERE)
from depgraph import names, parse_control  # noqa: E402


def stanzas(path: str):
    with lzma.open(path, "rt", errors="replace") as f:
        cur: dict = {}
        key = None
        for line in f:
            line = line.rstrip("\n")
            if not line:
                if cur:
                    yield cur
                cur, key = {}, None
            elif line[0] in " \t" and key:
                cur[key] += " " + line.strip()
            elif ":" in line:
                key, _, val = line.partition(":")
                cur[key] = val.strip()
        if cur:
            yield cur


def load_suite() -> tuple[dict, set]:
    src_ver: dict[str, str] = {}
    binaries: set[str] = set()
    for comp in ("main", "contrib", "non-free-firmware"):
        p = os.path.join(RESEARCH, f"{SUITE}-{comp}-Sources.xz")
        if os.path.exists(p):
            for s in stanzas(p):
                src_ver[s["Package"]] = s.get("Version", "")
        p = os.path.join(RESEARCH, f"{SUITE}-{comp}-amd64-Packages.xz")
        if os.path.exists(p):
            for s in stanzas(p):
                binaries.add(s["Package"])
                binaries |= names(s.get("Provides", ""))
    return src_ver, binaries


def patch_count(repo: str) -> tuple[int, int]:
    """(total quilt patches, patches that look DANOS/Vyatta-authored)."""
    series = os.path.join(ROOT, repo, "debian", "patches", "series")
    if not os.path.exists(series):
        return 0, 0
    total = ours = 0
    for line in open(series, errors="replace"):
        line = line.split("#")[0].strip()
        if not line:
            continue
        total += 1
        body = ""
        pf = os.path.join(ROOT, repo, "debian", "patches", line.split()[0])
        if os.path.exists(pf):
            body = open(pf, errors="replace").read(3000).lower()
        if re.search(r"vyatta|danos|att\.com|brocade|vrvdr", line.lower() + body):
            ours += 1
    return total, ours


def main() -> None:
    inv = {r["repo"]: r for r in json.load(open(os.path.join(TOP, "inventory.json")))}
    dg = json.load(open(os.path.join(TOP, "depgraph.json")))
    src_ver, suite_bins = load_suite()
    danos_bins = set(dg["produces"])

    forks = []
    for repo, r in sorted(inv.items()):
        if r["kind"] != "fork":
            continue
        src = r["source"] or repo
        total, ours = patch_count(repo)
        forks.append({"repo": repo, "source": src, "danos": r["version"],
                      SUITE: src_ver.get(src, ""), "patches": total, "danos_patches": ours})

    missing = {}
    for repo in sorted(os.listdir(ROOT)):
        ctl = os.path.join(ROOT, repo, "debian", "control")
        if not os.path.exists(ctl):
            continue
        paras = parse_control(open(ctl, errors="replace").read())
        if not paras:
            continue
        src = paras[0]
        # alternatives: a|b is satisfied if any member exists
        miss = []
        for group in (src.get("Build-Depends", "") + "," + src.get("Build-Depends-Indep", "")).split(","):
            alts = names(group)
            if alts and not any(a in suite_bins or a in danos_bins for a in alts):
                miss.append("|".join(sorted(alts)))
        if miss:
            missing[repo] = miss

    json.dump({"forks": forks, "missing_build_deps": missing},
              open(os.path.join(TOP, f"{SUITE}-gap.json"), "w"), indent=1)

    print(f"{'repo':34} {'danos':30} {SUITE:28} patches(ours)")
    for f in forks:
        print(f"{f['repo']:34} {f['danos'][:30]:30} {f[SUITE][:28] or '-- not in ' + SUITE:28} {f['patches']}({f['danos_patches']})")
    print(f"\n{len(missing)} repos with build-deps missing from {SUITE} and not built by DANOS:")
    counts: dict[str, int] = {}
    for repo, miss in missing.items():
        for m in miss:
            counts[m] = counts.get(m, 0) + 1
    for m, n in sorted(counts.items(), key=lambda kv: -kv[1]):
        who = [r for r, ms in missing.items() if m in ms]
        print(f"  {n:3} {m:45} {' '.join(who[:6])}{' ...' if len(who) > 6 else ''}")


if __name__ == "__main__":
    main()
