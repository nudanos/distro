import hashlib
import io
import tarfile
from pathlib import Path

here = Path(__file__).resolve().parent
source = here.parents[1] / 'vyatta-login'
archive = here / 'http/provider-source.tar'
paths = ['scripts/system-login-information', 't/system-login-information.t',
         't/lib/LoginProviderFixture.pm']
with tarfile.open(archive, 'w') as output:
    for name in paths:
        content = (source / name).read_bytes().replace(b'\r\n', b'\n')
        entry = tarfile.TarInfo('provider-source/' + name)
        entry.size = len(content)
        entry.mode = 0o755 if name.startswith('scripts/') else 0o644
        output.addfile(entry, io.BytesIO(content))
        print(name, hashlib.sha256(content).hexdigest())
print('archive', hashlib.sha256(archive.read_bytes()).hexdigest())
