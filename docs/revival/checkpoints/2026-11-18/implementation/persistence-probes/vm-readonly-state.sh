#!/bin/sh
set -eu
date -u
dpkg-query -W configd vyatta-login vyatta-system-login-v1-yang
systemctl show -p MainPID -p ActiveState configd
p=$(systemctl show -p MainPID --value configd)
sha256sum /proc/$p/exe /usr/sbin/configd
sha256sum /opt/vyatta/sbin/vyatta-password-rpc /opt/vyatta/sbin/vyatta-password-rpc.original
findmnt -T /config/config.boot -o TARGET,SOURCE,FSTYPE,OPTIONS -n
stat -c 'STARTUP_FILE dev=%d inode=%i uid=%u gid=%g mode=%a' /config/config.boot
sha256sum /config/config.boot
echo READONLY_STATE_DONE
