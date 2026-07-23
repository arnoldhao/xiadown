#!/bin/sh

# Keep the Launch Services-owned development app tied to the lifetime of the
# Wails dev runner. Launch Services reparents the app to launchd, so killing
# `open -W` alone does not stop the app. A later rebuild would otherwise
# overwrite the executable underneath the live process and invalidate the
# code identity TCC uses for privacy decisions.

set -u

mode=${1:-}
bundle_input=${2:-}
executable_name=${3:-}

if [ -z "$mode" ] || [ -z "$bundle_input" ] || [ -z "$executable_name" ]; then
	echo "usage: $0 <stop|run> <app-bundle> <executable-name>" >&2
	exit 2
fi

bundle_parent=$(CDPATH= cd -- "$(dirname -- "$bundle_input")" && pwd)
bundle_path="$bundle_parent/$(basename -- "$bundle_input")"
absolute_executable="$bundle_path/Contents/MacOS/$executable_name"
input_executable="$bundle_input/Contents/MacOS/$executable_name"

matching_pids() {
	/bin/ps -axww -o pid=,command= | /usr/bin/awk \
		-v absolute_target="$absolute_executable" \
		-v input_target="$input_executable" '
		{
			pid = $1
			sub(/^[[:space:]]*[0-9]+[[:space:]]+/, "", $0)
			if ($0 == absolute_target || $0 == input_target) {
				print pid
			}
		}
	' | /usr/bin/sort -nu
}

stop_app() {
	pids=$(matching_pids)
	if [ -z "$pids" ]; then
		return 0
	fi

	# TERM lets Wails and the native browser surfaces close cleanly. Bound the
	# wait so a stuck dev process can never block a rebuild indefinitely.
	kill -TERM $pids 2>/dev/null || true
	attempt=0
	while [ "$attempt" -lt 40 ]; do
		remaining=$(matching_pids)
		if [ -z "$remaining" ]; then
			return 0
		fi
		attempt=$((attempt + 1))
		sleep 0.1
	done

	remaining=$(matching_pids)
	if [ -n "$remaining" ]; then
		kill -KILL $remaining 2>/dev/null || true
	fi
}

case "$mode" in
	stop)
		stop_app
		;;
	run)
		cleanup() {
			trap - EXIT HUP INT TERM
			if [ -n "${open_pid:-}" ]; then
				kill -TERM "$open_pid" 2>/dev/null || true
			fi
			stop_app
		}
		trap cleanup EXIT HUP INT TERM

		# `open` is essential here: direct Mach-O execution makes the terminal or
		# task runner the TCC responsible process instead of XiaDown.
		runner_pid=$PPID
		supervisor_pid=$(/bin/ps -p "$runner_pid" -o ppid= 2>/dev/null | /usr/bin/tr -d ' ')
		/usr/bin/open -n -W "$bundle_path" &
		open_pid=$!
		status=0

		# Wails may be terminated by a watcher or an outer dev runner without
		# forwarding a signal to Launch Services. Notice that loss explicitly so
		# the launchd-owned app cannot outlive its dev supervisor.
		while kill -0 "$open_pid" 2>/dev/null; do
			current_supervisor=$(/bin/ps -p "$runner_pid" -o ppid= 2>/dev/null | /usr/bin/tr -d ' ')
			if [ -z "$supervisor_pid" ] || [ "$current_supervisor" != "$supervisor_pid" ] || ! kill -0 "$supervisor_pid" 2>/dev/null; then
				status=143
				break
			fi
			sleep 0.25
		done

		if kill -0 "$open_pid" 2>/dev/null; then
			kill -TERM "$open_pid" 2>/dev/null || true
		fi
		wait "$open_pid" 2>/dev/null || {
			open_status=$?
			if [ "$status" -eq 0 ]; then
				status=$open_status
			fi
		}
		cleanup
		exit "$status"
		;;
	*)
		echo "unknown mode: $mode" >&2
		exit 2
		;;
esac
