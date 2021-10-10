#include <cstdlib>
#include <string>

#define INITGUID
#include <windows.h>

#include <knownfolders.h>
#include <shellapi.h>
#include <shlobj.h>
#include <shobjidl.h>

#include "platform.h"

extern "C" void PlatformInit() {
  CoInitializeEx(nullptr, COINIT_APARTMENTTHREADED | COINIT_DISABLE_OLE1DDE);
}

extern "C" void ShowError(const char *msg) {
  MessageBoxA(nullptr, msg, "Error", MB_OK | MB_ICONERROR);
}

extern "C" DialogResult SaveFileDialog(const char *title,
                                       const char *default_filepath) {
  return {};
}

extern "C" DialogResult OpenFileDialog(const char *title,
                                       const char *default_filepath) {
  return {};
}

static DialogResult error(const char *msg) {
  DialogResult result;
  result.code = 1;

  auto strsize = strlen(msg) * sizeof(char);
  result.string = (char *)malloc(strsize);
  strcpy_s(result.string, strsize, msg);

  return result;
}

extern "C" DialogResult OpenFolderDialog(const char *title,
                                         const char *folder) {
  IFileOpenDialog *dialog;
  auto hr =
      CoCreateInstance(CLSID_FileOpenDialog, nullptr, CLSCTX_ALL,
                       IID_IFileOpenDialog, reinterpret_cast<void **>(&dialog));

  if (!SUCCEEDED(hr)) {
    return error("failed to create dialog");
  }
  FILEOPENDIALOGOPTIONS options;
  hr = dialog->GetOptions(&options);
  if (!SUCCEEDED(hr)) {
    return error("failed to read dialog options");
  }

  hr = dialog->SetOptions(options | FOS_PICKFOLDERS | FOS_FORCEFILESYSTEM |
                          FOS_NOREADONLYRETURN);
  if (!SUCCEEDED(hr)) {
    return error("failed to set dialog options");
  }

  LPWSTR wtitle;
  size_t title_len;
  if (mbstowcs_s(&title_len, nullptr, 0, title, 0)) {
    return error("failed to parse title");
  }

  wtitle = (LPWSTR)malloc(title_len * sizeof(WCHAR));
  if (mbstowcs_s(nullptr, wtitle, title_len, title, title_len)) {
    free(wtitle);
    return error("failed to copy title");
  }

  hr = dialog->SetTitle(wtitle);
  if (!SUCCEEDED(hr)) {
    free(wtitle);
    return error("failed to set title");
  }

  LPWSTR wfolder;
  size_t folder_len;
  if (mbstowcs_s(&folder_len, nullptr, 0, folder, 0)) {
    free(wtitle);
    return error("failed to parse folder path");
  }

  wfolder = (LPWSTR)malloc(folder_len * sizeof(WCHAR));
  if (mbstowcs_s(nullptr, wfolder, folder_len, folder, folder_len)) {
    free(wtitle);
    free(wfolder);
    return error("failed to copy folder path");
  }

  IShellItem *default_folder;
  hr = SHCreateItemFromParsingName(wfolder, nullptr,
                                   IID_PPV_ARGS(&default_folder));
  if (!SUCCEEDED(hr)) {
    free(wtitle);
    free(wfolder);
    return error("failed to fetch IShellItem for folder path");
  }

  hr = dialog->SetDefaultFolder(default_folder);
  if (!SUCCEEDED(hr)) {
    free(wtitle);
    free(wfolder);
    return error("failed to set default folder");
  }

  hr = dialog->Show(nullptr);

  free(wtitle);
  free(wfolder);

  if (!SUCCEEDED(hr)) {
    return error("failed to open dialog");
  }

  IShellItem *item;
  hr = dialog->GetResult(&item);
  if (!SUCCEEDED(hr)) {
    return error("failed to fetch result");
  }
  PWSTR path;
  hr = item->GetDisplayName(SIGDN_FILESYSPATH, &path);
  if (!SUCCEEDED(hr)) {
    return error("failed to read item path");
  }

  DialogResult result;
  size_t path_len;
  if (wcstombs_s(&path_len, nullptr, 0, path, 0)) {
    return error("failed to parse item path");
  }

  result.string = (char *)malloc(path_len * sizeof(char));
  if (wcstombs_s(nullptr, result.string, path_len, path, path_len)) {
    return error("failed to copy item path");
  }

  return result;
}

