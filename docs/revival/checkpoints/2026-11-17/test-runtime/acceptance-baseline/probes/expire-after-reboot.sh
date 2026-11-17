#!/bin/sh
set -eu
sha256sum /config/config.boot
for user in root upgrade renew; do printf '%s ' "$user"; getent shadow "$user" | sha256sum; done
chage -d 0 renew
printf 'after-expiry\n'
for user in root upgrade renew; do printf '%s ' "$user"; getent shadow "$user" | sha256sum; done
chage -l renew
echo REBOOT_EXPIRY_READY
