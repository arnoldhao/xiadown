//go:build !windows

package libraryapi

import "os"

func securePublicMusicResourceDirectory(path string) error {
	return os.Chmod(path, 0o700)
}

func securePublicMusicResourceTemporaryFile(path string) error {
	return os.Chmod(path, 0o600)
}

func securePublicMusicResourceVerifiedBlob(path string) error {
	return os.Chmod(path, 0o400)
}

func publicMusicResourceBlobIsProtected(_ string, info os.FileInfo) bool {
	return info != nil && info.Mode().Perm()&0o222 == 0
}
