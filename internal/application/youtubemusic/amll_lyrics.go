package youtubemusic

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"
	"unicode"

	"xiadown/internal/application/locallyrics"
)

const (
	lyricsProviderAMLL        = "amll_ttml_db"
	lyricsSourceAMLL          = "AMLL TTML DB"
	lyricsAttributionAMLL     = "AMLL TTML DB contributors"
	amllCommunitySearchURL    = "https://amlldb.bikonoo.com/api/search-lyrics"
	amllCommunityRawBaseURL   = "https://amlldb.bikonoo.com/raw-lyrics/"
	amllOfficialCatalogURL    = "https://raw.githubusercontent.com/amll-dev/amll-ttml-db/refs/heads/main/metadata/raw-lyrics-index.jsonl"
	amllOfficialRawBaseURL    = "https://raw.githubusercontent.com/amll-dev/amll-ttml-db/refs/heads/main/raw-lyrics/"
	amllCommunitySearchLimit  = 512 << 10
	amllCatalogResponseLimit  = 4 << 20
	amllTTMLResponseLimit     = 2 << 20
	amllCommunitySearchWait   = 3 * time.Second
	amllCommunityRawHedgeWait = 200 * time.Millisecond
	amllCatalogRequestWait    = 4 * time.Second
	amllTTMLRequestWait       = 5 * time.Second
	amllAutomaticProviderWait = 10 * time.Second
	amllCatalogFreshTTL       = 24 * time.Hour
	amllAutomaticProbeLimit   = 3
	amllCandidateDisplayLimit = 20
)

type amllStringList []string

func (values *amllStringList) UnmarshalJSON(data []byte) error {
	var list []string
	if err := json.Unmarshal(data, &list); err == nil {
		*values = normalizeAMLLStrings(list)
		return nil
	}
	var single string
	if err := json.Unmarshal(data, &single); err == nil {
		*values = normalizeAMLLStrings([]string{single})
		return nil
	}
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		*values = nil
		return nil
	}
	return fmt.Errorf("invalid AMLL string list")
}

type amllCommunityRecord struct {
	Platform    string         `json:"platform"`
	ID          string         `json:"id"`
	File        string         `json:"file"`
	Title       string         `json:"title"`
	Titles      amllStringList `json:"titles"`
	Artist      string         `json:"artist"`
	Artists     amllStringList `json:"artists"`
	Album       amllStringList `json:"album"`
	Albums      amllStringList `json:"albums"`
	AuthorNames amllStringList `json:"authorNames"`
	NCMIDs      amllStringList `json:"ncmIds"`
	QQIDs       amllStringList `json:"qqIds"`
	AMIDs       amllStringList `json:"amIds"`
	SpotifyIDs  amllStringList `json:"spotifyIds"`
}

type amllCatalogLine struct {
	Metadata     [][]json.RawMessage `json:"metadata"`
	RawLyricFile string              `json:"rawLyricFile"`
}

type amllRecord struct {
	RawLyricFile string
	Titles       []string
	Artists      []string
	Albums       []string
	AuthorNames  []string
	NCMIDs       []string
	QQIDs        []string
	AMIDs        []string
	SpotifyIDs   []string
	ISRCs        []string
}

type amllCatalogSnapshot struct {
	records   []amllRecord
	etag      string
	fetchedAt time.Time
}

type amllCatalogFetchCall struct {
	done    chan struct{}
	records []amllRecord
	err     error
}

func (client *Client) searchAMLLLyricsProvider(ctx context.Context, info LyricsSearchInfo) (LyricsResult, error) {
	return client.searchAMLLLyricsProviderWithin(ctx, info, amllAutomaticProviderWait)
}

