# NuDanOS Stage 0 + Build Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Publish all DANOS history to the `nudanos` GitHub org, and stand up the `distro` repo with a working `distro-build` pipeline. The pipeline has to prove itself end to end: three ported DANOS packages plus mirrored FRR 10.7.1, built on Debian 13 in containers, published as a signed apt repo, and installed in a clean Debian 13 container, all running in CI.

**Architecture:** A Python 3.9 stdlib script publishes the local bare mirrors to GitHub through `gh` and SSH. `distro-build` is a Go CLI:
- It reads `manifest.yaml` and clones `danos` entries.
- It mirrors pinned `apt` entries, works out build order from `debian/control`, and builds each package in a fresh `debian:trixie` container driven through the docker/podman CLI.
- It caches builds by source-tree hash and signs the result with `apt-ftparchive` and GnuPG.

**Tech Stack:** Go 1.26+ (module `github.com/nudanos/distro`, one dependency: `go.yaml.in/yaml/v3`), Python 3.9 stdlib, Docker or Podman, Debian 13 tooling (dpkg-dev, debhelper 13.24, lintian 2.122, apt 3.0), GitHub Actions, `gh` 2.101.0.

**Spec:** `docs/superpowers/specs/2026-09-28-debian13-revival-design.md`. Data this plan relies on is in `REVIEW.md`, `inventory.json`, `depgraph.json`, `m1-scope.json`, `trixie-gap.json` and `github-metadata.json`, all in `~/Documents/Claude/Projects/danOS Project/`.

**Scope note:** The spec's checkpoint 1.0 is split across three plans. This one (stage 0 plus the build foundation) comes first. The next two will be written once this lands, because they depend on real build results:
- **Plan 2:** port the milestone-1 repos in tier order, add `upstream` builds and `check-updates`, and archive the `debian`/`drop` repos.
- **Plan 3:** the ISO, boot tests, scenarios, the 2105 fixtures and GitHub Pages hosting.

## Global Constraints

- GitHub org login `nudanos` (lowercase), display name NuDanOS. Go module paths are `github.com/nudanos/<repo>`.
- Original branches and tags are never rewritten. Porting happens on a new `trixie` branch.
- Push only `refs/heads/*` and `refs/tags/*` to GitHub, never `refs/pull/*`. No single push may exceed GitHub's 2 GB limit.
- `jsouthworth/danos-bootstrap` has **no license**. Do not republish it and do not copy its code. `danos-buildpackage` and `danos-buildimage` are MIT: derived code must carry the attribution in `NOTICE`.
- Builder base image: `debian:trixie`. Go toolchain for package builds: `golang-go` from `trixie-backports` (Go 1.26).
- Debian packaging on `trixie` branches: `debhelper-compat (= 13)`, `Standards-Version: 4.7.2`, `Rules-Requires-Root: no`, `Maintainer: NuDanOS Maintainers <jon@fernandez.tech>` (**confirm this address with the user at plan review**), `Vcs-Git: https://github.com/nudanos/<repo>.git`, `Vcs-Browser: https://github.com/nudanos/<repo>`.
- Package builds run their test suites. A test failure fails the build. `lintian --fail-on error` gates every build.
- The work directory must be on a case-sensitive filesystem.
- The container engine is selectable (`-engine docker|podman`). Never shell out to anything but `git`, the engine, and (in `launch.py`) `gh`.
- Anything that publishes (creating repos, pushing, setting secrets, enabling Pages) needs the user's explicit go-ahead at that step. Secrets are set by the user, never typed by the agent.
- Commits use DCO sign-off (`git commit -s`, as DANOS required) and end with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

## Review Focus

1. **A work dir on macOS's default, case-insensitive filesystem**: `distro-build` must refuse with a clear message before cloning anything, not produce silently corrupted checkouts. (Task 10)
2. **Re-running `launch.py` after a partial failure** (network drop, rate limit) must resume. It must not recreate repos or re-push repos already marked done, and must retry the one that failed. (Task 2)
3. **One package fails to build**: packages that depend on it are reported as skipped, naming the failed dependency. Unrelated packages still build. The run exits non-zero. (Task 10)
4. **Stale cache**: editing a source file or changing the builder image definition forces a rebuild. Git metadata churn (`.git/`) must not. (Task 10)
5. **Real-world `Build-Depends` syntax**: alternatives, version constraints, `[arch]`, `<!nocheck>` profiles, `:any` qualifiers, `${substvars}`, multi-line fields. A mis-parse silently drops an edge and builds in the wrong order. (Task 5)

---

## File Structure

Launch tooling (lives in `distro`, runs on the Mac against the local mirrors):

| File | Responsibility |
|---|---|
| `launch/launch.py` | Create repos in the org, push mirrors in ≤1.5 GB batches, restore default branches, resume from state, verify |
| `launch/test_launch.py` | Unit tests for the pure planning functions |
| `tools/gen-manifest.py` | Generate `manifest.yaml` from the review data and the spec §4.3 dispositions |
| `tools/test_gen_manifest.py` | Unit tests for classification and YAML emission |
| `tools/new-archive-key.sh` | Create the archive signing key in a local GnuPG home |

`distro-build` (Go, module `github.com/nudanos/distro`):

| File | Responsibility |
|---|---|
| `cmd/distro-build/main.go` | Flags, subcommands (`builder`, `fetch`, `plan`, `build`, `repo`), wiring |
| `internal/control/control.go` | Parse deb822 `debian/control`, relationship fields, source packages |
| `internal/manifest/manifest.go` | Load and strictly validate `manifest.yaml` |
| `internal/plan/plan.go` | Build-order graph and tiers |
| `internal/engine/engine.go` | docker/podman CLI argument construction and execution |
| `internal/fetch/fetch.go` | Clone or update a git repo at a branch or tag |
| `internal/workspace/workspace.go` | Case-sensitivity check for the work dir |
| `internal/build/hash.go` | Source-tree hash (excludes `.git`) |
| `internal/build/build.go` | Ordered builds, cache, skip-on-failed-dependency, state file |
| `internal/aptmirror/aptmirror.go` | Mirror pinned packages from third-party apt repos (FRR) |
| `internal/aptrepo/aptrepo.go` | Container spec for publishing the signed repo |
| `builder/Dockerfile`, `builder/preferences` | The Debian 13 builder image |
| `builder/build-package.sh` | In-container build of one source package (derived from danos-buildpackage, MIT) |
| `builder/mirror-apt.sh` | In-container download of pinned binaries and sources |
| `builder/make-repo.sh` | In-container generation and signing of the apt repo |
| `tests/integration/pilot.sh` | End-to-end: build, publish, install in a clean Debian 13 container |
| `manifest.yaml` | Generated package list |
| `.github/workflows/ci.yml` | Go and Python tests for `distro` itself |
| `.github/workflows/package.yml` | Reusable: build one package repo's PR against the manifest |
| `.github/workflows/nightly.yml` | Pilot build, sign, integration test, repo artifact |
| `NOTICE`, `README.md`, `docs/` | Attribution, overview, spec, plan, review, workspace setup |

Workspace layout on the Mac:
- `/Volumes/nudanos/distro`: the `distro` git repo.
- `/Volumes/nudanos/work/{src,out,repo}`, `state.json`, `mirror-state.json`: build workspace.
- `~/Documents/Claude/Projects/danOS Project/{mirrors,mirrors-jsouthworth}`: bare mirrors (untouched).

---

### Task 1: Workspace and toolchain

**Files:**
- Create: `~/nudanos-work.sparsebundle` (disk image), mounted at `/Volumes/nudanos`
- Create: `~/.local/go` (Go toolchain), `~/.local/bin/gh`

**Interfaces:**
- Produces: `/Volumes/nudanos` (case-sensitive), working `go`, `gh` (authenticated as an owner of `nudanos`), `docker` (or `podman`), and a git identity. Every later task assumes these.

- [ ] **Step 1: Create and mount a case-sensitive volume** (up to 80 GB, allocated on demand; the Mac has ~97 GB free)

```bash
hdiutil create -type SPARSEBUNDLE -fs "Case-sensitive APFS" -size 80g -volname nudanos ~/nudanos-work.sparsebundle
hdiutil attach ~/nudanos-work.sparsebundle
mkdir -p /Volumes/nudanos/work
```

- [ ] **Step 2: Verify case sensitivity**

Run: `cd /Volumes/nudanos && touch a A && ls | grep -c '^[aA]$'; rm -f a A`
Expected: `2`

- [ ] **Step 3: Install Go 1.27.1 into `~/.local/go`** (official tarball; SHA-256 from go.dev)

```bash
mkdir -p ~/.local/bin && cd /tmp
curl -fsSLO https://go.dev/dl/go1.27.1.darwin-amd64.tar.gz
echo "8f8f52c6649542cf027bbc9b9c68d1ec042f9f34808a40413f0b8b3f66f3caa4  go1.27.1.darwin-amd64.tar.gz" | shasum -a 256 -c
rm -rf ~/.local/go && tar -C ~/.local -xzf go1.27.1.darwin-amd64.tar.gz
grep -q '.local/go/bin' ~/.zshrc || echo 'export PATH="$HOME/.local/go/bin:$HOME/.local/bin:$HOME/go/bin:$PATH"' >> ~/.zshrc
export PATH="$HOME/.local/go/bin:$HOME/.local/bin:$HOME/go/bin:$PATH"
```

Run: `go version`
Expected: `go version go1.27.1 darwin/amd64`

- [ ] **Step 4: Install `gh` 2.101.0 into `~/.local/bin`**

```bash
cd /tmp && curl -fsSLO https://github.com/cli/cli/releases/download/v2.101.0/gh_2.101.0_macOS_amd64.zip
echo "a6fd66c88e2f07d6e4e058173db341d07dd74d58cf8f19ae668293d2bb614ca3  gh_2.101.0_macOS_amd64.zip" | shasum -a 256 -c
unzip -o -q gh_2.101.0_macOS_amd64.zip && cp gh_2.101.0_macOS_amd64/bin/gh ~/.local/bin/gh
```

Run: `gh --version | head -1`
Expected: `gh version 2.101.0 (...)`

- [ ] **Step 5: USER ACTION: authenticate `gh`.** The user runs `gh auth login` in their own terminal: GitHub.com, SSH, browser login, scopes `repo`, `workflow`, `admin:org`. The agent must not handle the token.

Run: `gh auth status && gh api orgs/nudanos/memberships/$(gh api user --jq .login) --jq .role`
Expected: logged in, and `admin`

- [ ] **Step 6: USER ACTION: container engine.** Install Docker Desktop, then add `/Volumes/nudanos` under Settings → Resources → File sharing. Alternative: a Debian 13 VM with `apt install docker.io`, running the pipeline inside the VM.

Run: `docker version --format '{{.Server.Version}}' && docker run --rm -v /Volumes/nudanos:/v debian:trixie ls /v`
Expected: a server version, then `work`

- [ ] **Step 7: Git identity.** Ask the user what `user.name` to use; `user.email` is already `jon@fernandez.tech`.

```bash
git config --global user.name "<name the user gives>"
```

Run: `git config --global user.name && git config --global user.email`
Expected: both print non-empty values

---

### Task 2: Launch tool

**Files:**
- Create: `/Volumes/nudanos/distro/launch/launch.py`
- Test: `/Volumes/nudanos/distro/launch/test_launch.py`

**Interfaces:**
- Produces: CLI `python3 launch/launch.py --mirrors DIR --metadata FILE [--org nudanos] [--source-org danos] [--state FILE] [--only NAME...] [--dry-run] [--verify]`. Pure functions: `description(meta, source_org) -> str`, `push_plan(heads, tags, big, first_parent) -> list[list[str]]`, `pending(names, state, only) -> list[str]`.

- [ ] **Step 1: Create the repo directory and write the failing tests**

```bash
mkdir -p /Volumes/nudanos/distro/launch && cd /Volumes/nudanos/distro && git init -b main
```

`launch/test_launch.py`:
```python
from __future__ import annotations

import unittest

import launch


class DescriptionTest(unittest.TestCase):
    def test_credits_source_and_keeps_original(self):
        d = launch.description({"name": "configd", "description": "YANG config daemon"}, "danos")
        self.assertTrue(d.startswith("Preserved fork of danos/configd (DANOS, dormant since 2021)."))
        self.assertIn("Original: YANG config daemon", d)

    def test_empty_original_and_length_cap(self):
        self.assertNotIn("Original", launch.description({"name": "x", "description": None}, "danos"))
        self.assertLessEqual(len(launch.description({"name": "x", "description": "y" * 999}, "danos")), 350)


class PushPlanTest(unittest.TestCase):
    def test_small_repo_is_one_push_of_heads_and_tags(self):
        plan = launch.push_plan(["refs/heads/master"], ["refs/tags/v1"], False, {})
        self.assertEqual(plan, [["+refs/heads/*:refs/heads/*", "refs/tags/*:refs/tags/*"]])

    def test_small_repo_without_tags(self):
        self.assertEqual(launch.push_plan(["refs/heads/m"], [], False, {}), [["+refs/heads/*:refs/heads/*"]])

    def test_empty_repo_has_nothing_to_push(self):
        self.assertEqual(launch.push_plan([], [], False, {}), [])

    def test_big_repo_pushes_history_in_chunks_then_tags_in_batches(self):
        commits = [f"c{i}" for i in range(12000)]
        tags = [f"refs/tags/t{i}" for i in range(650)]
        plan = launch.push_plan(["refs/heads/main"], tags, True, {"refs/heads/main": commits})
        self.assertEqual(plan[0], ["c4999:refs/heads/main"])
        self.assertEqual(plan[1], ["c9999:refs/heads/main"])
        self.assertEqual(plan[2], ["refs/heads/main:refs/heads/main"])
        self.assertEqual([len(b) for b in plan[3:]], [300, 300, 50])
        self.assertNotIn("refs/pull", repr(plan))


class PendingTest(unittest.TestCase):
    def test_resume_skips_done_and_retries_failed(self):
        state = {"a": "done", "b": "failed"}
        self.assertEqual(launch.pending(["c", "a", "b"], state, None), ["b", "c"])

    def test_only_filter(self):
        self.assertEqual(launch.pending(["a", "b", "c"], {}, ["c", "a"]), ["a", "c"])


if __name__ == "__main__":
    unittest.main()
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd /Volumes/nudanos/distro/launch && python3 -m unittest -v test_launch`
Expected: FAIL with `ModuleNotFoundError: No module named 'launch'`

