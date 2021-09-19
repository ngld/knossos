#include <stdbool.h>
#include <stdint.h>
#include "src/lib.h"

#ifdef WIN32
#include <windows.h>
#define LOAD_SYM GetProcAddress
#else
#include <dlfcn.h>
#define LOAD_SYM dlsym
#endif

typedef bool (*extract_inno_ptr)(const char *path, const char *destination, inno_progress_callback callback, inno_log_callback log);
extract_inno_ptr extract_inno_ref = 0;

extern void libinnoextract_progress_cb(const char *message, float progress);
extern void libinnoextract_log_cb(uint8_t level, const char *message);

bool extract_inno_wrapper(const char *path, const char *destination) {
  return extract_inno_ref(path, destination, &libinnoextract_progress_cb,
                      &libinnoextract_log_cb);
}

bool load_libinnoextract(const char *lib_path, char **error) {
  if (extract_inno_ref) {
    return true;
  }

#ifdef WIN32
  HMODULE lib = LoadLibraryA(lib_path);
  if (!lib) {
    DWORD code = GetLastError();
    LPSTR message;
    FormatMessageA(FORMAT_MESSAGE_ALLOCATE_BUFFER | FORMAT_MESSAGE_FROM_SYSTEM |
                       FORMAT_MESSAGE_IGNORE_INSERTS,
                   NULL, code, 0, (LPSTR)&message, 0, NULL);
    *error = (char *)message;
    return false;
  }
#else
  void *lib = dlopen(lib_path, RTLD_NOW | RTLD_NODELETE);
  if (!lib) {
    *error = dlerror();
    return false;
  }
#endif

  extract_inno_ref = (extract_inno_ptr)LOAD_SYM(lib, "extract_inno");

#ifndef WIN32
  dlclose(lib);
#endif
  if (!extract_inno_ref) {
    *error = (char *)"One or more functions could not be found!";
    return false;
  }

  return true;
}
