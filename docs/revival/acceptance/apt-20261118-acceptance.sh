#!/bin/bash
# Run only inside the disposable baseline upgrade fixture.
set -euo pipefail
test "$(id -u)" = 0
tmp=$(mktemp -d)
trap 'rm -f "$tmp/key.asc" "$tmp/InRelease" "$tmp/repo.gpg" "$tmp/sources.list"; rmdir "$tmp"' EXIT
chmod 0755 "$tmp"
base=http://10.0.2.2:18772
curl --retry 2 -fsS "$base/nudanos-archive-keyring.asc" -o "$tmp/key.asc"
echo "e608bec258289850f1416e05809123b7b5740e3536ef68bb4599f197eef73479  $tmp/key.asc" | sha256sum -c
curl --retry 2 -fsS "$base/dists/trixie/InRelease" -o "$tmp/InRelease"
echo "d5ca28d0c75f74115a996136b822b09f2dbff30961f4ad2712a3c339da9985e1  $tmp/InRelease" | sha256sum -c
gpg --batch --dearmor -o "$tmp/repo.gpg" "$tmp/key.asc"
chmod 0644 "$tmp/repo.gpg"
# APT verifies the signature using this dedicated, checksum-pinned keyring.
cat > "$tmp/sources.list" <<EOF
deb [signed-by=$tmp/repo.gpg] $base trixie main
deb [signed-by=/usr/share/keyrings/debian-archive-keyring.gpg] http://deb.debian.org/debian trixie main
EOF
apt=(apt-get -o "Dir::Etc::sourcelist=$tmp/sources.list" -o Dir::Etc::sourceparts=-)
packages=(configd=2.18 vyatta-cfg=1.36 vyatta-login=2.46 vyatta-password-renewal=2.46
    vyatta-system-login-v1-yang=2.46 vyatta-debian-pam-configs-config=1.13)
systemctl is-active --quiet configd
old_pid=$(systemctl show --value -p MainPID configd)
old_hash=$(sha256sum "/proc/$old_pid/exe" | cut -d' ' -f1)
site_file=/etc/pam.d/login
test -f "$site_file" && test ! -L "$site_file"
printf '\n# Disposable NuDanOS APT acceptance site edit\n' >> "$site_file"
pam_hash=$(sha256sum "$site_file")
printf 'Before APT: active PID=%s executable SHA256=%s\n' "$old_pid" "$old_hash"
dpkg-query -W configd vyatta-cfg vyatta-login vyatta-system-login-v1-yang
"${apt[@]}" -o Acquire::Retries=2 update
"${apt[@]}" -s install "${packages[@]}"
DEBIAN_FRONTEND=noninteractive "${apt[@]}" -y -o Dpkg::Options::=--force-confold install "${packages[@]}"
systemctl is-active --quiet configd
new_pid=$(systemctl show --value -p MainPID configd)
new_hash=$(sha256sum "/proc/$new_pid/exe" | cut -d' ' -f1)
installed_hash=$(sha256sum /usr/sbin/configd | cut -d' ' -f1)
printf 'After APT: active PID=%s executable SHA256=%s installed SHA256=%s\n' "$new_pid" "$new_hash" "$installed_hash"
test "$old_pid" != "$new_pid"
test "$old_hash" != "$new_hash"
test "$new_hash" = "$installed_hash"
test "$pam_hash" = "$(sha256sum "$site_file")"
test -z "$(dpkg --audit)"
dpkg-query -W configd vyatta-cfg vyatta-login vyatta-password-renewal vyatta-system-login-v1-yang vyatta-debian-pam-configs-config
echo 'APT acceptance passed: configd updated before reboot, site PAM file retained, package audit clean'
