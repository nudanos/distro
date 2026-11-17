#!/bin/bash
set -euo pipefail
date -u
cat /etc/os-release
cat /proc/cmdline
systemctl show system-configure -p ActiveState -p SubState -p Result -p ExecMainStatus -p ExecMainExitTimestamp
test "$(systemctl show system-configure -p ActiveState --value)" = active
test "$(systemctl show system-configure -p Result --value)" = success
dpkg-query -W configd vyatta-login vyatta-password-renewal vyatta-debian-pam-configs-config cli-sandbox pam-sandbox whois
test "$(dpkg-query -W -f='${Version}' configd)" = 2.17
test "$(dpkg-query -W -f='${Version}' vyatta-login)" = 2.45
test "$(dpkg-query -W -f='${Version}' vyatta-password-renewal)" = 2.45
test "$(dpkg-query -W -f='${Version}' cli-sandbox)" = 0.33
test "$(dpkg-query -W -f='${Version}' pam-sandbox)" = 0.33
test "$(dpkg-query -W -f='${Version}' vyatta-debian-pam-configs-config)" = 1.13
if dpkg-query -W -f='${db:Status-Status}' vyatta-system-login-user-isolation-v1-yang 2>/dev/null | grep -qx installed; then
    echo UNEXPECTED_OPTIONAL_MODEL_INSTALLED
    exit 1
fi
test ! -e /var/lib/sandbox/etc/os-release
if grep -Eq '^[[:space:]]*session.*pam_sandbox\.so' /etc/pam.d/common-session; then
    echo UNEXPECTED_STOCK_ISOLATION_ACTIVE
    exit 1
fi
sha256sum /etc/pam.d/common-* /usr/lib/x86_64-linux-gnu/security/pam_vyatta_passwd.so /usr/lib/x86_64-linux-gnu/security/pam_sandbox.so
grep -nE '^[[:space:]]*[^#[:space:]]' /etc/pam.d/login /etc/pam.d/sshd /etc/pam.d/common-*
dpkg --audit
echo FINAL_STOCK_STATE_VERIFIED
