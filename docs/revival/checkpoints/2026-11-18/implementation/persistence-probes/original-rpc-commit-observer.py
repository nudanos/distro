#!/usr/bin/python3
"""Disposable-only observer for the exact original password RPC.

Forwards stdin, argv, environment and identity; never logs input, passwords,
hashes, return text or exception messages. Logs only operation boundaries and
the classification and length of a configd commit's returned text.
"""
import runpy
import sys
import syslog
from vyatta import configd

ORIGINAL = "/opt/vyatta/sbin/vyatta-password-rpc.original"
RealClient = configd.Client


def log(message):
    syslog.syslog(syslog.LOG_NOTICE, "vci-persistence-probe " + message)


class ObservedClient:
    RUNNING = RealClient.RUNNING
    CANDIDATE = RealClient.CANDIDATE

    def __init__(self, *args, **kwargs):
        self._client = RealClient(*args, **kwargs)

    def __getattr__(self, name):
        return getattr(self._client, name)

    def commit(self, *args, **kwargs):
        try:
            result = self._client.commit(*args, **kwargs)
        except Exception as exc:
            log("commit exception_type=" + type(exc).__name__)
            raise
        if isinstance(result, str):
            warning = "non-fatal failures detected" in result.lower()
            log("commit returned_text=yes nonfatal=" + str(warning).lower()
                + " length=" + str(len(result)))
        else:
            log("commit returned_text=no type=" + type(result).__name__)
        return result


configd.Client = ObservedClient
log("enter")
try:
    runpy.run_path(ORIGINAL, run_name="__main__")
except SystemExit as exc:
    code = exc.code if isinstance(exc.code, int) else 1
    log("exit code=" + str(code))
    raise
except Exception as exc:
    log("exception_type=" + type(exc).__name__)
    sys.exit(1)
else:
    log("exit code=0")
