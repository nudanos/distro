#!/bin/bash
# Index every .deb and source package under /pool into a signed apt repository in /repo.
# Env: KEY_ID (fingerprint in /gnupg), SUITE, HOST_UID, HOST_GID
set -euo pipefail
: "${KEY_ID:?}" "${SUITE:=trixie}" "${HOST_UID:=0}" "${HOST_GID:=0}"
comp=main arch=amd64
rm -rf /repo/dists /repo/pool
mkdir -p "/repo/pool/$comp" "/repo/dists/$SUITE/$comp/binary-$arch" "/repo/dists/$SUITE/$comp/source"
find /pool -type f \( -name '*.deb' -o -name '*.dsc' -o -name '*.tar.*' -o -name '*.diff.gz' \) \
    -exec cp -t "/repo/pool/$comp" {} +
cd /repo
apt-ftparchive packages "pool/$comp" > "dists/$SUITE/$comp/binary-$arch/Packages"
apt-ftparchive sources "pool/$comp" > "dists/$SUITE/$comp/source/Sources"
for f in "dists/$SUITE/$comp/binary-$arch/Packages" "dists/$SUITE/$comp/source/Sources"; do
    gzip -9kf "$f"
    xz -kf "$f"
done
apt-ftparchive \
    -o APT::FTPArchive::Release::Origin=NuDanOS \
    -o APT::FTPArchive::Release::Label=NuDanOS \
    -o APT::FTPArchive::Release::Suite="$SUITE" \
    -o APT::FTPArchive::Release::Codename="$SUITE" \
    -o APT::FTPArchive::Release::Architectures="$arch" \
    -o APT::FTPArchive::Release::Components="$comp" \
    release "dists/$SUITE" > /tmp/Release
mv /tmp/Release "dists/$SUITE/Release"
cp -r /gnupg /tmp/gnupg && chmod 700 /tmp/gnupg
export GNUPGHOME=/tmp/gnupg
gpg --batch --yes --default-key "$KEY_ID" --clearsign -o "dists/$SUITE/InRelease" "dists/$SUITE/Release"
gpg --batch --yes --default-key "$KEY_ID" --armor --detach-sign -o "dists/$SUITE/Release.gpg" "dists/$SUITE/Release"
gpg --armor --export "$KEY_ID" > nudanos-archive-keyring.asc
chown -R "$HOST_UID:$HOST_GID" /repo
