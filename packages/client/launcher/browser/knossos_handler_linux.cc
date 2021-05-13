#include "browser/knossos_handler.h"

#if defined(CEF_X11)
#include <X11/Xatom.h>
#include <X11/Xlib.h>

#pragma GCC diagnostic push
#pragma GCC diagnostic ignored "-Wdeprecated-declarations"
#include <gtk/gtk.h>
#pragma GCC diagnostic pop
#endif

#include <string>

#include "include/base/cef_logging.h"
#include "include/cef_browser.h"
#include "include/wrapper/cef_helpers.h"

void KnossosHandler::PlatformInit() { gtk_init(nullptr, nullptr); }

void KnossosHandler::PlatformTitleChange(CefRefPtr<CefBrowser> browser,
                                         const CefString &title) {
  std::string titleStr(title);

#if defined(CEF_X11)
  // Retrieve the X11 display shared with Chromium.
  ::Display *display = cef_get_xdisplay();
  DCHECK(display);

  // Retrieve the X11 window handle for the browser.
  ::Window window = browser->GetHost()->GetWindowHandle();
  if (window == kNullWindowHandle)
    return;

  // Retrieve the atoms required by the below XChangeProperty call.
  const char *kAtoms[] = {"_NET_WM_NAME", "UTF8_STRING"};
  Atom atoms[2];
  int result =
      XInternAtoms(display, const_cast<char **>(kAtoms), 2, false, atoms);
  if (!result)
    NOTREACHED();

  // Set the window title.
  XChangeProperty(display, window, atoms[0], atoms[1], 8, PropModeReplace,
                  reinterpret_cast<const unsigned char *>(titleStr.c_str()),
                  titleStr.size());

  // TODO(erg): This is technically wrong. So XStoreName and friends expect
  // this in Host Portable Character Encoding instead of UTF-8, which I believe
  // is Compound Text. This shouldn't matter 90% of the time since this is the
  // fallback to the UTF8 property above.
  XStoreName(display, browser->GetHost()->GetWindowHandle(), titleStr.c_str());
#endif // defined(CEF_X11)
}

CefRect KnossosHandler::GetScreenSize() {
  CEF_REQUIRE_UI_THREAD();
  CefRect screen_size;

#if defined(CEF_X11)
  XWindowAttributes attrs;
  XGetWindowAttributes(cef_get_xdisplay(),
                       XDefaultRootWindow(cef_get_xdisplay()), &attrs);

  screen_size.width = attrs.width;
  screen_size.height = attrs.height;
#endif

  return screen_size;
}

void KnossosHandler::ShowError(std::string msg) {
  auto dialog =
      gtk_message_dialog_new(NULL, GTK_DIALOG_MODAL, GTK_MESSAGE_ERROR,
                             GTK_BUTTONS_CLOSE, "%s", msg.c_str());
  gtk_dialog_run(GTK_DIALOG(dialog));
  gtk_widget_destroy(dialog);
}

static void
InternalOpenFileDialog(const char *title, GtkFileChooserAction action,
                       const char *default_filepath,
                       std::vector<std::string> accepted,
                       CefRefPtr<CefRunFileDialogCallback> callback) {
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

  std::vector<GtkFileFilter*> filters;
  for (auto item : accepted) {
    auto splitter = item.find("|");
    if (splitter == std::string::npos) {
      LOG(WARNING) << "Invalid file filter: " << item;
      continue;
    }

    auto filter = gtk_file_filter_new();
    gtk_file_filter_set_name(filter, item.substr(0, splitter).c_str());

    if (item.find("/") == std::string::npos) {
      gtk_file_filter_add_pattern(filter, item.substr(splitter + 1).c_str());
    } else {
      gtk_file_filter_add_mime_type(filter, item.substr(splitter + 1).c_str());
    }

    gtk_file_chooser_add_filter(chooser, filter);
    filters.push_back(filter);
  }

  auto result = gtk_native_dialog_run(GTK_NATIVE_DIALOG(dialog));
  if (result == GTK_RESPONSE_ACCEPT) {
    auto filename = gtk_file_chooser_get_filename(chooser);
    callback->OnFileDialogDismissed(0, std::vector<CefString>({filename}));
    g_free(filename);
  } else {
    callback->OnFileDialogDismissed(0, std::vector<CefString>({""}));
  }

  for (auto filter : filters) {
    g_object_unref(filter);
  }

  g_object_unref(dialog);
}

void KnossosHandler::SaveFileDialog(
    CefRefPtr<CefBrowser> browser, std::string title,
    std::string default_filepath, std::vector<std::string> accepted,
    CefRefPtr<CefRunFileDialogCallback> callback) {
  InternalOpenFileDialog(title.c_str(), GTK_FILE_CHOOSER_ACTION_SAVE,
                         default_filepath.c_str(), accepted, callback);
}

void KnossosHandler::OpenFileDialog(
    CefRefPtr<CefBrowser> browser, std::string title,
    std::string default_filepath, std::vector<std::string> accepted,
    CefRefPtr<CefRunFileDialogCallback> callback) {
  InternalOpenFileDialog(title.c_str(), GTK_FILE_CHOOSER_ACTION_OPEN,
                         default_filepath.c_str(), accepted, callback);
}

void KnossosHandler::OpenFolderDialog(
    CefRefPtr<CefBrowser> browser, std::string title, std::string folder,
    CefRefPtr<CefRunFileDialogCallback> callback) {
  InternalOpenFileDialog(title.c_str(), GTK_FILE_CHOOSER_ACTION_SELECT_FOLDER,
                         folder.c_str(), {}, callback);
}
