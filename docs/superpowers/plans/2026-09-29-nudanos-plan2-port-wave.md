# NuDanOS Plan 2: Port Wave and Upstream Builds

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Every milestone-1.0 package builds on Debian 13 from `nudanos` sources: the 88 remaining DANOS-authored repos plus 8 packages rebuilt at their latest upstream release. It happens in dependency order, in CI, with `check-updates` watching for new upstream releases.

**Architecture:** First extend `distro-build`:
- `build [pkg…]` builds the named packages plus their build-dependency closure.
- The local pool is indexed once per tier, and builds within a tier run in parallel.
- A new `upstream` kind combines an upstream release tag with Debian's (or our) packaging.
- `3.0 (quilt)` sources build with a generated orig tarball.
- `check-updates` compares pins with upstream tags and apt indexes.

Then a stdlib-Python `port.py` applies the Debian 13 checklist mechanically, and the repos are ported in five waves: tier 0, tiers 1–2, the Go core, upstream builds, and the dataplane split. Each wave pushes `trixie` branches, flips `ready: true` in the manifest, and must be green in CI before the next starts.

**Tech Stack:** Go 1.26+ (`github.com/nudanos/distro`), Python 3.9 stdlib, Docker/Podman, Debian 13 tooling (debhelper 13.24, dpkg 1.22, lintian 2.122, apt 3.0, gbp 0.9.38), GitHub Actions.

**Spec:** `docs/superpowers/specs/2026-09-28-debian13-revival-design.md`. Follow-ups carried from plan 1: `docs/superpowers/plans/2026-09-28-plan1-followups.md`.

**Scope:**
- **In:** spec §4.3–§4.4, §6 and §7.1–§7.3 for checkpoint 1.0, and the plan 1 follow-ups marked "must happen early in plan 2".
- **Moved to plan 3 (ISO, boot and scenario tests), where they can actually be exercised:**
  - the `nudanos-router` meta-package
  - moving ISO hooks into maintainer scripts (§4.5)
  - the kernel and FRR commit audits (they matter only at runtime)
- **Deferred:** a shared apt archive cache. apt's archive lock does not tolerate concurrent builds; revisit with an apt proxy if cold builds become the bottleneck.

## Global Constraints

- The GitHub org is `nudanos`; Go module paths are `github.com/nudanos/<repo>`. Original branches and tags are never rewritten; porting happens on `trixie` branches.
- Debian packaging on `trixie` branches:
  - `debhelper-compat (= 13)`, `Standards-Version: 4.7.2`, `Rules-Requires-Root: no`
  - `Maintainer: NuDanOS Maintainers <jon@fernandez.tech>`
  - `Vcs-Git: https://github.com/nudanos/<repo>.git`, `Vcs-Browser: https://github.com/nudanos/<repo>`
- Changelog entries are signed `NuDanOS Maintainers <jon@fernandez.tech>`. Upstream-kind builds are versioned `[epoch:]<upstream>-0nudanos1`, and **an existing epoch in the packaging is always kept**.
- Builder base image `debian:trixie`. Go for package builds is `golang-go` from `trixie-backports` (Go 1.26).
- Package builds run their test suites, and a failing test fails the build. `lintian --fail-on error` gates every build, with `--profile vyatta` once `lintian-profile-vyatta` is in the pool.
- The work directory must be case-sensitive (`/Volumes/nudanos/work` on the Mac).
- Anything outward-facing needs the user's explicit go-ahead at the step that says **STOP**: creating repos, archiving repos, secrets and environments. Pushing `trixie` branches to existing `nudanos/*` repos is covered by the one go-ahead requested at plan review. The agent never types or handles secrets.
- Commits use DCO sign-off (`git commit -s`) and end with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- Never republish or copy `jsouthworth/danos-bootstrap` (unlicensed).

## Review Focus

1. **Irregular upstream tag histories.** mstpd's `0.05` must not beat `0.2.0`, ntpsec tags look like `NTPsec_1_2_5`, and release candidates exist. `check-updates` and version selection must pick the real latest per entry. (Task 4)
2. **Epochs.** keepalived is `1:2.3.3-1` in Debian. Our `2.4.3` build must be `1:2.4.3-0nudanos1`, or apt silently prefers Debian's older package. (Task 3)
3. **Reproducible prepared trees.** Preparing the same tag twice must hash identically, so the changelog date comes from the tag's commit, not the clock. Otherwise upstream builds never cache. (Task 3)
4. **Re-running `port.py` on an already-ported repo** must change nothing: no second changelog entry, no mangled `Build-Depends`, no lost hand edits. (Task 5)
5. **A failure during parallel builds** must still skip its dependents in later tiers, and concurrent completions must not corrupt `state.json`. (Task 2)

---

## File Structure

| File | Responsibility |
|---|---|
| `internal/plan/plan.go` | + `Closure`, `Filter` (build selection) |
| `internal/build/build.go` | + `Workers` (parallel within a tier), `BeforeTier` hook |
| `internal/manifest/manifest.go` | + upstream fields (`version`, `tag`, `tag_pattern`, `packaging_ref`), `audit` list |
| `internal/upstream/upstream.go` | Prepare an upstream-kind source tree (upstream tag + packaging `debian/` + patches + changelog) |
| `internal/versions/versions.go` | Tag-pattern matching and numeric version comparison |
| `internal/updates/updates.go` | `check-updates` for upstream and apt entries |
| `cmd/distro-build/main.go` | `build [pkg…]`, `-jobs`, index-per-tier, upstream fetch, `check-updates` |
| `builder/index-pool.sh` | Index `/pool` once per tier |
| `builder/build-package.sh` | Read the pool in place, quilt orig generation, parallel jobs, lintian profile |
| `builder/apt-retries` | apt retry configuration for the image |
| `tools/port.py`, `tools/test_port.py` | Mechanical Debian 13 port of one checkout |
| `.github/workflows/package.yml`, `nightly.yml`, `updates.yml` | Closure builds with cache; weekly update issue |
| `manifest.yaml` | Readiness, upstream pins, audits |
| `docs/porting.md` | The port procedure used by every wave |

---

### Task 1: Build selection: `distro-build build [pkg…]`

**Files:**
- Modify: `internal/plan/plan.go`, `internal/plan/plan_test.go`, `cmd/distro-build/main.go`

**Interfaces:**
- Consumes: `plan.Graph{Deps}`, `(*Graph).Tiers()` (plan 1).
- Produces:
  - `func (g *Graph) Closure(names []string) (map[string]bool, error)`: the names plus their transitive build dependencies. It errors on an unknown name.
  - `func Filter(tiers [][]string, keep map[string]bool) [][]string`: tiers restricted to `keep`, with empty tiers dropped.
  - CLI: `distro-build [flags] build|plan [name…]`.

- [ ] **Step 1: Write the failing tests** (append to `internal/plan/plan_test.go`)

```go
func TestClosureIncludesTransitiveDependenciesOnly(t *testing.T) {
	g := &Graph{Deps: map[string][]string{"a": nil, "b": {"a"}, "c": {"b"}, "d": {"a"}, "e": nil}}
	got, err := g.Closure([]string{"c"})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"a": true, "b": true, "c": true}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Closure(c) = %v, want %v", got, want)
	}
	if _, err := g.Closure([]string{"nope"}); err == nil || !strings.Contains(err.Error(), "nope") {
		t.Errorf("unknown name: err = %v", err)
	}
}

func TestFilterKeepsOrderAndDropsEmptyTiers(t *testing.T) {
	tiers := [][]string{{"a", "e"}, {"b", "d"}, {"c"}}
	got := Filter(tiers, map[string]bool{"a": true, "c": true})
	want := [][]string{{"a"}, {"c"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Filter = %v, want %v", got, want)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd /Volumes/nudanos/distro && go test ./internal/plan/`
Expected: FAIL with `g.Closure undefined`

- [ ] **Step 3: Implement** (append to `internal/plan/plan.go`)

```go
// Closure returns names plus everything they transitively build-depend on.
func (g *Graph) Closure(names []string) (map[string]bool, error) {
	out := map[string]bool{}
	stack := append([]string(nil), names...)
	for _, n := range names {
		if _, ok := g.Deps[n]; !ok {
			return nil, fmt.Errorf("%s is not a ready package in the manifest", n)
		}
	}
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if out[n] {
			continue
		}
		out[n] = true
		stack = append(stack, g.Deps[n]...)
	}
	return out, nil
}

// Filter restricts tiers to the names in keep, preserving order.
func Filter(tiers [][]string, keep map[string]bool) [][]string {
	var out [][]string
	for _, t := range tiers {
		var kept []string
		for _, n := range t {
			if keep[n] {
				kept = append(kept, n)
			}
		}
		if len(kept) > 0 {
			out = append(out, kept)
		}
	}
	return out
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/plan/ -v`
Expected: 6 tests `PASS`

- [ ] **Step 5: Wire it into the CLI.** In `cmd/distro-build/main.go`:
  1. Change the argument check in `main` from `if flag.NArg() != 1` to `if flag.NArg() < 1`.
  2. Pass the names on: call `a.run(ctx, flag.Arg(0), flag.Args()[1:])` and change `run`'s signature to `func (a *app) run(ctx context.Context, cmd string, names []string) error`.
  3. Right after `g, tiers, err := graph(dirs)` and its error check, add:
```go
	if len(names) > 0 {
		keep, err := g.Closure(names)
		if err != nil {
			return err
		}
		tiers = plan.Filter(tiers, keep)
	}
```
  4. Change the usage line to `usage: distro-build [flags] builder|fetch|plan|build|repo|check-updates [name…]`.

- [ ] **Step 6: Verify on the pilot**

Run: `go build -o distro-build ./cmd/distro-build && ./distro-build -work /Volumes/nudanos/work plan dh-yang && ./distro-build -work /Volumes/nudanos/work plan nope; echo "exit=$?"`
Expected: `tier 0 (1): dh-yang`, then `error: nope is not a ready package in the manifest` and `exit=1`

- [ ] **Step 7: Commit**

```bash
git add internal/plan cmd/distro-build && git commit -s -m "distro-build: build/plan named packages plus their build-dependency closure

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: Pool indexed once per tier, parallel builds within a tier

**Files:**
- Modify: `internal/build/build.go`, `internal/build/build_test.go`, `builder/build-package.sh`, `builder/Dockerfile`, `cmd/distro-build/main.go`
- Create: `builder/index-pool.sh`, `builder/apt-retries`

**Interfaces:**
- Consumes: `build.Builder` (plan 1).
- Produces:
  - New `Builder` fields: `Workers int` (≤1 means serial) and `BeforeTier func(ctx context.Context) error` (nil = none).
  - Builder image contract: `/pool` holds `Packages{,.gz,.xz}` written by `/usr/local/bin/index-pool` (run with `/pool` read-write), and `build-package` reads `/pool` in place.
  - Env `JOBS`: the per-build `parallel=` value.
  - CLI flag: `-jobs N` (default 1).

- [ ] **Step 1: Write the failing tests.** First make plan 1's fixture safe for concurrent builds; its fake `Build` appends to a shared slice. In `internal/build/build_test.go`, add `mu sync.Mutex` to `type fixture struct`, and in `newFixture` replace `f.built = append(f.built, name)` with:
```go
			f.mu.Lock()
			f.built = append(f.built, name)
			f.mu.Unlock()
