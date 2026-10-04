#!/bin/bash
# The image's 95-build.txt hook stamps the NuDanOS version and build time into
# the files "show version", the login banner and install image read. Runs the
# hook against a fixture root shaped like vyatta-version's files.
# SPDX-License-Identifier: GPL-2.0-only
set -euo pipefail
ENGINE=${ENGINE:-docker}
HERE=$(cd "$(dirname "$0")/../.." && pwd)
$ENGINE run --rm -v "$HERE/image":/image:ro debian:trixie bash -euc '
  R=$(mktemp -d)
  mkdir -p $R/opt/vyatta/etc $R/etc $R/live-build/config
  printf "Version:      1.6.00000000\nDescription:  NuDanOS (UNKNOWN)\n" > $R/opt/vyatta/etc/version
  printf "Welcome to NuDanOS 00000000\n" > $R/opt/vyatta/etc/motd
  printf "ID=vyatta\nPRETTY_NAME=\"NuDanOS\"\nNAME=\"NuDanOS\"\n" > $R/etc/os-release.vyatta-version
  ln -s iso-build.txt $R/opt/vyatta/etc/build.txt
  printf "NUDANOS_VERSION=1.0~20261003\nSOURCE_DATE_EPOCH=1791041520\n" > $R/live-build/config/environment.chroot_hooks
  ROOT=$R sh /image/config/hooks/live/95-build.txt.chroot
  cat $R/opt/vyatta/etc/version $R/etc/os-release.vyatta-version $R/opt/vyatta/etc/build.txt $R/opt/vyatta/etc/motd
  # install image takes the image name from Version:, and refuses "~"
  grep -qx "Version:      1.0-20261003.1532" $R/opt/vyatta/etc/version
  grep -qx "Description:  NuDanOS 1.0~20261003" $R/opt/vyatta/etc/version
  grep -qx "VERSION=\"1.0~20261003\"" $R/etc/os-release.vyatta-version
  # show version prints VERSION_ID as Version: and PRETTY_NAME VERSION as
  # Description:, adding "(VYATTA_PROJECT_ID)" unless that ends in VERSION_ID
  grep -qx "VERSION_ID=\"1.0-20261003.1532\"" $R/etc/os-release.vyatta-version
  grep -qx "BUILD_ID=\"1.0-20261003.1532\"" $R/etc/os-release.vyatta-version
  grep -qx "VYATTA_PROJECT_ID=\"NuDanOS:1.0-20261003.1532\"" $R/etc/os-release.vyatta-version
  test "$(grep -c ^VERSION= $R/etc/os-release.vyatta-version)" = 1
  grep -q "^Built on: .*2026" $R/opt/vyatta/etc/iso-build.txt
  ! grep -q 00000000 $R/opt/vyatta/etc/motd
  # running twice changes nothing
  cp $R/etc/os-release.vyatta-version /tmp/a
  ROOT=$R sh /image/config/hooks/live/95-build.txt.chroot
  cmp /tmp/a $R/etc/os-release.vyatta-version
  echo build-version: OK'
