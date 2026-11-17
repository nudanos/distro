#!/bin/bash
set -u
date -u
systemctl show -p ActiveState -p SubState -p Result system-configure
dpkg --audit
sha256sum /var/lib/sandbox/var/lib/dpkg/status
findmnt -o TARGET,SOURCE,FSTYPE,OPTIONS / /boot /config
journalctl -b -k --no-pager | grep -Ei 'EXT4|recover|error|corrupt|fsck' || true
for f in /var/tmp/legacy-trace/login.trace*; do
  echo "$f"
  tail -35 "$f"
done
echo RECOVERY_CHECK_COMPLETE
