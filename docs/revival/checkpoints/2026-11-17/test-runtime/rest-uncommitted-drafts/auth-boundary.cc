// SPDX-License-Identifier: GPL-2.0-only
// Exercise the real authenticators. PAM/audit are external boundaries: never
// change this machine's login configuration or authenticate a real account.
#include <cassert>
#include <cstdarg>
#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <ctime>
#include <fcntl.h>
#include <openssl/evp.h>
#include <security/pam_appl.h>
#include <sys/stat.h>
#include <sys/wait.h>
#include <unistd.h>
#include "authbasic.hh"
#include "authsession.hh"

static std::string expected_user, expected_password, logs;
static int pam_calls;
static bool pam_accept = true;
extern "C" int pam_start(const char *, const char *user, const pam_conv *conv, pam_handle_t **p)
{
    ++pam_calls;
    assert(expected_user == user);
    pam_message message = { PAM_PROMPT_ECHO_OFF, "Password:" };
    const pam_message *messages[] = { &message };
    pam_response *response = NULL;
    assert(conv->conv(1, messages, &response, conv->appdata_ptr) == PAM_SUCCESS);
    assert(expected_password == response[0].resp);
    free(response[0].resp);
    free(response);
    *p = reinterpret_cast<pam_handle_t *>(1);
    return PAM_SUCCESS;
}
extern "C" int pam_authenticate(pam_handle_t *, int) { return pam_accept ? PAM_SUCCESS : PAM_AUTH_ERR; }
extern "C" int pam_acct_mgmt(pam_handle_t *, int) { return PAM_SUCCESS; }
extern "C" int pam_end(pam_handle_t *, int) { return PAM_SUCCESS; }
extern "C" int audit_setloginuid(uid_t) { return 0; }
extern "C" void vsyslog(int, const char *format, va_list ap)
{
    char buf[8192];
    vsnprintf(buf, sizeof(buf), format, ap);
    logs += buf;
}
static std::string basic(const std::string &plain)
{
    std::string encoded(4 * ((plain.size() + 2) / 3) + 1, '\0');
    int length = EVP_EncodeBlock(reinterpret_cast<unsigned char *>(&encoded[0]),
                    reinterpret_cast<const unsigned char *>(plain.data()), plain.size());
    encoded.resize(length);
    return "Basic " + encoded;
}
static void check_basic(const std::string &header, bool valid,
                        const std::string &user = "", const std::string &password = "")
{
    expected_user = user;
    expected_password = password;
    pam_calls = 0;
    Session session(true);
    session._request.set(Rest::HTTP_REQ_URI, "/rest/service/test");
    bool result = AuthBasic(true).authorized(header, session);
    assert(result == valid);
    assert(pam_calls == (valid ? 1 : 0));
    if (valid) assert(session._user == user && session._auth_type == Rest::AUTH_TYPE_BASIC);
}
static void write_session(const std::string &record)
{
    FILE *f = fopen("/var/run/gui/session", "w");
    assert(f);
    assert(fwrite(record.data(), 1, record.size(), f) == record.size());
    assert(fclose(f) == 0);
    assert(chmod("/var/run/gui/session", 0600) == 0);
}
static std::string read_file(const char *path)
{
    FILE *f = fopen(path, "r");
    assert(f);
    std::string result;
    char buf[2048];
    while (fgets(buf, sizeof(buf), f)) result += buf;
    fclose(f);
    return result;
}
static void basic_tests(const std::string &which)
{
    if (which == "max") {
        check_basic(basic("u:" + std::string(1022, 'p')), true, "u", std::string(1022, 'p'));
        return;
    }
    if (which == "nul") {
        check_basic(basic(std::string("u:p\0ignored", 11)), false);
        return;
    }
    if (which == "logs") {
        check_basic(basic("u:secret-password"), true, "u", "secret-password");
        assert(logs.find("secret-password") == std::string::npos);
        // The failure path must not log the password either.
        pam_accept = false;
        Session s(true);
        s._request.set(Rest::HTTP_REQ_URI, "/rest/service/test");
        assert(!AuthBasic(true).authorized(basic("u:secret-password"), s));
        assert(logs.find("secret-password") == std::string::npos);
        return;
    }
    check_basic("", false);
    check_basic("Basic", false);
    check_basic("Basic ", false);
    check_basic("Basic !!!!", false);
    check_basic("Basic dTpw$", false);
    check_basic("Basic dTpw=", false);
    check_basic("Basic dTpw\n", false);
    check_basic("BasicX dTpw", false);
    check_basic(basic("nocolon"), false);
    check_basic(basic(":password"), false);
    check_basic(basic("u:" + std::string(1023, 'p')), false);
    check_basic(basic("u:p"), true, "u", "p");
    check_basic(basic("u:"), true, "u", "");
    check_basic(basic("u:p:a:ss"), true, "u", "p:a:ss");
}
static void session_tests(const std::string &which)
{
    assert(geteuid() == 0);
    mkdir("/var/run/gui", 0700);
    chmod("/var/run/gui", 0700);
    AuthSession auth(true);
    const std::string token = "0123456789ABCDEF";
    const std::string record = token + ",root," + std::to_string(time(NULL)) + ",127.0.0.1,service-user\n";
    write_session(record);
    if (which == "session-valid") {
        Session s(true);
        assert(auth.authorized("Vyatta-Session " + token, s));
        assert(s._user == "root" && s._service_user && s._session_key == token);
        Session cookie(true);
        cookie._request.set(Rest::HTTP_REQ_PRAGMA, Rest::PRAGMA_NO_VYATTA_SESSION_UPDATE);
        const std::string before = read_file("/var/run/gui/session");
        assert(auth.authorized("other=x; vyatta2_path_loc=" + token + "; x=y", cookie));
        assert(read_file("/var/run/gui/session") == before);
        Session missing(true);
        assert(!auth.authorized("Vyatta-Session FFFFFFFFFFFFFFFF", missing));
        write_session(token + ",root,1,127.0.0.1,service-user\n");
        Session expired(true);
        assert(!auth.authorized("Vyatta-Session " + token, expired));
    } else if (which == "session-untrusted") {
        chmod("/var/run/gui", 0777);
        Session s(true);
        assert(!auth.authorized("Vyatta-Session " + token, s));
        // A failed authorization must not rewrite attacker-controlled state.
        assert(read_file("/var/run/gui/session") == record);
    } else if (which == "session-files") {
        chmod("/var/run/gui/session", 0666);
        Session s(true);
        assert(!auth.authorized("Vyatta-Session " + token, s));
    } else if (which == "session-symlink") {
        assert(rename("/var/run/gui/session", "/var/run/victim") == 0);
        assert(symlink("/var/run/victim", "/var/run/gui/session") == 0);
        Session s(true);
        assert(!auth.authorized("Vyatta-Session " + token, s));
        assert(read_file("/var/run/victim") == record);
    } else if (which == "session-lock") {
        assert(close(open("/var/run/gui/session.lck", O_CREAT | O_WRONLY, 0600)) == 0);
        Session s(true);
        assert(!auth.authorized("Vyatta-Session " + token, s));
        assert(access("/var/run/gui/session.lck", F_OK) == 0);
        assert(read_file("/var/run/gui/session") == record);
    } else if (which == "session-tmp-symlink") {
        FILE *f = fopen("/var/run/victim", "w");
        assert(f && fputs("do not truncate", f) >= 0 && fclose(f) == 0);
        assert(symlink("/var/run/victim", "/var/run/gui/session_tmp") == 0);
        Session s(true);
        assert(!auth.authorized("Vyatta-Session " + token, s));
        assert(read_file("/var/run/victim") == "do not truncate");
    } else if (which == "session-startup-unsafe") {
        chmod("/var/run/gui", 0777);
        AuthSession restarted(true);
        assert(read_file("/var/run/gui/session") == record);
        Session s(true);
        assert(!restarted.authorized("Vyatta-Session " + token, s));
    } else if (which == "session-uid") {
        pid_t child = fork();
        assert(child >= 0);
        if (!child) {
            assert(setgid(65534) == 0 && setuid(65534) == 0);
            assert(unlink("/var/run/gui/session") < 0);
            assert(rename("/var/run/gui/session", "/var/run/gui/stolen") < 0);
            assert(open("/var/run/gui/session_tmp", O_CREAT | O_WRONLY, 0600) < 0);
            _exit(0);
        }
        int status;
        assert(waitpid(child, &status, 0) == child && status == 0);
    } else abort();
}
int main(int argc, char **argv)
{
    assert(argc == 2);
    std::string which = argv[1];
    if (which.find("session-") == 0) session_tests(which);
    else basic_tests(which);
    printf("auth-boundary: %s OK\n", argv[1]);
}
