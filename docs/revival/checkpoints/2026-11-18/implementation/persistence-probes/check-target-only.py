#!/usr/bin/python3
"""Historical experiment for the superseded target-only design; not a correctness proof.

Compare saved config bytes, allowing only one user's password value to change.
"""
import argparse
import hashlib
import re
import sys


def fingerprint(data):
    return hashlib.sha256(data).hexdigest()


def redact(data, user):
    pattern = re.compile(
        rb'(?ms)(^[ \t]*user[ \t]+' + re.escape(user.encode())
        + rb'[ \t]*\{[^{}]*^[ \t]*authentication[ \t]*\{[^{}]*'
        + rb'^[ \t]*encrypted-password[ \t]+)(["\']?)([^\s"\']+)(\2)'
    )
    matches = list(pattern.finditer(data))
    if len(matches) != 1:
        raise ValueError("target password is not unique")
    m = matches[0]
    redacted = data[:m.start(3)] + b"<TARGET-PASSWORD>" + data[m.end(3):]
    return redacted, fingerprint(m.group(3))


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--user", required=True)
    parser.add_argument("--before", required=True)
    parser.add_argument("--after", required=True)
    args = parser.parse_args()
    if not args.user.isascii() or not args.user.replace("-", "").isalnum():
        raise ValueError("invalid user")
    with open(args.before, "rb") as f:
        before = f.read()
    with open(args.after, "rb") as f:
        after = f.read()
    b, old = redact(before, args.user)
    a, new = redact(after, args.user)
    print("TARGET_PASSWORD_FINGERPRINT_BEFORE", old)
    print("TARGET_PASSWORD_FINGERPRINT_AFTER", new)
    print("TARGET_PASSWORD_CHANGED" if old != new else "TARGET_PASSWORD_UNCHANGED")
    print("OTHER_CONFIG_BYTES_UNCHANGED" if a == b else "OTHER_CONFIG_BYTES_CHANGED")
    if old == new or a != b:
        raise RuntimeError("target-only byte check failed")
    print("TARGET_ONLY_BYTES_PASS")


if __name__ == "__main__":
    try:
        main()
    except Exception as exc:
        print("TARGET_ONLY_BYTES_FAIL", type(exc).__name__, file=sys.stderr)
        sys.exit(1)
