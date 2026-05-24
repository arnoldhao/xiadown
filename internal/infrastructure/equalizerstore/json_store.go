package equalizerstore

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"xiadown/internal/application/equalizer"
)

type JSONStore struct {
	path string
}

func NewJSONStore(path string) *JSONStore {
	return &JSONStore{path: path}
}

func DefaultJSONStore() (*JSONStore, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	return NewJSONStore(filepath.Join(configDir, "xiadown", "equalizer.json")), nil
}

func (store *JSONStore) Load() (equalizer.Settings, bool, error) {
	if store == nil || store.path == "" {
		return equalizer.FlatSettings(), false, nil
	}
	data, err := os.ReadFile(store.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return equalizer.FlatSettings(), false, nil
		}
		return equalizer.FlatSettings(), false, err
	}
	var settings equalizer.Settings
	if err := json.Unmarshal(data, &settings); err != nil {
		return equalizer.FlatSettings(), false, err
	}
	return equalizer.ClampSettings(settings), true, nil
}

func (store *JSONStore) Save(settings equalizer.Settings) error {
	if store == nil || store.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(store.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(equalizer.ClampSettings(settings), "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(store.path, append(data, '\n'), 0o644)
}
