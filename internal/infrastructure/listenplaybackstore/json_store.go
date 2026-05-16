package listenplaybackstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"xiadown/internal/application/listenplayback"
)

type JSONSessionStore struct {
	path string
}

func NewJSONSessionStore(path string) *JSONSessionStore {
	return &JSONSessionStore{path: strings.TrimSpace(path)}
}

func DefaultJSONSessionStore() (*JSONSessionStore, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	return NewJSONSessionStore(filepath.Join(configDir, "xiadown", "listen-playback-session.json")), nil
}

func (store *JSONSessionStore) SavePlaybackSession(_ context.Context, session listenplayback.RestoredPlaybackSession) error {
	if store == nil || strings.TrimSpace(store.path) == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(store.path), 0o755); err != nil {
		return fmt.Errorf("create playback session directory: %w", err)
	}
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("encode playback session: %w", err)
	}
	tempPath := store.path + ".tmp"
	if err := os.WriteFile(tempPath, data, 0o600); err != nil {
		return fmt.Errorf("write playback session: %w", err)
	}
	if err := os.Rename(tempPath, store.path); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("replace playback session: %w", err)
	}
	return nil
}

func (store *JSONSessionStore) LoadPlaybackSession(_ context.Context) (listenplayback.RestoredPlaybackSession, bool, error) {
	if store == nil || strings.TrimSpace(store.path) == "" {
		return listenplayback.RestoredPlaybackSession{}, false, nil
	}
	data, err := os.ReadFile(store.path)
	if errors.Is(err, os.ErrNotExist) {
		return listenplayback.RestoredPlaybackSession{}, false, nil
	}
	if err != nil {
		return listenplayback.RestoredPlaybackSession{}, false, fmt.Errorf("read playback session: %w", err)
	}
	var session listenplayback.RestoredPlaybackSession
	if err := json.Unmarshal(data, &session); err != nil {
		return listenplayback.RestoredPlaybackSession{}, false, fmt.Errorf("decode playback session: %w", err)
	}
	return session, len(session.Queue) > 0, nil
}

func (store *JSONSessionStore) ClearPlaybackSession(_ context.Context) error {
	if store == nil || strings.TrimSpace(store.path) == "" {
		return nil
	}
	if err := os.Remove(store.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("clear playback session: %w", err)
	}
	return nil
}
