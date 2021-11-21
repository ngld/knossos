#ifndef KNOSSOS_LAUNCHER_BROWSER_DYNKNOSSOS
#define KNOSSOS_LAUNCHER_BROWSER_DYNKNOSSOS

#include "../../../libknossos/api/cef_bridge.h"

#define KNOSSOS_LOG_DEBUG 1
#define KNOSSOS_LOG_INFO 2
#define KNOSSOS_LOG_WARNING 3
#define KNOSSOS_LOG_ERROR 4
#define KNOSSOS_LOG_FATAL 5

typedef void (*GODYN_KnossosFreeKnossosResponse)(KnossosResponse* response);
extern GODYN_KnossosFreeKnossosResponse KnossosFreeKnossosResponse;

// KnossosInit has to be called exactly once before calling any other exported function.
typedef uint8_t (*GODYN_KnossosInit)(KnossosInitParams* params);
extern GODYN_KnossosInit KnossosInit;

// KnossosHandleRequest handles an incoming request from CEF
typedef KnossosResponse* (*GODYN_KnossosHandleRequest)(char* urlPtr, int urlLen, void* bodyPtr, int bodyLen);
extern GODYN_KnossosHandleRequest KnossosHandleRequest;

bool LoadKnossos(const char* knossos_path, char** error);

#endif /* KNOSSOS_LAUNCHER_BROWSER_DYNKNOSSOS */
