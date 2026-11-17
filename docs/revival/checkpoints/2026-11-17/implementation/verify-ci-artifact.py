#!/usr/bin/env python3
import hashlib, pathlib, sys, zipfile
archive = pathlib.Path(sys.argv[1]); output = pathlib.Path(sys.argv[2])
assert not output.exists()
output.mkdir(parents=True)
with zipfile.ZipFile(archive) as z:
    assert all(pathlib.PurePosixPath(n).name == n for n in z.namelist())
    z.extractall(output)
changes = list(output.glob('*.changes'))
assert len(changes) == 1
fields = {}; name = None
for line in changes[0].read_text().splitlines():
    if line.startswith(' '): fields[name] += '\n' + line.strip()
    elif line: name, value = line.split(':', 1); fields[name] = value.strip()
for line in fields['Checksums-Sha256'].splitlines():
    if not line.strip(): continue
    expected, size, filename = line.split(); path = output / filename
    assert path.is_file() and path.stat().st_size == int(size)
    assert hashlib.sha256(path.read_bytes()).hexdigest() == expected, filename
    print(expected, filename)
print('source:', fields['Source'], 'version:', fields['Version'])
