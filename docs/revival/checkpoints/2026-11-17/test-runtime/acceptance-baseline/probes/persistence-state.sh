#!/bin/sh
set -u
date -u
systemctl show -p MainPID -p ActiveState configd
for u in root upgrade renew; do printf '%s shadow-entry ' "$u"; getent shadow "$u" | sha256sum; done
sha256sum /config/config.boot /etc/pam.d/login
perl -I/opt/vyatta/share/perl5 -MVyatta::Configd -MJSON -MDigest::SHA=sha256_hex -e 'my $c=Vyatta::Configd::Client->new(); for my $u (qw(upgrade renew)) { my $x=$c->tree_get_full_hash("system login user $u"); print "running $u ",sha256_hex(JSON::encode_json($x)),"\n"; }' 2>&1
python3 -c 'import re,hashlib; t=open("/config/config.boot").read(); [(lambda m: print("startup",u,hashlib.sha256(m.group(0).encode()).hexdigest() if m else "MISSING"))(re.search(r"user "+u+r"\s*\{\s*authentication\s*\{[^}]*\}",t,re.S)) for u in ("upgrade","renew")]'
echo PERSISTENCE_STATE_DONE
