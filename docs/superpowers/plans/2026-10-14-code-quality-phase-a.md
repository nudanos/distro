# Code quality Phase A (analyzers down to zero) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `distro-build lint` reports every analyzer finding in the 95 in-scope ports, the nightly stops any port getting worse, and every port is fixed to zero findings.

**Architecture:** analyzer output is parsed into one `lint.Finding` type in a new `internal/lint` package. Source tools (gofmt, perltidy, perlcritic, shellcheck, flake8, pylint, cppcheck) run in a new `nudanos/lint:trixie` image through a cross-compiled in-container runner, `cmd/lint`. Build-time findings (C/C++ compiler warnings, staticcheck) are read from `work/logs/<name>.log`. The `distro-build lint` subcommand merges both, prints them, and applies a per-port baseline ratchet, which is deleted once every port reaches zero.

**Tech Stack:** Go 1.26 (the distro module), Debian 13 analyzer packages, Podman/Docker via `internal/engine`, GitHub Actions.

**Spec:** `docs/superpowers/specs/2026-10-14-code-quality-design.md` (§1–§4 and §6; Phase B gets its own plans).

## Global Constraints

- In scope: `kind: danos` manifest entries with `milestone: "1.0"` and `ready: true` (95 on 2026-10-14), plus distro's own Go code. Patches against upstream code are not linted (spec §4.5).
- Tools and versions (trixie): gofmt (Go toolchain), go-staticcheck 2024.1, perltidy 20250105, libperl-critic-perl 1.156 (severity 3), shellcheck 0.10.0 (all levels), flake8 7.1.1, pylint 3.3.4 (no docstring or naming-convention checks), cppcheck 2.17 (`warning,style,performance,portability`), compiler warnings with `-Wall -Wextra`.
- A suppression is valid only inline, in the tool's own syntax, with `reason:` followed by text on the same line. lint fails on any suppression without one.
- Per-port configuration is `nudanos-lint.yaml` at the port root, and it may only list paths to skip.
- Commits: identity `NuDanOS <jon@fernandez.tech>`, `git commit -s`, trailer `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`. Committing and pushing distro `main` and port `trixie` branches are standing approvals. Creating repos, archiving and deleting need the user first.
- Every decision is ledgered in `.superpowers/sdd/<plan>/progress.md` as `Ruling: <what> — <why> — <cost if wrong>`.
- VM test runs go one at a time, never concurrently and never alongside a build. Work runs on VM 130 (`/srv/nudanos`, `-engine podman`, `/Volumes/nudanos` → `/srv/nudanos`).
- Never handle secrets: the archive key and `gh auth` are the user's.
- Starts after checkpoint 1.0 is closed (plan 4 Task 17).

## Review Focus

1. **A script with no file extension** (most vyatta scripts are `#!/usr/bin/perl` or `#!/bin/bash` under `scripts/`) must be linted by its shebang, not skipped. Pinned in Task 2.
2. **A tool that crashes or cannot parse a file** (a Perl syntax error, cppcheck's missing-include noise, pylint's import errors when dependencies are absent) must be an error that fails lint, never "0 findings". Pinned in Task 1 and Task 5.
3. **A suppression whose `reason:` is empty, or on the line below,** counts as unexplained. Pinned in Task 3.
4. **A port newly added to scope, or missing from the baseline,** starts at zero, not unlimited. Pinned in Task 4.
5. **A package served from the build cache,** whose log predates `-Wall -Wextra` or staticcheck, must be reported as a stale or missing build log, not as zero warnings. Pinned in Task 6.

---

### Task 1: Findings and tool-output parsers

**Files:**
- Create: `internal/lint/finding.go`, `internal/lint/parse.go`
- Test: `internal/lint/parse_test.go`, `internal/lint/testdata/out/*.txt` (real output captured from each trixie tool)

