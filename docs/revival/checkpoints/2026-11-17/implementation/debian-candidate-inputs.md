# Debian candidate package inputs

The candidate replaces four source packages in the verified November 3 baseline pool; the unchanged package provenance and its limitations are recorded in `debian-baseline-provenance.md`. CI artifacts were downloaded read-only and their ZIP SHA256 values matched GitHub artifact metadata. The artifacts include binary/source packages and `.changes` checksums.

| Source | Successful CI run / artifact | CI head | Merge commit and tree verification | ZIP SHA256 |
|---|---|---|---|---|
| vyatta-debian-pam-configs-config | 42534693937 / 13638448199 | 541f763 | ea6eb3f0342b342eaf374219772be45081c4aea0; both trees e318851fc7f52607b37e8a34fb38e6a9b00b6e5e | 0cf2df9aeeaf6c290e44f701c30f70aec7aa33b3ce2b2018db1816cf66782f31 |
| cli-sandbox | 42534710912 / 13639315647 | 8c71637 | c6a637f7dfc28f37993aafa2f1bd65fca4d5a674; both trees 3dbb4d3da3045c8f699c8d07e19dd472aa2a7725 | e56546bc63a6cb4b06174236b1991cd694af762419dc6a265aa406f6dad9bb63 |
| vyatta-login | 42535159479 / 13638717769 | c7212c2 | 783ff2120a3b7894275cff4fe160565e2fdb4bc1; both trees 515b81eab91cacf4575324a5fe9f7d66a0afc8f2 | 5416d12b3987683d77af8110a79e2da63e50a06f9b56f67f5ef93b44060fc57c |
| configd | 42534721985 / 13639521755 | b63160d | 390d7bf6df5023fce63c9dc9878ba1d66e83ea29; both trees 295fd9beae4e4716675aed2fa3479a19edbdbb9e | d74c1b973def4da1609a3f2c133db479aaa768afeee55b6e17401d0f34bae40f |

The corrected login 2.44 binary's `Depends` field includes `configd (>= 2.16)` and `vyatta-password-renewal (>= 2.44)`. A verified input is not a tested candidate. Candidate pool assembly and repository signing are complete; the provisional ISO is complete and the corrected final ISO and appliance tests are pending.


## Repository and image inputs

The clean candidate build sets `NUDANOS_VERSION=1.0~20261109` from launch and uses source epoch `1792760262` from distro commit `864370dc3ff5a6086ce298b147f79ac2c4de315e`. The Debian base image digest is `sha256:9c6e381421d0f48d1b8133e573f33c185ee16cbf505eda9c4a8a4791b25ea8a6`. Debian mirrors remain mutable; bit reproducibility is not claimed. Verify the ISO volume and embedded version files before handoff.

The candidate pool was assembled after final merge-tree checks, using the source and split binary names from the baseline signed indexes. All four CI archives matched GitHub artifact ZIP digests, every staged output matched its `.changes` SHA256 and size, and the new repository's Release signature, index hashes, 648 binary payloads and indexed source payloads verified. The public test key fingerprint is `632DE3028B010AD7971CF88FC8FF6AE6F7B0295F`. The complete repository tar SHA256 is `47c46ced4b28c8373bd46ddb7ab0b7de39a63c6a4c20203a5a3d57791c72f589`; public key SHA256 is `e608bec258289850f1416e05809123b7b5740e3536ef68bb4599f197eef73479`. The extracted repository resides at `test-runtime/candidate-repository/repo`. It is solely for disposable local testing.

The signed baseline binary versions are configd 2.15, vyatta-login 2.43, cli-sandbox 0.31 and PAM config 1.12; candidate versions are 2.16, 2.44, 0.32 and 1.13, respectively, and Debian version comparisons succeeded. Candidate renewal 2.44 owns `pam_vyatta_passwd.so` and declares `Breaks` and `Replaces` against login before 2.44. Per-file candidate artifact checksums are in `debian-candidate-assembly.log`.

The first TCG attempt was cancelled after WHPX was verified; its package-only cache and log were retained. The first accelerated scratch attempt hit an APT file-descriptor scan caused by an excessive Docker default limit and its optional autoclean was interrupted; this is not counted as a pass. A later scratch attempt restored a bootstrap cache whose filesystem ownership had been changed during preservation; it was rejected before producing an ISO. The completed provisional attempt used only the original downloaded package cache, no copied bootstrap filesystem, and `--ulimit nofile=65536:65536`. Its mandatory bootstrap gate passed: `/`, `/etc`, and `/usr` 0:0:755; dpkg status 0:0:644; dpkg 0:0:755; passwd 0:0:4755; empty dpkg audit. Failed-attempt logs are retained in the guest.

Stock `nudanos-router` does not select the sandbox packages. For this acceptance ISO only, `implementation/12-test-passive-sandbox.chroot` was added before the source-cleanup hook. It explicitly installs passive `cli-sandbox=0.32`, `pam-sandbox=0.32` and `python3-shared-storage=0.32`, then runs bounded APT autoclean and requires an empty `dpkg --audit`. Hook SHA256: `c1c7007e3b8b4ac33354204856a3901de475f8e12efdb9c952896a7c3f62f4c5`. It does not install the optional user-isolation YANG package or activate isolation; those are left to appliance acceptance. No product metapackage was changed.


The upgrade acceptance test subsequently exposed a pre-reboot configd compatibility defect. The current ISO and repository are provisional fixtures for fresh-install diagnostics, not the final candidate. The final repository and image must incorporate the corrected, reviewed and merged configd artifact. A completed bootstrap cache for that rebuild was preserved with `docker cp -a` while the build container was paused, without recursive chown: bootstrap `/` and `/etc` remain 0:0:755, passwd remains 0:0:4755, dpkg status remains 0:0:644, and `dpkg --root=<cache/bootstrap> --audit` passed with no findings.

The provisional clean image build reported `P: Build completed successfully` and wrote guest path `/srv/nudanos/image-step2/image/nudanos-1.0~20261109-amd64.iso`. Its bootstrap ownership gate passed and the local sandbox hook passed its package, autoclean, and dpkg-audit checks. The ISO was transferred to `test-runtime/provisional-image/nudanos-1.0~20261109-amd64.iso`; guest and host SHA256 matched `02d21998c9421410ccb70a91711f02faf4fe35cd3c1067e89b0edb1430819b0c`. Its volume is `NuDanOS 1.0~20261109` and embedded ID is `1.0-20261109.1257`. The final filesystem dpkg audit was clean; setuid/setgid and capability inventories matched baseline. Root and several `/etc` include directories were root:root 775 rather than baseline 755 due checkout umask; this is a known provisional-image limitation. The isolated final build input include directories were normalized to 755 without changing file modes. The ISO remains a provisional fresh-install fixture pending the configd upgrade fix.


