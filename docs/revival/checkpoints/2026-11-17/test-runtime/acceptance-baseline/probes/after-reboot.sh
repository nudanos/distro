#!/bin/sh
set -eu
systemctl show -p ActiveState -p SubState -p Result system-configure
systemctl show -p MainPID -p ActiveState configd
p=$(systemctl show -p MainPID --value configd)
readlink /proc/$p/exe
sha256sum /proc/$p/exe /usr/sbin/configd /config/config.boot /etc/pam.d/login
for user in root upgrade renew; do printf '%s ' "$user"; getent shadow "$user" | sha256sum; done
chage -l renew
dpkg --audit
echo POST_REBOOT_INSPECTED
