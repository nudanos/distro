// SPDX-License-Identifier: GPL-2.0-only
#include "authbasic.hh"

#include <cassert>
#include <openssl/evp.h>
#include <string>

static std::string encode(const std::string &text)
{
	std::string result(4 * ((text.size() + 2) / 3) + 1, '\0');
	const int size = EVP_EncodeBlock(
		reinterpret_cast<unsigned char *>(&result[0]),
		reinterpret_cast<const unsigned char *>(text.data()), text.size());
	result.resize(size);
	return "Basic " + result;
}

int main()
{
	AuthBasic auth(false);
	assert(auth.handle("Basic dGVzdDph"));
	assert(!auth.handle("Basics dGVzdDph"));
	assert(!auth.handle("Basic"));
	std::string name;
	std::string password;
	assert(AuthBasic::parse_credentials(encode("test:a:b"), name, password));
	assert(name == "test" && password == "a:b");
	assert(AuthBasic::parse_credentials(encode("test:"), name, password));
	assert(name == "test" && password.empty());
	assert(!AuthBasic::parse_credentials("Basic ", name, password));
	assert(!AuthBasic::parse_credentials("Basic !!!!", name, password));
	assert(!AuthBasic::parse_credentials("Basic dGVz=Dp6", name, password));
	assert(!AuthBasic::parse_credentials("Basic dGVzdDph=", name, password));
	assert(!AuthBasic::parse_credentials(encode("test"), name, password));
	assert(!AuthBasic::parse_credentials(encode(":a"), name, password));
	assert(!AuthBasic::parse_credentials(encode(std::string("test:a\0b", 8)), name, password));

	const std::string max_password(1019, 'a');
	assert(AuthBasic::parse_credentials(encode("test:" + max_password), name, password));
	assert(password == max_password);
	assert(!AuthBasic::parse_credentials(encode("test:" + max_password + "b"), name, password));

	HTTP request(false);
	request.set(Rest::HTTP_REQ_AUTHORIZATION, "auth-test-value");
	request.set(Rest::HTTP_REQ_COOKIE, "cookie-test-value");
	const std::string dump = request.dump();
	assert(dump.find("auth-test-value") == std::string::npos);
	assert(dump.find("cookie-test-value") == std::string::npos);
}
