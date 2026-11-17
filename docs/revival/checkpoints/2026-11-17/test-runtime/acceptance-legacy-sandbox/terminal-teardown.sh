#!/bin/bash
set -euo pipefail
for unit in cli-sandbox@nudanos cli-sandbox@serialtwo; do
    for attempt in $(seq 1 30); do
        state=$(systemctl show "$unit" -p ActiveState --value)
        test "$state" = inactive && break
        sleep 1
    done
    systemctl show "$unit" -p ActiveState -p SubState -p Result
    test "$state" = inactive
done
echo RUNTIME_MOUNTS
findmnt -rn -o TARGET | grep '/run/cli-sandbox' && exit 1
sha256sum /var/lib/sandbox/var/lib/dpkg/status
sha256sum /usr/lib/x86_64-linux-gnu/security/pam_sandbox.so
dpkg --audit
cat /home/nudanos/terminal-probe.log
echo SERIAL_TEARDOWN_COMPLETE
