//go:build !darwin && !windows

package discovery

import (
	"context"
	"fmt"
)

type unsupportedAdvertiser struct{}

func newPlatformAdvertiser() Advertiser { return unsupportedAdvertiser{} }

func (unsupportedAdvertiser) Register(context.Context, Advertisement) (Registration, error) {
	return nil, fmt.Errorf("%w on this platform", ErrUnavailable)
}
