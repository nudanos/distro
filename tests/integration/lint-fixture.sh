#!/bin/bash
# SPDX-License-Identifier: GPL-2.0-only
# Runs cmd/lint in the nudanos/lint image over internal/lint/testdata/port,
# which plants exactly one finding per analyzer, one suppression without a
# reason and one with: each tool must report exactly its one finding, and a
# file a tool cannot check must fail the run (exit 2), not pass as clean.
set -euo pipefail
cd "$(dirname "$0")/../.."
ENGINE=${ENGINE:-docker}
T=$(mktemp -d)
trap 'rm -rf "${T:?}"' EXIT

$ENGINE build -q -t nudanos/lint:trixie lint >/dev/null
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o "$T/lint" ./cmd/lint
cp -a internal/lint/testdata/port "$T/port"

run() { $ENGINE run --rm --network none -v "$1":/src:ro -v "$T/lint":/usr/local/bin/lint:ro nudanos/lint:trixie lint -port fixture /src; }

run "$T/port" > "$T/out.json"
python3 - "$T/out.json" <<'EOF'
import collections, json, sys
fs = json.load(open(sys.argv[1]))
got = collections.Counter(f["tool"] for f in fs)
want = {t: 1 for t in ("gofmt", "perltidy", "perlcritic", "shellcheck", "flake8", "pylint", "cppcheck", "lint")}
if dict(got) != want:
    sys.exit(f"lint-fixture: findings per tool {dict(got)}, want {want}")
sup = [f for f in fs if f["tool"] == "lint"]
if sup[0]["rule"] != "suppression-without-reason" or sup[0]["line"] != 5:
    sys.exit(f"lint-fixture: wrong suppression finding {sup}")
if any(f["tool"] == "shellcheck" and f["line"] == 4 for f in fs):
    sys.exit("lint-fixture: the explained suppression did not suppress")
EOF

printf '#!/usr/bin/perl\nsub {\n' > "$T/port/scripts/broken.pl"
set +e
run "$T/port" > /dev/null 2> "$T/err"
rc=$?
set -e
if [ "$rc" != 2 ]; then
    echo "lint-fixture: a Perl file with a syntax error (perltidy fails on it; perlcritic does not) gave exit $rc, want 2: $(cat "$T/err")" >&2
    exit 1
fi
echo "lint-fixture: OK"
