#!/bin/bash
# Exercise the real hook with fake host paths in a private mount namespace.
set -euo pipefail
if [ "$(readlink /proc/self/ns/mnt)" = "$(readlink /proc/1/ns/mnt)" ]; then
    echo 'Run with sudo unshare -m --propagation private bash' >&2
    exit 2
fi
repo=$(cd "$(dirname "$0")/.." && pwd)
fixture=$(mktemp -d)
mount --make-rprivate /
mkdir -p "$fixture"/{opt,profiles,bin,pam,var-lib,security,etc-upper,etc-work,bin-upper,bin-work}
mkdir -p "$fixture/var-lib/dpkg/info"
cp /var/lib/dpkg/status "$fixture/var-lib/dpkg/status"
cp "/var/lib/dpkg/info/libpam-modules:$(dpkg --print-architecture).list" "$fixture/var-lib/dpkg/info/"
mount -t overlay overlay -o "lowerdir=/etc,upperdir=$fixture/etc-upper,workdir=$fixture/etc-work" /etc
mount -t overlay overlay -o "lowerdir=/usr/bin,upperdir=$fixture/bin-upper,workdir=$fixture/bin-work" /usr/bin
mount --bind "$fixture/opt" /opt
mount --bind "$fixture/profiles" /usr/share/pam-configs
mount --bind "$fixture/pam" /etc/pam.d
mount --bind "$fixture/var-lib" /var/lib
mount --bind "$fixture/security" "/usr/lib/$(dpkg-architecture -qDEB_HOST_MULTIARCH)/security"
export PATH="$fixture/bin:$PATH"

cat > "$fixture/bin/pam-auth-update" <<'STUB'
#!/bin/sh
[ "${FAIL_UPDATE:-0}" = 0 ] || exit 1
[ "${IGNORE_UPDATE:-0}" = 0 ] || exit 0
case " $* " in
    *" --remove "*|*" --disable "*) : > /etc/pam.d/common-session;;
    *) echo "session ${TEST_CONTROL:-required} pam_sandbox.so" > /etc/pam.d/common-session;;
esac
STUB
cat > "$fixture/bin/systemctl" <<'STUB'
#!/bin/sh
[ "$*" = 'cat cli-sandbox@.service systemd-machined.service' ] && [ "${FAIL_SYSTEMD:-0}" = 0 ]
STUB
chmod +x "$fixture/bin/"*
hook="$repo/tmplscripts/system/login/user_isolation.sh"
profile=/usr/share/pam-configs/sandbox
session=/etc/pam.d/common-session
should_fail() {
    if bash "$hook" "$@"; then
        echo "sandbox hook reported success for a failed $* operation" >&2
        return 1
    fi
}

# Missing package assets must block activation.
touch "$session"
should_fail enable
test ! -e "$profile"
src=/opt/vyatta/share/pam-configs/sandbox
mkdir -p "$(dirname "$src")"
cp "$repo/src/pam_sandbox/pam-configs/sandbox" "$src"

# Deactivation retains the source for retry if PAM regeneration fails.
cp "$src" "$profile"
echo 'session required pam_sandbox.so' > "$session"
FAIL_UPDATE=1 should_fail disable
IGNORE_UPDATE=1 should_fail disable
test -f "$profile"
grep -q pam_sandbox.so "$session"
bash "$hook" disable
test ! -e "$profile"
if grep -q pam_sandbox.so "$session"; then exit 1; fi

# An installed profile alone is insufficient when the runtime is missing.
should_fail enable
test ! -e "$profile"
mkdir -p /opt/vyatta/sbin /var/lib/sandbox/{bin,etc} /etc/cli-sandbox/nspawn-templates
touch /var/lib/sandbox/etc/os-release /etc/cli-sandbox/nspawn-templates/cli_sandbox.nspawn
for file in /opt/vyatta/sbin/{cli-sandbox,cli_sandbox_init} /var/lib/sandbox/bin/vbash /usr/bin/systemd-nspawn; do
    printf '#!/bin/sh\nexit 0\n' > "$file"
    chmod +x "$file"
done
touch "$fixture/security/pam_sandbox.so"
FAIL_SYSTEMD=1 should_fail enable
FAIL_UPDATE=1 should_fail enable
test ! -e "$profile"
IGNORE_UPDATE=1 should_fail enable
test ! -e "$profile"
TEST_CONTROL=optional should_fail enable
test ! -e "$profile"
if grep -q pam_sandbox.so "$session"; then exit 1; fi

bash "$hook" enable
grep -q 'session required pam_sandbox.so' "$session"
test -f "$profile"
echo '# local edits' >> "$profile"
bash "$hook" disable
test -f "$profile"
grep -q 'local edits' "$profile"
rm "$profile"

# Package removal must also fail while the session rule is still active.
bash "$hook" enable
if IGNORE_UPDATE=1 sh "$repo/debian/pam-sandbox.prerm" remove; then
    echo 'package removal proceeded with pam_sandbox still active' >&2
    exit 1
fi
test -f "$profile"
sh "$repo/debian/pam-sandbox.prerm" remove
test ! -e "$profile"

# Unsupported native systems must reject activation even with Debian tools present.
rm /etc/os-release
echo 'ID=arch' > /etc/os-release
should_fail enable
test ! -e "$profile"
