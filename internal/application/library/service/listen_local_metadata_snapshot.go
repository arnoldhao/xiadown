package service

import (
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"os"

	"xiadown/internal/domain/library"
)

type listenLocalFileSnapshot struct {
	info   os.FileInfo
	digest [sha256.Size]byte
}

func snapshotListenLocalFile(ctx context.Context, path string) (listenLocalFileSnapshot, error) {
	pathInfo, err := os.Lstat(path)
	if err != nil {
		return listenLocalFileSnapshot{}, err
	}
	if pathInfo.Mode()&os.ModeSymlink != 0 || !pathInfo.Mode().IsRegular() {
		return listenLocalFileSnapshot{}, listenLocalMetadataPreservationError("a non-regular local file")
	}
	file, err := os.Open(path)
	if err != nil {
		return listenLocalFileSnapshot{}, err
	}
	defer file.Close()

	before, err := file.Stat()
	if err != nil {
		return listenLocalFileSnapshot{}, err
	}
	if !before.Mode().IsRegular() || !os.SameFile(pathInfo, before) {
		return listenLocalFileSnapshot{}, library.ErrListenLocalFileChanged
	}

	hasher := sha256.New()
	buffer := make([]byte, 1024*1024)
	for {
		if err := ctx.Err(); err != nil {
			return listenLocalFileSnapshot{}, err
		}
		read, readErr := file.Read(buffer)
		if read > 0 {
			_, _ = hasher.Write(buffer[:read])
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return listenLocalFileSnapshot{}, readErr
		}
	}
	after, err := file.Stat()
	if err != nil {
		return listenLocalFileSnapshot{}, err
	}
	if !sameListenLocalFileInfo(before, after) {
		return listenLocalFileSnapshot{}, library.ErrListenLocalFileChanged
	}

	result := listenLocalFileSnapshot{info: after}
	copy(result.digest[:], hasher.Sum(nil))
	return result, nil
}

func sameListenLocalFileSnapshot(left listenLocalFileSnapshot, right listenLocalFileSnapshot) bool {
	return sameListenLocalFileInfo(left.info, right.info) && left.digest == right.digest
}

func sameListenLocalFileInfo(left os.FileInfo, right os.FileInfo) bool {
	return left != nil && right != nil &&
		os.SameFile(left, right) &&
		left.Size() == right.Size() &&
		left.ModTime().Equal(right.ModTime()) &&
		left.Mode() == right.Mode() &&
		listenLocalFileChangeToken(left) == listenLocalFileChangeToken(right)
}
