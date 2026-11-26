#!/usr/bin/env python3
"""Build a new test pool by replacing the image-tools source and binary set."""
import hashlib
from pathlib import Path
import shutil
import sys


def paragraphs(path):
    for block in path.read_text().strip().split("\n\n"):
        fields = {}
        field = None
        for line in block.splitlines():
            if line.startswith((" ", "\t")):
                fields[field] += "\n" + line.strip()
            else:
                field, value = line.split(":", 1)
                fields[field] = value.strip()
        yield fields


repo, artifact, pool = (Path(arg).resolve() for arg in sys.argv[1:])
if pool.exists():
    raise SystemExit(f"Pool already exists: {pool}")
source = "vyatta-image-tools"
release = repo / "dists/trixie/main"
old = {
    Path(record["Filename"]).name
    for record in paragraphs(release / "binary-amd64/Packages")
    if record.get("Source", record["Package"]).split()[0] == source
}
for record in paragraphs(release / "source/Sources"):
    if record["Package"] == source:
        old.update(line.split()[2] for line in record["Checksums-Sha256"].splitlines() if line)
if not old or not any(name.endswith(".dsc") for name in old):
    raise SystemExit("Prior image-tools package set is missing")

changes, = artifact.glob("*.changes")
metadata, = paragraphs(changes)
if metadata["Source"] != source or metadata["Version"] != "5.57":
    raise SystemExit("Unexpected replacement package set")
packages = []
for line in metadata["Checksums-Sha256"].splitlines():
    if not line:
        continue
    expected, size, name = line.split()
    if Path(name).name != name:
        raise SystemExit(f"Invalid artifact filename: {name}")
    file = artifact / name
    with file.open("rb") as stream:
        sha256 = hashlib.file_digest(stream, "sha256").hexdigest()
    if sha256 != expected or file.stat().st_size != int(size):
        raise SystemExit(f"Invalid artifact payload: {file.name}")
    if name.endswith((".deb", ".dsc", ".tar.gz", ".tar.xz", ".tar.bz2")):
        packages.append(file)
if not any(p.suffix == ".deb" for p in packages) or not any(p.suffix == ".dsc" for p in packages):
    raise SystemExit("Both binary and source payloads are required")

original = repo / "pool/main"
if any(not (original / name).is_file() for name in old):
    raise SystemExit("The prior repository pool is incomplete")
shutil.copytree(original, pool, ignore=lambda _directory, names: old.intersection(names))
for file in packages:
    target = pool / file.name
    if target.exists():
        raise SystemExit(f"Duplicate replacement payload: {target.name}")
    shutil.copy2(file, target)
print(f"Replaced {len(old)} image-tools payloads with {len(packages)} verified 5.57 payloads")