```
Then append these tests (and add `"sync"` and `"time"` to the imports):

```go
func TestRunBuildsATierConcurrently(t *testing.T) {
	f := newFixture(t, "a", "b", "c")
	var mu sync.Mutex
	started, release := 0, make(chan struct{})
	inner := f.b.Build
	f.b.Workers = 3
	f.b.Build = func(ctx context.Context, name, src, out string) error {
		mu.Lock()
		started++
		if started == 3 {
			close(release)
		}
		mu.Unlock()
		select {
		case <-release:
		case <-time.After(5 * time.Second):
			return errors.New("builds in one tier did not run concurrently")
		}
		return inner(ctx, name, src, out)
	}
	rs, err := f.b.Run(context.Background(), [][]string{{"a", "b", "c"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, r := range rs {
		if r.Status != Built {
			t.Errorf("%s = %s %s", r.Name, r.Status, r.Detail)
		}
		names = append(names, r.Name)
	}
	if strings.Join(names, ",") != "a,b,c" {
		t.Errorf("result order = %v, want tier order a,b,c", names)
	}
}

func TestParallelFailureStillSkipsDependents(t *testing.T) {
	f := newFixture(t, "a", "b", "c", "d")
	f.b.Workers = 4
	f.fail["b"] = true
	rs, err := f.b.Run(context.Background(), [][]string{{"a", "b", "c"}, {"d"}}, map[string][]string{"d": {"b"}})
	if err != nil {
		t.Fatal(err)
	}
	s := statuses(rs)
	if s["b"].Status != Failed || s["d"].Status != Skipped || s["a"].Status != Built || s["c"].Status != Built {
		t.Errorf("statuses = %+v", s)
	}
	st, err := loadState(f.b.StateFile)
	if err != nil {
		t.Fatalf("state file unreadable after parallel run: %v", err)
	}
	if st["a"] == "" || st["c"] == "" || st["b"] != "" {
		t.Errorf("state = %v, want a and c recorded, b absent", st)
	}
}

func TestBeforeTierRunsOncePerTierBeforeItsBuilds(t *testing.T) {
	f := newFixture(t, "a", "b")
	var events []string
	f.b.BeforeTier = func(ctx context.Context) error { events = append(events, "index"); return nil }
	inner := f.b.Build
	f.b.Build = func(ctx context.Context, name, src, out string) error {
		events = append(events, name)
		return inner(ctx, name, src, out)
	}
	if _, err := f.b.Run(context.Background(), [][]string{{"a"}, {"b"}}, map[string][]string{"b": {"a"}}); err != nil {
		t.Fatal(err)
	}
	if strings.Join(events, ",") != "index,a,index,b" {
		t.Errorf("events = %v", events)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/build/ -run 'Concurrently|ParallelFailure|BeforeTier'`
Expected: FAIL with `f.b.Workers undefined`

- [ ] **Step 3: Rewrite `Run` in `internal/build/build.go`.** Add `"sync"` to the imports. Add these fields to `Builder`:
```go
	Workers    int                             // builds run at once within a tier; <=1 means serial
	BeforeTier func(ctx context.Context) error // e.g. index the pool; nil means none
```
Replace the whole `Run` function with:
```go
// Run builds every package in tiers order, up to Workers at once within a
// tier. A package whose dependency failed or was skipped is skipped. Results
// come back in tier order. The error return is reserved for state-file I/O,
// the BeforeTier hook and cancellation.
func (b *Builder) Run(ctx context.Context, tiers [][]string, deps map[string][]string) ([]Result, error) {
	state, err := loadState(b.StateFile)
	if err != nil {
		return nil, err
	}
	workers := b.Workers
	if workers < 1 {
		workers = 1
	}
	var mu sync.Mutex // guards state, bad, and state-file writes
	bad := map[string]bool{}
	var results []Result
	for _, tier := range tiers {
		if err := ctx.Err(); err != nil {
			return results, err
		}
		if b.BeforeTier != nil {
			if err := b.BeforeTier(ctx); err != nil {
				return results, err
			}
		}
		tierResults := make([]Result, len(tier))
		errs := make([]error, len(tier))
		sem := make(chan struct{}, workers)
		var wg sync.WaitGroup
		for i, name := range tier {
			blocker := ""
			for _, d := range deps[name] {
				if bad[d] {
					blocker = d
					break
				}
			}
			if blocker != "" {
				tierResults[i] = Result{name, Skipped, "dependency " + blocker + " did not build"}
				continue
			}
			wg.Add(1)
			sem <- struct{}{}
			go func(i int, name string) {
				defer wg.Done()
				defer func() { <-sem }()
				tierResults[i], errs[i] = b.one(ctx, name, deps[name], state, &mu)
			}(i, name)
		}
		wg.Wait()
		for i, r := range tierResults {
			if errs[i] != nil {
				return results, errs[i]
			}
			if r.Status == Failed || r.Status == Skipped {
				bad[r.Name] = true
			}
			results = append(results, r)
			fmt.Fprintf(b.Log, "==> %-40s %s %s\n", r.Name, r.Status, r.Detail)
		}
	}
	return results, nil
}

// one builds a single package. It reads dependency keys from state (those
// packages finished in earlier tiers) and records its own key under mu.
func (b *Builder) one(ctx context.Context, name string, deps []string, state map[string]string, mu *sync.Mutex) (Result, error) {
	src, out := b.SrcDirs[name], filepath.Join(b.OutRoot, name)
	hash, err := TreeHash(src)
	if err != nil {
		return Result{name, Failed, err.Error()}, nil
	}
	mu.Lock()
	// Dependencies' keys are part of ours, so a rebuilt dependency rebuilds
	// its dependents (transitively, since their keys change in turn).
	key := hash + ":" + b.KeySalt
	for _, d := range deps {
		key += ":" + d + "=" + state[d]
	}
	cached := state[name] == key && HasArtifacts(out)
	mu.Unlock()
	if cached {
		return Result{name, Cached, ""}, nil
	}
	if err := os.RemoveAll(out); err != nil {
		return Result{}, err
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return Result{}, err
	}
	err = b.Build(ctx, name, src, out)
	if err == nil && !HasArtifacts(out) {
		err = errors.New("build produced no .deb files")
	}
	mu.Lock()
	defer mu.Unlock()
	r := Result{name, Built, ""}
	if err != nil {
		delete(state, name)
		r = Result{name, Failed, err.Error()}
	} else {
		state[name] = key
	}
	return r, saveState(b.StateFile, state)
}
```
Note: `saveState` marshals `state` while holding `mu`, so concurrent writers cannot interleave.

- [ ] **Step 4: Run all build tests**

Run: `go test ./internal/build/ -race -v 2>&1 | grep -E '^(--- |ok|FAIL|WARNING: DATA RACE)'`
Expected: all `PASS`, and no `DATA RACE`

- [ ] **Step 5: Change the builder scripts to index the pool once and read it in place**

`builder/index-pool.sh`:
```bash
#!/bin/bash
# Index every .deb under /pool (mounted read-write) as a flat apt repository.
# Run once per build tier, so builds read the pool in place instead of copying it.
set -euo pipefail
cd /pool
apt-ftparchive packages . > Packages
gzip -9kf Packages
xz -kf Packages
```

`builder/apt-retries`:
```text
Acquire::Retries "3";
```

In `builder/build-package.sh`, replace the whole `setup_local_repo` function with:
```bash
setup_local_repo() {
    # /pool is indexed once per tier by index-pool; read it in place.
    echo 'deb [trusted=yes] file:/pool ./' > /etc/apt/sources.list.d/000-local.list
    printf 'Package: *\nPin: origin ""\nPin-Priority: 999\n' > /etc/apt/preferences.d/000-local
    apt-get update
}
```
Then replace `runuser -u builder -- dpkg-buildpackage -us -uc -I -i` with:
```bash
    runuser -u builder -- env DEB_BUILD_OPTIONS="parallel=${JOBS:-1}" dpkg-buildpackage -us -uc -I -i
```

Append to `builder/Dockerfile`:
```dockerfile
COPY apt-retries /etc/apt/apt.conf.d/80retries
COPY index-pool.sh /usr/local/bin/index-pool
RUN chmod 0755 /usr/local/bin/index-pool
```

- [ ] **Step 6: Wire `-jobs` and the index hook into `cmd/distro-build/main.go`.**
  1. Add `"runtime"` to the imports and `jobs int` to `app`.
  2. In `main`, add: `flag.IntVar(&a.jobs, "jobs", 1, "packages built at once within a tier")`.
  3. In `containerBuild`, add `"JOBS": strconv.Itoa(max(1, runtime.NumCPU()/max(1, a.jobs)))` to the `Env` map.
  4. In `run`, extend the `&build.Builder{…}` literal with `Workers: a.jobs, BeforeTier: a.indexPool`.
  5. Add this method:
```go
// indexPool writes the pool's apt index once, before a tier's builds start.
func (a *app) indexPool(ctx context.Context) error {
	if err := os.MkdirAll(a.outRoot(), 0o755); err != nil {
		return err
	}
	return a.eng.Run(ctx, engine.RunSpec{
		Image:   a.image,
		Mounts:  []engine.Mount{{Host: a.outRoot(), Container: "/pool"}},
		Network: "none",
		Cmd:     []string{"/usr/local/bin/index-pool"},
	}, os.Stderr, os.Stderr)
}
```

- [ ] **Step 7: Verify end to end on the pilot**

```bash
go build -o distro-build ./cmd/distro-build && ./distro-build -work /Volumes/nudanos/work builder >/dev/null
rm -f /Volumes/nudanos/work/state.json
./distro-build -work /Volumes/nudanos/work -jobs 3 build 2>&1 | tail -4
ls /Volumes/nudanos/work/out/Packages*
WORK=/Volumes/nudanos/work KEY=$(cat keys/FINGERPRINT) tests/integration/pilot.sh 2>&1 | tail -1
```
Expected: `dh-vci built`, `dh-yang built`, `vyatta-util built`; `Packages Packages.gz Packages.xz` exist; `pilot: OK`

- [ ] **Step 8: Commit**

```bash
git add internal/build builder cmd/distro-build && git commit -s -m "distro-build: index the pool once per tier and build a tier in parallel

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: Upstream-kind builds and `3.0 (quilt)` sources

**Files:**
- Modify: `internal/manifest/manifest.go`, `internal/manifest/manifest_test.go`, `builder/build-package.sh`, `cmd/distro-build/main.go`
- Create: `internal/upstream/upstream.go`, `internal/upstream/upstream_test.go`

**Interfaces:**
- Consumes: `manifest.Entry`, `fetch.Git`, `build.TreeHash`.
- Produces:
  - New `Entry` fields:
    - `Version string` (yaml `version`, the upstream version, e.g. `2.4.3`)
    - `Tag string` (`tag`)
    - `TagPattern string` (`tag_pattern`, a regexp with one capture group holding the version, `_` allowed as separator)
    - `PackagingRef string` (`packaging_ref`)
    - `Audit []AuditEntry` (`audit`)
  - `type AuditEntry struct { Patch, Verdict, Note string }`; verdict is `kept`, `upstreamed` or `dropped`.
  - `upstream.Prepare(ctx context.Context, e manifest.Entry, patchesDir, work string, log io.Writer) (string, error)`: returns `work/src/<name>`.
  - `upstream.DebianVersion(prevTop, upstreamVersion string) string`
  - `build-package.sh` builds `3.0 (quilt)` sources, generating `<src>_<upver>.orig.tar.xz` from the tree minus `debian/` when no orig tarball is present.

- [ ] **Step 1: Write the failing manifest tests.** In `internal/manifest/manifest_test.go`, delete the `"upstream ready"` case from `TestParseRejects`, then append:

```go
func TestUpstreamReadyNeedsPins(t *testing.T) {
	base := "packages:\n  - {name: k, kind: upstream, milestone: \"1.0\", ready: true, upstream: u, packaging: p, track: latest"
	cases := map[string]struct{ tail, want string }{
		"no version": {", tag: v1, tag_pattern: '^v(.*)$', packaging_ref: master}\n", "version"},
		"no tag":     {", version: \"1\", tag_pattern: '^v(.*)$', packaging_ref: master}\n", "tag"},
		"no ref":     {", version: \"1\", tag: v1, tag_pattern: '^v(.*)$'}\n", "packaging_ref"},
		"bad regexp": {", version: \"1\", tag: v1, tag_pattern: '^v(', packaging_ref: master}\n", "tag_pattern"},
		"no group":   {", version: \"1\", tag: v1, tag_pattern: '^v.*$', packaging_ref: master}\n", "capture group"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Parse([]byte(base + c.tail))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("err = %v, want mention of %q", err, c.want)
			}
		})
	}
	ok := base + ", version: \"2.4.3\", tag: v2.4.3, tag_pattern: '^v(\\d+\\.\\d+\\.\\d+)$', packaging_ref: master}\n"
	if _, err := Parse([]byte(ok)); err != nil {
		t.Errorf("complete upstream entry rejected: %v", err)
	}
}

func TestAuditVerdicts(t *testing.T) {
	y := "packages:\n  - name: k\n    kind: upstream\n    milestone: \"1.0\"\n    upstream: u\n    packaging: p\n    track: latest\n    audit:\n      - {patch: a.patch, verdict: maybe, note: x}\n"
	if _, err := Parse([]byte(y)); err == nil || !strings.Contains(err.Error(), "verdict") {
		t.Errorf("err = %v, want verdict error", err)
	}
}
```

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/manifest/`
Expected: FAIL. Unknown fields `version`/`audit` are rejected by strict decoding, so the error names the field rather than the rule.

- [ ] **Step 3: Implement the manifest changes.** In `internal/manifest/manifest.go`, add `"regexp"` to the imports. Add this type:
```go
// AuditEntry records the decision on one DANOS patch to an upstream package.
type AuditEntry struct {
	Patch   string `yaml:"patch"`
	Verdict string `yaml:"verdict"` // kept | upstreamed | dropped
	Note    string `yaml:"note,omitempty"`
}
```
Add these fields to `Entry`, after `Patches`:
```go
	Version      string       `yaml:"version,omitempty"`       // upstream: upstream version built
	Tag          string       `yaml:"tag,omitempty"`           // upstream: git tag of Version
	TagPattern   string       `yaml:"tag_pattern,omitempty"`   // upstream: regexp, group 1 = version
	PackagingRef string       `yaml:"packaging_ref,omitempty"` // upstream: branch or tag of packaging
	Audit        []AuditEntry `yaml:"audit,omitempty"`
```
Replace the `case Upstream:` block in `Validate` with:
```go
		case Upstream:
			if e.Upstream == "" || e.Packaging == "" {
				bad("upstream entries need upstream and packaging")
			}
			if e.Track != "latest" && e.Track != "lts" && e.Track != "debian" {
				bad("track must be latest, lts or debian")
			}
			if e.TagPattern != "" {
				if re, err := regexp.Compile(e.TagPattern); err != nil {
					bad("tag_pattern: " + err.Error())
				} else if re.NumSubexp() != 1 {
					bad("tag_pattern needs exactly one capture group (the version)")
				}
			}
			if e.Ready && (e.Version == "" || e.Tag == "" || e.PackagingRef == "" || e.TagPattern == "") {
				bad("ready upstream entries need version, tag, tag_pattern and packaging_ref")
			}
```
After the `switch`, still inside the loop, add:
```go
		for _, a := range e.Audit {
			if a.Verdict != "kept" && a.Verdict != "upstreamed" && a.Verdict != "dropped" {
				bad(fmt.Sprintf("audit %s: verdict must be kept, upstreamed or dropped", a.Patch))
			}
		}
```

- [ ] **Step 4: Run the manifest tests**

Run: `go test ./internal/manifest/ -v 2>&1 | grep -E '^(\s*--- |ok|FAIL)'`
Expected: all `PASS`, including `TestUpstreamReadyNeedsPins` with 5 subtests and `TestAuditVerdicts`. `TestRepoManifest` still passes, because no upstream entry is ready yet.

- [ ] **Step 5: Write the failing `upstream` tests**

`internal/upstream/upstream_test.go`:
```go
package upstream

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nudanos/distro/internal/build"
	"github.com/nudanos/distro/internal/manifest"
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@e", "GIT_COMMITTER_NAME=t",
		"GIT_COMMITTER_EMAIL=t@e", "GIT_COMMITTER_DATE=2026-03-30T07:55:19+00:00", "GIT_AUTHOR_DATE=2026-03-30T07:55:19+00:00")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func write(t *testing.T, path, body string) {
	t.Helper()
	os.MkdirAll(filepath.Dir(path), 0o755)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// fixture: an upstream repo tagged v2.4.3 (which ships its own stale debian/),
// a packaging repo whose changelog carries epoch 1, and a DANOS patch.
func fixture(t *testing.T) (manifest.Entry, string, string) {
	root := t.TempDir()
	up := filepath.Join(root, "up")
	os.MkdirAll(up, 0o755)
	git(t, up, "init", "-q", "-b", "master")
	write(t, filepath.Join(up, "configure.ac"), "AC_INIT([keepalived], [2.4.3])\n")
	write(t, filepath.Join(up, "debian", "control"), "Source: stale\n")
	git(t, up, "add", ".")
	git(t, up, "commit", "-qm", "release")
	git(t, up, "tag", "v2.4.3")

	pk := filepath.Join(root, "pkg")
	os.MkdirAll(pk, 0o755)
	git(t, pk, "init", "-q", "-b", "master")
	write(t, filepath.Join(pk, "debian", "changelog"), "keepalived (1:2.3.3-1) unstable; urgency=medium\n\n  * Upload.\n\n -- Debian <d@d>  Mon, 01 Jan 2024 00:00:00 +0000\n")
	write(t, filepath.Join(pk, "debian", "control"), "Source: keepalived\n")
	write(t, filepath.Join(pk, "debian", "source", "format"), "3.0 (quilt)\n")
	write(t, filepath.Join(pk, "debian", "patches", "series"), "debian-fix.patch\n")
	write(t, filepath.Join(pk, "debian", "patches", "debian-fix.patch"), "--- a\n+++ b\n")
	write(t, filepath.Join(pk, "README.packaging"), "not copied\n")
	git(t, pk, "add", ".")
	git(t, pk, "commit", "-qm", "packaging")

	patches := filepath.Join(root, "patches")
	write(t, filepath.Join(patches, "keepalived", "0001-vyatta-vrrp-hook.patch"), "--- a\n+++ b\n")

	e := manifest.Entry{Name: "keepalived", Kind: manifest.Upstream, Upstream: up, Tag: "v2.4.3", Version: "2.4.3",
		TagPattern: `^v(\d+\.\d+\.\d+)$`, Packaging: pk, PackagingRef: "master", Track: "latest"}
	return e, patches, filepath.Join(root, "work")
}

func TestPrepare(t *testing.T) {
	e, patches, work := fixture(t)
	dir, err := Prepare(context.Background(), e, patches, work, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "configure.ac")); !strings.Contains(string(b), "2.4.3") {
		t.Error("upstream source missing")
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "debian", "control")); string(b) != "Source: keepalived\n" {
		t.Errorf("debian/control = %q, want packaging's (upstream's debian/ replaced)", b)
	}
	if _, err := os.Stat(filepath.Join(dir, "README.packaging")); !os.IsNotExist(err) {
		t.Error("files outside packaging's debian/ were copied")
	}
	series, _ := os.ReadFile(filepath.Join(dir, "debian", "patches", "series"))
	if string(series) != "debian-fix.patch\n0001-vyatta-vrrp-hook.patch\n" {
		t.Errorf("series = %q", series)
	}
	cl, _ := os.ReadFile(filepath.Join(dir, "debian", "changelog"))
	first := strings.SplitN(string(cl), "\n", 2)[0]
	if first != "keepalived (1:2.4.3-0nudanos1) trixie; urgency=medium" {
		t.Errorf("changelog top = %q (epoch must be kept)", first)
	}
	if !strings.Contains(string(cl), " -- NuDanOS Maintainers <jon@fernandez.tech>  Mon, 30 Mar 2026 07:55:19 +0000") {
		t.Errorf("changelog trailer must carry the tag's commit date:\n%s", cl)
	}
	if !strings.Contains(string(cl), "keepalived (1:2.3.3-1) unstable") {
		t.Error("previous changelog entries lost")
	}
}

func TestPrepareIsReproducible(t *testing.T) {
	e, patches, work := fixture(t)
	d1, err := Prepare(context.Background(), e, patches, work, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	h1, _ := build.TreeHash(d1)
	d2, err := Prepare(context.Background(), e, patches, work, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if h2, _ := build.TreeHash(d2); h2 != h1 {
		t.Error("preparing the same tag twice produced different trees")
	}
}

func TestDebianVersion(t *testing.T) {
	cases := map[[2]string]string{
		{"1:2.3.3-1", "2.4.3"}:       "1:2.4.3-0nudanos1",
		{"5.9.4+dfsg-2+deb13u1", "5.9.5.2"}: "5.9.5.2-0nudanos1",
		{"1.2.9.2-5", "1.2.9.2"}:     "1.2.9.2-0nudanos1",
	}
	for in, want := range cases {
		if got := DebianVersion(in[0], in[1]); got != want {
			t.Errorf("DebianVersion(%q, %q) = %q, want %q", in[0], in[1], got, want)
		}
	}
}
```

- [ ] **Step 6: Run to verify they fail**

Run: `go test ./internal/upstream/`
Expected: FAIL with `undefined: Prepare`

- [ ] **Step 7: Implement `internal/upstream/upstream.go`**

```go
// Package upstream prepares source trees for packages built from an upstream
// release with Debian's (or our) packaging on top.
package upstream

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/nudanos/distro/internal/fetch"
	"github.com/nudanos/distro/internal/manifest"
)

const maintainer = "NuDanOS Maintainers <jon@fernandez.tech>"

// DebianVersion is the version for an upstream build: prevTop's epoch (if
// any), the upstream version, and revision 0nudanos1.
func DebianVersion(prevTop, upstreamVersion string) string {
	epoch := ""
	if i := strings.Index(prevTop, ":"); i > 0 {
		epoch = prevTop[:i+1]
	}
	return epoch + upstreamVersion + "-0nudanos1"
}

// Prepare checks out e.Upstream at e.Tag and e.Packaging at e.PackagingRef, and
// assembles work/src/<name>: the upstream tree (minus .git and any debian/),
// the packaging's debian/, our patches from patchesDir/<name> appended to the
// quilt series, and a changelog entry dated with the tag's commit date so the
// result is reproducible.
func Prepare(ctx context.Context, e manifest.Entry, patchesDir, work string, log io.Writer) (string, error) {
	upDir := filepath.Join(work, "upstream", e.Name)
	pkDir := filepath.Join(work, "packaging", e.Name)
	if err := fetch.Git(ctx, e.Upstream, e.Tag, upDir, log); err != nil {
		return "", err
	}
	if err := fetch.Git(ctx, e.Packaging, e.PackagingRef, pkDir, log); err != nil {
		return "", err
	}
	dir := filepath.Join(work, "src", e.Name)
	if err := os.RemoveAll(dir); err != nil {
		return "", err
	}
	if err := copyTree(upDir, dir, func(rel string) bool {
		return rel == ".git" || rel == "debian" || strings.HasPrefix(rel, ".git/") || strings.HasPrefix(rel, "debian/")
	}); err != nil {
		return "", err
	}
	if err := copyTree(filepath.Join(pkDir, "debian"), filepath.Join(dir, "debian"), func(string) bool { return false }); err != nil {
		return "", fmt.Errorf("%s: packaging has no debian/: %w", e.Name, err)
	}
	if err := addPatches(filepath.Join(patchesDir, e.Name), filepath.Join(dir, "debian", "patches")); err != nil {
		return "", err
	}
	date, err := gitOutput(ctx, upDir, "log", "-1", "--format=%cD", e.Tag)
	if err != nil {
		return "", err
	}
	if err := bumpChangelog(filepath.Join(dir, "debian", "changelog"), e, date); err != nil {
		return "", err
	}
	return dir, nil
}

func gitOutput(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %v in %s: %w", args, dir, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// copyTree copies src to dst, skipping paths (relative to src) where skip is true.
func copyTree(src, dst string, skip func(rel string) bool) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		rel = filepath.ToSlash(rel)
		if rel != "." && skip(rel) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(dst, rel)
		info, err := os.Lstat(p)
		if err != nil {
			return err
		}
		switch {
		case d.IsDir():
			return os.MkdirAll(target, 0o755)
		case info.Mode()&fs.ModeSymlink != 0:
			link, err := os.Readlink(p)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		default:
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			return os.WriteFile(target, b, info.Mode().Perm())
		}
	})
}

// addPatches copies from/*.patch (sorted) into to/ and appends them to to/series.
func addPatches(from, to string) error {
	names, err := filepath.Glob(filepath.Join(from, "*.patch"))
	if err != nil || len(names) == 0 {
		return err
	}
	if err := os.MkdirAll(to, 0o755); err != nil {
		return err
	}
	series, _ := os.ReadFile(filepath.Join(to, "series"))
	if len(series) > 0 && !bytes.HasSuffix(series, []byte("\n")) {
		series = append(series, '\n')
	}
	for _, n := range names {
		b, err := os.ReadFile(n)
		if err != nil {
			return err
		}
		base := filepath.Base(n)
		if err := os.WriteFile(filepath.Join(to, base), b, 0o644); err != nil {
			return err
		}
		series = append(series, []byte(base+"\n")...)
	}
	return os.WriteFile(filepath.Join(to, "series"), series, 0o644)
}

// bumpChangelog prepends an entry for e.Version, keeping the packaging's epoch.
func bumpChangelog(path string, e manifest.Entry, date string) error {
	old, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	first := strings.SplitN(string(old), "\n", 2)[0]
	fields := strings.Fields(first)
	if len(fields) < 2 {
		return fmt.Errorf("%s: unparseable first line %q", path, first)
	}
	src, prev := fields[0], strings.Trim(fields[1], "()")
	entry := fmt.Sprintf("%s (%s) trixie; urgency=medium\n\n  * New upstream release %s (tag %s), built by NuDanOS.\n\n -- %s  %s\n\n",
		src, DebianVersion(prev, e.Version), e.Version, e.Tag, maintainer, date)
	return os.WriteFile(path, append([]byte(entry), old...), 0o644)
}
```

- [ ] **Step 8: Run the upstream tests**

Run: `gofmt -l internal; go vet ./internal/upstream/ && go test ./internal/upstream/ -v 2>&1 | grep -E '^(--- |ok|FAIL)'`
Expected: 3 tests `PASS`

- [ ] **Step 9: Teach `build-package.sh` to build `3.0 (quilt)` sources.** Replace the block from `local format="1.0"` through its closing `fi` with:
```bash
    local format="1.0"
    [ -f debian/source/format ] && format="$(cat debian/source/format)"
    if [ "$format" = "3.0 (quilt)" ]; then
        local src upver
        src=$(dpkg-parsechangelog -S Source)
        upver=$(dpkg-parsechangelog -S Version | sed -E 's/^[0-9]+://; s/-[^-]*$//')
        if ! compgen -G "/build/${src}_${upver}.orig.tar.*" >/dev/null; then
            # No orig tarball: generate one from the tree minus debian/ (our
            # upstream kind and DANOS quilt forks keep the full source in git).
            tar -C /build --exclude=pkg/debian --exclude=pkg/.git --sort=name --mtime='@0' \
                --owner=0 --group=0 --numeric-owner -cJf "/build/${src}_${upver}.orig.tar.xz" pkg
        fi
    fi
```
(`--sort`, `--mtime`, `--owner` and `--group` make the tarball reproducible.)

- [ ] **Step 10: Prepare ready upstream entries in `fetch`.** In `cmd/distro-build/main.go`, add the import `"github.com/nudanos/distro/internal/upstream"`. In `fetch`, after the loop over `m.Ready(manifest.Danos)`, add:
```go
	for _, e := range m.Ready(manifest.Upstream) {
		fmt.Fprintf(os.Stderr, "==> prepare %s %s (%s)\n", e.Name, e.Version, e.Tag)
		d, err := upstream.Prepare(ctx, e, filepath.Join(filepath.Dir(a.manifest), "patches"), a.work, os.Stderr)
		if err != nil {
			return nil, err
		}
		dirs[e.Name] = d
	}
```
Also extend `readyNames` to include `manifest.Upstream` in the kinds it collects.

- [ ] **Step 11: Verify the full suite, and the quilt path on a real package, before any entry is marked ready.** Build keepalived 2.4.3 from Debian's packaging, locally only:

```bash
go vet ./... && go test ./... 2>&1 | grep -v '^ok' ; echo "suite done"
cat > /tmp/ka.yaml <<'YAML'
packages:
  - name: keepalived
    kind: upstream
    milestone: "1.0"
    ready: true
    upstream: https://github.com/acassen/keepalived
    version: "2.4.3"
    tag: v2.4.3
    tag_pattern: '^v(\d+\.\d+\.\d+)$'
    track: latest
    packaging: https://salsa.debian.org/debian/pkg-keepalived.git
    packaging_ref: master
YAML
./distro-build -manifest /tmp/ka.yaml -work /Volumes/nudanos/work-ka builder >/dev/null
./distro-build -manifest /tmp/ka.yaml -work /Volumes/nudanos/work-ka build 2>&1 | tail -3; ls /Volumes/nudanos/work-ka/out/keepalived/ | head
```
Expected: `suite done` with no failure lines. keepalived either builds (`keepalived_2.4.3-0nudanos1_amd64.deb` and a `keepalived_2.4.3.orig.tar.xz` next to the source package), or fails with a named quilt or build error. A named failure still proves the pipeline; making keepalived itself green is Task 12's job. Clean up with `rm -rf /Volumes/nudanos/work-ka /tmp/ka.yaml`.

- [ ] **Step 12: Commit**

```bash
git add internal/manifest internal/upstream builder/build-package.sh cmd/distro-build && git commit -s -m "distro-build: upstream-kind builds and 3.0 (quilt) sources

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 4: `check-updates`

**Files:**
- Create: `internal/versions/versions.go`, `internal/versions/versions_test.go`, `internal/updates/updates.go`, `internal/updates/updates_test.go`, `.github/workflows/updates.yml`
- Modify: `cmd/distro-build/main.go`

**Interfaces:**
- Consumes: `manifest.Entry` (upstream: `TagPattern`, `Version`; apt: `Source`, `Packages`), `control.Parse`.
- Produces:
  - `versions.Compare(a, b string) int`: numeric dot/underscore comparison; any non-numeric component sorts before numbers.
  - `versions.Latest(tags []string, pattern *regexp.Regexp) (tag, version string, ok bool)`
  - `type updates.Drift struct { Name, Current, Latest, Detail string }`
  - `updates.CheckUpstream(e manifest.Entry, tags []string) (*Drift, error)`
  - `updates.CheckApt(ctx context.Context, e manifest.Entry, client *http.Client) ([]Drift, error)`
  - CLI `check-updates`: prints drift, exits 1 on drift and 0 when current.

- [ ] **Step 1: Write the failing version tests**

`internal/versions/versions_test.go`:
```go
package versions

import (
	"regexp"
	"testing"
)

func TestCompare(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want int
	}{
		{"2.4.3", "2.4.10", -1}, {"0.2.0", "0.1.1", 1}, {"1.2.5", "1_2_5", 0},
		{"5.9.5.2", "5.9.5", 1}, {"1.32", "1.31", 1}, {"2.0", "2.0.0", -1},
	} {
		if got := Compare(c.a, c.b); got != c.want {
			t.Errorf("Compare(%q,%q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestLatestHonoursThePattern(t *testing.T) {
	// mstpd: 0.03-0.05 are ancient two-part tags that must not beat 0.2.0.
	mstpd := []string{"0.0.6", "0.0.9", "0.03", "0.04", "0.05", "0.1.0", "0.1.1", "0.2.0"}
	tag, v, ok := Latest(mstpd, regexp.MustCompile(`^(\d+\.\d+\.\d+)$`))
	if !ok || tag != "0.2.0" || v != "0.2.0" {
		t.Errorf("mstpd latest = %q %q %v", tag, v, ok)
	}
	ntp := []string{"NTPsec_1_2_4", "NTPsec_1_2_5", "NTPsec_1_2_5_rc1"}
	tag, v, ok = Latest(ntp, regexp.MustCompile(`^NTPsec_(\d+_\d+_\d+)$`))
	if !ok || tag != "NTPsec_1_2_5" || v != "1.2.5" {
		t.Errorf("ntpsec latest = %q %q %v", tag, v, ok)
	}
	if _, _, ok := Latest([]string{"foo"}, regexp.MustCompile(`^v(\d+)$`)); ok {
		t.Error("no matching tag should report ok=false")
	}
}
```

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/versions/`
Expected: FAIL with `undefined: Compare`

- [ ] **Step 3: Implement `internal/versions/versions.go`**

```go
// Package versions compares upstream release versions and picks tags.
package versions

import (
	"regexp"
	"strconv"
	"strings"
)

func parts(v string) []string {
	return strings.FieldsFunc(v, func(r rune) bool { return r == '.' || r == '_' })
}

// Compare orders versions component by component, numerically. A version
// with extra components is newer ("2.0.0" > "2.0"). A non-numeric component
// sorts before any number.
func Compare(a, b string) int {
	pa, pb := parts(a), parts(b)
	for i := 0; i < len(pa) || i < len(pb); i++ {
		if i >= len(pa) {
			return -1
		}
		if i >= len(pb) {
			return 1
		}
		na, ea := strconv.Atoi(pa[i])
		nb, eb := strconv.Atoi(pb[i])
		switch {
		case ea != nil && eb != nil:
			if c := strings.Compare(pa[i], pb[i]); c != 0 {
				return c
			}
		case ea != nil:
			return -1
		case eb != nil:
			return 1
		case na != nb:
			if na < nb {
				return -1
			}
			return 1
		}
	}
	return 0
}

// Latest returns the newest tag matching pattern (group 1 is the version,
// with '_' separators normalised to '.').
func Latest(tags []string, pattern *regexp.Regexp) (tag, version string, ok bool) {
	for _, t := range tags {
		m := pattern.FindStringSubmatch(t)
		if m == nil {
			continue
		}
		v := strings.ReplaceAll(m[1], "_", ".")
		if !ok || Compare(v, version) > 0 {
			tag, version, ok = t, v, true
		}
	}
	return tag, version, ok
}
```

- [ ] **Step 4: Run the version tests**

Run: `go test ./internal/versions/ -v 2>&1 | grep -E '^(--- |ok|FAIL)'`
Expected: 2 tests `PASS`

- [ ] **Step 5: Write the failing `updates` tests**

`internal/updates/updates_test.go`:
```go
package updates

import (
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nudanos/distro/internal/manifest"
)

func TestCheckUpstream(t *testing.T) {
	e := manifest.Entry{Name: "mstpd", Version: "0.1.1", TagPattern: `^(\d+\.\d+\.\d+)$`}
	d, err := CheckUpstream(e, []string{"0.05", "0.1.1", "0.2.0"})
	if err != nil {
		t.Fatal(err)
	}
	if d == nil || d.Latest != "0.2.0" || d.Current != "0.1.1" {
		t.Errorf("drift = %+v", d)
	}
	e.Version = "0.2.0"
	if d, _ := CheckUpstream(e, []string{"0.05", "0.2.0"}); d != nil {
		t.Errorf("up to date but drift = %+v", d)
	}
	if _, err := CheckUpstream(manifest.Entry{Name: "x", Version: "1", TagPattern: `^v(\d+)$`}, []string{"nope"}); err == nil {
		t.Error("no matching tags should be an error")
	}
}

func TestCheckApt(t *testing.T) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	zw.Write([]byte("Package: frr\nVersion: 10.7.2-0~deb13u1\n\nPackage: libyang3\nVersion: 3.13.6-1~deb13u1\n"))
	zw.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/frr/dists/trixie/frr-stable/binary-amd64/Packages.gz" {
			http.NotFound(w, r)
			return
		}
		w.Write(buf.Bytes())
	}))
	defer srv.Close()
	e := manifest.Entry{Name: "frr", Source: srv.URL + "/frr trixie frr-stable",
		Packages: map[string]string{"frr": "10.7.1-0~deb13u1", "libyang3": "3.13.6-1~deb13u1"}}
	ds, err := CheckApt(context.Background(), e, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	if len(ds) != 1 || ds[0].Name != "frr/frr" || ds[0].Latest != "10.7.2-0~deb13u1" {
		t.Errorf("drift = %+v", ds)
	}
}
```

- [ ] **Step 6: Run to verify they fail**

Run: `go test ./internal/updates/`
Expected: FAIL with `undefined: CheckUpstream`

- [ ] **Step 7: Implement `internal/updates/updates.go`**

```go
// Package updates reports manifest entries whose upstream has moved on.
package updates

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"

	"github.com/nudanos/distro/internal/control"
	"github.com/nudanos/distro/internal/manifest"
	"github.com/nudanos/distro/internal/versions"
)

