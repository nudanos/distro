#!/bin/sh
set -eu
cp -p /config/config.boot /run/expired-before.config
chmod 600 /run/expired-before.config
sha256sum /config/config.boot /run/expired-before.config
for u in root upgrade renew; do printf '%s shadow ' "$u"; getent shadow "$u" | sha256sum; done
chage -d 0 renew
printf 'AFTER_EXPIRY\n'
for u in root upgrade renew; do printf '%s shadow ' "$u"; getent shadow "$u" | sha256sum; done
chage -l renew
echo EXPIRED_FIXTURE_READY
