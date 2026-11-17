#!/bin/sh
set -eu
export DEBIAN_FRONTEND=noninteractive
apt-get install -y --no-install-recommends golang-1.26-go
ln -sfn /usr/lib/go-1.26/bin/go /usr/local/bin/go
ln -sfn /usr/lib/go-1.26/bin/gofmt /usr/local/bin/gofmt
go version
dpkg --audit
