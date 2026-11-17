#!/bin/sh
set -eu
for user in root upgrade renew; do printf '%s ' "$user"; getent shadow "$user" | sha256sum; done
chage -l renew
sha256sum /config/config.boot /etc/pam.d/login
systemctl show -p MainPID -p ActiveState configd
sha256sum /proc/1464/exe /usr/sbin/configd
echo AFTER_RENEW_OK
