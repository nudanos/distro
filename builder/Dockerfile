# NuDanOS package builder: Debian 13 "trixie" plus trixie-backports.
FROM debian:trixie
ENV DEBIAN_FRONTEND=noninteractive LANG=C.UTF-8
RUN echo 'deb http://deb.debian.org/debian trixie-backports main' > /etc/apt/sources.list.d/backports.list \
 && apt-get update \
 && apt-get install -y --no-install-recommends \
      build-essential devscripts dpkg-dev fakeroot lintian apt-utils equivs \
      git ca-certificates curl gnupg xz-utils \
 && rm -rf /var/lib/apt/lists/* \
 && useradd --create-home --uid 1000 builder
COPY preferences /etc/apt/preferences.d/nudanos
COPY build-package.sh /usr/local/bin/build-package
RUN chmod 0755 /usr/local/bin/build-package
COPY mirror-apt.sh /usr/local/bin/mirror-apt
COPY make-repo.sh /usr/local/bin/make-repo
RUN chmod 0755 /usr/local/bin/mirror-apt /usr/local/bin/make-repo
