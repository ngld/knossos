// Copyright (c) 2013 The Chromium Embedded Framework Authors. All rights
// reserved. Use of this source code is governed by a BSD-style license that
// can be found in the LICENSE file.

#include "knossos_app.h"

#include <cstdio>
#include <string>

#if !defined(OS_WIN)
#include <sys/stat.h>
#include <sys/types.h>
#endif

#include "include/cef_browser.h"
#include "include/cef_command_line.h"
#include "include/cef_file_util.h"
#include "include/cef_parser.h"
#include "include/cef_path_util.h"
#include "include/cef_process_message.h"
#include "include/cef_values.h"
#include "include/views/cef_browser_view.h"
#include "include/views/cef_window.h"
#include "include/wrapper/cef_closure_task.h"
#include "include/wrapper/cef_helpers.h"

#include "browser/knossos_bridge.h"
#include "browser/knossos_dev_tools.h"
#include "browser/knossos_handler.h"
#include "renderer/knossos_js_interface.h"

namespace {

// When using the Views framework this object provides the delegate
// implementation for the CefWindow that hosts the Views-based browser.
class KnossosWindowDelegate : public CefWindowDelegate {
public:
  explicit KnossosWindowDelegate(CefRefPtr<CefBrowserView> browser_view,
                                 bool main_browser)
      : main_browser_(main_browser), browser_view_(browser_view) {}

  void OnWindowCreated(CefRefPtr<CefWindow> window) override {
    // Add the browser view and show the window.
    window->AddChildView(browser_view_);
    window->Show();

    // Give keyboard focus to the browser view.
    browser_view_->RequestFocus();
  }

  void OnWindowDestroyed(CefRefPtr<CefWindow> window) override {
    browser_view_ = nullptr;
  }

  bool CanClose(CefRefPtr<CefWindow> window) override {
    // Allow the window to close if the browser says it's OK.
    CefRefPtr<CefBrowser> browser = browser_view_->GetBrowser();
    if (browser)
      return browser->GetHost()->TryCloseBrowser();
    return true;
  }

  bool IsFrameless(CefRefPtr<CefWindow> window) override {
    return main_browser_;
  }

  CefRect GetInitialBounds(CefRefPtr<CefWindow> window) override {
    CefRect screen_size = KnossosHandler::GetInstance()->GetScreenSize();
    CefRect window_size(0, 0, 1200, 800);

    window_size.x = (screen_size.width - window_size.width) / 2;
    window_size.y = (screen_size.height - window_size.height) / 2;

    return window_size;
  }

private:
  bool main_browser_;
  CefRefPtr<CefBrowserView> browser_view_;

  IMPLEMENT_REFCOUNTING(KnossosWindowDelegate);
  DISALLOW_COPY_AND_ASSIGN(KnossosWindowDelegate);
};

class KnossosBrowserViewDelegate : public CefBrowserViewDelegate {
public:
  KnossosBrowserViewDelegate() {}

  bool OnPopupBrowserViewCreated(CefRefPtr<CefBrowserView> browser_view,
                                 CefRefPtr<CefBrowserView> popup_browser_view,
                                 bool is_devtools) override {
    if (is_devtools) {
      CefWindow::CreateTopLevelWindow(
          new KnossosDevToolsWindowDelegate(popup_browser_view));
    } else {
      // Create a new top-level Window for the popup. It will show itself after
      // creation.
      CefWindow::CreateTopLevelWindow(
          new KnossosWindowDelegate(popup_browser_view, false));
    }

    // We created the Window.
    return true;
  }

private:
  IMPLEMENT_REFCOUNTING(KnossosBrowserViewDelegate);
  DISALLOW_COPY_AND_ASSIGN(KnossosBrowserViewDelegate);
};

} // namespace

KnossosApp::KnossosApp() {}

void KnossosApp::OnBeforeCommandLineProcessing(
    const CefString &process_type, CefRefPtr<CefCommandLine> command_line) {

  // Disable the component updater since we don't use Widevine and these updates are fetched from Google's
  // servers which tends to irritate users.
  // https://github.com/chromiumembedded/cef/issues/3149#issuecomment-1465028143
  command_line->AppendArgument("disable-component-update");

  if (process_type.empty()) {
    // Don't create a "GPUCache" directory
    command_line->AppendSwitch("disable-gpu-shader-disk-cache");

#if defined(OS_MAC)
    // Disable the toolchain prompt on macOS.
    command_line->AppendSwitch("use-mock-keychain");
#endif
  }
}

