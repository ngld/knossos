#ifndef KNOSSOS_SRC_LIBINNOEXTRACT
#define KNOSSOS_SRC_LIBINNOEXTRACT

#include <stdint.h>
#include <stdbool.h>

typedef void (*inno_progress_callback)(const char* message, float progress);
typedef void (*inno_log_callback)(uint8_t level, const char* message);

#ifdef __cplusplus
#define EXTERN extern "C"
#else
#define EXTERN extern
#endif

EXTERN bool extract_inno(const char *path, const char *destination, inno_progress_callback callback, inno_log_callback log);

#endif /* KNOSSOS_SRC_LIB */
