#!/bin/sh
# SPDX-License-Identifier: GPL-2.0-only
set -eu
cd "${srcdir:-.}"
build=$(mktemp -d)
trap 'rm -r "$build"' EXIT
export LC_ALL=C

${CXX:-c++} ${CXXFLAGS:-} -std=c++11 -UNDEBUG -Wall -ffunction-sections -fdata-sections \
    -I src/server tests/basic-credentials.cc src/server/authbasic.cc \
    src/server/http.cc src/server/common.cc -Wl,--gc-sections \
    -lcrypto -lpam -laudit -lcurl -o "$build/basic-credentials"
"$build/basic-credentials"
echo 'basic-credentials: OK'

${CXX:-c++} ${CXXFLAGS:-} -std=c++11 -UNDEBUG -Wall -ffunction-sections -fdata-sections \
    -I src/server tests/session-file.cc src/server/authsession.cc \
    -Wl,--gc-sections -o "$build/session-file"

if [ "${REST_TEST_ROOT:-0}" != 1 ]; then
    exit 0
fi
test "$(id -u)" = 0

unshare --mount sh -s -- "$build/session-file" "$PWD/lib" <<'TEST'
set -eu
reader=$1
lib=$2
issue() {
    perl -I "$lib" -e 'require Vyatta::WebGUI2::SessionKey;
        get_key("test-fixture", "operator", "192.0.2.5")' >/dev/null
}
issuer_refuses() {
    if issue; then
        echo 'session-issuer: accepted unsafe state' >&2
        exit 1
    fi
}
mount --make-rprivate /
mount -t tmpfs -o mode=0755 tmpfs /run
mkdir -m 0700 /run/gui
file=/run/gui/session
touch "$file"
chmod 0600 "$file"
"$reader" "$file" accept
"$reader" "$file" locked

chmod 0666 "$file"
"$reader" "$file" refuse
issuer_refuses
test "$(stat -c %a "$file")" = 666
chmod 0600 "$file"
chown 65534 "$file"
"$reader" "$file" refuse
issuer_refuses
chown 0 "$file"

mv "$file" /run/regular
ln -s /run/regular "$file"
"$reader" "$file" refuse
issuer_refuses
test -L "$file"
rm "$file"
mkfifo -m 0600 "$file"
"$reader" "$file" refuse
issuer_refuses
rm "$file"
mv /run/regular "$file"

chmod 0777 /run/gui
"$reader" "$file" refuse
issuer_refuses
echo 'session-file: OK'

chmod 0700 /run/gui
chmod 0666 /run/gui/session.lck
"$reader" "$file" refuse
issuer_refuses
chmod 0600 /run/gui/session.lck
(
    flock -n 8
    issuer_refuses
) 8<>/run/gui/session.lck
test -f /run/gui/session.lck
"$reader" "$file" publish
test "$(cat "$file")" = 'test data'
issue
test -s "$file"
test "$(stat -c %u:%a "$file")" = 0:600
test -z "$(find /run/gui -name 'session.??????' -print)"
echo 'session-issuer: OK'
TEST
