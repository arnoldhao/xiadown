//go:build darwin && cgo && !ios

#include <CoreServices/CoreServices.h>
#include <dispatch/dispatch.h>
#include <stdlib.h>
#include "watcher_darwin.h"

typedef struct {
    FSEventStreamRef stream;
    dispatch_queue_t queue;
    uintptr_t go_handle;
} xiadown_fsevents_watcher;

uint64_t xiadown_fsevents_current_event_id(void) {
    return (uint64_t)FSEventsGetCurrentEventId();
}

static void xiadown_fsevents_callback(
    ConstFSEventStreamRef stream,
    void *client_info,
    size_t count,
    void *event_paths,
    const FSEventStreamEventFlags flags[],
    const FSEventStreamEventId ids[]
) {
    (void)stream;
    xiadown_fsevents_watcher *watcher = client_info;
    CFArrayRef paths = event_paths;
    for (size_t index = 0; index < count; index++) {
        CFStringRef value = (CFStringRef)CFArrayGetValueAtIndex(
            paths,
            (CFIndex)index
        );
        if (value == NULL) {
            continue;
        }
        CFIndex length = CFStringGetMaximumSizeForEncoding(
            CFStringGetLength(value),
            kCFStringEncodingUTF8
        ) + 1;
        char *path = malloc((size_t)length);
        if (path == NULL) {
            continue;
        }
        if (CFStringGetCString(value, path, length, kCFStringEncodingUTF8)) {
            xiadownRootSyncFSEvent(
                watcher->go_handle,
                path,
                (uint64_t)ids[index],
                (uint32_t)flags[index]
            );
        }
        free(path);
    }
}

xiadown_fsevents_ref xiadown_fsevents_start(
    const char *path,
    uint64_t since,
    uintptr_t go_handle
) {
    if (path == NULL || path[0] == '\0') {
        return NULL;
    }
    xiadown_fsevents_watcher *watcher = calloc(1, sizeof(*watcher));
    if (watcher == NULL) {
        return NULL;
    }
    CFStringRef root = CFStringCreateWithCString(
        kCFAllocatorDefault,
        path,
        kCFStringEncodingUTF8
    );
    if (root == NULL) {
        free(watcher);
        return NULL;
    }
    const void *values[] = { root };
    CFArrayRef paths = CFArrayCreate(
        kCFAllocatorDefault,
        values,
        1,
        &kCFTypeArrayCallBacks
    );
    FSEventStreamContext context = {
        .version = 0,
        .info = watcher,
        .retain = NULL,
        .release = NULL,
        .copyDescription = NULL,
    };
    FSEventStreamEventId start_id = since == 0
        ? kFSEventStreamEventIdSinceNow
        : (FSEventStreamEventId)since;
    watcher->go_handle = go_handle;
    watcher->queue = dispatch_queue_create(
        "com.xiadown.library-root-sync",
        DISPATCH_QUEUE_SERIAL
    );
    watcher->stream = FSEventStreamCreate(
        kCFAllocatorDefault,
        xiadown_fsevents_callback,
        &context,
        paths,
        start_id,
        1.0,
        kFSEventStreamCreateFlagUseCFTypes |
            kFSEventStreamCreateFlagFileEvents |
            kFSEventStreamCreateFlagWatchRoot
    );
    CFRelease(paths);
    CFRelease(root);
    if (watcher->stream == NULL) {
        watcher->queue = NULL;
        free(watcher);
        return NULL;
    }
    FSEventStreamSetDispatchQueue(watcher->stream, watcher->queue);
    if (!FSEventStreamStart(watcher->stream)) {
        FSEventStreamInvalidate(watcher->stream);
        FSEventStreamRelease(watcher->stream);
        watcher->stream = NULL;
        watcher->queue = NULL;
        free(watcher);
        return NULL;
    }
    return watcher;
}

void xiadown_fsevents_stop(xiadown_fsevents_ref reference) {
    xiadown_fsevents_watcher *watcher = reference;
    if (watcher == NULL) {
        return;
    }
    FSEventStreamStop(watcher->stream);
    FSEventStreamInvalidate(watcher->stream);
    FSEventStreamRelease(watcher->stream);
    watcher->stream = NULL;
    watcher->queue = NULL;
    free(watcher);
}
