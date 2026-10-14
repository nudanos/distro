# NuDanOS code quality: analyzers and review (design)

Addendum to `2026-09-28-debian13-revival-design.md` (the spec). It answers the
user's request of 2026-10-11, "vet vendor code as well all code needs to be
checked for improvements". The `go vet` part is done: every Go port vets every
package, vendored code included, and the findings were fixed in plan 4 (ledger,
Task 17). This design covers the rest: static analysis for every language,
then a review of every port.

**Status (2026-10-14):** sections 1–6 approved by the user in brainstorming
(sections 1–3 were presented on 2026-10-11 and approved on 2026-10-14, after
the move to the build server). Next: the user reviews this written spec; then
the implementation plans (`superpowers:writing-plans`) and the choice of
execution method. No implementation before a plan is approved.

Decisions the user took while scoping:

- **Depth:** fix everything. Every analyzer finding and every review comment is
  fixed, or closed with a recorded ruling.
- **Structure:** analyzers first, then the review (approach 1 of 3). The
  alternatives were one port at a time with everything at once (no gate until
  the end) and analyzers inside each package's build (about six build
  dependencies added to each of 95 packages, and each rule change touching every
  port).
- **When:** after checkpoint 1.0 closes (a green nightly and the checkpoint
  commit) and before the 1.1 rewrites, so the rewrites start from reviewed code.

## 1. Scope

- **In scope:**
  - the ports NuDanOS builds for 1.0: every `kind: danos` manifest entry with
    `milestone: "1.0"` and `ready: true` (95 on 2026-10-14);
  - the distro repo's own Go code;
  - the patches NuDanOS carries against upstream packages (`patches/`, the 11
    ready `kind: upstream` entries and the patched Debian packages). These are
    read in the Phase B review, not linted (§4).
- **Out of scope:**
  - the upstream projects themselves (FRR, net-snmp, keepalived, ntpsec, owamp,
    the pam/nss tacplus sources). Reformatting code we do not own would mean
    permanent patches. A real bug found there gets a distro patch with a test
    and is reported upstream.
  - vyatta-dataplane's forwarding C, which waits for milestone 2. For 1.0 only
    what its protobuf build profile builds is in scope.
  - entries for later milestones (1.1, 2, later). Each is linted and reviewed
    when it becomes ready, under the same rules.
- **Done:** see §6.

## 2. `distro-build lint`

- **Where:** a new `distro-build` subcommand, `lint [name…]`, running in the
  existing builder container. The builder image gains the trixie analyzer
  packages (§3). Like every other command, it runs with Docker or Podman
  (`-engine`).
- **What it checks:** each port's source tree as checked out under `work/src`,
  using each language's tools wherever that language is found.
- **Configuration:** defaults live in distro, so a rule changes in one place.
  Each port may carry a small `nudanos-lint.yaml` that lists only its
  exceptions: paths to skip, such as generated files and test data.
- **Suppressions:** a finding can be silenced only inline, using the tool's own
  syntax, and the same line must give a reason. lint fails on a suppression
  without a reason, so "fix everything" cannot become "silence everything".
- **Compiler warnings:** C/C++ warnings are read from the package build logs the
  build already keeps (`work/logs/<name>.log`). Any `warning:` in in-scope code
  is a finding.
- **Output:** findings grouped by port, file and rule, then a summary count per
  port. The exit status is non-zero when the ratchet (§4) fails.

## 3. Rules per language

Tool defaults, except where noted. Every tool is packaged in Debian 13.

| Language | Tool (trixie version) | Rules |
|---|---|---|
| Go | gofmt; staticcheck (`go-staticcheck` 2024.1) | gofmt clean; staticcheck default checks. `go vet` stays in each package's build. |
| Perl | perltidy (20250105); perlcritic (1.156) | One shared perltidy profile; perlcritic severity 3 ("harsh"). Severity 1 mostly demands POD and version strings in every file. |
| Shell | shellcheck (0.10.0) | All levels, style included; covers `debian/` maintainer scripts too. |
| Python | flake8 (7.1.1); pylint (3.3.4) | flake8 defaults; pylint without its docstring and naming-convention checks. |
| C/C++ | cppcheck (2.17); compiler | cppcheck `warning,style,performance,portability`; compiler warnings at the package's own flags plus `-Wall -Wextra`. clang-tidy (19) is used in the Phase B review of C ports, not as a gate: most of these autotools builds cannot produce a compile database cheaply. |

## 4. Phase A: analyzers down to zero

