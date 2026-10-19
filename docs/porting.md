# Porting a DANOS repository to Debian 13

1. **Branch.** Clone from the org and branch from the original default branch:
   ```bash
   git clone git@github.com:nudanos/<repo>.git /Volumes/nudanos/port-<repo>
   git -C /Volumes/nudanos/port-<repo> checkout -b trixie
   ```
2. **Checklist.** `python3 tools/port.py /Volumes/nudanos/port-<repo>`. Read every `NOTE:`.
3. **Build locally**, from `distro/`, against every ready package:
   ```bash
   ./distro-build -work /Volumes/nudanos/work -jobs 4 -local <repo>=../port-<repo> build <repo>
   ```
   The entry must be `ready: true` in `manifest.yaml` for `-local` to apply.
4. **Fix failures at the cause.** Common ones on Debian 13:
   - GCC 14 errors: `-Wincompatible-pointer-types`, `-Wimplicit-function-declaration`, `-Wint-conversion`
   - `dh_missing --fail-missing`: list intentionally unshipped files in `debian/not-installed`
   - Python 3.13 removals (`imp`, `distutils`, `pipes`)
   - Perl 5.40 deprecations
   - Go vet failures in tests under Go 1.26
   - lintian errors

   Commit each fix separately, with the reason in the message.
5. **Record, then push.** Commit and push `ready: true` in `manifest.yaml` **first**: each push triggers the package CI, which rejects `-local` for an entry that isn't ready yet. Then push `trixie` and add `.github/workflows/package.yml` (the caller: `uses: nudanos/distro/.github/workflows/package.yml@main`, `with: package: <repo>`).

## Code quality: lint and staticcheck

`distro-build lint` checks every ready 1.0 port
(`docs/superpowers/specs/2026-10-14-code-quality-design.md`). Two of its checks
come from the package build log (`work/logs/<name>.log`), not from the lint
image:

- **C and C++** ports are built with `-Wall -Wextra -Wno-error` added
  (`DEB_CFLAGS_MAINT_APPEND`, `DEB_CXXFLAGS_MAINT_APPEND`; `-Wno-error` keeps a
  port's own `-Werror` from failing the build on them). Every `warning:` in
  the port's own files is a finding.
- **Go** ports run staticcheck in their build, next to `go vet`, because it
  needs the build's dependencies. Add `go-staticcheck <!nocheck>` to
  Build-Depends (Debian installs it as `go-staticcheck`) and this target, run
  after `vet` in `override_dh_auto_build`,
  using the same package list and `GOPATH` as the port's `vet` target:

  ```make
  override_dh_auto_build: vet staticcheck
  	dh_auto_build

  staticcheck:
  	GOPATH=$(GO_PATH) go-staticcheck <packages> 2>&1 | sed 's/^/nudanos-staticcheck: /' || true
  	@echo 'nudanos-staticcheck: done'
  ```

  The build does not fail on staticcheck findings; lint counts them. A Go
  port whose log lacks `nudanos-staticcheck: done` has the finding
  `staticcheck-not-run`.

A finding is silenced only inline, in the tool's own syntax, with
`reason: <why>` on the same line; lint fails on a suppression without one. A
port's only other exception is `nudanos-lint.yaml` with a `skip:` list of
paths (generated files, test data, code outside the 1.0 build): `path.Match`
globs, or a directory followed by `/` to skip its whole tree.
