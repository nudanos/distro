#!/bin/bash
# Build one Debian source package from /src (read-only) into /out.
# Earlier outputs under /pool are served as a local apt repository with priority
# over Debian, so packages build against each other.
#
# Derived from the buildpackage script in jsouthworth/danos-buildpackage,
# Copyright (c) 2019 John Southworth, MIT License (see NOTICE).
set -euo pipefail
shopt -s nullglob
: "${PKG:?}" "${HOST_UID:=0}" "${HOST_GID:=0}"

setup_local_repo() {
    mkdir -p /tmp/pool
    find /pool -name '*.deb' -exec cp -t /tmp/pool {} +
    # apt 3.0 probes compressed indexes first and logs read errors when only the plain one exists
    (cd /tmp/pool && apt-ftparchive packages . > Packages && gzip -9kf Packages && xz -kf Packages)
    echo 'deb [trusted=yes] file:/tmp/pool ./' > /etc/apt/sources.list.d/000-local.list
    printf 'Package: *\nPin: origin ""\nPin-Priority: 999\n' > /etc/apt/preferences.d/000-local
    apt-get update
}

main() {
    setup_local_repo
    rm -rf /build && mkdir -p /build
    cp -a /src /build/pkg
    cd /build/pkg
    local format="1.0"
    [ -f debian/source/format ] && format="$(cat debian/source/format)"
    if [ "$format" = "3.0 (quilt)" ]; then
        echo "error: $PKG uses 3.0 (quilt); building from an upstream tarball arrives in plan 2" >&2
        exit 2
    fi
    apt-get -y --no-install-recommends build-dep ./
    chown -R builder:builder /build
    runuser -u builder -- dpkg-buildpackage -us -uc -I -i
    lintian --fail-on error ../*.changes
    cp ../*.deb ../*.dsc ../*.tar.* ../*.buildinfo ../*.changes /out/
    chown -R "$HOST_UID:$HOST_GID" /out
}

main "$@"
