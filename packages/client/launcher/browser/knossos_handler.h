#ifndef KNOSSOS_LAUNCHER_BROWSER_KNOSSOS_HANDLER
#define KNOSSOS_LAUNCHER_BROWSER_KNOSSOS_HANDLER

#include <list>

#include "include/base/cef_callback.h"
#include "include/cef_client.h"
#include "include/cef_drag_handler.h"
#include "include/cef_thread.h"

#include "browser/knossos_archive.h"

class KnossosHandler : public CefClient,
                       public CefDisplayHandler,
                       public CefLifeSpanHandler,
                       public CefLoadHandler,
                       public CefContextMenuHandler,
                       public CefRequestHandler,
                       public CefDragHandler {
public:
  explicit KnossosHandler(bool use_views, std::string settings_path);
  ~KnossosHandler();

  // Provide access to the single global instance of this object.
  static KnossosHandler *GetInstance();

  // CefClient methods:
  virtual CefRefPtr<CefDisplayHandler> GetDisplayHandler() override {
    return this;
  }
  virtual CefRefPtr<CefLifeSpanHandler> GetLifeSpanHandler() override {
    return this;
  }
  virtual CefRefPtr<CefLoadHandler> GetLoadHandler() override { return this; }
  virtual CefRefPtr<CefContextMenuHandler> GetContextMenuHandler() override {
    return this;
  }
  virtual CefRefPtr<CefRequestHandler> GetRequestHandler() override {
    return this;
  }
  virtual CefRefPtr<CefDragHandler> GetDragHandler() override { return this; }

  virtual bool
  OnProcessMessageReceived(CefRefPtr<CefBrowser> browser,
                           CefRefPtr<CefFrame> frame,
                           CefProcessId source_process,
                           CefRefPtr<CefProcessMessage> message) override;

  // CefDisplayHandler methods:
  virtual void OnTitleChange(CefRefPtr<CefBrowser> browser,
                             const CefString &title) override;

  // CefLifeSpanHandler methods:
  /*virtual bool OnBeforePopup( CefRefPtr< CefBrowser > browser, CefRefPtr<
   * CefFrame > frame, const CefString& target_url, const CefString&
   * target_frame_name, CefLifeSpanHandler::WindowOpenDisposition
   * target_disposition, bool user_gesture, const CefPopupFeatures&
   * popupFeatures, CefWindowInfo& windowInfo, CefRefPtr< CefClient >& client,
   * CefBrowserSettings& settings, CefRefPtr< CefDictionaryValue >& extra_info,
   * bool* no_javascript_access ) override;*/
  virtual void OnAfterCreated(CefRefPtr<CefBrowser> browser) override;
  virtual bool DoClose(CefRefPtr<CefBrowser> browser) override;
  virtual void OnBeforeClose(CefRefPtr<CefBrowser> browser) override;

  // CefLoadHandler methods:
  virtual void OnLoadError(CefRefPtr<CefBrowser> browser,
                           CefRefPtr<CefFrame> frame, ErrorCode errorCode,
                           const CefString &errorText,
                           const CefString &failedUrl) override;

  // CefContextMenuHandler methods:
  virtual void OnBeforeContextMenu(CefRefPtr<CefBrowser> browser,
                                   CefRefPtr<CefFrame> frame,
                                   CefRefPtr<CefContextMenuParams> params,
                                   CefRefPtr<CefMenuModel> model) override;
  virtual bool
  OnContextMenuCommand(CefRefPtr<CefBrowser> browser, CefRefPtr<CefFrame> frame,
                       CefRefPtr<CefContextMenuParams> params, int command_id,
                       CefContextMenuHandler::EventFlags event_flags) override;

  // CefRequestHandler methods:
  virtual bool OnBeforeBrowse(CefRefPtr<CefBrowser> browser,
                              CefRefPtr<CefFrame> frame,
                              CefRefPtr<CefRequest> request, bool user_gesture,
                              bool is_redirect) override;
  virtual CefRefPtr<CefResourceRequestHandler> GetResourceRequestHandler(
      CefRefPtr<CefBrowser> browser, CefRefPtr<CefFrame> frame,
      CefRefPtr<CefRequest> request, bool is_navigation, bool is_download,
      const CefString &request_initiator,
      bool &disable_default_handling) override;

  // CefDragHandler methods:
  void OnDraggableRegionsChanged(
      CefRefPtr<CefBrowser> browser, CefRefPtr<CefFrame> frame,
      const std::vector<CefDraggableRegion> &regions) override;

  // Request that all existing browser windows close.
  void CloseAllBrowsers(bool force_close);
  void BroadcastMessage(CefRefPtr<CefProcessMessage> message);

  bool IsClosing() const { return is_closing_; }

  CefRect GetScreenSize();
  std::string GetSettingsPath() { return _settings_path; };
  CefRefPtr<CefBrowser> GetMainBrowser() { return browser_list_.front(); };

  bool PostKnossosTask(CefRefPtr<CefTask> task);
  bool PostKnossosTask(const base::Closure &closure);

  static void ShowError(std::string message);

private:
  friend class KnossosApp;

  // Platform-specific implementation.
  void PlatformInit();
  void PlatformTitleChange(CefRefPtr<CefBrowser> browser,
                           const CefString &title);
  void SaveFileDialog(CefRefPtr<CefBrowser> browser, std::string title,
                      std::string default_filepath,
                      std::vector<std::string> accepted,
                      CefRefPtr<CefRunFileDialogCallback> callback);
  void OpenFileDialog(CefRefPtr<CefBrowser> browser, std::string title,
                      std::string default_filepath,
                      std::vector<std::string> accepted,
                      CefRefPtr<CefRunFileDialogCallback> callback);
  void OpenFolderDialog(CefRefPtr<CefBrowser> browser, std::string title,
                        std::string folder,
                        CefRefPtr<CefRunFileDialogCallback> callback);

  // True if the application is using the Views framework.
  const bool use_views_;

  // List of existing browser windows. Only accessed on the CEF UI thread.
  typedef std::list<CefRefPtr<CefBrowser>> BrowserList;
  BrowserList browser_list_;

  bool is_closing_;
  CefRefPtr<KnossosArchive> _resources;

  std::string _settings_path;
  CefRefPtr<CefThread> knossos_thread_;

  // Include the default reference counting implementation.
  IMPLEMENT_REFCOUNTING(KnossosHandler);
};

#endif /* KNOSSOS_LAUNCHER_BROWSER_KNOSSOS_HANDLER */
