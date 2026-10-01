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
               "bvnos-linux-libc-dev": "linux-libc-dev", "bvnos-linux-libc-dev-vyatta": "linux-libc-dev",
               "python3-pytest-lazy-fixture": "python3-pytest-lazy-fixtures"}


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


START_FLAGS = {"--no-start", "--no-restart-on-upgrade", "--restart-after-upgrade", "--no-stop-on-upgrade",
               "--no-restart-after-upgrade"}
VALUE_OPTS = {"-p", "--package", "-N", "--no-package", "--name"}


def _make_blocks(text: str) -> list[tuple[str, list[str], int, int]]:
    """(target, joined recipe commands, start line, end line) for each rule in a makefile."""
    lines = text.split("\n")
    blocks = []
    i = 0
    while i < len(lines):
        m = re.match(r"^([A-Za-z0-9_.%-]+):", lines[i])
        if not m:
            i += 1
            continue
        start, cmds, cur = i, [], ""
        i += 1
        while i < len(lines) and lines[i].startswith("\t"):
            part = lines[i].strip()
            cur += (" " if cur else "") + part.rstrip("\\").strip()
            if not part.endswith("\\"):
                cmds.append(cur)
                cur = ""
            i += 1
        if cur:  # a recipe ending in a dangling "\" continuation
            cmds.append(cur)
        blocks.append((m.group(1), cmds, start, i))
    return blocks


def _with_flags(cmd: str, flags: list[str]) -> str:
    toks = cmd.split()[1:]
    out, pos, expect_value = [], None, False
    for t in toks:
        if expect_value:
            expect_value = False
        elif t.startswith("-"):
            expect_value = t in VALUE_OPTS
        elif pos is None:
            pos = len(out)
        out.append(t)
    flags = [f for f in flags if f not in out]
    out = out[:pos] + flags + out[pos:] if pos is not None else out + flags
    return " ".join(["dh_installsystemd"] + out)


def _split_systemd_cmd(cmd: str) -> tuple[list[str], list[str]]:
    """(selection tokens: -p/--name/units and other options, start flags) of a dh_systemd_* command."""
    sel, flags, take = [], [], False
    for t in cmd.split()[1:]:
        if take:
            sel.append(t)
            take = False
        elif t in START_FLAGS:
            flags.append(t)
        else:
            sel.append(t)
            take = t in VALUE_OPTS
    return sel, flags


def _selects(sel: list[str]) -> bool:
    """Whether a command names packages or units (rather than acting on all)."""
    return any(t in ("-p", "--package", "--name") or t.startswith(("-p", "--package=", "--name=")) or not t.startswith("-")
               for t in sel)


def _dedupe(tokens: list[str]) -> list[str]:
    out = []
    for t in tokens:
        if t not in out or not t.startswith("--"):
            out.append(t)
    return out


def port_systemd_overrides(text: str, notes: list[str] | None = None) -> str:
    """Rewrite override_dh_systemd_{enable,start} (removed in compat 11) as override_dh_installsystemd.

    The old overrides replaced the default: an empty one did nothing, and a start
    override started only the units it named. So each enable line keeps the start
    flags of the start command naming the same units, and gets --no-start when no
    start command named them. A start override that started some units with no
    enable override needs the package's full unit list to say which stay stopped;
    that is left as is with a NOTE (dh then refuses the old target, so the build
    fails loudly instead of starting units DANOS kept stopped). Other commands in
    the overrides are kept, with a NOTE.
    """
    notes = notes if notes is not None else []
    blocks = {t: (c, a, b) for t, c, a, b in _make_blocks(text) if t in ("override_dh_systemd_enable", "override_dh_systemd_start")}
    if not blocks:
        return text
    enable = blocks["override_dh_systemd_enable"][0] if "override_dh_systemd_enable" in blocks else None
    start = blocks["override_dh_systemd_start"][0] if "override_dh_systemd_start" in blocks else None
    foreign = [c for c in (enable or []) + (start or []) if not c.startswith(("dh_systemd_enable", "dh_systemd_start"))]
    for c in foreign:
        notes.append(f"NOTE: kept '{c}' from a dh_systemd_* override in override_dh_installsystemd; check it")
    global_flags, selective = [], []
    for c in (start or []):
        if not c.startswith("dh_systemd_start"):
            continue
        sel, flags = _split_systemd_cmd(c)
        if "--no-start" in flags or not _selects(sel):
            global_flags += [f for f in flags if f not in global_flags]
        else:
            selective.append((sel, flags))
    if start is not None and "--no-start" not in global_flags and (selective or not any(c.startswith("dh_systemd_start") for c in start)):
        rest = global_flags + ["--no-start"]  # units no start command named were not started
    else:
        rest = global_flags
    if enable is None and selective:
        named = ", ".join(" ".join(sel) for sel, _ in selective)
        notes.append(f"NOTE: override_dh_systemd_start started only {named}; write override_dh_installsystemd by hand, "
                     f"giving every other unit of the package(s) --no-start")
        return text
    new = []
    enable_cmds = [c for c in (enable or []) if c.startswith("dh_systemd_enable")]
    matched = set()
    for c in enable_cmds:
        sel, _ = _split_systemd_cmd(c)
        hit = next((k for k, (s2, _) in enumerate(selective) if s2 == sel), None)
        if hit is not None:
            matched.add(hit)
            new.append(_with_flags(c, [f for f in global_flags if f != "--no-start"] + selective[hit][1]))
        else:
            new.append(_with_flags(c, rest))
    for k, (sel, flags) in enumerate(selective):
        if k not in matched:  # started though the enable override did not enable it
            new.append(" ".join(["dh_installsystemd"] + _dedupe(sel + ["--no-enable"] + flags)))
    if enable is None or not enable_cmds:
        extra = ["--no-enable"] if enable is not None else []
        new.append(" ".join(["dh_installsystemd"] + _dedupe(extra + rest)))
    new += foreign
    block = "override_dh_installsystemd:\n" + "".join(f"\t{c}\n" for c in new)
    lines = text.split("\n")
    spans = sorted((a, b) for _, a, b in blocks.values())
    first = spans[0][0]
    for a, b in reversed(spans):
        del lines[a:b]
    lines.insert(first, block.rstrip("\n"))
    return "\n".join(lines)


