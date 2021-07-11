#ifndef KNOSSOS_PLATFORM_FOLDER
#define KNOSSOS_PLATFORM_FOLDER

#include <stdint.h>

#ifdef WIN32
#define EXPORT __declspec(dllexport)
#else
#define EXPORT extern
#endif

typedef void (*DialogCallback)(int, const char*);
typedef struct {
  uint8_t code;
  char *string;
} DialogResult;

EXPORT void PlatformInit();
EXPORT void ShowError(const char *message);
EXPORT DialogResult SaveFileDialog(const char *title, const char *default_filepath);
EXPORT DialogResult OpenFileDialog(const char *title, const char *default_filepath);
EXPORT DialogResult OpenFolderDialog(const char *title, const char *folder);

#endif /* KNOSSOS_PLATFORM_FOLDER */
