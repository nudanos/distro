# NuDanOS Plan 4: Checkpoint 1.0

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The nightly proves NuDanOS 1.0 routes (BGP, OSPF, VRRP), serves REST, SNMP and TACACS+, loads every DANOS 2105 reference config, and only then publishes. That closes checkpoint 1.0.

**Architecture:**
- `distro-build test scenario|scenarios|fixtures` start the tester container, which runs a new Go runner, `cmd/scenario`.
- The runner boots up to four QEMU routers from copy-on-write overlays of one installed disk, linked by QEMU socket networks, with a management NIC for SSH, HTTPS and SNMP.
- It drives them over the serial console with the layer 3 code (moved to `internal/boottest`) and judges scenario files in `tests/scenarios/`.
- The same runner boots the DANOS 2105 ISO (`-reference-iso`) to capture reference configs and `show` output into `tests/reference/2105/`.
- `test fixtures` loads every capture on NuDanOS. A boot-time hook sets DPDK-only settings aside instead of failing the boot commit.

**Tech Stack:** Go 1.26 (`github.com/nudanos/distro`; new deps `github.com/nwaples/tacplus` BSD-2-Clause and `golang.org/x/crypto/ssh` BSD-3-Clause), QEMU in `nudanos/tester:trixie` (+ `snmp`, `openssh-client`), Docker, bash and Perl in the port repos, GitHub Actions.

**Spec:** `docs/superpowers/specs/2026-10-05-plan4-checkpoint-1.0-design.md` (all four sections approved), an addendum to `docs/superpowers/specs/2026-09-28-debian13-revival-design.md`. Plan 3's spec and ledger (`danOS Project/plan3-ledger.md`, `plan3-rulings.md`) hold the decisions this builds on.

**Scope:**
- **In:** spec addendum §2–§7.
- **Plan 5:** the kernel vendor-commit audit and the deferred tooling minors.

## Global Constraints

- GitHub org `nudanos`. Pushing `trixie` branches of existing `nudanos/*` port repos and committing to `main` of `nudanos/distro` are approved. Anything else outward-facing (new repos, releases by hand) needs the user's go-ahead.
- Debian packaging on `trixie`:
  - `debhelper-compat (= 13)`, `Standards-Version: 4.7.2`, `Rules-Requires-Root: no`
  - `Maintainer: NuDanOS Maintainers <jon@fernandez.tech>`
  - port versions bump their last component; package builds run their tests, and `lintian --fail-on error --profile vyatta` gates every build
- Commits use `git commit -s` with identity `NuDanOS <jon@fernandez.tech>` (`-c user.name=NuDanOS -c user.email=jon@fernandez.tech`) and end with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- The agent never types or handles secrets. Test credentials (router users, SNMP v3 keys, the TACACS+ secret) are fixed test values committed in scenario files; they are not secrets.
- **The 2105 ISO is never committed or uploaded.** It lives at `~/Downloads/danos-2105-base-amd64.iso`, SHA-256 `6d500d5d7ea69ebca0b7ada2bd74cec40c87780f41cefd9b9cc0e14fb81d9b51`. The runner refuses an ISO with any other hash. Captures from it are committed.
- 2105 live login `tmpuser`/`tmppwd`. NuDanOS installed routers: admin `nudanos`, password `NuDanOS-test-1` (the layer 3 values).
- Router memory: NuDanOS 1024 MB, 2105 1536 MB; 2 vCPUs each. At most 4 routers per topology.
- **Links are point-to-point**, exactly two endpoints (QEMU `-netdev socket` listen/connect). Data NICs take PCI addresses from `03` in link order, so a router's first link is `dp0s3`, then `dp0s4`, `dp0s5`. The management NIC is `addr=0a` (`dp0s10`).
- Management: `10.0.2.15/24` on `dp0s10`, reached on-link from the runner at `10.0.2.2`. No default route (it would send scenario traffic into QEMU user networking). This refines the spec's "gateway `10.0.2.2`" (Task 6 commits the spec wording).
- Timeouts are written for KVM and multiplied by 6 when `/dev/kvm` is absent (as `cmd/boottest`).
- Tools in the tester image come from Debian 13 only.
- The work directory is case-sensitive (`/Volumes/nudanos/work` on the Mac). Run multi-repo loops under `bash` (zsh does not word-split).
- Port pushes: `printf 'repo\n' > /Volumes/nudanos/wave-X.txt && bash /Volumes/nudanos/wave-push.sh /Volumes/nudanos/wave-X.txt`.

## Review Focus

