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
