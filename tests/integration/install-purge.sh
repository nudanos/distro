#!/bin/bash
# Layer 2 (spec 8.2): in a clean Debian 13 container, install nudanos-router
# from /repo, remove and purge everything it brought, and fail on any
# maintainer-script error or on files left outside the allowlist.
set -euo pipefail
snapshot() {
    find / -xdev \( -path /proc -o -path /sys -o -path /dev -o -path /run -o -path /tmp \
        -o -path /var/cache -o -path /var/lib/apt -o -path /var/lib/dpkg -o -path /var/log \
        -o -path /repo -o -path /tests \) -prune -o -print | sort
}
apt-get update -qq && apt-get install -y -qq ca-certificates gpg >/dev/null
gpg --dearmor < /repo/nudanos-archive-keyring.asc > /usr/share/keyrings/nudanos.gpg
echo "deb [signed-by=/usr/share/keyrings/nudanos.gpg] file:/repo trixie main" > /etc/apt/sources.list.d/nudanos.list
apt-get update -qq
dpkg-query -W -f='${Package}\n' | sort > /tmp/pkgs-before
snapshot > /tmp/files-before
DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends nudanos-router
dpkg-query -W -f='${Package}\n' | sort > /tmp/pkgs-after
comm -13 /tmp/pkgs-before /tmp/pkgs-after > /tmp/pkgs-added
DEBIAN_FRONTEND=noninteractive apt-get purge -y $(cat /tmp/pkgs-added)
snapshot > /tmp/files-after
comm -13 /tmp/files-before /tmp/files-after > /tmp/left
grep -v -E -f <(grep -v '^#' /tests/install-allowlist.txt | awk 'NF{print $1}') /tmp/left > /tmp/unexplained || true
if [ -s /tmp/unexplained ]; then
    echo "files left after purge:"; cat /tmp/unexplained; exit 1
fi
echo "install-purge: OK ($(wc -l < /tmp/pkgs-added) packages)"
