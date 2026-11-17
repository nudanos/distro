#!/bin/bash
set -euo pipefail
work=/home/builder/task3-dpkg-upgrade-fixture
mkdir -p "$work"/{root,old-login/DEBIAN,old-renewal/DEBIAN}
mkdir -p "$work/root/var/lib/dpkg" "$work/old-login/usr/lib/x86_64-linux-gnu/security"
printf 'old module\n' > "$work/old-login/usr/lib/x86_64-linux-gnu/security/pam_vyatta_passwd.so"
cat > "$work/old-login/DEBIAN/control" <<'EOF'
Package: vyatta-login
Version: 2.43
Architecture: amd64
Maintainer: fixture <fixture@example.invalid>
Description: Disposable old login package fixture
EOF
cat > "$work/old-renewal/DEBIAN/control" <<'EOF'
Package: vyatta-password-renewal
Version: 2.43
Architecture: all
Maintainer: fixture <fixture@example.invalid>
Description: Disposable old renewal package fixture
EOF
dpkg-deb --build "$work/old-login" "$work/old-login.deb"
dpkg-deb --build "$work/old-renewal" "$work/old-renewal.deb"
dpkg --root="$work/root" --force-depends --unpack "$work/old-login.deb" "$work/old-renewal.deb"
dpkg --root="$work/root" --force-depends --auto-deconfigure --unpack /home/builder/task3-login-layout-v3/vyatta-login_2.44_amd64.deb /home/builder/task3-login-layout-v3/vyatta-password-renewal_2.44_amd64.deb
dpkg --root="$work/root" -S /usr/lib/x86_64-linux-gnu/security/pam_vyatta_passwd.so
dpkg --root="$work/root" -S /opt/vyatta/share/perl5/Vyatta/Login/Faillock.pm
