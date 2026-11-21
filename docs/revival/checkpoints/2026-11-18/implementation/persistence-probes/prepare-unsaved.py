#!/usr/bin/python3
"""Historical diagnostic: public Commit automatically saves; this assumption failed.

Commit a benign fixture via configd without invoking CLI save.

Only run on the disposable VCI test overlay after explicit test go-ahead.
The fixed marker is non-secret; output contains statuses and fingerprints only.
"""
import hashlib
import secrets
import sys
from vyatta import configd

MARKER = "probe-unsaved.invalid"


def boot_state():
    with open("/config/config.boot", "rb") as stream:
        data = stream.read()
    return hashlib.sha256(data).hexdigest(), MARKER.encode() in data


def main():
    before_hash, before_marker = boot_state()
    if before_marker:
        raise RuntimeError("fixture marker already in startup configuration")
    client = configd.Client()
    session = "unsaved-probe-" + secrets.token_hex(12)
    established = False
    try:
        client.session_setup(session)
        established = True
        client.session_exists()
        client.set(["system", "domain-name", MARKER])
        client.commit("disposable persistence probe: unrelated unsaved state")
    finally:
        if established:
            client.session_teardown()
    running = client.tree_get_dict("system domain-name", configd.Client.RUNNING)
    if running.get("domain-name") != MARKER:
        raise RuntimeError("running fixture marker not found")
    after_hash, after_marker = boot_state()
    print("UNRELATED_RUNNING_MARKER_PRESENT")
    print("STARTUP_FILE_UNCHANGED" if after_hash == before_hash else "STARTUP_FILE_CHANGED", before_hash, after_hash)
    print("STARTUP_MARKER_ABSENT" if not after_marker else "STARTUP_MARKER_PRESENT")
    if after_hash != before_hash or after_marker:
        raise RuntimeError("unrelated commit was persisted")
    print("UNSAVED_FIXTURE_READY")


if __name__ == "__main__":
    try:
        main()
    except Exception as exc:
        print("UNSAVED_FIXTURE_FAIL", type(exc).__name__, file=sys.stderr)
        sys.exit(1)
