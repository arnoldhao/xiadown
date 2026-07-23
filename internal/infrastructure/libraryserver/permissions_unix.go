//go:build !windows

package libraryserver

import "os"

func restrictTLSFile(path string, mode os.FileMode) error {
	return os.Chmod(path, mode)
}

func restrictTLSPrivateKey(path string) error {
	return os.Chmod(path, 0o600)
}
