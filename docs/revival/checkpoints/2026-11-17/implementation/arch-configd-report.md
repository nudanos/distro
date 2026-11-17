# Native Arch configd verification

Tested exact configd commit `b63160dd18e750102c7f04f655de3d16865cab23` on an isolated QEMU Arch Linux guest, with no product source changes or system installation of configd. Git archive was created with `git -c core.autocrlf=false archive`; transferred archive SHA-256 was `f4bf5d32ee93e2f0c95f34b4f3a9785fad3148650a70b4a64cfd8b2fd65ca9cf`. Guest `git hash-object server/conn_safety_test.go` matched the committed blob `867f9a030650a8e02e4c4fdf18b255b442d6c66e`. Dependency sources were staged from the previously prepared Debian GOPATH and `/usr/share/gocode/src`; the Go compiler, GCC, headers and linked libraries were native Arch packages, not Debian binaries.

Guest: Arch Linux x86-64, kernel `7.2.14-arch1-1`, Go `1.27.3` (Arch package `go 2:1.27.3-1`), GCC `16.2.1` and Arch audit `4.2.1-1`. `CGO_ENABLED=1`, `GO111MODULE=off`; GOPATH consisted of the staged source directories. Package ownership and environment details are in `test-runtime/arch-configd-logs/environment.log`.

- `go build -o ~/arch-configd/configd ./cmd/configd`: exit 0. The resulting test binary is x86-64 ELF, dynamically linked to Arch `/usr/lib/libaudit.so.1` and glibc; `-help` printed usage. This validates a native executable, not deployment or operation as an appliance service.
- `go vet ./...`: exit 0, no diagnostics.
- `go test -count=1 ./...`: exit 0. Server passed in 21.966s and session in 84.676s, including the committed recovery regressions.
- Builder compiled `go test -c -o ~/arch-configd/server.test ./server`; root transient systemd unit ran that binary with `-test.v -test.run=^TestPeerCredentialsRetainUIDWhenLoginUIDUnset$`. It reported PASS and `status=0/SUCCESS`, exercising a real unprivileged Unix socket peer with unset audit loginuid.

The latter checks used `GOMAXPROCS=2 GOFLAGS=-p=1`. The initial Arch overlay experienced a kernel oops during package installation; a clean replacement was provisioned and rebooted successfully. Parallel cold compilation in the replacement triggered guest soft-lockup watchdog warnings under QEMU TCG; duplicate vet work was stopped, and remaining checks ran sequentially. Complete captured output and exit statuses are under `test-runtime/arch-configd-logs/`.
