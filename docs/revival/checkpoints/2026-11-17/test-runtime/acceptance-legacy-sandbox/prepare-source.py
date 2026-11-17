import hashlib, io, json, subprocess, tarfile
from pathlib import Path
base = Path(__file__).resolve().parents[2]
repo = base / 'cli-sandbox'
out = Path(__file__).parent
files = subprocess.check_output(['git', 'ls-files', '-z', '--cached', '--others', '--exclude-standard'], cwd=repo).decode().split('\0')
modes = {}
for entry in subprocess.check_output(['git', 'ls-files', '-s', '-z'], cwd=repo).decode().split('\0'):
    if entry:
        header, path = entry.split('\t', 1)
        modes[path] = int(header.split()[0], 8) & 0o777
manifest = {}
with tarfile.open(out/'cli-sandbox-source.tar', 'w') as archive:
    for name in sorted(set(files)-{''}):
        data = (repo/name).read_bytes().replace(b'\r\n', b'\n')
        item = tarfile.TarInfo('cli-sandbox/'+name)
        item.size = len(data); item.mode = modes.get(name, 0o755 if name.endswith('.sh') else 0o644)
        archive.addfile(item, io.BytesIO(data))
        manifest[name] = hashlib.sha256(data).hexdigest()
(out/'source-manifest.json').write_text(json.dumps(manifest, indent=2)+'\n')
print(hashlib.sha256((out/'cli-sandbox-source.tar').read_bytes()).hexdigest())
