#!/bin/bash
set -euo pipefail
root=/srv/nudanos/image-step2-final
for p in /pool /repo /gnupg; do
  test -d "$p"
  ! mountpoint -q "$p"
  test -z "$(ls -A "$p")"
done
mkdir -p "$root/candidate-repo"
mount --bind "$root/candidate-pool" /pool
mount --bind "$root/candidate-repo" /repo
mount --bind /srv/nudanos/image-step2/test-gnupg /gnupg
mount -t tmpfs -o size=64m tmpfs /tmp
KEY_ID=632DE3028B010AD7971CF88FC8FF6AE6F7B0295F SUITE=trixie HOST_UID=1000 HOST_GID=1000 bash /srv/nudanos/image-step2/distro/builder/make-repo.sh
