#!/bin/sh
# SPDX-License-Identifier: GPL-2.0-only
# Run the actual init script with only daemon launches doubled. Every filesystem
# mutation is inside private tmpfs mounts in a new Linux mount namespace.
set -eu
script=$(realpath "${srcdir:-.}/scripts/vyatta-webgui-chunker-aux")
test "$(id -u)" = 0
unshare --mount sh -s -- "$script" <<'TEST'
set -eu
script=$1
mount --make-rprivate /
mount -t tmpfs -o mode=0755 tmpfs /run
mount -t tmpfs -o mode=0755 tmpfs /usr/sbin
printf '#!/bin/sh\nexit 0\n' > /usr/sbin/chunker2
chmod 0755 /usr/sbin/chunker2
printf '#!/bin/sh\ntouch /run/daemon-started\n' > /usr/sbin/start-stop-daemon
cp /usr/sbin/start-stop-daemon /usr/sbin/spawn-fcgi
chmod 0755 /usr/sbin/start-stop-daemon /usr/sbin/spawn-fcgi
chown() {
    case "$*" in
        *fcgi.socket) : ;;
        *) command chown "$@" ;;
    esac
}
set -- start
. "$script"
actual=$(stat -c %u:%a /run/gui)
if [ "$actual" != 0:700 ]; then
    echo "session-startup: expected root-owned 0700 directory, got $actual" >&2
    exit 1
fi
touch /run/gui/session
chmod 0600 /run/gui/session
setpriv --reuid=65534 --regid=65534 --clear-groups sh -ec '
    ! rm -f /run/gui/session
    ! touch /run/gui/session_tmp
'
rm /run/daemon-started
chmod 0777 /run/gui
if ( . "$script" ); then
    echo "session-startup: accepted unsafe directory" >&2; exit 1
fi
test ! -e /run/daemon-started
test "$(stat -c %a /run/gui)" = 777
chmod 0700 /run/gui
chown 65534 /run/gui
if ( . "$script" ); then
    echo "session-startup: accepted directory with the wrong owner" >&2; exit 1
fi
test ! -e /run/daemon-started
rm -r /run/gui
mkdir -m 0777 /run/victim
ln -s /run/victim /run/gui
if ( . "$script" ); then
    echo "session-startup: accepted symlink directory" >&2; exit 1
fi
test ! -e /run/daemon-started
test "$(stat -c %a /run/victim)" = 777
echo 'session-startup: OK'
TEST