void KnossosApp::OnContextInitialized() {
  CEF_REQUIRE_UI_THREAD();

  CefRefPtr<CefCommandLine> command_line =
      CefCommandLine::GetGlobalCommandLine();

  // Check if a "--url=" value was provided via the command-line. If so, use
  // that instead of the default URL.
  std::string url = command_line->GetSwitchValue("url");
  if (url.empty())
    url = "https://files.client.fsnebula.org/index.html";

  url = "https://files.client.fsnebula.org/splash.html?load=" +
        std::string(CefURIEncode(url, true));

  // KnossosHandler implements browser-level callbacks.
  CefRefPtr<KnossosHandler> handler(new KnossosHandler(_settings_path));

  // Specify CEF browser settings here.
  CefBrowserSettings browser_settings;

  // Create the BrowserView.
  CefRefPtr<CefBrowserView> browser_view = CefBrowserView::CreateBrowserView(
      handler, url, browser_settings, nullptr, nullptr,
      new KnossosBrowserViewDelegate());

  // Create the Window. It will show itself after creation.
  CefWindow::CreateTopLevelWindow(
      new KnossosWindowDelegate(browser_view, true));

  // Load libknossos on the thread dedicated to Knossos tasks
  handler->PostKnossosTask(base::BindOnce(PrepareLibKnossos, _settings_path));
}

// We can't use CefDirectoryExists() here because it expects threads to have
// been initialised.
static bool DirectoryExists(std::string path) {
#if defined(OS_WIN)
  auto attrs = GetFileAttributesA(path.c_str());
  return attrs != INVALID_FILE_ATTRIBUTES &&
         (attrs & FILE_ATTRIBUTE_DIRECTORY) != 0;
#else
  struct stat statbuf;
  auto error = stat(path.c_str(), &statbuf);
  return error == 0 && (statbuf.st_mode & S_IFDIR) != 0;
#endif
}

void KnossosApp::InitializeSettings(CefSettings &settings,
                                    std::string appDataPath) {
  CefString path;
  if (!CefGetPath(PK_DIR_EXE, path)) {
    LOG(ERROR) << "Could not find application directory!";
  }

  std::string tmp = path;
#if defined(OS_LINUX) || defined(OS_APPLE)
  std::string sep("/");
#else
  std::string sep("\\");
#endif
  tmp += sep + "portable_settings";

  std::string config_path;
  VLOG(1) << "Portable path: " << tmp;

  if (DirectoryExists(tmp)) {
    config_path = tmp;
  } else {
    if (!appDataPath.empty()) {
      config_path = appDataPath + "/Knossos";
    } else {
      LOG(ERROR) << "Could not find appdata directory!";
    }
  }

  if (config_path.empty()) {
    KnossosHandler::ShowError("Could not find a viable configuration folder.\n"
                              "Please check the log for details.");
    LOG(FATAL) << "Could not determine a valid config folder.";
  }

  VLOG(1) << "Config path: " << config_path;

  CefString cache_path(&settings.cache_path);
  cache_path = config_path + sep + "cache";

  CefString user_data_path(&settings.user_data_path);
  user_data_path = config_path + sep + "user_data";

  CefString log_file(&settings.log_file);
  log_file = config_path + sep + "debug.log";

  _settings_path = config_path;

  settings.persist_user_preferences = 1;
  settings.background_color = CefColorSetARGB(0xff, 0x1c, 0x1c, 0x1c);
}

#ifndef OS_APPLE

// Keep in sync with knossos_helper_app.cc
bool KnossosApp::OnProcessMessageReceived(
    CefRefPtr<CefBrowser> browser, CefRefPtr<CefFrame> frame,
    CefProcessId source_process, CefRefPtr<CefProcessMessage> message) {
  return KnossosJsInterface::ProcessMessage(browser, frame, source_process,
                                            message);
}

// Keep in sync with knossos_helper_app.cc
void KnossosApp::OnWebKitInitialized() { KnossosJsInterface::Init(); }

#endif
