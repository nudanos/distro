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

## Test

Each layer runs inside the `nudanos/tester` container, on the newest ISO in
`work/image`:

```bash
./distro-build -work /Volumes/nudanos/work image             # build the ISO
./distro-build -work /Volumes/nudanos/work test install      # layer 2: install, remove, purge
./distro-build -work /Volumes/nudanos/work test boot         # layer 3: boot, configure, install, reboot
./distro-build -work /Volumes/nudanos/work test scenario bgp # layer 4: one scenario (tests/scenarios/<name>)
./distro-build -work /Volumes/nudanos/work test scenarios    # layer 4: every scenario
./distro-build -work /Volumes/nudanos/work test fixtures     # every captured DANOS 2105 configuration loads
```

`test scenarios` fails when a gating scenario fails (`gating: false`, like
`mpls-ldp`, is reported only); `test fixtures` prints `fixtures: OK (<n>
configs)`. Transcripts, show dumps and diffs are kept in
`work/scenarios/<image>/<scenario>/`. Without `/dev/kvm` every timeout is six
times longer and a full run takes hours: run one at a time.

The 2105 references under `tests/reference/2105/` are committed, so neither CI
nor these commands need DANOS 2105. To capture them again, on a machine with
`danos-2105-base-amd64.iso` (SHA-256 in `tests/reference/2105/README.md`; never
commit or upload it):

```bash
./distro-build -work /Volumes/nudanos/work -reference-iso ~/Downloads/danos-2105-base-amd64.iso -capture test scenario <name>
./distro-build -work /Volumes/nudanos/work -reference-iso ~/Downloads/danos-2105-base-amd64.iso -capture test scenario sampler
```

`sampler` commits each `tests/reference/2105/sampler/<feature>.set` on one
2105 router and captures it; a feature that already has a capture is skipped
(delete its directory to capture it again).

## Lint

`distro-build lint [name…]` checks every ready 1.0 port (or the named ones)
with the Debian 13 analyzers: gofmt and staticcheck (Go), perltidy and
perlcritic (Perl), shellcheck, flake8 and pylint (Python), cppcheck and the
compiler's `-Wall -Wextra` warnings (C/C++). The source analyzers run in the
`nudanos/lint` image (`lint/`); compiler warnings and staticcheck come from
each port's build log, so build the ports first.

```bash
./distro-build -work /Volumes/nudanos/work lint                      # every port, against lint-baseline.json
./distro-build -work /Volumes/nudanos/work lint vyatta-cfg           # one port
./distro-build -work /Volumes/nudanos/work -update-baseline lint vyatta-cfg   # lower its allowance after fixes
```

`lint-baseline.json` holds how many findings each port may still have; lint
fails when any port has more, or any port missing from it has one. It only
goes down, and is removed once every port is at zero. A finding is silenced
only inline, in the tool's own syntax, with `reason: <why>` on the same line;
a port's `nudanos-lint.yaml` may only list paths to skip (docs/porting.md).

## Build hosts

Every package build, the ISO build and the boot test run inside Debian 13
containers, so the host only drives them. It needs:

- Go 1.26 or newer, git, python3
- Docker, or Podman (`distro-build -engine podman`). `image` runs a privileged container
  (live-build mounts loop devices), so use Docker or rootful Podman for it.
- `/dev/kvm` for `test boot` (it falls back to emulation, about six times slower)
- an amd64 machine and about 20 GB free (work directory plus container images)

Tested on 2026-10-04: distro-build built from source, its Go tests, and a
package build in a fresh work directory.

| Host | Distribution Go | Install |
|---|---|---|
| Arch Linux | 1.27 | `pacman -S go git python docker` |
| Fedora 44 | 1.26 | `dnf install golang git python3 moby-engine` |
| openSUSE Tumbleweed | 1.27 | `zypper in go git python3 docker` |
| openSUSE Leap 16.0 | 1.27 | `zypper in go git python3 docker` |
| AlmaLinux 9, Rocky Linux 9 | 1.26 | `dnf install golang git python3`, plus Docker CE from download.docker.com (or Podman) |
| Alpine edge | 1.27 | `apk add go git python3 docker bash` |
| Debian 13 | 1.24 * | `apt install golang-go git python3 docker.io` |
| Ubuntu 24.04 | 1.22 * | `apt install golang-go git python3 docker.io` |
| Alpine 3.23 | 1.25 * | `apk add go git python3 docker bash` |
| macOS | (Homebrew) | Go from go.dev or Homebrew, Docker Desktop |

\* The distribution's Go is older than 1.26. Set `GOTOOLCHAIN=auto` and Go
downloads the 1.26.0 toolchain `go.mod` names (some distributions, Alpine and
Fedora among them, default to `GOTOOLCHAIN=local`):

```bash
GOTOOLCHAIN=auto go build -o distro-build ./cmd/distro-build
```

The tests drove an existing Docker daemon through its socket; on a real host,
start the engine first (`systemctl enable --now docker`, or
`rc-update add docker && service docker start` on Alpine) and add your user to
the `docker` group or run as root.

NuDanOS itself is a Debian system: these hosts build it, they are not bases it
can be rebuilt on.

Contributions use the Developer Certificate of Origin: sign off commits with `git commit -s`.