func (client *Client) searchAMLLLyricsProviderWithin(ctx context.Context, info LyricsSearchInfo, budget time.Duration) (LyricsResult, error) {
	if info.PlainOnly || strings.TrimSpace(info.Title) == "" || !hasLRCLibAutomaticCorroboration(info) {
		return LyricsResult{Kind: lyricsResultUnavailable}, nil
	}
	if budget <= 0 || budget > amllAutomaticProviderWait {
		budget = amllAutomaticProviderWait
	}
	providerCtx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()
	candidates, err := client.searchAMLLLyricsCandidates(providerCtx, info)
	if err != nil {
		if ctx.Err() != nil {
			return LyricsResult{}, wrapRequestError(ctx.Err())
		}
		// AMLL is an optional enrichment provider. Its transport or catalog
		// failure must never turn an otherwise usable YTMusic/LRCLIB request
		// into a provider error.
		logLyricsf("amll automatic search soft failure %s err=%v", lyricsInfoSummary(info), err)
		return LyricsResult{Kind: lyricsResultUnavailable}, nil
	}

	probed := 0
	for _, candidate := range candidates {
		if !candidate.Accepted || candidate.ProviderID != lyricsProviderAMLL {
			continue
		}
		probed++
		result, fetchErr := client.fetchAMLLLyricsCandidate(
			providerCtx,
			candidate.ProviderTrackID,
			candidate.Confidence,
			candidate.Attribution,
		)
		if fetchErr == nil && result.Kind == lyricsResultSynced {
			// Candidate order already reflects the strongest identity match. Return
			// the first valid timed TTML even when it is line-only; the outer
			// provider arbitration can still upgrade it if another source returns
			// richer word or syllable timing.
			return result, nil
		}
		if ctx.Err() != nil {
			return LyricsResult{}, wrapRequestError(ctx.Err())
		}
		if providerCtx.Err() != nil {
			return LyricsResult{Kind: lyricsResultUnavailable}, nil
		}
		if fetchErr != nil {
			logLyricsf("amll automatic candidate soft failure id=%q err=%v", candidate.ProviderTrackID, fetchErr)
		}
		if probed >= amllAutomaticProbeLimit {
			break
		}
	}
	return LyricsResult{Kind: lyricsResultUnavailable}, nil
}

func (client *Client) searchAMLLLyricsCandidates(ctx context.Context, info LyricsSearchInfo) ([]LyricsCandidate, error) {
	return client.searchAMLLLyricsCandidatesForTitle(ctx, []LyricsSearchInfo{info})
}

// searchAMLLLyricsCandidatesForTitle performs the title-only community lookup
// once, then scores the returned records against every identity in the group.
// Callers must group infos by their normalized effective title.
func (client *Client) searchAMLLLyricsCandidatesForTitle(ctx context.Context, infos []LyricsSearchInfo) ([]LyricsCandidate, error) {
	normalized := make([]LyricsSearchInfo, 0, len(infos))
	for _, info := range infos {
		info = normalizeLyricsSearchInfo(info)
		if info.PlainOnly || strings.TrimSpace(info.Title) == "" {
			continue
		}
		normalized = append(normalized, info)
	}
	if len(normalized) == 0 {
		return []LyricsCandidate{}, nil
	}

	searchCtx, cancel := context.WithTimeout(ctx, amllCommunitySearchWait)
	records, communityErr := client.requestAMLLCommunitySearch(searchCtx, normalized[0])
	cancel()
	var catalog []amllRecord
	if communityErr != nil || len(records) == 0 {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		var catalogErr error
		catalog, catalogErr = client.loadAMLLCatalog(ctx)
		if catalogErr != nil {
			if communityErr == nil {
				// A successful empty community response is authoritative enough to
				// remain a soft miss when the catalog CDN is also unavailable.
				return []LyricsCandidate{}, nil
			}
			return nil, errors.Join(communityErr, catalogErr)
		}
	}

	merged := make([]LyricsCandidate, 0, amllCandidateDisplayLimit)
	mergedIndex := make(map[string]int, amllCandidateDisplayLimit)
	for _, info := range normalized {
		identityRecords := records
		if catalog != nil {
			identityRecords = searchAMLLCatalog(catalog, info)
		}
		for _, record := range dedupeAMLLRecords(identityRecords) {
			candidate, ok := lyricsCandidateFromAMLLRecord(record, info)
			if !ok {
				continue
			}
			key := strings.ToLower(strings.TrimSpace(candidate.ProviderID)) + "\x00" +
				strings.TrimSpace(candidate.ProviderTrackID)
			if index, exists := mergedIndex[key]; exists {
				merged[index] = mergeLyricsCandidate(merged[index], candidate)
				continue
			}
			mergedIndex[key] = len(merged)
			merged = append(merged, candidate)
		}
	}
	sort.SliceStable(merged, func(left, right int) bool {
		return betterLyricsCandidate(merged[left], merged[right])
	})
	if len(merged) > amllCandidateDisplayLimit {
		merged = merged[:amllCandidateDisplayLimit]
	}
	return merged, nil
}

