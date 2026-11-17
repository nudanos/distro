#!/bin/bash
# Run as root inside a dedicated private mount namespace on Debian.
set -euo pipefail
if [ "$(readlink /proc/self/ns/mnt)" = "$(readlink /proc/1/ns/mnt)" ]; then
    echo 'Run with sudo unshare -m --propagation private bash' >&2
    exit 2
fi
repo=$(cd "$(dirname "$0")/.." && pwd)
fixture=$(mktemp -d)
mount --make-rprivate /
mkdir -p "$fixture"/{pam,profiles,dpkg,bin}
touch "$fixture/dpkg/status"
mount --bind "$fixture/pam" /etc/pam.d
mount --bind "$fixture/profiles" /usr/share/pam-configs
mount --bind "$fixture/dpkg" /var/lib/dpkg
export PATH="$fixture/bin:$PATH"
# No managed auth state should be touched by this diversion test.
printf '#!/bin/sh\nexit 0\n' > "$fixture/bin/pam-auth-update"
chmod +x "$fixture/bin/pam-auth-update"
for svc in atd cron login sshd; do
    printf '# Distribution and admin config for %s\n' "$svc" > "/etc/pam.d/$svc"
    cp "$repo/etc/pam.d/$svc.vyatta" "/etc/pam.d/$svc.vyatta"
done
printf 'Name: Auditd\nDefault: yes\n' >/usr/share/pam-configs/auditd
printf 'password required pam_permit.so\npassword optional pam_succeed_if.so quiet uid > 1000\n@include common-password\n' >/etc/pam.d/passwd
cp /etc/pam.d/passwd "$fixture/passwd.expected"
cp /etc/pam.d/login "$fixture/login.expected"
cp /usr/share/pam-configs/auditd "$fixture/auditd.expected"

sh "$repo/debian/preinst" install
sh "$repo/debian/postinst" configure
test -L /etc/pam.d/login
cmp "$fixture/auditd.expected" /usr/share/pam-configs/auditd
test -z "$(dpkg-divert --listpackage /usr/share/pam-configs/auditd)"
cmp "$fixture/passwd.expected" /etc/pam.d/passwd
echo '# Admin edit while installed' >> /etc/pam.d/passwd
echo '# Admin edit while installed' >> "$fixture/passwd.expected"
rm /etc/pam.d/sshd
ln -s /etc/pam.d/site-sshd /etc/pam.d/sshd
echo '# Site-specific sshd' > /etc/pam.d/site-sshd
# A failed upgrade must retain the previous configured service policies.
sh "$repo/debian/postrm" abort-upgrade
test -L /etc/pam.d/login
# Simulate the previous package's recorded auditd diversion for removal.
dpkg-divert --package vyatta-debian-pam-configs-config --add --rename \
    --divert /usr/share/pam-configs/auditd.debian /usr/share/pam-configs/auditd
sh "$repo/debian/postrm" remove
cmp "$fixture/passwd.expected" /etc/pam.d/passwd
cmp "$fixture/login.expected" /etc/pam.d/login
cmp "$fixture/auditd.expected" /usr/share/pam-configs/auditd
test "$(readlink /etc/pam.d/sshd)" = /etc/pam.d/site-sshd
test -f /etc/pam.d/sshd.debian
test -z "$(dpkg-divert --list)"
sh "$repo/debian/postrm" purge
cmp "$fixture/passwd.expected" /etc/pam.d/passwd

# Upgrade migration also restores an old diversion without touching admin rules.
dpkg-divert --package vyatta-debian-pam-configs-config --add --rename \
    --divert /usr/share/pam-configs/auditd.debian /usr/share/pam-configs/auditd
sh "$repo/debian/postinst" configure
cmp "$fixture/auditd.expected" /usr/share/pam-configs/auditd
cmp "$fixture/passwd.expected" /etc/pam.d/passwd
test -z "$(dpkg-divert --list)"