// SPDX-License-Identifier: GPL-2.0-only
#include "authsession.hh"

#include <cassert>
#include <fcntl.h>
#include <sys/file.h>
#include <sys/stat.h>
#include <unistd.h>

int main(int argc, char **argv)
{
	assert(argc == 3);
	const std::string file = argv[1];
	const std::string mode = argv[2];
	if (mode == "locked") {
		const std::string lock_file = file + ".lck";
		const int fd = open(lock_file.c_str(), O_RDWR | O_CREAT, 0600);
		assert(fd >= 0 && flock(fd, LOCK_EX | LOCK_NB) == 0);
		{
			Lock store(file);
			assert(!store.status());
		}
		struct stat held;
		struct stat visible;
		assert(fstat(fd, &held) == 0 && stat(lock_file.c_str(), &visible) == 0);
		assert(held.st_dev == visible.st_dev && held.st_ino == visible.st_ino);
		close(fd);
	}

	Lock store(file);
	assert(store.status() == (mode != "refuse"));
	if (store.status())
		assert(fgetc(store.get_fp()) == EOF && !ferror(store.get_fp()));
	if (mode == "publish") {
		assert(fputs("test data\n", store.get_fp_tmp()) >= 0);
		assert(store.commit());
	}
}
