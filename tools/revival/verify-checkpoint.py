"""Verify checkpoint bytes and executable intent in the Git index."""

import argparse
import csv
import hashlib
import io
from pathlib import Path, PurePosixPath
import subprocess


parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("manifest", type=Path)
parser.add_argument("--manifest-sha256")
args = parser.parse_args()
repo = Path(subprocess.check_output(
    ["git", "rev-parse", "--show-toplevel"], text=True).strip()).resolve()
manifest = args.manifest.resolve().relative_to(repo).as_posix()
prefix = str(PurePosixPath(manifest).parent) + "/"
records = subprocess.check_output(
    ["git", "ls-files", "--stage", "-z", "--", prefix])
entries = {}
for record in filter(None, records.split(b"\0")):
    metadata, name = record.split(b"\t", 1)
    mode, oid, stage = metadata.decode().split()
    if stage != "0" or mode not in ("100644", "100755"):
        raise SystemExit("Unexpected index entry: " + name.decode())
    entries[name.decode()] = (mode, oid)
if manifest not in entries:
    raise SystemExit("Manifest is not staged")

oids = sorted({oid for _, oid in entries.values()})
result = subprocess.run(["git", "cat-file", "--batch"], check=True,
                        input=("\n".join(oids) + "\n").encode(),
                        stdout=subprocess.PIPE).stdout
objects = {}
offset = 0
for oid in oids:
    end = result.index(b"\n", offset)
    actual_oid, kind, size = result[offset:end].decode().split()
    if actual_oid != oid or kind != "blob":
        raise SystemExit("Unexpected Git object: " + oid)
    offset = end + 1
    objects[oid] = result[offset:offset + int(size)]
    offset += int(size) + 1

manifest_bytes = objects[entries[manifest][1]]
digest = hashlib.sha256(manifest_bytes).hexdigest()
if args.manifest_sha256 and digest != args.manifest_sha256:
    raise SystemExit("Manifest differs from the reviewed checksum")
rows = list(csv.DictReader(io.StringIO(manifest_bytes.decode()), delimiter="\t"))
expected = {manifest}
total = len(manifest_bytes)
for row in rows:
    relative = PurePosixPath(row["path"])
    if relative.is_absolute() or ".." in relative.parts:
        raise SystemExit("Unsafe manifest path")
    name = prefix + str(relative)
    if name in expected:
        raise SystemExit("Duplicate manifest path: " + name)
    expected.add(name)
    if name not in entries:
        raise SystemExit("Missing index entry: " + name)
    mode, oid = entries[name]
    data = objects[oid]
    if len(data) != int(row["bytes"]) or hashlib.sha256(data).hexdigest() != row["sha256"]:
        raise SystemExit("Content mismatch: " + name)
    if row["mode_intent"] == "script-shebang-executable-intent" and mode != "100755":
        raise SystemExit("Missing executable intent: " + name)
    total += len(data)
if set(entries) != expected:
    raise SystemExit("Unexpected extra index entries")
print(f"Verified {len(rows)} manifest entries; {len(entries)} Git files; {total} bytes")
print("Manifest SHA256 " + digest)