- [ ] **Step 3: Write `launch/launch.py`**

```python
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
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd /Volumes/nudanos/distro/launch && python3 -m unittest -v test_launch`
Expected: 8 tests, `OK`

- [ ] **Step 5: Dry run against the real mirrors** (read-only on GitHub: only `gh repo view`)

```bash
cd /Volumes/nudanos/distro
P="$HOME/Documents/Claude/Projects/danOS Project"
python3 launch/launch.py --mirrors "$P/mirrors" --metadata "$P/github-metadata.json" --state /Volumes/nudanos/launch-state.json --dry-run | tee /tmp/launch-dry.txt
grep -c ': create,' /tmp/launch-dry.txt; grep '\[big\]' /tmp/launch-dry.txt
```
Expected: `189`, and exactly one `[big]` line (`linux-vyatta`, with more than 4 pushes)

- [ ] **Step 6: Commit**

```bash
git add launch/ && git commit -s -m "launch: publish danos mirrors to the nudanos org

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: Launch: publish the history (outward-facing, needs go-ahead)

**Files:**
- Create: `/Volumes/nudanos/jsouthworth-metadata.json`, `/Volumes/nudanos/launch-state.json`, `/Volumes/nudanos/launch-state-js.json`

**Interfaces:**
- Consumes: `launch.py` (Task 2), `gh` auth (Task 1).
- Produces: `github.com/nudanos/<name>` for all 189 danos repos, plus `danos-buildpackage`, `danos-buildimage` and `vci-dhcpv6-pd` from jsouthworth. Every ref is identical to the local mirror.

- [ ] **Step 1: STOP and get explicit go-ahead from the user.** Show them the dry-run summary from Task 2 Step 5. This creates 192 public repos and uploads ~4.5 GB.

- [ ] **Step 2: Publish the danos mirrors** (expect 1–3 hours; the kernel is the long pole; safe to re-run)

```bash
cd /Volumes/nudanos/distro
P="$HOME/Documents/Claude/Projects/danOS Project"
python3 launch/launch.py --mirrors "$P/mirrors" --metadata "$P/github-metadata.json" --state /Volumes/nudanos/launch-state.json 2>&1 | tee /Volumes/nudanos/launch.log
```
Expected: exit 0. If any repo failed, re-run the same command until the exit code is 0.

- [ ] **Step 3: Verify every ref arrived**

Run: `P="$HOME/Documents/Claude/Projects/danOS Project"; python3 launch/launch.py --mirrors "$P/mirrors" --metadata "$P/github-metadata.json" --verify | grep -c '^ok'`
Expected: `189`

- [ ] **Step 4: Publish the three licensed jsouthworth repos** (never `danos-bootstrap`, which is unlicensed)

```bash
J="$HOME/Documents/Claude/Projects/danOS Project/mirrors-jsouthworth"
python3 - "$J" > /Volumes/nudanos/jsouthworth-metadata.json <<'EOF'
import json, subprocess, sys
out = []
for n in ["danos-buildpackage", "danos-buildimage", "vci-dhcpv6-pd"]:
    head = subprocess.run(["git", "-C", f"{sys.argv[1]}/{n}.git", "symbolic-ref", "--short", "HEAD"],
                          capture_output=True, text=True, check=True).stdout.strip()
    out.append({"name": n, "description": "", "default_branch": head})
print(json.dumps(out, indent=1))
EOF
python3 launch/launch.py --mirrors "$J" --metadata /Volumes/nudanos/jsouthworth-metadata.json --source-org jsouthworth --state /Volumes/nudanos/launch-state-js.json
python3 launch/launch.py --mirrors "$J" --metadata /Volumes/nudanos/jsouthworth-metadata.json --source-org jsouthworth --verify
```
Expected: three `ok` lines

- [ ] **Step 5: Spot-check on GitHub**

Run: `gh repo view nudanos/vyatta-dataplane --json defaultBranchRef,description --jq '.defaultBranchRef.name, .description' && git ls-remote git@github.com:nudanos/dpdk.git refs/heads/20.11.x`
Expected: `master`, the "Preserved fork of danos/vyatta-dataplane ..." description, and one line for `20.11.x`

---

### Task 4: `distro` repo scaffold

**Files:**
- Create: `/Volumes/nudanos/distro/go.mod`, `README.md`, `NOTICE`, `.gitignore`, `docs/workspace.md`
- Create (copied): `docs/superpowers/specs/2026-09-28-debian13-revival-design.md`, `docs/superpowers/plans/2026-09-28-nudanos-stage0-build-foundation.md`, `docs/REVIEW.md`, `docs/data/{inventory,depgraph,m1-scope,trixie-gap,github-metadata}.json`, `tools/{inventory,depgraph,trixie_gap,m1_scope}.py`
- Create: `.github/workflows/ci.yml`

**Interfaces:**
- Produces: module `github.com/nudanos/distro` (Go 1.26), the repo `nudanos/distro` on GitHub, and CI that runs `go test ./...` and both Python test suites.

- [ ] **Step 1: Module, docs and data**

```bash
cd /Volumes/nudanos/distro
P="$HOME/Documents/Claude/Projects/danOS Project"
go mod init github.com/nudanos/distro && go mod edit -go=1.26
mkdir -p docs/superpowers/specs docs/superpowers/plans docs/data tools
cp "$P/docs/superpowers/specs/2026-09-28-debian13-revival-design.md" docs/superpowers/specs/
cp "$P/docs/superpowers/plans/2026-09-28-nudanos-stage0-build-foundation.md" docs/superpowers/plans/
cp "$P/REVIEW.md" docs/REVIEW.md
cp "$P"/{inventory,depgraph,m1-scope,trixie-gap,github-metadata}.json docs/data/
cp "$P"/tools/{inventory,depgraph,trixie_gap,m1_scope}.py tools/
printf 'work/\n*.tmp\n/distro-build\n__pycache__/\n' > .gitignore
```

- [ ] **Step 2: Write `NOTICE`**

```text
NuDanOS distro
Copyright (c) 2026 NuDanOS contributors

NuDanOS continues DANOS (https://github.com/danos), released by AT&T under the
LGPL-2.1, MPL-2.0, MIT and BSD licenses recorded in each repository.

builder/build-package.sh and the container build approach in internal/build are
derived from danos-buildpackage (https://github.com/jsouthworth/danos-buildpackage):

  MIT License
  Copyright (c) 2019 John Southworth

  Permission is hereby granted, free of charge, to any person obtaining a copy of
  this software and associated documentation files (the "Software"), to deal in
  the Software without restriction, including without limitation the rights to
  use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
  the Software, and to permit persons to whom the Software is furnished to do so,
  subject to the following conditions:

  The above copyright notice and this permission notice shall be included in all
  copies or substantial portions of the Software.

  THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
  IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
  FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
  COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
  IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
  CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
```

- [ ] **Step 3: Write `README.md` and `docs/workspace.md`**

`README.md`:
````markdown
# NuDanOS distro

NuDanOS revives [DANOS](https://github.com/danos), the open-source router OS AT&T
released in 2019 and stopped maintaining in 2021, on Debian 13 "trixie".

This repository holds the package manifest, the `distro-build` tool, the builder
image, image definitions and CI. Each package lives in its own repository under
[github.com/nudanos](https://github.com/nudanos); porting happens on `trixie`
branches, and the original DANOS branches and tags are preserved unchanged.

- Design: [docs/superpowers/specs/2026-09-28-debian13-revival-design.md](docs/superpowers/specs/2026-09-28-debian13-revival-design.md)
- Code review and preservation record: [docs/REVIEW.md](docs/REVIEW.md)
- Local setup: [docs/workspace.md](docs/workspace.md)

## Build

```bash
go build -o distro-build ./cmd/distro-build
./distro-build -work /Volumes/nudanos/work builder   # build the Debian 13 builder image
./distro-build -work /Volumes/nudanos/work plan      # show build order
./distro-build -work /Volumes/nudanos/work build     # build every ready package
./distro-build -work /Volumes/nudanos/work -key <fingerprint> repo   # signed apt repo in work/repo
```

Contributions use the Developer Certificate of Origin: sign off commits with `git commit -s`.
````

`docs/workspace.md`:
````markdown
# Workspace setup

DANOS sources contain file names that differ only by case, and macOS volumes are
case-insensitive by default. `distro-build` refuses to run in such a directory.

On macOS, create a case-sensitive sparse volume once:

```bash
hdiutil create -type SPARSEBUNDLE -fs "Case-sensitive APFS" -size 80g -volname nudanos ~/nudanos-work.sparsebundle
hdiutil attach ~/nudanos-work.sparsebundle   # after each reboot
```

With Docker Desktop, add `/Volumes/nudanos` under Settings → Resources → File sharing.
On Linux (including a Debian 13 VM), any ext4/xfs directory works.

Requirements: Go 1.26+, git, and Docker or Podman (`-engine podman`).
````

- [ ] **Step 4: Write `.github/workflows/ci.yml`**

```yaml
name: ci
on:
  push:
    branches: [main]
  pull_request:
permissions:
  contents: read
jobs:
  test:
    runs-on: ubuntu-24.04
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - run: go vet ./...
      - run: go test ./...
      - run: python3 -m unittest discover -s launch -v
```
(The tools tests step is added in Task 8. Python 3.12 exits with status 5 when a discovery run finds no tests.)

- [ ] **Step 5: Verify the scaffold**

Run: `cd /Volumes/nudanos/distro && go vet ./... && python3 -m unittest discover -s launch`
Expected: `go vet` warns `matched no packages` and exits 0; the Python tests are `OK`

- [ ] **Step 6: Commit, and create and push `nudanos/distro`** (covered by the Task 3 go-ahead; confirm again if this runs in a later session)

```bash
git add -A && git commit -s -m "distro: scaffold, docs, review data, CI

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
gh repo create nudanos/distro --public --description "NuDanOS: manifest, build tooling and images for the DANOS revival on Debian 13" --source . --remote origin --push
```
Expected: `https://github.com/nudanos/distro`. The `ci` workflow is green in `gh run list -R nudanos/distro -L 1`

---

### Task 5: `debian/control` parser

**Files:**
- Create: `internal/control/control.go`
- Test: `internal/control/control_test.go`

**Interfaces:**
- Produces:
  - `type Paragraph map[string]string`
  - `func Parse(text string) []Paragraph`
  - `func Relations(field string) [][]string` (AND-groups of OR-alternatives, names only)
  - `type Source struct { Name string; BuildDepends [][]string; Binaries []string; Provides []string }`
  - `func ParseSource(text string) (*Source, error)`
  - `func ReadSource(path string) (*Source, error)`

- [ ] **Step 1: Write the failing tests**

```go
package control

import (
	"reflect"
	"testing"
)

const sample = `# leading comment
Source: vyatta-util
Section: net
Build-Depends: debhelper-compat (= 13),
 liburiparser-dev (>= 0.9) [amd64 arm64],
 check <!nocheck>,
 python3:any | python3-minimal,
 ${misc:Depends}
Build-Depends-Indep: dh-yang

Package: vyatta-util
Architecture: any
Depends: libvyatta-util1 (= ${binary:Version}), ${shlibs:Depends}
Description: utilities
 Multi-line description
 continues here.

Package: libvyatta-util1
Provides: libvyatta-util (= 1.0), vyatta-validate
`

func TestParseJoinsContinuationsAndSkipsComments(t *testing.T) {
	ps := Parse(sample)
	if len(ps) != 3 {
		t.Fatalf("got %d paragraphs, want 3", len(ps))
	}
	if ps[0]["Source"] != "vyatta-util" {
		t.Errorf("Source = %q", ps[0]["Source"])
	}
	if got := ps[1]["Description"]; got != "utilities Multi-line description continues here." {
		t.Errorf("Description = %q", got)
	}
}

func TestRelationsStripsEverythingButNames(t *testing.T) {
	got := Relations("debhelper-compat (= 13), libfoo-dev (>= 1.0) [amd64] <!nocheck>, a | b:any, ${misc:Depends},, check <!nocheck>")
	want := [][]string{{"debhelper-compat"}, {"libfoo-dev"}, {"a", "b"}, {"check"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Relations = %v, want %v", got, want)
	}
}

func TestParseSource(t *testing.T) {
	s, err := ParseSource(sample)
	if err != nil {
		t.Fatal(err)
	}
	wantBD := [][]string{{"debhelper-compat"}, {"liburiparser-dev"}, {"check"}, {"python3", "python3-minimal"}, {"dh-yang"}}
	if !reflect.DeepEqual(s.BuildDepends, wantBD) {
		t.Errorf("BuildDepends = %v, want %v", s.BuildDepends, wantBD)
	}
	if !reflect.DeepEqual(s.Binaries, []string{"libvyatta-util1", "vyatta-util"}) {
		t.Errorf("Binaries = %v", s.Binaries)
	}
	if !reflect.DeepEqual(s.Provides, []string{"libvyatta-util", "vyatta-validate"}) {
		t.Errorf("Provides = %v", s.Provides)
	}
}

func TestParseSourceRejectsMissingSource(t *testing.T) {
	if _, err := ParseSource("Package: x\n"); err == nil {
		t.Error("want error for control file without Source paragraph")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd /Volumes/nudanos/distro && go test ./internal/control/`
Expected: FAIL with `undefined: Parse`

- [ ] **Step 3: Write `internal/control/control.go`**

```go
// Package control parses Debian control files (deb822 paragraphs).
package control

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

// Paragraph maps field names to values. Continuation lines are joined with a space.
type Paragraph map[string]string

// Parse splits deb822 text into paragraphs. Lines starting with '#' are ignored.
func Parse(text string) []Paragraph {
	var out []Paragraph
	cur := Paragraph{}
	key := ""
	flush := func() {
		if len(cur) > 0 {
			out = append(out, cur)
		}
		cur, key = Paragraph{}, ""
	}
	sc := bufio.NewScanner(strings.NewReader(text))
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.TrimSpace(line) == "":
			flush()
		case strings.HasPrefix(line, "#"):
		case (line[0] == ' ' || line[0] == '\t') && key != "":
			cur[key] += " " + strings.TrimSpace(line)
		default:
			k, v, ok := strings.Cut(line, ":")
			if !ok {
				continue
			}
			key = strings.TrimSpace(k)
			cur[key] = strings.TrimSpace(v)
		}
	}
	flush()
	return out
}

var (
	reVersion = regexp.MustCompile(`\([^)]*\)`)
	reArch    = regexp.MustCompile(`\[[^\]]*\]`)
	reProfile = regexp.MustCompile(`<[^>]*>`)
)

// Relations parses a relationship field into AND-groups of OR-alternatives,
// keeping package names only. Version constraints, [arch] and <profile>
// restrictions, ":any"-style qualifiers and ${substvars} are dropped.
func Relations(field string) [][]string {
	var groups [][]string
	for _, group := range strings.Split(field, ",") {
		var alts []string
		for _, alt := range strings.Split(group, "|") {
			alt = reVersion.ReplaceAllString(alt, "")
			alt = reArch.ReplaceAllString(alt, "")
			alt = reProfile.ReplaceAllString(alt, "")
			alt = strings.TrimSpace(alt)
			if alt == "" || strings.HasPrefix(alt, "${") {
				continue
			}
			name, _, _ := strings.Cut(alt, ":")
			alts = append(alts, name)
		}
		if len(alts) > 0 {
			groups = append(groups, alts)
		}
	}
	return groups
}

// Source summarises one source package's control file.
type Source struct {
	Name         string
	BuildDepends [][]string // Build-Depends, -Indep and -Arch combined
	Binaries     []string   // sorted
	Provides     []string   // sorted
}

// ParseSource reads the source paragraph and binary paragraphs of a control file.
func ParseSource(text string) (*Source, error) {
	ps := Parse(text)
	if len(ps) == 0 || ps[0]["Source"] == "" {
		return nil, errors.New("no Source paragraph")
	}
	s := &Source{Name: ps[0]["Source"]}
	for _, f := range []string{"Build-Depends", "Build-Depends-Indep", "Build-Depends-Arch"} {
		s.BuildDepends = append(s.BuildDepends, Relations(ps[0][f])...)
	}
	for _, p := range ps[1:] {
		if n := p["Package"]; n != "" {
			s.Binaries = append(s.Binaries, n)
		}
		for _, g := range Relations(p["Provides"]) {
			s.Provides = append(s.Provides, g...)
		}
	}
	sort.Strings(s.Binaries)
	sort.Strings(s.Provides)
	return s, nil
}

// ReadSource parses the control file at path (normally <src>/debian/control).
func ReadSource(path string) (*Source, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	s, err := ParseSource(string(b))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return s, nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/control/ -v`
Expected: 4 tests, `PASS`

