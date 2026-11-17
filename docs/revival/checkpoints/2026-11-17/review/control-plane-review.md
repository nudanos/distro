# Control, configuration and management plane review

Reviewed 2026-11-03. Target: native Debian first, native Arch second. This is a source review of the current NuDanOS ports, not a review of preserved DANOS default branches alone. No product source was changed; no service, privileged script, attack request, or upstream mutation was executed.

## Outcome

Retain the YANG compiler/schema, config tree/diff machinery, configd session API, and the component boundary. They provide a coherent management model and substantially more tests than the legacy scripts. The present ports need a security release gate before management interfaces are exposed. Packaging progress and successful configuration scenarios do not establish safe authentication or malformed-message handling.

Native Arch is feasible without another management-plane implementation. First make the build, schema/component installation, service identity, authentication, and sandbox image interfaces distribution independent. Debian and Arch packages should wrap those same build/install interfaces. An Arch host running the Debian container produces a Debian system; it does not meet native Arch support.

## Ranked source findings

### CP-01 — P1: local users can forge a privileged REST session

`vyatta-rest` trixie SHA `a520840d1bc51a9bef4d86bb61bd2105068fb2a7`.

The REST start script creates `/var/run/gui` and makes it world writable without the sticky bit. That directory contains the authentication authority `/var/run/gui/session`. Directory write permission lets any local user replace a session file even when the file itself has restrictive permissions. The FastCGI daemon starts as root.

The session authenticator is registered unconditionally. It accepts either the `Vyatta-Session` authorization scheme or the `vyatta2_path_loc` cookie, finds a matching token and unexpired timestamp in that file, reads the claimed username, looks up its groups, sets the audit login UID and effective credentials, and returns success. There is no PAM/password check on this path. A forged CSV record has the parser's normal five fields: token, existing username, timestamp, source, access level. Choosing an existing `vyattacfg` administrator causes the real group lookup to grant configuration access. The source field is not compared with the current request. An access level other than `service-user` takes the ordinary local-account path. A running session authenticator initializes/clears the file on daemon startup; that does not protect it after startup.

