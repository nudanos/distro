#!/bin/bash
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive
test ! -e /var/lib/sandbox/etc/os-release
mkdir -p /var/tmp/final-fresh/original-apt
cp -a /etc/apt/sources.list /var/tmp/final-fresh/original-apt/ 2>/dev/null || true
test ! -e /etc/apt/sources.list || : >/etc/apt/sources.list
for file in /etc/apt/sources.list.d/*.list /etc/apt/sources.list.d/*.sources; do
    test -f "$file" || continue
    mv "$file" /var/tmp/final-fresh/original-apt/
done
curl -fsS http://10.0.2.2:18772/nudanos-archive-keyring.asc -o /usr/share/keyrings/nudanos-final.asc
echo 'e608bec258289850f1416e05809123b7b5740e3536ef68bb4599f197eef73479  /usr/share/keyrings/nudanos-final.asc' | sha256sum -c -
test "$(gpg --batch --show-keys --with-colons /usr/share/keyrings/nudanos-final.asc | awk -F: '$1=="fpr" {print $10; exit}')" = 632DE3028B010AD7971CF88FC8FF6AE6F7B0295F
cat >/etc/apt/sources.list.d/final-acceptance.list <<'EOF'
deb [signed-by=/usr/share/keyrings/nudanos-final.asc] http://10.0.2.2:18772 trixie main
deb [signed-by=/usr/share/keyrings/debian-archive-keyring.gpg] http://deb.debian.org/debian trixie main
EOF
apt-get update
apt-get install -y vyatta-system-login-user-isolation-v1-yang=0.33
dpkg --audit
test -f /var/lib/sandbox/etc/os-release
chroot /var/lib/sandbox dpkg-query -W vyatta-password-renewal libjansson4 libvyatta-config2
test "$(chroot /var/lib/sandbox dpkg-query -W -f='${Version}' vyatta-password-renewal)" = 2.45
if chroot /var/lib/sandbox dpkg-query -W -f='${db:Status-Status}' configd 2>/dev/null | grep -qx installed; then
    echo UNEXPECTED_CONFIGD_DAEMON_IN_SANDBOX
    exit 1
fi
test ! -x /var/lib/sandbox/usr/sbin/configd
chroot /var/lib/sandbox ldd /usr/lib/x86_64-linux-gnu/security/pam_vyatta_passwd.so | tee /var/tmp/final-fresh/module-ldd.log
if grep -q 'not found' /var/tmp/final-fresh/module-ldd.log; then exit 1; fi
sha256sum /var/lib/sandbox/usr/lib/x86_64-linux-gnu/security/pam_vyatta_passwd.so /usr/lib/x86_64-linux-gnu/security/pam_vyatta_passwd.so /var/lib/sandbox/var/lib/dpkg/status
grep -n pam_sandbox /etc/pam.d/common-session
touch /var/tmp/FINAL_HOST_ONLY_MARKER
echo FINAL_FRESH_ROOT_ACTIVATED
