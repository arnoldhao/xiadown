#ifndef XIADOWN_LIBRARY_ROOT_SYNC_WATCHER_DARWIN_H
#define XIADOWN_LIBRARY_ROOT_SYNC_WATCHER_DARWIN_H

#include <stdint.h>

typedef void *xiadown_fsevents_ref;

uint64_t xiadown_fsevents_current_event_id(void);
xiadown_fsevents_ref xiadown_fsevents_start(
    const char *path,
    uint64_t since,
    uintptr_t go_handle
);
void xiadown_fsevents_stop(xiadown_fsevents_ref watcher);

extern void xiadownRootSyncFSEvent(
    uintptr_t go_handle,
    char *path,
    uint64_t cursor,
    uint32_t flags
);

#endif
