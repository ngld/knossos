#ifdef WIN32
#include <windows.h>

#define LOAD_SYM GetProcAddress
#else
#include <dlfcn.h>

#define LOAD_SYM dlsym
#endif
#include "dynknossos.h"

GODYN_KnossosFreeKnossosResponse KnossosFreeKnossosResponse = 0;
GODYN_KnossosInit KnossosInit = 0;
GODYN_KnossosHandleRequest KnossosHandleRequest = 0;


bool LoadKnossos(const char* knossos_path, char** error) {
#ifdef WIN32
  HMODULE lib = LoadLibraryA(knossos_path);
  if (!lib) {
    auto code = GetLastError();
    LPSTR message;
    FormatMessageA(FORMAT_MESSAGE_ALLOCATE_BUFFER | FORMAT_MESSAGE_FROM_SYSTEM | FORMAT_MESSAGE_IGNORE_INSERTS,
       nullptr, code, 0, (LPSTR)&message, 0, nullptr);
    *error = (char*) message;
    return false;
  }

#else
  void* lib = dlopen(knossos_path, RTLD_NOW | RTLD_NODELETE);
  if (!lib) {
    *error = dlerror();
    return false;
  }
#endif

  KnossosFreeKnossosResponse = (GODYN_KnossosFreeKnossosResponse) LOAD_SYM(lib, "KnossosFreeKnossosResponse");
  KnossosInit = (GODYN_KnossosInit) LOAD_SYM(lib, "KnossosInit");
  KnossosHandleRequest = (GODYN_KnossosHandleRequest) LOAD_SYM(lib, "KnossosHandleRequest");

#ifndef WIN32
  dlclose(lib);
#endif

  if (!KnossosFreeKnossosResponse || !KnossosInit || !KnossosHandleRequest) {
    *error = (char*) "One or more functions could not be found!";
    return false;
  }
  return true;
}
