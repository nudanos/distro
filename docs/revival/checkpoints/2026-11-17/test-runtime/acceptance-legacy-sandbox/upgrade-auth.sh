#!/bin/bash
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive
curl -fsS http://10.0.2.2:18769/nudanos-archive-keyring.asc -o /tmp/interim-key.asc
test "$(gpg --batch --show-keys --with-colons /tmp/interim-key.asc | awk -F: '$1=="fpr" {print $10; exit}')" = 632DE3028B010AD7971CF88FC8FF6AE6F7B0295F
gpg --batch --yes --dearmor -o /usr/share/keyrings/nudanos-interim.gpg /tmp/interim-key.asc
cat >/etc/apt/sources.list.d/interim.list <<'EOF'
deb [signed-by=/usr/share/keyrings/nudanos-interim.gpg] http://10.0.2.2:18769 trixie main
deb [signed-by=/usr/share/keyrings/debian-archive-keyring.gpg] http://deb.debian.org/debian trixie main
EOF
aptopts=(-o Dir::Etc::sourcelist=/etc/apt/sources.list.d/interim.list -o Dir::Etc::sourceparts=-)
echo BEFORE_AUTH_UPGRADE
pid=$(systemctl show configd -p MainPID --value)
echo "CONFIGD_PID=$pid"
sha256sum /proc/$pid/exe /usr/sbin/configd /var/lib/sandbox/var/lib/dpkg/status
apt-get "${aptopts[@]}" update
apt-get "${aptopts[@]}" install -y configd=2.17 vyatta-login=2.45 vyatta-system-login-v1-yang=2.45 vyatta-password-renewal=2.45 vyatta-debian-pam-configs-config=1.13
echo AFTER_AUTH_UPGRADE
pid=$(systemctl show configd -p MainPID --value)
echo "CONFIGD_PID=$pid"
sha256sum /proc/$pid/exe /usr/sbin/configd /var/lib/sandbox/var/lib/dpkg/status
sha256sum /usr/lib/x86_64-linux-gnu/security/pam_vyatta_passwd.so /usr/lib/x86_64-linux-gnu/security/pam_sandbox.so
test ! -e /var/lib/sandbox/usr/lib/x86_64-linux-gnu/security/pam_vyatta_passwd.so
dpkg-query -W configd vyatta-login vyatta-password-renewal vyatta-debian-pam-configs-config cli-sandbox pam-sandbox
dpkg --audit
echo LEGACY_AUTH_UPGRADE_COMPLETE
