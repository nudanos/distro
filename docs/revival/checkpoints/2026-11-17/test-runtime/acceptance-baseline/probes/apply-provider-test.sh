#!/bin/sh
set -eu
cp -p /opt/vyatta/sbin/system-login-information /tmp/system-login-information.original
install -o root -g root -m 755 /tmp/provider-source/scripts/system-login-information /opt/vyatta/sbin/system-login-information
sha256sum /tmp/system-login-information.original /opt/vyatta/sbin/system-login-information
echo TEMP_PROVIDER_INSTALLED
