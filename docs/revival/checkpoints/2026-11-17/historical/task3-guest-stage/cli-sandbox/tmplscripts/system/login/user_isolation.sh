#!/bin/bash
# Copyright (c) 2018 AT&T intellectual property.
# All rights reserved
#
# SPDX-License-Identifier: GPL-2.0-only

PAM_PROFILE=sandbox
PAM_PROFILE_SRC=/opt/vyatta/share/pam-configs/${PAM_PROFILE}
PAM_PROFILE_DST=/usr/share/pam-configs/${PAM_PROFILE}
PAM_SESSION=/etc/pam.d/common-session
PROG=$0

err_exit()
{
	echo "${PROG}: ${1}" >&2
	exit 1
}

update_pam()
{
	DEBIAN_FRONTEND=noninteractive pam-auth-update "$@"
}

sandbox_active()
{
	grep -Eq '^[[:space:]]*-?session[[:space:]]+[^#]*[[:space:]/]pam_sandbox\.so([[:space:]]|$)' "$PAM_SESSION"
}

sandbox_required()
{
	grep -Eq '^[[:space:]]*session[[:space:]]+required[[:space:]]+(/[^[:space:]]*/)?pam_sandbox\.so[[:space:]]*(#.*)?$' "$PAM_SESSION"
}

check_prerequisites()
{
	[ -r /etc/os-release ] || err_exit "cannot determine the operating system"
	# shellcheck source=/dev/null
	. /etc/os-release
	case ${ID:-} in
		debian|danos) ;;
		*) err_exit "user isolation requires a supported Debian PAM and systemd integration" ;;
	esac
	[ -r "$PAM_PROFILE_SRC" ] || err_exit "missing PAM profile $PAM_PROFILE_SRC"
	if [ ! -x /usr/bin/systemd-nspawn ] || [ ! -x /opt/vyatta/sbin/cli-sandbox ] ||
		[ ! -x /opt/vyatta/sbin/cli_sandbox_init ] ||
		[ ! -r /etc/cli-sandbox/nspawn-templates/cli_sandbox.nspawn ] ||
		[ ! -r /var/lib/sandbox/etc/os-release ] || [ ! -x /var/lib/sandbox/bin/vbash ]; then
		err_exit "sandbox runtime or root filesystem is incomplete"
	fi
	systemctl cat cli-sandbox@.service systemd-machined.service >/dev/null ||
		err_exit "sandbox systemd units are unavailable"
	local arch module_dir
	arch=$(dpkg --print-architecture) || err_exit "cannot determine the PAM module architecture"
	module_dir=$(dpkg-query --listfiles "libpam-modules:$arch" | sed -n 's|/pam_unix\.so$||p')
	if [ -z "$module_dir" ] || [ ! -r "$module_dir/pam_sandbox.so" ]; then
		err_exit "PAM module pam_sandbox.so is unavailable in the native PAM directory"
	fi
}

action=${1:-enable}
command -v pam-auth-update >/dev/null || err_exit "pam-auth-update is unavailable"
[ -r "$PAM_SESSION" ] || err_exit "cannot read $PAM_SESSION"
case ${action} in
	disable)
		update_pam --disable "$PAM_PROFILE" || err_exit "could not disable the PAM profile"
		[ -r "$PAM_SESSION" ] || err_exit "cannot read $PAM_SESSION"
		if sandbox_active; then
			err_exit "PAM still loads pam_sandbox.so; review $PAM_SESSION"
		fi
		if cmp -s "$PAM_PROFILE_SRC" "$PAM_PROFILE_DST"; then
			rm -f "$PAM_PROFILE_DST" || err_exit "could not remove $PAM_PROFILE_DST"
		fi
		echo "User Isolation Disabled"
		;;
	enable)
		check_prerequisites
		created=false
		if [ ! -e "$PAM_PROFILE_DST" ] && [ ! -L "$PAM_PROFILE_DST" ]; then
			install -m 644 "$PAM_PROFILE_SRC" "$PAM_PROFILE_DST" ||
				err_exit "could not install $PAM_PROFILE_DST"
			created=true
		elif ! cmp -s "$PAM_PROFILE_SRC" "$PAM_PROFILE_DST"; then
			err_exit "$PAM_PROFILE_DST differs from the packaged profile"
		fi
		if ! update_pam --enable "$PAM_PROFILE" || ! sandbox_required; then
			# Remove a newly staged profile only after confirming it isn't live.
			if "$created" && sandbox_active; then
				update_pam --disable "$PAM_PROFILE" ||
					err_exit "could not roll back the PAM profile; review $PAM_SESSION"
			fi
			if "$created" && [ -r "$PAM_SESSION" ] && ! sandbox_active; then
				rm -f "$PAM_PROFILE_DST" || err_exit "could not roll back $PAM_PROFILE_DST"
			fi
			err_exit "could not activate the PAM profile; review $PAM_SESSION"
		fi
		echo "User Isolation Enabled"
		;;
	*)
		err_exit "error: unknown argument '${1}'"
		;;
esac

exit 0
