#include <fcntl.h>
#include <io.h>
#include <stdint.h>

#include <archive.h>

#ifdef _WIN32
#define close _close
#endif

int libarchive_get_fd(intptr_t handle) {
#ifdef _WIN32
  return _open_osfhandle(handle, O_RDONLY | O_BINARY);
#else
  return (int) handle;
#endif
}

void libarchive_close_fd(int fd) {
  close(fd);
}

