#!/bin/bash
set -euxo pipefail
sudo pacman -Syu --noconfirm --needed base-devel go audit git pam libxcrypt jansson python python-pip python-setuptools openssh systemd perl
sudo mkdir -p /srv/nudanos
sudo chown builder:builder /srv/nudanos
pacman -Q base-devel go gcc make pkgconf audit git pam libxcrypt jansson python python-pip python-setuptools openssh systemd perl
pacman -Ss 'python.*crypt' || true
pacman -Ss '^perl-ipc-run3$' || true