func (client *Client) requestAMLLCommunitySearch(ctx context.Context, info LyricsSearchInfo) ([]amllRecord, error) {
	query := buildAMLLSearchQuery(info)
	if query == "" {
		return []amllRecord{}, nil
	}
	body, err := json.Marshal(map[string]string{"query": query, "type": "title"})
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, amllCommunitySearchURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build AMLL community search request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", client.userAgent)
	response, err := client.httpClientForRequest().Do(request)
	if err != nil {
		return nil, fmt.Errorf("AMLL community search: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4<<10))
		return nil, fmt.Errorf("AMLL community search status %d", response.StatusCode)
	}
	data, err := readAMLLBoundedBody(response.Body, amllCommunitySearchLimit)
	if err != nil {
		return nil, fmt.Errorf("AMLL community search response: %w", err)
	}
	var payload []amllCommunityRecord
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("AMLL community search response invalid: %w", err)
	}
	records := make([]amllRecord, 0, len(payload))
	for _, item := range payload {
		record := amllRecordFromCommunity(item)
		if validAMLLRawLyricFile(record.RawLyricFile) {
			records = append(records, record)
		}
	}
	return records, nil
}

func (client *Client) loadAMLLCatalog(ctx context.Context) ([]amllRecord, error) {
	now := client.currentTime()
	client.amllCatalogMu.Lock()
	snapshot := client.amllCatalog
	if len(snapshot.records) > 0 && now.Sub(snapshot.fetchedAt) >= 0 && now.Sub(snapshot.fetchedAt) <= amllCatalogFreshTTL {
		records := cloneAMLLRecords(snapshot.records)
		client.amllCatalogMu.Unlock()
		return records, nil
	}
	if call := client.amllCatalogCall; call != nil {
		client.amllCatalogMu.Unlock()
		select {
		case <-call.done:
			return cloneAMLLRecords(call.records), call.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	call := &amllCatalogFetchCall{done: make(chan struct{})}
	client.amllCatalogCall = call
	client.amllCatalogMu.Unlock()

	requestCtx, cancel := context.WithTimeout(ctx, amllCatalogRequestWait)
	records, etag, notModified, err := client.requestAMLLCatalog(requestCtx, snapshot.etag)
	cancel()
	if err != nil && len(snapshot.records) > 0 && ctx.Err() == nil {
		// The catalog is append-oriented. A previously validated snapshot is a
		// safer availability fallback than failing every search during a CDN or
		// proxy outage.
		records = cloneAMLLRecords(snapshot.records)
		etag = snapshot.etag
		err = nil
	} else if notModified {
		if len(snapshot.records) == 0 {
			err = fmt.Errorf("AMLL catalog returned not-modified without a cached snapshot")
		} else {
			records = cloneAMLLRecords(snapshot.records)
			etag = snapshot.etag
		}
	}

	client.amllCatalogMu.Lock()
	if err == nil {
		client.amllCatalog = amllCatalogSnapshot{
			records:   cloneAMLLRecords(records),
			etag:      strings.TrimSpace(etag),
			fetchedAt: client.currentTime(),
		}
	}
	call.records = cloneAMLLRecords(records)
	call.err = err
	client.amllCatalogCall = nil
	close(call.done)
	client.amllCatalogMu.Unlock()
	return records, err
}

func (client *Client) requestAMLLCatalog(ctx context.Context, etag string) ([]amllRecord, string, bool, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, amllOfficialCatalogURL, nil)
	if err != nil {
		return nil, "", false, fmt.Errorf("build AMLL catalog request: %w", err)
	}
	request.Header.Set("Accept", "application/x-ndjson, application/json, text/plain")
	request.Header.Set("User-Agent", client.userAgent)
	if strings.TrimSpace(etag) != "" {
		request.Header.Set("If-None-Match", strings.TrimSpace(etag))
	}
	response, err := client.httpClientForRequest().Do(request)
	if err != nil {
		return nil, "", false, fmt.Errorf("AMLL catalog: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotModified {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4<<10))
		return nil, response.Header.Get("ETag"), true, nil
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4<<10))
		return nil, "", false, fmt.Errorf("AMLL catalog status %d", response.StatusCode)
	}
	data, err := readAMLLBoundedBody(response.Body, amllCatalogResponseLimit)
	if err != nil {
		return nil, "", false, fmt.Errorf("AMLL catalog response: %w", err)
	}
	records, err := parseAMLLCatalog(data)
	if err != nil {
		return nil, "", false, err
	}
	return records, response.Header.Get("ETag"), false, nil
}

