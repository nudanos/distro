#!/usr/bin/env python3
"""Inventory every cloned DANOS repo: origin, packaging, languages, legacy markers."""
from __future__ import annotations

import json
import os
import re
import sys
from collections import Counter

ROOT = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "repos")

EXT_LANG = {
    ".c": "C", ".h": "C", ".cc": "C++", ".cpp": "C++", ".hpp": "C++", ".hh": "C++",
    ".go": "Go", ".py": "Python", ".pl": "Perl", ".pm": "Perl", ".sh": "Shell",
    ".yang": "YANG", ".robot": "Robot", ".proto": "Protobuf", ".rb": "Ruby",
}
SKIP_DIRS = {".git", "vendor", "node_modules", "debian", "doc", "docs"}
# version suffixes that mark a Debian/upstream package re-packaged by Vyatta/DANOS
FORK_VER = re.compile(r"(vyatta|danos|deb\d+u|\+b\d|-\d+(\.\d+)*vyatta|~bpo|\+dfsg)", re.I)


def changelog(path: str) -> tuple[str, str, str]:
    try:
        with open(os.path.join(path, "debian", "changelog"), errors="replace") as f:
            text = f.read(200000)
    except OSError:
        return "", "", ""
    m = re.match(r"(\S+) \(([^)]+)\) ([^;]+);", text)
    d = re.search(r"^ -- .*?  (.+)$", text, re.M)
    return (m.group(1) if m else "", m.group(2) if m else "", d.group(1).strip() if d else "")


def read(path: str) -> str:
    try:
        with open(path, errors="replace") as f:
            return f.read()
    except OSError:
        return ""


def scan(repo: str) -> dict:
    path = os.path.join(ROOT, repo)
    src, ver, date = changelog(path)
    control = read(os.path.join(path, "debian", "control"))
    compat = read(os.path.join(path, "debian", "compat")).strip()
    m = re.search(r"debhelper-compat \(= (\d+)\)", control)
    if m and not compat:
        compat = m.group(1)
    std = re.search(r"Standards-Version:\s*(\S+)", control)
    lines: Counter = Counter()
    py2 = 0
    py_files = 0
    go_mod = os.path.exists(os.path.join(path, "go.mod"))
    files = 0
    tests = 0
    for dp, dns, fns in os.walk(path):
        dns[:] = [d for d in dns if d not in SKIP_DIRS]
        for fn in fns:
            files += 1
            if files > 60000:
                break
            ext = os.path.splitext(fn)[1]
            fp = os.path.join(dp, fn)
            if "test" in fn.lower() or "/test" in dp.lower():
                tests += 1
            lang = EXT_LANG.get(ext)
            if not lang and not ext:
                head = read(fp)[:80] if os.path.getsize(fp) < 2_000_000 else ""
                if head.startswith("#!"):
                    if "perl" in head: lang = "Perl"
                    elif "python" in head: lang = "Python"
                    elif "sh" in head.split("\n")[0]: lang = "Shell"
            if not lang:
                continue
            try:
                if os.path.getsize(fp) > 3_000_000:
                    continue
                body = read(fp)
            except OSError:
                continue
            lines[lang] += body.count("\n")
            if lang == "Python":
                py_files += 1
                first = body.split("\n", 1)[0]
                if (re.match(r"#!.*python2?\s*$", first) and "python3" not in first) or \
                        re.search(r"^\s*print [\"'a-zA-Z]", body, re.M) or "iteritems()" in body:
                    py2 += 1
    kind = "fork" if FORK_VER.search(ver) else "original"
    return {
        "repo": repo, "source": src, "version": ver, "last_changelog": date,
        "kind": kind, "compat": compat, "standards": std.group(1) if std else "",
        "lines": dict(lines), "total_lines": sum(lines.values()),
        "py_files": py_files, "py2_suspect": py2, "go_mod": go_mod,
        "jenkins": os.path.exists(os.path.join(path, "Jenkinsfile")), "test_files": tests,
    }


def main() -> None:
    repos = sorted(d for d in os.listdir(ROOT) if os.path.isdir(os.path.join(ROOT, d, ".git")))
    out = [scan(r) for r in repos]
    json.dump(out, open(sys.argv[1] if len(sys.argv) > 1 else "inventory.json", "w"), indent=1)
    print(len(out), "repos scanned")


if __name__ == "__main__":
    main()
