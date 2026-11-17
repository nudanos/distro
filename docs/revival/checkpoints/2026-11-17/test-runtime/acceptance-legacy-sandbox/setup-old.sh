#!/bin/bash
set -euo pipefail
exec > >(tee /var/tmp/legacy-setup.log) 2>&1
mkdir -p /var/tmp/legacy-evidence /etc/apt/legacy-saved
date -u
dpkg-query -W configd vyatta-login vyatta-password-renewal
cp -a /etc/apt/sources.list /etc/apt/legacy-saved/ 2>/dev/null || true
for f in /etc/apt/sources.list.d/*.list /etc/apt/sources.list.d/*.sources; do
  [ -f "$f" ] || continue
  mv "$f" /etc/apt/legacy-saved/
done
test ! -f /etc/apt/sources.list || : > /etc/apt/sources.list
curl -fsS http://10.0.2.2:18767/pinned-baseline.asc -o /usr/share/keyrings/legacy-baseline.asc
cat >/etc/apt/sources.list.d/legacy-fixture.list <<'EOF'
deb [signed-by=/usr/share/keyrings/legacy-baseline.asc] http://10.0.2.2:18767/baseline-repo trixie main
deb [signed-by=/usr/share/keyrings/debian-archive-keyring.gpg] http://deb.debian.org/debian trixie main
EOF
sha256sum /usr/share/keyrings/legacy-baseline.asc
apt-get update
apt-cache policy cli-sandbox pam-sandbox vyatta-system-login-user-isolation-v1-yang
DEBIAN_FRONTEND=noninteractive apt-get install -y cli-sandbox=0.31 pam-sandbox=0.31 python3-shared-storage=0.31 vyatta-system-login-user-isolation-v1-yang=0.31
dpkg --audit
dpkg-query -W cli-sandbox pam-sandbox vyatta-system-login-user-isolation-v1-yang vyatta-password-renewal
find /var/lib/sandbox -maxdepth 2 -name os-release -o -name vbash
systemctl status 'cli-sandbox@*' --no-pager || true
grep -n pam_sandbox /etc/pam.d/common-session || true
echo LEGACY_SETUP_COMPLETE