// Drift is one pinned version that is no longer the newest.
type Drift struct {
	Name, Current, Latest, Detail string
}

// CheckUpstream compares e.Version with the newest tag matching e.TagPattern.
func CheckUpstream(e manifest.Entry, tags []string) (*Drift, error) {
	re, err := regexp.Compile(e.TagPattern)
	if err != nil {
		return nil, fmt.Errorf("%s: tag_pattern: %w", e.Name, err)
	}
	tag, v, ok := versions.Latest(tags, re)
	if !ok {
		return nil, fmt.Errorf("%s: no tag matches %s", e.Name, e.TagPattern)
	}
	if versions.Compare(v, e.Version) <= 0 {
		return nil, nil
	}
	return &Drift{Name: e.Name, Current: e.Version, Latest: v, Detail: "tag " + tag}, nil
}

// CheckApt compares each pinned package with the version the repository
// currently serves (third-party repos carry only their current release).
func CheckApt(ctx context.Context, e manifest.Entry, client *http.Client) ([]Drift, error) {
	f := strings.Fields(e.Source)
	if len(f) != 3 {
		return nil, fmt.Errorf("%s: source must be URL SUITE COMPONENT", e.Name)
	}
	url := fmt.Sprintf("%s/dists/%s/%s/binary-amd64/Packages.gz", strings.TrimSuffix(f[0], "/"), f[1], f[2])
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: GET %s: %s", e.Name, url, resp.Status)
	}
	zr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return nil, err
	}
	body, err := io.ReadAll(zr)
	if err != nil {
		return nil, err
	}
	served := map[string]string{}
	for _, p := range control.Parse(string(body)) {
		served[p["Package"]] = p["Version"]
	}
	var out []Drift
	names := make([]string, 0, len(e.Packages))
	for n := range e.Packages {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		if v, ok := served[n]; ok && v != e.Packages[n] {
			out = append(out, Drift{Name: e.Name + "/" + n, Current: e.Packages[n], Latest: v, Detail: "apt " + f[0]})
		}
	}
	return out, nil
}
```

- [ ] **Step 8: Run the tests**

Run: `go test ./internal/versions/ ./internal/updates/ -v 2>&1 | grep -E '^(--- |ok|FAIL)'`
Expected: 4 tests `PASS`

- [ ] **Step 9: Add the `check-updates` command.** In `cmd/distro-build/main.go`, add the imports `"net/http"`, `"os/exec"`, `"time"` and `"github.com/nudanos/distro/internal/updates"`. In `run`'s `switch`, add `case "check-updates": return a.checkUpdates(ctx)`. Then add:
```go
func remoteTags(ctx context.Context, repo string) ([]string, error) {
	out, err := exec.CommandContext(ctx, "git", "ls-remote", "--tags", "--refs", repo).Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-remote %s: %w", repo, err)
	}
	var tags []string
	for _, line := range strings.Split(string(out), "\n") {
		if _, ref, ok := strings.Cut(line, "\trefs/tags/"); ok {
			tags = append(tags, ref)
		}
	}
	return tags, nil
}

