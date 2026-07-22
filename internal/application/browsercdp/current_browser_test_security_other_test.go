//go:build !windows

package browsercdp

func secureTrustedCurrentBrowserTestPath(_ string) error {
	return nil
}
