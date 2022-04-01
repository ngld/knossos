#define _LARGEFILE64_SOURCE
#define __STDC_WANT_LIB_EXT1__ 1

#include <stdlib.h>
#include <stdint.h>
#include <stdio.h>
#include <fcntl.h>
#include <string.h>

#ifdef _WIN32
#define O_CLOEXEC 0
#endif

int libarchive_get_fd(const char *filename, char **errormsg) {
  int fd = open(filename, O_RDONLY | O_BINARY | O_CLOEXEC);
  if (fd < 0) {
    *errormsg = malloc(256);
    strerror_s(*errormsg, 256, errno);
  }

  return fd;
}

int64_t libarchive_tell(int fd) {
  return lseek64(fd, 0, SEEK_CUR);
}

void libarchive_close_fd(int fd) {
  close(fd);
}

