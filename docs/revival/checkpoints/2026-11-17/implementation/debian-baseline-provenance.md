# Debian baseline image provenance

The preserved pre-change appliance ISO and package repository came from the same successful `nudanos/distro` nightly run [41652774933](https://github.com/nudanos/distro/actions/runs/41652774933), created 2026-11-03, with distro workflow commit `864370dc3ff5a6086ce298b147f79ac2c4de315e`. The four changes under test were not yet merged into their upstream branches at that time.

- GitHub Actions ISO artifact `13242785835`: advertised ZIP SHA256 `9b0e5acbbe84e10000f612330c56536842d656e14de63027547bfc031a98a860`; downloaded ZIP matched. Its extracted `nudanos-1.0~20261023-amd64.iso` is 766,982,144 bytes, SHA256 `87cb0f6c07c48a347eb3a6e613da988146f78c14cfa798d8da7f6016adf0f943`.
- GitHub Actions apt-repo artifact `13239336290`: downloaded ZIP SHA256 `76a4aeb2d24532aebcb71ce4dec8e71cf6bc5fe281b6e12344c462e8032f4836`, matching GitHub artifact metadata. The repository was previously verified against the distro-pinned signing key; verify again when preparing the candidate.
- Local copies: `test-runtime/downloads/nudanos-debian-baseline-iso-artifact.zip`, `test-runtime/baseline-image/nudanos-1.0~20261023-amd64.iso`, and `test-runtime/downloads/nudanos-debian-baseline-apt-repo.zip`.

The workflow SHA identifies distro configuration; the package manifest follows branches, so it does not alone identify all package source commits. Retain the signed package archive and package checksums as the evidence for unchanged baseline binaries. The extracted ISO has not yet been booted in this session, and no upgrade has been performed yet.
