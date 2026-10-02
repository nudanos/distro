#!/bin/bash
# Compile every YANG module the router installs, as the DANOS image hook
# 98-yangcheck did at ISO build time: the YANG of every built binary except
# those docs/kernel-forwarding.md leaves out of nudanos-router (DPDK-only or
# deferred packages may import modules that are not built), with the XPath
# plugin function names the built packages register.
# Usage: tools/yangcheck.sh WORK
set -euo pipefail
WORK=${1:?usage: yangcheck.sh WORK}
ENGINE=${ENGINE:-docker}
DOC="$(cd "$(dirname "$0")/.." && pwd)/docs/kernel-forwarding.md"
EXCLUDED=$(sed -n '/^## DPDK-only/,$p' "$DOC" | awk -F'|' 'NF>2{gsub(/ /,"",$2); print $2}' | grep -v '^Package$\|^-' | sort -u | tr '\n' ' ')
$ENGINE run --rm -v "$WORK/out":/out:ro -e "EXCLUDED=$EXCLUDED" nudanos/builder:trixie bash -euc '
  mkdir -p /tmp/y && cd /tmp/y
  for d in $(find /out -name "*.deb" ! -name "*-dbgsym_*"); do
    pkg=$(basename "$d"); pkg=${pkg%%_*}
    case " $EXCLUDED " in *" $pkg "*) continue ;; esac
    dpkg-deb --fsys-tarfile "$d" | tar -x --wildcards "./usr/share/configd/yang/*" "./usr/lib/xpath/plugins/*.ini" 2>/dev/null || true
  done
  # yangc learns custom XPath function names from /lib/xpath/plugins/*.ini.
  mkdir -p /usr/lib/xpath/plugins && cp usr/lib/xpath/plugins/*.ini /usr/lib/xpath/plugins/
  dpkg-deb -x "$(ls /out/configd/yang-utils_*.deb | sort -V | tail -1)" /tmp/u
  echo "yangcheck: $(ls usr/share/configd/yang | wc -l) modules"
  /tmp/u/usr/bin/yangc usr/share/configd/yang
  echo "yangcheck: OK"'