- [ ] **Step 5: Commit**

```bash
git add internal/control && git commit -s -m "control: parse debian/control and relationship fields

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 6: Manifest loader

**Files:**
- Create: `internal/manifest/manifest.go`
- Test: `internal/manifest/manifest_test.go`

**Interfaces:**
- Produces:
  - `type Kind string` with constants `Danos`, `Upstream`, `Apt`, `Debian`, `Drop`, `Reference`
  - `type Entry struct { Name, Milestone, Repo, Ref, Upstream, Track, Packaging, Patches, Source, Key, Note string; Kind Kind; Ready bool; Packages map[string]string }`
  - `type Manifest struct { Packages []Entry }`
  - `func Parse(data []byte) (*Manifest, error)`
  - `func Load(path string) (*Manifest, error)`
  - `func (m *Manifest) Validate() error`
  - `func (m *Manifest) Ready(k Kind) []Entry`

  `Reference` (repos kept for history but not built, such as `build-iso` and `tests`) extends the spec's four kinds. `Upstream` entries may not be `ready` until plan 2 implements them.

- [ ] **Step 1: Add the YAML dependency**

Run: `cd /Volumes/nudanos/distro && go get go.yaml.in/yaml/v3@v3.0.5`
Expected: `go.mod` gains `require go.yaml.in/yaml/v3 v3.0.5`

- [ ] **Step 2: Write the failing tests**

```go
package manifest

import (
	"strings"
	"testing"
)

const good = `packages:
  - name: dh-yang
    kind: danos
    milestone: "1.0"
    ready: true
    repo: https://github.com/nudanos/dh-yang
    ref: trixie
  - name: frr
    kind: apt
    milestone: "1.0"
    ready: true
    source: "https://deb.frrouting.org/frr trixie frr-stable"
    key: https://deb.frrouting.org/frr/keys.gpg
    packages:
      frr: 10.7.1-0~deb13u1
      libyang3: 3.13.6-1~deb13u1
  - name: strongswan
    kind: upstream
    milestone: "1.1"
    upstream: https://github.com/strongswan/strongswan
    track: latest
    packaging: https://salsa.debian.org/debian/strongswan.git
  - name: openssh
    kind: debian
    milestone: "1.0"
  - name: tests
    kind: reference
    milestone: none
`

func TestParseGood(t *testing.T) {
	m, err := Parse([]byte(good))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Packages) != 5 {
		t.Fatalf("got %d entries", len(m.Packages))
	}
	if r := m.Ready(Danos); len(r) != 1 || r[0].Name != "dh-yang" {
		t.Errorf("Ready(Danos) = %v", r)
	}
	if r := m.Ready(Apt); len(r) != 1 || r[0].Packages["libyang3"] != "3.13.6-1~deb13u1" {
		t.Errorf("Ready(Apt) = %v", r)
	}
}

func TestParseRejects(t *testing.T) {
	cases := map[string]struct{ yaml, want string }{
		"unknown field": {"packages:\n  - name: a\n    kind: danos\n    milestone: \"1.0\"\n    repo: r\n    ref: x\n    reff: typo\n", "reff"},
		"duplicate":     {"packages:\n  - {name: a, kind: debian, milestone: \"1.0\"}\n  - {name: a, kind: debian, milestone: \"1.0\"}\n", "duplicate"},
		"danos no ref":  {"packages:\n  - {name: a, kind: danos, milestone: \"1.0\", repo: r}\n", "repo and ref"},
		"upstream ready": {"packages:\n  - {name: a, kind: upstream, milestone: \"1.1\", ready: true, upstream: u, packaging: p, track: latest}\n", "not implemented"},
		"bad track":     {"packages:\n  - {name: a, kind: upstream, milestone: \"1.1\", upstream: u, packaging: p, track: newest}\n", "track"},
		"apt source":    {"packages:\n  - {name: a, kind: apt, milestone: \"1.0\", source: \"https://x trixie\", key: k, packages: {p: \"1\"}}\n", "URL SUITE COMPONENT"},
		"bad kind":      {"packages:\n  - {name: a, kind: rpm, milestone: \"1.0\"}\n", "unknown kind"},
		"bad milestone": {"packages:\n  - {name: a, kind: debian, milestone: \"3\"}\n", "milestone"},
		"debian ready":  {"packages:\n  - {name: a, kind: debian, milestone: \"1.0\", ready: true}\n", "cannot be ready"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Parse([]byte(c.yaml))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("err = %v, want mention of %q", err, c.want)
			}
		})
	}
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./internal/manifest/`
Expected: FAIL with `undefined: Parse`

- [ ] **Step 4: Write `internal/manifest/manifest.go`**

```go
// Package manifest loads and validates distro/manifest.yaml.
package manifest

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	yaml "go.yaml.in/yaml/v3"
)

// Kind says where a package comes from.
type Kind string

const (
	Danos     Kind = "danos"     // our repo on its trixie branch, own debian/ packaging
	Upstream  Kind = "upstream"  // latest upstream release built with Debian's packaging
	Apt       Kind = "apt"       // mirrored at pinned versions from a third-party apt repo
	Debian    Kind = "debian"    // Debian 13's own package, not built by us
	Drop      Kind = "drop"      // not carried forward
	Reference Kind = "reference" // kept for history, not a package we build
)

var milestones = map[string]bool{"1.0": true, "1.1": true, "1.5": true, "2": true, "later": true, "none": true}

// Entry is one package or repository in the manifest.
type Entry struct {
	Name      string            `yaml:"name"`
	Kind      Kind              `yaml:"kind"`
	Milestone string            `yaml:"milestone"`
	Ready     bool              `yaml:"ready,omitempty"`
	Repo      string            `yaml:"repo,omitempty"`
	Ref       string            `yaml:"ref,omitempty"`
	Upstream  string            `yaml:"upstream,omitempty"`
	Track     string            `yaml:"track,omitempty"`
	Packaging string            `yaml:"packaging,omitempty"`
	Patches   string            `yaml:"patches,omitempty"`
	Source    string            `yaml:"source,omitempty"`   // apt: "URL SUITE COMPONENT"
	Key       string            `yaml:"key,omitempty"`      // apt: signing key URL
	Packages  map[string]string `yaml:"packages,omitempty"` // apt: binary package -> exact version
	Note      string            `yaml:"note,omitempty"`
}

// Manifest is the whole package list.
type Manifest struct {
	Packages []Entry `yaml:"packages"`
}

