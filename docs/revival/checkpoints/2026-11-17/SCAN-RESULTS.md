# Checkpoint screening results

- Staged text: strict UTF-8, LF line endings, no BOM; final manifest records exact bytes. Original copy hashes precede redaction and normalization.
- Ten staged runtime controller/probe copies had transient fixture values or a generated rescue command removed; see REDACTED-PATHS.txt. The public stock live-login default and already-public synthetic unit-test inputs are intentionally retained. Redacted harnesses require new disposable fixture configuration.
- Screening found no actual private key or high-confidence token marker in staged text, and no forbidden runtime binary/package extensions, Git metadata, or reparse points. This is a bounded scan, not a guarantee that all sensitive information was detected.
- Twenty-eight historical source archives were listed and screened before inclusion for unsafe paths, links, special files, binary/runtime payloads and credential signatures. A private-key header marker is an archive shell validation pattern, not key material. Archives remain byte-identical; member permissions are recorded in historical/archive-modes.txt.
- All 159 task3 guest-stage files are historical. Two otherwise ignored distro notes are distinct and included. The four REST drafts are experimental historical material.
- The existing Debian helper was inventoried, reviewed authored sources exported, and shut down. Guest raw logs, disks, keys and runtime payloads stay local and are excluded. Historical Arch snapshots are host exports; arbitrary stopped guests were not exhaustively inventoried.
- Script shebangs in the manifest denote executable intent, not verified standalone Windows file mode. Source archive member modes are preserved in the archive inventories.
