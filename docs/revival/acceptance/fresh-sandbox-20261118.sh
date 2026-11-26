#!/bin/bash
# Run only in the disposable upgraded baseline fixture.
set -euo pipefail
test "$(id -u)" = 0
test ! -e /var/lib/sandbox/etc/os-release
base=http://10.0.2.2:18772
key=/usr/share/keyrings/nudanos-revival-fixture.asc
test ! -e "$key"
curl -fsS "$base/nudanos-archive-keyring.asc" -o "$key"
echo "e608bec258289850f1416e05809123b7b5740e3536ef68bb4599f197eef73479  $key" | sha256sum -c
backup=$(mktemp -d /var/tmp/nudanos-fixture-apt.XXXXXXXX)
# mmdebstrap reads the live guest's sources files during package configuration.
for file in /etc/apt/sources.list /etc/apt/sources.list.d/*.list /etc/apt/sources.list.d/*.sources; do
    if [ -f "$file" ]; then mv "$file" "$backup/"; fi
done
cat >/etc/apt/sources.list.d/nudanos-revival-fixture.list <<EOF
deb [signed-by=$key] $base trixie main
deb [signed-by=/usr/share/keyrings/debian-archive-keyring.gpg] http://deb.debian.org/debian trixie main
EOF
apt-get -o Acquire::Retries=2 update
DEBIAN_FRONTEND=noninteractive apt-get -y install \
    cli-sandbox=0.33 pam-sandbox=0.33 python3-shared-storage=0.33 \
    vyatta-system-login-user-isolation-v1-yang=0.33
test -s /var/lib/sandbox/etc/os-release
test -z "$(dpkg --audit)"
chroot /var/lib/sandbox dpkg-query -W vyatta-password-renewal
grep -F pam_sandbox.so /etc/pam.d/common-session
touch /var/tmp/NUDANOS_REVIVAL_HOST_ONLY
printf 'Fresh sandbox root created. Host root and mount namespace: '
stat -c '%d:%i' /
readlink /proc/self/ns/mnt