// checkUpdates reports pinned versions that upstream has superseded.
func (a *app) checkUpdates(ctx context.Context) error {
	m, err := manifest.Load(a.manifest)
	if err != nil {
		return err
	}
	var drift []updates.Drift
	for _, e := range m.Packages {
		if e.Kind != manifest.Upstream || e.TagPattern == "" || e.Version == "" {
			continue
		}
		tags, err := remoteTags(ctx, e.Upstream)
		if err != nil {
			return err
		}
		d, err := updates.CheckUpstream(e, tags)
		if err != nil {
			return err
		}
		if d != nil {
			drift = append(drift, *d)
		}
	}
	client := &http.Client{Timeout: 60 * time.Second}
	for _, e := range m.Packages {
		if e.Kind != manifest.Apt {
			continue
		}
		ds, err := updates.CheckApt(ctx, e, client)
		if err != nil {
			return err
		}
		drift = append(drift, ds...)
	}
	if len(drift) == 0 {
		fmt.Println("all pinned versions are current")
		return nil
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "PACKAGE\tPINNED\tLATEST\tWHERE")
	for _, d := range drift {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", d.Name, d.Current, d.Latest, d.Detail)
	}
	tw.Flush()
	return fmt.Errorf("%d pinned versions have newer releases", len(drift))
}
```
`check-updates` does not call `fetch`, so it needs neither Docker nor a case-sensitive work dir.

- [ ] **Step 10: Write `.github/workflows/updates.yml`**

```yaml
name: updates
on:
  schedule:
    - cron: "41 7 * * 1"
  workflow_dispatch:
