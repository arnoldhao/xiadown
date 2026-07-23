//go:build darwin && !ios

package appsessionvault

/*
#cgo CFLAGS: -mmacosx-version-min=14.0 -x objective-c
#cgo LDFLAGS: -framework Foundation -framework LocalAuthentication -framework Security

#include <stdlib.h>
#include <string.h>
#import <Foundation/Foundation.h>
#import <LocalAuthentication/LocalAuthentication.h>
#import <Security/Security.h>

static int sessionVaultLoadMasterKey(
	const char *serviceValue,
	const char *accountValue,
	void **bytesOut,
	size_t *lengthOut
) {
	@autoreleasepool {
		if (bytesOut != NULL) *bytesOut = NULL;
		if (lengthOut != NULL) *lengthOut = 0;
		if (serviceValue == NULL || accountValue == NULL || bytesOut == NULL || lengthOut == NULL) {
			return (int)errSecParam;
		}
		NSString *service = [NSString stringWithUTF8String:serviceValue];
		NSString *account = [NSString stringWithUTF8String:accountValue];
		if (service.length == 0 || account.length == 0) {
			return (int)errSecParam;
		}
		// Session reads are always non-interactive; a locked Keychain or a
		// code-signing ACL mismatch fails closed instead of showing a password
		// prompt. The key is cached after this first successful read.
		LAContext *authenticationContext = [[[LAContext alloc] init] autorelease];
		authenticationContext.interactionNotAllowed = YES;
		NSDictionary *query = @{
			(id)kSecClass: (id)kSecClassGenericPassword,
			(id)kSecAttrService: service,
			(id)kSecAttrAccount: account,
			(id)kSecReturnData: @YES,
			(id)kSecMatchLimit: (id)kSecMatchLimitOne,
			(id)kSecUseAuthenticationContext: authenticationContext,
		};
		CFTypeRef result = NULL;
		OSStatus status = SecItemCopyMatching((CFDictionaryRef)query, &result);
		if (status != errSecSuccess || result == NULL) {
			if (result != NULL) CFRelease(result);
			return (int)status;
		}
		NSData *data = (NSData *)result;
		NSUInteger length = data.length;
		void *copy = length == 0 ? NULL : malloc(length);
		if (length > 0 && copy == NULL) {
			CFRelease(result);
			return (int)errSecAllocate;
		}
		if (length > 0) memcpy(copy, data.bytes, length);
		CFRelease(result);
		*bytesOut = copy;
		*lengthOut = (size_t)length;
		return (int)errSecSuccess;
	}
}

static int sessionVaultInspectMasterKey(
	const char *serviceValue,
	const char *accountValue,
	int *existsOut
) {
	@autoreleasepool {
		if (existsOut != NULL) *existsOut = 0;
		if (serviceValue == NULL || accountValue == NULL || existsOut == NULL) {
			return (int)errSecParam;
		}
		NSString *service = [NSString stringWithUTF8String:serviceValue];
		NSString *account = [NSString stringWithUTF8String:accountValue];
		if (service.length == 0 || account.length == 0) {
			return (int)errSecParam;
		}
		LAContext *authenticationContext = [[[LAContext alloc] init] autorelease];
		authenticationContext.interactionNotAllowed = YES;
		NSDictionary *query = @{
			(id)kSecClass: (id)kSecClassGenericPassword,
			(id)kSecAttrService: service,
			(id)kSecAttrAccount: account,
			// Inventory asks only whether the item exists. It deliberately omits
			// kSecReturnData so opening Data Management never reads the key.
			(id)kSecReturnAttributes: @YES,
			(id)kSecMatchLimit: (id)kSecMatchLimitOne,
			(id)kSecUseAuthenticationContext: authenticationContext,
		};
		CFTypeRef result = NULL;
		OSStatus status = SecItemCopyMatching((CFDictionaryRef)query, &result);
		if (status == errSecItemNotFound) return (int)errSecSuccess;
		if (status != errSecSuccess) {
			if (result != NULL) CFRelease(result);
			return (int)status;
		}
		if (result != NULL) CFRelease(result);
		*existsOut = 1;
		return (int)errSecSuccess;
	}
}

static int sessionVaultStoreMasterKey(
	const char *serviceValue,
	const char *accountValue,
	const void *bytes,
	size_t length
) {
	@autoreleasepool {
		if (serviceValue == NULL || accountValue == NULL || bytes == NULL || length == 0) {
			return (int)errSecParam;
		}
		NSString *service = [NSString stringWithUTF8String:serviceValue];
		NSString *account = [NSString stringWithUTF8String:accountValue];
		NSData *data = [NSData dataWithBytes:bytes length:length];
		if (service.length == 0 || account.length == 0 || data == nil) {
			return (int)errSecParam;
		}
		// This is intentionally the macOS login Keychain rather than the Data
		// Protection Keychain. The latter requires a provisioning-profile-backed
		// application-identifier entitlement and rejects Wails development and
		// direct Developer ID builds with errSecMissingEntitlement. A single
		// classic Keychain item plus stable code signing keeps the master key
		// device-local without putting Keychain access in the per-session path.
		LAContext *authenticationContext = [[[LAContext alloc] init] autorelease];
		authenticationContext.interactionNotAllowed = YES;
		NSDictionary *item = @{
			(id)kSecClass: (id)kSecClassGenericPassword,
			(id)kSecAttrService: service,
			(id)kSecAttrAccount: account,
			(id)kSecValueData: data,
			(id)kSecAttrAccessible: (id)kSecAttrAccessibleWhenUnlocked,
			(id)kSecAttrSynchronizable: @NO,
			(id)kSecUseAuthenticationContext: authenticationContext,
		};
		return (int)SecItemAdd((CFDictionaryRef)item, NULL);
	}
}

static int sessionVaultDeleteMasterKey(
	const char *serviceValue,
	const char *accountValue
) {
	@autoreleasepool {
		if (serviceValue == NULL || accountValue == NULL) {
			return (int)errSecParam;
		}
		NSString *service = [NSString stringWithUTF8String:serviceValue];
		NSString *account = [NSString stringWithUTF8String:accountValue];
		if (service.length == 0 || account.length == 0) {
			return (int)errSecParam;
		}
		// Reset runs before the application UI exists. Fail closed if Keychain
		// interaction would be required instead of presenting a system prompt.
		LAContext *authenticationContext = [[[LAContext alloc] init] autorelease];
		authenticationContext.interactionNotAllowed = YES;
		NSDictionary *query = @{
			(id)kSecClass: (id)kSecClassGenericPassword,
			(id)kSecAttrService: service,
			(id)kSecAttrAccount: account,
			(id)kSecUseAuthenticationContext: authenticationContext,
		};
		OSStatus status = SecItemDelete((CFDictionaryRef)query);
		return status == errSecItemNotFound ? (int)errSecSuccess : (int)status;
	}
}

static int sessionVaultDeleteLegacyService(const char *serviceValue) {
	@autoreleasepool {
		if (serviceValue == NULL) return (int)errSecParam;
		NSString *service = [NSString stringWithUTF8String:serviceValue];
		if (service.length == 0) return (int)errSecParam;
		// Legacy cleanup follows the same strictly non-interactive policy.
		LAContext *authenticationContext = [[[LAContext alloc] init] autorelease];
		authenticationContext.interactionNotAllowed = YES;
		NSDictionary *query = @{
			(id)kSecClass: (id)kSecClassGenericPassword,
			(id)kSecAttrService: service,
			(id)kSecUseAuthenticationContext: authenticationContext,
		};
		OSStatus status = SecItemDelete((CFDictionaryRef)query);
		return status == errSecItemNotFound ? (int)errSecSuccess : (int)status;
	}
}
*/
import "C"

