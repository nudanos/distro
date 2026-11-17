#!/usr/bin/env bash
# Run with sudo unshare -m --propagation private bash arch-pam-overlay-run.sh.
set -euo pipefail
if [[ $EUID != 0 || $(readlink /proc/self/ns/mnt) == $(readlink /proc/1/ns/mnt) ]]; then
    echo 'A private root mount namespace is required' >&2
    exit 2
fi
repo=$(cd "$(dirname "$0")" && pwd)
base=$(mktemp -d)
cleanup() {
    if mountpoint -q "$base"; then umount -R "$base"; fi
    rmdir "$base"
}
trap cleanup EXIT
mount -t tmpfs -o size=512m tmpfs "$base"
mkdir "$base"/{lower,upper,work,root}
mount --bind / "$base/lower"
mount -o remount,bind,ro "$base/lower"
mount -t overlay overlay -o "lowerdir=$base/lower,upperdir=$base/upper,workdir=$base/work" "$base/root"
mount -t tmpfs -o mode=755 tmpfs "$base/root/dev"
mknod -m 666 "$base/root/dev/null" c 1 3
mknod -m 444 "$base/root/dev/urandom" c 1 9

unshare --pid --fork bash -c '
    set -eu
    root=$1
    shift
    mount -t proc -o ro,nosuid,nodev,noexec proc "$root/proc"
    exec chroot "$root" /usr/bin/setpriv --no-new-privs --bounding-set=-sys_admin,-sys_chroot /usr/bin/bash "$@"
' bash "$base/root" "$repo/arch-pam-check.sh" "$repo/login" "$repo/debian-pam" "$repo/sandbox" "$@"
