# Build, CI and release review

Reviewed distro commit `864370dc3ff5a6086ce298b147f79ac2c4de315e` on 2026-11-03. This is a static source review plus local Python unit tests and inspection of actual GitHub CI results. Linux package builds and router VMs were not run locally.

## Current evidence

- Organization inventory: 195 unique repositories, 47 archived.
- Manifest: 197 entries: 124 danos, 32 debian, 15 upstream, 15 drop, 10 reference, 1 apt. Ready: 96 danos, 11 upstream, 1 apt. All 96 ready danos refs are floating `trixie` branches; 95 are milestone 1.0 lint scope.
- Current distro unit CI passed: https://github.com/nudanos/distro/actions/runs/39957309285
- 2026-11-03 nightly passed: https://github.com/nudanos/distro/actions/runs/41652774933
- Nightly log reports install/purge of 1,097 packages; 39 configuration fixtures; BGP, OSPF, REST, SNMP, TACACS, VRRP gating passes and MPLS/LDP reported-only pass.
- Nightly lint: 9,923 findings in 48 ports, 95 checked. Committed allowance is 10,848 findings in 48 ports. These are mixed style/correctness findings, not 9,923 confirmed bugs.
- Local tests: launch suite 8/8, tools suite 42/42. Tools suite initially hit sandbox temp-directory permissions; rerun with a writable test temp path passed. No production source modification.
- Python executable was the bundled runtime. Go/Docker/Linux runtime unavailable here; a read-only attempt to obtain a local Go toolchain failed DNS, and no toolchain was installed.
- Latest release https://github.com/nudanos/distro/releases/tag/nightly-20261103 contains ISO and SHA256 sidecar and explicitly declares no firewall/NAT/QoS.

## Findings

### B1 — Release provenance is not locked across component branches (high release-engineering priority)

Evidence: manifest.yaml refs use `trixie`; internal/manifest/manifest.go validates full commit IDs only for upstream packaging_ref, while danos refs can float. cmd/distro-build/main.go:104-157 re-fetches sources; cmd/distro-build/lint.go:113 re-fetches again for lint after the build. internal/fetch/fetch.go:45-62 follows current origin branch tips. The nightly release body identifies only the distro SHA; main.go:381-411 derives image version from the distro commit date while both component branches and Debian archives can change independently.

Consequence: one distro commit/image version does not identify a complete source set. A push during a run can also cause source analyzers to examine different component code from that which produced the tested artifacts. Existing package/version manifests help inventory but do not provide a verified immutable full-input lock.

Suggestion: resolve an explicit release lock once, including every component commit, upstream tag's resolved commit, packaging commits, builder digest, repository snapshot/package versions and artifact checksums. Build, lint and test that exact set; publish it with the release. Development manifests can continue following trixie. Include a lock/release identifier in the image version.

### B2 — Builder changes and external build-dependency updates do not invalidate the package cache (medium)

Evidence: cmd/distro-build/main.go:286-296 returns the hash of builder/; :360-361 builds KeySalt from the image tag plus that directory hash. internal/engine/engine.go:116-128 builds with --pull. builder/Dockerfile:2-9 uses mutable debian:trixie and live apt repositories. The actual image ID and installed dependency versions are absent from the cache key (internal/build/build.go:197-208).

Consequence: rebuilding the same Dockerfile against updated Debian packages can reuse outputs from the previous builder/dependency set. The code does correctly invalidate on source changes and transitive in-graph source dependency changes; retain that behavior.

Suggestion: key on actual builder content ID plus the locked external build-dependency set/repository snapshot. Offer a clean release build and reproduce from the same locked snapshot. Record .buildinfo and compare independently built artifacts.

### B3 — A partial mirrored package cache is treated as complete (medium)

Evidence: internal/aptmirror/aptmirror.go:71-72 accepts the cache if its key matches and build.HasArtifacts(out) finds any one .deb (build.go:74-78). The loop verifying every pinned package runs only after a new download (:80-84). Thus removing one of the four FRR/libyang mirrored binaries while leaving another bypasses verification on the next mirror call.

Consequence: local cleanup or an incomplete restored cache can be signed and published with missing binaries, failing later install/image steps rather than being repaired by the mirror stage.

Suggestion: verify expected package name/version/architecture, each artifact digest and required source artifacts on every cache hit. Treat missing/corrupt artifacts as a cache miss. Test deletion of one sibling .deb and a truncated artifact.

### B4 — Upstream update checking reports no issue when a pinned APT package disappears (medium)

Evidence: internal/updates/updates.go:79-95 fills a map of served packages, then only reports a pin if it is present and its version differs (`if v, ok := served[n]; ok && ...`). A missing name is silently skipped.

Consequence: the weekly checker can report current pins while a required package has been removed from the external feed, even though a fresh mirror/build will fail.

Suggestion: missing pinned names must produce an unavailable/error result. Test missing packages, empty indexes, multiple offered versions, source/binary version differences and network errors separately from normal upgrade drift.

### B5 — Count-only lint budgets allow new correctness findings to replace cleaned-up style findings (medium improvement)

Evidence: internal/lint/baseline.go:12-42 stores only port -> count. The nightly 9,923 findings are within a 10,848 total budget.

Consequence: deleting several formatting findings can admit a new dangerous warning with no count increase. The current ratchet is useful for existing debt but insufficient for a security/behavior regression gate.

Suggestion: retain debt accounting but baseline stable fingerprints (tool/rule/path/context), fail on every new correctness/security finding, and classify style separately. Add ASan/UBSan to C/C++ tests, Go race tests and fuzzing for JSON/RPC/YANG/authentication boundaries, plus vulnerability scans that understand Go and distro package versions. Do not require cosmetic rewrites before correcting behavior.

### B6 — Repository/release signing should become an end-to-end update contract (release improvement)

Evidence: builder/make-repo.sh signs APT indexes offline and .github/workflows/nightly.yml:137-140 publishes ISO + checksum, without a detached image signature. An SHA256 sidecar hosted with the image is integrity metadata, not an independently authenticated publisher identity. The image-updater source needs matching verification; the packaging reviewer covers that path.

Suggestion: publish a signed release manifest binding ISO and lock hashes, verify it against a shipped dedicated NuDanOS key before installation, define rotation/revocation/replay policy, and test missing/wrong/expired signature behavior. Include license/source/buildinfo assets. Review actual GitHub environment protection settings separately; YAML declaring environment: release does not establish their configuration.

## Existing strengths to keep

Strict manifest field checking; topological build graph with binary/runtime closure; source+transitive dependency hashing; read-only source mounts and non-root package compilation; separate offline APT signing container; install/remove/purge/idempotence tests; BIOS + UEFI testing; captured 2105 fixtures; multiple real routing/service scenarios; source patch audit entries; upstream update checks; lint debt ratchet.

## References

Pinned source base: https://github.com/nudanos/distro/tree/864370dc3ff5a6086ce298b147f79ac2c4de315e

