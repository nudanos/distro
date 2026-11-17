# NuDanOS revival checkpoint — 2026-11-17

This is a curated source and evidence backup. The authoritative current status is [implementation/integration-progress.md](implementation/integration-progress.md); earlier reports are chronological records and do not supersede it. Password renewal can update running state without persisting the change across reboot; the target-only persistence repair is designed but **not implemented or validated**. Debian end-to-end acceptance remains incomplete. The 1.0~20261109.1 image was rejected for incorrect setgid file groups; no image was accepted or released. Native Arch implementation has not started.

## Component source checkpoints

These links identify the independently checkpointed component changes; the parent tracks their GitHub publication and verification. Existing merged PR work remains in the respective repositories.

- [VCI validation — da9a6ad](https://github.com/nudanos/vci/commit/da9a6ad99ba8c06b2a7221b888c8bbea764e7e3c)
- [vyatta-cfg boot hook safety — f71d97a](https://github.com/nudanos/vyatta-cfg/commit/f71d97aa7583f415baaaeb1dab2869cc01c136dd)
- [kernel forwarding safety — 5897ef6](https://github.com/nudanos/vyatta-kernel-forwarding/commit/5897ef6532b930e62d9e92e6e472aa466181998d)
- [REST hardening — 84143ff](https://github.com/nudanos/vyatta-rest/commit/84143ff4163c7ae9a4eaa6cafdddf4133580a4f6)
- [login state-provider fix — b08af22](https://github.com/nudanos/vyatta-login/commit/b08af22967fad622e3e54f2ce03ea2e52e4c8f15)
- [distro safety fixtures and documentation — 9375a365](https://github.com/nudanos/distro/commit/9375a365237f5fe06a3d03278f3f13c1633d6901)

## Contents and limitations

- `review/` holds authored review reports and repository inventories. Cloned upstream sources and their `.git` data are excluded.
- `implementation/` holds plans, reports, test/build scripts, local hooks, checksum inventories and reviewed host-side build logs. The Arch research subdirectory includes authored comparison scripts and metadata only.
- `test-runtime/` contains source-only controllers and probes, including four uncommitted REST draft files that are experimental and historical, not validated production changes. Transient disposable VM fixture passwords and a generated rescue command were redacted in the staged copies listed in [REDACTED-PATHS.txt](REDACTED-PATHS.txt). Those harnesses are **not turnkey** until replacement disposable fixture credentials are configured. Their originals remain local and outside Git. The documented public stock `vyatta/vyatta` live-login contract is retained.
- `historical/task3-guest-stage/`, `historical/distro-hidden-notes/` and `historical/archives/` are explicitly historical source/test snapshots, not current source-of-truth. They can contain intentional, already-public synthetic unit-test literals and salts, which are test-only and must never be used as real credentials. The 28 small archives were retained intact after listing and screening for unsafe paths, links, special entries, runtime binaries and credential signatures; see [archive-inventory.txt](historical/archive-inventory.txt). A PGP private-key header marker in the distro archive is a shell validation pattern, not private key material.
- `guest-export/` contains reviewed authored scripts and historical source tests exported from the existing disposable Debian helper. `guest-evidence-summary.txt` holds fixed counts and SHA256 identifiers only; raw guest logs stay in the local VM and are excluded. The helper was shut down after export. Historical Arch source snapshots are host exports; no fresh exhaustive inventory of arbitrary stopped guest files is claimed.

No VM disks, ISOs, squashfs images, binary packages, downloaded dependency caches, private SSH/signing keys, known-hosts, cloud-init data, raw serial/credential logs, or `.git` directories are included. This is not a byte-complete backup of the runtime environment. Binary source snapshots remain unchanged; staged UTF-8 text is normalized to LF. [ORIGINAL-SHA256.tsv](ORIGINAL-SHA256.tsv) records copies before redaction and normalization, while [MANIFEST-SHA256.tsv](MANIFEST-SHA256.tsv) records final staged bytes. The latter intentionally omits itself to avoid a recursive hash.

## Scan and review boundary

Known transient VM fixture literals were replaced in staged controller/probe copies. High-confidence private-key/token signatures and disallowed runtime file types were screened; no actual private key or token was found in staged text, and the archive-marker exception above was reviewed. This is a bounded scan, not a guarantee that every possible secret or sensitive detail is absent. Independent review is required before publication. No implementation, package build, image build, release, or host feature change was performed during this backup preparation.
