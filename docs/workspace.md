# Workspace setup

DANOS sources contain file names that differ only by case, and macOS volumes are
case-insensitive by default. `distro-build` refuses to run in such a directory.

On macOS, create a case-sensitive sparse volume once:

```bash
hdiutil create -type SPARSEBUNDLE -fs "Case-sensitive APFS" -size 80g -volname nudanos ~/nudanos-work.sparsebundle
hdiutil attach ~/nudanos-work.sparsebundle   # after each reboot
```

With Docker Desktop, add `/Volumes/nudanos` under Settings → Resources → File sharing.
On Linux (including a Debian 13 VM), any ext4/xfs directory works.

Requirements: Go 1.26+, git, and Docker or Podman (`-engine podman`).
