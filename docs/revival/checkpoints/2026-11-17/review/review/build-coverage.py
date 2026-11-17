"""Inventory reconciliation only; not a runtime/security audit of every tree."""
import csv
import json
import re
import subprocess
from collections import Counter
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
repos = json.loads((ROOT / "repository-inventory.json").read_text(encoding="utf-8-sig"))
text = (ROOT / "distro/manifest.yaml").read_text(encoding="utf-8")
entries = {}
for block in re.split(r"(?m)^  - name: ", text)[1:]:
    lines = block.splitlines()
    name = json.loads(lines[0])
    fields = {}
    for line in lines[1:]:
        match = re.match(r"^    ([a-z_]+): (.*)$", line)
        if not match:
            continue
        key, value = match.groups()
        if value.startswith('"'):
            try:
                value = json.loads(value)
            except json.JSONDecodeError:
                pass
        fields[key] = value
    entries[name] = fields

packaging = {
    "base-files", "vyatta-base", "vyatta-login", "vyatta-debian-system-config",
    "vyatta-debian-systemd-config", "vyatta-debian-ssh-server-config",
    "vyatta-debian-passwd-config", "vyatta-debian-pam-configs-config",
    "vyatta-debian-lldpd-config", "vyatta-image-tools", "live-boot-vyatta",
    "vyatta-platform", "vyatta-hotplug", "vyatta-sssd", "build-iso", "live-build-desc",
}
tooling = {"distro", "danos-buildimage", "danos-buildpackage"}
core = {"aaa", "cli-sandbox", "config", "configd", "encoding", "ephemerad", "libvci",
        "mgmterror", "notifyd", "op", "opd", "provisiond", "vci", "vyatta-cfg",
        "vyatta-config-mgmt", "vyatta-config-migrate", "vyatta-rest", "xpath-plugins", "yang"}
network = {"dpdk", "dpdk-kmods", "frr", "ifmgrd", "netconfd", "nudanos-router",
           "routing-instance", "vrf-manager", "vyatta-cfg-dataplane", "vyatta-dataplane",
           "vyatta-interfaces", "vyatta-kernel-forwarding", "vyatta-protocols-frr",
           "vyatta-route-broker", "vyatta-security-vpn", "vyatta-service-dhcp",
           "vyatta-service-dns", "vplane-config-npf", "vplane-config-qos"}

rows = []
for repo in sorted(repos, key=lambda item: item["name"]):
    name = repo["name"]
    entry = entries.get(name, {})
    kind = entry.get("kind", "tooling" if name in tooling else "UNMAPPED")
    milestone = entry.get("milestone", "n/a")
    ready = entry.get("ready", "false")
    if name in tooling:
        treatment = "orchestration/tooling; distro is current build system"
    elif kind == "danos":
        treatment = "selected source port" if ready == "true" else "deferred source port"
    elif kind == "debian":
        treatment = "consume Debian package; archived fork is preservation/reference"
    elif kind == "upstream":
        treatment = "build supported upstream with selected packaging and audited delta"
    elif kind == "apt":
        treatment = "consume pinned upstream apt repository packages"
    elif kind == "drop":
        treatment = "excluded from current distribution"
    elif kind == "reference":
        treatment = "reference only; not part of current distribution"
    else:
        treatment = "unmapped; requires triage"
    owner = ("packaging/os integration" if name in packaging else "build engine/tooling" if name in tooling
             else "management core" if name in core else "network/forwarding" if name in network
             else "inventory triage only")
    scope = "focused static source/packaging inspection" if name in packaging else (
        "separate reviewer source inspection; consult consolidated report" if name in core | network | tooling
        else "inventory/manifest classification; not exhaustive source audit")
    path = ROOT / "repos" / name
    sha = ""
    checked_ref = ""
    if name == "distro":
        path = ROOT / "distro"
    if (path / ".git").exists():
        sha = subprocess.check_output(["git", "-C", str(path), "rev-parse", "HEAD"], text=True).strip()
        checked_ref = subprocess.check_output(["git", "-C", str(path), "branch", "--show-current"], text=True).strip()
    rows.append({
        "repository": name, "url": repo["url"], "archived": repo["archived"],
        "default_branch": repo["default_branch"], "manifest_kind": kind,
        "milestone": milestone, "ready": ready, "selected_ref": entry.get("ref", ""),
        "treatment": treatment, "review_area": owner, "coverage": scope,
        "local_checkout_ref": checked_ref, "local_checkout_sha": sha,
        "manifest_note": entry.get("note", ""),
    })

with (ROOT / "review/repository-coverage.csv").open("w", newline="", encoding="utf-8") as stream:
    writer = csv.DictWriter(stream, fieldnames=list(rows[0]))
    writer.writeheader()
    writer.writerows(rows)

known = {repo["name"] for repo in repos}
summary = {
    "organization_repositories": len(rows),
    "archived": sum(repo["archived"] for repo in repos),
    "manifest_entries": len(entries),
    "org_by_kind": dict(Counter(row["manifest_kind"] for row in rows)),
    "org_by_milestone": dict(Counter(row["milestone"] for row in rows)),
    "org_danos_ready": sum(row["manifest_kind"] == "danos" and row["ready"] == "true" for row in rows),
    "manifest_only": {name: entries[name] for name in sorted(entries.keys() - known)},
    "org_without_manifest": sorted(known - entries.keys()),
    "archived_danos_selected": [row["repository"] for row in rows if row["archived"] and row["manifest_kind"] == "danos"],
    "focused_packaging_repositories": sorted(packaging),
}
(ROOT / "review/coverage-summary.json").write_text(json.dumps(summary, indent=2) + "\n", encoding="utf-8")
print(json.dumps(summary, indent=2))
