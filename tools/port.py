#!/usr/bin/env python3
"""Apply the NuDanOS Debian 13 port checklist (spec §6) to one checkout.

Edits debian/control, debian/rules and debian/changelog in place and removes
debian/compat. Prints NOTE: lines for anything a human must check. Running it
again on a ported checkout changes nothing.
"""
from __future__ import annotations

import argparse
import email.utils
import os
import re
import sys

MAINTAINER = "NuDanOS Maintainers <jon@fernandez.tech>"
STANDARDS = "4.7.2"
PORT_LINE = "Port to Debian 13 (trixie): debhelper-compat 13, Standards-Version 4.7.2, Rules-Requires-Root, NuDanOS Vcs."
DROP_DEPS = {"dh-systemd", "python3-pytest-pep8", "python3-pep8", "autotools-dev"}
RENAME_DEPS = {"pylint3": "pylint", "python-setuptools": "python3-setuptools",
               "bvnos-linux-libc-dev": "linux-libc-dev", "bvnos-linux-libc-dev-vyatta": "linux-libc-dev"}


def paragraphs(text: str) -> list[list[str]]:
    paras, cur = [], []
    for line in text.rstrip("\n").split("\n"):
        if line.strip() == "":
            if cur:
                paras.append(cur)
            cur = []
        else:
            cur.append(line)
    if cur:
        paras.append(cur)
    return paras


def fields(para: list[str]) -> list[tuple[str, str]]:
    """[(name, value-with-continuations)] preserving order."""
    out: list[tuple[str, str]] = []
    for line in para:
        if line[:1] in (" ", "\t") and out:
            out[-1] = (out[-1][0], out[-1][1] + "\n" + line)
        elif ":" in line:
            k, v = line.split(":", 1)
            out.append((k.strip(), v.strip()))
    return out


def dep_name(dep: str) -> str:
    return re.split(r"[\s(\[<:]", dep.strip(), 1)[0]


def port_deps(value: str, notes: list[str]) -> str:
    deps = [d.strip() for d in value.replace("\n", " ").split(",") if d.strip()]
    out = ["debhelper-compat (= 13)"]
    for d in deps:
        n = dep_name(d)
        if n in ("debhelper", "debhelper-compat") or n in DROP_DEPS:
            continue
        if n in RENAME_DEPS:
            if n.startswith("bvnos"):
                notes.append("NOTE: bvnos-linux-libc-dev replaced by linux-libc-dev; check the DANOS kernel headers it used")
            d = RENAME_DEPS[n]
        if d not in out:
            out.append(d)
    return ",\n ".join(out)


def fix_section(v: str) -> str:
    return v.split("/", 1)[1] if v.startswith(("contrib/", "non-free/")) else v


def port_control(text: str, repo: str) -> tuple[str, list[str]]:
    notes: list[str] = []
    paras = paragraphs(text)
    src = []
    for k, v in fields(paras[0]):
        kl = k.lower()  # deb822 field names are case-insensitive (libvci: "Build-depends")
        if kl in ("uploaders", "dm-upload-allowed") or kl.startswith(("vcs-", "xs-vcs-")):
            continue
        if kl == "build-depends":
            k = "Build-Depends"
        if k == "Maintainer":
            v = MAINTAINER
        elif k == "Standards-Version":
            v = STANDARDS
        elif k == "Section":
            v = fix_section(v)
        elif k == "Priority" and v == "extra":
            v = "optional"
        elif k == "Build-Depends":
            v = port_deps(v, notes)
        src.append((k, v))
    names = [k.lower() for k, _ in src]
    if "build-depends" not in names:
        src.append(("Build-Depends", "debhelper-compat (= 13)"))
    if "rules-requires-root" not in names:
        src.append(("Rules-Requires-Root", "no"))
    src.append(("Vcs-Git", f"https://github.com/nudanos/{repo}.git"))
    src.append(("Vcs-Browser", f"https://github.com/nudanos/{repo}"))
    out = ["\n".join(f"{k}: {v}" for k, v in src)]
    for p in paras[1:]:
        kept = []
        for k, v in fields(p):
            if k == "Priority" and v in ("extra", "optional"):
                continue
            if k == "Section":
                v = fix_section(v)
            kept.append(f"{k}: {v}")
        out.append("\n".join(kept))
    bd = dict(fields(out[0].split("\n"))).get("Build-Depends", "")
    if re.search(r"(?<![\w-])python(?:-all|-dev|-all-dev)?(?=\s*(?:[,(\[<]|$))", bd.replace("\n", " ")):
        notes.append("NOTE: Build-Depends still names a Python 2 package")
    return "\n\n".join(out) + "\n", notes