permissions:
  contents: read
  issues: write
jobs:
  check:
    runs-on: ubuntu-24.04
    steps:
      - uses: actions/checkout@v7
      - uses: actions/setup-go@v7
        with:
          go-version-file: go.mod
      - name: Check pinned versions against upstream
        id: check
        run: |
          go build -o distro-build ./cmd/distro-build
          set +e
          ./distro-build check-updates > report.txt 2>&1
          echo "rc=$?" >> "$GITHUB_OUTPUT"
          cat report.txt
      - name: Open or update the tracking issue
        if: steps.check.outputs.rc != '0'
        env:
          GH_TOKEN: ${{ github.token }}
        run: |
          title="Upstream updates available"
          body=$(printf 'Weekly `distro-build check-updates` (run %s):\n\n```\n%s\n```\n' "$GITHUB_RUN_ID" "$(cat report.txt)")
          num=$(gh issue list --state open --search "in:title \"$title\"" --json number --jq '.[0].number')
          if [ -n "$num" ]; then gh issue edit "$num" --body "$body"; else gh issue create --title "$title" --body "$body"; fi
```

- [ ] **Step 11: Run it for real**

Run: `go build -o distro-build ./cmd/distro-build && ./distro-build check-updates; echo "exit=$?"`
Expected: `all pinned versions are current` with `exit=0`, or a table naming FRR if a release newer than 10.7.1 has shipped (then `exit=1`). No upstream entries are pinned yet; Task 7 adds them.

- [ ] **Step 12: Commit**

```bash
git add internal/versions internal/updates cmd/distro-build .github/workflows/updates.yml
git commit -s -m "distro-build: check-updates and weekly tracking issue

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 5: `tools/port.py`, the mechanical Debian 13 port

**Files:**
- Create: `tools/port.py`, `tools/test_port.py`, `tools/set_ready.py`, `tools/test_set_ready.py`, `docs/porting.md`

**Interfaces:**
- Produces:
  - CLI `python3 tools/port.py <checkout-dir> [--repo NAME]`. It edits in place and prints `NOTE:` lines for things needing a human.
  - Pure functions `port_control(text, repo) -> (text, notes)`, `port_rules(text) -> text`, `bump_version(v) -> str` and `new_changelog(text, version, date) -> text`.
  - Guarantee: running it on an already-ported checkout changes nothing.
  - CLI `python3 tools/set_ready.py manifest.yaml NAME...`: sets `ready: true` on the named entries (after `milestone:`). It errors on an unknown name and is idempotent. Pure function `set_ready(text, names) -> text`.

- [ ] **Step 1: Write the failing tests**

`tools/test_port.py`:
```python
from __future__ import annotations

import importlib.util
import os
import shutil
import tempfile
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
spec = importlib.util.spec_from_file_location("port", os.path.join(HERE, "port.py"))
port = importlib.util.module_from_spec(spec)
spec.loader.exec_module(port)

OLD_CONTROL = """Source: vyatta-cfg-system
Section: contrib/net
Priority: extra
Maintainer: Vyatta Package Maintainers <DL-vyatta-help@att.com>
Uploaders: Someone <x@brocade.com>
Build-Depends: debhelper (>= 9.20160709),
 dh-systemd,
 dh-yang,
 pylint3,
 python3-pytest-pep8,
 bvnos-linux-libc-dev (>> 4.19),
 python-setuptools,
 autotools-dev
Standards-Version: 3.9.6
Vcs-Git: git://git.vyatta.com/vyatta-cfg-system

Package: vyatta-cfg-system
Architecture: all
Priority: extra
Section: contrib/admin
Depends: ${misc:Depends}
Description: system config
"""


class ControlTest(unittest.TestCase):
    def test_source_paragraph(self):
        text, notes = port.port_control(OLD_CONTROL, "vyatta-cfg-system")
        src = text.split("\n\n")[0]
        self.assertIn("Maintainer: NuDanOS Maintainers <jon@fernandez.tech>", src)
        self.assertNotIn("Uploaders", src)
        self.assertIn("Standards-Version: 4.7.2", src)
        self.assertIn("Rules-Requires-Root: no", src)
        self.assertIn("Vcs-Git: https://github.com/nudanos/vyatta-cfg-system.git", src)
        self.assertIn("Vcs-Browser: https://github.com/nudanos/vyatta-cfg-system", src)
        self.assertIn("Section: net", src)
        self.assertIn("Priority: optional", src)
        bd = src.split("Build-Depends:")[1].split("\nStandards")[0]
        deps = [d.strip() for d in bd.replace("\n", " ").split(",")]
        self.assertEqual(deps, ["debhelper-compat (= 13)", "dh-yang", "pylint", "linux-libc-dev",
                                "python3-setuptools"])
        self.assertTrue(any("bvnos" in n for n in notes))

    def test_binary_paragraphs(self):
        text, _ = port.port_control(OLD_CONTROL, "vyatta-cfg-system")
        binp = text.split("\n\n")[1]
        self.assertIn("Section: admin", binp)
        self.assertNotIn("Priority", binp)
        self.assertIn("Description: system config", binp)

    def test_idempotent(self):
        once, _ = port.port_control(OLD_CONTROL, "vyatta-cfg-system")
        twice, notes = port.port_control(once, "vyatta-cfg-system")
        self.assertEqual(once, twice)
        self.assertEqual(notes, [])


class RulesTest(unittest.TestCase):
    def test_drops_obsolete_addons(self):
        rules = "%:\n\tdh $@ --with systemd,python3,yang --parallel\n\noverride_x:\n\tdh $@ --with=autotools_dev\n"
        out = port.port_rules(rules)
        self.assertIn("\tdh $@ --with python3,yang\n", out)
        self.assertIn("\tdh $@\n", out)
        self.assertEqual(port.port_rules(out), out)


class VersionTest(unittest.TestCase):
    def test_bump(self):
        self.assertEqual(port.bump_version("1.29"), "1.30")
        self.assertEqual(port.bump_version("2.12"), "2.13")
        self.assertEqual(port.bump_version("1:6"), "1:7")
        self.assertEqual(port.bump_version("0.1.27"), "0.1.28")
        self.assertEqual(port.bump_version("1.0.1-1"), "1.0.1-1+nudanos1")
        self.assertEqual(port.bump_version("4.0.0~git20170308-0vyatta3"), "4.0.0~git20170308-0vyatta3+nudanos1")


class TreeTest(unittest.TestCase):
    def setUp(self):
        self.d = tempfile.mkdtemp()
        os.makedirs(os.path.join(self.d, "debian", "source"))
        with open(os.path.join(self.d, "debian", "control"), "w") as f:
            f.write(OLD_CONTROL)
        with open(os.path.join(self.d, "debian", "compat"), "w") as f:
            f.write("9\n")
        with open(os.path.join(self.d, "debian", "rules"), "w") as f:
            f.write("%:\n\tdh $@ --with systemd\n")
        with open(os.path.join(self.d, "debian", "changelog"), "w") as f:
            f.write("vyatta-cfg-system (2.35) unstable; urgency=medium\n\n  * Old.\n\n -- V <v@v>  Mon, 01 Jan 2021 00:00:00 +0000\n")

    def tearDown(self):
        shutil.rmtree(self.d)

    def test_port_tree_then_rerun_changes_nothing(self):
        port.port_tree(self.d, "vyatta-cfg-system", date="Tue, 29 Sep 2026 12:00:00 +0000")
        self.assertFalse(os.path.exists(os.path.join(self.d, "debian", "compat")))
        self.assertEqual(open(os.path.join(self.d, "debian", "rules")).read(), "%:\n\tdh $@\n")
        cl = open(os.path.join(self.d, "debian", "changelog")).read()
        self.assertTrue(cl.startswith("vyatta-cfg-system (2.36) trixie; urgency=medium\n"))
        self.assertIn(" -- NuDanOS Maintainers <jon@fernandez.tech>  Tue, 29 Sep 2026 12:00:00 +0000", cl)
        snapshot = {p: open(os.path.join(self.d, "debian", p)).read() for p in ("control", "rules", "changelog")}
        port.port_tree(self.d, "vyatta-cfg-system", date="Wed, 30 Sep 2026 12:00:00 +0000")
        again = {p: open(os.path.join(self.d, "debian", p)).read() for p in ("control", "rules", "changelog")}
        self.assertEqual(snapshot, again)


if __name__ == "__main__":
    unittest.main()
```

- [ ] **Step 2: Run to verify they fail**

Run: `python3 -m unittest discover -s tools -p 'test_port.py'`
Expected: FAIL with `FileNotFoundError` for `port.py`

- [ ] **Step 3: Implement `tools/port.py`**

```python
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
        if k in ("Uploaders", "DM-Upload-Allowed") or k.startswith(("Vcs-", "XS-Vcs-")):
            continue
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
    names = [k for k, _ in src]
    if "Build-Depends" not in names:
        src.append(("Build-Depends", "debhelper-compat (= 13)"))
    if "Rules-Requires-Root" not in names:
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
    def fix(m: re.Match) -> str:
        addons = [a for a in m.group(2).split(",") if a and a not in ("systemd", "autotools_dev", "autotools-dev")]
        return f" --with {','.join(addons)}" if addons else ""
    text = re.sub(r" --with(=| )([A-Za-z0-9_,-]+)", fix, text)
    return re.sub(r" --parallel\b", "", text)


def bump_version(v: str) -> str:
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
    if PORT_LINE not in old.split("\n -- ", 1)[0]:
        top = re.match(r"^\S+ \(([^)]+)\)", old).group(1)
        open(cl, "w").write(new_changelog(old, bump_version(top), date or email.utils.formatdate(localtime=True)))
    fmt = os.path.join(deb, "source", "format")
    if os.path.exists(fmt) and "quilt" in open(fmt).read():
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
```

- [ ] **Step 4: Run the tests**

Run: `python3 -m unittest discover -s tools -p 'test_port.py' -v 2>&1 | tail -3`
Expected: `Ran 6 tests`, `OK`

- [ ] **Step 5: Cross-check against the three pilot ports done by hand in plan 1.** Port pristine copies of the originals and compare the control files:

```bash
for r in dh-yang dh-vci vyatta-util; do
  rm -rf /tmp/pc-$r && git clone -q "$HOME/Documents/Claude/Projects/danOS Project/mirrors/$r.git" /tmp/pc-$r
  python3 tools/port.py /tmp/pc-$r --repo $r
  diff <(sed '/^$/q' /Volumes/nudanos/port-$r/debian/control) <(sed '/^$/q' /tmp/pc-$r/debian/control) && echo "$r source paragraph matches"
done
```
Expected: `matches` for `dh-yang` and `dh-vci`. `vyatta-util` differs: port.py keeps `autoconf, automake, libtool` as explicit build dependencies, which the plan 1 hand port dropped. port.py is right here (it resolves the deferred review minor about implicit autotools deps), so add them back to the published `vyatta-util` `trixie` branch in wave A. Also check that `debian/rules` in the ported copy still has its full content: `diff /Volumes/nudanos/port-vyatta-util/debian/rules /tmp/pc-vyatta-util/debian/rules` should show only the two obsolete `mkdir -p m4` lines (a comment and the command) that the hand port removed. They're harmless either way.

- [ ] **Step 6: Write the manifest helper, test first**

`tools/test_set_ready.py`:
```python
from __future__ import annotations

import importlib.util
import os
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
spec = importlib.util.spec_from_file_location("set_ready", os.path.join(HERE, "set_ready.py"))
sr = importlib.util.module_from_spec(spec)
spec.loader.exec_module(sr)

TEXT = """# header
packages:
  - name: "a"
    kind: "danos"
    milestone: "1.0"
    repo: "https://github.com/nudanos/a"
    ref: "trixie"
  - name: "b"
    kind: "danos"
    milestone: "1.0"
    ready: true
    repo: "https://github.com/nudanos/b"
    ref: "trixie"
"""


class SetReadyTest(unittest.TestCase):
    def test_sets_after_milestone_and_is_idempotent(self):
        out = sr.set_ready(TEXT, ["a", "b"])
        self.assertIn('  - name: "a"\n    kind: "danos"\n    milestone: "1.0"\n    ready: true\n    repo:', out)
        self.assertEqual(out.count("ready: true"), 2)
        self.assertEqual(sr.set_ready(out, ["a", "b"]), out)

    def test_unknown_name(self):
        with self.assertRaises(SystemExit):
            sr.set_ready(TEXT, ["zzz"])


if __name__ == "__main__":
    unittest.main()
```

Run: `python3 -m unittest discover -s tools -p 'test_set_ready.py'`
Expected: FAIL with `FileNotFoundError` for `set_ready.py`

`tools/set_ready.py`:
```python
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
```

