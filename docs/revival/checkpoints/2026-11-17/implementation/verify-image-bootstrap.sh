#!/bin/bash
set -euo pipefail
root=/tmp/lb/chroot
check() {
  local path=$1 expected=$2 actual
  actual=$(stat -c '%u:%g:%a' "$root$path")
  printf 'bootstrap ownership %s %s\n' "$path" "$actual"
  [ "$actual" = "$expected" ] || { echo "unexpected ownership/mode: $path" >&2; exit 1; }
}
shadow_gid=$(awk -F: '$1 == "shadow" {print $3}' "$root/etc/group")
crontab_gid=$(awk -F: '$1 == "crontab" {print $3}' "$root/etc/group")
[ -n "$shadow_gid" ] && [ -n "$crontab_gid" ]
check / 0:0:755
check /etc 0:0:755
check /usr 0:0:755
check /var/lib/dpkg/status 0:0:644
check /usr/bin/dpkg 0:0:755
check /usr/bin/passwd 0:0:4755
check /usr/bin/chage "0:$shadow_gid:2755"
check /usr/bin/expiry "0:$shadow_gid:2755"
check /usr/sbin/unix_chkpwd "0:$shadow_gid:2755"
check /usr/bin/crontab "0:$crontab_gid:2755"
find "$root" -xdev -type f -perm /6000 -printf 'bootstrap setid %m %U:%G %p\n' | sort
audit=$(dpkg --root="$root" --audit)
[ -z "$audit" ] || { printf '%s\n' "$audit" >&2; exit 1; }
echo 'bootstrap ownership, setid inventory, and dpkg audit passed'
