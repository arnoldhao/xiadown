//go:build darwin && !ios

package wails

/*
#cgo CFLAGS: -mmacosx-version-min=14.0 -x objective-c
#cgo LDFLAGS: -framework Foundation -framework LocalAuthentication -framework Security

#import <Foundation/Foundation.h>
#import <LocalAuthentication/LocalAuthentication.h>
#import <Security/Security.h>

static NSString *xiadownLegacyAppSessionService(void) {
	return @"com.dreamapp.xiadown.connector-app-session";
}

static int xiadownLegacyAppSessionCount(int *statusOut) {
	@autoreleasepool {
		if (statusOut != NULL) *statusOut = (int)errSecSuccess;
		LAContext *authenticationContext = [[[LAContext alloc] init] autorelease];
		authenticationContext.interactionNotAllowed = YES;
		NSDictionary *query = @{
			(id)kSecClass: (id)kSecClassGenericPassword,
			(id)kSecAttrService: xiadownLegacyAppSessionService(),
			(id)kSecReturnAttributes: @YES,
			(id)kSecMatchLimit: (id)kSecMatchLimitAll,
			(id)kSecUseAuthenticationContext: authenticationContext,
		};
		CFTypeRef result = NULL;
		OSStatus status = SecItemCopyMatching((CFDictionaryRef)query, &result);
		if (status == errSecItemNotFound) {
			if (statusOut != NULL) *statusOut = (int)errSecSuccess;
			return 0;
		}
		if (statusOut != NULL) *statusOut = (int)status;
		if (status != errSecSuccess || result == NULL) {
			if (result != NULL) CFRelease(result);
			return 0;
		}
		int count = 0;
		if (CFGetTypeID(result) == CFArrayGetTypeID()) {
			count = (int)CFArrayGetCount((CFArrayRef)result);
		} else if (CFGetTypeID(result) == CFDictionaryGetTypeID()) {
			count = 1;
		}
		CFRelease(result);
		return count;
	}
}

static int xiadownDeleteLegacyAppSessions(void) {
	@autoreleasepool {
		LAContext *authenticationContext = [[[LAContext alloc] init] autorelease];
		authenticationContext.interactionNotAllowed = YES;
		NSDictionary *query = @{
			(id)kSecClass: (id)kSecClassGenericPassword,
			(id)kSecAttrService: xiadownLegacyAppSessionService(),
			(id)kSecUseAuthenticationContext: authenticationContext,
		};
		OSStatus status = SecItemDelete((CFDictionaryRef)query);
		return status == errSecItemNotFound ? (int)errSecSuccess : (int)status;
	}
}
*/
import "C"

import "fmt"

func legacyAppSessionSecretInventory() (int, int64, error) {
	var status C.int
	count := int(C.xiadownLegacyAppSessionCount(&status))
	if status != C.int(C.errSecSuccess) {
		return 0, 0, fmt.Errorf("inspect legacy App Sessions: status %d", int(status))
	}
	// Keychain attributes intentionally omit kSecReturnData, so no secret is
	// read merely to manufacture a byte count.
	return count, 0, nil
}

func clearLegacyAppSessionSecrets() error {
	status := C.xiadownDeleteLegacyAppSessions()
	if status != C.int(C.errSecSuccess) {
		return fmt.Errorf("clear legacy App Sessions: status %d", int(status))
	}
	return nil
}
