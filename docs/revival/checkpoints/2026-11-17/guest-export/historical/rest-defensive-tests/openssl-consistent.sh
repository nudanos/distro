#!/bin/sh
# SPDX-License-Identifier: GPL-2.0-only
# Debian 13 ships lighttpd's TLS module separately (lighttpd-mod-openssl),
# and lighttpd 1.4.x will stop loading it implicitly. Whoever enables
# ssl.engine must depend on the module's package and load it by name, or
# lighttpd fails to start: "dlopen() failed for: .../mod_openssl.so".
set -eu
fail=0
if grep -q 'ssl.engine' scripts/vyatta-update-webgui-listen-addr.pl; then
    if ! sed -n '/^Package: vyatta-rest$/,/^$/p' debian/control | grep -q 'lighttpd-mod-openssl'; then
        echo "openssl-consistent: vyatta-rest enables ssl.engine but does not depend on lighttpd-mod-openssl" >&2
        fail=1
    fi
    # top level only: 10-ssl.conf is also included inside a $SERVER["socket"]
    # conditional, where lighttpd does not accept server.modules
    if ! grep -q 'server.modules += ( "mod_openssl" )' debian/vyatta-rest.postinst; then
        echo "openssl-consistent: lighttpd.conf enables ssl.engine without loading mod_openssl at top level" >&2
        fail=1
    fi
    if grep -q 'mod_openssl' scripts/vyatta-update-webgui-listen-addr.pl; then
        echo "openssl-consistent: 10-ssl.conf must not load modules (it is also included in a conditional)" >&2
        fail=1
    fi
fi
[ $fail = 0 ] && echo "openssl-consistent: OK"
exit $fail
