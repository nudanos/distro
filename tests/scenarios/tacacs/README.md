# tacacs

New for NuDanOS (no DANOS suite). NuDanOS logs TACACS+ users in through
pam_tacplus and nss_tacplus (a TACACS+ user maps to the local account tacacsN
of its privilege level); DANOS 2105 used a TACACS+ provider in its own sssd.
The runner serves TACACS+ (`internal/tacacs`)
on 127.0.0.1:49, which routers reach as 10.0.2.2:49. Checks: an administrator
(priv 15) and an operator (priv 1) log in with the right rights, a wrong
password is refused, and the local administrator still logs in when the
server is unreachable.

The checks are validated on NuDanOS only: 2105's sssd provider never resolved
the runner's users, so on 2105 the scenario captures the configuration and its
login checks fail. nss_tacplus reads the root-only `/etc/tacplus_servers`, so
only root (sshd) resolves TACACS+ users; the scenario has no `id` check from
the admin's console for that reason.
