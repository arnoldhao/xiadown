//go:build windows

package appsessionprofile

import (
	"os"
	"path/filepath"
	"strings"

	"xiadown/internal/domain/appsessions"
)

func platformBrowserDefinition(browserID string) (browserDefinition, error) {
	local := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
	roaming := strings.TrimSpace(os.Getenv("APPDATA"))
	if local == "" {
		var err error
		local, err = os.UserConfigDir()
		if err != nil {
			return browserDefinition{}, err
		}
	}
	definition := browserDefinition{id: browserID}
	switch browserID {
	case "chrome":
		definition.root = filepath.Join(local, "Google", "Chrome", "User Data")
	case "edge":
		definition.root = filepath.Join(local, "Microsoft", "Edge", "User Data")
	case "brave":
		definition.root = filepath.Join(local, "BraveSoftware", "Brave-Browser", "User Data")
	case "arc":
		matches, _ := filepath.Glob(filepath.Join(local, "Packages", "TheBrowserCompany.Arc_*", "LocalCache", "Local", "Arc", "User Data"))
		if len(matches) > 0 {
			definition.root = matches[0]
		} else {
			definition.root = filepath.Join(local, "Arc", "User Data")
		}
	case "vivaldi":
		definition.root = filepath.Join(local, "Vivaldi", "User Data")
	case "opera":
		if roaming == "" {
			roaming = local
		}
		definition.root = filepath.Join(roaming, "Opera Software", "Opera Stable")
	case "safari":
		return browserDefinition{}, appsessions.ErrUnsupported
	default:
		return browserDefinition{}, appsessions.ErrUnsupported
	}
	definition.localState = filepath.Join(definition.root, "Local State")
	return definition, nil
}
