# Task 1 — REST authentication boundary

Commit `84143ff4163c7ae9a4eaa6cafdddf4133580a4f6` in `vyatta-rest`.

## Changes

- Basic authentication decodes the exact Base64 result, rejects malformed input and decoded NUL bytes, and accepts up to 1,024 decoded bytes. The first colon separates the username from the password.
- REST request diagnostics redact authorization and cookie headers. The patch removes password, session row, token, and raw FastCGI environment and response dumps.
- The FastCGI reader and Perl issuer require a root-owned `0700` runtime directory and root-owned, single-link regular files with mode `0600`. They share a persistent advisory lock and publish updates by renaming a private temporary file. The reader commits a refresh before changing effective identity.
- Existing valid 16-character tokens in a protected store retain their header and cookie formats. New tokens use eight random bytes. Startup refuses an unsafe existing directory, so an upgrade from the former shared directory requires fresh runtime state and session reissuance. A legacy issuer running as an ordinary web-server user cannot use this store.

## Verification

- On the original source, a benign Basic scheme assertion failed and the session-file guard test showed that the reader accepted an empty file with mode `0666` in a private mount namespace.
- On Debian, `tests/rest-auth-checks.sh` passed with `REST_TEST_ROOT=1`. It covers Basic framing, padding, malformed input, byte limits, the first colon, NUL rejection, diagnostic redaction, unsafe session file types and ownership, lock contention, and private atomic writes. The issuer test discards its synthetic output.
- The Basic unit test also passed with GCC AddressSanitizer and UndefinedBehaviorSanitizer enabled.
- `tests/session-startup.sh` passed with a private `/run` and daemon launch stubs. It covers root-owned `0700` creation, unprivileged file modification refusal, and refusal of an unsafe mode, owner, or symlink at startup.
- The three existing shell checks and Perl syntax check passed. Focused compilation covered the Basic and session reader translation units. Standalone syntax checks passed for `authenticate.cc` and `appmode.cc`.

## Verification limits

- The full package build and installed FastCGI/PAM integration require the package build environment. `dpkg-checkbuilddeps` reported missing dependencies including `libfcgi-dev`, `libvyatta-cfg-dev`, `vyatta-opd-dev`, and `libvyatta-util-dev`. A standalone compile of `servicemode.cc` stopped at the missing `client/connect.h` header.
- The app and service helper dispatchers pass `SESSION` inside the child shell command string. This is a separate visibility path from the diagnostic output covered here.
