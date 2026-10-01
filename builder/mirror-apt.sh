#!/bin/bash
# Download pinned binary packages and their source packages from a third-party
# apt repository into /out.
# Env: SOURCE="URL SUITE COMPONENT", KEY_URL, PINS="pkg=version ...", HOST_UID, HOST_GID
set -euo pipefail
: "${SOURCE:?}" "${KEY_URL:?}" "${PINS:?}" "${HOST_UID:=0}" "${HOST_GID:=0}"
read -r url suite comp <<<"$SOURCE"
curl -fsSL "$KEY_URL" -o /tmp/key
if grep -q -- '-----BEGIN PGP' /tmp/key; then
    gpg --dearmor < /tmp/key > /usr/share/keyrings/mirror.gpg
else
    cp /tmp/key /usr/share/keyrings/mirror.gpg
fi
cat > /etc/apt/sources.list.d/mirror.list <<EOF
deb [signed-by=/usr/share/keyrings/mirror.gpg] $url $suite $comp
deb-src [signed-by=/usr/share/keyrings/mirror.gpg] $url $suite $comp
EOF
apt-get update --error-on=any -o Acquire::Retries=3
cd /out
declare -A seen
for pin in $PINS; do
    apt-get download "$pin"
    src=$(apt-cache show "$pin" | awk '/^Source:/ {print $2; exit}')
    src=${src:-${pin%%=*}}
    if [ -z "${seen[$src]:-}" ]; then
        seen[$src]=1
        apt-get source --download-only "$src=${pin#*=}"
    fi
done
chown -R "$HOST_UID:$HOST_GID" /out
