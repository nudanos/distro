# Guest export inventory

The existing Debian helper was started under its already available WHPX configuration solely for read-only inventory and source export, then shut down. No APT transactions, package builds, image builds, code implementation, host feature changes, or service modifications were performed.

`image-wrappers/` preserves three guest-authored wrapper variants. They are historical evidence: the previously attempted image and its bootstrap-cache approach were rejected; do not treat these wrappers as an approved build recipe. The current bootstrap ownership gate and other scripts are in `../implementation/`; guest hashes matched those host copies. `historical/network-security/`, `historical/rest-defensive-tests/` and `historical/rest-red-tests/` preserve reviewed source/test snapshots, including synthetic fixture inputs. They are not current production checkouts.

The guest dependency checkouts contained no substantive tracked changes after ignoring file-mode and line-ending noise; their lone untracked entry was a symlink to a system Go library. Cloned upstream trees, `.git` directories, that symlink, generated build trees, package artifacts and caches were not exported. Guest `revival-checks` source snapshots were no newer than the corresponding staged historical archives; five host archive hashes matched their guest originals. These checks are bounded and do not prove that every possible file in the VM has been reproduced.

`guest-evidence-summary.txt` contains only fixed marker counts, original log SHA256 digests and line counts. Original image/build/REST logs remain inside the local guest and are excluded from Git; no raw serial or credential log is included. Historical reports and test fixture strings are not live credentials.
