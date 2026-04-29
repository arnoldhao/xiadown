package ytdlp

var metadataWhitelist = map[string]struct{}{
	"id":                   {},
	"display_id":           {},
	"title":                {},
	"fulltitle":            {},
	"description":          {},
	"uploader":             {},
	"uploader_id":          {},
	"uploader_url":         {},
	"channel":              {},
	"channel_id":           {},
	"channel_url":          {},
	"creator":              {},
	"creator_url":          {},
	"artist":               {},
	"artist_id":            {},
	"artist_url":           {},
	"extractor":            {},
	"extractor_key":        {},
	"webpage_url":          {},
	"original_url":         {},
	"webpage_url_domain":   {},
	"webpage_url_basename": {},
	"thumbnail":            {},
	"thumbnails":           {},
	"duration":             {},
	"duration_string":      {},
	"upload_date":          {},
	"timestamp":            {},
	"release_year":         {},
}

func filterMetadata(payload map[string]any) map[string]any {
	if len(payload) == 0 {
		return nil
	}
	filtered := make(map[string]any)
	for key, value := range payload {
		if _, ok := metadataWhitelist[key]; !ok {
			continue
		}
		if value == nil {
			continue
		}
		filtered[key] = value
	}
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}

func collectMetadata(info map[string]any) []map[string]any {
	if info == nil {
		return nil
	}
	result := make([]map[string]any, 0, 1)
	seen := map[string]struct{}{}
	walkInfo(info, func(entry map[string]any) {
		filtered := filterMetadata(entry)
		if len(filtered) == 0 {
			return
		}
		raw := string(mustJSON(filtered))
		if _, ok := seen[raw]; ok {
			return
		}
		seen[raw] = struct{}{}
		result = append(result, filtered)
	})
	return result
}
