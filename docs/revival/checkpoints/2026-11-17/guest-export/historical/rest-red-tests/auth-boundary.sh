#!/bin/sh
# SPDX-License-Identifier: GPL-2.0-only
# Linux only. Session cases mount a private /run; no host state is modified.
# Run as root (or sudo) in the isolated Linux test environment for all cases.
set -eu
cd "${srcdir:-.}"
build=$(mktemp -d)
trap 'rm -rf "$build"' EXIT HUP INT TERM
${CXX:-c++} -std=c++11 -g -O1 -Wall -Wextra -fno-omit-frame-pointer \
    ${AUTH_TEST_CXXFLAGS:--fsanitize=address,undefined} \
    -include cstring -include unistd.h -I src/server \
    tests/auth-boundary.cc src/server/authbasic.cc src/server/authsession.cc \
    src/server/http.cc src/server/common.cc src/server/rl_str_proc.cc \
    -lcrypto -ljansson -lcurl -o "$build/auth-boundary"
cases="${AUTH_TEST_CASES:-max nul logs basic session-valid session-untrusted session-files session-symlink session-tmp-symlink session-lock session-startup-unsafe session-uid}"
status=0
for case_name in $cases; do
    case "$case_name" in
        session-*)
            if [ "$(id -u)" != 0 ]; then
                echo "auth-boundary: session cases require root and mount namespaces" >&2
                exit 1
            fi
            if ! unshare --mount sh -ec 'mount --make-rprivate /; mount -t tmpfs -o mode=0755 tmpfs /run; exec "$1" "$2"' sh "$build/auth-boundary" "$case_name"; then status=1; fi
            ;;
        *) if ! "$build/auth-boundary" "$case_name"; then status=1; fi ;;
    esac
done
exit "$status"
