#!/bin/sh
set -eu
curl -fsS http://10.0.2.2:18769/nudanos-archive-keyring.asc -o /tmp/interim-key.asc
sha256sum /tmp/interim-key.asc
gpg --batch --show-keys --with-colons /tmp/interim-key.asc | grep '^fpr:'
gpg --batch --yes --dearmor -o /usr/share/keyrings/nudanos-interim.gpg /tmp/interim-key.asc
cat >/etc/apt/sources.list.d/interim.list <<'EOF'
deb [signed-by=/usr/share/keyrings/nudanos-interim.gpg] http://10.0.2.2:18769 trixie main
deb [signed-by=/usr/share/keyrings/debian-archive-keyring.gpg] http://deb.debian.org/debian trixie main
EOF
apt-get -o Dir::Etc::sourcelist=/etc/apt/sources.list.d/interim.list -o Dir::Etc::sourceparts=- update
dpkg-query -W configd vyatta-login vyatta-system-login-v1-yang vyatta-debian-pam-configs-config
systemctl show -p MainPID -p ActiveState configd
p=$(systemctl show -p MainPID --value configd)
sha256sum /proc/$p/exe /usr/sbin/configd /etc/pam.d/common-auth /config/config.boot
dpkg --audit
echo INTERIM_PREPARE_OK
