//go:build windows

package appsessionprofile

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

type windowsCookieDecryptor struct {
	key []byte
}

func newPlatformCookieDecryptor(ctx context.Context, definition browserDefinition, _ string) (cookieDecryptor, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(definition.localState)
	if err != nil {
		return nil, fmt.Errorf("read browser Local State: %w", err)
	}
	if len(data) > 8<<20 {
		return nil, fmt.Errorf("browser Local State exceeds size limit")
	}
	var state struct {
		OSCrypt struct {
			EncryptedKey string `json:"encrypted_key"`
		} `json:"os_crypt"`
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("decode browser Local State: %w", err)
	}
	protected, err := base64.StdEncoding.DecodeString(strings.TrimSpace(state.OSCrypt.EncryptedKey))
	if err != nil {
		return nil, fmt.Errorf("decode browser cookie key: %w", err)
	}
	if !strings.HasPrefix(string(protected), "DPAPI") {
		return nil, fmt.Errorf("unsupported browser cookie key format")
	}
	key, err := windowsUnprotect(protected[len("DPAPI"):])
	if err != nil {
		return nil, fmt.Errorf("unprotect browser cookie key: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("invalid browser cookie key length")
	}
	return &windowsCookieDecryptor{key: key}, nil
}

func (decryptor *windowsCookieDecryptor) Decrypt(host string, encrypted []byte) (string, error) {
	if decryptor == nil || len(decryptor.key) != 32 {
		return "", fmt.Errorf("browser cookie key unavailable")
	}
	if len(encrypted) >= 3 && string(encrypted[:3]) == "v20" {
		return "", fmt.Errorf("browser uses app-bound cookie encryption")
	}
	if len(encrypted) >= 3 && (string(encrypted[:3]) == "v10" || string(encrypted[:3]) == "v11") {
		payload := encrypted[3:]
		if len(payload) < 12+16 {
			return "", fmt.Errorf("invalid browser cookie ciphertext")
		}
		block, err := aes.NewCipher(decryptor.key)
		if err != nil {
			return "", err
		}
		gcm, err := cipher.NewGCM(block)
		if err != nil {
			return "", err
		}
		plaintext, err := gcm.Open(nil, payload[:12], payload[12:], nil)
		if err != nil {
			return "", fmt.Errorf("authenticate browser cookie: %w", err)
		}
		return string(stripHostDigest(host, plaintext)), nil
	}
	plaintext, err := windowsUnprotect(encrypted)
	if err != nil {
		return "", fmt.Errorf("unprotect legacy browser cookie: %w", err)
	}
	return string(stripHostDigest(host, plaintext)), nil
}

func windowsUnprotect(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty DPAPI payload")
	}
	in := windows.DataBlob{Size: uint32(len(data)), Data: &data[0]}
	var out windows.DataBlob
	if err := windows.CryptUnprotectData(
		&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out,
	); err != nil {
		return nil, err
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	result := make([]byte, int(out.Size))
	copy(result, unsafe.Slice(out.Data, int(out.Size)))
	return result, nil
}
