#!/bin/bash
# Index every .deb under /pool (mounted read-write) as a flat apt repository.
# Run once per build tier, so builds read the pool in place instead of copying it.
set -euo pipefail
cd /pool
apt-ftparchive packages . > Packages
gzip -9kf Packages
xz -kf Packages
