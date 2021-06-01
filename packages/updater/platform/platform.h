#ifndef KNOSSOS_PLATFORM_FOLDER
#define KNOSSOS_PLATFORM_FOLDER

#include <stdint.h>

typedef void (*DialogCallback)(int, const char*);
typedef struct {
  uint8_t code;
  char *string;
} DialogResult;

void PlatformInit();
void ShowError(const char *message);
DialogResult SaveFileDialog(const char *title, const char *default_filepath);
DialogResult OpenFileDialog(const char *title, const char *default_filepath);
DialogResult OpenFolderDialog(const char *title, const char *folder);

#endif /* KNOSSOS_PLATFORM_FOLDER */
