from pathlib import Path
from email.parser import Parser
p=Path('/srv/nudanos/image-step2-20261118-provisional/candidate-repo/dists/trixie/main/binary-amd64/Packages')
wanted={'configd':'2.18','vyatta-cfg':'1.36','vyatta-login':'2.46','cli-sandbox':'0.33','vyatta-debian-pam-configs-config':'1.13'}
seen={name:[] for name in wanted}
for para in p.read_text().strip().split('\n\n'):
    fields=Parser().parsestr(para)
    source=fields.get('Source',fields['Package']).split()[0]
    if source in wanted:
        assert fields['Version']==wanted[source], (source, fields['Package'], fields['Version'])
        seen[source].append(fields['Package'])
for source,items in seen.items():
    assert items, source
    print(source, wanted[source], len(items), ','.join(items))
