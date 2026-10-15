# NuDanOS lint: the trixie analyzers distro-build lint runs (spec 2026-10-14).
# Kept apart from builder/: the builder's hash is part of every package's
# cache key.
FROM debian:trixie
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y --no-install-recommends \
      golang-go perltidy libperl-critic-perl shellcheck flake8 pylint cppcheck \
 && rm -rf /var/lib/apt/lists/*
COPY perltidyrc perlcriticrc pylintrc /lint/
