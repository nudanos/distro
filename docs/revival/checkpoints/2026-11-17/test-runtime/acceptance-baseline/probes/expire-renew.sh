#!/bin/sh
set -eu
printf 'phase=before-expiry\n'
for user in root upgrade renew; do printf '%s ' "$user"; getent shadow "$user" | sha256sum; done
id renew
chage -d 0 renew
printf 'phase=after-expiry\n'
for user in root upgrade renew; do printf '%s ' "$user"; getent shadow "$user" | sha256sum; done
chage -l renew
systemctl show -p MainPID -p ActiveState configd
sha256sum /proc/1464/exe /usr/sbin/configd
echo EXPIRE_SETUP_OK
