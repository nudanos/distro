#!/bin/sh
set -u
ps -o pid,uid,euid,gid,egid,args -C provisiond
for p in $(pidof provisiond); do grep -E '^(Name|Uid|Gid|CapEff):' /proc/$p/status; done
stat -c '%U:%G %a %F %n' /config /config/config.boot
systemctl cat provisiond
echo PROVISIOND_IDENTITY_DONE
