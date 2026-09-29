#!/bin/bash
# Create the NuDanOS archive signing key (Ed25519, no passphrase, 3 years) in a
# local GnuPG home and print its fingerprint. The private key never leaves that
# directory except when the user copies it into a CI secret.
set -euo pipefail
GNUPG=${GNUPG:-$HOME/.nudanos/gnupg}
ENGINE=${ENGINE:-docker}
mkdir -p "$GNUPG" && chmod 700 "$GNUPG"
$ENGINE run --rm -v "$GNUPG":/gnupg -e GNUPGHOME=/gnupg nudanos/builder:trixie \
    gpg --batch --passphrase '' --quick-gen-key "NuDanOS Archive Signing Key <jon@fernandez.tech>" ed25519 sign 3y
$ENGINE run --rm -v "$GNUPG":/gnupg -e GNUPGHOME=/gnupg nudanos/builder:trixie \
    gpg --list-keys --with-colons | awk -F: '/^fpr/ {print $10; exit}'