Run: `python3 -m unittest discover -s tools -p 'test_set_ready.py' -v 2>&1 | tail -2`
Expected: `Ran 2 tests`, `OK`

- [ ] **Step 7: Write `docs/porting.md`** (the procedure every wave follows)

````markdown
# Porting a DANOS repository to Debian 13

1. **Branch.** Clone from the org and branch from the original default branch:
   ```bash
   git clone git@github.com:nudanos/<repo>.git /Volumes/nudanos/port-<repo>
   git -C /Volumes/nudanos/port-<repo> checkout -b trixie
   ```
2. **Checklist.** `python3 tools/port.py /Volumes/nudanos/port-<repo>`. Read every `NOTE:`.
3. **Build locally**, from `distro/`, against every ready package:
   ```bash
   ./distro-build -work /Volumes/nudanos/work -jobs 4 -local <repo>=../port-<repo> build <repo>
   ```
   The entry must be `ready: true` in `manifest.yaml` for `-local` to apply.
4. **Fix failures at the cause.** Common ones on Debian 13:
   - GCC 14 errors: `-Wincompatible-pointer-types`, `-Wimplicit-function-declaration`, `-Wint-conversion`
   - `dh_missing --fail-missing`: list intentionally unshipped files in `debian/not-installed`
   - Python 3.13 removals (`imp`, `distutils`, `pipes`)
   - Perl 5.40 deprecations
   - Go vet failures in tests under Go 1.26
   - lintian errors

   Commit each fix separately, with the reason in the message.
5. **Push** `trixie` and add `.github/workflows/package.yml` (the caller: `uses: nudanos/distro/.github/workflows/package.yml@main`, `with: package: <repo>`).
6. **Record.** Make sure `ready: true` is committed in `manifest.yaml`.
````

- [ ] **Step 8: Commit**

