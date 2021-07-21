#ifndef KNOSSOS_PLATFORM_FOLDER
#define KNOSSOS_PLATFORM_FOLDER

#include <stdint.h>

#ifdef __cplusplus
#define EXPORT extern "C"
#else
#define EXPORT extern
#endif

typedef void (*DialogCallback)(int, const char*);
typedef struct {
  uint8_t code;
  char *string;
} DialogResult;

#ifdef __cplusplus
extern "C" {
#endif

extern void PlatformInit();
extern void ShowError(const char *message);
extern DialogResult SaveFileDialog(const char *title, const char *default_filepath);
extern DialogResult OpenFileDialog(const char *title, const char *default_filepath);
extern DialogResult OpenFolderDialog(const char *title, const char *folder);

#ifdef __cplusplus
}
#endif

#endif /* KNOSSOS_PLATFORM_FOLDER */
