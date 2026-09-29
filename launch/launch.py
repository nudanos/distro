#!/usr/bin/env python3
"""Publish local bare mirrors to a GitHub org.

For each repository: create it (public) if missing, push every branch and tag
(never refs/pull/*), restore the original default branch, and record progress in a
state file so an interrupted run resumes where it stopped. Repositories whose
packs exceed PUSH_LIMIT are pushed in first-parent chunks because GitHub rejects
single pushes over 2 GB. Needs an authenticated `gh` and SSH access to github.com.
"""
from __future__ import annotations

import argparse
import json
import os
import subprocess
import sys
import time

PUSH_LIMIT = 1_500_000_000
CHUNK = 5000
TAG_BATCH = 300


def description(meta: dict, source_org: str) -> str:
    base = f"Preserved fork of {source_org}/{meta['name']} (DANOS, dormant since 2021). Part of NuDanOS."
    orig = (meta.get("description") or "").strip()
    return (f"{base} Original: {orig}" if orig else base)[:350]


def push_plan(heads: list[str], tags: list[str], big: bool,
              first_parent: dict[str, list[str]]) -> list[list[str]]:
    """Successive refspec batches for `git push`. first_parent maps each head to its
    first-parent commits, oldest first; it is only consulted when big is true."""
    if not big:
        spec = []
        if heads:
            spec.append("+refs/heads/*:refs/heads/*")
        if tags:
            spec.append("refs/tags/*:refs/tags/*")
        return [spec] if spec else []
    batches = []
    for head in heads:
        commits = first_parent[head]
        for i in range(CHUNK - 1, len(commits) - 1, CHUNK):
            batches.append([f"{commits[i]}:{head}"])
        batches.append([f"{head}:{head}"])
    for i in range(0, len(tags), TAG_BATCH):
        batches.append([f"{t}:{t}" for t in tags[i:i + TAG_BATCH]])
    return batches


def pending(names: list[str], state: dict[str, str], only: list[str] | None) -> list[str]:
    wanted = set(only) if only else set(names)
    return sorted(n for n in names if n in wanted and state.get(n) != "done")


def run(cmd: list[str], check: bool = True) -> subprocess.CompletedProcess:
    return subprocess.run(cmd, check=check, capture_output=True, text=True)


def retry(cmd: list[str], attempts: int = 4) -> subprocess.CompletedProcess:
    for i in range(attempts):
        proc = run(cmd, check=False)
        if proc.returncode == 0:
            return proc
        if i == attempts - 1:
            raise RuntimeError(f"{' '.join(cmd[:4])}... failed:\n{proc.stderr.strip()}")
        time.sleep(10 * 2 ** i)
    raise AssertionError("unreachable")


def refs(mirror: str) -> tuple[list[str], list[str]]:
    out = run(["git", "-C", mirror, "for-each-ref", "--format=%(refname)",
               "refs/heads", "refs/tags"]).stdout.split()
    return ([r for r in out if r.startswith("refs/heads/")],
            [r for r in out if r.startswith("refs/tags/")])


def pack_bytes(mirror: str) -> int:
    pack = os.path.join(mirror, "objects", "pack")
    if not os.path.isdir(pack):
        return 0
    return sum(os.path.getsize(os.path.join(pack, f)) for f in os.listdir(pack))


def save_state(path: str, state: dict[str, str]) -> None:
    tmp = path + ".tmp"
    with open(tmp, "w") as f:
        json.dump(state, f, indent=1, sort_keys=True)
    os.replace(tmp, path)


def remote_ref_count(org: str, name: str) -> int:
    out = run(["git", "ls-remote", "--heads", "--tags", f"git@github.com:{org}/{name}.git"]).stdout
    return sum(1 for line in out.splitlines() if line and not line.endswith("^{}"))


def main(argv: list[str] | None = None) -> int:
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("--mirrors", required=True, help="directory holding NAME.git bare mirrors")
    ap.add_argument("--metadata", required=True, help="JSON list of {name, description, default_branch}")
    ap.add_argument("--org", default="nudanos")
    ap.add_argument("--source-org", default="danos")
    ap.add_argument("--state", default="launch-state.json")
    ap.add_argument("--only", nargs="*")
    ap.add_argument("--dry-run", action="store_true")
    ap.add_argument("--verify", action="store_true", help="compare local and remote ref counts only")
    a = ap.parse_args(argv)

    meta = {m["name"]: m for m in json.load(open(a.metadata))}
    state = json.load(open(a.state)) if os.path.exists(a.state) else {}

    if a.verify:
        bad = 0
        for name in sorted(meta):
            if a.only and name not in a.only:
                continue
            heads, tags = refs(os.path.join(a.mirrors, name + ".git"))
            remote = remote_ref_count(a.org, name)
            ok = remote == len(heads) + len(tags)
            bad += not ok
            print(f"{'ok  ' if ok else 'DIFF'} {name}: local {len(heads) + len(tags)} remote {remote}")
        return 1 if bad else 0

    for name in pending(sorted(meta), state, a.only):
        mirror = os.path.join(a.mirrors, name + ".git")
        heads, tags = refs(mirror)
        big = pack_bytes(mirror) > PUSH_LIMIT
        fp = {h: run(["git", "-C", mirror, "rev-list", "--first-parent", "--reverse", h]).stdout.split()
              for h in heads} if big else {}
        batches = push_plan(heads, tags, big, fp)
        exists = run(["gh", "repo", "view", f"{a.org}/{name}", "--json", "name"], check=False).returncode == 0
        print(f"{name}: {'exists' if exists else 'create'}, {len(heads)} branches, {len(tags)} tags, "
              f"{len(batches)} push(es){' [big]' if big else ''}", flush=True)
        if a.dry_run:
            continue
        try:
            if not exists:
                retry(["gh", "repo", "create", f"{a.org}/{name}", "--public", "--disable-wiki",
                       "--description", description(meta[name], a.source_org)])
            url = f"git@github.com:{a.org}/{name}.git"
            for batch in batches:
                retry(["git", "-C", mirror, "push", "--porcelain", url, *batch])
            default = meta[name].get("default_branch")
            if heads and default:
                retry(["gh", "repo", "edit", f"{a.org}/{name}", "--default-branch", default])
            state[name] = "done"
        except RuntimeError as err:
            state[name] = "failed"
            print(f"  FAILED: {err}", file=sys.stderr)
        save_state(a.state, state)
        time.sleep(1)
    failed = [n for n, s in state.items() if s == "failed"]
    if failed:
        print(f"{len(failed)} failed: {' '.join(sorted(failed))}; re-run to retry", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