// Parse decodes YAML strictly (unknown fields are errors) and validates it.
func Parse(data []byte) (*Manifest, error) {
	var m Manifest
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("manifest: %w", err)
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

// Load reads and parses a manifest file.
func Load(path string) (*Manifest, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	m, err := Parse(b)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return m, nil
}

// Validate checks every entry and reports all problems at once.
func (m *Manifest) Validate() error {
	var errs []string
	seen := map[string]bool{}
	for i, e := range m.Packages {
		bad := func(msg string) { errs = append(errs, fmt.Sprintf("entry %d (%s): %s", i, e.Name, msg)) }
		if e.Name == "" {
			bad("missing name")
		}
		if seen[e.Name] {
			bad("duplicate name")
		}
		seen[e.Name] = true
		if !milestones[e.Milestone] {
			bad("milestone must be one of 1.0, 1.1, 1.5, 2, later, none")
		}
		switch e.Kind {
		case Danos:
			if e.Repo == "" || e.Ref == "" {
				bad("danos entries need repo and ref")
			}
		case Upstream:
			if e.Upstream == "" || e.Packaging == "" {
				bad("upstream entries need upstream and packaging")
			}
			if e.Track != "latest" && e.Track != "lts" && e.Track != "debian" {
				bad("track must be latest, lts or debian")
			}
			if e.Ready {
				bad("upstream builds are not implemented yet (plan 2); keep ready: false")
			}
		case Apt:
			if len(strings.Fields(e.Source)) != 3 || e.Key == "" || len(e.Packages) == 0 {
				bad(`apt entries need source "URL SUITE COMPONENT", key and packages`)
			}
		case Debian, Drop, Reference:
			if e.Ready {
				bad(string(e.Kind) + " entries cannot be ready")
			}
		default:
			bad(fmt.Sprintf("unknown kind %q", e.Kind))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("manifest invalid:\n  %s", strings.Join(errs, "\n  "))
	}
	return nil
}

// Ready returns the entries of kind k that are marked ready, in manifest order.
func (m *Manifest) Ready(k Kind) []Entry {
	var out []Entry
	for _, e := range m.Packages {
		if e.Kind == k && e.Ready {
			out = append(out, e)
		}
	}
	return out
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/manifest/ -v`
Expected: `TestParseGood` and 9 `TestParseRejects` subtests `PASS`

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum internal/manifest && git commit -s -m "manifest: strict loader and validation

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 7: Build-order planning

**Files:**
- Create: `internal/plan/plan.go`
- Test: `internal/plan/plan_test.go`

**Interfaces:**
- Consumes: `control.Source` (Task 5).
- Produces:
  - `type Graph struct { Deps map[string][]string }` (node → sorted nodes it build-depends on)
  - `func Build(sources map[string]*control.Source) *Graph` (map keys are manifest names)
  - `func (g *Graph) Tiers() ([][]string, error)`

- [ ] **Step 1: Write the failing tests**

```go
package plan

import (
	"reflect"
	"strings"
	"testing"

	"github.com/nudanos/distro/internal/control"
)

func src(bins []string, provides []string, bd ...[]string) *control.Source {
	return &control.Source{Binaries: bins, Provides: provides, BuildDepends: bd}
}

func TestBuildResolvesBinariesProvidesAndAlternatives(t *testing.T) {
	g := Build(map[string]*control.Source{
		"dh-yang": src([]string{"dh-yang"}, nil, []string{"debhelper-compat"}),
		"yang":    src([]string{"golang-github-danos-yang-dev"}, []string{"yang-virtual"}, []string{"dh-yang"}),
		"configd": src([]string{"configd"}, nil,
			[]string{"missing-in-set", "yang-virtual"}, // first in-set alternative wins
			[]string{"configd"},                        // self-dependency ignored
			[]string{"libc6-dev"}),                     // external dependency ignored
	})
	want := map[string][]string{"dh-yang": nil, "yang": {"dh-yang"}, "configd": {"yang"}}
	for k, v := range want {
		if !reflect.DeepEqual(g.Deps[k], v) {
			t.Errorf("Deps[%s] = %v, want %v", k, g.Deps[k], v)
		}
	}
}

func TestTiers(t *testing.T) {
	g := &Graph{Deps: map[string][]string{"a": nil, "b": {"a"}, "c": {"a"}, "d": {"b", "c"}}}
	tiers, err := g.Tiers()
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{{"a"}, {"b", "c"}, {"d"}}
	if !reflect.DeepEqual(tiers, want) {
		t.Errorf("Tiers = %v, want %v", tiers, want)
	}
}

func TestTiersReportsCycle(t *testing.T) {
	g := &Graph{Deps: map[string][]string{"a": {"b"}, "b": {"a"}, "c": nil}}
	tiers, err := g.Tiers()
	if err == nil || !strings.Contains(err.Error(), "a, b") {
		t.Fatalf("err = %v, want cycle naming a, b", err)
	}
	if !reflect.DeepEqual(tiers, [][]string{{"c"}}) {
		t.Errorf("tiers before cycle = %v", tiers)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/plan/`
Expected: FAIL with `undefined: Build`

- [ ] **Step 3: Write `internal/plan/plan.go`**

```go
// Package plan orders source packages by their build dependencies.
package plan

import (
	"fmt"
	"sort"
	"strings"

	"github.com/nudanos/distro/internal/control"
)

// Graph holds build-order edges: Deps[a] lists the nodes a build-depends on.
type Graph struct {
	Deps map[string][]string
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Build derives edges from Build-Depends. A dependency counts only if another
// source in the set produces it (as a binary, or failing that as a Provides).
// For alternatives, the first alternative produced inside the set wins.
func Build(sources map[string]*control.Source) *Graph {
	names := sortedKeys(sources)
	producer := map[string]string{}
	for _, n := range names {
		for _, b := range sources[n].Binaries {
			producer[b] = n
		}
	}
	for _, n := range names {
		for _, p := range sources[n].Provides {
			if _, taken := producer[p]; !taken {
				producer[p] = n
			}
		}
	}
	g := &Graph{Deps: map[string][]string{}}
	for _, n := range names {
		set := map[string]bool{}
		for _, alts := range sources[n].BuildDepends {
			for _, a := range alts {
				if p, ok := producer[a]; ok {
					if p != n {
						set[p] = true
					}
					break
				}
			}
		}
		var deps []string
		if len(set) > 0 {
			deps = sortedKeys(set)
		}
		g.Deps[n] = deps
	}
	return g
}

// Tiers groups nodes so each tier depends only on earlier tiers. On a cycle it
// returns the tiers resolved so far and an error naming the unresolvable nodes.
func (g *Graph) Tiers() ([][]string, error) {
	done := map[string]bool{}
	remaining := sortedKeys(g.Deps)
	var tiers [][]string
	for len(remaining) > 0 {
		var tier, rest []string
		for _, n := range remaining {
			ready := true
			for _, d := range g.Deps[n] {
				if !done[d] {
					ready = false
					break
				}
			}
			if ready {
				tier = append(tier, n)
			} else {
				rest = append(rest, n)
			}
		}
		if len(tier) == 0 {
			return tiers, fmt.Errorf("dependency cycle (or blocked by one) among: %s", strings.Join(rest, ", "))
		}
		for _, n := range tier {
			done[n] = true
		}
		tiers = append(tiers, tier)
		remaining = rest
	}
	return tiers, nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/plan/ -v`
Expected: 3 tests `PASS`

- [ ] **Step 5: Commit**

```bash
git add internal/plan && git commit -s -m "plan: build-order graph and tiers

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 8: Manifest generator and `manifest.yaml`

**Files:**
- Create: `tools/gen-manifest.py`
- Test: `tools/test_gen_manifest.py`
- Create (generated): `manifest.yaml`
- Test: `internal/manifest/repo_test.go`

**Interfaces:**
- Consumes: `docs/data/*.json` (Task 4). The `Entry` field names and kinds come from Task 6.
- Produces: `manifest.yaml` with one entry per danos repo (for `frr`, the entry is the pinned apt mirror, not the fork), plus `vci-dhcpv6-pd`, and upstream entries for `libnetconf2` and `kea`. Only `dh-yang`, `dh-vci`, `vyatta-util` and `frr` are `ready: true`. Pure functions: `classify(name, data) -> dict`, `emit(entries) -> str`.

- [ ] **Step 1: Write the failing tests**

`tools/test_gen_manifest.py`:
```python
from __future__ import annotations

import importlib.util
import os
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
spec = importlib.util.spec_from_file_location("gen", os.path.join(HERE, "gen-manifest.py"))
gen = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gen)

DATA = {
    "inventory": {"configd": {"kind": "original"}, "strongswan": {"kind": "fork"},
                  "vyatta-poe": {"kind": "original"}, "yang": {"kind": "original"},
                  "vyatta-controller": {"kind": "original"}, "tests": {"kind": "original"},
                  "vyatta-service-dhcp": {"kind": "original"}, "aaa": {"kind": "original"}},
    "m1": {"configd", "vyatta-service-dhcp"},
    "edges": {"configd": ["yang"], "yang": ["dh-yang"], "dh-yang": []},
}


class ClassifyTest(unittest.TestCase):
    def test_runtime_closure_is_milestone_1(self):
        e = gen.classify("configd", DATA)
        self.assertEqual((e["kind"], e["milestone"], e["ref"]), ("danos", "1.0", "trixie"))
        self.assertEqual(e["repo"], "https://github.com/nudanos/configd")

    def test_build_dependency_closure_is_milestone_1(self):
        self.assertEqual(gen.classify("yang", DATA)["milestone"], "1.0")

    def test_rewrite_packages_are_1_1(self):
        self.assertEqual(gen.classify("vyatta-service-dhcp", DATA)["milestone"], "1.1")

    def test_fork_table_wins(self):
        e = gen.classify("strongswan", DATA)
        self.assertEqual((e["kind"], e["track"]), ("upstream", "latest"))
        self.assertEqual(e["packaging"], "https://salsa.debian.org/debian/strongswan.git")

    def test_hardware_dropped_dataplane_m2_reference_and_rest_later(self):
        self.assertEqual(gen.classify("vyatta-poe", DATA)["kind"], "drop")
        self.assertEqual(gen.classify("vyatta-controller", DATA)["milestone"], "2")
        self.assertEqual(gen.classify("tests", DATA)["kind"], "reference")
        self.assertEqual(gen.classify("aaa", DATA)["milestone"], "later")

    def test_netconf_forks_are_replaced(self):
        self.assertEqual(gen.classify("libnetconf", DATA)["kind"], "drop")
        self.assertEqual(gen.classify("pyang", DATA)["kind"], "drop")

    def test_frr_is_left_to_the_apt_entry(self):
        self.assertIsNone(gen.classify("frr", DATA))

    def test_pilot_is_ready(self):
        self.assertTrue(gen.classify("configd", DATA).get("ready") is None)
        d = dict(DATA, inventory=dict(DATA["inventory"], **{"dh-yang": {"kind": "original"}}))
        self.assertTrue(gen.classify("dh-yang", d)["ready"])


class EmitTest(unittest.TestCase):
    def test_quotes_values_that_yaml_would_misread(self):
        out = gen.emit([{"name": "x", "kind": "apt", "milestone": "1.0",
                         "packages": {"frr": "10.7.1-0~deb13u1"}, "note": "a: b # c"}])
        self.assertIn('milestone: "1.0"', out)
        self.assertIn('frr: "10.7.1-0~deb13u1"', out)
        self.assertIn('note: "a: b # c"', out)
        self.assertTrue(out.startswith("# Generated by tools/gen-manifest.py"))


if __name__ == "__main__":
    unittest.main()
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `python3 -m unittest discover -s tools -p 'test_*.py'`
Expected: FAIL: `FileNotFoundError` for `gen-manifest.py`

- [ ] **Step 3: Write `tools/gen-manifest.py`** (the dispositions are spec §4.3, verbatim)

```python
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
```

- [ ] **Step 4: Run the Python tests to verify they pass**

Run: `python3 -m unittest discover -s tools -p 'test_*.py' -v`
Expected: 9 tests `OK`

- [ ] **Step 5: Generate the manifest and write a Go test that loads it**

```bash
python3 tools/gen-manifest.py > manifest.yaml
grep -c '^  - name:' manifest.yaml; grep -c '^  - name: "frr"' manifest.yaml
```
Expected: `192` (189 danos repos with frr's replaced by the apt entry, plus vci-dhcpv6-pd, libnetconf2 and kea), then `1`

`internal/manifest/repo_test.go`:
```go
package manifest

import "testing"

// The committed manifest must always load, and the pilot must be what is ready.
func TestRepoManifest(t *testing.T) {
	m, err := Load("../../manifest.yaml")
	if err != nil {
		t.Fatal(err)
	}
	ready := map[string]bool{}
	for _, k := range []Kind{Danos, Apt} {
		for _, e := range m.Ready(k) {
			ready[e.Name] = true
		}
	}
	for _, n := range []string{"dh-yang", "dh-vci", "vyatta-util", "frr"} {
		if !ready[n] {
			t.Errorf("%s should be ready", n)
		}
	}
	if len(ready) != 4 {
		t.Errorf("ready entries = %v, want only the pilot", ready)
	}
}
```

- [ ] **Step 6: Run the Go test**

Run: `go test ./internal/manifest/ -run TestRepoManifest -v`
Expected: `PASS`

- [ ] **Step 7: Add the tools tests to CI, then commit**

Append to the steps in `.github/workflows/ci.yml`:
```yaml
      - run: python3 -m unittest discover -s tools -p 'test_*.py' -v
```

```bash
git add tools/gen-manifest.py tools/test_gen_manifest.py manifest.yaml internal/manifest/repo_test.go .github/workflows/ci.yml
git commit -s -m "manifest: generate package list from review data and spec dispositions

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 9: Container engine and builder image

**Files:**
- Create: `internal/engine/engine.go`
- Test: `internal/engine/engine_test.go`
- Create: `builder/Dockerfile`, `builder/preferences`, `builder/build-package.sh`

**Interfaces:**
- Produces:
  - `type Mount struct { Host, Container string; ReadOnly bool }`
  - `type RunSpec struct { Image string; Mounts []Mount; Env map[string]string; Workdir, Network string; Cmd []string }`
  - `type Engine struct { Bin string }`
  - `func (e Engine) RunArgs(s RunSpec) []string`
  - `func (e Engine) Run(ctx context.Context, s RunSpec, stdout, stderr io.Writer) error`
  - `func (e Engine) BuildImage(ctx context.Context, dir, tag string, stdout, stderr io.Writer) error`
  - Image `nudanos/builder:trixie` with `/usr/local/bin/build-package`. Contract: `/src` (ro) holds the source, `/pool` (ro) holds earlier outputs, `/out` (rw) receives the results; env `PKG`, `HOST_UID`, `HOST_GID`.

- [ ] **Step 1: Write the failing test**

```go
package engine

import (
	"reflect"
	"testing"
)

func TestRunArgs(t *testing.T) {
	got := Engine{Bin: "docker"}.RunArgs(RunSpec{
		Image:   "nudanos/builder:trixie",
		Mounts:  []Mount{{Host: "/w/src/a", Container: "/src", ReadOnly: true}, {Host: "/w/out/a", Container: "/out"}},
		Env:     map[string]string{"PKG": "a", "HOST_UID": "501"},
		Workdir: "/build",
		Network: "none",
		Cmd:     []string{"/usr/local/bin/build-package"},
	})
	want := []string{"run", "--rm", "--network", "none",
		"-v", "/w/src/a:/src:ro", "-v", "/w/out/a:/out",
		"-e", "HOST_UID=501", "-e", "PKG=a",
		"-w", "/build", "nudanos/builder:trixie", "/usr/local/bin/build-package"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("RunArgs =\n %v\nwant\n %v", got, want)
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/engine/`
Expected: FAIL with `undefined: Engine`

- [ ] **Step 3: Write `internal/engine/engine.go`**

```go
// Package engine runs containers through the docker or podman CLI.
package engine

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"sort"
)

// Mount is a bind mount.
type Mount struct {
	Host, Container string
	ReadOnly        bool
}

// RunSpec describes one container run.
type RunSpec struct {
	Image   string
	Mounts  []Mount
	Env     map[string]string
	Workdir string
	Network string // empty = engine default
	Cmd     []string
}

// Engine is a docker-compatible CLI ("docker" or "podman").
type Engine struct {
	Bin string
}

// RunArgs returns the CLI arguments for s, with env vars in sorted order.
func (e Engine) RunArgs(s RunSpec) []string {
	args := []string{"run", "--rm"}
	if s.Network != "" {
		args = append(args, "--network", s.Network)
	}
	for _, m := range s.Mounts {
		v := m.Host + ":" + m.Container
		if m.ReadOnly {
			v += ":ro"
		}
		args = append(args, "-v", v)
	}
	keys := make([]string, 0, len(s.Env))
	for k := range s.Env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		args = append(args, "-e", k+"="+s.Env[k])
	}
	if s.Workdir != "" {
		args = append(args, "-w", s.Workdir)
	}
	args = append(args, s.Image)
	return append(args, s.Cmd...)
}

// Run executes s and waits for it.
func (e Engine) Run(ctx context.Context, s RunSpec, stdout, stderr io.Writer) error {
	cmd := exec.CommandContext(ctx, e.Bin, e.RunArgs(s)...)
	cmd.Stdout, cmd.Stderr = stdout, stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s run %s: %w", e.Bin, s.Image, err)
	}
	return nil
}

// BuildImage builds the Dockerfile in dir and tags it.
func (e Engine) BuildImage(ctx context.Context, dir, tag string, stdout, stderr io.Writer) error {
	cmd := exec.CommandContext(ctx, e.Bin, "build", "--pull", "-t", tag, dir)
	cmd.Stdout, cmd.Stderr = stdout, stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s build %s: %w", e.Bin, dir, err)
	}
	return nil
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/engine/ -v`
Expected: `PASS`

- [ ] **Step 5: Write the builder image**

`builder/preferences`:
```text
# Go toolchain from trixie-backports (spec §4.2: Go 1.26).
Package: golang-go golang-src golang-doc golang-1.26-go golang-1.26-src golang-1.26-doc
Pin: release n=trixie-backports
Pin-Priority: 990
```

`builder/Dockerfile`:
```dockerfile
# NuDanOS package builder: Debian 13 "trixie" plus trixie-backports.
FROM debian:trixie
ENV DEBIAN_FRONTEND=noninteractive LANG=C.UTF-8
RUN echo 'deb http://deb.debian.org/debian trixie-backports main' > /etc/apt/sources.list.d/backports.list \
 && apt-get update \
 && apt-get install -y --no-install-recommends \
      build-essential devscripts dpkg-dev fakeroot lintian apt-utils equivs \
      git ca-certificates curl gnupg xz-utils \
 && rm -rf /var/lib/apt/lists/* \
 && useradd --create-home --uid 1000 builder
COPY preferences /etc/apt/preferences.d/nudanos
COPY build-package.sh /usr/local/bin/build-package
RUN chmod 0755 /usr/local/bin/build-package
```

`builder/build-package.sh`:
```bash
#!/bin/bash
# Build one Debian source package from /src (read-only) into /out.
# Earlier outputs under /pool are served as a local apt repository with priority
# over Debian, so packages build against each other.
#
# Derived from the buildpackage script in jsouthworth/danos-buildpackage,
# Copyright (c) 2019 John Southworth, MIT License (see NOTICE).
set -euo pipefail
shopt -s nullglob
: "${PKG:?}" "${HOST_UID:=0}" "${HOST_GID:=0}"

setup_local_repo() {
    mkdir -p /tmp/pool
    find /pool -name '*.deb' -exec cp -t /tmp/pool {} +
    (cd /tmp/pool && apt-ftparchive packages . > Packages)
    echo 'deb [trusted=yes] file:/tmp/pool ./' > /etc/apt/sources.list.d/000-local.list
    printf 'Package: *\nPin: origin ""\nPin-Priority: 999\n' > /etc/apt/preferences.d/000-local
    apt-get update
}

main() {
    setup_local_repo
    rm -rf /build && mkdir -p /build
    cp -a /src /build/pkg
    cd /build/pkg
    local format="1.0"
    [ -f debian/source/format ] && format="$(cat debian/source/format)"
    if [ "$format" = "3.0 (quilt)" ]; then
        echo "error: $PKG uses 3.0 (quilt); building from an upstream tarball arrives in plan 2" >&2
        exit 2
    fi
    apt-get -y --no-install-recommends build-dep ./
    chown -R builder:builder /build
    runuser -u builder -- dpkg-buildpackage -us -uc -I -i
    lintian --fail-on error ../*.changes
    cp ../*.deb ../*.dsc ../*.tar.* ../*.buildinfo ../*.changes /out/
    chown -R "$HOST_UID:$HOST_GID" /out
}

main "$@"
```

- [ ] **Step 6: Build the image and smoke-test the script on the unported `dh-yang`** (this shows the script works; lintian errors on unported packaging are acceptable here)

```bash
chmod +x builder/build-package.sh
docker build --pull -t nudanos/builder:trixie builder/
git clone -q https://github.com/nudanos/dh-yang /Volumes/nudanos/work/smoke-dh-yang
mkdir -p /Volumes/nudanos/work/smoke-out /Volumes/nudanos/work/smoke-pool
docker run --rm -v /Volumes/nudanos/work/smoke-dh-yang:/src:ro -v /Volumes/nudanos/work/smoke-pool:/pool:ro \
  -v /Volumes/nudanos/work/smoke-out:/out -e PKG=dh-yang -e HOST_UID=$(id -u) -e HOST_GID=$(id -g) \
  nudanos/builder:trixie /usr/local/bin/build-package; echo "exit=$?"
```
Expected: the log shows the local repo set up, `apt-get build-dep` resolving debhelper, and `dpkg-buildpackage` starting. It either finishes (exit 0, `dh-yang_0.3_all.deb` in `smoke-out`), or stops at a debhelper compat-9 deprecation or a lintian error on the unported packaging. Both outcomes prove the script's plumbing; Task 12 ports the packaging. Any other failure (apt, mounts, permissions) is a builder bug to fix here. Then clean up: `rm -rf /Volumes/nudanos/work/smoke-*`

- [ ] **Step 7: Commit**

```bash
git add internal/engine builder && git commit -s -m "builder: Debian 13 builder image and container engine wrapper

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 10: Fetch, workspace check, cached ordered builds, and the `distro-build` CLI

**Files:**
- Create: `internal/fetch/fetch.go`, `internal/fetch/fetch_test.go`
- Create: `internal/workspace/workspace.go`, `internal/workspace/workspace_test.go`
- Create: `internal/build/hash.go`, `internal/build/build.go`, `internal/build/build_test.go`
- Create: `cmd/distro-build/main.go`

**Interfaces:**
- Consumes: `manifest` (Task 6), `control` (Task 5), `plan` (Task 7), `engine` and the builder image contract (Task 9).
- Produces:
  - `fetch.Git(ctx context.Context, repo, ref, dir string, log io.Writer) error`
  - `workspace.CaseSensitive(dir string) (bool, error)`
  - `build.TreeHash(dir string) (string, error)`
  - `type build.Status string` (`Built`, `Cached`, `Failed`, `Skipped`)
  - `type build.Result struct { Name string; Status Status; Detail string }`
  - `type build.Func func(ctx context.Context, name, srcDir, outDir string) error`
  - `type build.Builder struct { SrcDirs map[string]string; OutRoot, StateFile, KeySalt string; Build Func; Log io.Writer }`
  - `func (b *Builder) Run(ctx context.Context, tiers [][]string, deps map[string][]string) ([]Result, error)`
  - `func build.HasArtifacts(dir string) bool`
  - CLI: `distro-build [-manifest F] [-work D] [-engine docker|podman] [-image T] [-builder-dir D] [-local name=dir]... builder|fetch|plan|build`

- [ ] **Step 1: Write the failing tests**

`internal/workspace/workspace_test.go`:
```go
package workspace

import (
	"runtime"
	"testing"
)

func TestCaseSensitiveOnLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux filesystems are case-sensitive; macOS default volumes are not")
	}
	ok, err := CaseSensitive(t.TempDir())
	if err != nil || !ok {
		t.Fatalf("CaseSensitive = %v, %v; want true, nil", ok, err)
	}
}

func TestCaseSensitiveCreatesMissingDir(t *testing.T) {
	dir := t.TempDir() + "/new/work"
	if _, err := CaseSensitive(dir); err != nil {
		t.Fatal(err)
	}
}
```

`internal/fetch/fetch_test.go`:
```go
package fetch

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@e", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@e")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestGitClonesBranchThenFollowsUpdates(t *testing.T) {
	origin := t.TempDir()
	git(t, origin, "init", "-q", "-b", "master")
	os.WriteFile(filepath.Join(origin, "f"), []byte("master"), 0o644)
	git(t, origin, "add", "f")
	git(t, origin, "commit", "-qm", "m")
	git(t, origin, "tag", "danos/2105")
	git(t, origin, "checkout", "-qb", "trixie")
	os.WriteFile(filepath.Join(origin, "f"), []byte("trixie-1"), 0o644)
	git(t, origin, "commit", "-qam", "t1")

	dir := filepath.Join(t.TempDir(), "src", "pkg")
	if err := Git(context.Background(), origin, "trixie", dir, io.Discard); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "f")); string(b) != "trixie-1" {
		t.Fatalf("after clone f = %q", b)
	}

	os.WriteFile(filepath.Join(origin, "f"), []byte("trixie-2"), 0o644)
	git(t, origin, "commit", "-qam", "t2")
	os.WriteFile(filepath.Join(dir, "junk"), []byte("x"), 0o644) // untracked files are cleaned
	if err := Git(context.Background(), origin, "trixie", dir, io.Discard); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "f")); string(b) != "trixie-2" {
		t.Fatalf("after update f = %q", b)
	}
	if _, err := os.Stat(filepath.Join(dir, "junk")); !os.IsNotExist(err) {
		t.Error("untracked file survived update")
	}

	if err := Git(context.Background(), origin, "danos/2105", dir, io.Discard); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "f")); string(b) != "master" {
		t.Fatalf("at tag f = %q", b)
	}
}

