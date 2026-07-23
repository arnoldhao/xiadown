package youtubemusic

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

const lyricsPresentationNoiseExpression = `(?:` +
	`official\s+(?:audio|video|music\s+video|mv)|` +
	`(?:official\s+)?lyrics?\s+video|` +
	`(?:official\s+)?visuali[sz]er|` +
	`(?:official\s+)?mv|` +
	`官方(?:音频|音頻|视频|影片|音乐视频|音樂影片|音樂錄影帶|mv|歌词视频|歌詞影片|可视化|視覺化)|` +
	`(?:歌词|歌詞)(?:视频|影片|版)|` +
	`(?:动态歌词|動態歌詞)|` +
	`(?:可视化|視覺化)` +
	`)`

var (
	lyricsTopicArtistSuffixPattern          = regexp.MustCompile(`(?i)^(.*?)\s*[-–—]\s*topic\s*$`)
	lyricsBracketedPresentationNoisePattern = regexp.MustCompile(
		`(?i)^(.*?)\s*[\(\[\{【]\s*` + lyricsPresentationNoiseExpression + `\s*[\)\]\}】]\s*$`,
	)
	lyricsSeparatedPresentationNoisePattern = regexp.MustCompile(
		`(?i)^(.*?)\s*(?:[-–—|:：])\s*` + lyricsPresentationNoiseExpression + `\s*$`,
	)
	lyricsSpacedPresentationNoisePattern = regexp.MustCompile(
		`(?i)^(.+?)\s+` + lyricsPresentationNoiseExpression + `\s*$`,
	)
	lyricsJoinedChinesePresentationNoisePattern = regexp.MustCompile(
		`(?i)^(.+?)` +
			`(?:官方(?:音频|音頻|视频|影片|音乐视频|音樂影片|音樂錄影帶|mv|歌词视频|歌詞影片|可视化|視覺化)|` +
			`(?:歌词|歌詞)(?:视频|影片|版)|(?:动态歌词|動態歌詞)|(?:可视化|視覺化))\s*$`,
	)
)

// normalizeLyricsIdentityTitle removes presentation-only suffixes commonly
// inherited from video titles. Semantic recording versions such as Live,
// Remix, Acoustic, and Instrumental are deliberately not part of the
// allow-list and therefore remain identity-significant.
func normalizeLyricsIdentityTitle(value string) string {
	result := strings.TrimSpace(norm.NFKC.String(value))
	for result != "" {
		previous := result
		for _, pattern := range []*regexp.Regexp{
			lyricsBracketedPresentationNoisePattern,
			lyricsSeparatedPresentationNoisePattern,
			lyricsSpacedPresentationNoisePattern,
			lyricsJoinedChinesePresentationNoisePattern,
		} {
			matches := pattern.FindStringSubmatch(result)
			if len(matches) != 2 {
				continue
			}
			if core := strings.TrimSpace(matches[1]); core != "" {
				result = core
			}
			break
		}
		if result == previous {
			break
		}
	}
	return result
}

// normalizeLyricsIdentityArtist removes only YouTube's anchored "- Topic"
// channel decoration. Keeping the match anchored avoids changing legitimate
// artist names that merely contain the word Topic.
func normalizeLyricsIdentityArtist(value string) string {
	result := strings.TrimSpace(norm.NFKC.String(value))
	for result != "" {
		matches := lyricsTopicArtistSuffixPattern.FindStringSubmatch(result)
		if len(matches) != 2 {
			break
		}
		core := strings.TrimSpace(matches[1])
		if core == "" || core == result {
			break
		}
		result = core
	}
	return result
}

type lrcLibSearchQueryVariant struct {
	info      LyricsSearchInfo
	titleOnly bool
}

