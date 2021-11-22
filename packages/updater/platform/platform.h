#ifndef KNOSSOS_PLATFORM_FOLDER
#define KNOSSOS_PLATFORM_FOLDER

#include <stdint.h>
#include <stdbool.h>

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
extern const char* CreateShortcut(const char* shortcut, const char* target);
extern char* GetDesktopDirectory();
extern char* GetStartMenuDirectory();
extern bool IsElevated();
extern char* RunElevated(const char* program, const char* args);

#ifdef __cplusplus
}
#endif

#endif /* KNOSSOS_PLATFORM_FOLDER */
