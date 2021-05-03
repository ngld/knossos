#include "knossos_handler.h"

#include <string>
#include <windows.h>

#include "include/cef_browser.h"

void KnossosHandler::PlatformInit() {}

void KnossosHandler::PlatformTitleChange(CefRefPtr<CefBrowser> browser,
                                         const CefString &title) {
  CefWindowHandle hwnd = browser->GetHost()->GetWindowHandle();
  if (hwnd)
    SetWindowText(hwnd, std::wstring(title).c_str());
}

CefRect KnossosHandler::GetScreenSize() {
  int width = GetSystemMetrics(SM_CXSCREEN);
  int height = GetSystemMetrics(SM_CYSCREEN);
  CefRect screen_size(0, 0, width, height);
  return screen_size;
}

void KnossosHandler::ShowError(std::string message) {
  MessageBoxA(NULL, message.c_str(), "Knossos",
              MB_OK | MB_ICONERROR | MB_TASKMODAL);
}

static void
InternalFileDialogHelper(CefBrowserHost::FileDialogMode mode,
                         CefRefPtr<CefBrowser> browser, std::string title,
                         std::string default_filepath,
                         std::vector<std::string> accepted,
                         CefRefPtr<CefRunFileDialogCallback> callback) {
  std::vector<CefString> accept_filters;
  for (auto item : accepted) {
    accept_filters.push_back(item);
  }

  browser->GetHost()->RunFileDialog(mode, title, default_filepath,
                                    accept_filters, 0, callback);
}

void KnossosHandler::SaveFileDialog(
    CefRefPtr<CefBrowser> browser, std::string title,
    std::string default_filepath, std::vector<std::string> accepted,
    CefRefPtr<CefRunFileDialogCallback> callback) {
  InternalFileDialogHelper(FILE_DIALOG_SAVE, browser, title, default_filepath,
                           accepted, callback);
}

void KnossosHandler::OpenFileDialog(
    CefRefPtr<CefBrowser> browser, std::string title,
    std::string default_filepath, std::vector<std::string> accepted,
    CefRefPtr<CefRunFileDialogCallback> callback) {
  InternalFileDialogHelper(FILE_DIALOG_OPEN, browser, title, default_filepath,
                           accepted, callback);
}

void KnossosHandler::OpenFolderDialog(
    CefRefPtr<CefBrowser> browser, std::string title, std::string folder,
    CefRefPtr<CefRunFileDialogCallback> callback) {
  InternalFileDialogHelper(FILE_DIALOG_OPEN_FOLDER, browser, title, folder,
                           std::vector<std::string>(), callback);
}
