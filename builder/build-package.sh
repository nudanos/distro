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
    # /pool is indexed once per tier by index-pool; read it in place.
    echo 'deb [trusted=yes] file:/pool ./' > /etc/apt/sources.list.d/000-local.list
    printf 'Package: *\nPin: origin ""\nPin-Priority: 999\n' > /etc/apt/preferences.d/000-local
    # Fail on any index that cannot be fetched (a DNS outage otherwise
    # surfaces as misleading "not installable" build-dependencies); retry
    # transient network errors first.
    apt-get update --error-on=any -o Acquire::Retries=3
}


main() {
    setup_local_repo
    rm -rf /build && mkdir -p /build
    cp -a /src /build/pkg
    # The CI caller workflow belongs to the repository, not the package; left in,
    # dh-golang copies it into the Go source tree and dh_missing fails on it.
    rm -rf /build/pkg/.github
    cd /build/pkg
    local format="1.0"
    [ -f debian/source/format ] && format="$(cat debian/source/format)"
    if [ "$format" = "3.0 (quilt)" ]; then
        local src upver
        src=$(dpkg-parsechangelog -S Source)
        upver=$(dpkg-parsechangelog -S Version | sed -E 's/^[0-9]+://; s/-[^-]*$//')
        if ! compgen -G "/build/${src}_${upver}.orig.tar.*" >/dev/null; then
            # No orig tarball: generate one from the tree minus debian/ (our
            # upstream kind and DANOS quilt forks keep the full source in git).
            tar -C /build --exclude=pkg/debian --exclude=pkg/.git --sort=name --mtime='@0' \
                --owner=0 --group=0 --numeric-owner -cJf "/build/${src}_${upver}.orig.tar.xz" pkg
        fi
    fi

    local prof=()
    [ -n "${DEB_BUILD_PROFILES:-}" ] && prof=(-P"${DEB_BUILD_PROFILES// /,}")
    apt-get -y --no-install-recommends "${prof[@]}" build-dep ./
    chown -R builder:builder /build
    runuser -u builder -- env DEB_BUILD_OPTIONS="parallel=${JOBS:-1}" DEB_BUILD_PROFILES="${DEB_BUILD_PROFILES:-}" dpkg-buildpackage -us -uc -I -i "${prof[@]}"
    # apt-cache show exits 0 for a name another package merely references
    # (apt 3); madison lists only real, installable versions.
    if [ -n "$(apt-cache madison lintian-profile-vyatta 2>/dev/null)" ]; then
        apt-get install -y --no-install-recommends lintian-profile-vyatta >/dev/null
        lintian --profile vyatta --fail-on error ../*.changes
    else
        # Bootstrap only: before lintian-profile-vyatta is in the pool, mirror the
        # one tag it disables (DANOS installs under /opt/vyatta by design).
        lintian --fail-on error --suppress-tags dir-or-file-in-opt ../*.changes
    fi

    cp ../*.deb ../*.dsc ../*.tar.* ../*.buildinfo ../*.changes /out/
    chown -R "$HOST_UID:$HOST_GID" /out
}

main "$@"