func parseAMLLCatalog(data []byte) ([]amllRecord, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 64<<10), 512<<10)
	records := make([]amllRecord, 0, 4096)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var payload amllCatalogLine
		if err := json.Unmarshal(line, &payload); err != nil {
			return nil, fmt.Errorf("AMLL catalog line invalid: %w", err)
		}
		record := amllRecordFromCatalogLine(payload)
		if validAMLLRawLyricFile(record.RawLyricFile) {
			records = append(records, record)
		}
		if len(records) > 20_000 {
			return nil, fmt.Errorf("AMLL catalog exceeds record limit")
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("AMLL catalog scan: %w", err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("AMLL catalog contains no usable records")
	}
	return dedupeAMLLRecords(records), nil
}

func amllRecordFromCatalogLine(line amllCatalogLine) amllRecord {
	record := amllRecord{RawLyricFile: strings.TrimSpace(line.RawLyricFile)}
	for _, pair := range line.Metadata {
		if len(pair) != 2 {
			continue
		}
		var key string
		var values []string
		if json.Unmarshal(pair[0], &key) != nil || json.Unmarshal(pair[1], &values) != nil {
			continue
		}
		values = normalizeAMLLStrings(values)
		switch strings.TrimSpace(key) {
		case "musicName":
			record.Titles = append(record.Titles, values...)
		case "artists":
			record.Artists = append(record.Artists, values...)
		case "album":
			record.Albums = append(record.Albums, values...)
		case "ttmlAuthorGithubLogin":
			record.AuthorNames = append(record.AuthorNames, values...)
		case "ncmMusicId":
			record.NCMIDs = append(record.NCMIDs, values...)
		case "qqMusicId":
			record.QQIDs = append(record.QQIDs, values...)
		case "appleMusicId":
			record.AMIDs = append(record.AMIDs, values...)
		case "spotifyId":
			record.SpotifyIDs = append(record.SpotifyIDs, values...)
		case "isrc":
			record.ISRCs = append(record.ISRCs, values...)
		}
	}
	return normalizeAMLLRecord(record)
}

