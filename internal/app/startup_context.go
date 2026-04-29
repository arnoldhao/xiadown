package app

import "strings"

const autoStartLaunchArgument = "--autostart"
const skipPreparedUpdateLaunchArgument = "--skip-prepared-update-once"

type startupContext struct {
	launchedByAutoStart bool
	skipPreparedUpdate  bool
}

func currentStartupContext(args []string) startupContext {
	context := startupContext{}
	for _, arg := range args {
		switch strings.ToLower(strings.TrimSpace(arg)) {
		case autoStartLaunchArgument:
			context.launchedByAutoStart = true
		case skipPreparedUpdateLaunchArgument:
			context.skipPreparedUpdate = true
		}
	}
	return context
}
