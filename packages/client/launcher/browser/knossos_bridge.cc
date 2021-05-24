#include "browser/knossos_bridge.h"

#include "include/base/cef_logging.h"
#include "include/cef_command_line.h"
#include "include/cef_values.h"
#include "include/cef_process_message.h"
#include "include/cef_path_util.h"

#include "browser/knossos_handler.h"
#include "dynknossos.h"

static void KnossosLogger(uint8_t level, char* message, int length) {
  std::string msg((const char*)message, length);
  switch (level) {
  case KNOSSOS_LOG_DEBUG:
#ifndef NDEBUG
    LOG(INFO) << "D: " << msg;
#endif
    break;
  case KNOSSOS_LOG_INFO:
    LOG(INFO) << msg;
    break;
  case KNOSSOS_LOG_WARNING:
    LOG(WARNING) << msg;
    break;
  case KNOSSOS_LOG_ERROR:
    LOG(ERROR) << msg;
    break;
  case KNOSSOS_LOG_FATAL:
    LOG(FATAL) << msg;
    break;
  default:
    LOG(FATAL) << "Invalid log level passed to KnossosLogger: " << level;
  }
}

static void KnossosMessageDispatcher(void* message, int length) {
  auto process_message = CefProcessMessage::Create("knJsMessage");
  process_message->GetArgumentList()->SetBinary(0, CefBinaryValue::Create(message, length));

  KnossosHandler::GetInstance()->BroadcastMessage(process_message);
}

void PrepareLibKnossos(std::string settings_path) {
  std::string libknossos_path;

  CefRefPtr<CefCommandLine> command_line =
      CefCommandLine::GetGlobalCommandLine();

  if (command_line->HasSwitch("libkn")) {
    libknossos_path = command_line->GetSwitchValue("libkn");
  } else {
#if defined(OS_WIN)
    libknossos_path = "libknossos.dll";
#elif defined(OS_LINUX)
    libknossos_path = "libknossos.so";
#elif defined(OS_MAC)
    libknossos_path = "./libknossos.dylib";
    CefString exe_dir;
    if (CefGetPath(PK_DIR_EXE, exe_dir)) {
      libknossos_path = exe_dir;
      libknossos_path += "/libknossos.dylib";
    }
#endif
  }

  CefString resource_path;
  if (!CefGetPath(PK_DIR_RESOURCES, resource_path)) {
    LOG(FATAL) << "Resource directory not found";
  }

  char* error;
  if (!LoadKnossos(libknossos_path.c_str(), &error)) {
    std::stringstream msg;
    msg << "Failed to load libknossos from " << libknossos_path
        << ": " << error;
    KnossosHandler::ShowError(msg.str());
    LOG(FATAL) << msg.str();
  }

  std::string path_conv = resource_path;
  KnossosInitParams params;
  params.resource_path = path_conv.c_str();
  params.resource_len = (int) path_conv.size();
  params.settings_path = settings_path.c_str();
  params.settings_len = (int) settings_path.size();

  params.log_cb = &KnossosLogger;
  params.message_cb = &KnossosMessageDispatcher;

  if (!KnossosInit(&params)) {
    LOG(FATAL) << "Failed to initialize libknossos";
  }
}
