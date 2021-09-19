#ifndef KNOSSOS_API_CEF_BRIDGE
#define KNOSSOS_API_CEF_BRIDGE

#include <stdlib.h>
#include <stdint.h>
#include <string.h>

#ifdef __MINGW32__
#define EXTERN extern __declspec(dllexport)
#else
#define EXTERN extern
#endif

typedef void (*KnossosLogCallback)(uint8_t level, char* message, int length);
typedef void (*KnossosMessageCallback)(void* message, int length);

typedef struct {
  const char* settings_path;
  const char* resource_path;
  int settings_len;
  int resource_len;
  KnossosLogCallback log_cb;
  KnossosMessageCallback message_cb;
} KnossosInitParams;

typedef struct {
	char* header_name;
  char* value;
  size_t header_len;
  size_t value_len;
} KnossosHeader;

typedef struct {
  KnossosHeader* headers;
	void* response_data;
  int status_code;
  uint8_t header_count;
  size_t response_length;
} KnossosResponse;

#ifndef KNOSSOS_BRAIN_LOADER
// internal
void call_log_cb(KnossosLogCallback cb, uint8_t level, char* message, int length);
void call_message_cb(KnossosMessageCallback cb, void* message, int length);
KnossosHeader* make_header_array(uint8_t length);
#endif

#endif /* KNOSSOS_API_CEF_BRIDGE */
