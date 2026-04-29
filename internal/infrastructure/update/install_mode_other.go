//go:build !windows

package update

func detectWindowsInstallKind(_ string) installKind {
	return installKindUnknown
}
