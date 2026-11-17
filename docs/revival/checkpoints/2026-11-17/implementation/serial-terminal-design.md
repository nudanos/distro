# Scoped serial terminal design

Approved by the root task after source/security review. A real Debian 13 serial login currently enters its nspawn sandbox, then util-linux login fails reopening `/dev/ttyS0` before starting the shell. SSH PTY sessions already work.

Capture the actual controlling fd0 terminal once before PAM's namespace-entry retry loop. Verify ttyname's path is a direct `/dev` basename, not a symlink, and its character-device inode/device matches fstat(fd0). Require tcgetsid(fd0) to match the caller's session. Skip nonterminal descriptors, noncontrolling descriptors and canonical numeric `/dev/pts/N` paths. Never consume PAM_TTY or another caller-supplied path.

After chroot, create only that character device beneath a no-follow, root-owned `/dev` that is not group/other writable. The alias must remain root:root mode000. This is deliberately narrower than copying source ownership: copying user-owned modes could leave stale rights when a shared per-user sandbox survives console logout. Atomic EEXIST validation accepts only the exact character-device number with root ownership and mode000; symlinks, wrong devices and broader existing permissions fail closed. Administrative custom terminal binds can therefore be refused rather than modified.

Util-linux v2.41.5 login chowns its already-open original fd0 after PAM, then reopens the terminal pathname in its root child before dropping UID. A root-only alias supports this flow without granting sandbox users a reusable device-open right. Source: https://raw.githubusercontent.com/util-linux/util-linux/v2.41.5/login-utils/login.c (chown_tty, fork_session and main). The actual Debian syscall trace agrees with this sequence through the failing open.

Alternative considered: a pinned bind mount of the host terminal. It requires more complex cross-namespace descriptor/mount handling, and permissions/lifetime require careful synchronization. Blanket device-directory binds and arbitrary PAM_TTY paths are excluded. The approved root-only node is smaller and does not grant unprivileged device-open rights.

Validation must include private-namespace/helper security regressions, an actual serial getty login, SSH preservation, root reopen versus unprivileged alias denial, console reuse between fixture users, namespace/host-marker boundaries, and clean teardown. A separate build overlay is used; the old working 0.31 root and baseline master stay read-only. Release is cli-sandbox0.33, with root source review before commit/push.
