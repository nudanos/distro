#!/bin/sh
# SPDX-License-Identifier: GPL-2.0-only
# lighttpd reaches the REST FastCGI app through a unix socket that the
# chunker init script spawns. Debian 13's lighttpd.service runs with
# PrivateTmp=yes, so a socket under /tmp is invisible to lighttpd and every
# /rest/ request answers 503. Both sides must name the same path, outside /tmp.
set -eu
fail=0
spawn=$(sed -n 's/^FCGISOCK=//p' scripts/vyatta-webgui-chunker-aux)
conf=$(sed -n 's/^ *"socket" => "\([^"]*\)".*/\1/p' debian/vyatta-rest.postinst)
if [ -z "$spawn" ] || [ -z "$conf" ]; then
    echo "fcgi-socket-consistent: socket path not found (FCGISOCK=$spawn, lighttpd \"socket\"=$conf)" >&2
    exit 1
fi
if [ "$spawn" != "$conf" ]; then
    echo "fcgi-socket-consistent: init script spawns $spawn, lighttpd connects to $conf" >&2
    fail=1
fi
case $spawn in
/tmp/*|/var/tmp/*)
    echo "fcgi-socket-consistent: $spawn is under a directory lighttpd.service makes private (PrivateTmp=yes)" >&2
    fail=1 ;;
esac
dir=$(dirname "$spawn")
if ! grep -qE "mkdir -p $dir( |$)" scripts/vyatta-webgui-chunker-aux; then
    echo "fcgi-socket-consistent: init script does not create $dir before spawning" >&2
    fail=1
fi
[ $fail = 0 ] && echo "fcgi-socket-consistent: OK"
exit $fail
