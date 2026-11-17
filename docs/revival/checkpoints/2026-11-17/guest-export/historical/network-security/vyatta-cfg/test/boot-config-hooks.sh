#!/bin/bash
# SPDX-License-Identifier: GPL-2.0-only
# vyatta-boot-config-loader runs every executable regular file in
# BOOT_CONFIG_HOOK_DIR (LC_ALL=C order) as "<hook> <boot file>" before
# loading the boot configuration. Ordinary hook failures are logged;
# EX_CONFIG (78) is a safety failure and must abort before load or commit.
set -u
here=$(cd "$(dirname "$0")" && pwd)
loader=$here/../scripts/vyatta-boot-config-loader
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
mkdir "$tmp/hooks" "$tmp/bin"
fail=0

hook() { printf '#!/bin/sh\n%s\n' "$2" > "$tmp/hooks/$1"; }
hook 10-a "printf a >> $tmp/marker; echo \"hook 10-a \$1\" >> $tmp/order"
hook 20-b "printf b >> $tmp/marker; echo 'hook 20-b' >> $tmp/order"
hook "25 spaced" "printf s >> $tmp/marker"
hook 30-fail "exit 3"
hook 40-skip "printf x >> $tmp/marker"
chmod +x "$tmp/hooks/10-a" "$tmp/hooks/20-b" "$tmp/hooks/25 spaced" "$tmp/hooks/30-fail"

# cli-shell-api records what it is asked; cfgcli (commit) succeeds
cat > "$tmp/capi" <<END
#!/bin/sh
echo "\$1" >> $tmp/capi-calls
[ "\$1" = loadFile ] && echo "loadFile \$2" >> $tmp/order
exit 0
END
printf '#!/bin/sh\necho commit >> %s/commits\nexit 0\n' "$tmp" > "$tmp/bin/cfgcli"
chmod +x "$tmp/capi" "$tmp/bin/cfgcli"

PATH=$tmp/bin:$PATH BOOT_CONFIG_CAPI=$tmp/capi BOOT_CONFIG_HOOK_DIR=$tmp/hooks \
    bash "$loader" "$tmp/config.boot" > "$tmp/out" 2>&1

check() {
    local msg=$1
    shift
    if ! "$@"; then echo "boot-config-hooks: $msg" >&2; fail=1; fi
}
marker=$(cat "$tmp/marker" 2>/dev/null)
check "hooks did not run in order (marker '$marker', want 'abs')" \
    [ "$marker" = abs ]
check "the hook did not get the boot file" \
    grep -qx "hook 10-a $tmp/config.boot" "$tmp/order"
check "no log line for the failing hook" \
    grep -q "boot-config hook 30-fail: failed (3)" "$tmp/out"
check "no log line for a succeeding hook" \
    grep -q "boot-config hook 10-a: ok" "$tmp/out"
check "loadFile was not called after a hook failure" \
    grep -qx "loadFile $tmp/config.boot" "$tmp/order"
check "hooks did not run before loadFile" \
    [ "$(head -1 "$tmp/order")" = "hook 10-a $tmp/config.boot" ]
check "loadFile did not run last" \
    [ "$(tail -1 "$tmp/order")" = "loadFile $tmp/config.boot" ]

# A safety hook cannot be treated like an optional diagnostic hook: loading
# the rest could discard an unsupported firewall and enable unfiltered transit.
hook 35-critical 'exit 78'
hook 45-after "printf x >> $tmp/marker"
chmod +x "$tmp/hooks/35-critical" "$tmp/hooks/45-after"
rm -f "$tmp/marker" "$tmp/order" "$tmp/capi-calls" "$tmp/commits"
critical_status=0
PATH=$tmp/bin:$PATH BOOT_CONFIG_CAPI=$tmp/capi BOOT_CONFIG_HOOK_DIR=$tmp/hooks \
    bash "$loader" "$tmp/config.boot" > "$tmp/critical-out" 2>&1 || critical_status=$?
check "critical hook did not fail the loader with EX_CONFIG" [ "$critical_status" = 78 ]
check "configuration loaded after a critical hook failure" \
    test "$(grep -c '^loadFile ' "$tmp/order" || true)" = 0
check "configuration committed after a critical hook failure" test ! -e "$tmp/commits"
check "session was not torn down after a critical hook failure" \
    grep -qx teardownSession "$tmp/capi-calls"
check "later hooks ran after a critical hook failure" \
    test "$(cat "$tmp/marker")" = abs
check "critical failure was not explained" \
    grep -q 'critical boot-config hook 35-critical' "$tmp/critical-out"
if [ $fail != 0 ]; then
    sed 's/^/    /' "$tmp/out" >&2
    exit 1
fi
echo "boot-config-hooks: OK"
