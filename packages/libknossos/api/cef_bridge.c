#include "cef_bridge.h"

void call_log_cb(KnossosLogCallback cb, uint8_t level, char* message, int length) {
	cb(level, message, length);
}

void call_message_cb(KnossosMessageCallback cb, void* message, int length) {
  cb(message, length);
}

KnossosHeader* make_header_array(uint8_t length) {
  return (KnossosHeader*) malloc(sizeof(KnossosHeader) * length);
}

EXTERN void KnossosFreeKnossosResponse(KnossosResponse* response) {
  for (int i = 0; i < response->header_count; i++) {
    KnossosHeader *hdr = &response->headers[i];
    free(hdr->header_name);
    free(hdr->value);
  }
  if (response->header_count > 0) free(response->headers);
  if (response->response_length > 0) free(response->response_data);
  free(response);
}
