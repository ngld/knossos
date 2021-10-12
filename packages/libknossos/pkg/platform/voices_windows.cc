#include <windows.h>
#include <sapi.h>
#include <sphelper.h>

struct voice_list {
  ULONG count;
  char** names;
};

static void free_voice_list(struct voice_list &list) {
  for (ULONG idx = 0; idx < list.count; idx++) {
    if (list.names[idx] == nullptr) {
      break;
    }

    free(list.names[idx]);
  }

  free(list.names);
  list.count = 0;
}

extern "C" struct voice_list get_voices() {
  struct voice_list result;
  IEnumSpObjectTokens *voicesEnum;
  auto hr = SpEnumTokens(SPCAT_VOICES, NULL, NULL, &voicesEnum);

  if (FAILED(hr))
    return result;

  hr = voicesEnum->GetCount(&result.count);

  if (FAILED(hr))
    return result;

  result.names = new char*[result.count];

  for (ULONG idx = 0; idx < result.count; idx++) {
    ISpObjectToken *voiceToken;
    hr = voicesEnum->Next(1, &voiceToken, NULL);

    if (FAILED(hr)) {
      free_voice_list(result);
      return result;
    }

    LPWSTR name;
    hr = voiceToken->GetStringValue(NULL, &name);
    voiceToken->Release();

    if (FAILED(hr)) {
      free_voice_list(result);
      return result;
    }

    size_t buffer_size;
    auto error = wcstombs_s(&buffer_size, nullptr, 0, name, 0);
    if (error < 0) {
      free_voice_list(result);
      CoTaskMemFree(name);
      return result;
    }

    result.names[idx] = new char[buffer_size];
    error = wcstombs_s(&buffer_size, result.names[idx], buffer_size, name, buffer_size);

    CoTaskMemFree(name);

    if (error < 0) {
      free_voice_list(result);
      return result;
    }
  }

  voicesEnum->Release();

  return result;
}
