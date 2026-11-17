#!/bin/sh
set -eu
# Disposable fixture: establish a known root recovery password without PAM.
h=$(openssl passwd -6 -salt rescuefixture '<disposable-fixture-password>')
usermod --password "$h" root
getent shadow root | sha256sum
echo ROOT_RECOVERY_FIXTURE_READY
