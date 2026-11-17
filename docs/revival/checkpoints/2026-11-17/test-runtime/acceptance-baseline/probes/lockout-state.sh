#!/bin/sh
set -u
printf 'CONFIG_BOOT_AUTO_DISABLE\n'
grep -A5 -B2 'auto-disable' /config/config.boot || true
printf 'PAM_COMMON_AUTH\n'
cat /etc/pam.d/common-auth
printf 'PAM_MANAGED_PROFILES\n'
ls -l /usr/share/pam-configs/vyatta* 2>&1
grep -E 'faillock|tally' /var/lib/pam/auth 2>&1 || true
printf 'LOCKTEST_COUNTER\n'
/usr/sbin/faillock --user locktest 2>&1 || true
systemctl show -p ActiveState -p SubState -p Result system-configure
echo LOCKOUT_STATE_DONE
