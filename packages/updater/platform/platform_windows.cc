#include <cstdlib>
#include <string>
#define INITGUID
#include <Windows.h>
#include <shobjidl.h>

#include "platform.h"

extern "C" void PlatformInit() {
  CoInitializeEx(nullptr, COINIT_APARTMENTTHREADED | COINIT_DISABLE_OLE1DDE);
}

extern "C" void ShowError(const char *msg) {
  MessageBoxA(nullptr, msg, "Error", MB_OK | MB_ICONERROR);
}

extern "C" DialogResult SaveFileDialog(const char *title, const char *default_filepath) {
  return {};
}

extern "C" DialogResult OpenFileDialog(const char *title, const char *default_filepath) {
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

extern "C" DialogResult OpenFolderDialog(const char *title, const char *folder) {

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
