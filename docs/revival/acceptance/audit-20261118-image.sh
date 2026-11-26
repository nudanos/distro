#!/bin/bash
set -euo pipefail
base=/srv/nudanos/image-step2-20261118-provisional
iso=${1:-$base/image-out/nudanos-1.0~20261118.1-amd64.iso}
work=$(mktemp -d "$base/audit.XXXXXXXX")
trap 'for dir in candidate-root baseline-root candidate-iso baseline-iso; do if mountpoint -q "$work/$dir"; then umount "$work/$dir"; fi; done' EXIT
mkdir "$work"/{candidate-root,baseline-root,candidate-iso,baseline-iso}
mount -o loop,ro /srv/nudanos/baseline-20261103.iso "$work/baseline-iso"
mount -o loop,ro "$iso" "$work/candidate-iso"
for image in baseline candidate; do
    root="$work/$image-root"
    mount -o loop,ro "$work/$image-iso/live/filesystem.squashfs" "$root"
    # Match groups by name because dynamically assigned IDs can differ.
    find "$root" -xdev -type f -perm /6000 -printf '%m %U:%G %P\n' |
        awk -v groups="$root/etc/group" 'BEGIN {while ((getline < groups) > 0) {split($0,g,":"); name[g[3]]=g[1]}} {split($2,ids,":"); $2=ids[1] ":" ((ids[2] in name) ? name[ids[2]] : ids[2]); print}' |
        LC_ALL=C sort > "$work/$image-setid.txt"
    getcap -r "$root" | sed "s|$root/||" | LC_ALL=C sort > "$work/$image-capabilities.txt"
done
diff -u "$work/baseline-setid.txt" "$work/candidate-setid.txt"
diff -u "$work/baseline-capabilities.txt" "$work/candidate-capabilities.txt"
root="$work/candidate-root"
check() {
    actual=$(stat -c '%u:%g:%a' "$root$1")
    printf '%s %s\n' "$1" "$actual"
    test "$actual" = "$2"
}
gid() { awk -F: -v name="$1" '$1 == name {print $3; found=1} END {exit !found}' "$root/etc/group"; }
shadow=$(gid shadow)
crontab=$(gid crontab)
check / 0:0:755
check /etc 0:0:755
check /usr 0:0:755
check /var/lib/dpkg/status 0:0:644
check /usr/bin/passwd 0:0:4755
check /usr/bin/chage "0:$shadow:2755"
check /usr/bin/expiry "0:$shadow:2755"
check /usr/sbin/unix_chkpwd "0:$shadow:2755"
check /usr/bin/crontab "0:$crontab:2755"
check /opt/vyatta/sbin/vyatta-password-rpc 0:0:755
check /opt/vyatta/sbin/vyatta_password_persistence.py 0:0:644
sha256sum "$root"/opt/vyatta/sbin/{vyatta-password-rpc,vyatta_password_persistence.py}
test -z "$(find "$root/etc" -xdev -type d -gid 0 -perm -0020 -printf '%P\n')"
test -z "$(dpkg --root="$root" --audit)"
dpkg-query --admindir="$root/var/lib/dpkg" -W -f='${Package} ${Version} ${db:Status-Status}\n' \
    configd vyatta-cfg vyatta-login vyatta-password-renewal vyatta-system-login-v1-yang \
    vyatta-debian-pam-configs-config cli-sandbox pam-sandbox python3-shared-storage
if dpkg-query --admindir="$root/var/lib/dpkg" -W -f='${db:Status-Status}' \
    vyatta-system-login-user-isolation-v1-yang 2>/dev/null | grep -qx installed; then
    echo 'Unexpected active-isolation YANG package' >&2
    exit 1
fi
printf 'Available audit inventories: %s\n' "$work"
