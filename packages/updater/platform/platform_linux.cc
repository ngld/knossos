#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <X11/Xlib.h>

#pragma GCC diagnostic push
#pragma GCC diagnostic ignored "-Wdeprecated-declarations"
#include <gtk/gtk.h>
#pragma GCC diagnostic pop

#include "platform.h"

void PlatformInit() {
  XInitThreads();
  gtk_init(nullptr, nullptr);
}

static void flush_gtk_events() {
  while (gtk_events_pending())
    gtk_main_iteration();
}

void ShowError(const char *msg) {
  auto dialog =
      gtk_message_dialog_new(NULL, GTK_DIALOG_MODAL, GTK_MESSAGE_ERROR,
                             GTK_BUTTONS_CLOSE, "%s", msg);
  gtk_dialog_run(GTK_DIALOG(dialog));
  gtk_widget_destroy(dialog);
  flush_gtk_events();
}

static DialogResult
InternalOpenFileDialog(const char *title, GtkFileChooserAction action,
                       const char *default_filepath) {
  const char *open_label;
  switch (action) {
  case GTK_FILE_CHOOSER_ACTION_OPEN:
    open_label = "_Open";
    break;
  case GTK_FILE_CHOOSER_ACTION_SAVE:
    open_label = "_Save";
    break;
  case GTK_FILE_CHOOSER_ACTION_SELECT_FOLDER:
    open_label = "_Select Folder";
    break;
  default:
    open_label = "_Select";
  }

  auto dialog =
      gtk_file_chooser_native_new(title, NULL, action, open_label, "_Cancel");
  auto chooser = GTK_FILE_CHOOSER(dialog);

  if (action == GTK_FILE_CHOOSER_ACTION_SAVE) {
    gtk_file_chooser_set_do_overwrite_confirmation(chooser, true);
  }

  if (strlen(default_filepath) > 0) {
    gtk_file_chooser_set_filename(chooser, default_filepath);
  }

  DialogResult result;
  auto gtk_result = gtk_native_dialog_run(GTK_NATIVE_DIALOG(dialog));
  if (gtk_result == GTK_RESPONSE_ACCEPT) {
    result.string = gtk_file_chooser_get_filename(chooser);
    result.code = 0;
  } else {
    result.code = 1;
  }

  g_object_unref(dialog);
  flush_gtk_events();
  return result;
}

DialogResult SaveFileDialog(
    const char *title,
    const char *default_filepath) {
  return InternalOpenFileDialog(title, GTK_FILE_CHOOSER_ACTION_SAVE,
                         default_filepath);
}

DialogResult OpenFileDialog(
    const char *title,
    const char *default_filepath) {
  return InternalOpenFileDialog(title, GTK_FILE_CHOOSER_ACTION_OPEN,
                         default_filepath);
}

DialogResult OpenFolderDialog(
    const char *title, const char *folder) {
  return InternalOpenFileDialog(title, GTK_FILE_CHOOSER_ACTION_SELECT_FOLDER,
                         folder);
}

extern char* GetDesktopDirectory() {
  // TODO
  return nullptr;
}

extern char* GetStartMenuDirectory() {
  // TODO
  return nullptr;
}

extern bool IsElevated() {
  return false;
}

extern const char* CreateShortcut(const char* shortcut, const char* target) {
  const char* error = "unsupported platform";
  int errlen = sizeof(char) * (strlen(error) + 1);
  char* result = (char*)malloc(errlen);
  memcpy(result, error, errlen);
  return result;
}

extern "C" char *RunElevated(const char *program, const char *args) {
  const char* error = "unsupported platform";
  int errlen = sizeof(char) * (strlen(error) + 1);
  char* result = (char*)malloc(errlen);
  memcpy(result, error, errlen);
  return result;
}