func TestGitUnknownRefFails(t *testing.T) {
	origin := t.TempDir()
	git(t, origin, "init", "-q", "-b", "master")
	git(t, origin, "commit", "-q", "--allow-empty", "-m", "m")
	if err := Git(context.Background(), origin, "trixie", filepath.Join(t.TempDir(), "p"), io.Discard); err == nil {
		t.Error("want error for missing ref")
	}
}
```

`internal/build/build_test.go`:
```go
package build

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestTreeHashIgnoresGitButNotContentOrMode(t *testing.T) {
	d := t.TempDir()
	write(t, filepath.Join(d, "debian", "control"), "Source: a\n")
	write(t, filepath.Join(d, ".git", "HEAD"), "ref: x\n")
	h1, _ := TreeHash(d)
	write(t, filepath.Join(d, ".git", "HEAD"), "ref: y\n")
	if h2, _ := TreeHash(d); h2 != h1 {
		t.Error(".git change altered hash")
	}
	os.Chmod(filepath.Join(d, "debian", "control"), 0o755)
	h3, _ := TreeHash(d)
	if h3 == h1 {
		t.Error("mode change did not alter hash")
	}
	write(t, filepath.Join(d, "debian", "control"), "Source: b\n")
	if h4, _ := TreeHash(d); h4 == h3 {
		t.Error("content change did not alter hash")
	}
}

type fixture struct {
	b     *Builder
	built []string
	fail  map[string]bool
}

func newFixture(t *testing.T, names ...string) *fixture {
	root := t.TempDir()
	f := &fixture{fail: map[string]bool{}}
	dirs := map[string]string{}
	for _, n := range names {
		dirs[n] = filepath.Join(root, "src", n)
		write(t, filepath.Join(dirs[n], "debian", "control"), "Source: "+n+"\n")
	}
	f.b = &Builder{SrcDirs: dirs, OutRoot: filepath.Join(root, "out"), StateFile: filepath.Join(root, "state.json"),
		KeySalt: "builder-v1", Log: io.Discard,
		Build: func(ctx context.Context, name, src, out string) error {
			f.built = append(f.built, name)
			if f.fail[name] {
				return errors.New("boom")
			}
			return os.WriteFile(filepath.Join(out, name+"_1_all.deb"), []byte(name), 0o644)
		}}
	return f
}

func statuses(rs []Result) map[string]Result {
	m := map[string]Result{}
	for _, r := range rs {
		m[r.Name] = r
	}
	return m
}

func TestRunSkipsDependentsOfAFailureAndBuildsTheRest(t *testing.T) {
	f := newFixture(t, "a", "b", "c", "d")
	f.fail["a"] = true
	rs, err := f.b.Run(context.Background(), [][]string{{"a", "c"}, {"b"}, {"d"}},
		map[string][]string{"b": {"a"}, "d": {"b", "c"}})
	if err != nil {
		t.Fatal(err)
	}
	s := statuses(rs)
	if s["a"].Status != Failed || s["c"].Status != Built {
		t.Errorf("a=%v c=%v", s["a"], s["c"])
	}
	if s["b"].Status != Skipped || !strings.Contains(s["b"].Detail, "a") {
		t.Errorf("b = %+v, want skipped naming a", s["b"])
	}
	if s["d"].Status != Skipped || !strings.Contains(s["d"].Detail, "b") {
		t.Errorf("d = %+v, want skipped naming b", s["d"])
	}
	if strings.Join(f.built, ",") != "a,c" {
		t.Errorf("built = %v, want a,c", f.built)
	}
}

