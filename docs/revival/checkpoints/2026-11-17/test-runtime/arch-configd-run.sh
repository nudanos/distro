#!/bin/bash
set -u
export GO111MODULE=off GOPATH=$HOME/arch-configd/gopath:$HOME/arch-configd/system CGO_ENABLED=1
cd "$HOME/arch-configd/gopath/src/github.com/danos/configd"
{ date -Is; cat /etc/os-release; uname -a; command -v go gcc; go version; gcc --version | head -1; pacman -Qo "$(command -v go)" "$(command -v gcc)" /usr/include/libaudit.h; go env GOOS GOARCH CGO_ENABLED GOPATH; } > "$HOME/arch-configd/environment.log" 2>&1
go list ./... > "$HOME/arch-configd/packages.log" 2>&1; echo $? > "$HOME/arch-configd/list.status"
go vet ./... > "$HOME/arch-configd/vet.log" 2>&1; echo $? > "$HOME/arch-configd/vet.status"
go test -count=1 ./... > "$HOME/arch-configd/test.log" 2>&1; echo $? > "$HOME/arch-configd/test.status"
go test -c -o "$HOME/arch-configd/server.test" ./server > "$HOME/arch-configd/peer-build.log" 2>&1; echo $? > "$HOME/arch-configd/peer-build.status"
file "$HOME/arch-configd/server.test" > "$HOME/arch-configd/binary.log" 2>&1
sudo systemd-run --wait --collect --pipe --unit=configd-arch-peer-test "$HOME/arch-configd/server.test" -test.v -test.run='^TestPeerCredentialsRetainUIDWhenLoginUIDUnset$' > "$HOME/arch-configd/peer.log" 2>&1; echo $? > "$HOME/arch-configd/peer.status"
echo done