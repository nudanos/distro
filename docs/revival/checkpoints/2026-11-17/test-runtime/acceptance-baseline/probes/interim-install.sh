#!/bin/sh
set -u
export DEBIAN_FRONTEND=noninteractive
printf '\n# acceptance interim site policy\n' >> /etc/pam.d/login
chage -d 0 renew
echo BEFORE_INTERIM_APT
for u in root upgrade renew; do printf '%s ' "$u"; getent shadow "$u" | sha256sum; done
sha256sum /etc/pam.d/login /etc/pam.d/common-auth /config/config.boot
systemctl show -p MainPID -p ActiveState configd
p=$(systemctl show -p MainPID --value configd)
sha256sum /proc/$p/exe /usr/sbin/configd
apt-get -y -o Dpkg::Options::=--force-confold -o Dir::Etc::sourcelist=/etc/apt/sources.list.d/interim.list -o Dir::Etc::sourceparts=- install vyatta-login=2.45
rc=$?
echo INTERIM_APT_EXIT_$rc
dpkg-query -W configd vyatta-login vyatta-password-renewal vyatta-system-login-v1-yang python3-crypt-r
dpkg --audit
systemctl show -p MainPID -p ActiveState configd
p=$(systemctl show -p MainPID --value configd)
sha256sum /proc/$p/exe /usr/sbin/configd /etc/pam.d/login /etc/pam.d/common-auth /config/config.boot
for u in root upgrade renew; do printf '%s ' "$u"; getent shadow "$u" | sha256sum; done
chage -l renew
echo INTERIM_APT_INSPECTED
exit "$rc"
