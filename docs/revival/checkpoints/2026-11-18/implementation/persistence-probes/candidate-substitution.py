#!/usr/bin/python3
"""Probe configd's private candidate substitution without committing.

Run as a privileged fixture user with --user and --password-file. The password
file is supplied separately, must be private, and is never printed. This probe
does not commit or save; it tears its candidate session down in all cases.
"""
import argparse
import hashlib
import hmac
import json
import os
import secrets
import stat
import sys

from crypt_r import crypt
from vyatta import configd


def fingerprint(data):
    return hashlib.sha256(data).hexdigest()


def snapshot(client, user):
    path = f"system login user {user} authentication encrypted-password"
    value = client.tree_get_dict(path, configd.Client.RUNNING)["encrypted-password"]
    with open("/etc/shadow", "rb") as stream:
        matches = [line for line in stream if line.startswith((user + ":").encode())]
    if len(matches) != 1:
        raise RuntimeError("target shadow entry unavailable")
    with open("/config/config.boot", "rb") as stream:
        boot = stream.read()
    running = client.tree_get_dict("system login", configd.Client.RUNNING)
    redacted_tree = json.dumps(running, sort_keys=True, separators=(",", ":")).encode()
    return (fingerprint(value.encode()), value == "********", fingerprint(matches[0]), fingerprint(boot), fingerprint(redacted_tree))


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--user", required=True)
    parser.add_argument("--password-file", required=True)
    args = parser.parse_args()
    if not args.user.isascii() or not args.user.replace("-", "").isalnum():
        raise ValueError("invalid fixture user")
    mode = os.stat(args.password_file).st_mode
    if not stat.S_ISREG(mode) or mode & 0o077:
        raise ValueError("password file must be private and regular")
    with open(args.password_file, encoding="utf-8") as stream:
        password = stream.read().rstrip("\n")
    if not password or any(c in password for c in "\0\r\n"):
        raise ValueError("invalid fixture password")
    client = configd.Client()
    before = snapshot(client, args.user)
    session = "persistence-probe-" + secrets.token_hex(12)
    established = False
    candidate = None
    failure = None
    try:
        client.session_setup(session)
        established = True
        client.session_exists()
        client.set(["system", "login", "user", args.user, "authentication",
                    "plaintext-password", password])
        print("CANDIDATE_SET_RETURNED")
        path = f"system login user {args.user} authentication encrypted-password"
        result = client.tree_get_dict(path, configd.Client.CANDIDATE)
        candidate = result.get("encrypted-password")
        print("CANDIDATE_ENCRYPTED_FIELD", "PRESENT" if candidate else "ABSENT")
        if not isinstance(candidate, str):
            print("CANDIDATE_VALUE_STATUS", "NON_STRING")
        elif not candidate:
            print("CANDIDATE_VALUE_STATUS", "EMPTY")
        elif candidate.startswith("!"):
            print("CANDIDATE_VALUE_STATUS", "LOCKED_BANG")
        elif candidate == "********":
            print("CANDIDATE_VALUE_STATUS", "REDACTED_MASK")
        elif candidate.startswith("*"):
            print("CANDIDATE_VALUE_STATUS", "MASK_OR_STAR")
        else:
            print("CANDIDATE_VALUE_STATUS", "UNLOCKED_STRING")
        if not isinstance(candidate, str) or not candidate or candidate.startswith(("!", "*")):
            raise ValueError("candidate encrypted-password not usable")
        calculated = crypt(password, candidate)
        if not calculated or not hmac.compare_digest(calculated, candidate):
            raise ArithmeticError("candidate hash does not verify fixture password")
        print("CANDIDATE_HASH_VERIFIES", fingerprint(candidate.encode()))
    except Exception as exc:
        failure = exc
    finally:
        if established:
            client.session_teardown()
    after = snapshot(client, args.user)
    if before[1] or after[1]:
        print("RUNNING_SECRET", "MASKED_NOT_VERIFIED")
    else:
        print("RUNNING_SECRET", "UNCHANGED" if before[0] == after[0] else "CHANGED", before[0], after[0])
    for label, old, new in zip(("SHADOW_ENTRY", "CONFIG_BOOT", "RUNNING_REDACTED_TREE"), before[2:], after[2:]):
        print(label, "UNCHANGED" if old == new else "CHANGED", old, new)
    if before != after:
        raise RuntimeError("observable state changed")
    if failure is not None:
        raise failure
    print("CANDIDATE_PROBE_PASS")


if __name__ == "__main__":
    try:
        main()
    except Exception as exc:
        # Avoid exception text, which can contain config paths or fixture values.
        print("CANDIDATE_PROBE_FAIL", type(exc).__name__, file=sys.stderr)
        sys.exit(1)
