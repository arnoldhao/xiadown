//go:build darwin && cgo && !ios

#ifndef XIADOWN_PERMISSION_GUIDE_DARWIN_H
#define XIADOWN_PERMISSION_GUIDE_DARWIN_H

int xiadownOpenPermissionGuide(
	const char *settings_url,
	const char *permission_name,
	const char *hint
);

#endif
