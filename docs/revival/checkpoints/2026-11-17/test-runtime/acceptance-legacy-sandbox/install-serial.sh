#!/bin/bash
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive
sha256sum /var/lib/sandbox/var/lib/dpkg/status
sha256sum /tmp/serial-package/*.deb
apt-get install -y /tmp/serial-package/cli-sandbox_0.33_all.deb /tmp/serial-package/pam-sandbox_0.33_amd64.deb /tmp/serial-package/python3-shared-storage_0.33_all.deb /tmp/serial-package/vyatta-system-login-user-isolation-v1-yang_0.33_all.deb
dpkg --audit
sha256sum /var/lib/sandbox/var/lib/dpkg/status
sha256sum /usr/lib/x86_64-linux-gnu/security/pam_sandbox.so
/opt/vyatta/share/tmplscripts/system/login/user_isolation.sh enable
install -m755 /dev/stdin /home/nudanos/terminal-probe.sh <<'PROBE'
#!/bin/bash
set -u
exec > >(tee /home/nudanos/terminal-probe.log) 2>&1
date -u
id
tty
ps -o pid,ppid,sid,tty,args -p $$
readlink /proc/self/ns/mnt /proc/self/ns/pid /proc/self/ns/net
stat -c 'ROOT %d:%i' /
stat -c 'ALIAS %F %t:%T %u:%g %a' /dev/ttyS0
cat /proc/self/cgroup
python3 - <<'PY'
import os,errno
try:
    fd=os.open('/dev/ttyS0',os.O_RDWR|os.O_NONBLOCK)
except OSError as e:
    assert e.errno==errno.EACCES, e
    print('UNPRIVILEGED_ALIAS_DENIED_EACCES')
else:
    os.close(fd)
    raise AssertionError('unprivileged alias open succeeded')
PY
test ! -e /var/tmp/LEGACY_HOST_ONLY_MARKER && echo HOST_MARKER_HIDDEN
echo SERIAL_TERMINAL_PROBE_COMPLETE
PROBE
chown nudanos:users /home/nudanos/terminal-probe.sh
echo SERIAL_PACKAGE_READY
