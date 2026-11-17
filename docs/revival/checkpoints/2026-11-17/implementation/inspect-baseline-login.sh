#!/bin/bash
set -e
root=/srv/nudanos/image-step2/inspect/root
iso=/srv/nudanos/image-step2/inspect/iso
find "$root" -iname '*config.boot*' -o -name user-setup.conf
cat "$iso/live/config.conf"
grep -R -n -E 'vyatta|LIVE_USERNAME|LIVE_PASSWORD' "$root/etc/live" "$iso/live/config.conf.d" 2>/dev/null | head -40 || :
python3 - <<'PY'
import crypt_r
with open('/srv/nudanos/image-step2/inspect/root/etc/shadow') as f:
    for line in f:
        if line.startswith('vyatta:'):
            pw = line.split(':')[1]
            print('shadow-vyatta-present; published password matches:', crypt_r.crypt('vyatta', pw) == pw)
PY
