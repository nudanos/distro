#!/bin/sh
set -u
date -u
/usr/sbin/faillock --dir /run/vyatta-faillock --user locktest
perl -I/opt/vyatta/share/perl5 -MVyatta::Login::Faillock=tally_counts -e 'my %c=tally_counts(); print "RAW_COUNT ",($c{locktest}//0),"\n"'
timeout 30 /opt/vyatta/sbin/system-login-information --action=get-state >/tmp/provider-out.json 2>/tmp/provider-err
echo PROVIDER_EXIT_$?
wc -c /tmp/provider-err
head -4 /tmp/provider-err
python3 -c 'import json; d=json.load(open("/tmp/provider-out.json")); print("PROVIDER",*[x for x in d.get("user",[]) if x.get("name")=="locktest"])' 2>&1
perl -I/opt/vyatta/share/perl5 -MVyatta::Configd -MJSON -e 'my $c=Vyatta::Configd::Client->new();my $o=$c->tree_get_full_hash("system login user-info");print "TREE ",encode_json($o),"\n"' >/tmp/tree-out 2>/tmp/tree-err
python3 -c 'import json; x=open("/tmp/tree-out").read(); d=json.loads(x.removeprefix("TREE ")); print("TREE_LOCKTEST",*[u for u in d.get("user-info",{}).get("user",[]) if u.get("name")=="locktest"])' 2>&1
cat /tmp/tree-err
echo PROVIDER_CHECK_DONE
