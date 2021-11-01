#include <dispatch/dispatch.h>

extern void libknossos_run_item(int id);

void run_on_main(int id) {
    dispatch_sync(dispatch_get_main_queue(), ^{
        libknossos_run_item(id);
    });
}