**Interfaces:**
- Produces:
  - `type Finding struct { Port, Tool, File string; Line int; Rule, Message string }`
  - `func (f Finding) String() string` → `port: file:line: tool rule: message`
  - `func Parse(tool string, out []byte) ([]Finding, error)` for tool ∈ `gofmt perltidy perlcritic shellcheck flake8 pylint cppcheck`; Port is left empty for the caller.
  - The output formats the runner (Task 5) must request:
    - gofmt: `gofmt -l`, one finding per file, rule `gofmt`;
    - perltidy: one finding per file whose tidied copy differs, rule `perltidy`;
    - perlcritic: `--verbose '%f:%l:%c:%p:%m\n'`;
    - shellcheck: `-f gcc`;
    - flake8: default `%(path)s:%(row)d:%(col)d: %(code)s %(text)s`;
    - pylint: `--output-format=parseable`;
    - cppcheck: `--template='{file}:{line}:{id}:{message}'`.

- [ ] **Step 1: Capture real output.** Run each tool in a `debian:trixie` container on a small file with one known finding, and save its output to `testdata/out/<tool>.txt`. Also save `pylint-fatal.txt` (an F0001 or syntax error) and `cppcheck-error.txt` (a syntaxError).
- [ ] **Step 2: Write the failing tests:**
  - `TestParseEachTool` (table-driven): for each tool, the parsed `[]Finding` equals the expected File, Line, Rule and Message.
  - `TestParseToolErrorIsAnError`: pylint fatal or syntax messages and cppcheck `syntaxError` return a non-nil error naming the file.
- [ ] **Step 3: Run** `go test ./internal/lint/ -run TestParse -count=1`. Expected: FAIL (package does not exist).
- [ ] **Step 4: Implement** `Finding`, `String` and `Parse`. Use one line-oriented parser per tool, selected by name in a map. An unknown tool name is an error.
- [ ] **Step 5: Run** the same test. Expected: PASS.
- [ ] **Step 6: Commit** "lint: findings and analyzer output parsers".

### Task 2: Which files each tool checks

**Files:**
- Create: `internal/lint/files.go`
- Test: `internal/lint/files_test.go`, `internal/lint/testdata/port/` (a fixture port tree)

**Interfaces:**
- Produces:
  - `type Config struct { Skip []string }`, loaded from `nudanos-lint.yaml` (`skip:` list of `path.Match` globs relative to the port root; any other key is an error).
  - `func LoadConfig(portDir string) (Config, error)`: a missing file gives an empty Config.
  - `func Classify(portDir string, cfg Config) (map[string][]string, error)`: language → relative paths. Languages: `go perl shell python c`.

- [ ] **Step 1: Write the failing tests** against `testdata/port`:
  - `TestClassifyByExtensionAndShebang`:
    - `.go` → go;
    - `.pl`, `.pm`, `.t`, and an extensionless file starting `#!/usr/bin/perl` → perl;
    - `.sh`, an extensionless `#!/bin/bash` file, and `debian/postinst` → shell;
    - `.py` and a `#!/usr/bin/python3` file → python;
    - `.c`, `.h`, `.cc`, `.cpp`, `.hpp` → c.
  - `TestClassifySkipsByDefault`: `.git/`, `vendor/`, `debian/patches/`, `debian/tmp/` and `debian/<pkg>/` (build staging) are never listed.
  - `TestConfigSkipGlobs`: a port with `skip: ["generated/*"]` drops `generated/x.go`. An unknown key such as `rules:` is an error.
- [ ] **Step 2: Run** `go test ./internal/lint/ -run 'TestClassify|TestConfig' -count=1`. Expected: FAIL.
- [ ] **Step 3: Implement** with `filepath.WalkDir`. For files without a known extension, read only the first line to find a shebang.
- [ ] **Step 4: Run** the same tests. Expected: PASS.
- [ ] **Step 5: Ledger** `Ruling: vendor/ is not linted — third-party code we do not own (spec §1); go vet still covers it in the build — cost if wrong: findings in vendored code go unreported.` Then commit "lint: classify port files by language".

### Task 3: Unexplained suppressions

**Files:**
- Create: `internal/lint/suppress.go`
- Test: `internal/lint/suppress_test.go`