static char *getErrorMessage() {
  auto error = GetLastError();
  char *message;
  if (!FormatMessageA(FORMAT_MESSAGE_ALLOCATE_BUFFER |
                          FORMAT_MESSAGE_FROM_SYSTEM |
                          FORMAT_MESSAGE_IGNORE_INSERTS,
                      nullptr, error, 0, (LPSTR)&message, 0, nullptr)) {
    auto fallback_message = "Failed to retrieve error from windows";
    message = new char[strlen(fallback_message) + 1];
    memcpy(message, fallback_message,
           (strlen(fallback_message) + 1) * sizeof(char));
  }

  return message;
}

extern "C" const char *CreateShortcut(const char *shortcut,
                                      const char *target) {

  wchar_t *wshortcut;
  size_t wshortcut_len;
  auto error = mbstowcs_s(&wshortcut_len, nullptr, 0, shortcut, 0);
  if (error < 0) {
    return "failed to convert shortcut path";
  }

  wshortcut = new wchar_t[wshortcut_len];
  error = mbstowcs_s(&wshortcut_len, wshortcut, wshortcut_len, shortcut, wshortcut_len);
  if (error < 0) {
    return "failed to convert shortcut path";
  }

  IShellLinkA *link;
  auto hr = CoCreateInstance(CLSID_ShellLink, nullptr, CLSCTX_INPROC_SERVER,
                             IID_IShellLinkA, reinterpret_cast<void **>(&link));
  if (!SUCCEEDED(hr)) {
    return "failed to allocate IShellLinkA";
  }

  hr = link->SetPath(target);
  if (!SUCCEEDED(hr)) {
    link->Release();
    return "failed to set path";
  }

  IPersistFile *file;
  hr = link->QueryInterface(IID_IPersistFile, reinterpret_cast<void **>(&file));
  if (!SUCCEEDED(hr)) {
    link->Release();
    return "failed to retrieve IPersistFile";
  }

  hr = file->Save(wshortcut, true);
  file->Release();
  link->Release();
  delete[] wshortcut;

  if (!SUCCEEDED(hr)) {
    return "failed to save shortcut";
  }

  return nullptr;
}

static char *getFolderHelper(KNOWNFOLDERID folder_id) {
  wchar_t *path;
  auto hr = SHGetKnownFolderPath(folder_id, 0, nullptr, &path);
  if (!SUCCEEDED(hr)) {
    return nullptr;
  }

  size_t path_len;
  auto error = wcstombs_s(&path_len, nullptr, 0, path, 0);
  if (error < 0) {
    CoTaskMemFree(path);
    return nullptr;
  }

  char *c_path = new char[path_len];
  error = wcstombs_s(&path_len, c_path, path_len * sizeof(char), path,
                     path_len * sizeof(char));

  CoTaskMemFree(path);
  if (error < 0) {
    delete[] c_path;
    return nullptr;
  }

  return c_path;
}

extern "C" char *GetDesktopDirectory() {
  return getFolderHelper(FOLDERID_PublicDesktop);
}

extern "C" char *GetStartMenuDirectory() {
  return getFolderHelper(FOLDERID_CommonStartMenu);
}

extern "C" bool IsElevated() {
  PSID adminGroup;
  SID_IDENTIFIER_AUTHORITY ntAuth = SECURITY_NT_AUTHORITY;
  if (!AllocateAndInitializeSid(&ntAuth, 2, SECURITY_BUILTIN_DOMAIN_RID,
                                DOMAIN_ALIAS_RID_ADMINS, 0, 0, 0, 0, 0, 0,
                                &adminGroup)) {
    return false;
  }

  int result;
  if (!CheckTokenMembership(nullptr, adminGroup, &result)) {
    return false;
  }

  return !!result;
}

extern "C" char *RunElevated(const char *program, const char *args) {
  SHELLEXECUTEINFOA info;
  info.cbSize = sizeof(info);
  info.fMask = SEE_MASK_DEFAULT;
  info.hwnd = nullptr;
  info.lpVerb = "runas";
  info.lpFile = program;
  info.lpParameters = args;
  info.lpDirectory = nullptr;
  info.nShow = SW_NORMAL;

  if (ShellExecuteExA(&info)) {
    return nullptr;
  }

  return getErrorMessage();
}
