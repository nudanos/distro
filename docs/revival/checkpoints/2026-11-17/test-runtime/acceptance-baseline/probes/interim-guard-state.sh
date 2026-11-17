#!/bin/sh
set -u
dpkg-query -W vyatta-login vyatta-system-login-v1-yang configd
sha256sum /etc/pam.d/common-auth /config/config.boot
for u in root upgrade renew; do printf '%s ' "$u"; getent shadow "$u" | sha256sum; done
if getent passwd guarduser; then echo GUARDUSER_EXISTS; else echo GUARDUSER_ABSENT; fi
grep -E 'pam_(tacplus|sss|faillock)' /etc/pam.d/common-auth || true
echo GUARD_STATE_DONE