func amllRecordFromCommunity(item amllCommunityRecord) amllRecord {
	rawFile := strings.TrimSpace(item.File)
	if rawFile == "" {
		rawFile = strings.TrimSpace(item.ID)
	}
	titles := append([]string(nil), item.Titles...)
	if strings.TrimSpace(item.Title) != "" {
		titles = append([]string{item.Title}, titles...)
	}
	artists := append([]string(nil), item.Artists...)
	if strings.TrimSpace(item.Artist) != "" {
		artists = append([]string{item.Artist}, artists...)
	}
	albums := append([]string(nil), item.Albums...)
	albums = append(albums, item.Album...)
	return normalizeAMLLRecord(amllRecord{
		RawLyricFile: rawFile,
		Titles:       titles,
		Artists:      artists,
		Albums:       albums,
		AuthorNames:  item.AuthorNames,
		NCMIDs:       item.NCMIDs,
		QQIDs:        item.QQIDs,
		AMIDs:        item.AMIDs,
		SpotifyIDs:   item.SpotifyIDs,
	})
}

func searchAMLLCatalog(catalog []amllRecord, info LyricsSearchInfo) []amllRecord {
	targetTitle := normalizeLyricsMatchText(info.Title)
	if targetTitle == "" {
		return nil
	}
	results := make([]amllRecord, 0, 32)
	for _, record := range catalog {
		bestTitleScore := 0.0
		containsTarget := false
		for _, title := range record.Titles {
			normalizedTitle := normalizeLyricsMatchText(title)
			bestTitleScore = max(bestTitleScore, lyricsTextSimilarity(targetTitle, normalizedTitle))
			containsTarget = containsTarget || strings.Contains(normalizedTitle, targetTitle) || strings.Contains(targetTitle, normalizedTitle)
		}
		if bestTitleScore < 0.35 && !containsTarget {
			continue
		}
		results = append(results, record)
	}
	return results
}

func lyricsCandidateFromAMLLRecord(record amllRecord, info LyricsSearchInfo) (LyricsCandidate, bool) {
	if !validAMLLRawLyricFile(record.RawLyricFile) {
		return LyricsCandidate{}, false
	}
	title := bestAMLLTitle(record.Titles, info.Title)
	artist := strings.Join(record.Artists, ", ")
	album := bestAMLLText(record.Albums, info.Album)
	if title == "" {
		return LyricsCandidate{}, false
	}
	match, accepted := scoreLRCLibCandidate(lrcLibModel{
		TrackName:  title,
		ArtistName: artist,
		AlbumName:  album,
	}, info)
	if accepted && !hasLRCLibAutomaticCorroboration(info) {
		accepted = false
		match.rejection = "insufficient metadata for automatic match"
	}
	attribution := lyricsAttributionAMLL
	if author := firstAMLLString(record.AuthorNames); author != "" {
		attribution += " · " + author
	}
	return LyricsCandidate{
		ProviderID:      lyricsProviderAMLL,
		ProviderTrackID: record.RawLyricFile,
		Title:           title,
		Artist:          artist,
		Album:           album,
		HasSynced:       true,
		// The catalog proves that the payload is timed TTML, but it does not
		// describe its granularity. AMLL explicitly accepts exceptional
		// line-timed submissions, so only promise the conservative lower bound
		// until preview/fetch parsing reports the actual capability.
		TimingQuality: "line",
		Attribution:   attribution,
		Confidence:    lyricsCandidatePercent(match.confidence),
		TitleScore:    lyricsCandidatePercent(match.titleScore),
		ArtistScore:   lyricsCandidatePercent(match.artistScore),
		AlbumScore:    lyricsCandidatePercent(match.albumScore),
		DurationScore: lyricsCandidatePercent(match.durationScore),
		DurationDiff:  match.durationDiff,
		Accepted:      accepted,
		Rejection:     match.rejection,
	}, true
}