```bash
git add tools/port.py tools/test_port.py tools/set_ready.py tools/test_set_ready.py docs/porting.md && git commit -s -m "tools: port.py applies the Debian 13 checklist; set_ready; porting procedure

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 6: Port `lintian-profile-vyatta`; builds use `--profile vyatta`

**Files:**
- Modify in `nudanos/lintian-profile-vyatta` (`trixie` branch): `debian/*` via port.py, `profiles/vyatta/main.profile`
- Modify: `builder/build-package.sh`, `manifest.yaml`

**Interfaces:**
- Produces: the binary package `lintian-profile-vyatta` in the pool. When it is present, `build-package.sh` installs it and runs `lintian --profile vyatta --fail-on error`; otherwise it falls back to `--suppress-tags dir-or-file-in-opt`.

- [ ] **Step 1: Port the repo**

```bash
cd /Volumes/nudanos && git clone -q git@github.com:nudanos/lintian-profile-vyatta.git port-lintian-profile-vyatta
cd port-lintian-profile-vyatta && git checkout -b trixie && python3 /Volumes/nudanos/distro/tools/port.py . && git add -A
```

- [ ] **Step 2: Check the profile against lintian 2.122.** Every tag in the profile must still exist:
```bash
docker run --rm -v "$PWD":/p nudanos/builder:trixie bash -c 'for t in dir-or-file-in-opt newer-standards-version bad-distribution-in-changes-file extended-description-is-empty section-area-mismatch backports-changes-missing; do lintian-explain-tags "$t" >/dev/null 2>&1 && echo "ok $t" || echo "GONE $t"; done'
```
Expected: `ok` for each. For any `GONE` line, delete that tag's stanza from `profiles/vyatta/main.profile` and record a ruling. `Disable-Tags-From-Check: nmu` must also refer to an existing check. Run `docker run --rm nudanos/builder:trixie find /usr/share/lintian -ipath '*check*nmu*'`; if it prints nothing, delete that line and record a ruling.

- [ ] **Step 3: Commit the port, mark the entry ready, and build it**

```bash
git commit -q -s -m "Port to Debian 13 (trixie)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
cd /Volumes/nudanos/distro
```
In `manifest.yaml`, change the `lintian-profile-vyatta` entry to `milestone: "1.0"` and `ready: true`. Then:
```bash
./distro-build -work /Volumes/nudanos/work -local lintian-profile-vyatta=../port-lintian-profile-vyatta build lintian-profile-vyatta 2>&1 | tail -2
```
Expected: `lintian-profile-vyatta built`

- [ ] **Step 4: Use the profile in `build-package.sh`.** Replace the lintian block (the two comment lines and the `lintian --fail-on error --suppress-tags dir-or-file-in-opt ../*.changes` line) with:
```bash
    if apt-cache show lintian-profile-vyatta >/dev/null 2>&1; then
        apt-get install -y --no-install-recommends lintian-profile-vyatta >/dev/null
        lintian --profile vyatta --fail-on error ../*.changes
    else
        # Bootstrap only: before lintian-profile-vyatta is in the pool, mirror the
        # one tag it disables (DANOS installs under /opt/vyatta by design).
        lintian --fail-on error --suppress-tags dir-or-file-in-opt ../*.changes
    fi
```

- [ ] **Step 5: Rebuild the image and prove `vyatta-util` passes through the profile**

```bash
./distro-build -work /Volumes/nudanos/work builder >/dev/null
./distro-build -work /Volumes/nudanos/work build vyatta-util 2>&1 | grep -E 'profile vyatta|vyatta-util' | tail -3
```
Expected: `vyatta-util built`. Its build ran `lintian --profile vyatta` (visible in the log), and `dir-or-file-in-opt` is not reported.

- [ ] **Step 6: Push the branch and commit**

```bash
git -C /Volumes/nudanos/port-lintian-profile-vyatta push -u origin trixie
git add builder/build-package.sh manifest.yaml && git commit -s -m "builder: lint with lintian-profile-vyatta once it is in the pool

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 7: Manifest decisions for the Go libraries and the upstream pins

**Files:**
- Modify: `manifest.yaml`, `internal/manifest/repo_test.go`, `tools/gen-manifest.py` (header note only)

**Interfaces:**
- Consumes: the Task 3 schema (upstream pins, audit).
- Produces: the manifest state that the waves build on.

- [ ] **Step 1: Switch the Go libraries Debian 13 ships at semver-compatible versions to `kind: debian`.** For each of `golang-github-mdlayher-netlink`, `golang-github-mdlayher-genetlink`, `golang-github-josharian-native`, `golang-github-youmark-pkcs8` and `golang-golang-x-sys`, replace its entry with:
```yaml
  - name: "<name>"
    kind: "debian"
    milestone: "1.0"
    note: "Debian 13 ships a newer semver-compatible version (<debian binary>=<version>)"
```
Use these binaries and versions:
- `golang-github-mdlayher-netlink-dev=1.7.2-1`
- `golang-github-mdlayher-genetlink-dev=1.3.1-1`
- `golang-github-josharian-native-dev=1.1.0-1`
- `golang-github-youmark-pkcs8-dev=1.1-3`
- `golang-golang-x-sys-dev=0.22.0-1`

- [ ] **Step 2: Pin the eight upstream entries built in wave D (Task 12).** Set these fields; leave `ready` unset for now:

| name | version | tag | tag_pattern | packaging | packaging_ref |
|---|---|---|---|---|---|
| keepalived | 2.4.3 | v2.4.3 | `^v(\d+\.\d+\.\d+)$` | https://salsa.debian.org/debian/pkg-keepalived.git | master |
| libteam | 1.32 | v1.32 | `^v(\d+\.\d+)$` | https://salsa.debian.org/debian/libteam.git | master |
| net-snmp | 5.9.5.2 | v5.9.5.2 | `^v(\d+\.\d+\.\d+(?:\.\d+)?)$` | https://salsa.debian.org/debian/net-snmp.git | master |
| ntp | 1.2.5 | NTPsec_1_2_5 | `^NTPsec_(\d+_\d+_\d+)$` | https://salsa.debian.org/debian/ntpsec.git | debian/unstable |
| owamp | 5.2.6 | v5.2.6 | `^v(\d+\.\d+\.\d+)$` | https://github.com/nudanos/owamp | master |
| i2util | 5.2.6 | v5.2.6 | `^v(\d+\.\d+\.\d+)$` | https://github.com/nudanos/i2util | master |
| pam_tacplus | 1.7.0 | v1.7.0 | `^v(\d+\.\d+\.\d+)$` | https://github.com/nudanos/pam_tacplus | master |
| mstpd | 0.2.0 | 0.2.0 | `^(\d+\.\d+\.\d+)$` | https://github.com/nudanos/mstpd | master |

Change `netplug` to `kind: "danos"`, `repo: "https://github.com/nudanos/netplug"`, `ref: "trixie"`, `note: "Debian's 1.2.9.2 plus 12 DANOS patches; a 3.0 (quilt) fork with the full source in git"`, and remove its `upstream`/`track`/`packaging` fields.

- [ ] **Step 3: Replace `TestRepoManifest`'s pilot-only assertion.** From here on, readiness grows wave by wave. Replace the body of `internal/manifest/repo_test.go` with:
```go
package manifest

import "testing"

// The committed manifest must always load; the pilot stays ready; every ready
// upstream entry carries full pins (Validate enforces the fields).
func TestRepoManifest(t *testing.T) {
	m, err := Load("../../manifest.yaml")
	if err != nil {
		t.Fatal(err)
	}
	ready := map[string]bool{}
	for _, k := range []Kind{Danos, Apt, Upstream} {
		for _, e := range m.Ready(k) {
			ready[e.Name] = true
		}
	}
	for _, n := range []string{"dh-yang", "dh-vci", "vyatta-util", "frr", "lintian-profile-vyatta"} {
		if !ready[n] {
			t.Errorf("%s should be ready", n)
		}
	}
}
```

- [ ] **Step 4: Mark `tools/gen-manifest.py` as historical.** Change its first docstring line to `"""Generated the initial manifest.yaml (plan 1). manifest.yaml is now edited by hand; do not re-run.`, and update the header comment in `manifest.yaml` to `# Created by tools/gen-manifest.py (plan 1); edited by hand since plan 2.`

- [ ] **Step 5: Verify**

Run: `go test ./internal/manifest/ -v 2>&1 | grep -E '^(--- |ok|FAIL)' && ./distro-build check-updates; echo "exit=$?"`
Expected: `TestRepoManifest PASS`. `check-updates` reports the eight upstream entries as current, unless a newer upstream release has shipped since 2026-09-28 (then it names it, `exit=1`; bump the pin in the same commit).

- [ ] **Step 6: Commit**

```bash
git add manifest.yaml internal/manifest/repo_test.go tools/gen-manifest.py && git commit -s -m "manifest: Debian's Go libraries where compatible; pin upstream builds

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 8: CI: closure builds with caching

**Files:**
- Modify: `.github/workflows/package.yml`, `.github/workflows/nightly.yml`

**Interfaces:**
- Consumes: `build [name]` (Task 1) and `-jobs` (Task 2).
- Produces: a package PR builds only its closure, restoring `work/out` and `state.json` from its own repo's cache. The nightly builds everything ready and caches the same way.

- [ ] **Step 1: Replace the build step and add caching in `package.yml`.** Replace the `Build against the manifest…` step with:
```yaml
      - uses: actions/cache@v5
        with:
          path: |
            ${{ runner.temp }}/work/out
            ${{ runner.temp }}/work/state.json
            ${{ runner.temp }}/work/mirror-state.json
          key: work-${{ inputs.package }}-${{ github.run_id }}
          restore-keys: work-${{ inputs.package }}-
      - name: Build this package and its build-dependency closure
        working-directory: distro
        run: |
          go build -o distro-build ./cmd/distro-build
          ./distro-build -work "$RUNNER_TEMP/work" builder
          ./distro-build -work "$RUNNER_TEMP/work" -jobs 2 -local "${{ inputs.package }}=$GITHUB_WORKSPACE/pkg" build "${{ inputs.package }}"
```

- [ ] **Step 2: Check the cache action's current major version before committing**

Run: `git ls-remote --tags --refs https://github.com/actions/cache.git | awk '{print $2}' | sed 's#refs/tags/##' | grep -E '^v[0-9]+$' | sort -V | tail -1`
Expected: a tag such as `v5`. Use exactly that in both workflows. If it differs from `v5`, record a ruling.

- [ ] **Step 3: Add the same cache to `nightly.yml` before `Build, sign, install`** (key `work-nightly-${{ github.run_id }}`, restore-keys `work-nightly-`), and change that step to:
```yaml
      - name: Build all ready packages, sign, install the pilot set
        run: |
          go build -o distro-build ./cmd/distro-build
          ./distro-build -work "$RUNNER_TEMP/work" builder
          ./distro-build -work "$RUNNER_TEMP/work" -jobs 2 build
          WORK="$RUNNER_TEMP/work" KEY="$(cat keys/FINGERPRINT)" tests/integration/pilot.sh
```
(`pilot.sh` rebuilds what's already cached, which costs seconds, then signs and runs the install checks.)

- [ ] **Step 4: Push and verify**

```bash
git add .github/workflows && git commit -s -m "ci: build package closures with a per-repo cache; parallel nightly

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>" && git push origin main
gh workflow run nightly -R nudanos/distro
```
Then push an empty commit to `nudanos/dh-yang`'s `trixie` branch, `git -C /Volumes/nudanos/port-dh-yang commit --allow-empty -s -m "ci: exercise closure build" && git -C /Volumes/nudanos/port-dh-yang push`, and watch both runs with `gh run watch --exit-status`.
Expected: both green. The dh-yang run's log shows `build dh-yang` building only `dh-yang`.

---

### Task 9: Wave A: tier 0

**Files:** a `trixie` branch in each repo below; `manifest.yaml` (`ready: true` per repo).

**Interfaces:**
- Consumes: `port.py` (Task 5), the profile (Task 6), the manifest (Task 7), the procedure in `docs/porting.md`.
- Produces: the tier-0 packages, which every later wave builds on.

**Repos (28):** base-files, golang-dbus, golang-github-zeromq-goczmq, golang-jsouthworth-dyn, golang-jsouthworth-hash, libnss-vrfdns, libvci, live-boot-vyatta, lu, tacplusd, utils, vrf-manager, vyatta-base, vyatta-bash, vyatta-cfg-default, vyatta-config-migrate, vyatta-curl-wrapper, vyatta-debian-lldpd-config, vyatta-debian-pam-configs-config, vyatta-debian-passwd-config, vyatta-debian-ssh-server-config, vyatta-debian-system-config, vyatta-debian-systemd-config, vyatta-hotplug, vyatta-ipv6-rtradv, vyatta-lldp, vyatta-mibs-misc, vyatta-version.

Known specifics:
- `golang-dbus` is `3.0 (quilt)` with its source in git; the builder generates the orig tarball.
- `vrf-manager` needs `bvnos-linux-libc-dev` → `linux-libc-dev` and `python-setuptools` → `python3-setuptools`. Check the DANOS-specific kernel headers it includes against Debian's `linux-libc-dev`.
- `libvci`'s `Build-depends` (lowercase) now parses correctly, so it no longer lands in tier 0 by accident. `distro-build plan` orders it.
- `vyatta-hotplug` hard-depends on `vyatta-dataplane`. Wave E relaxes that; until then, build it but expect installing it to fail.

- [ ] **Step 1: Clone, branch and port every repo in the wave**

```bash
cd /Volumes/nudanos
WAVE="base-files golang-dbus golang-github-zeromq-goczmq golang-jsouthworth-dyn golang-jsouthworth-hash libnss-vrfdns libvci live-boot-vyatta lu tacplusd utils vrf-manager vyatta-base vyatta-bash vyatta-cfg-default vyatta-config-migrate vyatta-curl-wrapper vyatta-debian-lldpd-config vyatta-debian-pam-configs-config vyatta-debian-passwd-config vyatta-debian-ssh-server-config vyatta-debian-system-config vyatta-debian-systemd-config vyatta-hotplug vyatta-ipv6-rtradv vyatta-lldp vyatta-mibs-misc vyatta-version"
echo "$WAVE" > /Volumes/nudanos/wave-a.txt
for r in $WAVE; do
  [ -d port-$r ] || git clone -q git@github.com:nudanos/$r.git port-$r
  git -C port-$r checkout -q -B trixie
  python3 distro/tools/port.py port-$r | sed "s/^/$r: /"
  git -C port-$r add -A && git -C port-$r commit -q -s -m "Port to Debian 13 (trixie)" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
done
```
Expected: one line per `NOTE:`. Record each in the ledger.

- [ ] **Step 2: Mark the wave ready and build it locally**

```bash
cd /Volumes/nudanos/distro
WAVE=$(cat /Volumes/nudanos/wave-a.txt)
python3 tools/set_ready.py manifest.yaml $WAVE
LOCALS=""; for r in $WAVE; do LOCALS="$LOCALS -local $r=../port-$r"; done
./distro-build -work /Volumes/nudanos/work -jobs 4 $LOCALS build $WAVE > /Volumes/nudanos/wave-a.log 2>&1; echo "exit=$?"
grep -E '\b(failed|skipped)\b' /Volumes/nudanos/wave-a.log
```
Expected at first: some `failed` lines. That is the work of this wave.

- [ ] **Step 3: Fix every failure at its cause, one repo at a time** (`docs/porting.md` §4). For each failed repo:
  1. Read the first real error in its section of the log.
  2. Fix it in `/Volumes/nudanos/port-<repo>` as a separate commit, with the cause in the message.
  3. Re-run `build <repo>` with the same `-local` flags.

  If a failure is outside the repo (a missing Debian 13 package, a builder gap), fix it where it lives and record a ruling. Never mark a failing package ready without a fix; if it can't be fixed within the wave, set it back to `ready: false`, add a `note:` with the reason, and record a ruling. Repeat until the Step 2 command reports no `failed`/`skipped` lines.

- [ ] **Step 4: Push the branches, add CI callers, commit the manifest**

```bash
cd /Volumes/nudanos
for r in $(cat /Volumes/nudanos/wave-a.txt); do
  mkdir -p port-$r/.github/workflows
  printf 'name: package\non:\n  push:\n    branches: [trixie]\n  pull_request:\n    branches: [trixie]\njobs:\n  build:\n    uses: nudanos/distro/.github/workflows/package.yml@main\n    with:\n      package: %s\n' $r > port-$r/.github/workflows/package.yml
  git -C port-$r add .github && git -C port-$r commit -q -s -m "ci: build with nudanos/distro package workflow" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
  git -C port-$r push -q -u origin trixie
done
cd distro && git add manifest.yaml && git commit -s -m "manifest: wave A (tier 0) ready

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>" && git push origin main
```

- [ ] **Step 5: Verify in CI**

```bash
gh workflow run nightly -R nudanos/distro; sleep 20
gh run watch -R nudanos/distro --exit-status "$(gh run list -R nudanos/distro -w nightly -L 1 --json databaseId --jq '.[0].databaseId')"
for r in $(cat /Volumes/nudanos/wave-a.txt); do gh run list -R nudanos/$r -L 1 --json conclusion --jq ".[0].conclusion" | sed "s/^/$r: /"; done
```
Expected: the nightly is green, and every repo shows `success`.

---

### Task 10: Wave B: tiers 1–2

Same procedure as Task 9 Steps 1–5 (clone/branch/port → mark ready and build locally → fix at the cause → push with callers and commit the manifest → verify in CI). Use `WAVE` = the list below, `/Volumes/nudanos/wave-b.txt` and `/Volumes/nudanos/wave-b.log`.

**Repos (35):** aaa, cli-sandbox, golang-github-jsouthworth-objtree, golang-jsouthworth-transduce, golang-jsouthworth-try, mgmterror, routing-instance, vyatta-cfg, vyatta-cfg-dataplane, vyatta-cfg-journalbeat, vyatta-client-ssh, vyatta-config-mgmt, vyatta-image-tools, vyatta-interfaces-l2tpeth, vyatta-log, vyatta-op, vyatta-openvpn-yang, vyatta-service-bridge, vyatta-service-gnss, vyatta-service-ntp, vyatta-service-portmonitor, vyatta-service-snmp, vyatta-service-ssh, vyatta-service-switch, vyatta-service-telnet, vyatta-service-twamp, vyatta-snmp-vrf-agent, vyatta-sssd, vyatta-syslog, vyatta-tech-support, vyatta-vrrp, golang-jsouthworth-seq, vyatta-cfg-bonding, vyatta-cfg-system, vyatta-login, vyatta-tacacs.

Known specifics:
- `golang-github-jsouthworth-objtree` is `3.0 (quilt)`; the builder generates the orig tarball.
- `vyatta-cfg-system`: `pylint3` → `pylint`, done by port.py. Expect pylint 3.x to flag new warnings. Fix the code, or scope the check to errors (`pylint -E`) with a ruling.
- `vyatta-vrrp`: `python3-pytest-pep8` is dropped. Run `pycodestyle` from the test target instead, if the tests relied on it.
- `vyatta-snmp-vrf-agent`: `bvnos-linux-libc-dev` → `linux-libc-dev`. Check its kernel headers, like `vrf-manager`.
- `vyatta-cfg-dataplane` depends on the dataplane protobuf libraries. If it fails before wave E, set it back to `ready: false` with note `needs wave E protobuf-only dataplane` and port it in Task 13.
- `vyatta-service-bridge` depends on `vplane-config-npf` (milestone 2). Wave E decides whether to relax that; here, port and build only.
- `vyatta-sssd`, `vyatta-platform`: port.py notes any Python 2 build dependency. Move it to the `python3-` equivalent.

- [ ] **Step 1:** Clone, branch and port the repos (as Task 9 Step 1, with this list).
- [ ] **Step 2:** Mark them ready and build locally (as Task 9 Step 2, with this list).
- [ ] **Step 3:** Fix every failure at its cause (as Task 9 Step 3).
- [ ] **Step 4:** Push the branches with CI callers and commit the manifest (as Task 9 Step 4). The manifest commit message is `manifest: wave B (tiers 1-2) ready`.
- [ ] **Step 5:** Verify in CI (as Task 9 Step 5).

---

### Task 11: Wave C: the Go management core

Same procedure as Task 9 Steps 1–5. Use `WAVE` = the list below, `/Volumes/nudanos/wave-c.txt` and `/Volumes/nudanos/wave-c.log`.

**Repos (20), in dependency order:** golang-jsouthworth-immutable, golang-github-danos-encoding-rfc7951, golang-jsouthworth-etm, vci, yang, config, ephemerad, notifyd, vyatta-interfaces, vyatta-protocols-common, vyatta-service-dns, xpath-plugins, configd, op, provisiond, vyatta-protocols-frr, ifmgrd, opd, vyatta-kdump, vyatta-rest.

Known specifics:
- **Go 1.26 and GOPATH builds.** dh-golang builds these in GOPATH mode.
  - If Go 1.26 rejects GOPATH mode (`go: modules disabled by GO111MODULE=off` or similar), stop and record a ruling. The fallback is Debian 13's default Go 1.24 for wave C: remove the `golang-go` lines from `builder/preferences`, rebuild the builder, and pull the M1.1 modules conversion forward only if 1.24 fails too.
  - Expect `go vet` failures in test runs, such as printf verbs and copylocks. Fix the code; don't disable vet.
- **The `encoding` duplicate.** `encoding` (milestone later) and `golang-github-danos-encoding-rfc7951` both produce `golang-github-danos-encoding-rfc7951-dev`. Only the latter is ready, so `plan` stays unambiguous. Keep it that way.
- **`vyatta-protocols-frr`** configures FRR through `vtysh`/`frr.conf`. FRR 10.7's CLI differs from 7.5 in places. Build-time tests catch syntax only; runtime behaviour is plan 3's scenario tests. Note anything suspicious in the ledger for plan 3.
- **`opd` imports `brocade.com/vyatta/cmdclient` in `cmd/opstress`.** If dh-golang builds that command, exclude it with `DH_GOLANG_EXCLUDES := cmd/opstress` in `debian/rules`, as a separate commit.

- [ ] **Step 1:** Clone, branch and port the repos (as Task 9 Step 1, with this list).
- [ ] **Step 2:** Mark them ready and build locally (as Task 9 Step 2, with this list).
- [ ] **Step 3:** Fix every failure at its cause (as Task 9 Step 3).
- [ ] **Step 4:** Push the branches with CI callers and commit the manifest (as Task 9 Step 4). The manifest commit message is `manifest: wave C (Go management core) ready`.
- [ ] **Step 5:** Verify in CI (as Task 9 Step 5).

---

### Task 12: Wave D: upstream builds and the patch audit

**Files:** `manifest.yaml` (`ready`, `audit`), `patches/<name>/*.patch`, `nudanos/netplug` `trixie` branch.

**Interfaces:**
- Consumes: the upstream kind (Task 3) and the pins (Task 7).
- Produces: the eight upstream packages at their pinned latest release, and netplug. Each DANOS patch carries a recorded verdict.

**Packages:** keepalived, libteam, net-snmp, ntp (ntpsec), owamp, i2util, pam_tacplus, mstpd; and netplug (`danos` kind).

- [ ] **Step 1: Build each with its packaging unmodified.** Set `ready: true` on all eight, then:
```bash
cd /Volumes/nudanos/distro && ./distro-build -work /Volumes/nudanos/work -jobs 4 build keepalived libteam net-snmp ntp owamp i2util pam_tacplus mstpd > /Volumes/nudanos/wave-d.log 2>&1; echo "exit=$?"
grep -E '\b(failed|skipped)\b' /Volumes/nudanos/wave-d.log
```
Typical failures are Debian quilt patches that no longer apply to the newer upstream. Refresh a patch if it's still needed, and drop it if upstream absorbed it. The Debian-packaged four live in salsa, and we don't fork their packaging. Put fixes to their `debian/` as patches under `patches/<name>/`, and modify packaging only through those patches.

For `owamp`, `i2util`, `pam_tacplus` and `mstpd`, the packaging is our fork's `master` `debian/`. Carry changes on a `trixie` branch of that fork, and set `packaging_ref: trixie` for the entry.

- [ ] **Step 2: Audit the DANOS patches** (spec §4.1). For each package, list the DANOS-authored patches in the original fork: `git -C "$HOME/Documents/Claude/Projects/danOS Project/mirrors/<fork>.git" show master:debian/patches/series`, filtered to vyatta/danos/att-authored ones as in `tools/trixie_gap.py`. For each patch:
  - **Upstreamed:** the change is already in the pinned release. Grep the upstream tree at the tag for the patch's added lines.
  - **Kept:** a milestone-1 DANOS package depends on the behaviour. Grep the ported DANOS repos for the option, hook or file the patch adds. Copy the patch to `patches/<name>/`, refreshed so it applies.
  - **Dropped:** neither.

  Record every verdict in the entry's `audit:` list with a one-line `note:`.

  Expected volume (from `trixie-gap.json`): keepalived 18, owamp 28, libteam 10, net-snmp 5, ntp 3, mstpd 5. i2util and pam_tacplus have none.
- [ ] **Step 3: netplug.** Port `nudanos/netplug` with `tools/port.py` on a `trixie` branch. It's `3.0 (quilt)` with the full source in git, so the builder generates the orig. Mark it ready and build it.
- [ ] **Step 4: Rebuild the wave, and every ready package** (dependents must build against the new libraries: net-snmp for the SNMP packages, libteam for bonding):
```bash
./distro-build -work /Volumes/nudanos/work -jobs 4 build 2>&1 | grep -E '\b(failed|skipped)\b'; echo "grep exit=$? (1 = none failed)"
```
Expected: `grep exit=1`.
- [ ] **Step 5: Commit, push, verify.** Commit the manifest, patches and pins (`manifest: wave D upstream builds ready, patch audit recorded`). Push any `trixie` packaging branches. Run the nightly and confirm it's green. Then run `./distro-build check-updates` and confirm `all pinned versions are current`.

---

### Task 13: Wave E: split the dataplane for kernel forwarding

**Files:**
- `nudanos/vyatta-dataplane` `trixie`: `meson_options.txt`, `meson.build`, `debian/control`, `debian/rules`
- `nudanos/vyatta-cfg-system`, `nudanos/vyatta-hotplug`, `nudanos/vyatta-cfg-bonding`, `nudanos/vyatta-interfaces` (whichever produce the packages below): `debian/control`
- New repo `nudanos/vyatta-kernel-forwarding` (**STOP for go-ahead before creating it**)

**Interfaces:**
- Consumes: the Debian build profile mechanism (`<!pkg.vyatta-dataplane.protobuf-only>`).
- Produces (spec §4.4):
  - Profile `pkg.vyatta-dataplane.protobuf-only`: builds only the protobuf packages, with no DPDK build dependency.
  - Virtual package `vyatta-forwarding`, provided by `vyatta-dataplane` (M2) and by the new `vyatta-kernel-forwarding` (M1).
  - `vyatta-system`, `vyatta-system-network-v1-yang`, `vyatta-hotplug` and `vyatta-interfaces-bonding` depend on `vyatta-forwarding` instead of `vyatta-dataplane`.

- [ ] **Step 1: Add a meson option to the dataplane that builds only `protobuf/`.** In `meson_options.txt`, add:
```text
option('protobuf_only', type : 'boolean', value : false, description : 'Build only the protobuf libraries (no DPDK)')
```
In `meson.build`, wrap everything from `dpdk_dep = dependency('libdpdk'…` through the end of the file, except `subdir('protobuf')` and the dependencies it needs, in `if not get_option('protobuf_only')`. Move `subdir('protobuf')` and `protobuf_dep`/`proto_c_dep` above that `if`. Read `protobuf/meson.build` first to find exactly which variables it uses, and keep those outside the `if`.

- [ ] **Step 2: Wire the Debian build profile.** In `debian/control`, mark every DPDK-only build dependency (`libdpdk-dev`, `libvyatta-dpdk-swport-dev`, `librte-*`, `bvnos-linux-libc-dev-vyatta`, `libndpi-dev`, `libosip2-dev`, `libnuma-dev`, `libpcap-dev`, `libmnl-dev`, `liburcu-dev`, `libczmq-dev`, `check`, `lcov`) with ` <!pkg.vyatta-dataplane.protobuf-only>`. Mark every binary package other than the protobuf ones with `Build-Profiles: <!pkg.vyatta-dataplane.protobuf-only>`. The protobuf ones are `libvyatta-dataplane-proto*`, `libvyatta-dataplane-proto-support`, `golang-github-danos-vyatta-dataplane-protobuf-dev` and `vyatta-dataplane-protocols-versions`. In `debian/rules`, pass `-Dprotobuf_only=true` to `dh_auto_configure` when `$(DEB_BUILD_PROFILES)` contains `pkg.vyatta-dataplane.protobuf-only`.

- [ ] **Step 3: Let the manifest request a build profile, test first.** Append to `internal/manifest/manifest_test.go`:
```go
func TestProfiles(t *testing.T) {
	m, err := Parse([]byte("packages:\n  - {name: d, kind: danos, milestone: \"1.0\", repo: r, ref: trixie, profiles: [pkg.vyatta-dataplane.protobuf-only]}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got := m.Packages[0].Profiles; len(got) != 1 || got[0] != "pkg.vyatta-dataplane.protobuf-only" {
		t.Errorf("Profiles = %v", got)
	}
}
```
and to `cmd/distro-build/main_test.go` (add `"reflect"` to its imports):
```go
func TestBuildEnvCarriesProfiles(t *testing.T) {
	got := buildEnv("vyatta-dataplane", 501, 20, 4, []string{"pkg.a", "pkg.b"})
	want := map[string]string{"PKG": "vyatta-dataplane", "HOST_UID": "501", "HOST_GID": "20", "JOBS": "4",
		"DEB_BUILD_PROFILES": "pkg.a pkg.b"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildEnv = %v, want %v", got, want)
	}
	if _, ok := buildEnv("x", 0, 0, 1, nil)["DEB_BUILD_PROFILES"]; ok {
		t.Error("no profiles must not set DEB_BUILD_PROFILES")
	}
}
```
Run: `go test ./internal/manifest/ ./cmd/distro-build/`
Expected: FAIL (`field profiles not found`, `undefined: buildEnv`)

Implement: add `Profiles []string \`yaml:"profiles,omitempty"\`` to `manifest.Entry`. In `main.go`, add:
```go
// buildEnv is the environment for one package build.
func buildEnv(name string, uid, gid, jobs int, profiles []string) map[string]string {
	env := map[string]string{"PKG": name, "HOST_UID": strconv.Itoa(uid), "HOST_GID": strconv.Itoa(gid),
		"JOBS": strconv.Itoa(jobs)}
	if len(profiles) > 0 {
		env["DEB_BUILD_PROFILES"] = strings.Join(profiles, " ")
	}
	return env
}
```
`containerBuild` needs each package's profiles, so give it the manifest: change it to `func (a *app) containerBuild(m *manifest.Manifest) build.Func`. Inside, build `profiles := map[string][]string{}` from `m.Packages`. In the returned func, compute `u, g := a.eng.OwnerIDs(os.Getuid(), os.Getgid())` and set `Env: buildEnv(name, u, g, max(1, runtime.NumCPU()/max(1, a.jobs)), profiles[name])`. Update the call in `run` to `a.containerBuild(m)`. In `build-package.sh`, replace the build-dep and dpkg-buildpackage lines with:
```bash
    local prof=()
    [ -n "${DEB_BUILD_PROFILES:-}" ] && prof=(-P"${DEB_BUILD_PROFILES// /,}")
    apt-get -y --no-install-recommends "${prof[@]}" build-dep ./
    chown -R builder:builder /build
    runuser -u builder -- env DEB_BUILD_OPTIONS="parallel=${JOBS:-1}" DEB_BUILD_PROFILES="${DEB_BUILD_PROFILES:-}" dpkg-buildpackage -us -uc -I -i "${prof[@]}"
```
Run: `go test ./internal/manifest/ ./cmd/distro-build/ -v 2>&1 | grep -E '^(--- |ok|FAIL)'`
Expected: `TestProfiles` and `TestBuildEnvCarriesProfiles` `PASS`. Then set `profiles: ["pkg.vyatta-dataplane.protobuf-only"]` on the `vyatta-dataplane` entry.

- [ ] **Step 4: STOP: get the go-ahead to create `nudanos/vyatta-kernel-forwarding`**, then create it with this `debian/` (3.0 native, version 0.1):
```text
Source: vyatta-kernel-forwarding
Section: net
Priority: optional
Maintainer: NuDanOS Maintainers <jon@fernandez.tech>
Build-Depends: debhelper-compat (= 13)
Standards-Version: 4.7.2
Rules-Requires-Root: no
Vcs-Git: https://github.com/nudanos/vyatta-kernel-forwarding.git
Vcs-Browser: https://github.com/nudanos/vyatta-kernel-forwarding

Package: vyatta-kernel-forwarding
Architecture: all
Depends: ${misc:Depends}
Provides: vyatta-forwarding
Conflicts: vyatta-dataplane
Description: NuDanOS forwarding in the Linux kernel (no DPDK dataplane)
 Satisfies vyatta-forwarding for systems that forward packets in the Linux
 kernel instead of the DPDK dataplane. Firewall, NAT and QoS require the
 dataplane and are not available with this package.
```
Also create `debian/rules` (`%:` / `\tdh $@`), `debian/changelog` (0.1, trixie, NuDanOS Maintainers), `debian/copyright` (LGPL-2.1, matching DANOS) and `debian/source/format` (`3.0 (native)`). Add a manifest entry: `kind: danos`, `ready: true`, `milestone: "1.0"`.

- [ ] **Step 5: Relax the hard dataplane dependencies.** For each binary package below, find its source repo with `grep -l "^Package: <pkg>$" /Volumes/nudanos/port-*/debian/control`. In its `Depends:`, replace `vyatta-dataplane` with `vyatta-forwarding`, and `libvyatta-dataplane-proto-support` stays as is (now buildable). The packages are `vyatta-system`, `vyatta-system-network-v1-yang`, `vyatta-hotplug` and `vyatta-interfaces-bonding`.

  For `vyatta-service-bridge`, read how it uses `vplane-config-npf` (`grep -rn npf` in the repo). If only bridge-firewall features need it, move that dependency to a `Recommends:`. Otherwise set the package to milestone 2 in the manifest with a note. Record either way as a ruling.

- [ ] **Step 6: Build and verify installability.** Build everything ready, then install the forwarding-dependent packages in a clean container:
```bash
./distro-build -work /Volumes/nudanos/work -jobs 4 build 2>&1 | grep -E '\b(failed|skipped)\b'; echo "grep exit=$?"
./distro-build -work /Volumes/nudanos/work -key "$(cat keys/FINGERPRINT)" repo >/dev/null
docker run --rm -v /Volumes/nudanos/work/repo:/repo:ro debian:trixie bash -euc '
  apt-get update -qq && apt-get install -y -qq ca-certificates gpg >/dev/null
  gpg --dearmor < /repo/nudanos-archive-keyring.asc > /usr/share/keyrings/nudanos.gpg
  echo "deb [signed-by=/usr/share/keyrings/nudanos.gpg] file:/repo trixie main" > /etc/apt/sources.list.d/nudanos.list
  apt-get update -qq
  apt-get install -y --no-install-recommends vyatta-kernel-forwarding vyatta-system vyatta-hotplug libvyatta-dataplane-proto-support >/dev/null
  dpkg -s vyatta-dataplane >/dev/null 2>&1 && { echo "dataplane got installed"; exit 1; }; echo "install ok without dataplane"'
