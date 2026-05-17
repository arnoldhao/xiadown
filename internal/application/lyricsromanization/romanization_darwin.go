//go:build darwin && cgo

package lyricsromanization

/*
#cgo darwin CFLAGS: -x objective-c
#cgo darwin LDFLAGS: -framework CoreFoundation -framework Foundation -framework NaturalLanguage

#import <CoreFoundation/CoreFoundation.h>
#import <Foundation/Foundation.h>
#import <NaturalLanguage/NaturalLanguage.h>
#include <stdlib.h>
#include <string.h>

static int xiadown_is_latin_only_token(NSString *token) {
    for (NSUInteger index = 0; index < [token length]; index++) {
        unichar ch = [token characterAtIndex:index];
        if (ch <= 0x024F) {
            continue;
        }
        if (ch >= 0x2000 && ch <= 0x206F) {
            continue;
        }
        if (ch >= 0x20A0 && ch <= 0x20CF) {
            continue;
        }
        if (ch >= 0xFE00 && ch <= 0xFE0F) {
            continue;
        }
        if (ch >= 0xFFF0 && ch <= 0xFFFF) {
            continue;
        }
        return 0;
    }
    return 1;
}

static void xiadown_append_token(NSMutableString *result, NSString *piece) {
    if (piece == nil || [piece length] == 0) {
        return;
    }
    if ([result length] > 0) {
        [result appendString:@" "];
    }
    [result appendString:piece];
}

int xiadown_romanization_available() {
    @autoreleasepool {
        if (@available(macOS 10.14, *)) {
            return 1;
        }
        return 0;
    }
}

char* xiadown_dominant_language(const char *input) {
    @autoreleasepool {
        if (input == NULL) {
            return NULL;
        }
        NSString *text = [NSString stringWithUTF8String:input];
        if (text == nil || [text length] == 0) {
            return NULL;
        }
        if (@available(macOS 10.14, *)) {
            NLLanguageRecognizer *recognizer = [[NLLanguageRecognizer alloc] init];
            [recognizer processString:text];
            NLLanguage language = [recognizer dominantLanguage];
            if (language != nil) {
                return strdup([language UTF8String]);
            }
        }
        return NULL;
    }
}

char* xiadown_romanize_with_locale(const char *input, const char *localeIdentifier) {
    @autoreleasepool {
        if (input == NULL || localeIdentifier == NULL) {
            return NULL;
        }
        NSString *text = [NSString stringWithUTF8String:input];
        NSString *localeText = [NSString stringWithUTF8String:localeIdentifier];
        if (text == nil || localeText == nil || [text length] == 0) {
            return NULL;
        }

        CFLocaleRef locale = CFLocaleCreate(kCFAllocatorDefault, (__bridge CFStringRef)localeText);
        if (locale == NULL) {
            return NULL;
        }
        CFStringTokenizerRef tokenizer = CFStringTokenizerCreate(
            kCFAllocatorDefault,
            (__bridge CFStringRef)text,
            CFRangeMake(0, [text length]),
            kCFStringTokenizerUnitWord,
            locale
        );
        CFRelease(locale);
        if (tokenizer == NULL) {
            return NULL;
        }

        NSMutableString *result = [NSMutableString string];
        CFStringTokenizerTokenType tokenType = CFStringTokenizerAdvanceToNextToken(tokenizer);
        while (tokenType != kCFStringTokenizerTokenNone) {
            CFRange tokenRange = CFStringTokenizerGetCurrentTokenRange(tokenizer);
            if (tokenRange.location >= 0 && tokenRange.length > 0 && tokenRange.location + tokenRange.length <= [text length]) {
                NSString *token = [text substringWithRange:NSMakeRange(tokenRange.location, tokenRange.length)];
                if (xiadown_is_latin_only_token(token)) {
                    xiadown_append_token(result, token);
                } else {
                    CFTypeRef attr = CFStringTokenizerCopyCurrentTokenAttribute(
                        tokenizer,
                        kCFStringTokenizerAttributeLatinTranscription
                    );
                    if (attr != NULL) {
                        xiadown_append_token(result, (NSString *)attr);
                        CFRelease(attr);
                    } else {
                        xiadown_append_token(result, token);
                    }
                }
            }
            tokenType = CFStringTokenizerAdvanceToNextToken(tokenizer);
        }
        CFRelease(tokenizer);

        if ([result length] == 0) {
            return NULL;
        }
        return strdup([result UTF8String]);
    }
}

*/
import "C"

import "unsafe"

func systemRomanizationAvailable() bool {
	return C.xiadown_romanization_available() != 0
}

func romanizeWithLocale(text string, locale string) string {
	cText := C.CString(text)
	defer C.free(unsafe.Pointer(cText))
	cLocale := C.CString(locale)
	defer C.free(unsafe.Pointer(cLocale))
	result := C.xiadown_romanize_with_locale(cText, cLocale)
	if result == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(result))
	return C.GoString(result)
}

func dominantLanguage(text string) string {
	cText := C.CString(text)
	defer C.free(unsafe.Pointer(cText))
	result := C.xiadown_dominant_language(cText)
	if result == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(result))
	return C.GoString(result)
}