func (client *Client) fetchAMLLLyricsCandidate(ctx context.Context, rawFile string, confidence int, attribution string) (LyricsResult, error) {
	if !validAMLLRawLyricFile(rawFile) {
		return LyricsResult{}, fmt.Errorf("invalid AMLL lyrics candidate id")
	}
	if strings.TrimSpace(attribution) == "" {
		attribution = lyricsAttributionAMLL
	}
	parsed, err := client.fetchAMLLTTMLHedged(ctx, rawFile)
	if err != nil {
		return LyricsResult{}, err
	}
	return lyricsResultFromAMLL(parsed, rawFile, confidence, attribution), nil
}

func (client *Client) fetchAMLLTTMLHedged(ctx context.Context, rawFile string) (locallyrics.Result, error) {
	if !validAMLLRawLyricFile(rawFile) {
		return locallyrics.Result{}, fmt.Errorf("invalid AMLL lyrics candidate id")
	}

	type fetchResult struct {
		provider string
		lyrics   locallyrics.Result
		err      error
	}

	fetchCtx, cancelFetches := context.WithCancel(ctx)
	defer cancelFetches()
	results := make(chan fetchResult, 2)
	startFetch := func(provider string, requestURL string) {
		go func() {
			requestCtx, cancelRequest := context.WithTimeout(fetchCtx, amllTTMLRequestWait)
			defer cancelRequest()
			parsed, err := client.requestAndParseAMLLTTML(requestCtx, requestURL)
			// The channel is sized for both hedged requests, so a sibling that
			// finishes after the caller has selected a winner can always publish
			// and exit without leaking a goroutine.
			results <- fetchResult{provider: provider, lyrics: parsed, err: err}
		}()
	}

	escapedFile := url.PathEscape(rawFile)
	startFetch("official", amllOfficialRawBaseURL+escapedFile)
	started := 1
	completed := 0
	communityStarted := false
	failures := make([]error, 0, 2)

	hedgeTimer := time.NewTimer(amllCommunityRawHedgeWait)
	defer hedgeTimer.Stop()
	hedgeTimerCh := hedgeTimer.C
	startCommunity := func() {
		if communityStarted || fetchCtx.Err() != nil {
			return
		}
		communityStarted = true
		started++
		startFetch("community", amllCommunityRawBaseURL+escapedFile)
	}

	for {
		select {
		case result := <-results:
			completed++
			if result.err == nil {
				if err := ctx.Err(); err != nil {
					return locallyrics.Result{}, err
				}
				cancelFetches()
				return result.lyrics, nil
			}
			failures = append(failures, fmt.Errorf("AMLL %s raw: %w", result.provider, result.err))
			if result.provider == "official" && !communityStarted {
				if !hedgeTimer.Stop() {
					select {
					case <-hedgeTimer.C:
					default:
					}
				}
				hedgeTimerCh = nil
				startCommunity()
			}
			if communityStarted && completed == started {
				return locallyrics.Result{}, errors.Join(failures...)
			}
		case <-hedgeTimerCh:
			hedgeTimerCh = nil
			startCommunity()
		case <-ctx.Done():
			return locallyrics.Result{}, ctx.Err()
		}
	}
}

func (client *Client) requestAndParseAMLLTTML(ctx context.Context, requestURL string) (locallyrics.Result, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return locallyrics.Result{}, fmt.Errorf("build AMLL TTML request: %w", err)
	}
	request.Header.Set("Accept", "application/ttml+xml, application/xml, text/xml, text/plain, application/octet-stream")
	request.Header.Set("User-Agent", client.userAgent)
	response, err := client.httpClientForRequest().Do(request)
	if err != nil {
		return locallyrics.Result{}, fmt.Errorf("AMLL TTML request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4<<10))
		return locallyrics.Result{}, fmt.Errorf("AMLL TTML status %d", response.StatusCode)
	}
	data, err := readAMLLBoundedBody(response.Body, amllTTMLResponseLimit)
	if err != nil {
		return locallyrics.Result{}, fmt.Errorf("AMLL TTML response: %w", err)
	}
	if !bytes.Contains(bytes.ToLower(data), []byte("<tt")) {
		return locallyrics.Result{}, fmt.Errorf("AMLL TTML response invalid")
	}
	parsed, err := locallyrics.ParseContent(data, locallyrics.FormatTTML, locallyrics.Options{
		MaxBytes:             amllTTMLResponseLimit,
		DisablePlainFallback: true,
	})
	if err != nil {
		return locallyrics.Result{}, fmt.Errorf("parse AMLL TTML: %w", err)
	}
	if len(parsed.Lines) == 0 || parsed.TimingQuality == locallyrics.TimingQualityPlain {
		return locallyrics.Result{}, fmt.Errorf("AMLL TTML contains no timed lyrics")
	}
	return parsed, nil
}

