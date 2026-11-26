#!/bin/bash
# Run only on a disposable child of the preserved legacy sandbox fixture.
set -euo pipefail
test "$(id -u)" = 0
root=/var/lib/sandbox
root_status="$root/var/lib/dpkg/status"
old_root=$(sha256sum "$root_status")
module=/usr/lib/x86_64-linux-gnu/security/pam_vyatta_passwd.so
test ! -e "$root$module"
dpkg-query --admindir="$root/var/lib/dpkg" -W vyatta-password-renewal libvyatta-config2
tmp=$(mktemp -d)
trap 'rm -f "$tmp/key.asc" "$tmp/repo.gpg" "$tmp/InRelease" "$tmp/sources.list"; rmdir "$tmp"' EXIT
chmod 0755 "$tmp"
base=http://10.0.2.2:18772
curl -fsS "$base/nudanos-archive-keyring.asc" -o "$tmp/key.asc"
echo "e608bec258289850f1416e05809123b7b5740e3536ef68bb4599f197eef73479  $tmp/key.asc" | sha256sum -c
curl -fsS "$base/dists/trixie/InRelease" -o "$tmp/InRelease"
echo "d5ca28d0c75f74115a996136b822b09f2dbff30961f4ad2712a3c339da9985e1  $tmp/InRelease" | sha256sum -c
gpg --batch --dearmor -o "$tmp/repo.gpg" "$tmp/key.asc"
chmod 0644 "$tmp/repo.gpg"
cat >"$tmp/sources.list" <<EOF
deb [signed-by=$tmp/repo.gpg] $base trixie main
deb [signed-by=/usr/share/keyrings/debian-archive-keyring.gpg] http://deb.debian.org/debian trixie main
EOF
apt=(apt-get -o "Dir::Etc::sourcelist=$tmp/sources.list" -o Dir::Etc::sourceparts=-)
"${apt[@]}" -o Acquire::Retries=2 update
DEBIAN_FRONTEND=noninteractive "${apt[@]}" -y install \
    configd=2.18 vyatta-cfg=1.36 vyatta-login=2.46 vyatta-password-renewal=2.46 \
    vyatta-system-login-v1-yang=2.46 vyatta-debian-pam-configs-config=1.13 \
    cli-sandbox=0.33 pam-sandbox=0.33 python3-shared-storage=0.33 \
    vyatta-system-login-user-isolation-v1-yang=0.33
test "$old_root" = "$(sha256sum "$root_status")"
test ! -e "$root$module"
test -f "$module"
test -z "$(dpkg --audit)"
dpkg-query -W configd vyatta-cfg vyatta-login pam-sandbox vyatta-system-login-user-isolation-v1-yang
test -e /var/tmp/LEGACY_HOST_ONLY_MARKER
printf 'Legacy root unchanged. Host root and mount namespace: '
stat -c '%d:%i' /
readlink /proc/self/ns/mnt
