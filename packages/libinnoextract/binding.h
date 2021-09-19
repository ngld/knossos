#ifndef KNOSSOS_LIBINNOEXTRACT_BINDING
#define KNOSSOS_LIBINNOEXTRACT_BINDING

#include <stdbool.h>

extern bool extract_inno_wrapper(const char *path, const char *destination);
extern bool load_libinnoextract(const char *lib_path, char **error);

#endif /* KNOSSOS_BINDING */
