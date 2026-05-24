package browsercdp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"
)

type runtimeProcessRecord struct {
	ID             string    `json:"id"`
	PID            int       `json:"pid"`
	ProcessGroupID int       `json:"processGroupId,omitempty"`
	ExecutablePath string    `json:"executablePath,omitempty"`
	UserDataDir    string    `json:"userDataDir,omitempty"`
	CDPURL         string    `json:"cdpUrl,omitempty"`
	CDPPort        int       `json:"cdpPort,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
}

type RuntimeProcessInfo struct {
	ID             string    `json:"id"`
	PID            int       `json:"pid"`
	ProcessGroupID int       `json:"processGroupId,omitempty"`
	ExecutablePath string    `json:"executablePath,omitempty"`
	UserDataDir    string    `json:"userDataDir,omitempty"`
	CDPURL         string    `json:"cdpUrl,omitempty"`
	CDPPort        int       `json:"cdpPort,omitempty"`
	Ready          bool      `json:"ready"`
	CreatedAt      time.Time `json:"createdAt"`
}

var runtimeRegistryDir = defaultRuntimeRegistryDir

func ListRuntimeProcesses(ctx context.Context) ([]RuntimeProcessInfo, error) {
	dir := runtimeRegistryDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	result := make([]RuntimeProcessInfo, 0, len(entries))
	for _, entry := range entries {
		if ctx != nil {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		record, readErr := readRuntimeProcessRecord(path)
		if readErr != nil {
			_ = os.Remove(path)
			continue
		}
		info := runtimeProcessInfoFromRecord(record)
		if info.CDPPort > 0 {
			checkCtx := ctx
			if checkCtx == nil {
				checkCtx = context.Background()
			}
			readyCtx, cancel := context.WithTimeout(checkCtx, 250*time.Millisecond)
			info.Ready = CheckCDPReady(readyCtx, "127.0.0.1", info.CDPPort) == nil
			cancel()
		}
		result = append(result, info)
	}
	return result, nil
}

func StopRuntimeProcess(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	path := runtimeRecordPath(runtimeRegistryDir(), id)
	record, err := readRuntimeProcessRecord(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if err := terminateRuntimeProcessGroup(record.PID, record.ProcessGroupID); err != nil {
		return err
	}
	unregisterRuntimeProcess(id)
	return nil
}

func runtimeProcessInfoFromRecord(record runtimeProcessRecord) RuntimeProcessInfo {
	return RuntimeProcessInfo{
		ID:             strings.TrimSpace(record.ID),
		PID:            record.PID,
		ProcessGroupID: record.ProcessGroupID,
		ExecutablePath: strings.TrimSpace(record.ExecutablePath),
		UserDataDir:    strings.TrimSpace(record.UserDataDir),
		CDPURL:         strings.TrimSpace(record.CDPURL),
		CDPPort:        record.CDPPort,
		CreatedAt:      record.CreatedAt,
	}
}

func CleanupStaleRuntimes(ctx context.Context) error {
	dir := runtimeRegistryDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	var firstErr error
	for _, entry := range entries {
		if ctx != nil {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		record, readErr := readRuntimeProcessRecord(path)
		if readErr != nil {
			_ = os.Remove(path)
			if firstErr == nil {
				firstErr = readErr
			}
			continue
		}
		if record.PID > 0 {
			if killErr := terminateRuntimeProcessGroup(record.PID, record.ProcessGroupID); killErr != nil && firstErr == nil {
				firstErr = killErr
			}
			zap.L().Debug(
				"cleaned stale browser runtime process",
				zap.Int("pid", record.PID),
				zap.Int("processGroupID", record.ProcessGroupID),
				zap.String("execPath", record.ExecutablePath),
			)
		}
		_ = os.Remove(path)
	}
	return firstErr
}

func registerRuntimeProcess(record runtimeProcessRecord) (string, error) {
	if record.PID <= 0 {
		return "", nil
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now()
	}
	record.ID = runtimeRecordID(record.PID, record.CDPPort)
	dir := runtimeRegistryDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	_ = os.Chmod(dir, 0o700)
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return "", err
	}
	path := runtimeRecordPath(dir, record.ID)
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o600); err != nil {
		return "", err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}
	return record.ID, nil
}

func unregisterRuntimeProcess(id string) {
	id = strings.TrimSpace(id)
	if id == "" {
		return
	}
	_ = os.Remove(runtimeRecordPath(runtimeRegistryDir(), id))
}

func readRuntimeProcessRecord(path string) (runtimeProcessRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return runtimeProcessRecord{}, err
	}
	var record runtimeProcessRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return runtimeProcessRecord{}, err
	}
	return record, nil
}

func runtimeRecordID(pid int, port int) string {
	return fmt.Sprintf("%d-%d", pid, port)
}

func runtimeRecordPath(dir string, id string) string {
	return filepath.Join(dir, strings.TrimSpace(id)+".json")
}

func defaultRuntimeRegistryDir() string {
	cacheDir, err := os.UserCacheDir()
	if err == nil && strings.TrimSpace(cacheDir) != "" {
		return filepath.Join(cacheDir, "xiadown", "browsercdp", "runtimes")
	}
	return filepath.Join(os.TempDir(), "xiadown", "browsercdp", "runtimes")
}
