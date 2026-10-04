#!/usr/bin/env python3
"""nudanos-router's dependency list: every binary in the repo except development
packages, debug symbols, the DPDK dataplane itself, and the packages
docs/kernel-forwarding.md excludes (DPDK-only, deferred, optional or
platform-specific), each with a reason. A hardware platform's deviation
module that the doc does not list is an error, not a dependency.

  router_deps.py PACKAGES_FILE KF_DOC              print the list
  router_deps.py --check CONTROL PACKAGES_FILE KF_DOC   exit 1 on drift
"""
from __future__ import annotations

import re
import sys

ALWAYS_OUT = {"vyatta-dataplane", "nudanos-router"}
SUFFIX_OUT = ("-dev", "-dbgsym", "-dbg", "-doc", "-tests", "-test")
EXCLUDING_SECTIONS = ("## DPDK-only", "## Deferred", "## Optional", "## Platform-specific")
# Deviation modules for a hardware platform (vyatta-...-deviation(s)-<platform>-v1-yang).
# DANOS's own platform deviations and kernel forwarding's are the router's.
PLATFORM_DEVIATION = re.compile(r"-deviations?-(?!danos-)[a-z0-9]+.*-v1-yang$")


def excluded(doc: str) -> set[str]:
    out, on = set(), False
    for line in doc.splitlines():
        if line.startswith("## "):
            on = line.startswith(EXCLUDING_SECTIONS)
            continue
        m = re.match(r"\|\s*([a-z0-9][a-z0-9.+-]+)\s*\|", line)
        if on and m and m.group(1) != "Package":
            out.add(m.group(1))
    return out


def router_deps(packages: str, doc: str) -> list[str]:
    names = set(re.findall(r"^Package: (\S+)$", packages, re.M))
    skip = excluded(doc) | ALWAYS_OUT
    deps = sorted(n for n in names if n not in skip and not n.endswith(SUFFIX_OUT))
    undecided = [n for n in deps if PLATFORM_DEVIATION.search(n)]
    if undecided:
        raise ValueError("hardware platform deviation modules need a decision in "
                         "docs/kernel-forwarding.md: " + ", ".join(undecided))
    return deps


def drift(control: str, packages: str, doc: str) -> tuple[list[str], list[str]]:
    m = re.search(r"^Package: nudanos-router$.*?^Depends:(.*?)(?=^\S|\Z)", control, re.M | re.S)
    have = {d.strip().split()[0] for d in (m.group(1) if m else "").split(",") if d.strip() and not d.strip().startswith("$")}
    want = set(router_deps(packages, doc))
    return sorted(want - have), sorted(have - want)


def main(argv: list[str]) -> int:
    if argv and argv[0] == "--check":
        control, packages, doc = (open(p).read() for p in argv[1:4])
        missing, extra = drift(control, packages, doc)
        for n in missing:
            print(f"missing from nudanos-router: {n}")
        for n in extra:
            print(f"in nudanos-router but not in the set: {n}")
        return 1 if missing or extra else 0
    packages, doc = (open(p).read() for p in argv[0:2])
    print("\n".join(router_deps(packages, doc)))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
