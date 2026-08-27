#ifndef NOTIFY_DARWIN_H
#define NOTIFY_DARWIN_H

#include <stdbool.h>

// true when running from a real .app bundle — UNUserNotificationCenter
// requires one and throws otherwise.
bool unBundled(void);

// install the notification delegate and request authorization once.
void unSetup(void);

// post one notification; payload lands in userInfo and comes back to Go
// via goNotificationClicked when the user clicks the banner.
void unNotify(const char *ident, const char *title, const char *subtitle,
              const char *body, const char *payload);

#endif
