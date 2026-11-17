# Serial Terminal Implementation Plan

> Execute inline with superpowers:executing-plans. Root reviews the final diff before commit/push.

**Goal:** Restore sandboxed Debian serial login while exposing only the validated session terminal to privileged login reopening.

**Architecture:** A small C helper captures immutable controlling-terminal identity before retries and creates a root-only alias after sandbox chroot. Existing SSH PTYs and excluded users retain their existing flow.

**Tech Stack:** C, Linux terminal/device APIs, PAM, Debian package rules, isolated QEMU WHPX guest.

**Spec:** `implementation/serial-terminal-design.md`.

## Constraints and review focus

- No PAM_TTY input, arbitrary path, whole-/dev bind, copied user permissions or user-writable destination directory.
- Capture before retries; validate source inode and device; skip canonical PTYs and noncontrolling fd0.
- Reject symlink, foreign device, writable/unowned /dev, and broad pre-existing alias metadata.
- Test unprivileged alias denial and cross-user console reuse, not only root node creation.
- Preserve read-only baseline/old-working disks and use a separate build overlay.

## Implementation and validation

- [x] Write C/private-namespace regression harness before helper implementation; retain actual serial failure trace as appliance RED evidence.
- [x] Add terminal helper and observe initial missing-node failure, then implement capture and root-only alias validation.
- [x] Integrate capture once before retry and installation after chroot; update Makefile/tests and changelog0.33.
- [x] Build exact source with stock Debian rules on independent overlay; run C tests, existing suite and namespace tests.
- [x] Install candidate0.33 on test overlay, test actual serial and SSH sessions, denial/boundaries/teardown and console reuse.
- [x] Root reviews diff before commit/push; retain logs and exact source/package hashes.
- [x] Upgrade separate old-root fixture with corrected auth packages and0.33, retaining oldroot package inventory; test renewal fallback and serial/SSH.
