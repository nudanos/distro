#!/usr/bin/env bash
# Run through arch-pam-overlay-run.sh on a disposable Arch guest.
set -euo pipefail

if [[ $EUID != 0 || $(findmnt -n -o FSTYPE -T /etc/pam.d) != overlay ]]; then
    echo 'The disposable Arch overlay root is required' >&2
    exit 2
fi
if ! grep -qx 'ID=arch' /etc/os-release; then
    echo 'This check expects native Arch Linux' >&2
    exit 2
fi

login_repo=${1:?Pass the vyatta-login source directory}
pam_repo=${2:?Pass the Debian PAM configuration source directory}
sandbox_repo=${3:?Pass the cli-sandbox source directory}
work=$(mktemp -d)
pam_fingerprint() { tar --sort=name -C /etc -cf - pam.d | sha256sum; }
original=$(pam_fingerprint)

if command -v dpkg-divert >/dev/null; then
    echo 'Expected a native Arch guest without dpkg-divert' >&2
    exit 2
fi
for invocation in 'preinst install' 'postinst configure' 'postrm remove'; do
    read -r script action <<< "$invocation"
    status=0
    /bin/sh "$pam_repo/debian/$script" "$action" >"$work/$script.log" 2>&1 || status=$?
    if ! grep -q 'dpkg-divert' "$work/$script.log" || [[ $script == preinst && $status != 127 ]]; then
        printf 'Unexpected result from Debian %s %s on Arch, exit %s\n' "$script" "$action" "$status" >&2
        cat "$work/$script.log" >&2
        exit 1
    fi
    printf 'Debian %s %s on Arch exit %s, dpkg-divert unavailable\n' "$script" "$action" "$status"
    [[ $(pam_fingerprint) == "$original" ]]
done
echo 'Debian scripts left the Arch PAM service files byte-identical'

status=0
bash "$sandbox_repo/tmplscripts/system/login/user_isolation.sh" enable >"$work/sandbox.log" 2>&1 || status=$?
if [[ $status != 1 ]] || ! grep -F 'pam-auth-update is unavailable' "$work/sandbox.log"; then
    printf 'Unexpected sandbox hook status %s\n' "$status" >&2
    cat "$work/sandbox.log" >&2
    exit 1
fi
[[ $(pam_fingerprint) == "$original" ]]

if [[ ${4:-} == --package-only ]]; then
    exit 0
fi

grep -q 'pam_faillock\.so' /etc/pam.d/system-auth
test -r /usr/lib/security/pam_faillock.so
test -x /usr/bin/faillock
name=nudanos-arch-lockout-test
if getent passwd "$name" >/dev/null || getent passwd 24680 >/dev/null; then
    echo 'The temporary PAM test account already exists' >&2
    exit 2
fi
hash=$(openssl passwd -6 -salt nudanostest correct)
day=$(( $(date +%s) / 86400 ))
gid=$(getent group users | cut -d: -f3)
printf '%s:x:24680:%s::/nonexistent:/bin/false\n' "$name" "$gid" >> /etc/passwd
sed -i '/^root:/d' /etc/shadow
printf '%s:%s:%s:0:99999:7:::\n' "$name" "$hash" "$day" root "$hash" "$day" >> /etc/shadow
mkdir "$work/tallies"
printf 'dir = %s\ndeny = 2\nunlock_time = 60\nsilent\nnodelay\nlocal_users_only\n' "$work/tallies" > /etc/security/faillock.conf
printf 'auth include system-auth\n' > /etc/pam.d/nudanos-lockout-test
cc -Wall -Wextra -Werror -o "$work/pam-auth" "$login_repo/tests/pam-auth.c" -lpam

expect_rejected() {
    if "$work/pam-auth" "$@"; then
        echo "Unexpected successful authentication for $1" >&2
        exit 1
    fi
}

"$work/pam-auth" "$name" correct
expect_rejected "$name" wrong
"$work/pam-auth" "$name" correct
expect_rejected "$name" wrong
expect_rejected "$name" wrong
expect_rejected "$name" correct
faillock --user "$name" --reset
"$work/pam-auth" "$name" correct
expect_rejected root wrong
expect_rejected root wrong
"$work/pam-auth" root correct
echo 'Arch stock system-auth passed password, failure reset, threshold, administrative reset, and root recovery checks'
