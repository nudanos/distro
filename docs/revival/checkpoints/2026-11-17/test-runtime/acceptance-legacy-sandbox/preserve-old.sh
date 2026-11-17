#!/bin/bash
set -eu
for i in $(seq 1 60); do
  state=$(systemctl is-active cli-sandbox@nudanos.service || true)
  test "$state" = inactive && break
  sleep 1
done
test "$state" = inactive
systemctl show -p ActiveState -p SubState -p Result cli-sandbox@nudanos.service
journalctl -b -u cli-sandbox@nudanos.service --no-pager -n 18
sha256sum /var/lib/sandbox/var/lib/dpkg/status
dpkg --audit
dpkg-query -W configd cli-sandbox pam-sandbox vyatta-password-renewal
findmnt -rn | grep /run/cli-sandbox && exit 1 || true
echo OLDROOT_CLEAN_TEARDOWN_VERIFIED
sync
poweroff
