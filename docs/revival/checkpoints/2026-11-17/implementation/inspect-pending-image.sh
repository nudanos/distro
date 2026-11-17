#!/bin/bash
set -euo pipefail
base=/srv/nudanos/image-step2-final
root="$base/verify/root"
baseline=/srv/nudanos/image-step2/verify-baseline/root
sudo mkdir -p /srv/nudanos/image-step2/verify-baseline/iso "$baseline"
sudo mount -o loop,ro /srv/nudanos/baseline-20261103.iso /srv/nudanos/image-step2/verify-baseline/iso
sudo mount -o loop,ro /srv/nudanos/image-step2/verify-baseline/iso/live/filesystem.squashfs "$baseline"
for name in baseline candidate; do
  if [ "$name" = baseline ]; then r=$baseline; else r=$root; fi
  sudo find "$r" -xdev -type f -perm /6000 -printf '%m %U:%G %p\n' | sed "s|$r||" | sort > "$base/$name-setid.txt"
  sudo getcap -r "$r" 2>/dev/null | sed "s|$r||" | sort > "$base/$name-capabilities.txt"
done
diff -u "$base/baseline-setid.txt" "$base/candidate-setid.txt"
diff -u "$base/baseline-capabilities.txt" "$base/candidate-capabilities.txt"
sudo dpkg-query --admindir="$root/var/lib/dpkg" -W -f='${Package} ${Version} ${db:Status-Status}\n' configd vyatta-login vyatta-password-renewal vyatta-debian-pam-configs-config cli-sandbox pam-sandbox python3-shared-storage
if sudo dpkg-query --admindir="$root/var/lib/dpkg" -W -f='${db:Status-Status}' vyatta-system-login-user-isolation-v1-yang 2>/dev/null | grep -qx installed; then
  echo 'unexpected installed isolation YANG package' >&2; exit 1
fi
sudo find "$root/etc" -xdev -type d -gid 0 -perm -0020 -printf '%m %U:%G %p\n' > "$base/group-writable-etc.txt"
test ! -s "$base/group-writable-etc.txt"
echo 'baseline setid and capabilities match; isolation YANG absent; no root-group-writable /etc directories'
