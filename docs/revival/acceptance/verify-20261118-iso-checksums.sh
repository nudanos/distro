#!/bin/bash
set -euo pipefail
iso=${1:?Specify the audited test ISO}
work=$(mktemp -d)
trap 'if mountpoint -q "$work"; then umount "$work"; fi; rmdir "$work"' EXIT
mount -o loop,ro "$iso" "$work"
(
    cd "$work"
    for manifest in sha256sum.txt md5sum.txt; do
        test -s "$manifest" || { echo "Missing $manifest" >&2; exit 1; }
        grep -Eq ' [ *](\./)?live/filesystem\.squashfs$' "$manifest" ||
            { echo "Missing root filesystem checksum in $manifest" >&2; exit 1; }
    done
    sha256sum -c --quiet sha256sum.txt
    md5sum -c --quiet md5sum.txt
)
echo 'Both ISO manifests cover the root filesystem and all listed checksums match'
