# NuDanOS plan 3: "Boots" (design)

Addendum to `2026-09-28-debian13-revival-design.md` (the spec). Plan 3 takes the
milestone-1.0 package set, which plan 2 ported and builds (103 ready entries, CI and
nightly green), to a bootable, installable system.

**Status (2026-10-01):** brainstorming in progress.

| Section | State |
|---|---|
| 1. Scope and success criteria | **Approved** by the user |
| 2. Interfaces under kernel forwarding | **Approved** by the user |
| 3. Image | **Draft**, not yet reviewed with the user |
| 4. Tests and CI | **Draft**, not yet reviewed with the user |

Next steps, in order: review sections 3 and 4 with the user, write the final spec
(this file, with drafts resolved), get the user's approval of the written spec,
then write the implementation plan (`superpowers:writing-plans`) and ask the user
to choose an execution method. No implementation before that.

Decisions the user took while scoping (2026-10-01):

- Plan 3 is **Boots** (spec items 1 + 2 below). Multi-VM scenarios, the 2105
  compatibility fixtures, the kernel vendor-commit audit and the 8 deferred
  tooling minors are **plan 4**.
- Physical interfaces keep **DANOS dataplane names** (`dp0sN`) under kernel
  forwarding, so 2105 configs load unchanged and VRRP, per-interface OSPF/BGP,
  bridge and bonding (which augment only `interfaces dataplane`) keep working.
  This supersedes spec §8.4's "kernel interface names instead of `dp0sX`".
- Login: the **live ISO keeps `vyatta`/`vyatta`**; `install image` requires a new
  password and rejects `vyatta` before writing the disk.
- Image tooling: **live-build**, porting DANOS's `build-iso` config (GPL-2.0, per-file
  SPDX). `vyatta-image-tools` (install image, add system image, rollback) expects
  the live-boot squashfs layout, which rules out mmdebstrap-by-hand and
  debian-installer.

## 1. Scope and success criteria (approved)

Plan 3 is done when the nightly, unattended, on a fresh build:

1. builds a hybrid ISO from the signed repo with live-build in a container;
2. passes **layer 2**: in a clean `debian:trixie` container, `nudanos-router`
   installs, removes and purges with no maintainer-script errors and no leftover
   files outside documented state directories;
3. passes **layer 3** in QEMU: boot the ISO, serial-console login as
   `vyatta`/`vyatta`, `configure`, set an address on `dp0s3`, the hostname and a
   user, `commit`, `save`, reboot, verify the config, `install image` to a virtual
   disk (refusing the default password), boot from disk, verify again,
   `show version` reports Debian 13, FRR 10.7 and the kernel;
4. on success, publishes the ISO as a GitHub Release artifact.

## 2. Interfaces under kernel forwarding (approved)

- **Naming.** `vyatta-kernel-forwarding` ships a udev rule that renames each
  kernel NIC from its predictable name to the dataplane name by replacing the `en`
  prefix with `dp0` (`enp0s3` → `dp0s3`, `enp2s0f1` → `dp0p2s0f1`). Names it cannot
  map (USB NICs etc.) are left alone and appear under `interfaces system`.
- **Type table.** It also ships the `dp → dataplane` netdevice entry in its own file
  under `/opt/vyatta/etc/netdevice.d/`, so `Vyatta::Interface` recognises `dp*`
  without `vplane-config` (which is DPDK-only: it depends on the dataplane protocol
  virtuals and on `iproute (>= …-vyatta)`, absent from trixie).
- **Model untangling.** `vyatta-interfaces-dataplane-v1-yang` (built from
  `vyatta-cfg-dataplane`) changes `Depends: vplane-config` to
  `vplane-config | vyatta-kernel-forwarding`, and its protocol-version dependency
  (`vyatta-dataplane-op-ifconfig-1`) gets the same alternative. Its other
  dependencies (`vyatta-interfaces-switch-v1-yang`, `vyatta-update-vifs`, …) must be
  installable without DPDK; anything it needs from `vplane-config`'s scripts moves
  to a package both can use. **The plan's first task is an inventory** of what the
  dataplane interface model's configd actions (`configd_create.d`,
  `configd_delete.d`, `vyatta-interfaces.pl`, `vplane-affinity`, …) call, sorting
  each into: works on a kernel netdev / must be a no-op / must be refused.