import (
	"context"
	"fmt"
	"unsafe"
)

type darwinMasterKeyStore struct {
	service string
	account string
}

func newPlatformMasterKeyStore() masterKeyStore {
	return darwinMasterKeyStore{
		service: DarwinKeychainService,
		account: masterKeyAccount,
	}
}

func inspectPlatformMasterKey(ctx context.Context) (int, int64, error) {
	if err := contextError(ctx); err != nil {
		return 0, 0, err
	}
	service := C.CString(DarwinKeychainService)
	account := C.CString(masterKeyAccount)
	defer C.free(unsafe.Pointer(service))
	defer C.free(unsafe.Pointer(account))
	var exists C.int
	status := C.sessionVaultInspectMasterKey(service, account, &exists)
	if status != C.int(C.errSecSuccess) {
		return 0, 0, fmt.Errorf("inspect Session Vault Keychain item: status %d", int(status))
	}
	if exists == 0 {
		return 0, 0, nil
	}
	return 1, 0, nil
}

func (store darwinMasterKeyStore) Load(ctx context.Context) ([]byte, error) {
	store = store.withDefaults()
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	service := C.CString(store.service)
	account := C.CString(store.account)
	defer C.free(unsafe.Pointer(service))
	defer C.free(unsafe.Pointer(account))
	var raw unsafe.Pointer
	var length C.size_t
	status := C.sessionVaultLoadMasterKey(service, account, &raw, &length)
	if status == C.int(C.errSecItemNotFound) {
		return nil, errMasterKeyNotFound
	}
	if status != C.int(C.errSecSuccess) {
		return nil, fmt.Errorf("load Session Vault Keychain item: status %d", int(status))
	}
	if raw == nil || length == 0 {
		return nil, fmt.Errorf("load Session Vault Keychain item: empty value")
	}
	defer C.free(raw)
	return C.GoBytes(raw, C.int(length)), nil
}