Evidence: [directory permissions and root spawn](https://github.com/nudanos/vyatta-rest/blob/a520840d1bc51a9bef4d86bb61bd2105068fb2a7/scripts/vyatta-webgui-chunker-aux#L38-L48), [both authenticators registered](https://github.com/nudanos/vyatta-rest/blob/a520840d1bc51a9bef4d86bb61bd2105068fb2a7/src/server/authenticate.cc#L33-L36), [CSV trust and token/expiry check](https://github.com/nudanos/vyatta-rest/blob/a520840d1bc51a9bef4d86bb61bd2105068fb2a7/src/server/authsession.cc#L223-L285), [identity switch](https://github.com/nudanos/vyatta-rest/blob/a520840d1bc51a9bef4d86bb61bd2105068fb2a7/src/server/authsession.cc#L308-L349), [configuration access-level gate](https://github.com/nudanos/vyatta-rest/blob/a520840d1bc51a9bef4d86bb61bd2105068fb2a7/src/server/confmode.cc#L119).

Impact: a local low-privilege account or compromised local service can impersonate a management administrator through HTTPS whenever REST is enabled. This is not a claim that a remote unauthenticated client can directly write the file. It applies equally to native Debian and Arch if these permissions are preserved.

Recommendation: move authentication state to a root-owned non-writable runtime directory, open state safely without following attacker-controlled links, and remove the obsolete session scheme if no supported client needs it. If retained, centralize token issuance and credential verification. Test with a separate unprivileged Linux UID; verify inability to replace the session state and inability to reuse fabricated records.

### CP-02 — P1: Basic authentication reads beyond its decoded input before authentication

Same REST SHA.

`AuthBasic::authorized` decodes attacker-controlled Base64 into an uninitialized `unsigned char out[1025]`, with a maximum read length of 1024. `BIO_read` writes a counted byte sequence, not a C string. The code neither rejects decode errors nor writes a terminator. It then constructs `std::string((char*)out)`; debug logging also passes this buffer to `%s`. Neither operation uses the returned length. A normal non-NUL credential reads uninitialized trailing bytes; a maximum-length non-NUL credential can read beyond the buffer. Invalid Base64 can leave the buffer wholly uninitialized. This executes before PAM authenticates the request.

Evidence: [decoder returns only a byte count](https://github.com/nudanos/vyatta-rest/blob/a520840d1bc51a9bef4d86bb61bd2105068fb2a7/src/server/authbasic.cc#L77-L94), [uninitialized buffer and unbounded string construction](https://github.com/nudanos/vyatta-rest/blob/a520840d1bc51a9bef4d86bb61bd2105068fb2a7/src/server/authbasic.cc#L127-L140).

Impact: undefined behavior in the root REST worker from an unauthenticated Basic header, including potential worker crashes and stale-memory-dependent authentication parsing. Information disclosure into logs is conditional on debug mode; remote code execution was not established. Debug is off by default and enabled by `/run/vyatta-webgui2/debug_webgui2` ([main.cc](https://github.com/nudanos/vyatta-rest/blob/a520840d1bc51a9bef4d86bb61bd2105068fb2a7/src/server/main.cc#L75-L92)). In debug mode the same file additionally logs plaintext passwords explicitly at lines 155 and 225-226; that should be removed independently of the bounds fix.

Recommendation: accept only successful bounded decoding, construct the credential string with its explicit length, reject embedded NUL/invalid delimiters, and remove credential logging. Add malformed/zero-length/1024-byte authentication tests under ASan/UBSan on Linux.

### CP-03 — P1: one JSON `null` parameter can terminate configd

`configd` trixie SHA `240e958c481b5024620a5599a665f5b31c222ec4`; `config` SHA `160bc60115ab149531f6f11c03e6f4a1cad563a0`.

The JSON-RPC decoder stores parameters as `[]interface{}`. A JSON null becomes a nil interface. `SrvConn.Call` takes `reflect.TypeOf(v)`, gets nil, and immediately invokes `t1.ConvertibleTo(t2)` on it. No request-handler recovery protects that panic. A single null passed to an otherwise permitted one-string method such as `SessionExists` reaches this path. An unrecovered panic in this goroutine terminates the Go process, losing all live sessions. The shipped socket is mode 0777. The generated ACM protocol policy explicitly makes execution default allow, so a normal data-node deny rule is not a mitigation for this parameter parser.

Evidence: [reflect dereference](https://github.com/nudanos/configd/blob/240e958c481b5024620a5599a665f5b31c222ec4/server/conn.go#L256-L274), [handler calls it directly](https://github.com/nudanos/configd/blob/240e958c481b5024620a5599a665f5b31c222ec4/server/conn.go#L213-L229), [socket mode](https://github.com/nudanos/configd/blob/240e958c481b5024620a5599a665f5b31c222ec4/debian/configd.socket#L5-L8), [generated execution default](https://github.com/nudanos/config/blob/160bc60115ab149531f6f11c03e6f4a1cad563a0/auth/acmd/vci-acm-v1.go#L57-L67).

Impact: local denial of service against the shared management daemon. A remote route to this raw RPC parser has not been demonstrated. systemd restart limits outage duration but does not preserve sessions or prevent a repeated crash. Both native distributions require the same parser fix.

Recommendation: handle nil explicitly, validate JSON kinds against the complete method signature, and return structured invalid-parameter errors. Recover at the connection boundary as defense in depth. Add null, wrong-shape, omitted-parameter and large-input cases; restrict the socket to callers that need it if feasible.

### CP-04 — P1: untrusted VCI signals are assumed to have a valid string body

`vci` trixie SHA `0499b9e2b4c5ee7883e0f0c397685cfd26d762c4`.

The bus configuration allows every local user to connect and send signals. VCI subscribes by interface and member alone, without a sender restriction. `DeliverSignal` indexes `signal.Body[0]` and type-asserts it to string before schema validation runs. A matching signal with an empty body or a non-string first argument therefore panics the subscriber process. A validly shaped forged signal also passes the transport boundary without an authenticated producer check; later YANG validation checks content, not who emitted it.

Evidence: [all-user connect and signal send policy](https://github.com/nudanos/vci/blob/0499b9e2b4c5ee7883e0f0c397685cfd26d762c4/vcidbus/vyatta-vci-bus.conf#L32-L45), [unchecked body](https://github.com/nudanos/vci/blob/0499b9e2b4c5ee7883e0f0c397685cfd26d762c4/dbus_transport.go#L137-L153), [subscription without sender restriction](https://github.com/nudanos/vci/blob/0499b9e2b4c5ee7883e0f0c397685cfd26d762c4/dbus_transport.go#L201-L214), [later content validation](https://github.com/nudanos/vci/blob/0499b9e2b4c5ee7883e0f0c397685cfd26d762c4/subscription.go#L185-L217).

Impact: local denial of service against components with active subscriptions, including the notification dispatcher. The exact affected subscriber set depends on the installed component models and active ACM receiver rules. The bundled base policy is permissive; receiver ACLs are not producer authentication. This is not a verified remote network attack.

Recommendation: check body length and types before use; associate each notification with its authorized producer and enforce producer policy on the bus. Limit and bound message queues. Add malformed D-Bus signal and unauthorized-sender integration tests, with the receiving process required to remain alive. Apply to the common VCI implementation for both distributions.

### CP-05 — P1: unset audit login UID changes an unprivileged peer into root identity

Same configd SHA.

`getCreds` first obtains the kernel-authenticated peer UID using `SO_PEERCRED`, then overwrites it with `getLoginUid`. For the unset audit UID, `getLoginUid` returns `(0, LoginPidError)`. `Handle` explicitly ignores that particular error and proceeds. The connection is consequently assigned UID 0, `Superuser=true`, root's group list, username and home, even when its actual peer UID was unprivileged. Unset login UIDs are explicitly anticipated by the code for daemon/boot processes.

Evidence: [unset becomes 0](https://github.com/nudanos/configd/blob/240e958c481b5024620a5599a665f5b31c222ec4/server/conn.go#L103-L117), [overwrite and ignored error](https://github.com/nudanos/configd/blob/240e958c481b5024620a5599a665f5b31c222ec4/server/conn.go#L126-L155), [root context flag](https://github.com/nudanos/configd/blob/240e958c481b5024620a5599a665f5b31c222ec4/server/conn.go#L158-L169), [superuser bypass of session ownership](https://github.com/nudanos/configd/blob/240e958c481b5024620a5599a665f5b31c222ec4/session/sessionmgr.go#L53-L65), [caller-selected privileged helper identity](https://github.com/nudanos/configd/blob/240e958c481b5024620a5599a665f5b31c222ec4/server/config_mgmt.go#L191-L201).

Impact: identity confusion and session-ownership bypass for local processes whose audit UID is unset. File helper commands are selected for root rather than the actual peer. Data ACM is still evaluated using the resulting root groups, so this review does not claim every possible configuration operation is automatically authorized. An Arch port with different PAM/audit setup increases the need to test this boundary; it is already relevant to Debian service accounts.

Recommendation: preserve real/effective peer identity separately from audit attribution. For an unset audit UID, use the authenticated peer UID or reject the connection according to an explicit daemon policy. Do not infer privilege from a failed attribution lookup. Test set/unset audit IDs and privilege-dropping services with distinct UIDs.

### CP-06 — P2: successful commits can silently lose their recovery configuration

Same configd SHA.

`CommitMgr.writeRunning` creates/truncates the running config file and returns errors from creation, chmod, show and write. Its caller ignores the returned error after publishing the new in-memory tree, and reports the commit as successful. A full filesystem, read-only mount, permissions error, interrupted write or process crash can leave the running recovery file absent, partial or empty. A later daemon restart loads that file while ignoring load errors. The service explicitly describes `-runfile` as recovery state in case of restart.

Evidence: [truncating writer](https://github.com/nudanos/configd/blob/240e958c481b5024620a5599a665f5b31c222ec4/session/commitmgr.go#L63-L82), [ignored error after memory publish](https://github.com/nudanos/configd/blob/240e958c481b5024620a5599a665f5b31c222ec4/session/commitmgr.go#L161-L183), [ignored recovery load errors](https://github.com/nudanos/configd/blob/240e958c481b5024620a5599a665f5b31c222ec4/server/server.go#L46-L48).

Recommendation: atomically replace the recovery file with a mode-0600 temporary file in the same filesystem, check write/sync/close/rename results, and return an observable degraded-state error when persistence fails. Fail explicitly or recover from a known-good source when startup parsing fails. Test ENOSPC, read-only directories, truncated files and a forced restart. This is independent of Debian versus Arch packaging.

### CP-07 — P1 for native Arch support: isolation activation is Debian-specific and can falsely report success

`cli-sandbox` trixie SHA `7a3483a262d2b2bb6ded3dd62ecc0b3d68196c6e`.

User-isolation activation calls `pam-auth-update`, copies Debian PAM profile snippets, prints `User Isolation Enabled`, and exits 0 without checking the activation command. The source script contains no fail-fast option. If the PAM tool is unavailable or fails, configuration can claim isolation was enabled although PAM never activates `pam_sandbox.so`. This is an actual silent-failure bug, not merely a different package name. The sandbox root creator is also explicitly Debian-only: it invokes `mmdebstrap`, reads `/etc/apt` sources and hardcodes suite `trixie`.

Evidence: [unchecked activation](https://github.com/nudanos/cli-sandbox/blob/7a3483a262d2b2bb6ded3dd62ecc0b3d68196c6e/tmplscripts/system/login/user_isolation.sh#L18-L36), [Debian root construction](https://github.com/nudanos/cli-sandbox/blob/7a3483a262d2b2bb6ded3dd62ecc0b3d68196c6e/scripts/cli-sandbox-create#L1-L17), [PAM profile](https://github.com/nudanos/cli-sandbox/blob/7a3483a262d2b2bb6ded3dd62ecc0b3d68196c6e/src/pam_sandbox/pam-configs/sandbox).

Recommendation: require an explicit distro authentication adapter and a distro rootfs constructor, fail the configuration transaction when either activation or prerequisite verification fails, and run login/session isolation tests on each native distro. The PAM C module and nspawn namespace logic can remain shared. Debian also needs failure propagation rather than a success message after an error.

## Additional security work requiring follow-up

These are concrete review leads or design debt, not extra fully validated exploit claims:

- REST app mode interpolates raw request URI, method, username and session token into shell source before `/bin/sh -c` and `/bin/bash -p -c`. `Command::validate` always returns true. The app template walker runs only after that shell parsing. Investigate shell injection and whether supported clients still need this endpoint. [appmode.cc:56-96](https://github.com/nudanos/vyatta-rest/blob/a520840d1bc51a9bef4d86bb61bd2105068fb2a7/src/server/appmode.cc#L56-L96), [command.cc:24-31](https://github.com/nudanos/vyatta-rest/blob/a520840d1bc51a9bef4d86bb61bd2105068fb2a7/src/server/command.cc#L24-L31). `Mode::system_out` deliberately drops real IDs to effective IDs, so ordinary Basic app execution should not be described as automatically root.
- Session token generation encodes the IPv4 source into half of a 16-hex-digit token, leaving only four random bytes. The receiver does not enforce the source field. Replace with high-entropy opaque tokens if keeping sessions. [SessionKey.pm:39-47](https://github.com/nudanos/vyatta-rest/blob/a520840d1bc51a9bef4d86bb61bd2105068fb2a7/lib/Vyatta/WebGUI2/SessionKey.pm#L39-L47).
- AAA plugin load/setup failures are logged and skipped; users fall back to the normal ACM authorizer if no usable protocol claims them. Decide whether a configured required AAA provider may fail open in this way, especially during plugin ABI mismatches. [aaa.go:185-203](https://github.com/nudanos/aaa/blob/ba179454d70e755df3525939637b9e8d70bee41f/aaa.go#L185-L203), [authorizer selection](https://github.com/nudanos/config/blob/160bc60115ab149531f6f11c03e6f4a1cad563a0/auth/auth.go#L337-L383).
- The VCI bus default policy grants any user ownership of `net.vyatta.configd` even though config-write authorization trusts that sender name. A normal active configd owns it already, but startup/restart name squatting merits a dedicated integration test. [bus policy:101-111](https://github.com/nudanos/vci/blob/0499b9e2b4c5ee7883e0f0c397685cfd26d762c4/vcidbus/vyatta-vci-bus.conf#L101-L111).
- Most script execution paths have no timeout/cancellation. A stuck check or component call can occupy a commit or notification worker indefinitely. Establish operational timeouts, cancellation semantics and partial-apply diagnostics without assuming the legacy commit path is a fully atomic distributed transaction.

## Architecture and native Debian/Arch boundary

### Shared core worth retaining

The management chain is CLI/REST/NETCONF to configd, then schema validation and session/tree operations, then VCI components and legacy script hooks. `opd` handles operational commands; `op` merges YANG and legacy templates. `provisiond` bridges legacy schema RPC/state hooks; `ephemerad` turns declarative script specifications into component interfaces; `notifyd` runs registered scripts from notifications. This is a reasonable staged migration strategy if the security and error contracts are made explicit.

The management plane already has a language-neutral C/C++/Python/Perl bridge in `libvci`. Retain this boundary while consolidating build definitions, rather than rewriting every feature script before producing a useful release. Keep the configd API stable and add adapters around it. A later REST replacement can use the common schema/session API and eliminate the C++ FastCGI authentication/shell surface incrementally.

`yangd` is not another independent daemon needed for the first release: configd embeds its temporary YANG validation and RPC lookup service (`cmd/configd/yangd.go`); the separate repository is marked later in the manifest. Similarly, the old `golang-github-danos-encoding-rfc7951` repository is explicitly superseded by `encoding`. Do not port duplicate implementations just because the organization contains both.

The trixie `vyatta-bash` is now only packaging for `/bin/vbash` pointing to distribution Bash, rather than the preserved Bash fork. This is a useful simplification for both distros. Current SHA `da737f7b4b70d577e2b6b723881f7d18336607fa`; [binary package depends on Bash 5+](https://github.com/nudanos/vyatta-bash/blob/da737f7b4b70d577e2b6b723881f7d18336607fa/debian/control#L11-L13), [symlink list](https://github.com/nudanos/vyatta-bash/blob/da737f7b4b70d577e2b6b723881f7d18336607fa/debian/vyatta-bash.links). Native Arch still needs its own shell registration hook.

### Seams to extract before writing many PKGBUILDs

1. **One reproducible Go graph.** configd/config/yang/encoding and the small supporting libraries are still chiefly Debian GOPATH packages. Only selected repositories have partial module files. Establish a pinned common module workspace or generated vendored graph and common build/test targets. Keep repository import paths stable initially. Debian and Arch wrappers should invoke those targets rather than independently discover dependency versions.
2. **Eliminate package-manager-driven source rewriting.** `vci/debian/rules` queries `dpkg-query`, compares the installed go-systemd package version, and `sed -i` rewrites D-Bus API calls in source before build. [lines 14-32](https://github.com/nudanos/vci/blob/0499b9e2b4c5ee7883e0f0c397685cfd26d762c4/debian/rules#L14-L32). Commit one modern D-Bus API implementation and encode dependencies in the common graph. A native Arch Go build should not need dpkg or Debian source mutation.
3. **Common install contract.** Add `DESTDIR`, `prefix`, `libdir`, schema/model directory, component directory, runtime directory and helper paths as explicit build/install settings. Many runtime paths currently use `/opt/vyatta`, `/lib/vci`, `/usr/share/configd`, `/var/run`, while Debian manifests do the actual file installation. Preserve compatibility paths initially but own them centrally. `libvci/Makefile` hardcodes gcc/g++, writes pkg-config `libdir` using `DEB_HOST_MULTIARCH`, and lacks a shared full install target; packaging installs its bindings separately. [Makefile](https://github.com/nudanos/libvci/blob/69108123dab1bd64deb5fe3e7e77396e22ddbc7c/Makefile).
4. **Schema/component tools separate from package wrappers.** YANG syntax/import validation, model capability generation, VCI service/policy generation and schema cache reload are product tooling. `dh-yang`/`dh-vci` integration should be Debian adapters to that tooling, with Arch package hooks using the same idempotent commands. Publish an ownership/dependency manifest for modules rather than infer all product dependencies from `debian/control`.
5. **Explicit Linux identity/service contract.** Define configd/management user and group ownership, secrets visibility, socket modes, audit capabilities and D-Bus producer permissions as tested product requirements. Implement distro account creation with native tools/sysusers; install units/tmpfiles with a common layout. Avoid inheriting world-writable socket and runtime-directory policies as accidental compatibility requirements.
6. **Separate authentication and sandbox root provisioning.** PAM integration must have Debian and Arch implementations with the same behavioral test suite. The namespace code can remain shared. Build the sandbox root from the target distribution's repositories, not an apt-created trixie root on Arch. Verify files/sockets shared across namespaces and schema authorization from every northbound API.
7. **Treat Go plugins as one release unit.** AAA and XPath plugins use Go's `plugin` mechanism. XPath packaging already documents the need for matching trimpath flags to avoid package mismatch ([rules:10-18](https://github.com/nudanos/xpath-plugins/blob/b3e2dce52f2721954f01e2b0fc301ecf010520b5/debian/rules#L10-L18)). Compiler, dependency graph and flags must match consumers on both distros. A rolling Arch compiler update requires coordinated rebuilds; a numeric API version alone does not solve that. Prefer explicit out-of-process protocol boundaries for independently upgradable security plugins eventually.
8. **NETCONF is a separate port milestone.** `netconfd` has no trixie branch at review time despite the manifest naming it for 1.1, and is not marked ready. The inspected master SHA `0153a12ba11d0fde7b4bc97e9802507d49f59627` builds against the old `libnetconf` API. Select a maintained server/library boundary and preserve the existing configd adapter semantics and NETCONF tests. This is incomplete planned work, not a regression in the ready release set.

## Recommended gates

Before a release exposes HTTPS or privileged local management: close CP-01 through CP-05; run malformed RPC/auth/D-Bus cases under memory/race tooling; run identity tests from unprivileged accounts and service UIDs; verify no secrets in logs; prove state recovery after daemon failure and I/O errors. Fix CP-07 before claiming user-isolation support on either target when activation fails.

For native Arch: first prove the common core library build, configd service identity, VCI model installation and a simple CLI session in a clean Arch root. Then add native PAM/sandbox integration, feature components and northbound APIs. Use the same configuration fixtures and security tests on Debian and Arch; add target-specific package install/upgrade/reload tests. Keep an explicit support matrix for kernel-forwarding versus DPDK feature availability.

## Coverage and limits

Deep source tracing: configd RPC credential/dispatch/session/commit/recovery paths; REST authentication, session storage, process dispatch and shell helper paths; VCI D-Bus transport, policy and subscription handling; cli-sandbox PAM activation/build/root creation and selected namespace code; AAA loading/selection; Go plugin packaging.

Selected implementation/build reads, not exhaustive: config auth/tree/commit; yang compiler/XPath adapters; libvci C archive/SWIG packaging; op/opd command execution and identity; provisiond, ephemerad, notifyd; vyatta-cfg build entry points; vyatta-config-mgmt/migrate scripts; encoding/mgmterror; XPath plugins; objtree and jsouthworth dyn/etm/hash/immutable/seq/transduce/try libraries. No additional concrete defect established in these selected library reads.

Reference-only reads: old netconfd and standalone yangd; superseded encoding repository; upstream-replaced golang-dbus and pkcs8 repositories. The preserved `golang` toolchain and every upstream Go fork were not audited; they are not selected for native rebuilding by the current manifest.

All named active control-plane ports were cloned at pinned trixie heads, except future `netconfd`/`yangd` (master only). `control-plane-shas.json` records local checkout SHAs, including other team members' concurrently cloned repositories. This is a focused security/architecture review, not a full line-by-line audit of roughly 200k+ management-plane lines. No Linux runtime tests could be executed in this Windows environment: Go/C/C++/Perl and WSL distributions were unavailable. Findings are source-established and have not been reproduced against a booted router. The main reviewer separately inspected published nightly happy-path evidence; that does not cover these malformed-input or multi-UID cases.

The unsupported original base OS/toolchain, dead infrastructure, legacy-script migration and lack of modules are already discussed in `distro/docs/REVIEW.md`. This review does not count those observations again as newly discovered vulnerabilities. CP-01 through CP-07 are concrete current source findings absent from that preservation review; several originate in inherited code rather than NuDanOS changes.
