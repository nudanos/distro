#!/usr/bin/env python3
"""Create a candidate pool from a verified baseline and four CI archives."""
import hashlib
import pathlib
import shutil
import sys
import zipfile

root = pathlib.Path(sys.argv[1]).resolve()
archives = [pathlib.Path(p).resolve() for p in sys.argv[2:]]
expected = {'configd', 'vyatta-login', 'cli-sandbox', 'vyatta-debian-pam-configs-config'}
assert len(archives) == 4
base = root / 'baseline-verified'
out = root / 'candidate-pool'
assert not out.exists(), f'refusing existing {out}'
def digest(path):
    h = hashlib.sha256()
    with path.open('rb') as f:
        for chunk in iter(lambda: f.read(1024 * 1024), b''):
            h.update(chunk)
    return h.hexdigest()
def paragraphs(path):
    text = path.read_text()
    for block in text.strip().split('\n\n'):
        fields = {}; name = None
        for line in block.splitlines():
            if line.startswith(' '):
                fields[name] += '\n' + line.strip()
            else:
                name, value = line.split(':', 1); fields[name] = value.strip()
        yield fields
shutil.copytree(root / 'verified-pool', out)
removed = {name: [] for name in expected}
for p in paragraphs(base / 'dists/trixie/main/binary-amd64/Packages'):
    source = p.get('Source', p['Package']).split()[0]
    if source in expected:
        filename = pathlib.Path(p['Filename']).name
        (out / filename).unlink()
        removed[source].append(filename)
for p in paragraphs(base / 'dists/trixie/main/source/Sources'):
    source = p['Package']
    if source in expected:
        for line in p['Checksums-Sha256'].splitlines():
            if line.strip():
                filename = line.split()[2]
                (out / filename).unlink()
                removed[source].append(filename)
assert all(removed.values()), f'missing baseline package source: {removed}'
seen = set()
for archive in archives:
    extract = root / 'verified-inputs' / archive.stem
    assert not extract.exists()
    extract.mkdir(parents=True)
    with zipfile.ZipFile(archive) as z:
        assert all(pathlib.PurePosixPath(n).name == n for n in z.namelist())
        z.extractall(extract)
    changes = list(extract.glob('*.changes'))
    assert len(changes) == 1
    fields = next(paragraphs(changes[0]))
    source = fields['Source'].split()[0]
    assert source in expected and source not in seen, source
    seen.add(source)
    added = []
    for line in fields['Checksums-Sha256'].splitlines():
        if not line.strip(): continue
        sha, size, filename = line.split()
        path = extract / filename
        assert path.is_file() and path.stat().st_size == int(size) and digest(path) == sha, filename
        if filename.endswith(('.deb', '.dsc', '.tar.gz', '.tar.xz', '.tar.bz2', '.diff.gz')):
            assert not (out / filename).exists(), filename
            shutil.copy2(path, out / filename); added.append(filename)
    assert any(n.endswith('.deb') for n in added) and any(n.endswith('.dsc') for n in added), source
    print(f'{source}: removed {len(removed[source])}, added {len(added)}')
    for n in added: print(f'  {digest(out / n)}  {n}')
assert seen == expected
