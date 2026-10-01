#!/bin/bash
# Copy the NuDanOS archive signing key into the nudanos/distro `release`
# environment secret. gpg runs inside the builder image (it is not installed on
# the Mac); the key goes from ~/.nudanos/gnupg straight into GitHub.
set -euo pipefail
FPR=44235C5FF45218554D862EAABA18191EBBD0A857
# gpg-agent cannot create its socket on the macOS bind mount, so work on a copy
# inside the container (as builder/make-repo.sh does); the mount is read-only.
key=$(docker run --rm -v "$HOME/.nudanos/gnupg:/gnupg:ro" nudanos/builder:trixie bash -c \
    'mkdir -m 700 /tmp/gnupg && cd /gnupg && find . -type d -exec mkdir -p /tmp/gnupg/{} \; &&
     find . -type f -exec cp -p {} /tmp/gnupg/{} \; &&
     GNUPGHOME=/tmp/gnupg gpg --batch --armor --export-secret-keys '"$FPR")
case "$key" in
    "-----BEGIN PGP PRIVATE KEY BLOCK-----"*) ;;
    *) echo "export produced no private key; secret NOT changed" >&2; exit 1 ;;
esac
printf '%s\n' "$key" | gh secret set NUDANOS_ARCHIVE_KEY --env release -R nudanos/distro
echo "key length: ${#key} characters (a key is a few hundred; 0 means empty)"