1. **A router that never reaches login** (kernel panic, a hung boot, QEMU exiting at start): the scenario fails at its timeout naming the router and showing its last console lines; the other routers are killed; the next scenario still runs. Test: `TestStartFailsWhenQEMUExits` and `TestRunnerContinuesAfterFailedScenario` in Tasks 3 and 8.
2. **Four emulated routers at once on the Mac** (CPU contention on top of TCG): boots and checks must still finish inside their scaled timeouts. Test: the `bgp` scenario run on the Mac in Task 12, recorded with its wall-clock time.
3. **Busy or leftover ports** (a previous run's QEMU still holding a forwarded port): the runner picks free ports at start and fails clearly if a port is taken mid-run. Test: `TestAllocatePortsSkipsBusyPort` in Task 3.
4. **Odd config files at boot:** quoted values containing braces, comments, a DPDK-only leaf nested deep under an interface, a file without a version footer, a second boot (must be a no-op, original kept once). Test: the set-aside cases in Task 14.
5. **A `show` filter that hides real changes:** masking uptimes and counters must not mask prefixes, AS numbers, next hops or states. Test: `TestNormalizeKeepsRoutingFacts` in Task 7.

---

## File Structure

| File | Responsibility |
|---|---|
| `internal/boottest/steps.go` | Moved from `cmd/boottest`: prompts, `Login`, `LoginRefused`, `Configure`, `Commit`, `Op`, `InstallImage`, `Halt` |
| `internal/topology/topology.go` | `VMSpec`, `Topology`, link and port planning, QEMU argument building |
| `internal/topology/vm.go` | Start a VM, connect its console, stop it; overlays |
| `internal/scenario/file.go` | Scenario file types, parsing, validation, interface placeholders |
| `internal/scenario/checks.go` | `op`, `action`, `http`, `snmp`, `login` checks |
| `internal/scenario/show.go` | `show` capture, normalisation, diff, `accepted.diff` |
| `internal/scenario/image.go` | Image profiles: NuDanOS (installed base disk) and 2105 (live ISO) |
| `internal/scenario/run.go` | Run one scenario: boot, base config, scenario config, checks, show, teardown |
| `internal/scenario/gochecks.go` | Named Go checks registered with `RegisterGo` (e.g. `snmp-counter-moves`) |
| `internal/tacacs/server.go` | Minimal TACACS+ test server |
| `internal/fixtures/fixtures.go` | Load check of every 2105 capture; the reboot test |
| `cmd/scenario/main.go` | Runner entry point inside the tester container |
| `cmd/distro-build/main.go` | + `test scenario <name>`, `test scenarios`, `test fixtures`; flags `-reference-iso`, `-capture` |
| `tester/Dockerfile` | + `snmp`, `openssh-client` |
| `tests/scenarios/{bgp,ospf,vrrp,rest,snmp,tacacs,mpls-ldp}/scenario.yaml` | Scenario definitions |
| `tests/reference/2105/README.md` | ISO hash, how to recapture |
| `tests/reference/2105/<scenario>/<router>/…` | Captured `config.boot`, `commands.txt`, `show/*.txt`; `accepted.diff` per scenario |
| `tests/reference/2105/sampler/<feature>.set` (+ captures) | Feature sampler inputs and 2105 captures |
| `.github/workflows/nightly.yml` | + fixtures and scenarios; release gating |
| port `vyatta-protocols-frr`: `scripts/frr/configs/steps.json`, `tests/fpm-consistent.sh`, `debian/rules` | FRR fpm fix and its check |
| port `vyatta-cfg`: `scripts/vyatta-boot-config-loader`, `tests/boot-config-hooks.sh` | Pre-load hook directory |
| port `vyatta-kernel-forwarding`: `boot-config.d/50-dpdk-set-aside`, `update-motd.d/60-dpdk-set-aside`, `dpdk-only-paths`, `tests/set-aside.t`, `tests/set-aside-consistency.t` | Set-aside hook, its path list and tests |

---

### Task 1: FRR: stop writing fpm lines zebra no longer understands

**Files:**
- Modify: port `vyatta-protocols-frr` `scripts/frr/configs/steps.json`, `debian/rules`, `debian/changelog`
- Create: port `vyatta-protocols-frr` `tests/fpm-consistent.sh`

**Interfaces:**
- Produces: `frr.conf` without `fpm` lines; a build-time check that fails if `steps.json` names `fpm` while `etc/frr/daemons.danos` does not load `dplane_fpm_nl`.

- [ ] **Step 1: Write the failing check** `tests/fpm-consistent.sh` (POSIX sh, SPDX `GPL-2.0-only`): exit 1 with `fpm-consistent: steps.json configures fpm but daemons.danos does not load dplane_fpm_nl` when `grep -q fpm scripts/frr/configs/steps.json` and `! grep -q dplane_fpm_nl etc/frr/daemons.danos`; else print `fpm-consistent: OK`.
- [ ] **Step 2: Run it**
  Run: `cd /Volumes/nudanos/port-vyatta-protocols-frr && sh tests/fpm-consistent.sh; echo rc=$?`
  Expected: the error message, `rc=1`.
- [ ] **Step 3: Remove** `"fpm address 127.0.0.1"` and `"no fpm use-next-hop-groups"` from `steps.json` (keep valid JSON), and wire the check into the build: `override_dh_auto_build: vet fpm-check` with target `fpm-check:` running `sh tests/fpm-consistent.sh`.
- [ ] **Step 4: Run it again**
  Run: `sh tests/fpm-consistent.sh && python3 -m json.tool scripts/frr/configs/steps.json >/dev/null && echo json-ok`
  Expected: `fpm-consistent: OK` and `json-ok`.
- [ ] **Step 5: Changelog, build, commit, push.** Changelog entry: "Stop configuring zebra's FPM, whose module kernel forwarding no longer loads; frr-reload failed on every routing commit". Build with `./distro-build -work /Volumes/nudanos/work -local vyatta-protocols-frr=../port-vyatta-protocols-frr build vyatta-protocols-frr` (expect `built`), commit in the port, push (wave procedure), and wait for its CI to pass.

---

### Task 2: Move the console steps into `internal/boottest`

**Files:**
- Create: `internal/boottest/steps.go`, `internal/boottest/steps_test.go`
- Modify: `cmd/boottest/steps.go`, `cmd/boottest/steps_test.go`, `cmd/boottest/main.go`

**Interfaces:**
- Consumes: `boottest.Console` (`Expect`, `First`, `FirstNudging`, `Send`, `Discard`, `Tail`).
- Produces (package `boottest`):
  - `var LoginPrompt, PassPrompt, OpPrompt, CfgPrompt, CfgDone *regexp.Regexp` (today's `loginPrompt` … `cfgDone`)
  - `func Login(c *Console, user, password string, prompt *regexp.Regexp, total time.Duration) error`
  - `func LoginRefused(c *Console, user, password string, prompt *regexp.Regexp, total time.Duration) error`
  - `func Configure(c *Console, cmd string, timeout time.Duration) error`
  - `func Commit(c *Console, want *regexp.Regexp, pause, timeout time.Duration) error`
  - `func Op(c *Console, cmd string, want *regexp.Regexp, timeout time.Duration) error`
  - `func OpOutput(c *Console, cmd string, timeout time.Duration) (string, error)`: runs `cmd`, returns everything printed before the next `OpPrompt`, with escape sequences and `\r` removed
  - `func InstallImage(c *Console, admin, password string, t func(time.Duration) time.Duration) error` (today's `installSteps`, admin name a parameter)
  - `func Halt(c *Console, t func(time.Duration) time.Duration) error`

- [ ] **Step 1: Move the tests first.** Move the replay tests for login, configure, commit, op, the installer and the console regexes from `cmd/boottest/steps_test.go` to `internal/boottest/steps_test.go`, renamed to the exported names. Add `TestOpOutputStripsEscapesAndPrompt`: the console replays `"show version\r\n\x1b[?2004l\r\rVersion:      1.0-20261004.0334\x1b[m\r\n\x1b[?2004hvyatta@node:~$ "`; `OpOutput` returns exactly `"Version:      1.0-20261004.0334\n"`.
- [ ] **Step 2: Run them**
  Run: `go test ./internal/boottest/`
  Expected: build failure, undefined `Login`, `OpOutput`, …
- [ ] **Step 3: Move the implementations** into `internal/boottest/steps.go` with the signatures above. `cmd/boottest` keeps `liveSteps`, `diskSteps` and `main`, calling the exported functions; `InstallImage` replaces `installSteps(c, t)` as `boottest.InstallImage(c, adminUser, testPassword, t)`.
- [ ] **Step 4: Run all Go tests**
  Run: `go vet ./... && go test ./...`
  Expected: every package `ok`, including `TestOpOutputStripsEscapesAndPrompt` and `cmd/boottest`'s remaining tests.
- [ ] **Step 5: Commit** (`git add internal/boottest cmd/boottest`), message "boottest: export the console steps for the scenario runner".

---

### Task 3: `internal/topology`: plan and start VMs

**Files:**
- Create: `internal/topology/topology.go`, `internal/topology/vm.go`, `internal/topology/topology_test.go`, `internal/topology/vm_test.go`

**Interfaces:**
- Consumes: `boottest.NewConsole`.
- Produces:
  - `type Link struct{ A, B string }`
  - `type VMSpec struct { Name string; MemMB int; Disk string; ISO string; KVM bool; Data []DataNIC; Mgmt Ports; SerialSock string }`
  - `type DataNIC struct { PCI int; Peer string; Listen bool; Port int }` (PCI 3, 4, 5…; `Listen` true on link side A)
  - `type Ports struct { SSH, HTTPS, SNMP int }` (host ports forwarded to 10.0.2.15:22/443 tcp and 161 udp)
  - `func Plan(routers []string, links []Link, memMB int, kvm bool, alloc func(n int) ([]int, error)) ([]VMSpec, error)`
  - `func AllocatePorts(n int) ([]int, error)`: n distinct free 127.0.0.1 ports (listen-and-close probe)
  - `func (v VMSpec) QEMUArgs() []string`
  - `func Interface(specs []VMSpec, router, peer string) (string, error)`: router's interface name toward peer (`dp0s3`…)
  - `type VM struct { Spec VMSpec; Console *boottest.Console }` with `func Start(spec VMSpec, log io.Writer) (*VM, error)` and `func (vm *VM) Stop(grace time.Duration)`
  - `func Overlay(base, path string) error` (`qemu-img create -f qcow2 -b base -F qcow2 path`)

- [ ] **Step 1: Write the planning tests** in `topology_test.go`:
  - `TestPlanAssignsInterfacesInLinkOrder`: routers `R1 R2 R3`, links `R1-R2, R2-R3`; `Interface(specs,"R2","R1")=="dp0s3"`, `Interface(specs,"R2","R3")=="dp0s4"`, `Interface(specs,"R3","R2")=="dp0s3"`.
  - `TestPlanRejects`: more than 4 routers; a link naming an unknown router; a link from a router to itself; a duplicate link. Each returns an error naming the offender.
  - `TestQEMUArgs`: contains `-m 1024`, `-smp 2`, `virtio-net-pci,netdev=d3,addr=03`, `socket,id=d3,listen=127.0.0.1:<port>` on side A and `connect=` on side B, `virtio-net-pci,netdev=m,addr=0a`, `user,id=m,hostfwd=tcp:127.0.0.1:<ssh>-10.0.2.15:22,hostfwd=tcp:127.0.0.1:<https>-10.0.2.15:443,hostfwd=udp:127.0.0.1:<snmp>-10.0.2.15:161`, `-serial unix:<sock>,server=on,wait=on`, `-enable-kvm -cpu host` only when `KVM`, `-cpu max` otherwise, `-cdrom <iso> -boot d` only when `ISO` is set.
  - `TestAllocatePortsSkipsBusyPort`: hold one listener, allocate 20 ports, none equals the held port, all distinct.
- [ ] **Step 2: Run** `go test ./internal/topology/`. Expected: build failure (undefined identifiers).
- [ ] **Step 3: Implement** `topology.go` with the interfaces above. One link uses one port: side A listens, side B connects.
- [ ] **Step 4: Write the VM tests** in `vm_test.go`, using a fake `qemu-system-x86_64` shell script on `PATH`:
  - `TestStartConnectsConsole`: the fake script only sleeps; the test itself listens on `SerialSock` (a unix socket) and writes `"node login: "` when `Start` connects; the returned VM's `Console.Expect(boottest.LoginPrompt)` succeeds.
  - `TestStartFailsWhenQEMUExits`: the fake exits 1 immediately; `Start` returns an error containing `qemu exited` within 5 s and leaves no process behind.
- [ ] **Step 5: Implement** `vm.go` (`Start` follows `cmd/boottest`'s `boot`: start QEMU, dial the socket with retries, fail fast if the process exits first; `Stop(grace)` waits then kills).
- [ ] **Step 6: Run** `go vet ./internal/topology/ && go test ./internal/topology/`. Expected: `ok`.
- [ ] **Step 7: Commit**, message "topology: plan and start multi-VM QEMU topologies".

---

### Task 4: TACACS+ test server

**Files:**
- Create: `internal/tacacs/server.go`, `internal/tacacs/server_test.go`
- Modify: `go.mod`, `go.sum`

**Interfaces:**
- Produces: `type User struct{ Name, Password string; Priv int }`; `func Serve(l net.Listener, secret string, users []User) error` (blocks; authentication by PAP or ASCII login, authorization returns `priv-lvl=<Priv>` and pass-add for known users, fail otherwise; accounting replies success).

- [ ] **Step 1: Verify the library before depending on it.** `go get github.com/nwaples/tacplus@latest`, then read its `LICENSE` (expect BSD-2-Clause) and `server.go`: confirm `RequestHandler`, `ServerConnHandler{Handler, ConnConfig}`, `ConnConfig.Secret`, `Server{ServeConn}.Serve(l)`. Record the version in the ledger. If any differs, stop and rule (fallback: `tac_plus` from source in the tester image).
- [ ] **Step 2: Write the tests** with the library's own client: `TestServeAuthenticatesKnownUser` (user `tacadmin`/`tac-admin-1`, priv 15: PAP authen passes; author response contains `priv-lvl=15`), `TestServeRefusesWrongPassword`, `TestServeRefusesUnknownUser`.
- [ ] **Step 3: Run** `go test ./internal/tacacs/`. Expected: build failure.
- [ ] **Step 4: Implement** `server.go`.
- [ ] **Step 5: Run** `go test ./internal/tacacs/`. Expected: `ok`.
- [ ] **Step 6: Commit** (`go.mod go.sum internal/tacacs`), message "tacacs: minimal TACACS+ test server".

---

### Task 5: Scenario files: types, parsing, validation, placeholders

**Files:**
- Create: `internal/scenario/file.go`, `internal/scenario/file_test.go`

**Interfaces:**
- Consumes: `topology.Link`.
- Produces:
  - `type File struct { Name string; Gating bool; Routers map[string]Router; RouterOrder []string; Links []topology.Link; Show []string; Checks []Check; TACACS *TACACSServer }`
  - `type Router struct { Config []string }`
  - `type Check struct { Name, Router string; Timeout time.Duration; Op *OpCheck; Action *Action; HTTP *HTTPCheck; SNMP *SNMPCheck; Login *LoginCheck; Go string }` (exactly one kind set)
  - `type OpCheck struct { Command, Want string; Absent bool }` (`Absent`: the pattern must not match)
  - `type Action struct { Configure []string; Commit bool }`
  - `type HTTPCheck struct { Method, Path, Body string; Status int; Want string; Auth bool; Steps []HTTPCheck }`
  - `type SNMPCheck struct { Version, Community, User, AuthKey, PrivKey, OID, Want string }`
  - `type LoginCheck struct { User, Password string; Expect string; Want, NotWant string }` (`Expect`: `ok` or `refused`; `Want` / `NotWant`: patterns the output of `id` must / must not match after login)
  - `type TACACSServer struct { Secret string; Users []tacacs.User }`
  - `func Load(path string) (*File, error)`; `func (f *File) Expand(specs []topology.VMSpec) error` replaces `${R1:R2}` in config lines and check commands with R1's interface toward R2.
- YAML shape (`go.yaml.in/yaml/v3`): `routers` is an ordered mapping (keep file order in `RouterOrder`); `links` a list of two-element lists; check timeouts as Go durations (`2m`).

- [ ] **Step 1: Write the tests:** `TestLoadOrdersRoutersAndLinks`, `TestLoadRejectsCheckWithTwoKinds`, `TestLoadRejectsCheckWithNoKind`, `TestLoadRejectsUnknownRouterInCheck`, `TestLoadDefaultsTimeoutTo2m`, `TestExpandReplacesInterfacePlaceholders` (`set interfaces dataplane ${R2:R1} address 10.0.12.2/24` on R2 becomes `… dp0s3 …`), `TestExpandRejectsPlaceholderWithoutLink`.
- [ ] **Step 2: Run** `go test ./internal/scenario/`. Expected: build failure.
- [ ] **Step 3: Implement** `file.go`.
- [ ] **Step 4: Run** `go test ./internal/scenario/`. Expected: `ok`.
- [ ] **Step 5: Commit**, message "scenario: scenario file format".

---

### Task 6: Image profiles and the installed base disk

**Files:**
- Create: `internal/scenario/image.go`, `internal/scenario/image_test.go`
- Modify: `docs/superpowers/specs/2026-10-05-plan4-checkpoint-1.0-design.md` (management wording)

**Interfaces:**
- Consumes: `boottest.InstallImage`, `boottest.Login`, `boottest.Halt`, `topology.Start`, `topology.Overlay`.
- Produces:
  - `type Image struct { Name string; ISO string; MemMB int; User, Password string; Live bool }`
  - `func NuDanOS(iso string) Image` (1024 MB, `nudanos`/`NuDanOS-test-1`, `Live: false`)
  - `func Reference2105(iso string) (Image, error)` (1536 MB, `tmpuser`/`tmppwd`, `Live: true`; error unless the file's SHA-256 is `6d500d5d7ea69ebca0b7ada2bd74cec40c87780f41cefd9b9cc0e14fb81d9b51`)
  - `func EnsureBase(img Image, workDir string, kvm bool, log io.Writer, t func(time.Duration) time.Duration) (string, error)`: for NuDanOS, returns `workDir/base-<first 12 hex of the ISO's SHA-256>.qcow2`, installing to it first if absent (live login `vyatta`/`vyatta`, `InstallImage(c, "nudanos", "NuDanOS-test-1", t)`, `Halt`); for 2105 returns `""` (live).
  - `func BaseConfig(router string) []string`: `set system host-name <router lowercase>`, `set interfaces dataplane dp0s10 address 10.0.2.15/24`, `set service ssh`.

- [ ] **Step 1: Write the tests:** `TestReference2105RefusesWrongHash` (a temp file), `TestBaseConfig` (exact three lines for `R1`, host name `r1`), `TestEnsureBaseReusesExistingDisk` (an existing `base-<hash>.qcow2` is returned without starting QEMU; `PATH` holds a fake `qemu-system-x86_64` that fails the test if run).
- [ ] **Step 2: Run** `go test ./internal/scenario/ -run 'Reference2105|BaseConfig|EnsureBase'`. Expected: build failure.
- [ ] **Step 3: Implement** `image.go`.
- [ ] **Step 4: Run** the same. Expected: `ok`.
- [ ] **Step 5: Spec wording.** In the spec's §2 management bullet, replace "with gateway `10.0.2.2` (the runner's side, where the TACACS+ server listens)" with "reached on-link from `10.0.2.2` (the runner's side, where the TACACS+ server listens); no default route, which would send scenario traffic into QEMU user networking". Ledger the ruling.
- [ ] **Step 6: Commit** (`internal/scenario/image*.go`, the spec), message "scenario: image profiles and the installed base disk".

---

### Task 7: `show` capture, normalisation, diff

**Files:**
- Create: `internal/scenario/show.go`, `internal/scenario/show_test.go`

**Interfaces:**
- Produces:
  - `func Normalize(command, output string) string`: replaces volatile fields with placeholders: uptimes and ages (`\d+d\d+h\d+m`, `\d+w\dd\dh`, `\d\d:\d\d:\d\d`) → `<time>`; packet and byte counters in `show interfaces` / `show dataplane` columns → `<n>`; BGP message counters and table versions in `show ip bgp summary` → `<n>`; OSPF dead-timer column → `<time>`; trailing spaces stripped.
  - `func Diff(want, got string) string`: unified-format line diff (`--- 2105`, `+++ nudanos`), `""` when equal.
  - `func CheckAccepted(diff, accepted string) error`: nil when `diff == accepted` (both may be empty); otherwise an error whose text is the unaccepted diff.

- [ ] **Step 1: Write the tests:** `TestNormalizeMasksUptime` (a `show ip bgp summary` line with `00:05:12` and `1d02h03m`), `TestNormalizeKeepsRoutingFacts` (after normalising a `show ip route` and a `show ip bgp summary` sample, the strings `10.0.12.0/24`, `via 10.0.12.1`, `65001`, `Established`, `Full/DR` are all still present), `TestDiffEmptyWhenEqual`, `TestDiffShowsChangedLine`, `TestCheckAcceptedPassesExactMatch`, `TestCheckAcceptedFailsOnNewDifference`.
- [ ] **Step 2: Run** `go test ./internal/scenario/ -run 'Normalize|Diff|Accepted'`. Expected: build failure.
- [ ] **Step 3: Implement** `show.go`.
- [ ] **Step 4: Run** the same. Expected: `ok`.
- [ ] **Step 5: Commit**, message "scenario: show normalisation and diff".

---

### Task 8: Checks, the runner and `distro-build test scenario`

**Files:**
- Create: `internal/scenario/checks.go`, `internal/scenario/checks_test.go`, `internal/scenario/run.go`, `internal/scenario/run_test.go`, `cmd/scenario/main.go`
- Modify: `cmd/distro-build/main.go`, `tester/Dockerfile`, `go.mod`, `go.sum`

**Interfaces:**
- Consumes: Tasks 2–7.
- Produces:
  - `func RunCheck(ctx context.Context, c Check, r *Routers) error`, with `type Routers struct { VMs map[string]*topology.VM; Ports map[string]topology.Ports; Admin, Password string; T func(time.Duration) time.Duration }`
    - `op`: repeat `OpOutput` every 10 s until `Want` matches (or, with `Absent`, stops matching) or the timeout passes; the error shows the last output
    - `action`: `Configure` each line, then `Commit(nil)` when `Commit`
    - `http`: HTTPS to `127.0.0.1:<Ports.HTTPS>` with basic auth (admin) when `Auth`, TLS verification off (self-signed); `Steps` run in order, and a step's `Location` response header is substituted for `{location}` in the next step's `Path` (the REST conf-session flow)
    - `snmp`: exec `snmpwalk -v2c -c <community>` or `-v3 -l authPriv -u … -a SHA -A … -x AES -X …` against `127.0.0.1:<Ports.SNMP>`; output must match `Want`
    - `login`: SSH (`golang.org/x/crypto/ssh`, password auth, host key ignored) to `127.0.0.1:<Ports.SSH>`; `ok` must log in, and the remote command `id`'s output must match `Want` and must not match `NotWant` when they are set; `refused` must fail authentication
    - `go`: calls the function registered under `Check.Go`; an unregistered name is an error
  - `func RegisterGo(name string, f func(ctx context.Context, r *Routers) error)` (scenario tasks register their named checks in `internal/scenario/gochecks.go`)
  - `type Options struct { Scenario string; ISO string; Reference bool; Capture bool; KVM bool; Work string; Tests string }`
  - `func Run(ctx context.Context, o Options) (Result, error)` with `type Result struct { Name string; Gating, Passed bool; Failed []string; Transcripts string }`. The run: load `tests/scenarios/<name>/scenario.yaml` → image → base disk → plan → overlays → start all VMs → `Login` all → base config + scenario config (`Configure` each, one `Commit` per router) → start the TACACS+ server on `127.0.0.1:49` when declared → checks in file order (a failed check is recorded, later checks still run) → `show` capture per router → with `Capture`: write `tests/reference/2105/<name>/<router>/{config.boot,commands.txt,show/<cmd>.txt}`; otherwise compare each normalised `show` against the reference and `accepted.diff` → `Halt` and `Stop` every VM, always.
  - `cmd/scenario`: flags `-scenario`, `-all`, `-iso`, `-reference`, `-capture`, `-kvm`, `-work /work`, `-tests /tests`; prints one line per scenario (`PASS|FAIL <name> (<gating|reported>) <minutes>`) and exits 1 if a gating scenario failed.
  - `distro-build test scenario <name>` / `test scenarios`: build `cmd/scenario` like `testBoot` builds `boottest`, mount the ISO dir at `/iso`, `tests/` at `/tests` (read-write only with `-capture`), `work/scenarios` at `/work`, pass `/dev/kvm` when present. Flags `-reference-iso <path>` (run against 2105) and `-capture` (requires `-reference-iso`).
  - `tester/Dockerfile`: add `snmp openssh-client`.
  - **2105 capture details:** `config.boot` via `save /tmp/capture.boot` in configuration mode then `cat /tmp/capture.boot` (live ISO has no `/config/config.boot`); `commands.txt` via `show configuration commands`.

- [ ] **Step 1: Write the check tests** in `checks_test.go`:
  - `TestHTTPCheckFollowsLocation` (an `httptest.NewTLSServer`: `POST /rest/conf` → 201 with `Location: /rest/conf/ABC`; the next step `PUT {location}/set/system/host-name/r1` reaches `/rest/conf/ABC/set/system/host-name/r1`)
  - `TestHTTPCheckRequiresAuthWhenAsked`
  - `TestSNMPCheckRunsSnmpwalk` (fake `snmpwalk` on `PATH` prints `IF-MIB::ifDescr.3 = STRING: dp0s3`; it records its args; the check passes for `Want: dp0s3` and the args include `-v3 -l authPriv`)
  - `TestLoginCheckOKAndRefused` (an in-process `x/crypto/ssh` server accepting `nudanos`/`NuDanOS-test-1`, running `id` as `uid=1000(nudanos) groups=…vyattaadm…`)
  - `TestOpCheckRetriesUntilMatch` (console replay: first output without `Established`, second with it)
- [ ] **Step 2: Run** `go test ./internal/scenario/ -run Check`. Expected: build failure.
- [ ] **Step 3: Implement** `checks.go` (`go get golang.org/x/crypto/ssh`).
- [ ] **Step 4: Run** the same. Expected: `ok`.
- [ ] **Step 5: Write the runner tests** in `run_test.go` with a fake QEMU that serves a scripted console per VM (login, configure, commit, `show` outputs): `TestRunPassesAndCapturesShow`, `TestRunRecordsFailedCheckAndContinues` (two checks, the first fails; both are attempted; `Result.Failed` names the first), `TestRunStopsAllVMsOnError` (one VM's QEMU exits at start; `Run` returns an error naming that router; no fake QEMU left running), `TestRunnerContinuesAfterFailedScenario` (`cmd/scenario -all` over two scenario files, the first failing, still runs the second).
- [ ] **Step 6: Run** `go test ./internal/scenario/ ./cmd/scenario/`. Expected: build failure, then implement `run.go` and `cmd/scenario/main.go`, then `ok`.
- [ ] **Step 7: Wire `distro-build`** (`test scenario|scenarios`, the two flags, usage text). Add `TestScenarioSpecMounts` in `cmd/distro-build` asserting the run spec's mounts (`/iso` ro, `/tests` ro without `-capture` and rw with it, `/work`) and that `-capture` without `-reference-iso` is an error.
- [ ] **Step 8: Run** `go vet ./... && go test ./...`. Expected: all `ok`.
- [ ] **Step 9: Commit**, message "scenario: checks, runner and distro-build test scenario".

---

### Task 9: Single-router scenarios: `rest`, `snmp`, `tacacs`

**Files:**
- Create: `tests/scenarios/rest/scenario.yaml`, `tests/scenarios/snmp/scenario.yaml`, `tests/scenarios/tacacs/scenario.yaml`, each with a `README.md` naming its source (Robot suite or new); `tests/reference/2105/README.md`; captures under `tests/reference/2105/{rest,snmp,tacacs}/`

**Interfaces:**
- Consumes: Task 8's runner and file format.
- Produces: three gating scenarios and their 2105 references.

Scenario content (config as `set` lines; values are test values):
- **`rest`** (from `danos_restapi.robot`): R1 config `set service https`. Checks: `http` `POST /rest/op/show/version` then `GET {location}` 200 matching `Version:`; the conf-session flow `POST /rest/conf` → `PUT {location}/set/system/host-name/rest-test` → `POST {location}/commit` → `DELETE {location}`, then `op` `show host name` wants `rest-test`; `http` `GET /rest/op` without auth wants 401.
- **`snmp`**: R1 config `set service snmp community nudanos-ro authorization ro`, `set service snmp v3 user snmpv3 auth type sha`, `… auth plaintext-key snmpv3-auth-1`, `… privacy type aes`, `… privacy plaintext-key snmpv3-priv-1`, `… mode ro`, `set interfaces dataplane dp0s10 description mgmt`. Checks: v2c walk of `SNMPv2-MIB::sysName` wants `r1`; v3 walk of `IF-MIB::ifDescr` wants `dp0s10`; `go: snmp-counter-moves` reads `IF-MIB::ifInOctets` for `dp0s10` over v2c, makes SSH traffic to the router, reads again and requires the counter to have grown. It lives in `internal/scenario/gochecks.go`, tested with the fake `snmpwalk` returning two increasing values.
- **`tacacs`**: TACACS+ server secret `nudanos-tac-1`, users `tacadmin`/`tac-admin-1` priv 15 and `tacop`/`tac-op-1` priv 1. R1 config `set system login tacplus-server 10.0.2.2 secret nudanos-tac-1`. Checks: `login` `tacadmin` ok, `Want: vyattaadm`; `login` `tacop` ok, `NotWant: vyattaadm`; `login` `tacadmin` with a wrong password refused; `action` delete the tacplus-server and add an unreachable one (`10.0.2.99`), then `login` `nudanos`/`NuDanOS-test-1` ok (local fallback).

The exact config syntax and the privilege mapping are proven on 2105 first. A line that 2105 rejects is corrected in the scenario file and ledgered.

- [ ] **Step 1: Write the three scenario files and READMEs**, and `tests/reference/2105/README.md` (the ISO's file name and SHA-256, "never commit the ISO", and the recapture command `./distro-build -work /Volumes/nudanos/work -reference-iso ~/Downloads/danos-2105-base-amd64.iso -capture test scenario <name>`).
- [ ] **Step 2: Run each on 2105 and capture**
  Run: `./distro-build -work /Volumes/nudanos/work -reference-iso ~/Downloads/danos-2105-base-amd64.iso -capture test scenario rest` (then `snmp`, `tacacs`)
  Expected: `PASS rest (gating)` etc., and files under `tests/reference/2105/<name>/R1/`. A failing check on 2105 means the check or the syntax is wrong: fix the file (ledger it) and rerun until 2105 passes.
- [ ] **Step 3: Run each on NuDanOS**
  Run: `./distro-build -work /Volumes/nudanos/work test scenario rest` (then `snmp`, `tacacs`)
  Expected: initially, failures that are NuDanOS bugs or `show` differences.
- [ ] **Step 4: Resolve every failure.** A NuDanOS bug: write a failing test in the owning port, fix it, rebuild the port, rebuild the image (`./distro-build -work /Volumes/nudanos/work repo` with `-key`, then `image`), rerun. A `show` difference that is expected (formatting): add it to `tests/reference/2105/<name>/accepted.diff` and ledger each hunk with its reason. Repeat until all three pass on NuDanOS.
- [ ] **Step 5: Commit** scenarios, references and accepted diffs (`git add tests/scenarios tests/reference`), message "scenarios: rest, snmp, tacacs with 2105 references".

---

### Task 10: `ospf` and `vrrp`

**Files:**
- Create: `tests/scenarios/ospf/scenario.yaml`, `tests/scenarios/vrrp/scenario.yaml` (+ READMEs), captures under `tests/reference/2105/{ospf,vrrp}/`

**Interfaces:**
- Consumes: Task 8; Task 1's FRR fix.
- Produces: two gating scenarios and their references.

Scenario content:
- **`ospf`**: routers R1 R2 R3; links R1–R2 (10.0.12.0/24), R2–R3 (10.0.23.0/24); loopbacks `lo` 10.255.0.N/32. R1 and R2's R1 side in area 0; R2's R3 side and R3 in area 1 (`set protocols ospf area 0 network 10.0.12.0/24`, `… area 1 network 10.0.23.0/24`, loopbacks in their router's area). Show: `show ip ospf neighbor`, `show ip route ospf`. Checks: `op` R1 `show ip ospf neighbor` wants `Full`, same on R3; `op` R1 `show ip route 10.255.0.3` wants `via 10.0.12.2`; `op` R1 `ping 10.255.0.3 count 3 interface 10.255.0.1` wants ` 0% packet loss`; `action` on R1 `set interfaces dataplane ${R1:R2} ip ospf cost 100` + commit, then `op` R1 `show ip route 10.255.0.3` wants `\[110/1[0-9][0-9]\]` (the metric now includes the cost of 100).
- **`vrrp`**: routers R1 R2; link R1–R2 (10.0.0.0/24; R1 .1, R2 .2). Group 10, virtual address 10.0.0.254/24; R1 priority 200, R2 priority 100, preempt on both (`set interfaces dataplane ${R1:R2} vrrp vrrp-group 10 virtual-address 10.0.0.254/24`, `… priority 200`, `… preempt true`). Show: `show vrrp`. Checks: `op` R1 `show vrrp` wants `MASTER`, R2 wants `BACKUP`; `action` R1 `set interfaces dataplane ${R1:R2} disable` + commit, then R2 wants `MASTER`; `action` R1 `delete interfaces dataplane ${R1:R2} disable` + commit, then R1 wants `MASTER` again (preemption).

- [ ] **Step 1: Write both scenario files and READMEs.**
- [ ] **Step 2: Run on 2105 with `-capture`** (as Task 9 Step 2). Expected: `PASS ospf (gating)`, `PASS vrrp (gating)`; fix and ledger any check 2105 fails.
- [ ] **Step 3: Run on NuDanOS** (as Task 9 Step 3).
- [ ] **Step 4: Resolve every failure** (as Task 9 Step 4). The fpm fix from Task 1 must already be in the image: confirm `frr-reload.py failed` does not appear in any transcript (`grep -L` over `work/scenarios/ospf/*.log`).
- [ ] **Step 5: Commit**, message "scenarios: ospf, vrrp with 2105 references".

---

### Task 11: `bgp`

**Files:**
- Create: `tests/scenarios/bgp/scenario.yaml` (+ README mapping each check to its `BGP_DANOS.robot` goal), captures under `tests/reference/2105/bgp/`

**Interfaces:**
- Consumes: Task 8; Task 1.
- Produces: the four-router gating scenario and its references.

Topology and addressing follow `BGP_DANOS_testdata.robot` without the LAN hosts: R1–R2 201.1.1.0/24 (.3 R1, .4 R2), R2–R3 202.1.1.0/24 (.3 R2, .4 R3), R2–R4 203.1.1.0/24 (.3 R2, .4 R4), R3–R4 204.1.1.0/24 (.3 R3, .4 R4), R1–R4 205.1.1.0/24 (.3 R1, .4 R4); loopbacks `lo5` 1.1.1.N/32 on RN; AS 100 (R1), 200 (R2, R3, R4); every BGP instance gets `parameters ebgp-requires-policy disabled` (as the suite, for FRR's newer default). Links in this order: R1–R2, R2–R3, R2–R4, R3–R4, R1–R4.

The suite's goals become phases. Each phase is an `action` (configure + commit on the routers it names) followed by its checks; a phase may delete the previous phase's BGP config first:
1. eBGP direct (R1–R2): R1 `show ip bgp summary` neighbor 201.1.1.4 `Established`; R2 learns 1.1.1.1/32 from R1.
2. eBGP multihop (R1 to R2's loopback with `ebgp-multihop 3`, `update-source`, static routes to the loopbacks): session `Established`.
3. iBGP direct (R2–R3, AS 200) and 4. iBGP multihop (loopback to loopback with `update-source lo5`).
5. Route reflector (R4 reflects for R2 and R3 with `route-reflector-client`): R3 learns R2's prefix with an ORIGINATOR_ID/CLUSTER_LIST (`show ip bgp 1.1.1.2/32` wants `Originator: 1.1.1.2`); the three reflection rules from the suite (client→client, client→non-client, non-client→client).
6. Next hop: without `next-hop-self` R3's route via R2 shows the eBGP next hop 201.1.1.3; with `next-hop-self` on R2 it shows 202.1.1.3.
7. iBGP vs eBGP selection: R4 learns 1.1.1.1/32 over both; best path is the eBGP one.
8. Local preference: set on R2's import from R1 to 300; R3's best path shows `Local Pref: 300` (or `localpref 300`).
9. Confederation: AS 200 split into sub-ASes 65002 (R2) and 65003 (R3, R4) with `parameters confederation identifier 200 peers …`; R1 still sees AS 200 only.
10. Dampening: `parameters dampening` on R2; flap R1's loopback twice (actions), R2 `show ip bgp dampening dampened-paths` (or the 2105 equivalent) lists 1.1.1.1/32.

Show: `show ip bgp summary`, `show ip bgp`, `show ip route bgp` per router, captured after phase 8 (before confederation reshapes the AS layout). The exact 2105 command forms are proven in Step 2.

- [ ] **Step 1: Write the scenario file and README.**
- [ ] **Step 2: Run on 2105 with `-capture`.** Expected: `PASS bgp (gating)`; fix and ledger checks or syntax 2105 rejects.
- [ ] **Step 3: Run on NuDanOS, on the Mac, under emulation.** Record wall-clock time in the ledger (Review Focus 2).
- [ ] **Step 4: Resolve every failure** (as Task 9 Step 4).
- [ ] **Step 5: Commit**, message "scenarios: bgp (DANOS BGP suite) with 2105 references".

---

### Task 12: `mpls-ldp` (reported, non-gating)

**Files:**
- Create: `tests/scenarios/mpls-ldp/scenario.yaml` (`gating: false`) (+ README), captures under `tests/reference/2105/mpls-ldp/`

**Interfaces:**
- Consumes: Task 8.
- Produces: the reported scenario.

Content: R1–R2 10.0.12.0/24, R2–R3 10.0.23.0/24, loopbacks 10.255.0.N/32, OSPF area 0 everywhere for reachability, `set protocols mpls-ldp address-family ipv4 transport-address 10.255.0.N`, `… discovery interface interface ${RN:RM}` per link, `… label-policy allocate host-routes`. Checks: `op` R2 `show mpls ldp neighbor` wants two `OPERATIONAL` sessions; `op` R1 `show mpls ldp binding` (2105 form proven in Step 2) wants a label for 10.255.0.3/32; `op` R1 `ping 10.255.0.3 count 3 interface 10.255.0.1` wants ` 0% packet loss`.

- [ ] **Step 1: Write the file and README.**
- [ ] **Step 2: Run on 2105 with `-capture`.** Expected: `PASS mpls-ldp (reported)`.
- [ ] **Step 3: Run on NuDanOS.** A failure here is recorded in the ledger with its cause (kernel MPLS modules, `net.mpls.*` sysctls, FRR ldpd); fix it if it is a small NuDanOS bug, otherwise ledger it as a known 1.0 limitation. It never blocks.
- [ ] **Step 4: Commit**, message "scenarios: mpls-ldp (reported) with 2105 references".

---

### Task 13: Boot-config hook directory in `vyatta-cfg`

**Files:**
- Modify: port `vyatta-cfg` `scripts/vyatta-boot-config-loader`, `Makefile.am` (or packaging that installs `/opt/vyatta/etc/boot-config.d/`), `debian/changelog`
- Create: port `vyatta-cfg` `tests/boot-config-hooks.sh`

**Interfaces:**
- Produces: before `loadFile`, the loader runs every executable regular file in `${BOOT_CONFIG_HOOK_DIR:-/opt/vyatta/etc/boot-config.d}` in `LC_ALL=C` sorted order as `<hook> <boot file>`, logs `boot-config hook <name>: ok|failed (<status>)` and never aborts on a hook failure. The directory ships empty.

- [ ] **Step 1: Write the failing test** `tests/boot-config-hooks.sh` (bash): a temp hook dir with `10-a` (appends `a` to a marker file), `20-b` (appends `b`), `30-fail` (exits 3), and a non-executable `40-skip`; `CAPI` replaced by a stub (`BOOT_CONFIG_CAPI` override, default `/bin/cli-shell-api`) that records `loadFile`, and `cfgcli` by a stub on `PATH`. Assert: the marker reads `ab`; the log contains `boot-config hook 30-fail: failed (3)`; `loadFile` was still called; `40-skip` did not run.
- [ ] **Step 2: Run** `bash tests/boot-config-hooks.sh`. Expected: FAIL (no hooks run).
- [ ] **Step 3: Implement** in `vyatta-boot-config-loader` (add `BOOT_CONFIG_CAPI` and `BOOT_CONFIG_HOOK_DIR` overrides; the hook loop before `# do load`); install the empty directory; run the test from the package build (`override_dh_auto_test` or the existing test target).
- [ ] **Step 4: Run** `bash tests/boot-config-hooks.sh`. Expected: `boot-config-hooks: OK`.
- [ ] **Step 5: Changelog, build** (`-local vyatta-cfg=../port-vyatta-cfg build vyatta-cfg`, expect `built`), **commit, push, CI.**

---

### Task 14: Set-aside hook in `vyatta-kernel-forwarding`

**Files:**
- Create: port `vyatta-kernel-forwarding` `boot-config.d/50-dpdk-set-aside` (Perl, SPDX `GPL-2.0-only`), `update-motd.d/60-dpdk-set-aside`, `dpdk-only-paths`, `tests/set-aside.t`, `tests/set-aside-consistency.t`, test fixtures under `tests/set-aside/`
- Modify: `debian/vyatta-kernel-forwarding.install`, `Makefile` (`check` runs `prove tests/`), `debian/changelog`

**Interfaces:**
- Consumes: Task 13's hook directory.
- Produces:
  - `dpdk-only-paths`: one path pattern per line, `*` matching one node name, e.g. `security firewall`, `service nat`, `policy qos`, `interfaces dataplane * cpu-affinity`, `interfaces dataplane * receive-cpu-affinity`, `interfaces dataplane * transmit-cpu-affinity`, `interfaces dataplane * ip gratuitous-arp`, `interfaces dataplane * vif * ip gratuitous-arp`; plus value conditions `interfaces dataplane * speed !auto` and `interfaces dataplane * duplex !auto` (a trailing `!auto` means: only when the value is not `auto`)
  - `50-dpdk-set-aside <config.boot>`: parses the brace-structured config (nodes, `tag value {` blocks, leaves with quoted values, `/* … */` comments, the version footer). It removes matching nodes. When anything was removed, it saves the original once as `<dir>/config.boot.2105-original`, writes the removed nodes to `<dir>/config.boot.dpdk-only` in config syntax, logs each path with `logger -t dpdk-set-aside`, With nothing to remove, the file is untouched (same bytes, same mtime).
  - `/etc/update-motd.d/60-dpdk-set-aside`: when `/config/config.boot.dpdk-only` exists, prints a notice naming it and saying the settings need the DPDK dataplane; prints nothing otherwise. The notice keeps appearing on later boots, after the hook has already removed the lines from the boot file.

- [ ] **Step 1: Write the tests** in `tests/set-aside.t` (fixtures in `tests/set-aside/`):
  - removes `security firewall` and a nested `interfaces dataplane dp0s3 cpu-affinity 1` while keeping the interface's address
  - `speed 100m` removed, `speed auto` kept
  - a quoted description containing `{` and `}` survives untouched
  - comments survive
  - a file without the version footer works
  - nothing to remove: byte-identical, no side files
  - second run on its own output: no change, and `config.boot.2105-original` is not overwritten
  - `config.boot.dpdk-only` holds exactly the removed nodes

  In `tests/set-aside-consistency.t`:
  - every `deviation` target in `yang/vyatta-kernel-forwarding-deviations-v1.yang` maps to a `dpdk-only-paths` entry
  - every package in `distro/docs/kernel-forwarding.md`'s DPDK-only section that ships a config YANG module has its top-level container covered; the doc path comes from `NUDANOS_KF_DOC`, and the test skips with a message when it is unset

  The hook takes a `SET_ASIDE_LOGGER` override for tests. Add `motd notice` cases: the update-motd script prints the notice when the set-aside file exists (path from `SET_ASIDE_FILE` override) and nothing otherwise.
- [ ] **Step 2: Run** `prove tests/set-aside.t tests/set-aside-consistency.t`. Expected: FAIL (hook missing).
- [ ] **Step 3: Implement** the hook and the path list; install the hook to `/opt/vyatta/etc/boot-config.d/` and the list to `/usr/share/vyatta-kernel-forwarding/dpdk-only-paths`.
- [ ] **Step 4: Run** `NUDANOS_KF_DOC=/Volumes/nudanos/distro/docs/kernel-forwarding.md prove tests/`. Expected: all pass.
- [ ] **Step 5: Changelog, build, commit, push, CI.** Document the behaviour in `distro/docs/kernel-forwarding.md` (a "2105 configs at boot" section) in the same task, committed to distro.

---

### Task 15: Sampler and `distro-build test fixtures`

**Files:**
- Create: `tests/reference/2105/sampler/<feature>.set` for `interfaces`, `vif`, `bridge`, `bonding`, `static-routes`, `policy`, `bgp-options`, `ospf-options`, `vrrp`, `snmp`, `tacacs`, `ntp`, `syslog`, `dns`, `ssh`, `users`, `time-zone` (with `set system time-zone US/Pacific`), `lldp`, and DPDK-only `firewall`, `nat`, `qos`, `cpu-affinity`, `speed-duplex`, `gratuitous-arp`; their 2105 captures; `internal/fixtures/fixtures.go`, `internal/fixtures/fixtures_test.go`
- Modify: `internal/scenario/run.go` (sampler capture mode), `cmd/scenario/main.go` (`-fixtures`), `cmd/distro-build/main.go` (`test fixtures`)

**Interfaces:**
- Consumes: Tasks 8, 13, 14 (the image must contain both port changes).
- Produces:
  - sampler capture: `distro-build -reference-iso … -capture test scenario sampler` commits each `<feature>.set` on one 2105 router and writes `sampler/<feature>/{config.boot,commands.txt}`. DPDK-only inputs commit on 2105, which has the dataplane.
  - `func Check(ctx context.Context, vm *topology.VM, refs []Ref, t func(time.Duration) time.Duration) []Failure` with `type Ref struct{ Name, ConfigBoot, Commands string }` and `type Failure struct{ Name, Detail string }`. For each ref: copy the `config.boot` into the VM over the console (base64 in a heredoc to `/tmp/ref.boot`); run `/opt/vyatta/etc/boot-config.d/50-dpdk-set-aside /tmp/ref.boot`; then in configuration mode run `load /tmp/ref.boot` and `commit`. Compare `show configuration commands` with `commands.txt` minus the set-aside lines and minus the base-config lines (management address, host name). Any other difference is a `Failure`.
  - `func RebootTest(ctx context.Context, vm *topology.VM, ref Ref, t func(time.Duration) time.Duration) error`: writes the `cpu-affinity` sampler capture (plus the management base config) to `/config/config.boot` with `sudo`, reboots, logs in, and requires all of these:
    - `show configuration commands` contains the sampler's non-DPDK lines
    - it lacks `cpu-affinity`
    - `/config/config.boot.dpdk-only` exists
    - the login banner mentions `dpdk-only`
  - `distro-build test fixtures`: one NuDanOS VM, every ref under `tests/reference/2105/` (scenario routers and sampler), then the reboot test; prints `fixtures: OK (<n> configs)` or the failures and exits 1.

- [ ] **Step 1: Write the fixtures tests** with a scripted console: `TestCheckPassesWhenOnlySetAsideLinesDiffer`, `TestCheckReportsUnexpectedDifference` (an extra missing line is named), `TestCheckIgnoresBaseConfigLines`.
- [ ] **Step 2: Run** `go test ./internal/fixtures/`. Expected: build failure; implement; `ok`.
- [ ] **Step 3: Write the sampler inputs** (values are test values; each file's first line is a comment naming the feature).
- [ ] **Step 4: Capture the sampler on 2105.** Expected: a `config.boot` and `commands.txt` per feature. A line 2105 rejects is fixed in the input and ledgered.
- [ ] **Step 5: Rebuild the image** with Tasks 1, 13, 14 in it (`build`, `repo -key …`, `image`).
- [ ] **Step 6: Run** `./distro-build -work /Volumes/nudanos/work test fixtures`.
  Expected: initially, failures: NuDanOS bugs or set-aside gaps. Resolve each test-first in the owning port, as Task 9 Step 4, until `fixtures: OK (<n> configs)`. Ledger the answer to the spec's open question (what `loadFile` does with nodes whose YANG is absent).
- [ ] **Step 7: Commit** sampler, captures and fixtures code, message "fixtures: 2105 sampler and the load check".

---

### Task 16: Nightly: fixtures and scenarios gate the release

**Files:**
- Modify: `.github/workflows/nightly.yml`

**Interfaces:**
- Consumes: `distro-build test fixtures`, `distro-build test scenarios`.

- [ ] **Step 1: Add the steps** after "UEFI smoke boot", each with `timeout-minutes`:
  - `Fixtures (2105 configs load)`: `./distro-build -work "$RUNNER_TEMP/work" test fixtures`
  - `Scenarios (layer 4)`: `./distro-build -work "$RUNNER_TEMP/work" test scenarios`

  Add an `actions/upload-artifact` step with `if: always()` for `work/scenarios/` (transcripts, show dumps, diffs). The release job already needs `build`; the new steps sit in `build`, so a gating failure blocks publishing. Write the scenario summary table to `$GITHUB_STEP_SUMMARY`; `cmd/scenario` prints it, and the step appends it.
- [ ] **Step 2: Validate the YAML** (`ruby -ryaml -e 'YAML.load_file(".github/workflows/nightly.yml")'`). Expected: no error.
- [ ] **Step 3: Commit and push** distro, message "nightly: fixtures and layer 4 scenarios gate the release".

---

### Task 17: End to end and checkpoint 1.0

**Files:**
- Modify: `docs/superpowers/specs/2026-09-28-debian13-revival-design.md` (§8.4, §8.5, §9), `README.md` (test commands)

- [ ] **Step 1: Run every layer locally** on a fresh image (`build`, `repo -key …`, `image`, `test install`, `test boot`, `test fixtures`, `test scenarios`). Expected: `boottest: OK`, `fixtures: OK (<n> configs)`, every gating scenario `PASS`, `mpls-ldp` PASS or a ledgered known limitation.
- [ ] **Step 2: Trigger the nightly** (`gh workflow run nightly -R nudanos/distro --ref main`; approved in plan 3 as run-and-publish). Expected: build and release jobs succeed; the step summary lists all scenarios.
- [ ] **Step 3: Amend the base spec.**
  - §8.4: the Go runner, with the Robot suites as the test-case source.
  - §8.5: the sampler, committed captures, the 2105 ISO kept off CI, boot-time set-aside.
  - §9: mark the 1.0 row done with the date, the nightly run and what passed.
  - README: document `test scenario`, `test scenarios` and `test fixtures`, plus how to capture against 2105.
- [ ] **Step 4: Commit and push**, message "Checkpoint 1.0: scenarios and 2105 fixtures pass in the nightly".
