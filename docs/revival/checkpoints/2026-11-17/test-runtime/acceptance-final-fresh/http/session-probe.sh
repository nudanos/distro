#!/bin/bash
set -euo pipefail
exec > >(tee "$HOME/final-session-probe.log") 2>&1
date -u
id
tty
readlink /proc/self/ns/mnt /proc/self/ns/pid /proc/self/ns/net
stat -c 'ROOT %d:%i' /
cat /proc/self/cgroup
test ! -e /var/tmp/FINAL_HOST_ONLY_MARKER
echo HOST_MARKER_HIDDEN
dpkg-query -W vyatta-password-renewal libjansson4 libvyatta-config2
test "$(dpkg-query -W -f='${Version}' vyatta-password-renewal)" = 2.45
test ! -x /usr/sbin/configd
sha256sum /usr/lib/x86_64-linux-gnu/security/pam_vyatta_passwd.so
if grep -F ' /usr/lib/x86_64-linux-gnu/security/pam_vyatta_passwd.so ' /proc/self/mountinfo; then
    echo UNEXPECTED_HOST_MODULE_BIND_ON_FRESH_ROOT
    exit 1
fi
grep -E '/run/vyatta/configd|/home|/etc/(passwd|shadow|group)' /proc/self/mountinfo
if test -e /dev/ttyS0; then
    stat -c 'ALIAS %F %t:%T %u:%g %a' /dev/ttyS0
    python3 - <<'PY'
import os,errno
try:
    fd=os.open('/dev/ttyS0',os.O_RDWR|os.O_NONBLOCK)
except OSError as e:
    assert e.errno==errno.EACCES, e
    print('UNPRIVILEGED_ALIAS_DENIED_EACCES')
else:
    os.close(fd)
    raise AssertionError('unprivileged terminal alias open succeeded')
PY
fi
echo FINAL_ISOLATED_SESSION_VERIFIED
