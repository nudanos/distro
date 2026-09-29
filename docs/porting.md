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
5. **Push** `trixie` and add `.github/workflows/package.yml` (the caller: `uses: nudanos/distro/.github/workflows/package.yml@main`, `with: package: <repo>`).
6. **Record.** Make sure `ready: true` is committed in `manifest.yaml`.
