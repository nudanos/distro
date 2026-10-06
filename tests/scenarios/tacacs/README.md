# tacacs

New for NuDanOS (no DANOS suite). The runner serves TACACS+ (`internal/tacacs`)
on 127.0.0.1:49, which routers reach as 10.0.2.2:49. Checks: an administrator
(priv 15) and an operator (priv 1) log in with the right rights, a wrong
password is refused, and the local administrator still logs in when the
server is unreachable.