**Interfaces:**
- Produces: `func Unexplained(lang, path string, src []byte) []Finding`, returning findings with Tool `lint` and Rule `suppression-without-reason`.
- Markers per language:

  | Language | Markers |
  |---|---|
  | go | `//lint:ignore`, `//lint:file-ignore`, `//nolint` |
  | perl | `## no critic`, `#<<<` |
  | shell | `# shellcheck disable=` |
  | python | `# noqa`, `# pylint: disable` |
  | c | `// cppcheck-suppress`, `/* cppcheck-suppress`, `#pragma GCC diagnostic ignored`, `#pragma clang diagnostic ignored` |

- [ ] **Step 1: Write the failing test.** `TestUnexplainedSuppressions` (table-driven) expects exactly one finding for each of these:
  - a marker without `reason:`;
  - `reason:` followed only by spaces;
  - the reason on the next line instead of the same one.

  It expects none for `# shellcheck disable=SC2086 # reason: word splitting is intended`, and none for a marker that appears inside a string literal on a line that isn't a comment.
- [ ] **Step 2: Run** `go test ./internal/lint/ -run TestUnexplained -count=1`. Expected: FAIL.
- [ ] **Step 3: Implement** a per-line scan. The marker must begin a comment, and `reason:\s*\S` must follow it on the same line.
- [ ] **Step 4: Run** the same test. Expected: PASS.
- [ ] **Step 5: Commit** "lint: a suppression needs a reason".

### Task 4: The baseline ratchet

**Files:**
- Create: `internal/lint/baseline.go`
- Test: `internal/lint/baseline_test.go`

**Interfaces:**
- Produces:
  - `type Baseline map[string]int` (port → allowed findings), stored as `lint-baseline.json` at the distro root.
  - `func LoadBaseline(path string) (Baseline, bool, error)`: false when the file does not exist, which means zero is enforced everywhere.
  - `func (b Baseline) Check(counts map[string]int) []string`: one message per port over its allowance.
  - `func (b Baseline) Update(counts map[string]int, ports []string) (Baseline, error)`: lowers the named ports to their current count, deleting zero entries. It refuses any increase.
  - `func NewBaseline(counts map[string]int) Baseline`: the first baseline, with ports at zero left out.
  - `func (b Baseline) Save(path string) error`: sorted keys, one per line.

- [ ] **Step 1: Write the failing tests:**
  - `TestCheckFailsAboveAllowance`.
  - `TestCheckPortMissingFromBaselineMustBeZero`: a port absent from the baseline with 1 finding fails.
  - `TestNoBaselineFileMeansZeroEverywhere`.
  - `TestUpdateOnlyLowers`: an increase is an error naming the port, and a port at 0 is removed.
  - `TestSaveIsStable`: same input, byte-identical file.
  - `TestNewBaselineOmitsZero`.
- [ ] **Step 2: Run** `go test ./internal/lint/ -run 'TestCheck|TestNoBaseline|TestUpdate|TestSave|TestNewBaseline' -count=1`. Expected: FAIL.
- [ ] **Step 3: Implement** the five functions.
- [ ] **Step 4: Run** the same tests. Expected: PASS.
- [ ] **Step 5: Commit** "lint: per-port baseline ratchet".

### Task 5: The lint image and the in-container runner

**Files:**
- Create:
  - `lint/Dockerfile` (`FROM debian:trixie`, installs `golang-go perltidy libperl-critic-perl shellcheck flake8 pylint cppcheck`);
  - `lint/perltidyrc`;
  - `lint/perlcriticrc` (`severity = 3`);
  - `lint/pylintrc` (disables `missing-module-docstring missing-class-docstring missing-function-docstring invalid-name import-error`);
  - `lint/flake8.cfg`.
- Create: `cmd/lint/main.go`
- Test: `cmd/lint/main_test.go`, `tests/integration/lint-fixture.sh`

**Interfaces:**
- Consumes: `lint.Parse`, `lint.Classify`, `lint.LoadConfig`, `lint.Unexplained` (Tasks 1–3).
- Produces:
  - The `lint` binary. `lint -port <name> /src` writes `[]lint.Finding` as JSON on stdout.
  - Exit status 2 on a tool error (stderr names the tool and file); 0 otherwise, findings or not.
  - Profiles are read from `/lint` in the image.

