#!/bin/sh
# SPDX-License-Identifier: GPL-2.0-only
set -eu
cd "${srcdir:-.}"
test "$(id -u)" = 0
status=0
for case_name in valid unprivileged unsafe-directory symlink lock; do
    if ! unshare --mount sh -ec 'mount --make-rprivate /; mount -t tmpfs -o mode=0755 tmpfs /run; exec perl -I src/server/lib/perl5 tests/session-issuer.pl "$1"' sh "$case_name"; then
        status=1
    fi
done
exit "$status"
