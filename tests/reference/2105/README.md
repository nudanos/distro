# DANOS 2105 references

Captured from `danos-2105-base-amd64.iso`, SHA-256
`6d500d5d7ea69ebca0b7ada2bd74cec40c87780f41cefd9b9cc0e14fb81d9b51`.
**Never commit or upload the ISO**; only these captures are committed, so CI
never needs it. The runner refuses any other ISO.

Per scenario and router: `config.boot` (`show configuration`: on the live ISO
2105 saves outside the login shell's view, so the tree is captured without its
version footer), `commands.txt` (`show configuration commands`) and `show/<command>.txt`. `accepted.diff` per
scenario lists the reviewed differences in NuDanOS's `show` output.

Recapture one scenario (on a machine that has the ISO):

    ./distro-build -work /Volumes/nudanos/work -reference-iso ~/Downloads/danos-2105-base-amd64.iso -capture test scenario <name>
