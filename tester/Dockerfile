# NuDanOS boot tester: QEMU with BIOS and UEFI firmware.
FROM debian:trixie
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y --no-install-recommends \
      qemu-system-x86 qemu-utils ovmf ca-certificates snmp openssh-client \
 && rm -rf /var/lib/apt/lists/*