def port_rules(text: str) -> str:
    """Drop obsolete dh addons; put GOPATH-style Go builds in GOPATH mode.

    DANOS Go packages run `go vet` from custom targets with GOPATH set; Go 1.26
    defaults to module mode ("go: cannot find main module") unless GO111MODULE=off.
    """
    def fix(m: re.Match) -> str:
        addons = [a for a in m.group(2).split(",") if a and a not in ("systemd", "autotools_dev", "autotools-dev")]
        return f" --with {','.join(addons)}" if addons else ""
    text = re.sub(r" --with(=| )([A-Za-z0-9_,-]+)", fix, text)
    text = re.sub(r" --parallel\b", "", text)
    if re.search(r"--buildsystem[= ]golang|--with[= ][^\n]*\bgolang\b", text) and "GO111MODULE" not in text:
        lines = text.split("\n")
        at = 1 if lines and lines[0].startswith("#!") else 0
        lines.insert(at, "export GO111MODULE := off")
        text = "\n".join(lines)
    return text


def bump_version(v: str, native: bool = False) -> str:
    if "-" in v and native:
        # A native version may not carry a Debian revision.
        return v.rsplit("-", 1)[0] + "+nudanos1"
    if "-" in v:
        return v + "+nudanos1"
    m = re.match(r"^(.*?)(\d+)$", v)
    if not m:
        return v + "+nudanos1"
    return m.group(1) + str(int(m.group(2)) + 1).zfill(len(m.group(2)))


def new_changelog(text: str, version: str, date: str) -> str:
    src = text.split(" ", 1)[0]
    entry = f"{src} ({version}) trixie; urgency=medium\n\n  * {PORT_LINE}\n\n -- {MAINTAINER}  {date}\n\n"
    return entry + text


def port_tree(d: str, repo: str, date: str | None = None) -> list[str]:
    deb = os.path.join(d, "debian")
    notes: list[str] = []
    ctl = os.path.join(deb, "control")
    text, notes = port_control(open(ctl).read(), repo)
    open(ctl, "w").write(text)
    compat = os.path.join(deb, "compat")
    if os.path.exists(compat):
        os.remove(compat)
    rules = os.path.join(deb, "rules")
    if os.path.exists(rules):
        new = port_rules(open(rules).read())  # read fully before truncating for write
        open(rules, "w").write(new)
    cl = os.path.join(deb, "changelog")
    old = open(cl).read()
    fmt = os.path.join(deb, "source", "format")
    quilt = os.path.exists(fmt) and "quilt" in open(fmt).read()
    if PORT_LINE not in old.split("\n -- ", 1)[0]:
        top = re.match(r"^\S+ \(([^)]+)\)", old).group(1)
        # 3.0 (native) and 1.0 without an orig tarball both build as native.
        open(cl, "w").write(new_changelog(old, bump_version(top, native=not quilt),
                                          date or email.utils.formatdate(localtime=True)))
    if quilt:
        notes.append("NOTE: 3.0 (quilt): the builder generates the orig tarball from the tree minus debian/")
    return notes


def main(argv: list[str] | None = None) -> int:
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("dir")
    ap.add_argument("--repo", help="GitHub repo name (default: directory name without port- prefix)")
    a = ap.parse_args(argv)
    repo = a.repo or os.path.basename(os.path.abspath(a.dir)).removeprefix("port-")
    for n in port_tree(a.dir, repo):
        print(n)
    return 0


if __name__ == "__main__":
    sys.exit(main())
