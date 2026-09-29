# Plan 1 follow-ups

Carried forward from executing `2026-09-28-nudanos-stage0-build-foundation.md`, so that plan 2 starts from them. They come from the execution ledger and the final whole-branch review.

## Must happen early in plan 2

- **`build [pkg…]` with build-dependency closure, plus nightly pool seeding and caching.** Spec §7.3 says package PRs build against the latest nightly repo; today they rebuild every ready package. Do this before more than ~10 packages are ready.
- **Per-build cost.** Index the pool once per tier instead of copying and re-indexing it per build, share an apt cache, set `DEB_BUILD_OPTIONS=parallel=N` and `Acquire::Retries`, and allow parallel builds within a tier.
- **Port `lintian-profile-vyatta` in 1.0** (the manifest says `later`), then replace the builder's `--suppress-tags dir-or-file-in-opt` with `--profile vyatta`.
- **Archive the `debian`/`drop` repos** (moved here from stage 0).

## Fixed in the final review

- deb822 field names case-sensitive (libvci Build-depends dropped edges) — TestFieldNamesAreCaseInsensitive RED→GREEN, control 5/5
- cache key ignoring build-dependencies — TestRunRebuildsDependentsWhenADependencyChanges RED (a=built b=cached) → GREEN, build 5/5
- duplicate producers resolved silently — TestBuildRejectsDuplicateProducers RED (Build had no error return) → GREEN, plan 4/4
- stale pool entries leaking into builds/repo — TestPruneRemovesOutputOfEntriesNoLongerReady RED (undefined Prune) → GREEN; wired into fetch and repo
- -engine podman ownership (rootless subuid chown) — TestOwnerIDs RED (undefined) → GREEN; all chown ids now via Engine.OwnerIDs. Not exercised with a real podman (none installed).
- builder cache salt not tied to the running image — TestBuildArgsLabelsTheImage/TestLabelArgs/TestCheckBuilder RED (undefined) → GREEN; live: unlabeled image refused, builder+build OK, edited builder/ refused, revert → cached
- (workflow side) signing key readable org-wide — nightly.yml runs in environment 'release'. Verified by CI config only; the environment, its main-only branch rule, the env secret and deleting the org secret are user actions (secrets are never handled by the agent). Until then the org secret remains readable org-wide.

## Rulings made during execution

