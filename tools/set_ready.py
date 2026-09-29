#!/usr/bin/env python3
"""Set `ready: true` on named manifest.yaml entries (idempotent)."""
from __future__ import annotations

import re
import sys


def set_ready(text: str, names: list[str]) -> str:
    head, *entries = text.split("\n  - ")
    seen = set()
    for i, e in enumerate(entries):
        m = re.match(r'name: "?([^"\n]+)"?', e)
        if not m or m.group(1) not in names:
            continue
        seen.add(m.group(1))
        if re.search(r"^    ready: true$", e, re.M):
            continue
        entries[i] = re.sub(r"^(    milestone: [^\n]*)$", r"\1\n    ready: true", e, count=1, flags=re.M)
    missing = sorted(set(names) - seen)
    if missing:
        sys.exit(f"not in manifest: {' '.join(missing)}")
    return "\n  - ".join([head, *entries])


def main() -> int:
    path, names = sys.argv[1], sys.argv[2:]
    text = open(path).read()
    open(path, "w").write(set_ready(text, names))
    return 0


if __name__ == "__main__":
    sys.exit(main())