def port_rules(text: str, notes: list[str] | None = None) -> str:
    """Drop obsolete dh addons; put GOPATH-style Go builds in GOPATH mode.

    DANOS Go packages run `go vet` from custom targets with GOPATH set; Go 1.26
    defaults to module mode ("go: cannot find main module") unless GO111MODULE=off.
    A rules file that exports GOPATH (libvci's Makefile build) is in the same position.
    """
    def fix(m: re.Match) -> str:
        addons = [a for a in m.group(2).split(",") if a and a not in ("systemd", "autotools_dev", "autotools-dev")]
        return f" --with {','.join(addons)}" if addons else ""
    text = port_systemd_overrides(text, notes)
    text = re.sub(r" --with(=| )([A-Za-z0-9_,-]+)", fix, text)
    text = re.sub(r" --parallel\b", "", text)
    gopath = r"--buildsystem[= ]golang|--with[= ][^\n]*\bgolang\b|^export GOPATH\b"
    if re.search(gopath, text, re.M) and "GO111MODULE" not in text:
        lines = text.split("\n")
        at = 1 if lines and lines[0].startswith("#!") else 0
        lines.insert(at, "export GO111MODULE := off")
        text = "\n".join(lines)
    return text


_ALIASED = re.compile(r"^/?(lib|bin|sbin)(/|$)")


def _usr(path: str) -> str:
    m = _ALIASED.match(path)
    if not m:
        return path
    return ("/usr/" if path.startswith("/") else "usr/") + path.lstrip("/")


def port_install(text: str) -> tuple[str, list[str]]:
    """Move two-column .install destinations out of /lib, /bin, /sbin (merged-usr).

    Single-column entries name files in debian/tmp; under /lib they need the
    upstream install fixed, so they are only noted.
    """
    notes, out = [], []
    for line in text.split("\n"):
        toks = line.split()
        if len(toks) >= 2 and not line.lstrip().startswith("#"):
            dest = _usr(toks[-1])
            if dest != toks[-1]:
                line = line[: line.rstrip().rfind(toks[-1])] + dest
        elif len(toks) == 1 and _ALIASED.match(toks[0]):
            notes.append(f"NOTE: .install entry {toks[0]} comes from debian/tmp under /lib|/bin|/sbin; fix the upstream install path")
        out.append(line)
    return "\n".join(out), notes


def port_links(text: str) -> str:
    """Move .links and .dirs paths out of /lib, /bin, /sbin (merged-usr)."""
    return "\n".join(" ".join(_usr(t) for t in line.split()) if line.strip() and not line.lstrip().startswith("#")
                     else line for line in text.split("\n"))


def port_transform(text: str) -> str:
    """Escape ${N} backreferences in config-package-dev transforms (debhelper compat 13 expands ${...})."""
    return re.sub(r"\$\{(\d+)\}", r"${Dollar}{\1}", text)


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
        new = port_rules(open(rules).read(), notes)  # read fully before truncating for write
        open(rules, "w").write(new)
    for f in sorted(os.listdir(deb)):
        path = os.path.join(deb, f)
        if f == "install" or f.endswith(".install"):
            new, n = port_install(open(path).read())
            notes += n
            open(path, "w").write(new)
        elif f.endswith(".transform"):
            new = port_transform(open(path).read())  # read fully before truncating for write
            open(path, "w").write(new)
        elif f in ("links", "dirs") or f.endswith((".links", ".dirs")):
            new = port_links(open(path).read())
            open(path, "w").write(new)
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
