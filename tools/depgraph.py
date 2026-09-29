#!/usr/bin/env python3
"""Map debian/control across repos: binary packages each repo produces, and which
other repos its Build-Depends pull in. Prints build tiers (topological levels)."""
from __future__ import annotations

import json
import os
import re
import sys

ROOT = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "repos")


def parse_control(text: str) -> list[dict]:
    paras, cur, key = [], {}, None
    for line in text.splitlines():
        if not line.strip():
            if cur:
                paras.append(cur)
            cur, key = {}, None
        elif line[0] in " \t" and key:
            cur[key] += " " + line.strip()
        elif ":" in line and not line.startswith("#"):
            key, _, val = line.partition(":")
            key = key.strip()
            cur[key] = val.strip()
    if cur:
        paras.append(cur)
    return paras


def names(field: str) -> set[str]:
    out = set()
    for part in re.split(r"[,|]", field):
        part = re.sub(r"\(.*?\)|\[.*?\]|<.*?>", "", part).strip()
        if part and not part.startswith("$"):
            out.add(part.split(":")[0])
    return out


def main() -> None:
    produces: dict[str, str] = {}
    bdeps: dict[str, set[str]] = {}
    for repo in sorted(os.listdir(ROOT)):
        path = os.path.join(ROOT, repo, "debian", "control")
        if not os.path.exists(path):
            continue
        paras = parse_control(open(path, errors="replace").read())
        if not paras:
            continue
        src = paras[0]
        bdeps[repo] = names(src.get("Build-Depends", "") + "," + src.get("Build-Depends-Indep", ""))
        for p in paras[1:]:
            for n in names(p.get("Package", "")):
                produces[n] = repo
            for n in names(p.get("Provides", "")):
                produces.setdefault(n, repo)
    edges = {r: sorted({produces[d] for d in ds if d in produces and produces[d] != r})
             for r, ds in bdeps.items()}
    # topological tiers
    tiers, done, remaining = [], set(), set(edges)
    while remaining:
        tier = sorted(r for r in remaining if set(edges[r]) <= done)
        if not tier:  # cycle
            tiers.append(["CYCLE:"] + sorted(remaining))
            break
        tiers.append(tier)
        done |= set(tier)
        remaining -= set(tier)
    json.dump({"produces": produces, "edges": edges, "tiers": tiers},
              open(sys.argv[1] if len(sys.argv) > 1 else "depgraph.json", "w"), indent=1)
    print(f"{len(produces)} binary packages from {len(bdeps)} source repos")
    for i, t in enumerate(tiers):
        print(f"tier {i} ({len(t)}): {' '.join(t)}")


if __name__ == "__main__":
    main()
