#!/bin/bash
# Simulate installing each binary package of the repo together with
# vyatta-kernel-forwarding, in a clean Debian 13 container, and report the
# first unmet dependency of each one that cannot be installed.
# Usage: WORK=/Volumes/nudanos/work tests/integration/installable.sh [pkg…]
set -euo pipefail
: "${WORK:?set WORK to the distro-build work directory}"
ENGINE=${ENGINE:-docker}
$ENGINE run --rm -v "$WORK/repo":/repo:ro -e "PKGS=$*" debian:trixie bash -euc '
  apt-get update -qq >/dev/null && apt-get install -y -qq ca-certificates gpg >/dev/null
  gpg --dearmor < /repo/nudanos-archive-keyring.asc > /usr/share/keyrings/nudanos.gpg
  echo "deb [signed-by=/usr/share/keyrings/nudanos.gpg] file:/repo trixie main" > /etc/apt/sources.list.d/nudanos.list
  apt-get update -qq >/dev/null
  pkgs=${PKGS:-$(awk "/^Package: /{print \$2}" /repo/dists/trixie/main/binary-amd64/Packages | sort -u)}
  # apt-get -s takes no lock, so simulations run in parallel.
  check() {
    if apt-get install -s --no-install-recommends vyatta-kernel-forwarding "$1" >"/tmp/o.$1" 2>&1; then
      echo "OK $1"
    else
      echo "NO $1 $(grep -m1 -E "Depends:|Conflicts:|Breaks:" "/tmp/o.$1" | tr -s " " | cut -c1-200)"
    fi
  }
  export -f check
  printf "%s\n" $pkgs | xargs -P 8 -I{} bash -c "check {}" | sort -k2,2'
