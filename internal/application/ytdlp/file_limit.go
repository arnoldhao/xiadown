package ytdlp

const minYTDLPFileLimit uint64 = 4096

func ytdlpFileLimitTarget(current uint64, maximum uint64, minimum uint64) uint64 {
	if minimum == 0 || current >= minimum {
		return current
	}
	if maximum > 0 && maximum < minimum {
		if maximum > current {
			return maximum
		}
		return current
	}
	return minimum
}
