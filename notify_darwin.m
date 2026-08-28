#import <CoreServices/CoreServices.h>
#import <Foundation/Foundation.h>
#import <UserNotifications/UserNotifications.h>
#include "notify_darwin.h"

// implemented in Go (notify_darwin.go)
extern void goNotificationClicked(char *payload);

@interface ANNotifyDelegate : NSObject <UNUserNotificationCenterDelegate>
@end

@implementation ANNotifyDelegate
// show banners even while the dashboard window is frontmost
- (void)userNotificationCenter:(UNUserNotificationCenter *)center
       willPresentNotification:(UNNotification *)notification
         withCompletionHandler:(void (^)(UNNotificationPresentationOptions))completionHandler {
    completionHandler(UNNotificationPresentationOptionBanner | UNNotificationPresentationOptionSound);
}

- (void)userNotificationCenter:(UNUserNotificationCenter *)center
didReceiveNotificationResponse:(UNNotificationResponse *)response
         withCompletionHandler:(void (^)(void))completionHandler {
    if ([response.actionIdentifier isEqualToString:UNNotificationDefaultActionIdentifier]) {
        NSString *payload = response.notification.request.content.userInfo[@"payload"];
        if (payload != nil) {
            goNotificationClicked((char *)[payload UTF8String]);
        }
    }
    completionHandler();
}
@end

static ANNotifyDelegate *anDelegate = nil;

bool unBundled(void) {
    return [[NSBundle mainBundle] bundleIdentifier] != nil;
}

void unSetup(void) {
    if (!unBundled()) {
        return;
    }
    UNUserNotificationCenter *c = [UNUserNotificationCenter currentNotificationCenter];
    anDelegate = [ANNotifyDelegate new];
    c.delegate = anDelegate;
    [c requestAuthorizationWithOptions:(UNAuthorizationOptionAlert | UNAuthorizationOptionSound |
                                        UNAuthorizationOptionBadge)
                     completionHandler:^(BOOL granted, NSError *error){
                     }];
}

int unAuthStatus(void) {
    if (!unBundled()) {
        return -1;
    }
    __block int result = 2;
    dispatch_semaphore_t sem = dispatch_semaphore_create(0);
    [[UNUserNotificationCenter currentNotificationCenter]
        getNotificationSettingsWithCompletionHandler:^(UNNotificationSettings *s) {
          switch (s.authorizationStatus) {
          case UNAuthorizationStatusAuthorized:
          case UNAuthorizationStatusProvisional:
              result = 1;
              break;
          case UNAuthorizationStatusDenied:
              result = 0;
              break;
          default:
              result = 2;
              break;
          }
          dispatch_semaphore_signal(sem);
        }];
    dispatch_semaphore_wait(sem, dispatch_time(DISPATCH_TIME_NOW, (int64_t)(700 * NSEC_PER_MSEC)));
    return result;
}

void unRequestAuth(void) {
    if (!unBundled()) {
        return;
    }
    [[UNUserNotificationCenter currentNotificationCenter]
        requestAuthorizationWithOptions:(UNAuthorizationOptionAlert | UNAuthorizationOptionSound |
                                         UNAuthorizationOptionBadge)
                      completionHandler:^(BOOL granted, NSError *error){
                      }];
}

int aeAutomationStatus(const char *bundleID, bool ask) {
    NSAppleEventDescriptor *target = [NSAppleEventDescriptor
        descriptorWithBundleIdentifier:[NSString stringWithUTF8String:bundleID]];
    OSStatus s = AEDeterminePermissionToAutomateTarget(target.aeDesc, typeWildCard,
                                                       typeWildCard, ask);
    if (s == noErr) {
        return 1;
    }
    if (s == errAEEventNotPermitted) {
        return 0;
    }
    if (s == -1744) { /* errAEEventWouldRequireUserConsent */
        return 2;
    }
    return -1;
}

void unNotify(const char *ident, const char *title, const char *subtitle,
              const char *body, const char *payload) {
    UNMutableNotificationContent *content = [UNMutableNotificationContent new];
    if (title != NULL) {
        content.title = [NSString stringWithUTF8String:title];
    }
    if (subtitle != NULL && subtitle[0] != '\0') {
        content.subtitle = [NSString stringWithUTF8String:subtitle];
    }
    if (body != NULL) {
        content.body = [NSString stringWithUTF8String:body];
    }
    content.sound = [UNNotificationSound defaultSound];
    if (payload != NULL && payload[0] != '\0') {
        content.userInfo = @{@"payload" : [NSString stringWithUTF8String:payload]};
    }
    NSString *nid = (ident != NULL && ident[0] != '\0')
                        ? [NSString stringWithUTF8String:ident]
                        : [[NSUUID UUID] UUIDString];
    UNNotificationRequest *req = [UNNotificationRequest requestWithIdentifier:nid
                                                                      content:content
                                                                      trigger:nil];
    [[UNUserNotificationCenter currentNotificationCenter]
        addNotificationRequest:req
         withCompletionHandler:^(NSError *e){
         }];
}
