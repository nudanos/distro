# GitHub publication and native Arch verification

The four local commits were pushed first. GitHub returned each exact commit SHA for `refs/heads/revival/native-linux`.

| Repository | Verified commit |
| --- | --- |
| [vyatta-login](https://github.com/nudanos/vyatta-login/tree/revival/native-linux) | `7e3b45e08fb999c44e6ee119ef05894acda02933` |
| [vyatta-debian-pam-configs-config](https://github.com/nudanos/vyatta-debian-pam-configs-config/tree/revival/native-linux) | `541f763b0bd19f20c8eaa470ff60bd975cb27a49` |
| [cli-sandbox](https://github.com/nudanos/cli-sandbox/tree/revival/native-linux) | `8c71637cdd49227b394deadfbf477b7c20a5d65f` |
| [configd](https://github.com/nudanos/configd/tree/revival/native-linux) | `b63160dd18e750102c7f04f655de3d16865cab23` |

## PAM and package lifecycle

- The replacement native Arch x86_64 guest ran kernel `7.2.14-arch1-1`, PAM `1.7.3-1`, pambase `20260616-1`, and GCC `16.2.1+r23+gd564253eb6c8-1`. The final package transaction passed signature checks and `pacman -Dk`. The first overlay was discarded from testing after a kernel scheduler crash interrupted its package update. See [Arch environment](C:/Users/Jon/Documents/Projects/nudanos-revival/implementation/arch-environment-report.md).
- The published Debian PAM configuration package's `preinst install` exited 127 because Arch has no `dpkg-divert`. `postinst configure` and `postrm remove` printed the missing-helper error and exited 0. All three left the Arch PAM service files byte-identical in the test. This package lifecycle requires a native Arch adapter.
- The published `cli-sandbox` activation hook exited 1 with `pam-auth-update is unavailable`, leaving PAM service files byte-identical. Arch uses `system-auth` and related pambase service files. [Arch pambase file list](https://archlinux.org/packages/core/any/pambase/files/).
- The native PAM test compiled the published `tests/pam-auth.c` using Arch's GCC and PAM. With an isolated `deny = 2` test configuration and stock `system-auth`, it verified correct password acceptance, failure-counter reset on successful login, lockout after two incorrect attempts, administrative reset with Arch's `faillock` utility, and root recovery. It exited 0.
- The PAM probes ran in a separate PID namespace and chroot with a read-only bind of the Arch guest filesystem as the lower layer and a temporary RAM write layer. The test process dropped its mount and chroot capabilities. Reproduction from the staged guest tree is `sudo unshare --mount --propagation private bash arch-pam-overlay-run.sh`. Both shell harnesses are in the implementation directory.
- Git for Windows initially converted text during archive creation. The PAM sources were re-exported using `git -c core.autocrlf=false archive`, and the tested source files were checked against their Git blobs before transfer.

## Login and sandbox source checks

- All six password RPC tests passed with Arch's official `python-crypt_r` 3.13.1-1 and the repository's configd client fixture. The privileged serializer harness also passed the target effective identity regression. The password module compiled against native PAM and Jansson with fixture configd headers and unresolved configd symbols. A production link requires native Vyatta client libraries.
- `pam_sandbox.so` built with its existing Makefile and linked against Arch PAM and systemd. All four sandbox Python tests passed. A live sandboxed login remains untested.
- The actual login lockout validator refused Arch because the Debian profile manager is absent. The stock PAM test establishes that the underlying native lockout functionality is available. [Login and sandbox test details](C:/Users/Jon/Documents/Projects/nudanos-revival/implementation/arch-auth-report.md).

## Configd source checks

- The exact published configd source built with native Arch Go `1.27.3` and GCC. The resulting x86-64 ELF links Arch's `/usr/lib/libaudit.so.1` and glibc, and its help command ran successfully.
- Native `go vet ./...` and the full `go test -count=1 ./...` suite passed. The server suite took 21.966 seconds and the session suite took 84.676 seconds. Source dependencies were staged from the existing source snapshots and rebuilt on Arch. Debian binaries were not used.
- The real unset-login-UID regression passed under a transient systemd service, with an actual unprivileged Unix-socket peer. It reported PASS rather than SKIP and the unit exited `0/SUCCESS`.
- Compilation and verification were serialized after CPU contention under QEMU TCG. The continued run was checked after the client resumed on November 8. See [Configd verification details](C:/Users/Jon/Documents/Projects/nudanos-revival/implementation/arch-configd-report.md).

The four published repositories have Debian packaging and no PKGBUILDs. These checks do not cover a pacman installation or an integrated Arch appliance session.
