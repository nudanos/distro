#!/bin/bash
set -uo pipefail
exec > >(tee /var/tmp/legacy-old-inventory.log) 2>&1
date -u
id
dpkg --audit
dpkg-query -W configd cli-sandbox pam-sandbox vyatta-password-renewal libpam0g libjansson4
dpkg-query -L pam-sandbox | grep '\.so'
ls -l /usr/lib/security/pam_sandbox.so /usr/lib/x86_64-linux-gnu/security/pam_sandbox.so
ldd /usr/lib/security/pam_sandbox.so
systemctl cat cli-sandbox@.service
cat /etc/cli-sandbox/nspawn-templates/cli_sandbox.nspawn
find /var/lib/sandbox -name pam_vyatta_passwd.so -o -name libjansson.so.4 -o -name vbash
chroot /var/lib/sandbox dpkg-query -W vyatta-password-renewal libvyatta-config2 libjansson4
sha256sum /usr/lib/x86_64-linux-gnu/security/pam_vyatta_passwd.so /var/lib/sandbox/var/lib/dpkg/status
grep -n pam_sandbox /etc/pam.d/common-session
cat /proc/self/loginuid
echo OLD_INVENTORY_COMPLETE