- work directly on `main` of the brand-new, empty distro repo (no worktree) — the plan targets main of a repo that does not exist yet and the user approved the plan — cost if wrong: history rewrite on an unpublished repo.
- distro repo initialised and plan+spec copied at setup instead of Task 2 Step 1 / Task 4 Step 1 — the ledger scripts need the plan inside the repo — cost if wrong: none (Task 4 re-copy is a no-op).
- gofmt -w on manifest_test.go (plan's map literal alignment not gofmt-clean) — whitespace only — cost if wrong: none.
- Task 4 Step 1's copy of REVIEW.md, docs/data and tools/*.py done ahead of Task 8 (which consumes docs/data) because Tasks 3/4 wait on gh auth — cost if wrong: none.
- tasks 5-11 code written and unit-tested ahead of tasks 3-4 (which need gh auth) — they share no state with the launch — cost if wrong: none.
- empty root commit "Initialize NuDanOS distro repository" added as Task 2's BASE — task-done needs a valid BASE and the repo had no commits — cost if wrong: one extra empty commit.
- Task 3 (publish) deferred until the user's explicit go-ahead; Tasks 4-11 committed locally meanwhile (nothing pushed) — Task 3 is only a publish step with no files — cost if wrong: none.
- tasks 4-8 were implemented before being committed, so task-done's closing test runs execute against the working tree, which holds later tasks' code too. Each task's RED→GREEN was observed during development and recorded above — cost if wrong: a task's commit could depend on a later file; mitigated by `go build` checks at the T10/T11 commits and a full suite run after the last commit.
- build-package.sh also writes Packages.gz/.xz for the local pool — apt 3.0 logged 6 read errors + 6 symlink warnings per update without them (reproduced with empty and full pools; 0 after the fix; resolution worked either way) — cost if wrong: two extra files per build.
- plan Expected listed libyang_3.13.6-1~deb13u1.dsc; FRR's repo names that source package libyang3 (libyang3_3.13.6-1~deb13u1.dsc) — the code dedupes by the real Source field, so this is a wording error in the plan — cost if wrong: none. Verified: FRR mirror 4 debs + 2 sources; repo exit 0; gpgv Good signature; Packages 4, Sources 2.
- build-package.sh suppresses lintian tag dir-or-file-in-opt — DANOS installs to /opt/vyatta project-wide and lintian-profile-vyatta's main.profile disables exactly this tag; the spec asks for linting against that profile, which the plan ports in plan 2 — cost if wrong: an /opt install by a non-DANOS package would go unflagged until the profile lands.
- bumped actions/checkout, setup-go and upload-artifact to v7 in all three workflows (plan said v4/v5/v4) — GitHub flagged Node 20 deprecation; spec wants latest versions — cost if wrong: revert three lines.
- launch.py --verify compares ref counts strictly, so repos that already carry a pushed trixie branch show DIFF (+1); checked with a per-ref SHA diff instead — cost if wrong: none (diff shows nothing missing).
- #5 package PRs rebuild the whole ready set / no nightly seeding (spec §7.3 says "against the latest nightly repo") — deferred to plan 2 — no effect with 3 ready packages; plan 2 must add `build [pkg…]` with build-dep closure plus nightly pool seeding before enabling more than ~10 ready packages — cost if wrong: slow PR CI, and unrelated failures turning every package check red.
- #8 per-package cost (pool copy/re-index per build, no apt cache, serial, no parallel=) — deferred to plan 2 with #5 — no effect at 3 packages — cost if wrong: cold builds hit the 6 h runner limit early in plan 2.
- declined "CLI instead of Engine API client" — stands; plan global constraint chose the CLI (simpler, podman-compatible) — cost if wrong: none.
- declined "apt-get build-dep ./ instead of mk-build-deps" — stands; equivalent — cost if wrong: none.
- declined "dbgsym not published" — the reviewer's premise is wrong: trixie dbgsym packages are *.deb (libvyatta-util1-dbgsym_0.30_amd64.deb is in out/ and the repo); stands — cost if wrong: none.
- declined "networked, non-hermetic builds" — stands; spec asks for Debian 13 + security updates, not snapshot pinning — cost if wrong: less reproducible rebuilds.
- declined "over-approximated edges for arch/alternatives", ".diff.gz not copied", "lintian as root", "analysis tools not reviewed", "data files not read line by line", "launch history not re-audited", "image/test/check-updates out of scope" — all stand as the reviewer argued — cost if wrong: none for M1.
- declined "primary key used for signing in CI (suggest subkey)" — deferred to the Pages/public-repo step in plan 3, alongside the release environment — cost if wrong: a CI compromise exposes the primary key rather than a revocable subkey.

## Deferred minor findings

- aptmirror checks presence by name only (pkg_*.deb), is skipped on a cache hit, keeps no per-deb sha256, and the FRR key is fetched TOFU — pin the fingerprint.
- a transitive skip names the intermediate package, not the root failure.
- cancellation — no --init/--name, SIGTERM not handled; containers can outlive the CLI.
- no work-dir lock against concurrent runs.
- mirror-state.json is written non-atomically.
- fetch ignores a changed repo URL for existing clones (no set-url).
- `plan` requires Docker and network because fetch mirrors apt entries.
- launch.py — uncaught CalledProcessError; a transient gh repo view error is treated as "missing"; --verify counts refs only.
- manifest lintian-profile-vyatta is milestone later; spec §6 and the Task 12 ruling need it in 1.0.
- manifest Name not validated as a Debian source name, though it is used in paths.
- golang-jsouthworth-* entries lack the M1.1 vendored-module note.
- vyatta-util port relies implicitly on dh-autoreconf for autoconf/automake/libtool, and autoreconf runs twice.
- the key-handling workflow pins actions by tag, not SHA; callers use package.yml@main.
- no automated test for the case-insensitive refusal or for aptmirror.Mirror verification.
