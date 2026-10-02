#!/bin/bash
# Build the NuDanOS ISO with live-build. /repo is the signed NuDanOS repo
# (read-only), /image this live-build config (read-only), /out the output.
# Runs in the builder image from the /image mount, so the builder image (and
# with it every package's cache key) does not change when this script does.
# Env: NUDANOS_VERSION, SOURCE_DATE_EPOCH, HOST_UID, HOST_GID
# SPDX-License-Identifier: GPL-2.0-only
set -euo pipefail
: "${NUDANOS_VERSION:?}" "${SOURCE_DATE_EPOCH:?}" "${HOST_UID:=0}" "${HOST_GID:=0}"
export SOURCE_DATE_EPOCH
apt-get update --error-on=any -qq -o Acquire::Retries=3
apt-get install -y -qq --no-install-recommends live-build python3 >/dev/null
# Serve the repo to the image chroot (it shares this network namespace).
( cd /repo && exec python3 -m http.server 8080 --bind 127.0.0.1 >/dev/null 2>&1 ) &
cp -a /image /tmp/lb && cd /tmp/lb
rm -f make-image.sh
cp /repo/nudanos-archive-keyring.asc config/archives/nudanos.key.chroot
NUDANOS_VERSION="$NUDANOS_VERSION" lb config
lb build
iso=$(ls -1 /tmp/lb/*.hybrid.iso | head -1)
cp "$iso" "/out/nudanos-${NUDANOS_VERSION}-amd64.iso"
chown "$HOST_UID:$HOST_GID" "/out/nudanos-${NUDANOS_VERSION}-amd64.iso"
echo "image: /out/nudanos-${NUDANOS_VERSION}-amd64.iso"
