//go:build darwin && !ios

#import "os_notification_rich_darwin.h"
#include <Foundation/Foundation.h>
#include <string.h>

#if __MAC_OS_X_VERSION_MAX_ALLOWED >= 110000
#import <UserNotifications/UserNotifications.h>
#endif

static char* xiadownCopyString(NSString *value) {
    const char *utf8 = value ? [value UTF8String] : "unknown notification error";
    if (!utf8) {
        utf8 = "unknown notification error";
    }
    return strdup(utf8);
}

static char* xiadownCopyError(NSError *error) {
    if (!error) {
        return xiadownCopyString(@"unknown notification error");
    }
    return xiadownCopyString([error localizedDescription]);
}

char* xiadownSendNotificationWithAttachment(
    const char *identifier,
    const char *title,
    const char *subtitle,
    const char *body,
    const char *data_json,
    const char *image_path
) {
    if (@available(macOS 11.0, *)) {
        NSString *nsIdentifier = [NSString stringWithUTF8String:identifier ?: ""];
        NSString *nsTitle = [NSString stringWithUTF8String:title ?: ""];
        NSString *nsSubtitle = [NSString stringWithUTF8String:subtitle ?: ""];
        NSString *nsBody = [NSString stringWithUTF8String:body ?: ""];

        UNMutableNotificationContent *content = [[UNMutableNotificationContent alloc] init];
        content.title = nsTitle;
        if (nsSubtitle.length > 0) {
            content.subtitle = nsSubtitle;
        }
        content.body = nsBody;
        content.sound = [UNNotificationSound defaultSound];

        if (data_json && strlen(data_json) > 0) {
            NSString *dataJsonString = [NSString stringWithUTF8String:data_json];
            NSData *jsonData = [dataJsonString dataUsingEncoding:NSUTF8StringEncoding];
            NSError *jsonError = nil;
            NSDictionary *parsedData = [NSJSONSerialization JSONObjectWithData:jsonData options:0 error:&jsonError];
            if (jsonError) {
                return xiadownCopyError(jsonError);
            }
            if (parsedData) {
                content.userInfo = parsedData;
            }
        }

        if (image_path && strlen(image_path) > 0) {
            NSString *path = [NSString stringWithUTF8String:image_path];
            NSURL *url = [NSURL fileURLWithPath:path];
            NSError *attachmentError = nil;
            UNNotificationAttachment *attachment =
                [UNNotificationAttachment attachmentWithIdentifier:@"cover" URL:url options:nil error:&attachmentError];
            if (attachmentError) {
                return xiadownCopyError(attachmentError);
            }
            if (attachment) {
                content.attachments = @[attachment];
            }
        }

        UNNotificationRequest *request =
            [UNNotificationRequest requestWithIdentifier:nsIdentifier content:content trigger:nil];
        UNUserNotificationCenter *center = [UNUserNotificationCenter currentNotificationCenter];
        dispatch_semaphore_t semaphore = dispatch_semaphore_create(0);
        __block NSError *requestError = nil;
        [center addNotificationRequest:request withCompletionHandler:^(NSError * _Nullable error) {
            requestError = error;
            dispatch_semaphore_signal(semaphore);
        }];

        dispatch_time_t timeout = dispatch_time(DISPATCH_TIME_NOW, 5 * NSEC_PER_SEC);
        if (dispatch_semaphore_wait(semaphore, timeout) != 0) {
            return xiadownCopyString(@"sending notification timed out");
        }
        if (requestError) {
            return xiadownCopyError(requestError);
        }
        return NULL;
    } else {
        return xiadownCopyString(@"rich notifications require macOS 11.0 or newer");
    }
}
