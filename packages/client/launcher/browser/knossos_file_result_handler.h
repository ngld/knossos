#ifndef PACKAGES_CLIENT_LAUNCHER_BROWSER_KNOSSOS_FILE_RESULT_HANDLER
#define PACKAGES_CLIENT_LAUNCHER_BROWSER_KNOSSOS_FILE_RESULT_HANDLER

#include "include/cef_base.h"
#include "include/cef_browser.h"

class KnossosFileResultHandler : public CefRunFileDialogCallback {
public:
  KnossosFileResultHandler(CefRefPtr<CefFrame> request_frame, int promise_id, bool multi);

  virtual void
  OnFileDialogDismissed(const std::vector<CefString> &file_paths) override;

private:
  CefRefPtr<CefFrame> request_frame_;
  int promise_id_;
  bool multi_;

  IMPLEMENT_REFCOUNTING(KnossosFileResultHandler);
};

#endif /* PACKAGES_CLIENT_LAUNCHER_BROWSER_KNOSSOS_FILE_RESULT_HANDLER */
