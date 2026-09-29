#!/bin/bash
# End-to-end pilot: build every ready package, publish the signed repo, then install
# from it in a clean Debian 13 container that trusts only Debian and NuDanOS.
set -euo pipefail
cd "$(dirname "$0")/../.."
: "${WORK:?set WORK to a case-sensitive directory}" "${KEY:?set KEY to the archive key fingerprint}"
ENGINE=${ENGINE:-docker}
GNUPG=${GNUPG:-$HOME/.nudanos/gnupg}
go build -o distro-build ./cmd/distro-build
./distro-build -work "$WORK" -engine "$ENGINE" builder
./distro-build -work "$WORK" -engine "$ENGINE" build
./distro-build -work "$WORK" -engine "$ENGINE" -gnupg "$GNUPG" -key "$KEY" repo
$ENGINE run --rm -v "$WORK/repo":/repo:ro debian:trixie bash -euxc '
    apt-get update
    apt-get install -y --no-install-recommends ca-certificates gpg
    gpg --dearmor < /repo/nudanos-archive-keyring.asc > /usr/share/keyrings/nudanos.gpg
    echo "deb [signed-by=/usr/share/keyrings/nudanos.gpg] file:/repo trixie main" > /etc/apt/sources.list.d/nudanos.list
    apt-get update
    apt-get install -y --no-install-recommends dh-yang dh-vci vyatta-util frr
    test -x /usr/bin/dh_yang
    test -f /usr/share/perl5/Debian/Debhelper/Sequence/yang.pm
    test -x /usr/bin/dh_vci_enable
    test -f /usr/share/perl5/Debian/Debhelper/Sequence/vci.pm
    test -x /opt/vyatta/sbin/vyatta-validate-type
    dpkg-query -W -f="\${Version}\n" frr | grep -qx "10.7.1-0~deb13u1"
    dpkg-query -W -f="\${Version}\n" libyang3 | grep -qx "3.13.6-1~deb13u1"
'
echo "pilot: OK"
