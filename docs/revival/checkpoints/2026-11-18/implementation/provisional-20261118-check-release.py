from pathlib import Path
import hashlib
base=Path('/srv/nudanos/image-step2-20261118-provisional/candidate-repo/dists/trixie')
lines=(base/'Release').read_text().splitlines(); section=False; count=0
for line in lines:
    if line=='SHA256:': section=True; continue
    if section and line and not line.startswith(' '): break
    if section and line.strip():
        expected,size,name=line.split(); p=base/name
        assert p.is_file() and p.stat().st_size==int(size) and hashlib.sha256(p.read_bytes()).hexdigest()==expected, name
        count+=1
assert count>0
print('All Release SHA256 entries verified:',count)
