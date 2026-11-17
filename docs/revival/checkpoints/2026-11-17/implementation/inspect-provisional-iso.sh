#!/bin/bash
set -euo pipefail
base=/srv/nudanos/image-step2
for name in verify-baseline verify-provisional; do
  root="$base/$name/root"
  echo "=== $name ==="
  sudo dpkg-query --admindir="$root/var/lib/dpkg" -W -f='${Package} ${Version} ${db:Status-Status}\n' configd vyatta-login vyatta-password-renewal vyatta-debian-pam-configs-config cli-sandbox pam-sandbox python3-shared-storage 2>/dev/null || :
  sudo find "$root" -xdev -type f -perm /6000 -printf '%m %U:%G %p\n' | sed "s|$root||" | sort
  sudo getcap -r "$root" 2>/dev/null | sed "s|$root||" | sort
  sudo find "$root/etc" -xdev -type d -gid 0 -perm -0020 -printf '%m %U:%G %p\n' | sed "s|$root||" | sort
 done
