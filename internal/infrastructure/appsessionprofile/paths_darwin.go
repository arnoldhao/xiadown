//go:build darwin && !ios

package appsessionprofile

import (
	"os"
	"path/filepath"

	"xiadown/internal/domain/appsessions"
)

func platformBrowserDefinition(browserID string) (browserDefinition, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return browserDefinition{}, err
	}
	applicationSupport := filepath.Join(home, "Library", "Application Support")
	definition := browserDefinition{id: browserID}
	switch browserID {
	case "chrome":
		definition.root = filepath.Join(applicationSupport, "Google", "Chrome")
		definition.keychainItems = []keychainItem{{service: "Chrome Safe Storage", account: "Chrome"}}
	case "edge":
		definition.root = filepath.Join(applicationSupport, "Microsoft Edge")
		definition.keychainItems = []keychainItem{{service: "Microsoft Edge Safe Storage", account: "Microsoft Edge"}}
	case "brave":
		definition.root = filepath.Join(applicationSupport, "BraveSoftware", "Brave-Browser")
		definition.keychainItems = []keychainItem{{service: "Brave Safe Storage", account: "Brave"}}
	case "arc":
		definition.root = filepath.Join(applicationSupport, "Arc", "User Data")
		definition.keychainItems = []keychainItem{{service: "Arc Safe Storage", account: "Arc"}}
	case "vivaldi":
		definition.root = filepath.Join(applicationSupport, "Vivaldi")
		definition.keychainItems = []keychainItem{{service: "Vivaldi Safe Storage", account: "Vivaldi"}}
	case "opera":
		definition.root = filepath.Join(applicationSupport, "com.operasoftware.Opera")
		definition.keychainItems = []keychainItem{{service: "Opera Safe Storage", account: "Opera"}}
	case "safari":
		return browserDefinition{}, appsessions.ErrUnsupported
	default:
		return browserDefinition{}, appsessions.ErrUnsupported
	}
	definition.localState = filepath.Join(definition.root, "Local State")
	return definition, nil
}