func (store darwinMasterKeyStore) Store(ctx context.Context, key []byte) error {
	store = store.withDefaults()
	if err := contextError(ctx); err != nil {
		return err
	}
	if len(key) != masterKeyBytes {
		return fmt.Errorf("invalid master key length")
	}
	service := C.CString(store.service)
	account := C.CString(store.account)
	defer C.free(unsafe.Pointer(service))
	defer C.free(unsafe.Pointer(account))
	status := C.sessionVaultStoreMasterKey(
		service,
		account,
		unsafe.Pointer(&key[0]),
		C.size_t(len(key)),
	)
	if status == C.int(C.errSecDuplicateItem) {
		return errMasterKeyAlreadyExists
	}
	if status != C.int(C.errSecSuccess) {
		return fmt.Errorf("store Session Vault Keychain item: status %d", int(status))
	}
	return nil
}

func (store darwinMasterKeyStore) Delete(ctx context.Context) error {
	store = store.withDefaults()
	if err := contextError(ctx); err != nil {
		return err
	}
	service := C.CString(store.service)
	account := C.CString(store.account)
	defer C.free(unsafe.Pointer(service))
	defer C.free(unsafe.Pointer(account))
	status := C.sessionVaultDeleteMasterKey(service, account)
	if status != C.int(C.errSecSuccess) {
		return fmt.Errorf("delete Session Vault Keychain item: status %d", int(status))
	}
	return nil
}

func (store darwinMasterKeyStore) withDefaults() darwinMasterKeyStore {
	if store.service == "" {
		store.service = DarwinKeychainService
	}
	if store.account == "" {
		store.account = masterKeyAccount
	}
	return store
}

func deleteLegacyAppSessionSecrets(ctx context.Context) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	service := C.CString(legacyKeychainService)
	defer C.free(unsafe.Pointer(service))
	status := C.sessionVaultDeleteLegacyService(service)
	if status != C.int(C.errSecSuccess) {
		return fmt.Errorf("delete legacy App Session Keychain items: status %d", int(status))
	}
	return nil
}

func contextError(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}
