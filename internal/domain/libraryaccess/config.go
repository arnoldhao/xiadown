package libraryaccess

import (
	"context"
	"errors"
	"path"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	// DefaultLANPort is stable so Windows can keep one exact Private/
	// LocalSubnet firewall rule across launches. A legacy persisted zero is
	// normalized to this value instead of selecting a new ephemeral port and
	// prompting for elevation again on every restart.
	DefaultLANPort            = 43127
	DefaultTailscaleHTTPSPort = 443
	DefaultTailscalePath      = "/xiadown"
	// A DNS-SD service instance is one DNS label and therefore may contain at
	// most 63 UTF-8 bytes (not 63 runes or UTF-16 code units).
	MaxDeviceNameLength = 63
)

var (
	ErrConfigNotFound = errors.New("library access config not found")
	ErrInvalidConfig  = errors.New("invalid library access config")
)

// Config contains only non-secret access preferences. Pairing credentials and
// access tokens belong to the device grant store and must never be persisted
// here.
type Config struct {
	RemoteEnabled      bool
	LANEnabled         bool
	LANPort            int
	TailscaleEnabled   bool
	TailscaleHTTPSPort int
	TailscalePath      string
	DeviceName         string
}

type ConfigParams struct {
	RemoteEnabled      bool
	LANEnabled         bool
	LANPort            int
	TailscaleEnabled   bool
	TailscaleHTTPSPort int
	TailscalePath      string
	DeviceName         string
}

func NewConfig(params ConfigParams) (Config, error) {
	deviceName := strings.TrimSpace(params.DeviceName)
	tailscalePath := strings.TrimSpace(params.TailscalePath)
	if params.LANPort < 0 || params.LANPort > 65535 ||
		params.TailscaleHTTPSPort < 1 || params.TailscaleHTTPSPort > 65535 ||
		!validTailscalePath(tailscalePath) ||
		deviceName == "" || !utf8.ValidString(deviceName) || len([]byte(deviceName)) > MaxDeviceNameLength {
		return Config{}, ErrInvalidConfig
	}
	lanPort := params.LANPort
	if lanPort == 0 {
		lanPort = DefaultLANPort
	}
	return Config{
		RemoteEnabled: params.RemoteEnabled, LANEnabled: params.LANEnabled,
		LANPort: lanPort, TailscaleEnabled: params.TailscaleEnabled,
		TailscaleHTTPSPort: params.TailscaleHTTPSPort,
		TailscalePath:      tailscalePath, DeviceName: deviceName,
	}, nil
}

func DefaultConfig(deviceName string) (Config, error) {
	return NewConfig(ConfigParams{
		LANEnabled: true, TailscaleHTTPSPort: DefaultTailscaleHTTPSPort,
		TailscalePath: DefaultTailscalePath, DeviceName: deviceName,
	})
}

func validTailscalePath(value string) bool {
	if value == "" || value == "/" || !strings.HasPrefix(value, "/") ||
		strings.ContainsAny(value, "?#\\") || path.Clean(value) != value {
		return false
	}
	for _, character := range value {
		if unicode.IsLetter(character) || unicode.IsDigit(character) ||
			strings.ContainsRune("/-._~", character) {
			continue
		}
		return false
	}
	return true
}

type Repository interface {
	Get(context.Context) (Config, error)
	Save(context.Context, Config) error
	GetManagedTailscaleRoute(context.Context) (ManagedTailscaleRoute, error)
	TransitionManagedTailscaleRoute(context.Context, TailscaleRouteTransition) (ManagedTailscaleRoute, error)
}
