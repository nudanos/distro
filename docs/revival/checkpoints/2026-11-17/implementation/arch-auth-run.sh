#!/bin/bash
set -u
cd /tmp/arch-auth || exit 1
echo '== environment =='
cat /etc/os-release
uname -a
pacman -Q pam libxcrypt jansson systemd python gcc make perl 2>&1
for x in /usr/lib/security/pam_faillock.so /usr/bin/faillock /usr/sbin/faillock /usr/sbin/pam-auth-update /etc/pam.d/common-auth; do ls -l "$x" 2>&1; done
python3 -c 'import crypt_r; print(crypt_r.__file__)' 2>&1 || true
echo '== sandbox module =='
cd /tmp/arch-auth/sandbox || exit 1
make clean && make && file src/pam_sandbox/pam_sandbox.so && ldd src/pam_sandbox/pam_sandbox.so
echo "sandbox build status: $?"
echo '== password module =='
cd /tmp/arch-auth/login || exit 1
make -C src clean
make -C src CPPFLAGS=-I../tests/include LDLIBS='-ljansson -lpam'
echo "password shared build status: $? (fixture configd header; unresolved configd symbols)"
nm -u src/pam_vyatta_passwd/pam_vyatta_passwd.so | grep configd
cc -Wall -Wextra -Werror -Itests/include -o /tmp/arch-auth/password-payload tests/password-payload.c -ljansson -lpam && /tmp/arch-auth/password-payload
echo "payload harness status: $?"
echo '== crypt_r =='
cd /tmp/arch-auth/crypt_r-3.13.1 || exit 1
cp src/crypt_r.py /tmp/arch-auth/login/tests/fixtures/
cc -shared -fPIC $(python3-config --includes) src/_crypt_r.c -lcrypt -o "/tmp/arch-auth/login/tests/fixtures/_crypt_r$(python3-config --extension-suffix)"
echo "crypt_r extension build status: $?"
cd /tmp/arch-auth/login || exit 1
PYTHONPATH=tests/fixtures python3 -c 'import crypt_r, _crypt_r; print(crypt_r.__file__, _crypt_r.__file__)'
PYTHONPATH=tests/fixtures python3 tests/password-rpc.py
echo "password RPC tests status: $?"
