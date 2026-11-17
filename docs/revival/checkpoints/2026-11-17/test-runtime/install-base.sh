#!/bin/sh
set -eu
export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y --no-install-recommends \
  build-essential git ca-certificates rsync pkg-config perl \
  libipc-run3-perl libfile-slurp-perl libjson-perl libtemplate-perl libnetaddr-ip-perl \
  python3 python3-crypt-r python3-passlib \
  libssl-dev libpam0g-dev libaudit-dev libjansson-dev libcurl4-openssl-dev \
  golang-dbus-dev golang-github-fsnotify-fsnotify-dev golang-go-systemd-dev \
  golang-golang-x-crypto-dev golang-github-go-ini-ini-dev dbus-x11 libsystemd-dev
printf '\nBase package installation completed.\n'
