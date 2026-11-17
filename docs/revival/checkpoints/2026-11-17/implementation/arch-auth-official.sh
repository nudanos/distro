#!/bin/bash
cd /tmp/arch-auth/login || exit 1
rm -f tests/fixtures/crypt_r.py tests/fixtures/_crypt_r*.so
python3 -c 'import crypt_r, _crypt_r; print(crypt_r.__file__, _crypt_r.__file__)'
pacman -Q python-crypt_r
python3 tests/password-rpc.py
echo "official-rpc-status:$?"
sudo /tmp/arch-auth/password-payload
echo "privileged-payload-status:$?"
PERL5LIB=lib perl -MVyatta::Login::Faillock=validate_lockout -e 'eval { validate_lockout(3, 60, undef, ["local"]) }; print $@; exit($@ ? 1 : 0)'
echo "validation-status:$?"
