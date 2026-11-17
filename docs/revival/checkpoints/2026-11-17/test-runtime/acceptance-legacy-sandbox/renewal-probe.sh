#!/bin/bash
set -euo pipefail
exec > >(tee /home/nudanos/renewal-probe.log) 2>&1
date -u
id
tty
readlink /proc/self/ns/mnt /proc/self/ns/pid /proc/self/ns/net
dpkg-query -W vyatta-password-renewal libvyatta-config2 libjansson4
sha256sum /usr/lib/x86_64-linux-gnu/security/pam_vyatta_passwd.so
grep 'pam_vyatta_passwd.so' /proc/self/mountinfo
cat /etc/pam.d/passwd /etc/pam.d/common-password
command -v passwd
test ! -e /var/tmp/LEGACY_HOST_ONLY_MARKER
echo LEGACY_BOUND_MODULE_READY
