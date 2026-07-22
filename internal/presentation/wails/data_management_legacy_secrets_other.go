//go:build (!darwin && !windows) || ios

package wails

func legacyAppSessionSecretInventory() (int, int64, error) { return 0, 0, nil }
func clearLegacyAppSessionSecrets() error                  { return nil }
