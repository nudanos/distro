# Arch authentication and PAM probe

## Scope and source

The tested login and sandbox worktrees were clean at `7e3b45e08fb999c44e6ee119ef05894acda02933` and `8c71637cdd49227b394deadfbf477b7c20a5d65f`. The Linux-normalized `export-worktree.py` archives were compared to `git -c core.autocrlf=false archive`: all tracked file bytes matched. Sandbox executable bits matched; Git's tar used group-write permissions, while the exporter used 0644/0755.

The disposable Arch cloud guest initially reported `ID=arch`, PAM 1.7.3-1, libxcrypt 4.5.2-1, Jansson 2.15.1-1, systemd 262.1-1, and Python 3.14.9-1. Read-only inspection found `/usr/lib/security/pam_faillock.so` and `/usr/bin/faillock`, but no `/etc/pam.d/common-auth` and no `pam-auth-update` in PATH. These observations establish module availability, not a working NuDanOS lockout stack.

## Boundary and preparation

`Vyatta::Login::Faillock::validate_lockout` requires `/usr/sbin/pam-auth-update`, the faillock executable and module, and a recognized Debian-generated `/etc/pam.d/common-auth` stack. Arch's available faillock alone does not satisfy this integration. The sandbox hook also checks `pam-auth-update` and `common-session` before its OS check; the later prerequisite check explicitly accepts only Debian, DANOS, or Vyatta IDs. A native port needs an Arch-specific PAM stack manager, module paths, lifecycle/rollback and package dependencies; compiling a module does not supply this integration.

Debian's declared `python3-crypt-r` comes from python-crypt-r 3.13.1; the original source archive downloaded via Debian source metadata has SHA-256 `caae85eff5e7fba36f2883d91c0737b45fb91bffeb182d6acd1425c8590dfddd`. The upstream `_crypt_r` C extension was built temporarily against Arch's Python and libxcrypt; the RPC tests passed using that build. The guest operator then installed official Arch `extra/python-crypt_r 3.13.1-1`, and the tests were rerun using its `/usr/lib/python3.14/site-packages/crypt_r.py` and native extension. This is the actual Arch package, not the temporary build.

An attempted combined source-transfer/read/hook command was rejected by automatic approval review because it included the isolation enable hook outside a private namespace. A second attempt using only a private mount namespace was also rejected: a mount namespace alone does not isolate ordinary writes to shared `/etc/pam.d` files. Neither command ran. The main investigator subsequently ran the exact sandbox enable hook in a fully isolated, read-only-backed overlay chroot with a separate PID namespace and reduced capabilities. It exited 1 with `pam-auth-update is unavailable`; the guest PAM service files' archive hash stayed identical.

The first guest stopped with a kernel NULL-pointer oops during package installation and could not boot after the interrupted kernel upgrade. A clean guest was provisioned and updated successfully; the tests below ran on Linux 7.2.14-arch1-1. No live PAM stack was changed by these tests.

## Native build and test results

Commands below ran from clean exported source trees under `/tmp/arch-auth`, using GCC 16.2.1, make 4.4.1, PAM 1.7.3, Jansson 2.15.1, systemd 262.1, Python 3.14.9 and libxcrypt 4.5.2. The guest operator installed compiler/build tooling, `jansson`, `pam`, `libxcrypt`, `systemd`, Python, `perl-ipc-run3`, and official `python-crypt_r`; no product package was installed.

```sh
cd /tmp/arch-auth/sandbox
make clean && make
ldd src/pam_sandbox/pam_sandbox.so
cd lib && PYTHONPATH=. python -m unittest discover -v -s ../tests

cd /tmp/arch-auth/login
make -C src clean
make -C src CPPFLAGS=-I../tests/include LDLIBS='-ljansson -lpam'
cc -Wall -Wextra -Werror -Itests/include -o /tmp/arch-auth/password-payload tests/password-payload.c -ljansson -lpam
sudo /tmp/arch-auth/password-payload
python3 tests/password-rpc.py
PERL5LIB=lib perl -MVyatta::Login::Faillock=validate_lockout -e 'eval { validate_lockout(3, 60, undef, ["local"]) }; print $@; exit($@ ? 1 : 0)'
```

- The sandbox module built with its unmodified Makefile and linked Arch `libsystemd.so.0` and `libpam.so.0` (exit 0). This is a module build, not a deployed sandbox session.
- The sandbox Python unittest suite passed all four tests with native Arch Python and PyYAML.
- The password module compiled and linked against native PAM/Jansson (exit 0), using the repository's **fixture configd header** and omitting unavailable `libvyatta-config` and `libvyatta-util`; its four configd symbols remain unresolved. Thus this is not a complete production link or install. The serializer harness uses stubbed configd transport and passed as root, including the target effective identity case.
- The production password RPC suite passed all six tests with the official Arch `crypt_r` package: SHA-512, yescrypt, Unicode, invalid/locked credentials, and password-change fixtures. The configd client was the repository test fixture; no running configd or deployed AAA policy was exercised.
- `validate_lockout(3, 60, ...)` returned 1 and `Login auto-disable requires Debian's PAM profile manager and the faillock module and utility`. The native module and utility exist, but Debian's profile manager does not.
- Independently, the main investigator exercised stock Arch `system-auth` in an isolated chroot: real `pam_authenticate` success, failure reset after success, deny=2 lockout, administrative reset and root exemption passed. That establishes native faillock capability, not compatibility of the Debian-specific Vyatta adapter.
