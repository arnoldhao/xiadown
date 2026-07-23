package service

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const (
	// Local media helpers must never turn a library path or a local manifest into
	// an unmanaged network client. Keep the allow-list deliberately small: these
	// protocols cover ordinary files, local encrypted media, inline data, and the
	// progress pipe used by FFmpeg without permitting any socket transport.
	localMediaProtocolWhitelist = "file,pipe,crypto,data"
	localMediaProtocolBlacklist = "http,https,tcp,tls,udp,rtp,rtsp,rtmp,rtmps,srt,ftp,ftps,sftp,ssh,gopher,gophers,zmq,unix"
	localMediaReadWriteTimeout  = 15 * time.Second
	localMediaProbeTimeout      = 30 * time.Second
)

func localMediaInputPolicyArgs() []string {
	return []string{
		"-protocol_whitelist", localMediaProtocolWhitelist,
		"-protocol_blacklist", localMediaProtocolBlacklist,
		"-rw_timeout", strconv.FormatInt(localMediaReadWriteTimeout.Microseconds(), 10),
	}
}

func appendLocalMediaFFmpegInput(args []string, path string) []string {
	args = append(args, localMediaInputPolicyArgs()...)
	return append(args, "-i", strings.TrimSpace(path))
}

func appendLocalMediaFFprobeInput(args []string, path string) []string {
	args = append(args, localMediaInputPolicyArgs()...)
	return append(args, strings.TrimSpace(path))
}

func withLocalMediaProbeTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(ctx, localMediaProbeTimeout)
}

func configureLocalMediaToolCommand(command *exec.Cmd) {
	if command == nil {
		return
	}
	command.Env = localMediaToolEnvironment(os.Environ())
	configureProcessGroup(command)
}

func localMediaToolEnvironment(environment []string) []string {
	result := make([]string, 0, len(environment))
	for _, item := range environment {
		key := item
		if index := strings.IndexByte(item, '='); index >= 0 {
			key = item[:index]
		}
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "http_proxy", "https_proxy", "all_proxy", "ftp_proxy", "ftps_proxy", "socks_proxy", "no_proxy", "rsync_proxy":
			continue
		default:
			result = append(result, item)
		}
	}
	return result
}
