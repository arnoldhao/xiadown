//go:build !darwin || ios

package browserprofile

func safariSource() (Source, bool) {
	return Source{}, false
}

func listSafariProfiles() []Profile {
	return []Profile{}
}
