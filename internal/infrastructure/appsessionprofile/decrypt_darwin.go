//go:build darwin && !ios

package appsessionprofile

/*
#cgo CFLAGS: -mmacosx-version-min=14.0 -x objective-c
#cgo LDFLAGS: -framework Foundation -framework Security

#include <stdlib.h>
#include <string.h>
#import <Foundation/Foundation.h>
#import <Security/Security.h>

static int appSessionProfileLoadSafeStorage(
	const char *serviceValue,
	const char *accountValue,
	void **bytesOut,
	size_t *lengthOut
) {
	@autoreleasepool {
		if (bytesOut != NULL) *bytesOut = NULL;
		if (lengthOut != NULL) *lengthOut = 0;
		if (serviceValue == NULL || bytesOut == NULL || lengthOut == NULL) {
			return (int)errSecParam;
		}
		NSString *service = [NSString stringWithUTF8String:serviceValue];
		NSString *account = accountValue == NULL ? nil : [NSString stringWithUTF8String:accountValue];
		if (service.length == 0) return (int)errSecParam;
		NSMutableDictionary *query = [@{
			(id)kSecClass: (id)kSecClassGenericPassword,
			(id)kSecAttrService: service,
			(id)kSecReturnData: @YES,
			(id)kSecMatchLimit: (id)kSecMatchLimitOne,
		} mutableCopy];
		if (account.length > 0) query[(id)kSecAttrAccount] = account;
		CFTypeRef result = NULL;
		OSStatus status = SecItemCopyMatching((CFDictionaryRef)query, &result);
		[query release];
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
*/
import "C"

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/sha1"
	"errors"
	"fmt"
	"unsafe"
)

type darwinCookieDecryptor struct {
	key []byte
}

func newPlatformCookieDecryptor(ctx context.Context, definition browserDefinition, _ string) (cookieDecryptor, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var lastErr error
	for _, item := range definition.keychainItems {
		password, err := loadDarwinSafeStorage(item)
		if err != nil {
			lastErr = err
			continue
		}
		key, err := pbkdf2.Key(sha1.New, string(password), []byte("saltysalt"), 1003, 16)
		if err != nil {
			return nil, fmt.Errorf("derive browser cookie key: %w", err)
		}
		return &darwinCookieDecryptor{key: key}, nil
	}
	if lastErr == nil {
		lastErr = errors.New("browser Safe Storage item is not configured")
	}
	return nil, lastErr
}

func (decryptor *darwinCookieDecryptor) Decrypt(host string, encrypted []byte) (string, error) {
	if decryptor == nil || len(decryptor.key) != 16 {
		return "", fmt.Errorf("browser cookie key unavailable")
	}
	if len(encrypted) < 3 || (string(encrypted[:3]) != "v10" && string(encrypted[:3]) != "v11") {
		return "", fmt.Errorf("unsupported browser cookie cipher")
	}
	payload := encrypted[3:]
	if len(payload) == 0 || len(payload)%aes.BlockSize != 0 {
		return "", fmt.Errorf("invalid browser cookie ciphertext")
	}
	block, err := aes.NewCipher(decryptor.key)
	if err != nil {
		return "", err
	}
	plaintext := make([]byte, len(payload))
	cipher.NewCBCDecrypter(block, []byte("                ")).CryptBlocks(plaintext, payload)
	padding := int(plaintext[len(plaintext)-1])
	if padding <= 0 || padding > aes.BlockSize || padding > len(plaintext) {
		return "", fmt.Errorf("invalid browser cookie padding")
	}
	for _, value := range plaintext[len(plaintext)-padding:] {
		if int(value) != padding {
			return "", fmt.Errorf("invalid browser cookie padding")
		}
	}
	plaintext = stripHostDigest(host, plaintext[:len(plaintext)-padding])
	return string(plaintext), nil
}

func loadDarwinSafeStorage(item keychainItem) ([]byte, error) {
	service := C.CString(item.service)
	defer C.free(unsafe.Pointer(service))
	var account *C.char
	if item.account != "" {
		account = C.CString(item.account)
		defer C.free(unsafe.Pointer(account))
	}
	var raw unsafe.Pointer
	var length C.size_t
	status := C.appSessionProfileLoadSafeStorage(service, account, &raw, &length)
	if status == C.int(C.errSecItemNotFound) {
		return nil, fmt.Errorf("browser Safe Storage item not found")
	}
	if status != C.int(C.errSecSuccess) {
		return nil, fmt.Errorf("read browser Safe Storage item: status %d", int(status))
	}
	if raw == nil || length == 0 {
		return nil, fmt.Errorf("browser Safe Storage item is empty")
	}
	defer C.free(raw)
	return C.GoBytes(raw, C.int(length)), nil
}