- [ ] **Step 1: Write the failing test.** `TestToolArgs` checks the exact command line per tool:
  - perlcritic gets `--profile /lint/perlcriticrc` and the verbose format from Task 1;
  - pylint gets `--rcfile /lint/pylintrc --output-format=parseable`;
  - cppcheck gets `--enable=warning,style,performance,portability --suppress=missingIncludeSystem --inline-suppr --template=…`;
  - shellcheck gets `-f gcc -S style`.
- [ ] **Step 2: Run** `go test ./cmd/lint/ -count=1`. Expected: FAIL.
- [ ] **Step 3: Implement** `func toolArgs(tool string, files []string) []string` and `main`:
  1. classify;
  2. run each tool once per language over its file list (perltidy once per file into a temp copy, then compare);
  3. parse;
  4. add `Unexplained` findings;
  5. set Port;
  6. print JSON.

  A non-zero tool exit that isn't its "findings present" status is an error.
- [ ] **Step 4: Run** `go test ./cmd/lint/ -count=1`. Expected: PASS.
- [ ] **Step 5: Write `tests/integration/lint-fixture.sh`.** It builds the image, cross-compiles `cmd/lint`, runs it on `internal/lint/testdata/port`, and asserts:
  - exactly one finding per tool (gofmt, perltidy, perlcritic, shellcheck, flake8, pylint, cppcheck);
  - exactly one `suppression-without-reason` finding;
  - no finding for the explained suppression;
  - exit status 2 when a file with a Perl syntax error is added.

  Plant those findings in the fixture now.
- [ ] **Step 6: Run** `bash tests/integration/lint-fixture.sh`. Expected: `lint-fixture: OK`.
- [ ] **Step 7: Ledger** two rulings:
  - `Ruling: lint gets its own image (lint/, nudanos/lint:trixie) instead of the builder — the builder's hash is in every package cache key; adding analyzers there rebuilds all packages — cost if wrong: two images to keep on trixie (spec §2 amended).`
  - `Ruling: pylint import-error disabled — ports' Python dependencies are not installed in the lint image — cost if wrong: a misspelled import is caught only by the package tests.`

  Then commit "lint: analyzer image and in-container runner".

### Task 6: Build-log findings and flags

**Files:**
- Create: `internal/lint/buildlog.go`
- Modify:
  - `cmd/distro-build/main.go`: `buildEnv` and `keyExtras` (in-scope ports get `DEB_CFLAGS_MAINT_APPEND`/`DEB_CXXFLAGS_MAINT_APPEND = -Wall -Wextra`, and `flags=wall-wextra` in their cache key);
  - `docs/porting.md`: the staticcheck rule for Go ports.
- Test: `internal/lint/buildlog_test.go`, `cmd/distro-build/main_test.go`

**Interfaces:**
- Produces:
  - `func FromBuildLog(port string, log []byte) ([]Finding, error)`, covering:
    - gcc/g++ `file:line:col: warning: msg [-Wflag]`, with Tool `cc` and Rule the flag. Paths under `/usr/` are ignored.
    - lines prefixed `nudanos-staticcheck: file:line:col: msg (SA1234)`, with Tool `staticcheck`.
  - A C port's log without the first line `nudanos-lint-flags: wall-wextra` gives an error: "build log predates lint flags, rebuild <port>". `distro-build` writes that line into the log itself (`packageLog`) before starting the container, so `builder/` and its hash do not change.
  - A Go port's log without `nudanos-staticcheck: done` gives one finding, Tool `lint` and Rule `staticcheck-not-run`. It's counted in the baseline and cleared when Task 9 adds the target.
- The Go-port convention, in `docs/porting.md`, next to the existing vet target:
  ```make
  staticcheck:
  	… staticcheck ./... 2>&1 | sed 's/^/nudanos-staticcheck: /' || true
  	@echo 'nudanos-staticcheck: done'
  ```
  It goes into `override_dh_auto_build` after `vet`, with `go-staticcheck <!nocheck>` added to Build-Depends.