// buildLRCLibSearchQueryVariants produces a small, deterministic relaxation
// ladder. Each step drops only metadata that commonly differs across local
// files and community catalogs; the caller remains responsible for scoring
// every returned record against the full target identity.
func buildLRCLibSearchQueryVariants(info LyricsSearchInfo) []lrcLibSearchQueryVariant {
	info = normalizeLyricsSearchInfo(info)
	if strings.TrimSpace(info.Title) == "" {
		return nil
	}

	variants := make([]lrcLibSearchQueryVariant, 0, 4)
	seen := make(map[string]bool, 4)
	appendVariant := func(candidate LyricsSearchInfo) {
		values := buildLRCLibSearchQuery(candidate)
		key := values.Encode()
		if key == "" || seen[key] {
			return
		}
		seen[key] = true
		variants = append(variants, lrcLibSearchQueryVariant{
			info:      candidate,
			titleOnly: strings.TrimSpace(candidate.Artist) == "" && strings.TrimSpace(candidate.Album) == "",
		})
	}

	appendVariant(info)

	withoutAlbum := info
	withoutAlbum.Album = ""
	appendVariant(withoutAlbum)

	primaryArtist := primaryLyricsArtistForQuery(info.Artist)
	if primaryArtist != "" {
		withPrimaryArtist := withoutAlbum
		withPrimaryArtist.Artist = primaryArtist
		appendVariant(withPrimaryArtist)
	}

	titleOnly := info
	titleOnly.Artist = ""
	titleOnly.Album = ""
	appendVariant(titleOnly)

	return variants
}

func primaryLyricsArtistForQuery(value string) string {
	parts := lyricsArtistSeparator.Split(strings.TrimSpace(value), 2)
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimSpace(parts[0])
}

const (
	lyricsScriptLatin uint32 = 1 << iota
	lyricsScriptHan
	lyricsScriptJapanese
	lyricsScriptHangul
	lyricsScriptCyrillic
	lyricsScriptGreek
	lyricsScriptOther
)

// lyricsArtistScriptsDisjoint identifies artist values that cannot be
// compared lexically because they are written in disjoint scripts, for example
// "Jay Chou" and "周杰伦". It does not assert that the values are aliases.
func lyricsArtistScriptsDisjoint(left string, right string) bool {
	leftScripts := lyricsIdentityScriptMask(left)
	rightScripts := lyricsIdentityScriptMask(right)
	return leftScripts != 0 && rightScripts != 0 && leftScripts&rightScripts == 0
}

// titleOnlyLRCLibAutomaticEligible is the sole exception to the rule that a
// record discovered only by a title-only provider query requires explicit
// confirmation. A disjoint-script artist pair cannot be compared lexically,
// so an exact normalized title plus a provider duration within two seconds is
// accepted as independent identity evidence. Same-script conflicts, missing
// durations, and ordinary title-only matches remain manual.
func titleOnlyLRCLibAutomaticEligible(model lrcLibModel, info LyricsSearchInfo) bool {
	info = normalizeLyricsSearchInfo(info)
	targetTitle := normalizeLyricsMatchText(normalizeLyricsIdentityTitle(info.Title))
	candidateTitle := normalizeLyricsMatchText(normalizeLyricsIdentityTitle(model.TrackName))
	if targetTitle == "" || targetTitle != candidateTitle {
		return false
	}
	targetArtist := normalizeLyricsIdentityArtist(info.Artist)
	candidateArtist := normalizeLyricsIdentityArtist(model.ArtistName)
	if !lyricsArtistScriptsDisjoint(targetArtist, candidateArtist) {
		return false
	}
	_, durationDiff, compared, compatible := lrcLibDurationSimilarity(
		info.DurationSeconds,
		model.Duration,
	)
	return compared && compatible && durationDiff <= 2
}

func lyricsIdentityScriptMask(value string) uint32 {
	var result uint32
	for _, character := range norm.NFKC.String(value) {
		if !unicode.IsLetter(character) {
			continue
		}
		switch {
		case unicode.In(character, unicode.Latin):
			result |= lyricsScriptLatin
		case unicode.In(character, unicode.Han):
			result |= lyricsScriptHan
		case unicode.In(character, unicode.Hiragana, unicode.Katakana):
			result |= lyricsScriptJapanese
		case unicode.In(character, unicode.Hangul):
			result |= lyricsScriptHangul
		case unicode.In(character, unicode.Cyrillic):
			result |= lyricsScriptCyrillic
		case unicode.In(character, unicode.Greek):
			result |= lyricsScriptGreek
		default:
			result |= lyricsScriptOther
		}
	}
	return result
}
