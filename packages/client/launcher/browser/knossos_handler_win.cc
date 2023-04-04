#include "knossos_handler.h"

#include <string>
#include <windows.h>
#include <shobjidl.h>

#include "include/cef_browser.h"

void KnossosHandler::PlatformInit() {
  CoInitializeEx(nullptr, COINIT_APARTMENTTHREADED | COINIT_DISABLE_OLE1DDE);
}

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

static LPWSTR utf8tomb(std::string input) {
  size_t result_size;
  if (mbstowcs_s(&result_size, nullptr, 0, input.c_str(), 0)) {
    return nullptr;
  }

  LPWSTR result = (LPWSTR)std::malloc(result_size);
  if (mbstowcs_s(&result_size, result, result_size, input.c_str(), 0)) {
    std::free(result);
    return nullptr;
  }

  return result;
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
                                    accept_filters, callback);
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
  IFileOpenDialog *dialog;
  auto hr =
      CoCreateInstance(CLSID_FileOpenDialog, nullptr, CLSCTX_ALL,
                       IID_IFileOpenDialog, reinterpret_cast<void **>(&dialog));

  if (!SUCCEEDED(hr)) {
    callback->OnFileDialogDismissed({});
    return;
  }
  FILEOPENDIALOGOPTIONS options;
  hr = dialog->GetOptions(&options);
  if (!SUCCEEDED(hr)) {
    callback->OnFileDialogDismissed({});
    return;
  }

  hr = dialog->SetOptions(options | FOS_PICKFOLDERS | FOS_FORCEFILESYSTEM |
                          FOS_NOREADONLYRETURN);
  if (!SUCCEEDED(hr)) {
    callback->OnFileDialogDismissed({});
    return;
  }

  auto mb_title = utf8tomb(title);
  if (!mb_title) {
    callback->OnFileDialogDismissed({});
    return;
  }

  auto mb_folder = utf8tomb(folder);
  if (!mb_folder) {
    std::free(mb_title);
    callback->OnFileDialogDismissed({});
    return;
  }

  dialog->SetTitle(mb_title);

  IShellItem *default_folder;
  hr = SHCreateItemFromParsingName(mb_folder, nullptr,
                                   IID_PPV_ARGS(&default_folder));
  if (!SUCCEEDED(hr)) {
    std::free(mb_title);
    std::free(mb_folder);
    callback->OnFileDialogDismissed({});
    return;
  }

  hr = dialog->SetDefaultFolder(default_folder);
  if (!SUCCEEDED(hr)) {
    std::free(mb_title);
    std::free(mb_folder);
    callback->OnFileDialogDismissed({});
    return;
  }

  hr = dialog->Show(nullptr);

  std::free(mb_title);
  std::free(mb_folder);

  if (!SUCCEEDED(hr)) {
    return callback->OnFileDialogDismissed({});
    return;
  }

  IShellItem *item;
  hr = dialog->GetResult(&item);
  if (!SUCCEEDED(hr)) {
    callback->OnFileDialogDismissed({});
    return;
  }

  PWSTR path;
  hr = item->GetDisplayName(SIGDN_FILESYSPATH, &path);
  if (!SUCCEEDED(hr)) {
    callback->OnFileDialogDismissed({});
    return;
  }

  callback->OnFileDialogDismissed({CefString(path)});
  std::free(path);
}