- [ ] **Step 1: Write the failing tests:**
  - `TestFromBuildLogWarnings` (two warnings, one in `/usr/include` ignored);
  - `TestFromBuildLogStaticcheck`;
  - `TestFromBuildLogStaleIsAnError` (C port, no flags line);
  - `TestFromBuildLogGoWithoutStaticcheckIsAFinding`;
  - `TestPackageLogWritesFlagsLine` (in `cmd/distro-build`, in scope only);
  - in `cmd/distro-build`, `TestBuildEnvAddsWarningFlagsInScope` and `TestKeyExtrasIncludeFlagsInScope` (an out-of-scope upstream entry gets neither).
- [ ] **Step 2: Run** `go test ./internal/lint/ ./cmd/distro-build/ -count=1`. Expected: FAIL.
- [ ] **Step 3: Implement.** `packageLog` gains the in-scope flag. If `DEB_CFLAGS_MAINT_APPEND` does not reach `dpkg-buildpackage` through `runuser` (Step 5 shows it), forwarding it needs a `builder/build-package.sh` change. That alters the builder hash and means a one-time full rebuild in Task 8: ledger it as a Ruling.
- [ ] **Step 4: Run** the tests. Expected: PASS.
- [ ] **Step 5: Verify flags reach the compiler.** Run `distro-build -engine podman build libvci`, then `grep -c -- '-Wextra' work/logs/libvci.log`. Expected: ≥ 1.
- [ ] **Step 6: Commit** "lint: findings from build logs; -Wall -Wextra for in-scope C ports".

### Task 7: `distro-build lint`

**Files:**
- Modify: `cmd/distro-build/main.go` (the `lint` case in `run`, `func (a *app) lint(ctx, names []string) error`, flag `-update-baseline`, usage string)
- Test: `cmd/distro-build/main_test.go`

**Interfaces:**
- Consumes: `a.fetch` (honours `-local`), `lint.LoadBaseline`, `lint.FromBuildLog`, the `lint` binary, and the image (Task 5), built with `a.eng.BuildImage(ctx, "lint", "nudanos/lint:trixie", …)` as `tester` is.
- Produces:
  - `distro-build lint [name…]` covers the in-scope ports (Global Constraints), or the named ones (a named out-of-scope port is an error).
  - It prints findings sorted by port, file and line, then `lint: <n> findings in <m> ports`, then any ratchet failures.
  - It exits non-zero on a ratchet failure or a tool or stale-log error.
  - `-update-baseline` lowers the named ports, or every port, via `Baseline.Update`.
  - `-init-baseline` writes `NewBaseline(counts)` and refuses if `lint-baseline.json` exists.
  - `func inScope(m *manifest.Manifest) []string`.
  - `func lintSpec(port, src, binDir string) engine.RunSpec`: `/src` read-only, binary at `/usr/local/bin/lint`, no network.

- [ ] **Step 1: Write the failing tests:**
  - `TestInScopeIsReadyDanos10` (fixture manifest with each kind, milestone and ready combination);
  - `TestLintSpecMounts` (read-only src, `--network none`);
  - `TestLintNamedOutOfScopeIsAnError`.
- [ ] **Step 2: Run** `go test ./cmd/distro-build/ -run 'TestInScope|TestLintSpec|TestLintNamed' -count=1`. Expected: FAIL.
- [ ] **Step 3: Implement** them. Ports run one container each, sequentially, and the output is collected per port.
- [ ] **Step 4: Run** the tests, then `go vet ./... && go test ./...`. Expected: all ok.
- [ ] **Step 5: Smoke run.** `distro-build -engine podman -work /Volumes/nudanos/work lint aaa`. Expected: findings or none, `lint: … findings in 1 ports`, and no tool errors.
- [ ] **Step 6: Commit** "distro-build lint".

### Task 8: First full run, baseline and nightly