1. **The tool.** Builder image packages, the `lint` command, and its tests
   (§6).
2. **A ratchet, not a big bang.**
   - The first full run writes `lint-baseline.json` in distro: each port's
     finding count.
   - The nightly fails if any port's count rises above its baseline.
   - When a port reaches zero, its entry is removed.
   - When the file is empty, lint enforces zero everywhere and the baseline
     mechanism is deleted.
3. **Fixing**, port by port in the build's dependency order (libraries first):
   - **Formatting** (gofmt, perltidy) goes in a commit of its own with no other
     change, so a reviewer can skip a pure reformat.
   - **Analyzer findings** go in a separate commit. A fix that changes behaviour
     gets a test that fails first (the vet work's convention). Pure style fixes
     (unused variables, quoting) need none.
4. **Verification and pushing** in batches, as §6 describes.
5. **Not linted:** patches against upstream code. The tools would report the
   upstream code around each hunk. Phase B reads them instead.

## 5. Phase B: reviewing each port

- **Order:** by how much the router depends on the code, in batches of about
  6–10 ports. Each batch gets its own implementation plan, which fixes the
  batch's exact list. The groups below set the order; the names in them are
  the first members, not the whole list.
  1. **Router core:** configd, config, vyatta-cfg, vyatta-login, vyatta-tacacs,
     aaa, vyatta-rest, vyatta-image-tools, vyatta-kernel-forwarding.
  2. **Routing and services:** vyatta-protocols-frr, vyatta-protocols-common,
     vyatta-vrrp, vyatta-service-snmp, vyatta-service-ntp, vyatta-service-ssh,
     vyatta-service-dns and similar.
  3. **Interfaces and system:** vyatta-interfaces, vyatta-cfg-system,
     vyatta-cfg-dataplane (1.0 profile), vyatta-service-bridge,
     vyatta-cfg-bonding.
  4. **The rest:** libraries, op commands, and the upstream patches.
- **How a port is reviewed:** the reviewer reads the whole port, not a diff,
  against a fixed checklist:
  - correctness;
  - error handling;
  - security: input reaching a shell or SQL, privilege, secrets in logs;
  - concurrency;
  - fit with Debian 13;
  - dead code;
  - tests.

  The checklist is seeded from `docs/REVIEW.md`. C ports also get clang-tidy.
- **Review record:** `docs/reviews/<port>.md` in distro. It lists every finding
  with a severity, then the commit that fixed it, or a ledger-style ruling
  (`Ruling: <what> — <why> — <cost if wrong>`) when it was not fixed as
  written.
- **Fixing:**
  - A bug gets a test that fails first, then the fix.
  - Cleanups are fixed directly.
  - Every comment is closed.
- **Stop and ask the user:** any fix that changes behaviour an operator can see,
  such as CLI or configuration syntax, `show` output, or anything a DANOS 2105
  configuration relies on. Such a change breaks "2105 configs load unchanged",
  so the user decides it; it is not ruled on alone.

## 6. Testing, verification and done

- **The lint tool:** Go tests in distro, run with the rest of `go test ./...`.
  A fixture port carries one planted finding per tool, a suppression with a
  reason (which must pass) and one without (which must fail). Each test fails
  before the code that satisfies it.
- **Every fix:** the package's own build tests must pass, vet included. A change
  in behaviour needs a test that failed before the fix.
- **Every batch, in Phase A and Phase B:**
  - one full test run on the build server before pushing: `test install`,
    `test boot`, `test fixtures` (the 2105 configurations) and `test scenarios`
    (the seven 1.0 scenarios);
  - VM runs one at a time, never concurrently and never alongside a build (the
    user's standing rule);
  - nothing is pushed on a red run.
- **Guarding operator-visible behaviour:** the 2105 fixtures and the scenarios'
  `show` comparisons catch accidental behaviour changes. A fix that would add a
  line to any `accepted.diff` is the "stop and ask" case of §5. It is never
  absorbed by updating the file alone.
- **The nightly:** gains a lint step after the build, with the ratchet first and
  zero at the end. It must stay green throughout.
- **Done when:**
  1. `distro-build lint` reports zero findings in the nightly and
     `lint-baseline.json` is gone;
  2. every in-scope port has a `docs/reviews/<port>.md` with every finding
     closed, by a fix commit or a ruling;
  3. the nightly is green with layers 1–4 and the 2105 fixtures;
  4. the spec's roadmap (§9) has a row for this effort, with the date.
