#!/bin/sh
set -u
date -u
dpkg-query -L vyatta-system-login-v1-yang | grep 'vyatta-system-login-v1.yang' || true
perl -I/opt/vyatta/share/perl5 -MVyatta::Configd -MJSON -e 'my $c=Vyatta::Configd::Client->new(); my $o=$c->tree_get_full_hash("system login user-info"); print encode_json($o),"\n";' 2>&1
echo TREE_STATE_DONE
