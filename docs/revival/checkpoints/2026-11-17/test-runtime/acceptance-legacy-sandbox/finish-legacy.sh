#!/bin/bash
set -euo pipefail
bash /tmp/terminal-teardown.sh
echo AUTHENTICATION_RESULTS
journalctl -u ssh --since '2026-11-09 18:48:00 UTC' --no-pager | grep -E 'Failed password|Accepted password|session opened|session closed' || true
echo RUNNING_DAEMON
pid=$(systemctl show configd -p MainPID --value)
echo "CONFIGD_PID=$pid"
sha256sum /proc/$pid/exe /usr/sbin/configd
echo BOUND_MODULE_SESSION_PROBE
cat /home/nudanos/renewal-probe.log
echo OLDROOT_PACKAGE_INVENTORY
sha256sum /var/lib/sandbox/var/lib/dpkg/status
test ! -e /var/lib/sandbox/usr/lib/x86_64-linux-gnu/security/pam_vyatta_passwd.so
dpkg --audit
echo LEGACY_FINAL_GATE_COMPLETE
sync
systemctl poweroff