func TestRunCachesUntilSourceOrBuilderChanges(t *testing.T) {
	f := newFixture(t, "a")
	run := func() Status {
		rs, err := f.b.Run(context.Background(), [][]string{{"a"}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		return rs[0].Status
	}
	if s := run(); s != Built {
		t.Fatalf("first run = %s", s)
	}
	if s := run(); s != Cached {
		t.Fatalf("second run = %s", s)
	}
	write(t, filepath.Join(f.b.SrcDirs["a"], ".git", "FETCH_HEAD"), "x")
	if s := run(); s != Cached {
		t.Fatalf("after .git change = %s", s)
	}
	write(t, filepath.Join(f.b.SrcDirs["a"], "debian", "rules"), "#!/usr/bin/make -f\n")
	if s := run(); s != Built {
		t.Fatalf("after source change = %s", s)
	}
	f.b.KeySalt = "builder-v2"
	if s := run(); s != Built {
		t.Fatalf("after builder change = %s", s)
	}
	os.RemoveAll(filepath.Join(f.b.OutRoot, "a"))
	if s := run(); s != Built {
		t.Fatalf("after artifacts removed = %s", s)
	}
}

func TestRunFailsWhenBuildProducesNoDebs(t *testing.T) {
	f := newFixture(t, "a")
	f.b.Build = func(ctx context.Context, name, src, out string) error { return nil }
	rs, _ := f.b.Run(context.Background(), [][]string{{"a"}}, nil)
	if rs[0].Status != Failed {
		t.Errorf("status = %s, want failed", rs[0].Status)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/fetch/ ./internal/workspace/ ./internal/build/`
Expected: FAIL with `undefined: Git`, `undefined: CaseSensitive`, `undefined: TreeHash`

- [ ] **Step 3: Write `internal/workspace/workspace.go`**

```go
// Package workspace checks properties of the build work directory.
package workspace

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// CaseSensitive reports whether dir's filesystem distinguishes names by case.
// It creates dir if missing.
func CaseSensitive(dir string) (bool, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false, err
	}
	probe, err := os.MkdirTemp(dir, ".casecheck-")
	if err != nil {
		return false, err
	}
	defer os.RemoveAll(probe)
	if err := os.WriteFile(filepath.Join(probe, "a"), nil, 0o644); err != nil {
		return false, err
	}
	_, err = os.Stat(filepath.Join(probe, "A"))
	switch {
	case err == nil:
		return false, nil
	case errors.Is(err, fs.ErrNotExist):
		return true, nil
	default:
		return false, err
	}
}
```

- [ ] **Step 4: Write `internal/fetch/fetch.go`**

```go
// Package fetch checks out package sources.
package fetch

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

func run(ctx context.Context, log io.Writer, dir string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Stdout, cmd.Stderr = log, log
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git %v: %w", args, err)
	}
	return nil
}

// Git clones repo into dir (or fetches if dir already holds a clone), checks out
// ref detached (a remote branch, else a tag), and removes untracked files.
func Git(ctx context.Context, repo, ref, dir string, log io.Writer) error {
	if _, err := os.Stat(filepath.Join(dir, ".git")); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
			return err
		}
		if err := run(ctx, log, "", "clone", "--no-checkout", repo, dir); err != nil {
			return err
		}
	} else if err := run(ctx, log, dir, "fetch", "--tags", "--force", "--prune", "origin"); err != nil {
		return err
	}
	target := "origin/" + ref
	if run(ctx, io.Discard, dir, "rev-parse", "--verify", "--quiet", target+"^{commit}") != nil {
		target = "refs/tags/" + ref
		if run(ctx, io.Discard, dir, "rev-parse", "--verify", "--quiet", target+"^{commit}") != nil {
			return fmt.Errorf("%s: no branch or tag %q", repo, ref)
		}
	}
	if err := run(ctx, log, dir, "checkout", "--quiet", "--force", "--detach", target); err != nil {
		return err
	}
	return run(ctx, log, dir, "clean", "-ffdxq")
}
```

- [ ] **Step 5: Write `internal/build/hash.go`**

```go
package build

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// TreeHash is a sha256 over every non-.git file under dir: relative path, mode
// and contents (symlink target for links), in sorted path order.
func TreeHash(dir string) (string, error) {
	var paths []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		if !d.IsDir() {
			paths = append(paths, p)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(paths)
	h := sha256.New()
	for _, p := range paths {
		rel, _ := filepath.Rel(dir, p)
		info, err := os.Lstat(p)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(h, "%s\x00%o\x00", filepath.ToSlash(rel), info.Mode())
		if info.Mode()&fs.ModeSymlink != 0 {
			target, err := os.Readlink(p)
			if err != nil {
				return "", err
			}
			io.WriteString(h, target)
		} else {
			f, err := os.Open(p)
			if err != nil {
				return "", err
			}
			_, err = io.Copy(h, f)
			f.Close()
			if err != nil {
				return "", err
			}
		}
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
```

- [ ] **Step 6: Write `internal/build/build.go`**

```go
// Package build runs ordered, cached package builds.
package build

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// Status is the outcome of one package in a run.
type Status string

const (
	Built   Status = "built"
	Cached  Status = "cached"
	Failed  Status = "failed"
	Skipped Status = "skipped"
)

// Result reports one package.
type Result struct {
	Name   string
	Status Status
	Detail string
}

// Func builds the source in srcDir, writing .deb and source artifacts into outDir.
type Func func(ctx context.Context, name, srcDir, outDir string) error

// Builder builds packages tier by tier. Artifacts for package N go to
// OutRoot/N; the whole OutRoot is what later builds install dependencies from.
type Builder struct {
	SrcDirs   map[string]string // manifest name -> source checkout
	OutRoot   string
	StateFile string // JSON: name -> cache key of the last successful build
	KeySalt   string // part of every cache key; change it to invalidate all
	Build     Func
	Log       io.Writer
}

// HasArtifacts reports whether dir holds at least one .deb.
func HasArtifacts(dir string) bool {
	m, _ := filepath.Glob(filepath.Join(dir, "*.deb"))
	return len(m) > 0
}

func loadState(path string) (map[string]string, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	st := map[string]string{}
	if err := json.Unmarshal(b, &st); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return st, nil
}

func saveState(path string, st map[string]string) error {
	b, err := json.MarshalIndent(st, "", " ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Run builds every package in tiers order. A package whose dependency failed or
// was skipped is skipped. The error return is reserved for state-file I/O.
func (b *Builder) Run(ctx context.Context, tiers [][]string, deps map[string][]string) ([]Result, error) {
	state, err := loadState(b.StateFile)
	if err != nil {
		return nil, err
	}
	bad := map[string]bool{}
	var results []Result
	record := func(r Result) {
		if r.Status == Failed || r.Status == Skipped {
			bad[r.Name] = true
		}
		results = append(results, r)
		fmt.Fprintf(b.Log, "==> %-40s %s %s\n", r.Name, r.Status, r.Detail)
	}
	for _, tier := range tiers {
		for _, name := range tier {
			if err := ctx.Err(); err != nil {
				return results, err
			}
			blocker := ""
			for _, d := range deps[name] {
				if bad[d] {
					blocker = d
					break
				}
			}
			if blocker != "" {
				record(Result{name, Skipped, "dependency " + blocker + " did not build"})
				continue
			}
			src, out := b.SrcDirs[name], filepath.Join(b.OutRoot, name)
			hash, err := TreeHash(src)
			if err != nil {
				record(Result{name, Failed, err.Error()})
				continue
			}
			key := hash + ":" + b.KeySalt
			if state[name] == key && HasArtifacts(out) {
				record(Result{name, Cached, ""})
				continue
			}
			if err := os.RemoveAll(out); err != nil {
				return results, err
			}
			if err := os.MkdirAll(out, 0o755); err != nil {
				return results, err
			}
			err = b.Build(ctx, name, src, out)
			if err == nil && !HasArtifacts(out) {
				err = errors.New("build produced no .deb files")
			}
			if err != nil {
				delete(state, name)
				record(Result{name, Failed, err.Error()})
			} else {
				state[name] = key
				record(Result{name, Built, ""})
			}
			if err := saveState(b.StateFile, state); err != nil {
				return results, err
			}
		}
	}
	return results, nil
}
```

- [ ] **Step 7: Run the unit tests to verify they pass**

Run: `go test ./internal/fetch/ ./internal/workspace/ ./internal/build/ -v`
Expected: all `PASS`. On macOS, `TestCaseSensitiveOnLinux` is skipped.

- [ ] **Step 8: Write `cmd/distro-build/main.go`** (Task 11 adds `repo` and apt mirroring; the stubs below are replaced there)

```go
// Command distro-build builds the NuDanOS package set.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/nudanos/distro/internal/build"
	"github.com/nudanos/distro/internal/control"
	"github.com/nudanos/distro/internal/engine"
	"github.com/nudanos/distro/internal/fetch"
	"github.com/nudanos/distro/internal/manifest"
	"github.com/nudanos/distro/internal/plan"
	"github.com/nudanos/distro/internal/workspace"
)

type localFlags map[string]string

func (l localFlags) String() string {
	var s []string
	for k, v := range l {
		s = append(s, k+"="+v)
	}
	sort.Strings(s)
	return strings.Join(s, ",")
}

func (l localFlags) Set(v string) error {
	name, dir, ok := strings.Cut(v, "=")
	if !ok || name == "" || dir == "" {
		return fmt.Errorf("want name=dir, got %q", v)
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	l[name] = abs
	return nil
}

type app struct {
	manifest, work, image, builderDir, gnupg, key string
	eng                                           engine.Engine
	local                                         localFlags
}

func (a *app) outRoot() string { return filepath.Join(a.work, "out") }

func (a *app) checkWork() error {
	ok, err := workspace.CaseSensitive(a.work)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("work directory %s is on a case-insensitive filesystem; DANOS sources contain "+
			"names that differ only by case. Use a case-sensitive volume (see docs/workspace.md)", a.work)
	}
	return nil
}

// fetch mirrors every ready apt entry, then checks out every ready danos entry
// (or uses its -local override) and returns the source directory for each.
func (a *app) fetch(ctx context.Context, m *manifest.Manifest) (map[string]string, error) {
	if err := a.checkWork(); err != nil {
		return nil, err
	}
	if err := a.mirror(ctx, m); err != nil {
		return nil, err
	}
	dirs := map[string]string{}
	for _, e := range m.Ready(manifest.Danos) {
		if d, ok := a.local[e.Name]; ok {
			dirs[e.Name] = d
			continue
		}
		d := filepath.Join(a.work, "src", e.Name)
		fmt.Fprintf(os.Stderr, "==> fetch %s@%s\n", e.Name, e.Ref)
		if err := fetch.Git(ctx, e.Repo, e.Ref, d, os.Stderr); err != nil {
			return nil, err
		}
		dirs[e.Name] = d
	}
	for name := range a.local {
		if _, ok := dirs[name]; !ok {
			return nil, fmt.Errorf("-local %s: no ready danos entry with that name", name)
		}
	}
	return dirs, nil
}

// mirror is replaced in Task 11.
func (a *app) mirror(ctx context.Context, m *manifest.Manifest) error { return nil }

func graph(dirs map[string]string) (*plan.Graph, [][]string, error) {
	srcs := map[string]*control.Source{}
	for name, d := range dirs {
		s, err := control.ReadSource(filepath.Join(d, "debian", "control"))
		if err != nil {
			return nil, nil, err
		}
		srcs[name] = s
	}
	g := plan.Build(srcs)
	tiers, err := g.Tiers()
	return g, tiers, err
}

func (a *app) containerBuild() build.Func {
	uid, gid := strconv.Itoa(os.Getuid()), strconv.Itoa(os.Getgid())
	return func(ctx context.Context, name, src, out string) error {
		return a.eng.Run(ctx, engine.RunSpec{
			Image: a.image,
			Mounts: []engine.Mount{
				{Host: src, Container: "/src", ReadOnly: true},
				{Host: a.outRoot(), Container: "/pool", ReadOnly: true},
				{Host: out, Container: "/out"},
			},
			Env: map[string]string{"PKG": name, "HOST_UID": uid, "HOST_GID": gid},
			Cmd: []string{"/usr/local/bin/build-package"},
		}, os.Stderr, os.Stderr)
	}
}

func (a *app) run(ctx context.Context, cmd string) error {
	switch cmd {
	case "builder":
		return a.eng.BuildImage(ctx, a.builderDir, a.image, os.Stderr, os.Stderr)
	case "fetch", "plan", "build":
	case "repo":
		return a.repo(ctx)
	default:
		return fmt.Errorf("unknown command %q", cmd)
	}
	m, err := manifest.Load(a.manifest)
	if err != nil {
		return err
	}
	dirs, err := a.fetch(ctx, m)
	if err != nil || cmd == "fetch" {
		return err
	}
	g, tiers, err := graph(dirs)
	if err != nil {
		return err
	}
	if cmd == "plan" {
		for i, t := range tiers {
			fmt.Printf("tier %d (%d): %s\n", i, len(t), strings.Join(t, " "))
		}
		return nil
	}
	salt, err := build.TreeHash(a.builderDir)
	if err != nil {
		return fmt.Errorf("hashing builder dir: %w", err)
	}
	b := &build.Builder{SrcDirs: dirs, OutRoot: a.outRoot(), StateFile: filepath.Join(a.work, "state.json"),
		KeySalt: a.image + ":" + salt, Build: a.containerBuild(), Log: os.Stderr}
	results, err := b.Run(ctx, tiers, g.Deps)
	if err != nil {
		return err
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	failed := 0
	for _, r := range results {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", r.Name, r.Status, r.Detail)
		if r.Status == build.Failed || r.Status == build.Skipped {
			failed++
		}
	}
	tw.Flush()
	if failed > 0 {
		return fmt.Errorf("%d of %d packages failed or were skipped", failed, len(results))
	}
	return nil
}

// repo is replaced in Task 11.
func (a *app) repo(ctx context.Context) error { return fmt.Errorf("repo: not available yet") }

func main() {
	home, _ := os.UserHomeDir()
	a := &app{local: localFlags{}}
	flag.StringVar(&a.manifest, "manifest", "manifest.yaml", "path to manifest.yaml")
	flag.StringVar(&a.work, "work", "work", "work directory (must be case-sensitive)")
	engineBin := flag.String("engine", "docker", "container engine: docker or podman")
	flag.StringVar(&a.image, "image", "nudanos/builder:trixie", "builder image tag")
	flag.StringVar(&a.builderDir, "builder-dir", "builder", "directory holding the builder Dockerfile")
	flag.StringVar(&a.gnupg, "gnupg", filepath.Join(home, ".nudanos", "gnupg"), "GnuPG home with the archive signing key (repo)")
	flag.StringVar(&a.key, "key", "", "archive signing key fingerprint (repo)")
	flag.Var(a.local, "local", "use a local checkout for a ready danos entry: name=dir (repeatable)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: distro-build [flags] builder|fetch|plan|build|repo\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	if *engineBin != "docker" && *engineBin != "podman" {
		fmt.Fprintln(os.Stderr, "error: -engine must be docker or podman")
		os.Exit(2)
	}
	a.eng = engine.Engine{Bin: *engineBin}
	abs, err := filepath.Abs(a.work)
	if err == nil {
		a.work = abs
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := a.run(ctx, flag.Arg(0)); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 9: Verify the case-insensitive refusal (Review Focus 1) and `plan` against the real org**

```bash
go build -o distro-build ./cmd/distro-build
./distro-build -work /tmp/nudanos-ci-check plan; echo "exit=$?"
```
Expected: `error: work directory /tmp/nudanos-ci-check is on a case-insensitive filesystem ...`, `exit=1`. `/tmp` is on the Mac's default volume.

```bash
./distro-build -work /Volumes/nudanos/work plan
```
Expected: `error: ... no branch or tag "trixie"`. The pilot `trixie` branches arrive in Task 12, so this proves fetch reaches `github.com/nudanos`.

- [ ] **Step 10: Commit**

```bash
git add internal/fetch internal/workspace internal/build cmd/distro-build
git commit -s -m "distro-build: fetch, case check, cached ordered builds, CLI

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 11: FRR mirroring, signed apt repo, archive key

**Files:**
- Create: `internal/aptmirror/aptmirror.go`, `internal/aptmirror/aptmirror_test.go`
- Create: `internal/aptrepo/aptrepo.go`, `internal/aptrepo/aptrepo_test.go`
- Create: `builder/mirror-apt.sh`, `builder/make-repo.sh`, `tools/new-archive-key.sh`
- Create: `keys/nudanos-archive.asc`
- Modify: `builder/Dockerfile` (copy the two scripts), `cmd/distro-build/main.go` (replace the `mirror` and `repo` stubs)

**Interfaces:**
- Consumes: `manifest.Entry` (apt kind), `engine`, `build.HasArtifacts`.
- Produces:
  - `aptmirror.Key(e manifest.Entry) string`
  - `aptmirror.Spec(image string, e manifest.Entry, outDir string, uid, gid int) engine.RunSpec`
  - `aptmirror.Mirror(ctx context.Context, eng engine.Engine, image string, e manifest.Entry, outRoot, stateFile string, log io.Writer) (cached bool, err error)`
  - `aptrepo.Spec(image, outRoot, repoDir, gnupgDir, keyID string, uid, gid int) engine.RunSpec`
  - `work/repo/` with `dists/trixie/{InRelease,Release,Release.gpg}`, `main/binary-amd64/Packages*`, `main/source/Sources*`, `pool/main/`, `nudanos-archive-keyring.asc`

- [ ] **Step 1: Write the failing tests**

`internal/aptmirror/aptmirror_test.go`:
```go
package aptmirror

import (
	"reflect"
	"testing"

	"github.com/nudanos/distro/internal/engine"
	"github.com/nudanos/distro/internal/manifest"
)

var frr = manifest.Entry{Name: "frr", Kind: manifest.Apt,
	Source: "https://deb.frrouting.org/frr trixie frr-stable", Key: "https://deb.frrouting.org/frr/keys.gpg",
	Packages: map[string]string{"libyang3": "3.13.6-1~deb13u1", "frr": "10.7.1-0~deb13u1"}}

func TestSpec(t *testing.T) {
	got := Spec("img", frr, "/w/out/frr", 501, 20)
	want := engine.RunSpec{Image: "img",
		Mounts: []engine.Mount{{Host: "/w/out/frr", Container: "/out"}},
		Env: map[string]string{"SOURCE": frr.Source, "KEY_URL": frr.Key,
			"PINS": "frr=10.7.1-0~deb13u1 libyang3=3.13.6-1~deb13u1", "HOST_UID": "501", "HOST_GID": "20"},
		Cmd: []string{"/usr/local/bin/mirror-apt"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Spec =\n %+v\nwant\n %+v", got, want)
	}
}

func TestKeyChangesWithPins(t *testing.T) {
	k1 := Key(frr)
	bumped := frr
	bumped.Packages = map[string]string{"libyang3": "3.13.6-1~deb13u1", "frr": "10.7.2-0~deb13u1"}
	if Key(bumped) == k1 {
		t.Error("pin change did not change key")
	}
	if Key(frr) != k1 {
		t.Error("key not stable")
	}
}
```

`internal/aptrepo/aptrepo_test.go`:
```go
package aptrepo

import (
	"reflect"
	"testing"

	"github.com/nudanos/distro/internal/engine"
)

func TestSpec(t *testing.T) {
	got := Spec("img", "/w/out", "/w/repo", "/h/gnupg", "ABCD", 501, 20)
	want := engine.RunSpec{Image: "img",
		Mounts: []engine.Mount{{Host: "/w/out", Container: "/pool", ReadOnly: true},
			{Host: "/w/repo", Container: "/repo"}, {Host: "/h/gnupg", Container: "/gnupg", ReadOnly: true}},
		Env:     map[string]string{"KEY_ID": "ABCD", "SUITE": "trixie", "HOST_UID": "501", "HOST_GID": "20"},
		Network: "none",
		Cmd:     []string{"/usr/local/bin/make-repo"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Spec =\n %+v\nwant\n %+v", got, want)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/aptmirror/ ./internal/aptrepo/`
Expected: FAIL with `undefined: Spec`

- [ ] **Step 3: Write `internal/aptmirror/aptmirror.go`**

```go
// Package aptmirror copies pinned packages from third-party apt repositories
// (such as FRR's) into the build output, so releases are reproducible.
package aptmirror

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/nudanos/distro/internal/build"
	"github.com/nudanos/distro/internal/engine"
	"github.com/nudanos/distro/internal/manifest"
)

func pins(e manifest.Entry) []string {
	var p []string
	for pkg, ver := range e.Packages {
		p = append(p, pkg+"="+ver)
	}
	sort.Strings(p)
	return p
}

// Key identifies an entry's content; it changes when source, key or pins change.
func Key(e manifest.Entry) string {
	h := sha256.Sum256([]byte(e.Source + "\x00" + e.Key + "\x00" + strings.Join(pins(e), " ")))
	return hex.EncodeToString(h[:])
}

// Spec is the container run that downloads e's pinned binaries and sources into outDir.
func Spec(image string, e manifest.Entry, outDir string, uid, gid int) engine.RunSpec {
	return engine.RunSpec{Image: image,
		Mounts: []engine.Mount{{Host: outDir, Container: "/out"}},
		Env: map[string]string{"SOURCE": e.Source, "KEY_URL": e.Key, "PINS": strings.Join(pins(e), " "),
			"HOST_UID": strconv.Itoa(uid), "HOST_GID": strconv.Itoa(gid)},
		Cmd: []string{"/usr/local/bin/mirror-apt"}}
}

func readState(path string) (map[string]string, error) {
	st := map[string]string{}
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return st, nil
	}
	if err != nil {
		return nil, err
	}
	return st, json.Unmarshal(b, &st)
}

// Mirror downloads e into outRoot/<name> unless the stored key matches and the
// .debs are present. It verifies every pinned package arrived.
func Mirror(ctx context.Context, eng engine.Engine, image string, e manifest.Entry,
	outRoot, stateFile string, log io.Writer) (bool, error) {
	st, err := readState(stateFile)
	if err != nil {
		return false, err
	}
	out := filepath.Join(outRoot, e.Name)
	key := Key(e)
	if st[e.Name] == key && build.HasArtifacts(out) {
		return true, nil
	}
	if err := os.RemoveAll(out); err != nil {
		return false, err
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return false, err
	}
	if err := eng.Run(ctx, Spec(image, e, out, os.Getuid(), os.Getgid()), log, log); err != nil {
		return false, fmt.Errorf("mirror %s: %w", e.Name, err)
	}
	for pkg := range e.Packages {
		if m, _ := filepath.Glob(filepath.Join(out, pkg+"_*.deb")); len(m) == 0 {
			return false, fmt.Errorf("mirror %s: %s was not downloaded", e.Name, pkg)
		}
	}
	st[e.Name] = key
	b, _ := json.MarshalIndent(st, "", " ")
	return false, os.WriteFile(stateFile, b, 0o644)
}
```

- [ ] **Step 4: Write `internal/aptrepo/aptrepo.go`**

```go
// Package aptrepo publishes build output as a signed apt repository.
package aptrepo

import (
	"strconv"

	"github.com/nudanos/distro/internal/engine"
)

// Spec is the (network-less) container run that indexes outRoot into repoDir
// and signs it with keyID from gnupgDir.
func Spec(image, outRoot, repoDir, gnupgDir, keyID string, uid, gid int) engine.RunSpec {
	return engine.RunSpec{Image: image,
		Mounts: []engine.Mount{
			{Host: outRoot, Container: "/pool", ReadOnly: true},
			{Host: repoDir, Container: "/repo"},
			{Host: gnupgDir, Container: "/gnupg", ReadOnly: true},
		},
		Env: map[string]string{"KEY_ID": keyID, "SUITE": "trixie",
			"HOST_UID": strconv.Itoa(uid), "HOST_GID": strconv.Itoa(gid)},
		Network: "none",
		Cmd:     []string{"/usr/local/bin/make-repo"}}
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/aptmirror/ ./internal/aptrepo/ -v`
Expected: 3 tests `PASS`

- [ ] **Step 6: Write the two container scripts and add them to the image**

`builder/mirror-apt.sh`:
```bash
#!/bin/bash
# Download pinned binary packages and their source packages from a third-party
# apt repository into /out.
# Env: SOURCE="URL SUITE COMPONENT", KEY_URL, PINS="pkg=version ...", HOST_UID, HOST_GID
set -euo pipefail
: "${SOURCE:?}" "${KEY_URL:?}" "${PINS:?}" "${HOST_UID:=0}" "${HOST_GID:=0}"
read -r url suite comp <<<"$SOURCE"
curl -fsSL "$KEY_URL" -o /tmp/key
if grep -q -- '-----BEGIN PGP' /tmp/key; then
    gpg --dearmor < /tmp/key > /usr/share/keyrings/mirror.gpg
else
    cp /tmp/key /usr/share/keyrings/mirror.gpg
fi
cat > /etc/apt/sources.list.d/mirror.list <<EOF
deb [signed-by=/usr/share/keyrings/mirror.gpg] $url $suite $comp
deb-src [signed-by=/usr/share/keyrings/mirror.gpg] $url $suite $comp
EOF
apt-get update
cd /out
declare -A seen
for pin in $PINS; do
    apt-get download "$pin"
    src=$(apt-cache show "$pin" | awk '/^Source:/ {print $2; exit}')
    src=${src:-${pin%%=*}}
    if [ -z "${seen[$src]:-}" ]; then
        seen[$src]=1
        apt-get source --download-only "$src=${pin#*=}"
    fi
done
chown -R "$HOST_UID:$HOST_GID" /out
```

`builder/make-repo.sh`:
```bash
#!/bin/bash
# Index every .deb and source package under /pool into a signed apt repository in /repo.
# Env: KEY_ID (fingerprint in /gnupg), SUITE, HOST_UID, HOST_GID
set -euo pipefail
: "${KEY_ID:?}" "${SUITE:=trixie}" "${HOST_UID:=0}" "${HOST_GID:=0}"
comp=main arch=amd64
rm -rf /repo/dists /repo/pool
mkdir -p "/repo/pool/$comp" "/repo/dists/$SUITE/$comp/binary-$arch" "/repo/dists/$SUITE/$comp/source"
find /pool -type f \( -name '*.deb' -o -name '*.dsc' -o -name '*.tar.*' -o -name '*.diff.gz' \) \
    -exec cp -t "/repo/pool/$comp" {} +
cd /repo
apt-ftparchive packages "pool/$comp" > "dists/$SUITE/$comp/binary-$arch/Packages"
apt-ftparchive sources "pool/$comp" > "dists/$SUITE/$comp/source/Sources"
for f in "dists/$SUITE/$comp/binary-$arch/Packages" "dists/$SUITE/$comp/source/Sources"; do
    gzip -9kf "$f"
    xz -kf "$f"
done
apt-ftparchive \
    -o APT::FTPArchive::Release::Origin=NuDanOS \
    -o APT::FTPArchive::Release::Label=NuDanOS \
    -o APT::FTPArchive::Release::Suite="$SUITE" \
    -o APT::FTPArchive::Release::Codename="$SUITE" \
    -o APT::FTPArchive::Release::Architectures="$arch" \
    -o APT::FTPArchive::Release::Components="$comp" \
    release "dists/$SUITE" > /tmp/Release
mv /tmp/Release "dists/$SUITE/Release"
cp -r /gnupg /tmp/gnupg && chmod 700 /tmp/gnupg
export GNUPGHOME=/tmp/gnupg
gpg --batch --yes --default-key "$KEY_ID" --clearsign -o "dists/$SUITE/InRelease" "dists/$SUITE/Release"
gpg --batch --yes --default-key "$KEY_ID" --armor --detach-sign -o "dists/$SUITE/Release.gpg" "dists/$SUITE/Release"
gpg --armor --export "$KEY_ID" > nudanos-archive-keyring.asc
chown -R "$HOST_UID:$HOST_GID" /repo
```

Append to `builder/Dockerfile`:
```dockerfile
COPY mirror-apt.sh /usr/local/bin/mirror-apt
COPY make-repo.sh /usr/local/bin/make-repo
RUN chmod 0755 /usr/local/bin/mirror-apt /usr/local/bin/make-repo
```

- [ ] **Step 7: Replace the `mirror` and `repo` stubs in `cmd/distro-build/main.go`**

Add the imports `"github.com/nudanos/distro/internal/aptmirror"` and `"github.com/nudanos/distro/internal/aptrepo"`. Replace the two stub functions with:
```go
func (a *app) mirror(ctx context.Context, m *manifest.Manifest) error {
	for _, e := range m.Ready(manifest.Apt) {
		cached, err := aptmirror.Mirror(ctx, a.eng, a.image, e, a.outRoot(),
			filepath.Join(a.work, "mirror-state.json"), os.Stderr)
		if err != nil {
			return err
		}
		status := "downloaded"
		if cached {
			status = "cached"
		}
		fmt.Fprintf(os.Stderr, "==> mirror %s %s\n", e.Name, status)
	}
	return nil
}

func (a *app) repo(ctx context.Context) error {
	if a.key == "" {
		return fmt.Errorf("repo: -key <fingerprint> is required")
	}
	if err := a.checkWork(); err != nil {
		return err
	}
	dir := filepath.Join(a.work, "repo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return a.eng.Run(ctx, aptrepo.Spec(a.image, a.outRoot(), dir, a.gnupg, a.key, os.Getuid(), os.Getgid()),
		os.Stderr, os.Stderr)
}
```

- [ ] **Step 8: Write `tools/new-archive-key.sh`, create the key, commit the public half**

```bash
#!/bin/bash
# Create the NuDanOS archive signing key (Ed25519, no passphrase, 3 years) in a
# local GnuPG home and print its fingerprint. The private key never leaves that
# directory except when the user copies it into a CI secret.
set -euo pipefail
GNUPG=${GNUPG:-$HOME/.nudanos/gnupg}
ENGINE=${ENGINE:-docker}
mkdir -p "$GNUPG" && chmod 700 "$GNUPG"
$ENGINE run --rm -v "$GNUPG":/gnupg -e GNUPGHOME=/gnupg nudanos/builder:trixie \
    gpg --batch --passphrase '' --quick-gen-key "NuDanOS Archive Signing Key <jon@fernandez.tech>" ed25519 sign 3y
$ENGINE run --rm -v "$GNUPG":/gnupg -e GNUPGHOME=/gnupg nudanos/builder:trixie \
    gpg --list-keys --with-colons | awk -F: '/^fpr/ {print $10; exit}'
```

```bash
chmod +x tools/new-archive-key.sh builder/*.sh
./distro-build builder
KEY=$(tools/new-archive-key.sh | tail -1); echo "$KEY" | tee keys/FINGERPRINT
docker run --rm -v ~/.nudanos/gnupg:/gnupg -e GNUPGHOME=/gnupg nudanos/builder:trixie gpg --armor --export "$KEY" > keys/nudanos-archive.asc
```
Expected: a 40-hex-character fingerprint, and `keys/nudanos-archive.asc` beginning `-----BEGIN PGP PUBLIC KEY BLOCK-----`

- [ ] **Step 9: Mirror FRR for real, and sign a repo holding just FRR**

```bash
go build -o distro-build ./cmd/distro-build
./distro-build -work /Volumes/nudanos/work fetch; ls /Volumes/nudanos/work/out/frr/
```
`fetch` mirrors FRR first, then stops at `no branch or tag "trixie"` for the pilot repos (their branches arrive in Task 12). That error is expected here.

Expected in `out/frr/`: `frr_10.7.1-0~deb13u1_amd64.deb`, `frr-pythontools_...`, `frr-snmp_...`, `libyang3_3.13.6-1~deb13u1_amd64.deb`, plus `frr_10.7.1-0~deb13u1.dsc`/`.tar.*` and `libyang_3.13.6-1~deb13u1.dsc`/`.tar.*`

```bash
./distro-build -work /Volumes/nudanos/work -key "$(cat keys/FINGERPRINT)" repo
ls /Volumes/nudanos/work/repo/dists/trixie/
```
Expected: `InRelease  Release  Release.gpg  main`

- [ ] **Step 10: Commit**

```bash
git add internal/aptmirror internal/aptrepo builder tools/new-archive-key.sh keys cmd/distro-build
git commit -s -m "distro-build: mirror pinned apt packages, publish signed repo, archive key

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 12: Pilot port and end-to-end integration test

**Files:**
- Modify in `nudanos/dh-yang`, `nudanos/dh-vci`, `nudanos/vyatta-util` (new `trixie` branch): `debian/control`, `debian/changelog`; delete `debian/compat`; `vyatta-util` also changes `debian/rules`
- Create: `tests/integration/pilot.sh`

**Interfaces:**
- Consumes: the `distro-build` CLI (Tasks 10–11), the signing key (Task 11).
- Produces: `trixie` branches in the three repos (the Debian 13 port checklist applied), and `tests/integration/pilot.sh` (env `WORK`, `KEY`, optional `ENGINE`, `GNUPG`). Exit 0 means build → sign → install worked.

- [ ] **Step 1: Write the integration test first**

`tests/integration/pilot.sh`:
```bash
#!/bin/bash
# End-to-end pilot: build every ready package, publish the signed repo, then install
# from it in a clean Debian 13 container that trusts only Debian and NuDanOS.
set -euo pipefail
cd "$(dirname "$0")/../.."
: "${WORK:?set WORK to a case-sensitive directory}" "${KEY:?set KEY to the archive key fingerprint}"
ENGINE=${ENGINE:-docker}
GNUPG=${GNUPG:-$HOME/.nudanos/gnupg}
go build -o distro-build ./cmd/distro-build
./distro-build -work "$WORK" -engine "$ENGINE" builder
./distro-build -work "$WORK" -engine "$ENGINE" build
./distro-build -work "$WORK" -engine "$ENGINE" -gnupg "$GNUPG" -key "$KEY" repo
$ENGINE run --rm -v "$WORK/repo":/repo:ro debian:trixie bash -euxc '
    apt-get update
    apt-get install -y --no-install-recommends ca-certificates gpg
    gpg --dearmor < /repo/nudanos-archive-keyring.asc > /usr/share/keyrings/nudanos.gpg
    echo "deb [signed-by=/usr/share/keyrings/nudanos.gpg] file:/repo trixie main" > /etc/apt/sources.list.d/nudanos.list
    apt-get update
    apt-get install -y --no-install-recommends dh-yang dh-vci vyatta-util frr
    test -x /usr/bin/dh_yang
    test -f /usr/share/perl5/Debian/Debhelper/Sequence/yang.pm
    test -x /usr/bin/dh_vci_enable
    test -f /usr/share/perl5/Debian/Debhelper/Sequence/vci.pm
    test -x /opt/vyatta/sbin/vyatta-validate-type
    dpkg-query -W -f="\${Version}\n" frr | grep -qx "10.7.1-0~deb13u1"
    dpkg-query -W -f="\${Version}\n" libyang3 | grep -qx "3.13.6-1~deb13u1"
'
echo "pilot: OK"
```

- [ ] **Step 2: Run it to verify it fails**

Run: `chmod +x tests/integration/pilot.sh && WORK=/Volumes/nudanos/work KEY=$(cat keys/FINGERPRINT) tests/integration/pilot.sh`
Expected: FAIL with `no branch or tag "trixie"`

- [ ] **Step 3: Port `dh-yang` and `dh-vci`** (same checklist for both; shown for `dh-yang`, then repeat with `dh-vci`)

```bash
cd /Volumes/nudanos && git clone git@github.com:nudanos/dh-yang.git port-dh-yang && cd port-dh-yang
git checkout -b trixie
git rm -q debian/compat
```
Edit `debian/control` so the source paragraph reads exactly as below. For `dh-vci`, use `Section: devel` and the `dh-vci` names, and keep each file's binary paragraph unchanged:
```text
Source: dh-yang
Section: admin
Priority: optional
Maintainer: NuDanOS Maintainers <jon@fernandez.tech>
Build-Depends: debhelper-compat (= 13)
Standards-Version: 4.7.2
Rules-Requires-Root: no
Vcs-Git: https://github.com/nudanos/dh-yang.git
Vcs-Browser: https://github.com/nudanos/dh-yang
```
Then add the changelog entry (for `dh-vci`, the version becomes `0.5`):
```bash
DEBFULLNAME="NuDanOS Maintainers" DEBEMAIL="jon@fernandez.tech" \
  docker run --rm -e DEBFULLNAME -e DEBEMAIL -v "$PWD":/p -w /p nudanos/builder:trixie \
  dch --newversion 0.4 --distribution trixie --force-distribution \
  "Port to Debian 13: debhelper-compat 13, Standards-Version 4.7.2, Rules-Requires-Root, NuDanOS Vcs."
git add -A && git commit -s -m "Port to Debian 13 (trixie)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

- [ ] **Step 4: Port `vyatta-util`**

```bash
cd /Volumes/nudanos && git clone git@github.com:nudanos/vyatta-util.git port-vyatta-util && cd port-vyatta-util
git checkout -b trixie && git rm -q debian/compat
```
Source paragraph of `debian/control`:
```text
Source: vyatta-util
Section: net
Priority: optional
Maintainer: NuDanOS Maintainers <jon@fernandez.tech>
Build-Depends: debhelper-compat (= 13), liburiparser-dev
Standards-Version: 4.7.2
Rules-Requires-Root: no
Vcs-Git: https://github.com/nudanos/vyatta-util.git
Vcs-Browser: https://github.com/nudanos/vyatta-util
```
In the binary paragraphs: change `Section: contrib/libdevel` to `Section: libdevel`, and delete the `Priority: optional` line under `libvyatta-util-dev` (the source default applies). In `debian/rules`: replace `dh $@ --with autotools_dev` with `dh $@`, and delete the `override_dh_auto_configure` block's `mkdir -p m4` comment and command lines. Keep `debian/autogen.sh` and `dh_auto_configure -- --prefix=/opt/vyatta --libdir=/usr/lib --includedir=/usr/include`. Then:
```bash
DEBFULLNAME="NuDanOS Maintainers" DEBEMAIL="jon@fernandez.tech" \
  docker run --rm -e DEBFULLNAME -e DEBEMAIL -v "$PWD":/p -w /p nudanos/builder:trixie \
  dch --newversion 0.30 --distribution trixie --force-distribution \
  "Port to Debian 13: debhelper-compat 13 (drops autotools_dev addon), Standards-Version 4.7.2, NuDanOS Vcs."
git add -A && git commit -s -m "Port to Debian 13 (trixie)

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

- [ ] **Step 5: Build the three ports locally before publishing anything**

```bash
cd /Volumes/nudanos/distro
WORK=/Volumes/nudanos/work
./distro-build -work $WORK -local dh-yang=../port-dh-yang -local dh-vci=../port-dh-vci -local vyatta-util=../port-vyatta-util build
```
Expected: `dh-yang built`, `dh-vci built`, `vyatta-util built`, exit 0. If lintian or the build fails, fix the port (the failure names the tag or step), commit, and re-run until green. Record each fix in the commit message.

- [ ] **Step 6: STOP for go-ahead, then push the three `trixie` branches**

```bash
for r in dh-yang dh-vci vyatta-util; do git -C /Volumes/nudanos/port-$r push -u origin trixie; done
```

- [ ] **Step 7: Run the integration test**

Run: `WORK=/Volumes/nudanos/work KEY=$(cat keys/FINGERPRINT) tests/integration/pilot.sh`
Expected: ends with `pilot: OK`

- [ ] **Step 8: Verify `plan` sees the real order**

Run: `./distro-build -work /Volumes/nudanos/work plan`
Expected: `tier 0 (3): dh-vci dh-yang vyatta-util`. None of the three build-depends on another, and `frr` is mirrored rather than planned.

- [ ] **Step 9: Commit the test**

```bash
git add tests/integration/pilot.sh && git commit -s -m "tests: end-to-end pilot build, sign and install

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 13: CI: package workflow, nightly pilot, secrets

**Files:**
- Create: `.github/workflows/package.yml`, `.github/workflows/nightly.yml`
- Create in each pilot repo's `trixie` branch: `.github/workflows/package.yml` (the caller)

**Interfaces:**
- Consumes: `distro-build -local` (Task 10), `tests/integration/pilot.sh` (Task 12), key fingerprint `keys/FINGERPRINT` (Task 11).
- Produces:
  - Reusable workflow `nudanos/distro/.github/workflows/package.yml@main`, input `package` (manifest name).
  - Nightly run that uploads `work/repo` as artifact `apt-repo`.
  - Org secret `NUDANOS_ARCHIVE_KEY` (armored private key), set by the user.

- [ ] **Step 1: Write the reusable package workflow**

`.github/workflows/package.yml`:
```yaml
name: package
on:
  workflow_call:
    inputs:
      package:
        description: manifest name of the calling package repository
        required: true
        type: string
permissions:
  contents: read
jobs:
  build:
    runs-on: ubuntu-24.04
    steps:
      - uses: actions/checkout@v4
        with:
          repository: nudanos/distro
          path: distro
      - uses: actions/checkout@v4
        with:
          path: pkg
      - uses: actions/setup-go@v5
        with:
          go-version-file: distro/go.mod
      - name: Build against the manifest, using this checkout for the package
        working-directory: distro
        run: |
          go build -o distro-build ./cmd/distro-build
          ./distro-build -work "$RUNNER_TEMP/work" builder
          ./distro-build -work "$RUNNER_TEMP/work" -local "${{ inputs.package }}=$GITHUB_WORKSPACE/pkg" build
      - uses: actions/upload-artifact@v4
        if: always()
        with:
          name: debs-${{ inputs.package }}
          path: ${{ runner.temp }}/work/out/${{ inputs.package }}/
          if-no-files-found: ignore
```

- [ ] **Step 2: Write the nightly workflow**

`.github/workflows/nightly.yml`:
```yaml
name: nightly
on:
  schedule:
    - cron: "17 6 * * *"
  workflow_dispatch:
permissions:
  contents: read
jobs:
  pilot:
    runs-on: ubuntu-24.04
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - name: Import archive signing key
        env:
          NUDANOS_ARCHIVE_KEY: ${{ secrets.NUDANOS_ARCHIVE_KEY }}
        run: |
          mkdir -p "$HOME/.nudanos/gnupg" && chmod 700 "$HOME/.nudanos/gnupg"
          printf '%s\n' "$NUDANOS_ARCHIVE_KEY" | GNUPGHOME="$HOME/.nudanos/gnupg" gpg --batch --import
      - name: Build, sign, install
        run: WORK="$RUNNER_TEMP/work" KEY="$(cat keys/FINGERPRINT)" tests/integration/pilot.sh
      - uses: actions/upload-artifact@v4
        with:
          name: apt-repo
          path: ${{ runner.temp }}/work/repo/
```

- [ ] **Step 3: Commit and push the `distro` workflows**

```bash
git add .github/workflows && git commit -s -m "ci: reusable package workflow and nightly pilot

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
git push origin main
```

- [ ] **Step 4: USER ACTION: set the signing-key secret.** The user runs this in their own terminal. It reads the private key straight from the local GnuPG home, so it never passes through the agent:

```bash
docker run --rm -v ~/.nudanos/gnupg:/gnupg -e GNUPGHOME=/gnupg nudanos/builder:trixie \
  gpg --armor --export-secret-keys "$(cat /Volumes/nudanos/distro/keys/FINGERPRINT)" \
  | gh secret set NUDANOS_ARCHIVE_KEY --org nudanos --visibility all
```
Run: `gh secret list --org nudanos`
Expected: `NUDANOS_ARCHIVE_KEY`

- [ ] **Step 5: Add the caller workflow to each pilot repo** (`dh-yang` shown; repeat for `dh-vci` and `vyatta-util`, changing `package:`)

`.github/workflows/package.yml` on the `trixie` branch:
```yaml
name: package
on:
  push:
    branches: [trixie]
  pull_request:
    branches: [trixie]
jobs:
  build:
    uses: nudanos/distro/.github/workflows/package.yml@main
    with:
      package: dh-yang
```
```bash
cd /Volumes/nudanos/port-dh-yang && mkdir -p .github/workflows
cat > .github/workflows/package.yml <<'YAML'
name: package
on:
  push:
    branches: [trixie]
  pull_request:
    branches: [trixie]
jobs:
  build:
    uses: nudanos/distro/.github/workflows/package.yml@main
    with:
      package: dh-yang
YAML
git add .github && git commit -s -m "ci: build with nudanos/distro package workflow

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>" && git push
```

- [ ] **Step 6: Verify CI**

```bash
for r in dh-yang dh-vci vyatta-util; do gh run watch -R nudanos/$r --exit-status "$(gh run list -R nudanos/$r -L 1 --json databaseId --jq '.[0].databaseId')"; done
gh workflow run nightly -R nudanos/distro && sleep 20
gh run watch -R nudanos/distro --exit-status "$(gh run list -R nudanos/distro -w nightly -L 1 --json databaseId --jq '.[0].databaseId')"
```
Expected: all four runs succeed, and the nightly log ends with `pilot: OK`

- [ ] **Step 7: Record completion in the spec's roadmap**

In `docs/superpowers/specs/2026-09-28-debian13-revival-design.md` §9, append to the `0: Launch` row: ` — done 2026-MM-DD (plan 1)` with the actual date. Commit with `git commit -s` and push.