**Files:**
- Create: `lint-baseline.json`
- Modify:
  - `.github/workflows/nightly.yml` (step "Lint (no port gets worse)" after "Build, sign, check the package set": `./distro-build -work "$RUNNER_TEMP/work" lint`, which appends the summary line to `$GITHUB_STEP_SUMMARY`);
  - `README.md` (a "## Lint" section: the command, `nudanos-lint.yaml`, the reason rule, `-update-baseline`);
  - `docs/superpowers/specs/2026-10-14-code-quality-design.md` (amend §2 and §3 with Task 5's and Task 6's rulings).

- [ ] **Step 1: Rebuild** what the new flags invalidate: `distro-build -engine podman build`. Expected: every in-scope C and Go port rebuilt, the rest cached. Record the count.
- [ ] **Step 2: Run** `distro-build -engine podman lint`. Expected: no tool or stale-log errors. Findings are expected.
- [ ] **Step 3: Write the baseline.** Run `distro-build -engine podman lint -init-baseline`. Commit `lint-baseline.json` with the per-tool totals in the message.
- [ ] **Step 4: Add the nightly step, README and spec amendment;** run `actionlint`. Expected: clean.
- [ ] **Step 5: Verify the ratchet bites.** Add one `echo $x` to a port's script with `-local`, run `distro-build lint <port>`, and expect a non-zero exit naming the port. Then revert the change.
- [ ] **Step 6: Commit and push** "lint: baseline and nightly step". Ledger the totals.

### Task 9: Fix the ports to zero (repeat per batch)

**Files:** in the port repos only (`/srv/nudanos/port-<name>`, branch `trixie`), plus `lint-baseline.json` in distro.

**Batches:** the order of `distro-build plan` restricted to in-scope ports, libraries first, cut into batches of 10. Write the batch lists into the ledger before starting batch 1.

For each port in the batch:
- [ ] **Step 1: Formatting commit.** Run gofmt or perltidy (the shared profile from `lint/perltidyrc`) over the port's files. Commit "Reformat with gofmt/perltidy (no other change)".
- [ ] **Step 2: Go ports get the staticcheck target.** Use Task 6's convention, in the same commit as its findings' fixes.
- [ ] **Step 3: Findings.** Run `distro-build -local <name>=/srv/nudanos/port-<name> lint <name>`, then fix each finding:
  - a behaviour change gets a failing test first, run from the package build;
  - pure style fixes need none;
  - a finding that cannot be fixed gets an inline suppression with `reason:`, and a ledger Ruling.

  Commit "Fix analyzer findings".
- [ ] **Step 4: Confirm zero.** `distro-build -local … lint <name>` prints `lint: 0 findings in 1 ports`.

For the batch:
- [ ] **Step 5: Build** with every batch port as `-local`. Expected: all built, package tests included.
- [ ] **Step 6: Full test run**, one at a time:
  1. `repo` (test key until the archive key is on the server)
  2. `image`
  3. `test install`
  4. `test boot`
  5. `test fixtures`
  6. `test scenarios`

  Expected: install-purge OK, boottest OK, fixtures OK (39), all scenarios PASS. Any new `accepted.diff` line stops the batch for the user (spec §6).
- [ ] **Step 7: Push** each port's `trixie` branch. In distro, run `lint -update-baseline <batch ports>` and commit "lint: batch N at zero".

### Task 10: Zero everywhere

**Files:** delete `lint-baseline.json` and the ratchet code (`Baseline`, `-update-baseline`, `-init-baseline`); modify `internal/lint`, `cmd/distro-build/main.go`, `README.md` and the spec's §9 roadmap row.

- [ ] **Step 1: Change the test.** `TestNoBaselineFileMeansZeroEverywhere` becomes `TestLintFailsOnAnyFinding`. Remove the baseline tests.
- [ ] **Step 2: Remove** the baseline code. Run `go vet ./... && go test ./...`. Expected: all ok.
- [ ] **Step 3: Run** `distro-build lint`. Expected: `lint: 0 findings in 95 ports`.
- [ ] **Step 4: Commit and push** "lint: zero findings everywhere; baseline removed". After the next nightly is green, ledger Phase A as complete.
