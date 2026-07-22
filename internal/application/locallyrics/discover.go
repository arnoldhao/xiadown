package locallyrics

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type sidecarSpec struct {
	mainName         string
	translationNames []string
}

// DiscoverSidecars parses every supported sidecar beside mediaPath and sorts
// candidates by actual lyric capability. It never searches outside the media
// directory and never derives a path from lyric file contents.
func DiscoverSidecars(ctx context.Context, mediaPath string, options Options) ([]Candidate, error) {
	options = normalizeOptions(options)
	if strings.TrimSpace(mediaPath) == "" || strings.ContainsRune(mediaPath, '\x00') {
		return nil, ErrInvalidPath
	}
	mediaAbsolute, err := filepath.Abs(mediaPath)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidPath, err)
	}
	mediaInfo, err := os.Stat(mediaAbsolute)
	if err != nil {
		return nil, err
	}
	if !mediaInfo.Mode().IsRegular() {
		return nil, ErrInvalidPath
	}

	root := filepath.Dir(mediaAbsolute)
	specs := candidateFileNames(mediaPath)
	candidates := make([]Candidate, 0, len(specs))
	seenMainPaths := make(map[string]struct{}, len(specs))
	var firstRejectedError error
	for _, spec := range specs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		mainPath := filepath.Join(root, spec.mainName)
		mainContent, resolvedMainPath, err := secureReadFile(root, mainPath, options)
		if err != nil {
			if isMissingFile(err) {
				continue
			}
			if firstRejectedError == nil && (errors.Is(err, ErrTooLarge) || errors.Is(err, ErrUnsafeFile) || errors.Is(err, ErrPathEscape)) {
				firstRejectedError = err
			}
			continue
		}
		resolvedKey := strings.ToLower(filepath.Clean(resolvedMainPath))
		if _, exists := seenMainPaths[resolvedKey]; exists {
			continue
		}
		seenMainPaths[resolvedKey] = struct{}{}

		mainResult, err := ParseContent(mainContent, formatFromName(spec.mainName), options)
		if err != nil {
			if firstRejectedError == nil {
				firstRejectedError = err
			}
			continue
		}
		mainResult.SourcePath = resolvedMainPath
		mainResult.Attribution = Attribution{Kind: SourceSidecar, Label: "Local sidecar"}

		translationResult, resolvedTranslationPath := bestTranslationSidecar(ctx, root, spec.translationNames, options)
		if resolvedTranslationPath != "" {
			mergeTranslation(&mainResult, translationResult, options.TranslationTolerance)
			mainResult.TranslationSourcePath = resolvedTranslationPath
		}

		candidate := Candidate{
			Path:            resolvedMainPath,
			TranslationPath: resolvedTranslationPath,
			Result:          mainResult,
		}
		candidate.Score = scoreCandidate(candidate)
		candidates = append(candidates, candidate)
	}

	if len(candidates) == 0 && firstRejectedError != nil {
		return nil, firstRejectedError
	}
	sort.SliceStable(candidates, func(left, right int) bool {
		if candidates[left].Score != candidates[right].Score {
			return candidates[left].Score > candidates[right].Score
		}
		return strings.ToLower(candidates[left].Path) < strings.ToLower(candidates[right].Path)
	})
	return candidates, nil
}

// LoadBestSidecar returns the highest-capability local candidate.
func LoadBestSidecar(ctx context.Context, mediaPath string, options Options) (Result, error) {
	candidates, err := DiscoverSidecars(ctx, mediaPath, options)
	if err != nil {
		return Result{}, err
	}
	if len(candidates) == 0 {
		return Result{}, ErrNoLyrics
	}
	return candidates[0].Result, nil
}

func bestTranslationSidecar(ctx context.Context, root string, names []string, options Options) (Result, string) {
	bestResult := Result{}
	bestPath := ""
	bestScore := -1
	seen := make(map[string]struct{})
	for _, name := range names {
		if ctx.Err() != nil {
			break
		}
		key := strings.ToLower(name)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		content, resolvedPath, err := secureReadFile(root, filepath.Join(root, name), options)
		if err != nil {
			continue
		}
		result, err := ParseContent(content, formatFromName(name), options)
		if err != nil || len(result.Lines) == 0 {
			continue
		}
		score := scoreResult(result)
		if score > bestScore {
			bestScore = score
			bestResult = result
			bestPath = resolvedPath
		}
	}
	return bestResult, bestPath
}

func scoreCandidate(candidate Candidate) int {
	score := scoreResult(candidate.Result)
	translated := 0
	for _, line := range candidate.Result.Lines {
		if line.Translation != "" {
			translated++
		}
	}
	if translated > 0 {
		score += 100 + minInt(translated, 50)
	}
	return score
}

func scoreResult(result Result) int {
	score := timingQualityRank(result.TimingQuality) * 10_000
	exactEnds := 0
	timedWords := 0
	for _, line := range result.Lines {
		if !line.EndEstimated && line.End > line.Start {
			exactEnds++
		}
		for _, word := range line.Words {
			if word.End > word.Start {
				timedWords++
			}
		}
	}
	score += minInt(exactEnds, 1_000)
	score += minInt(timedWords, 2_000)
	score += minInt(len(result.Lines), 500)
	return score
}

func candidateFileNames(mediaPath string) []sidecarSpec {
	fileName := portableBase(mediaPath)
	stem := portableStem(fileName)
	bases := uniqueStrings([]string{stem, fileName})
	mainExtensions := []string{".lrc", ".vtt", ".ttml", ".LRC", ".VTT", ".TTML"}
	translationSuffixes := []string{".t.lrc", ".t.vtt", ".T.LRC", ".T.VTT"}

	specs := make([]sidecarSpec, 0, len(bases)*len(mainExtensions))
	for _, base := range bases {
		otherBases := make([]string, 0, len(bases))
		otherBases = append(otherBases, base)
		for _, other := range bases {
			if other != base {
				otherBases = append(otherBases, other)
			}
		}
		translations := make([]string, 0, len(otherBases)*len(translationSuffixes))
		for _, translationBase := range otherBases {
			for _, suffix := range translationSuffixes {
				translations = append(translations, translationBase+suffix)
			}
		}
		for _, extension := range mainExtensions {
			specs = append(specs, sidecarSpec{
				mainName:         base + extension,
				translationNames: translations,
			})
		}
	}
	return specs
}

func portableBase(value string) string {
	normalized := strings.ReplaceAll(value, "\\", "/")
	if slash := strings.LastIndex(normalized, "/"); slash >= 0 {
		normalized = normalized[slash+1:]
	}
	return normalized
}

func portableStem(fileName string) string {
	lastDot := strings.LastIndex(fileName, ".")
	if lastDot <= 0 {
		return fileName
	}
	return fileName[:lastDot]
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func formatFromName(name string) Format {
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".ttml"):
		return FormatTTML
	case strings.HasSuffix(lower, ".vtt"):
		return FormatVTT
	case strings.HasSuffix(lower, ".lrc"):
		return FormatLRC
	default:
		return FormatPlain
	}
}

func minInt(left int, right int) int {
	if left < right {
		return left
	}
	return right
}