func lyricsResultFromAMLL(parsed locallyrics.Result, rawFile string, confidence int, attribution string) LyricsResult {
	lines := make([]LyricLine, 0, len(parsed.Lines))
	for _, line := range parsed.Lines {
		romanizedText := ""
		romanizedKind := ""
		for _, alternate := range line.AlternateTexts {
			role := strings.ToLower(strings.TrimSpace(alternate.Role))
			if romanizedText == "" && (role == "romanization" || role == "romanized" || role == "roman" || role == "pinyin") {
				romanizedText = strings.TrimSpace(alternate.Text)
				if role == "pinyin" {
					romanizedKind = "pinyin"
				} else {
					romanizedKind = "romanized"
				}
			}
		}
		lines = append(lines, LyricLine{
			StartMs:         amllDurationMilliseconds(line.Start),
			DurationMs:      amllDurationMilliseconds(max(0, line.End-line.Start)),
			Text:            line.Text,
			TranslationText: line.Translation,
			RomanizedText:   romanizedText,
			RomanizedKind:   romanizedKind,
			Words:           timedWordsFromAMLL(line.Words),
		})
	}
	return enrichLyricsResult(LyricsResult{
		Kind:            lyricsResultSynced,
		Source:          lyricsSourceAMLL,
		ProviderID:      lyricsProviderAMLL,
		ProviderTrackID: rawFile,
		Attribution:     attribution,
		TimingQuality:   string(parsed.TimingQuality),
		Confidence:      confidence,
		Text:            parsed.PlainText,
		Lines:           lines,
	})
}

func timedWordsFromAMLL(words []locallyrics.Word) []TimedWord {
	if len(words) == 0 {
		return nil
	}
	result := make([]TimedWord, 0, len(words))
	for _, word := range words {
		endsWithSpace := word.EndsWithSpace
		result = append(result, TimedWord{
			StartMs:       amllDurationMilliseconds(word.Start),
			EndMs:         amllDurationMilliseconds(word.End),
			Text:          word.Text,
			EndsWithSpace: &endsWithSpace,
			Syllables:     timedWordsFromAMLL(word.Syllables),
		})
	}
	return result
}

func amllDurationMilliseconds(value time.Duration) int {
	if value <= 0 {
		return 0
	}
	return int(value / time.Millisecond)
}

func buildAMLLSearchQuery(info LyricsSearchInfo) string {
	// The community endpoint's title search is not a general token search: a
	// query like "Idol YOASOBI" can return no rows even though "Idol" does.
	// Send title only, then apply XiaDown's title/artist/album scoring locally.
	return strings.TrimSpace(info.Title)
}

func normalizeAMLLRecord(record amllRecord) amllRecord {
	record.RawLyricFile = strings.TrimSpace(record.RawLyricFile)
	record.Titles = normalizeAMLLStrings(record.Titles)
	record.Artists = normalizeAMLLStrings(record.Artists)
	record.Albums = normalizeAMLLStrings(record.Albums)
	record.AuthorNames = normalizeAMLLStrings(record.AuthorNames)
	record.NCMIDs = normalizeAMLLStrings(record.NCMIDs)
	record.QQIDs = normalizeAMLLStrings(record.QQIDs)
	record.AMIDs = normalizeAMLLStrings(record.AMIDs)
	record.SpotifyIDs = normalizeAMLLStrings(record.SpotifyIDs)
	record.ISRCs = normalizeAMLLStrings(record.ISRCs)
	return record
}

func normalizeAMLLStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		key := strings.ToLower(trimmed)
		if trimmed == "" || seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, trimmed)
	}
	return result
}

func dedupeAMLLRecords(records []amllRecord) []amllRecord {
	result := make([]amllRecord, 0, len(records))
	indexes := make(map[string]int, len(records))
	for _, raw := range records {
		record := normalizeAMLLRecord(raw)
		if !validAMLLRawLyricFile(record.RawLyricFile) {
			continue
		}
		key := amllRecordIdentityKey(record)
		if index, ok := indexes[key]; ok {
			// Raw filenames begin with a millisecond timestamp. Keeping the
			// lexicographically newer file collapses historical revisions while
			// preserving the immutable file as the eventual providerTrackId.
			if record.RawLyricFile > result[index].RawLyricFile {
				result[index] = record
			}
			continue
		}
		indexes[key] = len(result)
		result = append(result, record)
	}
	return result
}

func amllRecordIdentityKey(record amllRecord) string {
	identities := []struct {
		prefix string
		values []string
	}{
		{prefix: "ncm", values: record.NCMIDs},
		{prefix: "qq", values: record.QQIDs},
		{prefix: "apple", values: record.AMIDs},
		{prefix: "spotify", values: record.SpotifyIDs},
		{prefix: "isrc", values: record.ISRCs},
	}
	for _, identity := range identities {
		if value := firstAMLLString(identity.values); value != "" {
			return identity.prefix + ":" + strings.ToLower(value)
		}
	}
	return strings.Join([]string{
		normalizeLyricsMatchText(firstAMLLString(record.Titles)),
		normalizeLyricsMatchText(strings.Join(record.Artists, ",")),
		normalizeLyricsMatchText(firstAMLLString(record.Albums)),
	}, "\x00")
}

func validAMLLRawLyricFile(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || len(trimmed) > 255 || path.Base(trimmed) != trimmed || !strings.HasSuffix(strings.ToLower(trimmed), ".ttml") {
		return false
	}
	firstDash := strings.IndexByte(trimmed, '-')
	if firstDash != 13 || strings.IndexByte(trimmed[firstDash+1:], '-') < 1 {
		return false
	}
	for _, character := range trimmed[:firstDash] {
		if character < '0' || character > '9' {
			return false
		}
	}
	for _, character := range trimmed {
		if unicode.IsLetter(character) || unicode.IsDigit(character) || character == '-' || character == '_' || character == '.' {
			continue
		}
		return false
	}
	return true
}

func firstAMLLString(values []string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func bestAMLLTitle(values []string, target string) string {
	best := firstAMLLString(values)
	bestScore := -1.0
	for _, value := range values {
		score, compatible := lyricsTitleSimilarity(target, value)
		if !compatible {
			score = lyricsTextSimilarity(normalizeLyricsMatchText(target), normalizeLyricsMatchText(value))
		}
		if score > bestScore {
			best = strings.TrimSpace(value)
			bestScore = score
		}
	}
	return best
}

func bestAMLLText(values []string, target string) string {
	best := firstAMLLString(values)
	if strings.TrimSpace(target) == "" {
		return best
	}
	bestScore := -1.0
	for _, value := range values {
		score := lyricsTextSimilarity(normalizeLyricsMatchText(target), normalizeLyricsMatchText(value))
		if score > bestScore {
			best = strings.TrimSpace(value)
			bestScore = score
		}
	}
	return best
}

func cloneAMLLRecords(records []amllRecord) []amllRecord {
	if len(records) == 0 {
		return nil
	}
	clone := make([]amllRecord, len(records))
	copy(clone, records)
	return clone
}

func readAMLLBoundedBody(reader io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("response exceeds %d-byte limit", limit)
	}
	return data, nil
}