```
Expected: `grep exit=1` (nothing failed), then `install ok without dataplane`.

- [ ] **Step 7: Commit, push, verify in CI** (as Task 9 Steps 4–5). This includes `vyatta-dataplane` (protobuf-only) and `vyatta-kernel-forwarding` with CI callers, and any wave B packages that were deferred to here.

---

### Task 14: Archive the `debian` and `drop` repos (STOP for go-ahead)

**Files:** none (GitHub state); `docs/superpowers/specs/2026-09-28-debian13-revival-design.md` §9.

- [ ] **Step 1: List the candidates**

```bash
cd /Volumes/nudanos/distro && python3 - <<'PY'
import re
t = open("manifest.yaml").read().split("\n  - ")[1:]
for e in t:
    kind = re.search(r'kind: "?(\w+)', e).group(1)
    name = re.search(r'name: "?([^"\n]+)', e).group(1)
    if kind in ("debian", "drop"):
        print(name)
PY
```
Expected: 46 names (26 debian and 15 drop entries, plus the five Go libraries switched to `debian` in Task 7).

- [ ] **Step 2: STOP and show the user the list.** Archiving makes a repo read-only; it is reversible with `gh repo unarchive`. Proceed only on an explicit yes.
- [ ] **Step 3: Archive**

```bash
for r in $(python3 - <<'PY'
import re
t = open("manifest.yaml").read().split("\n  - ")[1:]
for e in t:
    if re.search(r'kind: "?(debian|drop)', e):
        print(re.search(r'name: "?([^"\n]+)', e).group(1))
PY
); do gh repo archive nudanos/$r --yes && echo "archived $r"; done
```
- [ ] **Step 4: Verify and record**

Run: `gh repo list nudanos -L 300 --archived --json name --jq length`
Expected: the Step 1 count. Then update spec §9's `0: Launch` row to say archiving is done (date), and commit (`spec: archiving done`).
