//go:build darwin && !ios

#ifndef XIADOWN_OS_NOTIFICATION_RICH_DARWIN_H
#define XIADOWN_OS_NOTIFICATION_RICH_DARWIN_H

char* xiadownSendNotificationWithAttachment(
    const char *identifier,
    const char *title,
    const char *subtitle,
    const char *body,
    const char *data_json,
    const char *image_path
);

#endif
