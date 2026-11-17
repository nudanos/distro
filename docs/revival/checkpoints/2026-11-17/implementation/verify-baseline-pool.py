#!/usr/bin/env python3
"""Verify a signed baseline repository's index/payload hashes; create its candidate pool copy."""
import hashlib
import pathlib
import shutil
import sys

base = pathlib.Path(sys.argv[1]).resolve()
out = pathlib.Path(sys.argv[2]).resolve()
assert not out.exists(), f"refusing existing output {out}"
def digest(path):
    h = hashlib.sha256()
    with path.open('rb') as f:
        for chunk in iter(lambda: f.read(1024 * 1024), b''):
            h.update(chunk)
    return h.hexdigest()
def paragraphs(text):
    for block in text.strip().split('\n\n'):
        fields = {}
        name = None
        for line in block.splitlines():
            if line.startswith(' '):
                fields[name] += '\n' + line.strip()
            else:
                name, value = line.split(':', 1)
                fields[name] = value.strip()
        yield fields
release = next(paragraphs((base / 'dists/trixie/Release').read_text()))
checks = {}
for line in release['SHA256'].splitlines():
    if not line.strip():
        continue
    sha, size, path = line.split()
    checks[path] = (sha, int(size))
for name in ('main/binary-amd64/Packages', 'main/source/Sources'):
    path = base / 'dists/trixie' / name
    assert (digest(path), path.stat().st_size) == checks[name], f'index mismatch {name}'
seen = set()
for p in paragraphs((base / 'dists/trixie/main/binary-amd64/Packages').read_text()):
    path = base / p['Filename']
    assert path.resolve().is_relative_to(base)
    assert digest(path) == p['SHA256'] and path.stat().st_size == int(p['Size']), f'payload mismatch {path}'
    key = (p['Package'], p['Version'], p['Architecture'])
    assert key not in seen, f'duplicate package {key}'
    seen.add(key)
for p in paragraphs((base / 'dists/trixie/main/source/Sources').read_text()):
    for line in p['Checksums-Sha256'].splitlines():
        if not line.strip():
            continue
        sha, size, name = line.split()
        path = base / p['Directory'] / name
        assert path.resolve().is_relative_to(base)
        assert digest(path) == sha and path.stat().st_size == int(size), f'source mismatch {path}'
shutil.copytree(base / 'pool/main', out)
print(f'verified {len(seen)} binary index entries, source entries and all hashes; copied to {out}')
