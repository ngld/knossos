#define _LARGEFILE64_SOURCE
#define __STDC_WANT_LIB_EXT1__ 1

#include <stdlib.h>
#include <stdint.h>
#include <stdio.h>
#include <fcntl.h>
#include <unistd.h>
#include <string.h>
#include <errno.h>

#ifdef _WIN32
#define O_CLOEXEC 0
#else
#define O_BINARY 0
#endif

#ifdef __APPLE__
#define lseek64 lseek
#endif

int libarchive_get_fd(const char *filename, char **errormsg) {
  int fd = open(filename, O_RDONLY | O_BINARY | O_CLOEXEC);
  if (fd < 0) {
    *errormsg = malloc(256);
#ifdef _WIN32
    strerror_s(*errormsg, 256, errno);
#else
    strerror_r(errno, *errormsg, 256);
#endif
  }

  return fd;
}

int64_t libarchive_tell(int fd) {
  return lseek64(fd, 0, SEEK_CUR);
}

void libarchive_close_fd(int fd) {
  close(fd);
}
