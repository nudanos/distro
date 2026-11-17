#!/bin/bash
set -eu
touch /var/tmp/LEGACY_HOST_ONLY_MARKER
printf 'HOST namespace/root identities\n'
readlink /proc/self/ns/mnt /proc/self/ns/pid /proc/self/ns/net
stat -c 'root dev:inode %d:%i' /
install -m 755 /dev/stdin /home/nudanos/legacy-session.sh <<'EOF'
#!/bin/bash
set -u
exec > >(tee /home/nudanos/legacy-session.log) 2>&1
date -u
id
printf 'SESSION namespace/root identities\n'
readlink /proc/self/ns/mnt /proc/self/ns/pid /proc/self/ns/net
stat -c 'root dev:inode %d:%i' /
cat /proc/self/cgroup
if test -e /var/tmp/LEGACY_HOST_ONLY_MARKER; then echo HOST_MARKER_VISIBLE; else echo HOST_MARKER_HIDDEN; fi
dpkg-query -W vyatta-password-renewal cli-sandbox 2>&1 || true
ls -l /usr/lib/x86_64-linux-gnu/security/pam_vyatta_passwd.so
echo LEGACY_SESSION_COMPLETE
EOF
chown nudanos:users /home/nudanos/legacy-session.sh
echo SESSION_PROBE_READY
