#include <Windows.h>
#include "platform.h"

void PlatformInit() {
	// TODO
}

void ShowError(const char *msg) {
  MessageBoxA(NULL, msg, "Knossos Updater",
              MB_OK | MB_ICONERROR | MB_TASKMODAL);
}


DialogResult SaveFileDialog(
    const char *title,
    const char *default_filepath,
    DialogCallback callback) {
  // TODO
  return {};
}

DialogResult OpenFileDialog(
    const char *title,
    const char *default_filepath,
    DialogCallback callback) {
  // TODO
  return {};
}

DialogResult OpenFolderDialog(
    const char *title, const char *folder,
    DialogCallback callback) {
  // TODO
  return {};
}
