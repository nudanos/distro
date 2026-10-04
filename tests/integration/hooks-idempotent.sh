#!/bin/bash
# Install the packages that took over DANOS image hooks, run their maintainer
# scripts a second time, and check nothing is duplicated; purge base-files-vyatta
# and check the os-release diversion is gone.
set -euo pipefail
: "${WORK:?}"
ENGINE=${ENGINE:-docker}
$ENGINE run --rm -v "$WORK/repo":/repo:ro debian:trixie bash -euxc '
  apt-get update -qq >/dev/null && apt-get install -y -qq ca-certificates gpg >/dev/null
  gpg --dearmor < /repo/nudanos-archive-keyring.asc > /usr/share/keyrings/nudanos.gpg
  echo "deb [signed-by=/usr/share/keyrings/nudanos.gpg] file:/repo trixie main" > /etc/apt/sources.list.d/nudanos.list
  apt-get update -qq >/dev/null
  apt-get install -y -qq --no-install-recommends vyatta-kernel-forwarding base-files-vyatta vyatta-system >/dev/null
  grep -q "NuDanOS" /etc/os-release
  dpkg-reconfigure base-files-vyatta vyatta-system
  test "$(grep -c "^auto lo" /etc/network/interfaces)" = 1
  test "$(dpkg-divert --list /etc/os-release | wc -l)" = 1
  # Diversions are made in preinst; an upgrade runs the old postrm with
  # "upgrade", which must not undo them. A reinstall is an upgrade.
  apt-get install -y -qq --no-install-recommends vyatta-version >/dev/null
  test "$(readlink /etc/os-release.vyatta)" = os-release.vyatta-version
  apt-get install -y -qq --reinstall base-files-vyatta vyatta-version vyatta-system >/dev/null
  test "$(dpkg-divert --list /etc/os-release | wc -l)" = 1
  test "$(dpkg-divert --list /etc/os-release.vyatta | wc -l)" = 1
  test "$(readlink /etc/os-release.vyatta)" = os-release.vyatta-version
  # base-files-vyatta is Essential: removing it takes an explicit override.
  apt-get purge -y -qq --allow-remove-essential base-files-vyatta >/dev/null
  test -z "$(dpkg-divert --list /etc/os-release)"
  grep -q "Debian" /etc/os-release
  # live-build diverts /etc/os-release locally while it installs packages
  # (bootstrap_debootstrap); base-files-vyatta must install under it.
  rm -f /etc/os-release && cp /usr/lib/os-release /etc/os-release
  dpkg-divert --quiet --local --add --no-rename --divert /etc/os-release.debootstrap /etc/os-release
  apt-get install -y -qq --no-install-recommends base-files-vyatta >/dev/null
  grep -q "NuDanOS" /etc/os-release
  test "$(dpkg-divert --listpackage /etc/os-release)" = base-files-vyatta
  echo hooks-idempotent: OK'
# vyatta-kernel-forwarding selects the legacy iptables back end. In a large
# transaction (the ISO build) dpkg can reach its postinst while iptables is
# unpacked but not configured: the binaries exist, the alternatives do not.
$ENGINE run --rm -v "$WORK/repo":/repo:ro debian:trixie bash -euxc '
  apt-get update -qq >/dev/null && apt-get install -y -qq ca-certificates gpg >/dev/null
  gpg --dearmor < /repo/nudanos-archive-keyring.asc > /usr/share/keyrings/nudanos.gpg
  echo "deb [signed-by=/usr/share/keyrings/nudanos.gpg] file:/repo trixie main" > /etc/apt/sources.list.d/nudanos.list
  apt-get update -qq >/dev/null
  apt-get install -y -qq --no-install-recommends udev >/dev/null
  cd /tmp && apt-get download -qq iptables vyatta-kernel-forwarding
  dpkg --unpack --force-depends iptables_*.deb >/dev/null
  dpkg -i --force-depends vyatta-kernel-forwarding_*.deb
  apt-get install -y -qq -f >/dev/null
  echo kernel-forwarding-iptables: OK'
# vyatta-password-renewal diverts pam-configs/unix and copies its own file in;
# removing it must restore Debian's.
$ENGINE run --rm -v "$WORK/repo":/repo:ro debian:trixie bash -euxc '
  apt-get update -qq >/dev/null && apt-get install -y -qq ca-certificates gpg >/dev/null
  gpg --dearmor < /repo/nudanos-archive-keyring.asc > /usr/share/keyrings/nudanos.gpg
  echo "deb [signed-by=/usr/share/keyrings/nudanos.gpg] file:/repo trixie main" > /etc/apt/sources.list.d/nudanos.list
  apt-get update -qq >/dev/null
  apt-get install -y -qq --no-install-recommends vyatta-kernel-forwarding vyatta-password-renewal >/dev/null
  apt-get remove -y -qq vyatta-password-renewal >/dev/null
  test -z "$(dpkg-divert --list /usr/share/pam-configs/unix)"
  dpkg --verify libpam-runtime
  echo password-renewal-remove: OK'
