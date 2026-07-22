package libraryapi

import "unicode/utf8"

// JSON Schema maxLength counts Unicode code points, not UTF-8 bytes. Validate
// the decoded query without normalization or truncation so every endpoint that
// references the shared OpenAPI SearchQuery parameter has identical semantics.
const maxPublicSearchQueryLength = 512

func validPublicSearchQuery(value string) bool {
	// A valid UTF-8 string with at most N code points cannot exceed N*UTFMax
	// bytes. This equivalent fast-fail keeps oversized input work bounded while
	// the public contract remains code-point based.
	if len(value) > maxPublicSearchQueryLength*utf8.UTFMax {
		return false
	}
	return utf8.ValidString(value) && utf8.RuneCountInString(value) <= maxPublicSearchQueryLength
}
