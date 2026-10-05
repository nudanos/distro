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
