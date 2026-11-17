"""Make a Linux test archive of a Windows Git worktree, including local edits."""

import io
import subprocess
import sys
import tarfile
from pathlib import Path


repo, destination = map(Path, sys.argv[1:])


def git(*args):
    return subprocess.check_output(["git", "-C", str(repo), *args]).split(b"\0")


files = {}
for record in filter(None, git("ls-files", "--stage", "-z")):
    info, name = record.split(b"\t", 1)
    mode, _, stage = info.split()
    if stage != b"0":
        raise SystemExit("Cannot export a worktree with unresolved conflicts")
    files[name.decode()] = mode
for name in filter(None, git("ls-files", "--others", "--exclude-standard", "-z")):
    files.setdefault(name.decode(), b"100644")

with tarfile.open(destination, "w:gz") as archive:
    for name, mode in sorted(files.items()):
        path = repo / name
        if mode == b"160000" or not (path.is_file() or path.is_symlink()):
            continue
        item = tarfile.TarInfo(name)
        if mode == b"120000":
            item.type = tarfile.SYMTYPE
            item.linkname = str(path.readlink()) if path.is_symlink() else path.read_text()
            archive.addfile(item)
            continue
        data = path.read_bytes()
        if b"\0" not in data:
            data = data.replace(b"\r\n", b"\n")
        item.size = len(data)
        item.mode = 0o755 if mode == b"100755" else 0o644
        archive.addfile(item, io.BytesIO(data))