- **DPDK-only settings** (affinity, breakout, hardware binding) are refused at
  commit under kernel forwarding with a clear message ("requires the DPDK
  dataplane"), not an obscure failure. Op commands that query the DPDK process
  (`show dataplane …`) say the same.
- **Runtime fixes in scope** (from the plan 2 ledger, plus whatever the boot test
  exposes):
  - port `vyatta-service-ntp` to ntpsec (`/etc/ntpsec/ntp.conf`, `ntpsec.service`,
    `/etc/default/ntpsec`; drop `ntpq -c privatestat`);
  - `vyatta-bridge.pl` must keep kernel `multicast_flood`/`broadcast_flood` on
    bridge ports under kernel forwarding (it turns them off because "the
    dataplane implements flooding");
  - `vyatta-vrrp` writes `start_delay`, which no keepalived release supports; map
    it to what keepalived 2.4.3 offers;
  - `libvyatta-interface-perl` uses `Vyatta::ioctl` from `vyatta-system` without
    declaring it.

## 3. Image (DRAFT — not yet reviewed with the user)

- **`nudanos-router` meta-package** (spec §4.5). Depends on the milestone-1 binary
  package set that installs without DPDK; the ISO installs it, and so will
  `install.sh` (M1.5). A CI check generates the list from the manifest and fails if
  the meta-package drifts. *Open:* where its source lives — a new
  `nudanos/nudanos-router` repo (creating a repo needs the user's go-ahead) or a
  native package inside `distro`.
- **Image hooks move into packages** (spec §4.5): user-isolation sandbox
  (`0990-create-chroot-fs`) → `cli-sandbox` postinst; iptables/ebtables legacy
  alternatives (`14-iptables-legacy`) → `vyatta-kernel-forwarding` postinst;
  `os-release` (`96-os-release`) → `base-files-vyatta` postinst; loopback in
  `/etc/network/interfaces` (`01-interfaces`) → `vyatta-cfg-system` postinst;
  `98-yangcheck` → a build-time CI check. Only live-boot/ISO hooks stay in `image/`
  (`01-boot_live`, `02-live-vmlinuz`, `03-live-config`, `00-mk_buildid`,
  `00-manifest`, `99-cleanup`, locale, busybox).
- **`distro/image/`**: DANOS `build-iso` (GPL-2.0) ported to live-build on trixie.
  Package lists collapse to `nudanos-router` + live-boot/live-config + the kernel
  (Debian `linux-image-amd64`; the spec says trixie-backports — confirm against the
  trixie stable kernel during planning) + bootloaders (GRUB EFI, isolinux for
  BIOS). DANOS-only lists (dataplane, opennsl, PoE, DPI, flowmon, …) are dropped.
- **`distro-build image`**: runs `lb build` in a privileged builder container
  against the signed repo plus Debian, with `SOURCE_DATE_EPOCH` set for
  reproducibility. Output: `work/image/nudanos-<version>-amd64.iso`.
- **`install image`**: makes the existing password prompt
  (`vyatta-install-image.functions:_dialog_enter_password`) mandatory for the
  administrator and rejects `vyatta`.

## 4. Tests and CI (DRAFT — not yet reviewed with the user)

- **Layer 2, `distro-build test install`**: clean `debian:trixie` container with
  the signed repo; install `nudanos-router`, remove, purge; fail on
  maintainer-script errors or files left outside an allowlist of state directories.
- **Layer 3, `distro-build test boot`**: a Go driver for QEMU over the serial
  console (unix socket, expect-style steps with per-step timeouts) running the
  section 1 script. KVM when `/dev/kvm` exists (GitHub runners), TCG emulation
  otherwise (the user's Intel Mac: Docker Desktop has no nested KVM; slow but
  usable for debugging). Every run keeps the full serial transcript as a log and
  CI artifact.
- **CI**: the nightly gains image → layer 2 → layer 3 → release jobs. If a cold
  build plus image exceeds the 6-hour job limit, split per the spec (§7.3). The
  ISO goes to a GitHub Release only when layers 1–3 pass.
- **Error handling**: each boot-test step names what it waited for and shows the
  last serial lines on failure; QEMU is always torn down; a hung boot fails at its
  timeout rather than hanging the job.

## Environment facts for whoever continues

See `docs/superpowers/handoffs/2026-10-01-plan3.md`.
