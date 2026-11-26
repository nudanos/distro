#!/bin/bash
set -euo pipefail
test "$(id -u)" = 0
test -z "$(systemctl show -p FragmentPath --value boot-grub.mount)"
root=$(mktemp -d)
chmod 0755 "$root"
base=http://10.0.2.2:18771/image-tools-5.57
cd "$root"
while read -r hash file; do
    curl -fsS "$base/$file" -o "$file"
    echo "$hash  $file" | sha256sum -c
done <<'EOF'
e889260fac1cafcac86346d701e875533c04305a73f4993f30a4bc6dd2b207e1 libvyatta-live-perl_5.57_all.deb
c599500cf44e4bc336b76e44601159306f9edd403030f16c06efa1b3930935d8 vyatta-image-tools-initramfs-tools_5.57_all.deb
fb4f626592effc4f89cf9f412f728ecebb3dc5f34564e8b57d480e7c9a6dfa88 vyatta-image-tools_5.57_all.deb
EOF
DEBIAN_FRONTEND=noninteractive apt-get -y install ./*.deb
test -z "$(dpkg --audit)"
dpkg-query -W vyatta-image-tools vyatta-image-tools-initramfs-tools libvyatta-live-perl
systemctl show -p FragmentPath -p What boot-grub.mount
echo 'Packaged image tools installed on the unpatched target'
