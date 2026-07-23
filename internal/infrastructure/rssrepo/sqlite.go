package rssrepo

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode"

	domainrss "xiadown/internal/domain/rss"

	"github.com/uptrace/bun"
)

type SQLiteRepository struct {
	db                     *bun.DB
	collectionMutationMu   sync.Mutex
	sharedPublicMutationMu sync.Mutex
}

const (
	maxLegacySyncSubscriptionRows = 2000
	// The synchronization protocol advertises retainedFrom and reset_required,
	// so a bounded journal is safe: clients older than the retained cursor rebuild
	// from a snapshot instead of making the desktop database grow forever.
	maxRetainedRSSChanges          = 50_000
	maxRetainedRSSMutationReceipts = 50_000
	rssMutationReceiptTTL          = 30 * 24 * time.Hour
	rssSyncJournalPruneInterval    = int64(256)
)

func NewSQLiteRepository(db *bun.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

type subscriptionRow struct {
	bun.BaseModel    `bun:"table:rss_subscriptions"`
	ID               string     `bun:"id,pk"`
	WorkspaceID      string     `bun:"workspace_id"`
	FeedURL          string     `bun:"feed_url"`
	SourceAccess     string     `bun:"source_access"`
	PublicFeedURL    string     `bun:"public_feed_url"`
	SiteURL          string     `bun:"site_url"`
	Title            string     `bun:"title"`
	Description      string     `bun:"description"`
	IconURL          string     `bun:"icon_url"`
	ViewType         string     `bun:"view_type"`
	ResolvedViewType string     `bun:"resolved_view_type,scanonly"`
	CategoryID       *string    `bun:"category_id"`
	SortOrder        int        `bun:"sort_order"`
	Enabled          bool       `bun:"enabled"`
	UnreadCount      int        `bun:"unread_count,scanonly"`
	ETag             string     `bun:"etag"`
	LastModified     string     `bun:"last_modified"`
	ValidatorURL     string     `bun:"validator_url"`
	LastFetchedAt    *time.Time `bun:"last_fetched_at"`
	LastSuccessAt    *time.Time `bun:"last_success_at"`
	LastError        string     `bun:"last_error"`
	CreatedAt        time.Time  `bun:"created_at"`
	UpdatedAt        time.Time  `bun:"updated_at"`
	Revision         int64      `bun:"revision"`
}

type lightweightSyncSubscriptionRow struct {
	bun.BaseModel `bun:"table:rss_subscriptions,alias:subscription_row"`
	ID            string                             `bun:"id"`
	WorkspaceID   string                             `bun:"workspace_id"`
	Title         string                             `bun:"title"`
	Description   string                             `bun:"description"`
	IconAvailable bool                               `bun:"icon_available"`
	ViewType      domainrss.ViewType                 `bun:"view_type"`
	CategoryID    *string                            `bun:"category_id"`
	SortOrder     int                                `bun:"sort_order"`
	Enabled       bool                               `bun:"enabled"`
	UnreadCount   int                                `bun:"unread_count"`
	CreatedAt     time.Time                          `bun:"created_at"`
	UpdatedAt     time.Time                          `bun:"updated_at"`
	Revision      int64                              `bun:"revision"`
	SourceAccess  domainrss.SubscriptionSourceAccess `bun:"source_access"`
	PublicFeedURL string                             `bun:"public_feed_url"`
}

type lightweightSyncEntryRow struct {
	bun.BaseModel                  `bun:"table:rss_entries,alias:entry_row"`
	ID                             string              `bun:"id"`
	SubscriptionID                 string              `bun:"subscription_id"`
	Title                          string              `bun:"title"`
	Author                         string              `bun:"author"`
	Summary                        string              `bun:"summary"`
	Kind                           domainrss.EntryKind `bun:"kind"`
	ThumbnailAvailable             bool                `bun:"thumbnail_available"`
	Platform                       string              `bun:"platform"`
	PlatformVideoID                string              `bun:"platform_video_id"`
	PublishedAt                    *time.Time          `bun:"published_at"`
	SourceUpdatedAt                *time.Time          `bun:"source_updated_at"`
	ReadAt                         *time.Time          `bun:"read_at"`
	StarredAt                      *time.Time          `bun:"starred_at"`
	ArticleProgressFraction        *float64            `bun:"article_progress_fraction"`
	ArticleProgressAnchor          string              `bun:"article_progress_anchor"`
	ArticleProgressContentRevision *int64              `bun:"article_progress_content_revision"`
	VideoProgressSeconds           *float64            `bun:"video_progress_seconds"`
	VideoDurationSeconds           *float64            `bun:"video_duration_seconds"`
	VideoCompleted                 bool                `bun:"video_completed"`
	ReadRevision                   int64               `bun:"read_revision"`
	StarredRevision                int64               `bun:"starred_revision"`
	ArticleProgressRevision        int64               `bun:"article_progress_revision"`
	VideoProgressSecondsRevision   int64               `bun:"video_progress_seconds_revision"`
	StateRevision                  int64               `bun:"state_revision"`
	ContentRevision                int64               `bun:"revision"`
	CreatedAt                      time.Time           `bun:"created_at"`
	ModifiedAt                     time.Time           `bun:"modified_at"`
}

type entryRow struct {
	bun.BaseModel                  `bun:"table:rss_entries"`
	ID                             string     `bun:"id,pk"`
	SubscriptionID                 string     `bun:"subscription_id"`
	ExternalID                     string     `bun:"external_id"`
	URL                            string     `bun:"url"`
	Title                          string     `bun:"title"`
	Author                         string     `bun:"author"`
	Summary                        string     `bun:"summary"`
	ContentHTML                    string     `bun:"content_html"`
	Kind                           string     `bun:"kind"`
	ImageURLsJSON                  string     `bun:"image_urls_json"`
	MediaJSON                      string     `bun:"media_json"`
	MediaURL                       string     `bun:"media_url"`
	MediaType                      string     `bun:"media_type"`
	ThumbnailURL                   string     `bun:"thumbnail_url"`
	Platform                       string     `bun:"platform"`
	PlatformVideoID                string     `bun:"platform_video_id"`
	PublishedAt                    *time.Time `bun:"published_at"`
	SourceUpdatedAt                *time.Time `bun:"source_updated_at"`
	ReadAt                         *time.Time `bun:"read_at"`
	StarredAt                      *time.Time `bun:"starred_at"`
	ArticleProgressFraction        *float64   `bun:"article_progress_fraction"`
	ArticleProgressAnchor          string     `bun:"article_progress_anchor"`
	ArticleProgressContentRevision *int64     `bun:"article_progress_content_revision"`
	VideoProgressSeconds           *float64   `bun:"video_progress_seconds"`
	VideoDurationSeconds           *float64   `bun:"video_duration_seconds"`
	VideoCompleted                 bool       `bun:"video_completed"`
	ReadRevision                   int64      `bun:"read_revision"`
	StarredRevision                int64      `bun:"starred_revision"`
	ArticleProgressRevision        int64      `bun:"article_progress_revision"`
	VideoProgressSecondsRevision   int64      `bun:"video_progress_seconds_revision"`
	StateRevision                  int64      `bun:"state_revision"`
	ReadStateUpdatedAt             *time.Time `bun:"read_state_updated_at"`
	ReadStateDeviceID              string     `bun:"read_state_device_id"`
	ReadStateSubjectID             string     `bun:"read_state_subject_id"`
	Revision                       int64      `bun:"revision"`
	ContentHash                    string     `bun:"content_hash"`
	CreatedAt                      time.Time  `bun:"created_at"`
	ModifiedAt                     time.Time  `bun:"modified_at"`
}

type changeRow struct {
	bun.BaseModel `bun:"table:rss_changes"`
	Sequence      int64     `bun:"sequence,pk,autoincrement"`
	WorkspaceID   string    `bun:"workspace_id"`
	SubjectID     string    `bun:"subject_id,nullzero"`
	EntityType    string    `bun:"entity_type"`
	EntityID      string    `bun:"entity_id"`
	Operation     string    `bun:"operation"`
	Revision      int64     `bun:"revision"`
	PayloadJSON   string    `bun:"payload_json"`
	ChangedAt     time.Time `bun:"changed_at"`
}

type tombstoneRow struct {
	bun.BaseModel   `bun:"table:rss_tombstones"`
	WorkspaceID     string    `bun:"workspace_id,pk"`
	EntityType      string    `bun:"entity_type,pk"`
	EntityID        string    `bun:"entity_id,pk"`
	DeletedSequence int64     `bun:"deleted_sequence"`
	DeletedAt       time.Time `bun:"deleted_at"`
}

type mutationRow struct {
	bun.BaseModel `bun:"table:rss_client_mutations"`
	DeviceID      string    `bun:"device_id,pk"`
	MutationID    string    `bun:"mutation_id,pk"`
	EntryID       string    `bun:"entry_id"`
	RequestHash   string    `bun:"request_hash"`
	ResultJSON    string    `bun:"result_json"`
	CreatedAt     time.Time `bun:"created_at"`
}

type publicMutationRow struct {
	bun.BaseModel `bun:"table:rss_public_mutations"`
	DeviceID      string    `bun:"device_id,pk"`
	MutationID    string    `bun:"mutation_id,pk"`
	MutationKind  string    `bun:"mutation_kind"`
	RequestHash   string    `bun:"request_hash"`
	ResultJSON    string    `bun:"result_json"`
	CreatedAt     time.Time `bun:"created_at"`
}

type fetchLeaseRow struct {
	bun.BaseModel  `bun:"table:rss_fetch_leases"`
	SubscriptionID string    `bun:"subscription_id,pk"`
	LeaseID        string    `bun:"lease_id"`
	DeviceID       string    `bun:"device_id"`
	AcquiredAt     time.Time `bun:"acquired_at"`
	ExpiresAt      time.Time `bun:"expires_at"`
}

type entryOriginRow struct {
	bun.BaseModel  `bun:"table:rss_entry_origins"`
	SubscriptionID string    `bun:"subscription_id,pk"`
	OriginKey      string    `bun:"origin_key,pk"`
	EntryID        string    `bun:"entry_id"`
	LastObservedAt time.Time `bun:"last_observed_at"`
}

type observationSourceRow struct {
	bun.BaseModel  `bun:"table:rss_observation_sources"`
	SubscriptionID string    `bun:"subscription_id,pk"`
	DeviceID       string    `bun:"device_id,pk"`
	UpstreamETag   string    `bun:"upstream_etag"`
	LastModified   string    `bun:"last_modified"`
	ContentHash    string    `bun:"content_hash"`
	FetchedAt      time.Time `bun:"fetched_at"`
	AcceptedAt     time.Time `bun:"accepted_at"`
}

type discoveryMetaRow struct {
	bun.BaseModel `bun:"table:rss_discovery_meta"`
	Source        string    `bun:"source,pk"`
	SourceURL     string    `bun:"source_url"`
	FetchedAt     time.Time `bun:"fetched_at"`
	RouteCount    int       `bun:"route_count"`
	UpdatedAt     time.Time `bun:"updated_at"`
}

type discoveryRouteRow struct {
	bun.BaseModel     `bun:"table:rss_discovery_routes"`
	ID                string `bun:"id,pk"`
	Provider          string `bun:"provider"`
	Title             string `bun:"title"`
	URL               string `bun:"url"`
	Description       string `bun:"description"`
	SourceID          string `bun:"source_id"`
	SourceName        string `bun:"source_name"`
	SourceURL         string `bun:"source_url"`
	SiteURL           string `bun:"site_url"`
	RoutePath         string `bun:"route_path"`
	ExamplePath       string `bun:"example_path"`
	CategoriesJSON    string `bun:"categories_json"`
	Heat              int    `bun:"heat"`
	Language          string `bun:"language"`
	Region            string `bun:"region"`
	ViewType          string `bun:"view_type"`
	RequiresConfig    bool   `bun:"requires_config"`
	RequiresPuppeteer bool   `bun:"requires_puppeteer"`
	NeedsParameters   bool   `bun:"needs_parameters"`
	ParametersJSON    string `bun:"parameters_json"`
}

type discoveryCategoryAggregateRow struct {
	CategoryID    string `bun:"category_id"`
	CategoryCount int    `bun:"category_count"`
	Example       string `bun:"example"`
}

func (repo *SQLiteRepository) GetDiscoveryState(ctx context.Context) (domainrss.DiscoveryState, error) {
	if repo == nil || repo.db == nil {
		return domainrss.DiscoveryState{}, errors.New("rss repository unavailable")
	}
	meta := discoveryMetaRow{Source: "rsshub"}
	if err := repo.db.NewSelect().Model(&meta).WherePK().Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domainrss.DiscoveryState{}, nil
		}
		return domainrss.DiscoveryState{}, err
	}
	return discoveryStateFromMeta(meta), nil
}

func (repo *SQLiteRepository) QueryDiscovery(ctx context.Context, query domainrss.DiscoveryQuery) (domainrss.DiscoveryPage, error) {
	if repo == nil || repo.db == nil {
		return domainrss.DiscoveryPage{}, errors.New("rss repository unavailable")
	}
	query.Query = strings.ToLower(strings.TrimSpace(query.Query))
	query.RouteID = strings.TrimSpace(query.RouteID)
	query.CategoryID = strings.TrimSpace(query.CategoryID)
	query.Language = strings.TrimSpace(query.Language)
	query.Offset = max(query.Offset, 0)
	if query.Limit <= 0 {
		query.Limit = 80
	}
	query.Limit = min(query.Limit, 200)

	page := domainrss.DiscoveryPage{
		Categories: []domainrss.DiscoveryCategory{}, Routes: []domainrss.DiscoveryRoute{},
		Offset: query.Offset, Limit: query.Limit,
	}
	err := repo.db.RunInTx(ctx, &sql.TxOptions{ReadOnly: true}, func(ctx context.Context, tx bun.Tx) error {
		meta := discoveryMetaRow{Source: "rsshub"}
		if err := tx.NewSelect().Model(&meta).WherePK().Scan(ctx); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				page.Categories = []domainrss.DiscoveryCategory{{ID: "all", Examples: []string{}}}
				return nil
			}
			return err
		}
		page.State = discoveryStateFromMeta(meta)

		languageWhere, languageArgs := discoveryLanguageWhere(query.Language)
		var languageCount int
		if err := tx.NewRaw("SELECT COUNT(*) FROM rss_discovery_routes AS r"+languageWhere, languageArgs...).Scan(ctx, &languageCount); err != nil {
			return err
		}
		categories, err := queryDiscoveryCategories(ctx, tx, languageWhere, languageArgs, languageCount)
		if err != nil {
			return err
		}
		page.Categories = categories

		where, args := discoveryRouteWhere(query)
		if err := tx.NewRaw("SELECT COUNT(*) FROM rss_discovery_routes AS r"+where, args...).Scan(ctx, &page.FilteredRouteCount); err != nil {
			return err
		}

		orderBy, orderArgs := discoveryRouteOrderBy(query)
		rows := make([]discoveryRouteRow, 0, query.Limit)
		pageSQL := `SELECT r.id, r.provider, r.title, r.url, r.description,
	r.source_id, r.source_name, r.source_url, r.site_url, r.route_path,
	r.example_path, r.categories_json, r.heat, r.language, r.region,
	r.view_type, r.requires_config, r.requires_puppeteer,
	r.needs_parameters, r.parameters_json
FROM rss_discovery_routes AS r` + where + orderBy + " LIMIT ? OFFSET ?"
		pageArgs := append(append(append([]any(nil), args...), orderArgs...), query.Limit, query.Offset)
		if err := tx.NewRaw(pageSQL, pageArgs...).Scan(ctx, &rows); err != nil {
			return err
		}
		page.Routes = make([]domainrss.DiscoveryRoute, 0, len(rows))
		for _, row := range rows {
			page.Routes = append(page.Routes, discoveryRouteFromRow(row))
		}
		page.HasMore = query.Offset+len(page.Routes) < page.FilteredRouteCount
		return nil
	})
	if err != nil {
		return domainrss.DiscoveryPage{}, err
	}
	return page, nil
}

// FindDiscoveryRoute is the bounded lookup used by the desktop favicon proxy.
// It deliberately skips the category aggregation and result counts performed
// by QueryDiscovery, since every image request needs only one persisted route.
func (repo *SQLiteRepository) FindDiscoveryRoute(ctx context.Context, query domainrss.DiscoveryQuery) (domainrss.DiscoveryRoute, error) {
	if repo == nil || repo.db == nil {
		return domainrss.DiscoveryRoute{}, errors.New("rss repository unavailable")
	}
	query = domainrss.DiscoveryQuery{
		RouteID: strings.TrimSpace(query.RouteID), CategoryID: strings.TrimSpace(query.CategoryID),
		Sort: "popular", Limit: 1,
	}
	if (query.RouteID == "") == (query.CategoryID == "") {
		return domainrss.DiscoveryRoute{}, domainrss.ErrNotFound
	}
	where, args := discoveryRouteWhere(query)
	orderBy, orderArgs := discoveryRouteOrderBy(query)
	row := discoveryRouteRow{}
	const routeSQL = `SELECT r.id, r.provider, r.title, r.url, r.description,
	r.source_id, r.source_name, r.source_url, r.site_url, r.route_path,
	r.example_path, r.categories_json, r.heat, r.language, r.region,
	r.view_type, r.requires_config, r.requires_puppeteer,
	r.needs_parameters, r.parameters_json
FROM rss_discovery_routes AS r`
	if err := repo.db.NewRaw(routeSQL+where+orderBy+" LIMIT 1", append(args, orderArgs...)...).Scan(ctx, &row); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domainrss.DiscoveryRoute{}, domainrss.ErrNotFound
		}
		return domainrss.DiscoveryRoute{}, err
	}
	return discoveryRouteFromRow(row), nil
}

func discoveryStateFromMeta(meta discoveryMetaRow) domainrss.DiscoveryState {
	return domainrss.DiscoveryState{
		SourceURL: meta.SourceURL, FetchedAt: meta.FetchedAt.UTC(), RouteCount: meta.RouteCount,
	}
}

func discoveryLanguageWhere(language string) (string, []any) {
	if language == "" {
		return "", nil
	}
	return " WHERE r.language = ?", []any{language}
}

func discoveryRouteWhere(query domainrss.DiscoveryQuery) (string, []any) {
	clauses := make([]string, 0, 4)
	args := make([]any, 0, 12)
	if query.RouteID != "" {
		clauses = append(clauses, "r.id = ?")
		args = append(args, query.RouteID)
	}
	if query.Language != "" {
		clauses = append(clauses, "r.language = ?")
		args = append(args, query.Language)
	}
	if query.CategoryID != "" && query.CategoryID != "all" {
		clauses = append(clauses, `EXISTS (
	SELECT 1 FROM json_each(CASE WHEN json_valid(r.categories_json) THEN r.categories_json ELSE '[]' END) AS category
	WHERE category.type = 'text' AND category.value = ?
)`)
		args = append(args, query.CategoryID)
	}
	if query.Query != "" {
		tokenClauses := make([]string, 0, 12)
		for _, token := range discoveryQueryTokens(query.Query) {
			searchClauses := make([]string, 0, 8)
			for _, column := range []string{"title", "description", "source_name", "source_url", "site_url", "url", "route_path"} {
				searchClauses = append(searchClauses, "instr(lower(coalesce(r."+column+", '')), ?) > 0")
				args = append(args, token)
			}
			searchClauses = append(searchClauses, `EXISTS (
	SELECT 1 FROM json_each(CASE WHEN json_valid(r.categories_json) THEN r.categories_json ELSE '[]' END) AS search_category
	WHERE search_category.type = 'text' AND instr(lower(CAST(search_category.value AS TEXT)), ?) > 0
)`)
			args = append(args, token)
			tokenClauses = append(tokenClauses, "("+strings.Join(searchClauses, " OR ")+")")
		}
		if len(tokenClauses) > 0 {
			// Match any token for broad discovery recall. Relevance ordering below
			// rewards routes that match the complete phrase or every token.
			clauses = append(clauses, "("+strings.Join(tokenClauses, " OR ")+")")
		} else {
			clauses = append(clauses, "1 = 0")
		}
	}
	if len(clauses) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

func discoveryQueryTokens(query string) []string {
	parts := strings.FieldsFunc(strings.ToLower(strings.TrimSpace(query)), func(character rune) bool {
		return !(unicode.IsLetter(character) || unicode.IsDigit(character))
	})
	result := make([]string, 0, min(len(parts), 12))
	seen := make(map[string]struct{}, cap(result))
	for _, part := range parts {
		if part == "" {
			continue
		}
		if _, duplicate := seen[part]; duplicate {
			continue
		}
		seen[part] = struct{}{}
		result = append(result, part)
		if len(result) == 12 {
			break
		}
	}
	return result
}

func discoveryRouteOrderBy(query domainrss.DiscoveryQuery) (string, []any) {
	if strings.EqualFold(strings.TrimSpace(query.Sort), "title") {
		return " ORDER BY r.title COLLATE NOCASE ASC, r.id ASC", nil
	}
	phrase := strings.ToLower(strings.TrimSpace(query.Query))
	tokens := discoveryQueryTokens(phrase)
	if phrase == "" || len(tokens) == 0 {
		return " ORDER BY r.heat DESC, r.title COLLATE NOCASE ASC, r.id ASC", nil
	}

	parts := make([]string, 0, 8+len(tokens)*5)
	args := make([]any, 0, 8+len(tokens)*5)
	for _, field := range []struct {
		column string
		weight int
	}{
		{"title", 960}, {"source_name", 800}, {"route_path", 560},
		{"url", 480}, {"description", 320}, {"source_url", 240}, {"site_url", 240},
	} {
		parts = append(parts, fmt.Sprintf("CASE WHEN instr(lower(coalesce(r.%s, '')), ?) > 0 THEN %d ELSE 0 END", field.column, field.weight))
		args = append(args, phrase)
	}
	for _, token := range tokens {
		for _, field := range []struct {
			column string
			weight int
		}{
			{"title", 24}, {"source_name", 20}, {"route_path", 14},
			{"url", 12}, {"description", 8}, {"site_url", 6},
		} {
			parts = append(parts, fmt.Sprintf("CASE WHEN instr(lower(coalesce(r.%s, '')), ?) > 0 THEN %d ELSE 0 END", field.column, field.weight))
			args = append(args, token)
		}
	}
	return " ORDER BY (" + strings.Join(parts, " + ") + ") DESC, r.heat DESC, r.title COLLATE NOCASE ASC, r.id ASC", args
}

func queryDiscoveryCategories(ctx context.Context, tx bun.Tx, languageWhere string, languageArgs []any, languageCount int) ([]domainrss.DiscoveryCategory, error) {
	allExamples := make([]string, 0, 2)
	if err := tx.NewRaw(
		"SELECT r.title FROM rss_discovery_routes AS r"+languageWhere+" ORDER BY r.heat DESC, r.title COLLATE NOCASE ASC, r.id ASC LIMIT 2",
		languageArgs...,
	).Scan(ctx, &allExamples); err != nil {
		return nil, err
	}

	languagePredicate := ""
	if len(languageArgs) > 0 {
		languagePredicate = " AND r.language = ?"
	}
	categorySQL := `WITH expanded AS (
	SELECT CAST(category.value AS TEXT) AS category_id, r.title, r.heat, r.id
	FROM rss_discovery_routes AS r
	JOIN json_each(CASE WHEN json_valid(r.categories_json) THEN r.categories_json ELSE '[]' END) AS category
	WHERE category.type = 'text' AND trim(CAST(category.value AS TEXT)) <> ''` + languagePredicate + `
), category_counts AS (
	SELECT category_id, COUNT(*) AS category_count FROM expanded GROUP BY category_id
), example_candidates AS (
	SELECT category_id, title, MAX(heat) AS heat, MIN(id) AS id
	FROM expanded GROUP BY category_id, title
), ranked AS (
	SELECT category_id, title,
		ROW_NUMBER() OVER (PARTITION BY category_id ORDER BY heat DESC, title COLLATE NOCASE ASC, id ASC) AS rank
	FROM example_candidates
)
SELECT ranked.category_id, category_counts.category_count, ranked.title AS example
FROM ranked JOIN category_counts USING (category_id)
WHERE ranked.rank <= 2
ORDER BY ranked.category_id ASC, ranked.rank ASC`
	rows := make([]discoveryCategoryAggregateRow, 0)
	if err := tx.NewRaw(categorySQL, languageArgs...).Scan(ctx, &rows); err != nil {
		return nil, err
	}
	result := []domainrss.DiscoveryCategory{{ID: "all", Count: languageCount, Examples: allExamples}}
	byID := make(map[string]int, len(rows))
	for _, row := range rows {
		index, ok := byID[row.CategoryID]
		if !ok {
			index = len(result)
			byID[row.CategoryID] = index
			result = append(result, domainrss.DiscoveryCategory{
				ID: row.CategoryID, Count: row.CategoryCount, Examples: []string{},
			})
		}
		if row.Example != "" && len(result[index].Examples) < 2 {
			result[index].Examples = append(result[index].Examples, row.Example)
		}
	}
	return result, nil
}

func (repo *SQLiteRepository) LoadDiscoveryCache(ctx context.Context) (domainrss.DiscoveryCache, error) {
	if repo == nil || repo.db == nil {
		return domainrss.DiscoveryCache{}, errors.New("rss repository unavailable")
	}
	meta := discoveryMetaRow{Source: "rsshub"}
	if err := repo.db.NewSelect().Model(&meta).WherePK().Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domainrss.DiscoveryCache{Routes: []domainrss.DiscoveryRoute{}}, nil
		}
		return domainrss.DiscoveryCache{}, err
	}
	rows := make([]discoveryRouteRow, 0, meta.RouteCount)
	if err := repo.db.NewSelect().Model(&rows).
		OrderExpr("heat DESC, title COLLATE NOCASE ASC, id ASC").Scan(ctx); err != nil {
		return domainrss.DiscoveryCache{}, err
	}
	routes := make([]domainrss.DiscoveryRoute, 0, len(rows))
	for _, row := range rows {
		routes = append(routes, discoveryRouteFromRow(row))
	}
	return domainrss.DiscoveryCache{
		Routes: routes, SourceURL: meta.SourceURL, FetchedAt: meta.FetchedAt.UTC(),
	}, nil
}

func (repo *SQLiteRepository) ReplaceDiscoveryCache(ctx context.Context, cache domainrss.DiscoveryCache) error {
	if repo == nil || repo.db == nil {
		return errors.New("rss repository unavailable")
	}
	fetchedAt := cache.FetchedAt.UTC()
	if fetchedAt.IsZero() {
		return errors.New("rss discovery fetched time is required")
	}
	rows := make([]discoveryRouteRow, 0, len(cache.Routes))
	for _, route := range cache.Routes {
		rows = append(rows, discoveryRouteToRow(route))
	}
	return repo.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewDelete().Model((*discoveryRouteRow)(nil)).Where("1 = 1").Exec(ctx); err != nil {
			return err
		}
		const batchSize = 100
		for start := 0; start < len(rows); start += batchSize {
			end := min(start+batchSize, len(rows))
			batch := rows[start:end]
			if _, err := tx.NewInsert().Model(&batch).Exec(ctx); err != nil {
				return fmt.Errorf("insert RSS discovery route batch: %w", err)
			}
		}
		meta := discoveryMetaRow{
			Source: "rsshub", SourceURL: cache.SourceURL, FetchedAt: fetchedAt,
			RouteCount: len(rows), UpdatedAt: fetchedAt,
		}
		_, err := tx.NewInsert().Model(&meta).
			On("CONFLICT (source) DO UPDATE").
			Set("source_url = EXCLUDED.source_url").
			Set("fetched_at = EXCLUDED.fetched_at").
			Set("route_count = EXCLUDED.route_count").
			Set("updated_at = EXCLUDED.updated_at").Exec(ctx)
		return err
	})
}

func (repo *SQLiteRepository) ListSubscriptions(ctx context.Context) ([]domainrss.Subscription, error) {
	if repo == nil || repo.db == nil {
		return nil, errors.New("rss repository unavailable")
	}
	rows := make([]subscriptionRow, 0)
	err := selectSubscriptionColumns(repo.db.NewSelect().Model(&rows)).
		Where("subscription_row.workspace_id = ?", domainrss.DefaultWorkspaceID).
		Where("NOT EXISTS (SELECT 1 FROM rss_sources source WHERE source.subscription_id = subscription_row.id)").
		OrderExpr(`COALESCE((SELECT category.sort_order FROM rss_categories category WHERE category.id = subscription_row.category_id), 2147483647) ASC,
			subscription_row.sort_order ASC, subscription_row.title COLLATE NOCASE ASC, subscription_row.id ASC`).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]domainrss.Subscription, 0, len(rows))
	for _, row := range rows {
		items = append(items, subscriptionFromRow(row))
	}
	return items, nil
}

func selectSubscriptionColumns(query *bun.SelectQuery) *bun.SelectQuery {
	return query.
		Column("subscription_row.*").
		ColumnExpr("(SELECT COUNT(*) FROM rss_entries e WHERE e.subscription_id = subscription_row.id AND e.read_at IS NULL) AS unread_count").
		ColumnExpr(`CASE
			WHEN subscription_row.view_type <> 'auto' THEN subscription_row.view_type
			ELSE COALESCE((
				SELECT CASE
					WHEN COUNT(*) * 5 >= (
						SELECT COUNT(*) * 3
						FROM rss_entries all_entries
						WHERE all_entries.subscription_id = subscription_row.id
					) THEN dominant.kind
					ELSE 'auto'
				END
				FROM rss_entries dominant
				WHERE dominant.subscription_id = subscription_row.id
				GROUP BY dominant.kind
				ORDER BY COUNT(*) DESC, dominant.kind ASC
				LIMIT 1
			), 'auto')
		END AS resolved_view_type`)
}

func (repo *SQLiteRepository) ListLightweightSyncSubscriptions(ctx context.Context, limit int) ([]domainrss.SyncSubscription, error) {
	if repo == nil || repo.db == nil {
		return nil, errors.New("rss repository unavailable")
	}
	limit = normalizeLimit(limit, 100, maxLegacySyncSubscriptionRows)
	rows := make([]lightweightSyncSubscriptionRow, 0, limit)
	if err := applyPublicSyncSubscriptionEligibility(selectLightweightSyncSubscriptionColumns(
		repo.db.NewSelect().Model(&rows), "subscription_row",
	), "subscription_row").
		Where("subscription_row.workspace_id = ?", domainrss.DefaultWorkspaceID).
		OrderExpr("subscription_row.title COLLATE NOCASE ASC, subscription_row.id ASC").
		Limit(limit).Scan(ctx, &rows); err != nil {
		return nil, err
	}
	items := make([]domainrss.SyncSubscription, 0, len(rows))
	for _, row := range rows {
		items = append(items, syncSubscriptionFromLightweightRow(row))
	}
	return items, nil
}

func (repo *SQLiteRepository) GetSubscription(ctx context.Context, id string) (domainrss.Subscription, error) {
	row := subscriptionRow{ID: strings.TrimSpace(id)}
	err := selectSubscriptionColumns(repo.db.NewSelect().Model(&row)).
		Where("subscription_row.id = ?", row.ID).
		Where("subscription_row.workspace_id = ?", domainrss.DefaultWorkspaceID).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return domainrss.Subscription{}, domainrss.ErrNotFound
	}
	return subscriptionFromRow(row), err
}

func (repo *SQLiteRepository) GetSyncSubscription(ctx context.Context, id string) (domainrss.Subscription, error) {
	row := subscriptionRow{ID: strings.TrimSpace(id)}
	err := applyPublicSyncSubscriptionEligibility(selectSubscriptionColumns(repo.db.NewSelect().Model(&row)), "subscription_row").
		Where("subscription_row.id = ?", row.ID).
		Where("subscription_row.workspace_id = ?", domainrss.DefaultWorkspaceID).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return domainrss.Subscription{}, domainrss.ErrNotFound
	}
	return subscriptionFromRow(row), err
}

func applyPublicSyncSubscriptionEligibility(query *bun.SelectQuery, alias string) *bun.SelectQuery {
	prefix := strings.TrimSpace(alias)
	if prefix != "" {
		prefix += "."
	}
	return query.
		Where("NOT EXISTS (SELECT 1 FROM rss_sources source WHERE source.subscription_id = " + prefix + "id)").
		Where("NOT EXISTS (SELECT 1 FROM rss_tombstones marker WHERE marker.entity_type = 'local_subscription' AND marker.entity_id = " + prefix + "id)")
}

func (repo *SQLiteRepository) CreateSubscription(ctx context.Context, item domainrss.Subscription) (domainrss.Subscription, error) {
	if strings.TrimSpace(item.WorkspaceID) == "" {
		item.WorkspaceID = domainrss.DefaultWorkspaceID
	}
	row := subscriptionToRow(item)
	err := repo.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewInsert().Model(&row).Exec(ctx); err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "rss_subscriptions.feed_url") ||
				strings.Contains(strings.ToLower(err.Error()), "unique constraint") {
				return domainrss.ErrDuplicateFeed
			}
			return err
		}
		if err := initializeSubscriptionFieldRevisionsTx(ctx, tx, item.ID, item.Revision); err != nil {
			return err
		}
		return appendChange(ctx, tx, "subscription", item.ID, "upsert", item.Revision, syncSubscriptionProjection(item), item.UpdatedAt)
	})
	return item, err
}

// CreateFeed persists the subscription, its initial entries, and every sync
// change as one unit. A failed initial entry write must never leave behind a
// subscription that cannot be retried because of the feed URL uniqueness
// constraint.
func (repo *SQLiteRepository) CreateFeed(ctx context.Context, update domainrss.FeedUpdate) (domainrss.Subscription, domainrss.UpsertResult, error) {
	item := update.Subscription
	if strings.TrimSpace(item.WorkspaceID) == "" {
		item.WorkspaceID = domainrss.DefaultWorkspaceID
	}
	row := subscriptionToRow(item)
	result := domainrss.UpsertResult{}
	err := repo.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewInsert().Model(&row).Exec(ctx); err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "rss_subscriptions.feed_url") ||
				strings.Contains(strings.ToLower(err.Error()), "unique constraint") {
				return domainrss.ErrDuplicateFeed
			}
			return err
		}
		if err := initializeSubscriptionFieldRevisionsTx(ctx, tx, item.ID, item.Revision); err != nil {
			return err
		}
		entryChanges, err := upsertFeedEntries(ctx, tx, item.ID, update.Entries, &result)
		if err != nil {
			return err
		}
		if err := tx.NewSelect().Model((*entryRow)(nil)).
			ColumnExpr("COUNT(*)").
			Where("subscription_id = ?", item.ID).
			Where("read_at IS NULL").
			Scan(ctx, &item.UnreadCount); err != nil {
			return err
		}
		if err := appendChange(ctx, tx, "subscription", item.ID, "upsert", item.Revision, syncSubscriptionProjection(item), item.UpdatedAt); err != nil {
			return err
		}
		return appendPendingEntryChanges(ctx, tx, entryChanges)
	})
	if err != nil {
		return domainrss.Subscription{}, domainrss.UpsertResult{}, err
	}
	return item, result, nil
}

func (repo *SQLiteRepository) UpdateSubscription(ctx context.Context, item domainrss.Subscription) (domainrss.Subscription, error) {
	err := repo.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		existing := subscriptionRow{ID: strings.TrimSpace(item.ID)}
		if err := tx.NewSelect().Model(&existing).WherePK().Scan(ctx); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return domainrss.ErrNotFound
			}
			return err
		}
		desktopLocal, err := isDesktopLocalSourceSubscriptionTx(ctx, tx, existing.ID)
		if err != nil {
			return err
		}
		if item.Revision != existing.Revision+1 {
			return domainrss.ErrRevisionConflict
		}
		row := subscriptionToRow(item)
		contentChanged := subscriptionContentChanged(existing, row)
		if !contentChanged {
			// Conditional fetch bookkeeping is local operational state. Keeping the
			// public revision/timestamp stable prevents every 304 or failed poll from
			// generating a device-sync change that carries no new reader content.
			row.Revision = existing.Revision
			row.UpdatedAt = existing.UpdatedAt
			item.Revision = existing.Revision
			item.UpdatedAt = existing.UpdatedAt
		}
		result, err := tx.NewUpdate().Model(&row).
			Column("feed_url", "source_access", "public_feed_url", "site_url", "title", "description", "icon_url", "view_type", "category_id", "sort_order", "enabled", "etag", "last_modified", "validator_url", "last_fetched_at", "last_success_at", "last_error", "updated_at", "revision").
			Where("id = ?", row.ID).
			Where("revision = ?", existing.Revision).
			Exec(ctx)
		if err != nil {
			return err
		}
		count, _ := result.RowsAffected()
		if count == 0 {
			return domainrss.ErrRevisionConflict
		}
		if !contentChanged {
			return nil
		}
		if err := recordChangedSubscriptionFieldRevisionsTx(ctx, tx, existing, row); err != nil {
			return err
		}
		if desktopLocal {
			return nil
		}
		if err := tx.NewSelect().Model((*entryRow)(nil)).ColumnExpr("COUNT(*)").
			Where("subscription_id = ?", item.ID).Where("read_at IS NULL").Scan(ctx, &item.UnreadCount); err != nil {
			return err
		}
		return appendChange(ctx, tx, "subscription", item.ID, "upsert", item.Revision, syncSubscriptionProjection(item), item.UpdatedAt)
	})
	return item, err
}

func (repo *SQLiteRepository) DeleteSubscription(ctx context.Context, id string, changedAt time.Time) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return domainrss.ErrNotFound
	}
	return repo.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var revision int64
		if err := tx.NewSelect().Model((*subscriptionRow)(nil)).Column("revision").Where("id = ?", id).Scan(ctx, &revision); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return domainrss.ErrNotFound
			}
			return err
		}
		desktopLocal, err := isDesktopLocalSourceSubscriptionTx(ctx, tx, id)
		if err != nil {
			return err
		}
		if desktopLocal {
			if err := markDesktopLocalSourceSyncJournalTx(ctx, tx, id, changedAt); err != nil {
				return err
			}
			_, err := tx.NewDelete().Model((*subscriptionRow)(nil)).Where("id = ?", id).Exec(ctx)
			return err
		}
		if err := appendChange(ctx, tx, "subscription", id, "delete", revision+1, map[string]any{"id": id}, changedAt); err != nil {
			return err
		}
		var sequence int64
		if err := tx.NewSelect().Model((*changeRow)(nil)).ColumnExpr("MAX(sequence)").Scan(ctx, &sequence); err != nil {
			return err
		}
		if _, err := tx.NewInsert().Model(&tombstoneRow{
			WorkspaceID: domainrss.DefaultWorkspaceID, EntityType: "subscription",
			EntityID: id, DeletedSequence: sequence, DeletedAt: changedAt.UTC(),
		}).On("CONFLICT (workspace_id, entity_type, entity_id) DO UPDATE").
			Set("deleted_sequence = EXCLUDED.deleted_sequence").
			Set("deleted_at = EXCLUDED.deleted_at").Exec(ctx); err != nil {
			return err
		}
		_, err = tx.NewDelete().Model((*subscriptionRow)(nil)).Where("id = ?", id).Exec(ctx)
		return err
	})
}

func (repo *SQLiteRepository) UpsertFeed(ctx context.Context, update domainrss.FeedUpdate) (domainrss.UpsertResult, error) {
	result := domainrss.UpsertResult{}
	err := repo.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		existing := subscriptionRow{ID: strings.TrimSpace(update.Subscription.ID)}
		if err := tx.NewSelect().Model(&existing).
			Column("subscription_row.*").
			ColumnExpr("(SELECT COUNT(*) FROM rss_entries e WHERE e.subscription_id = subscription_row.id AND e.read_at IS NULL) AS unread_count").
			WherePK().Scan(ctx); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return domainrss.ErrNotFound
			}
			return err
		}
		desktopLocal, err := isDesktopLocalSourceSubscriptionTx(ctx, tx, existing.ID)
		if err != nil {
			return err
		}
		if update.Subscription.Revision != existing.Revision+1 {
			return domainrss.ErrRevisionConflict
		}
		incoming := subscriptionToRow(update.Subscription)
		contentChanged := subscriptionContentChanged(existing, incoming)
		entryChanges, err := upsertFeedEntries(ctx, tx, update.Subscription.ID, update.Entries, &result)
		if err != nil {
			return err
		}
		if err := tx.NewSelect().Model((*entryRow)(nil)).
			ColumnExpr("COUNT(*)").
			Where("subscription_id = ?", update.Subscription.ID).
			Where("read_at IS NULL").
			Scan(ctx, &update.Subscription.UnreadCount); err != nil {
			return err
		}
		contentChanged = contentChanged || update.Subscription.UnreadCount != existing.UnreadCount
		if !contentChanged {
			incoming.Revision = existing.Revision
			incoming.UpdatedAt = existing.UpdatedAt
			update.Subscription.Revision = existing.Revision
			update.Subscription.UpdatedAt = existing.UpdatedAt
		}
		updated, err := tx.NewUpdate().Model(&incoming).
			Column("site_url", "title", "description", "icon_url", "view_type", "category_id", "sort_order", "enabled", "etag", "last_modified", "validator_url", "last_fetched_at", "last_success_at", "last_error", "updated_at", "revision").
			Where("id = ?", incoming.ID).
			Where("revision = ?", existing.Revision).
			Exec(ctx)
		if err != nil {
			return err
		}
		count, _ := updated.RowsAffected()
		if count == 0 {
			return domainrss.ErrRevisionConflict
		}
		if contentChanged {
			if err := recordChangedSubscriptionFieldRevisionsTx(ctx, tx, existing, incoming); err != nil {
				return err
			}
		}
		if desktopLocal {
			return nil
		}
		if contentChanged {
			if err := appendChange(ctx, tx, "subscription", update.Subscription.ID, "upsert", update.Subscription.Revision, syncSubscriptionProjection(update.Subscription), update.Subscription.UpdatedAt); err != nil {
				return err
			}
		}
		if err := appendPendingEntryChanges(ctx, tx, entryChanges); err != nil {
			return err
		}
		return nil
	})
	return result, err
}

func isDesktopLocalSourceSubscriptionTx(ctx context.Context, tx bun.Tx, subscriptionID string) (bool, error) {
	var count int
	if err := tx.NewSelect().Model((*sourceRow)(nil)).ColumnExpr("COUNT(*)").
		Where("subscription_id = ?", strings.TrimSpace(subscriptionID)).
		Where("workspace_id = ?", domainrss.DefaultWorkspaceID).
		Scan(ctx, &count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func isDesktopLocalSyncEntryTx(ctx context.Context, tx bun.Tx, entryID, subscriptionID string) (bool, error) {
	var local bool
	if err := tx.NewSelect().ColumnExpr(`CASE WHEN
		EXISTS (SELECT 1 FROM rss_sources source WHERE source.subscription_id = ?)
		OR EXISTS (SELECT 1 FROM rss_tombstones marker WHERE marker.entity_type = 'local_entry' AND marker.entity_id = ?)
		OR EXISTS (SELECT 1 FROM rss_tombstones marker WHERE marker.entity_type = 'local_subscription' AND marker.entity_id = ?)
		THEN 1 ELSE 0 END`, strings.TrimSpace(subscriptionID), strings.TrimSpace(entryID), strings.TrimSpace(subscriptionID)).
		Scan(ctx, &local); err != nil {
		return false, err
	}
	return local, nil
}

func isDesktopLocalSyncEntryIDTx(ctx context.Context, tx bun.Tx, entryID string) (bool, error) {
	entryID = strings.TrimSpace(entryID)
	var local bool
	if err := tx.NewSelect().ColumnExpr(`CASE WHEN
		EXISTS (
			SELECT 1 FROM rss_entries entry
			JOIN rss_sources source ON source.subscription_id = entry.subscription_id
			WHERE entry.id = ?
		)
		OR EXISTS (SELECT 1 FROM rss_tombstones marker WHERE marker.entity_type = 'local_entry' AND marker.entity_id = ?)
		OR EXISTS (
			SELECT 1 FROM rss_entries entry
			JOIN rss_tombstones marker ON marker.entity_type = 'local_subscription' AND marker.entity_id = entry.subscription_id
			WHERE entry.id = ?
		)
		THEN 1 ELSE 0 END`, entryID, entryID, entryID).Scan(ctx, &local); err != nil {
		return false, err
	}
	return local, nil
}

// Legacy builds briefly journaled organization sources as ordinary public RSS
// entities. Preserve the journal sequences for cursor monotonicity, but leave
// durable local markers before the source/subscription cascade removes the
// relationships that identify those historical rows.
func markDesktopLocalSourceSyncJournalTx(ctx context.Context, tx bun.Tx, subscriptionID string, changedAt time.Time) error {
	var sequence int64
	if err := tx.NewSelect().Model((*changeRow)(nil)).ColumnExpr("COALESCE(MAX(sequence), 0)").Where(`
		(entity_type = 'subscription' AND entity_id = ?)
		OR (entity_type IN ('entry', 'entry_state') AND entity_id IN (
			SELECT entry.id FROM rss_entries entry WHERE entry.subscription_id = ?
		))
		OR (entity_type = 'download' AND entity_id IN (
			SELECT download.id
			FROM rss_entry_downloads download
			JOIN rss_entries entry ON entry.id = download.entry_id
			WHERE entry.subscription_id = ?
		))
	`, subscriptionID, subscriptionID, subscriptionID).Scan(ctx, &sequence); err != nil {
		return err
	}
	if sequence <= 0 {
		return nil
	}
	markers := []tombstoneRow{{
		WorkspaceID: domainrss.DefaultWorkspaceID, EntityType: "local_subscription",
		EntityID: subscriptionID, DeletedSequence: sequence, DeletedAt: changedAt.UTC(),
	}}
	entryRows := make([]struct {
		ID string `bun:"id"`
	}, 0)
	if err := tx.NewSelect().Model((*entryRow)(nil)).Column("id").
		Where("subscription_id = ?", subscriptionID).Scan(ctx, &entryRows); err != nil {
		return err
	}
	for _, entry := range entryRows {
		markers = append(markers, tombstoneRow{
			WorkspaceID: domainrss.DefaultWorkspaceID, EntityType: "local_entry",
			EntityID: entry.ID, DeletedSequence: sequence, DeletedAt: changedAt.UTC(),
		})
	}
	downloadRows := make([]struct {
		ID string `bun:"id"`
	}, 0)
	if err := tx.NewSelect().Table("rss_entry_downloads").Column("id").
		Where("entry_id IN (SELECT id FROM rss_entries WHERE subscription_id = ?)", subscriptionID).
		Scan(ctx, &downloadRows); err != nil {
		return err
	}
	for _, download := range downloadRows {
		markers = append(markers, tombstoneRow{
			WorkspaceID: domainrss.DefaultWorkspaceID, EntityType: "local_download",
			EntityID: download.ID, DeletedSequence: sequence, DeletedAt: changedAt.UTC(),
		})
	}
	for index := range markers {
		if _, err := tx.NewInsert().Model(&markers[index]).
			On("CONFLICT (workspace_id, entity_type, entity_id) DO UPDATE").
			Set("deleted_sequence = EXCLUDED.deleted_sequence").
			Set("deleted_at = EXCLUDED.deleted_at").Exec(ctx); err != nil {
			return err
		}
	}
	return nil
}

type pendingEntryChange struct {
	entityID  string
	revision  int64
	payload   domainrss.SyncEntry
	changedAt time.Time
}

func upsertFeedEntries(
	ctx context.Context,
	tx bun.Tx,
	subscriptionID string,
	entries []domainrss.Entry,
	result *domainrss.UpsertResult,
) ([]pendingEntryChange, error) {
	changes := make([]pendingEntryChange, 0, len(entries))
	for _, item := range entries {
		if item.SubscriptionID != subscriptionID {
			return nil, fmt.Errorf("RSS entry %q belongs to subscription %q, want %q", item.ID, item.SubscriptionID, subscriptionID)
		}
		row := entryToRow(item)
		existing := entryRow{}
		origin := entryOriginRow{}
		err := tx.NewSelect().Model(&existing).
			Where("subscription_id = ?", row.SubscriptionID).
			Where("external_id = ?", row.ExternalID).
			Scan(ctx)
		if err == nil && strings.TrimSpace(item.OriginKey) != "" {
			origin = entryOriginRow{SubscriptionID: subscriptionID, OriginKey: strings.TrimSpace(item.OriginKey)}
			originErr := tx.NewSelect().Model(&origin).WherePK().Scan(ctx)
			if originErr != nil && !errors.Is(originErr, sql.ErrNoRows) {
				return nil, originErr
			}
			if errors.Is(originErr, sql.ErrNoRows) {
				origin = entryOriginRow{}
			} else if origin.EntryID != existing.ID {
				// The versioned origin mapping is the canonical identity. A changed
				// fallback ExternalID (or a legacy alias collision) must never remap
				// an existing origin to a second entry.
				err = tx.NewSelect().Model(&existing).Where("id = ?", origin.EntryID).Scan(ctx)
				if err == nil {
					row.ExternalID = existing.ExternalID
					item.ExternalID = existing.ExternalID
				}
			}
		}
		if errors.Is(err, sql.ErrNoRows) && strings.TrimSpace(item.OriginKey) != "" {
			origin = entryOriginRow{SubscriptionID: subscriptionID, OriginKey: strings.TrimSpace(item.OriginKey)}
			originErr := tx.NewSelect().Model(&origin).WherePK().Scan(ctx)
			switch {
			case originErr == nil:
				err = tx.NewSelect().Model(&existing).Where("id = ?", origin.EntryID).Scan(ctx)
				if err == nil {
					row.ExternalID = existing.ExternalID
					item.ExternalID = existing.ExternalID
				}
			case !errors.Is(originErr, sql.ErrNoRows):
				return nil, originErr
			}
		}
		if err == nil {
			item.ID = existing.ID
		}
		if err == nil && origin.EntryID != "" && !item.ObservedAt.IsZero() && !item.ObservedAt.After(origin.LastObservedAt) {
			if err := upsertEntryOriginTx(ctx, tx, subscriptionID, item.OriginKey, existing.ID, origin.LastObservedAt); err != nil {
				return nil, err
			}
			continue
		}
		switch {
		case errors.Is(err, sql.ErrNoRows):
			row.Revision = 1
			item.Revision = 1
			if _, err := tx.NewInsert().Model(&row).Exec(ctx); err != nil {
				return nil, err
			}
			result.Created++
			changes = append(changes, pendingEntryChange{
				entityID: item.ID, revision: item.Revision,
				payload: syncEntryProjection(item), changedAt: item.ModifiedAt,
			})
		case err != nil:
			return nil, err
		case existing.ContentHash == row.ContentHash:
			// Content is unchanged, but a successful observation still advances
			// the origin freshness watermark used to reject late older payloads.
		default:
			row.ID = existing.ID
			item.ID = existing.ID
			row.CreatedAt = existing.CreatedAt
			item.CreatedAt = existing.CreatedAt
			row.ReadAt = existing.ReadAt
			row.StarredAt = existing.StarredAt
			row.ArticleProgressFraction = existing.ArticleProgressFraction
			row.ArticleProgressAnchor = existing.ArticleProgressAnchor
			row.ArticleProgressContentRevision = existing.ArticleProgressContentRevision
			row.VideoProgressSeconds = existing.VideoProgressSeconds
			row.VideoDurationSeconds = existing.VideoDurationSeconds
			row.VideoCompleted = existing.VideoCompleted
			row.ReadRevision = existing.ReadRevision
			row.StarredRevision = existing.StarredRevision
			row.ArticleProgressRevision = existing.ArticleProgressRevision
			row.VideoProgressSecondsRevision = existing.VideoProgressSecondsRevision
			row.StateRevision = existing.StateRevision
			row.ReadStateUpdatedAt = existing.ReadStateUpdatedAt
			row.ReadStateDeviceID = existing.ReadStateDeviceID
			row.ReadStateSubjectID = existing.ReadStateSubjectID
			item.ReadAt = existing.ReadAt
			item.StarredAt = existing.StarredAt
			applyStateRowToEntry(&item, existing)
			item.StateRevision = existing.StateRevision
			item.ReadStateUpdatedAt = existing.ReadStateUpdatedAt
			row.Revision = existing.Revision + 1
			item.Revision = row.Revision
			if _, err := tx.NewUpdate().Model(&row).WherePK().Exec(ctx); err != nil {
				return nil, err
			}
			result.Updated++
			changes = append(changes, pendingEntryChange{
				entityID: item.ID, revision: item.Revision,
				payload: syncEntryProjection(item), changedAt: item.ModifiedAt,
			})
		}
		if strings.TrimSpace(item.OriginKey) != "" {
			observedAt := item.ObservedAt
			if observedAt.IsZero() {
				observedAt = item.ModifiedAt
			}
			entryID := item.ID
			if existing.ID != "" && entryID == "" {
				entryID = existing.ID
			}
			if err := upsertEntryOriginTx(ctx, tx, subscriptionID, item.OriginKey, entryID, observedAt); err != nil {
				return nil, err
			}
		}
	}
	return changes, nil
}

func upsertEntryOriginTx(
	ctx context.Context,
	tx bun.Tx,
	subscriptionID, originKey, entryID string,
	observedAt time.Time,
) error {
	originKey = strings.TrimSpace(originKey)
	entryID = strings.TrimSpace(entryID)
	if originKey == "" || entryID == "" {
		return nil
	}
	row := entryOriginRow{
		SubscriptionID: subscriptionID, OriginKey: originKey, EntryID: entryID,
		LastObservedAt: observedAt.UTC(),
	}
	_, err := tx.NewInsert().Model(&row).On("CONFLICT (subscription_id, origin_key) DO UPDATE").
		Set("entry_id = EXCLUDED.entry_id").
		Set("last_observed_at = MAX(last_observed_at, EXCLUDED.last_observed_at)").
		Exec(ctx)
	return err
}

func appendPendingEntryChanges(ctx context.Context, tx bun.Tx, changes []pendingEntryChange) error {
	for _, change := range changes {
		if err := appendChange(ctx, tx, "entry", change.entityID, "upsert", change.revision, change.payload, change.changedAt); err != nil {
			return err
		}
	}
	return nil
}

func (repo *SQLiteRepository) ListEntries(ctx context.Context, request domainrss.EntryQuery) (domainrss.EntryPage, error) {
	request.Limit = normalizeLimit(request.Limit, 100, 500)
	if request.Offset < 0 {
		request.Offset = 0
	}
	var total int
	var snapshot int64
	rows := make([]entryRow, 0, request.Limit)
	// The backend refresher writes independently of the UI. Capture the change
	// high-water, filtered count, and page rows in one SQLite read snapshot so
	// clients never receive a new total paired with rows from an older view.
	err := repo.db.RunInTx(ctx, &sql.TxOptions{ReadOnly: true}, func(ctx context.Context, tx bun.Tx) error {
		if err := tx.NewSelect().Model((*changeRow)(nil)).
			ColumnExpr("COALESCE(MAX(sequence), 0)").
			Where("workspace_id = ?", domainrss.DefaultWorkspaceID).
			Scan(ctx, &snapshot); err != nil {
			return err
		}
		if err := applyEntryQuery(tx.NewSelect().Model((*entryRow)(nil)), request).
			ColumnExpr("COUNT(*)").Scan(ctx, &total); err != nil {
			return err
		}
		return applyEntryQuery(selectLightweightEntryColumns(tx.NewSelect().Model(&rows), "entry_row"), request).
			OrderExpr("COALESCE(published_at, source_updated_at, created_at) DESC, id DESC").
			Limit(request.Limit).Offset(request.Offset).Scan(ctx)
	})
	if err != nil {
		return domainrss.EntryPage{}, err
	}
	items := make([]domainrss.Entry, 0, len(rows))
	for _, row := range rows {
		items = append(items, entryFromRow(row))
	}
	page := domainrss.EntryPage{Items: items, Total: total, Snapshot: snapshot}
	if request.Offset+len(items) < total {
		page.NextOffset = request.Offset + len(items)
	}
	return page, nil
}

// ListSyncEntries is the paired-device list boundary. Its row type and SELECT
// intentionally cannot hydrate reusable source URLs, article bodies, or the
// image/media JSON arrays needed only by the Desktop layouts.
func (repo *SQLiteRepository) ListSyncEntries(ctx context.Context, request domainrss.EntryQuery) (domainrss.SyncEntryPage, error) {
	request.Limit = normalizeLimit(request.Limit, 100, 500)
	if request.Offset < 0 {
		request.Offset = 0
	}
	var total int
	if err := applyPublicSyncEntryQuery(repo.db.NewSelect().Model((*entryRow)(nil)), request).ColumnExpr("COUNT(*)").Scan(ctx, &total); err != nil {
		return domainrss.SyncEntryPage{}, err
	}
	rows := make([]lightweightSyncEntryRow, 0, request.Limit)
	if err := applyPublicSyncEntryQuery(selectLightweightSyncEntryColumns(repo.db.NewSelect().Model(&rows), "entry_row"), request).
		OrderExpr("COALESCE(published_at, source_updated_at, created_at) DESC, id DESC").
		Limit(request.Limit).Offset(request.Offset).Scan(ctx); err != nil {
		return domainrss.SyncEntryPage{}, err
	}
	items := make([]domainrss.SyncEntry, 0, len(rows))
	for _, row := range rows {
		items = append(items, syncEntryFromLightweightRow(row))
	}
	page := domainrss.SyncEntryPage{Items: items, Total: total}
	if request.Offset+len(items) < total {
		page.NextOffset = request.Offset + len(items)
	}
	return page, nil
}

func applyPublicSyncEntryQuery(query *bun.SelectQuery, request domainrss.EntryQuery) *bun.SelectQuery {
	return applyPublicSyncEntryEligibility(applyEntryQuery(query, request), "entry_row")
}

func applyPublicSyncEntryEligibility(query *bun.SelectQuery, alias string) *bun.SelectQuery {
	prefix := strings.TrimSpace(alias)
	if prefix != "" {
		prefix += "."
	}
	return query.
		Where("NOT EXISTS (SELECT 1 FROM rss_sources source WHERE source.subscription_id = " + prefix + "subscription_id)").
		Where("NOT EXISTS (SELECT 1 FROM rss_tombstones marker WHERE marker.entity_type = 'local_entry' AND marker.entity_id = " + prefix + "id)").
		Where("NOT EXISTS (SELECT 1 FROM rss_tombstones marker WHERE marker.entity_type = 'local_subscription' AND marker.entity_id = " + prefix + "subscription_id)")
}

func applyEntryQuery(query *bun.SelectQuery, request domainrss.EntryQuery) *bun.SelectQuery {
	query = query.Where("subscription_id IN (SELECT id FROM rss_subscriptions WHERE workspace_id = ?)", domainrss.DefaultWorkspaceID)
	if id := strings.TrimSpace(request.SubscriptionID); id != "" {
		query = query.Where("subscription_id = ?", id)
	}
	if id := strings.TrimSpace(request.CollectionID); id != "" {
		query = query.Where(`(
			(EXISTS (SELECT 1 FROM rss_collections collection WHERE collection.id = ? AND collection.workspace_id = ? AND collection.kind = 'subscriptions')
			 AND subscription_id IN (SELECT member.subscription_id FROM rss_collection_subscriptions member WHERE member.collection_id = ?))
			OR
			(EXISTS (SELECT 1 FROM rss_collections collection WHERE collection.id = ? AND collection.workspace_id = ? AND collection.kind = 'entries')
			 AND id IN (SELECT member.entry_id FROM rss_collection_entries member WHERE member.collection_id = ?))
		)`, id, domainrss.DefaultWorkspaceID, id, id, domainrss.DefaultWorkspaceID, id)
	}
	if id := strings.TrimSpace(request.CategoryID); id != "" {
		query = query.Where("subscription_id IN (SELECT id FROM rss_subscriptions WHERE workspace_id = ? AND category_id = ?)", domainrss.DefaultWorkspaceID, id)
	}
	if request.SourceKind != "" {
		query = query.Where("subscription_id IN (SELECT subscription_id FROM rss_sources WHERE workspace_id = ? AND kind = ?)", domainrss.DefaultWorkspaceID, string(request.SourceKind))
	} else if strings.TrimSpace(request.SubscriptionID) == "" &&
		strings.TrimSpace(request.CollectionID) == "" &&
		strings.TrimSpace(request.CategoryID) == "" {
		// Local Inbox/notification producers are a dormant compatibility layer,
		// not part of ordinary Desktop feed timelines. Explicit subscription,
		// collection, and category scopes remain addressable for compatibility.
		query = query.Where("subscription_id NOT IN (SELECT source.subscription_id FROM rss_sources source WHERE source.workspace_id = ?)", domainrss.DefaultWorkspaceID)
	}
	if request.Kind != "" {
		query = query.Where("kind = ?", string(request.Kind))
	}
	if request.UnreadOnly {
		query = query.Where("read_at IS NULL")
	}
	if request.StarredOnly {
		query = query.Where("starred_at IS NOT NULL")
	}
	if value := strings.TrimSpace(request.Query); value != "" {
		like := "%" + strings.ToLower(value) + "%"
		query = query.Where("(LOWER(title) LIKE ? OR LOWER(summary) LIKE ? OR LOWER(author) LIKE ?)", like, like, like)
	}
	return query
}

func (repo *SQLiteRepository) GetEntry(ctx context.Context, id string) (domainrss.Entry, error) {
	row := entryRow{ID: strings.TrimSpace(id)}
	err := repo.db.NewSelect().Model(&row).WherePK().
		Where("subscription_id IN (SELECT id FROM rss_subscriptions WHERE workspace_id = ?)", domainrss.DefaultWorkspaceID).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return domainrss.Entry{}, domainrss.ErrNotFound
	}
	return entryFromRow(row), err
}

func (repo *SQLiteRepository) GetSyncEntry(ctx context.Context, id string) (domainrss.Entry, error) {
	row := entryRow{ID: strings.TrimSpace(id)}
	err := applyPublicSyncEntryEligibility(
		repo.db.NewSelect().Model(&row).WherePK().
			Where("subscription_id IN (SELECT id FROM rss_subscriptions WHERE workspace_id = ?)", domainrss.DefaultWorkspaceID),
		"entry_row",
	).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return domainrss.Entry{}, domainrss.ErrNotFound
	}
	return entryFromRow(row), err
}

func (repo *SQLiteRepository) ApplyReadMutation(ctx context.Context, mutation domainrss.ReadMutation) (domainrss.EntryState, error) {
	read := mutation.Read
	requestHash, err := legacyReadMutationHash(mutation)
	if err != nil {
		return domainrss.EntryState{}, err
	}
	stateMutation := domainrss.StateMutation{
		Scope:   domainrss.SyncScope{WorkspaceID: domainrss.DefaultWorkspaceID, SubjectID: domainrss.DefaultSubjectID},
		EntryID: mutation.EntryID, Field: domainrss.EntryStateFieldRead, Read: &read,
		DeviceID: mutation.DeviceID, MutationID: mutation.MutationID,
		RequestHash: requestHash, ChangedAt: mutation.ChangedAt, AllowDesktopLocal: true,
	}
	return repo.applyStateMutation(ctx, stateMutation, mutation.ExpectedRevision)
}

func (repo *SQLiteRepository) ApplyStateMutation(ctx context.Context, mutation domainrss.StateMutation) (domainrss.EntryState, error) {
	expectedRevision := mutation.ExpectedRevision
	return repo.applyStateMutation(ctx, mutation, &expectedRevision)
}

func (repo *SQLiteRepository) applyStateMutation(
	ctx context.Context,
	mutation domainrss.StateMutation,
	expectedRevision *int64,
) (domainrss.EntryState, error) {
	var state domainrss.EntryState
	err := repo.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		scope, err := normalizeSyncScope(mutation.Scope)
		if err != nil {
			return err
		}
		if _, err := validateSyncScopeTx(ctx, tx, scope); err != nil {
			return err
		}
		deviceID := strings.TrimSpace(mutation.DeviceID)
		mutationID := strings.TrimSpace(mutation.MutationID)
		requestHash := strings.ToLower(strings.TrimSpace(mutation.RequestHash))
		if deviceID == "" || mutationID == "" || requestHash == "" {
			return errors.New("RSS state mutation identity and request hash are required")
		}
		if !mutation.AllowDesktopLocal {
			desktopLocal, err := isDesktopLocalSyncEntryIDTx(ctx, tx, mutation.EntryID)
			if err != nil {
				return err
			}
			if desktopLocal {
				return domainrss.ErrNotFound
			}
		}
		// Local eligibility is checked before receipt replay, while ordinary
		// deleted-entry retries retain their historical idempotency semantics.
		if deviceID != "" && mutationID != "" {
			cached := mutationRow{}
			err := tx.NewSelect().Model(&cached).
				Where("device_id = ? AND mutation_id = ?", deviceID, mutationID).
				Scan(ctx)
			if err == nil {
				if cached.EntryID != strings.TrimSpace(mutation.EntryID) || cached.RequestHash == "" ||
					!strings.EqualFold(cached.RequestHash, requestHash) {
					return domainrss.ErrIdempotencyConflict
				}
				return json.Unmarshal([]byte(cached.ResultJSON), &state)
			}
			if !errors.Is(err, sql.ErrNoRows) {
				return err
			}
		}
		row := entryRow{ID: strings.TrimSpace(mutation.EntryID)}
		entryQuery := tx.NewSelect().Model(&row).
			Column("entry_row.*").
			Join("JOIN rss_subscriptions AS subscription ON subscription.id = entry_row.subscription_id").
			Where("entry_row.id = ?", row.ID).
			Where("subscription.workspace_id = ?", scope.WorkspaceID)
		if !mutation.AllowDesktopLocal {
			entryQuery = applyPublicSyncEntryEligibility(entryQuery, "entry_row")
		}
		if err := entryQuery.Scan(ctx); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return domainrss.ErrNotFound
			}
			return err
		}
		desktopLocal, err := isDesktopLocalSyncEntryTx(ctx, tx, row.ID, row.SubscriptionID)
		if err != nil {
			return err
		}
		currentFieldRevision := stateFieldRevision(row, mutation.Field)
		if currentFieldRevision < 0 {
			return errors.New("invalid RSS state mutation field")
		}
		if expectedRevision != nil && currentFieldRevision != *expectedRevision {
			return &domainrss.StateConflictError{State: entryStateFromRow(row, scope.SubjectID, "")}
		}
		changedAt := mutation.ChangedAt.UTC()
		if changedAt.IsZero() {
			changedAt = time.Now().UTC()
		}
		switch mutation.Field {
		case domainrss.EntryStateFieldRead:
			if mutation.Read == nil {
				return errors.New("RSS read mutation value is required")
			}
			row.ReadAt = nil
			if *mutation.Read {
				row.ReadAt = &changedAt
			}
			row.ReadRevision++
		case domainrss.EntryStateFieldStarred:
			if mutation.Starred == nil {
				return errors.New("RSS starred mutation value is required")
			}
			row.StarredAt = nil
			if *mutation.Starred {
				row.StarredAt = &changedAt
			}
			row.StarredRevision++
		case domainrss.EntryStateFieldArticleProgress:
			if mutation.ArticleProgress == nil {
				return errors.New("RSS article progress value is required")
			}
			row.ArticleProgressFraction = &mutation.ArticleProgress.Fraction
			row.ArticleProgressAnchor = strings.TrimSpace(mutation.ArticleProgress.Anchor)
			row.ArticleProgressContentRevision = &mutation.ArticleProgress.ContentRevision
			row.ArticleProgressRevision++
		case domainrss.EntryStateFieldVideoProgressSeconds:
			if mutation.VideoProgressSeconds == nil {
				return errors.New("RSS video progress value is required")
			}
			row.VideoProgressSeconds = cloneFloat(mutation.VideoProgressSeconds)
			if mutation.VideoDurationSeconds != nil {
				row.VideoDurationSeconds = cloneFloat(mutation.VideoDurationSeconds)
			}
			row.VideoCompleted = row.VideoDurationSeconds != nil && *row.VideoDurationSeconds > 0 &&
				*row.VideoProgressSeconds >= *row.VideoDurationSeconds
			row.VideoProgressSecondsRevision++
		default:
			return errors.New("invalid RSS state mutation field")
		}
		row.ReadStateUpdatedAt = &changedAt
		row.ReadStateDeviceID = deviceID
		row.ReadStateSubjectID = scope.SubjectID
		row.StateRevision++
		if _, err := tx.NewUpdate().Model(&row).
			Column(
				"read_at", "starred_at",
				"article_progress_fraction", "article_progress_anchor", "article_progress_content_revision",
				"video_progress_seconds", "video_duration_seconds", "video_completed",
				"read_revision", "starred_revision", "article_progress_revision", "video_progress_seconds_revision",
				"read_state_updated_at", "read_state_device_id", "read_state_subject_id", "state_revision",
			).
			WherePK().Exec(ctx); err != nil {
			return err
		}
		state = entryStateFromRow(row, scope.SubjectID, mutationID)
		if deviceID != "" && mutationID != "" {
			encoded, err := json.Marshal(state)
			if err != nil {
				return err
			}
			if _, err = tx.NewInsert().Model(&mutationRow{
				DeviceID: deviceID, MutationID: mutationID, EntryID: row.ID,
				RequestHash: requestHash, ResultJSON: string(encoded), CreatedAt: changedAt,
			}).Exec(ctx); err != nil {
				return err
			}
		}
		if desktopLocal {
			return pruneRSSMutationReceiptsTx(
				ctx, tx, maxRetainedRSSMutationReceipts, time.Now().UTC().Add(-rssMutationReceiptTTL),
			)
		}
		// Append after the receipt so a periodic retention pass observes and
		// bounds the mutation that belongs to this same atomic state change.
		return appendChange(ctx, tx, "entry_state", row.ID, "upsert", state.Revision, state, changedAt)
	})
	return state, err
}

func (repo *SQLiteRepository) ListChanges(ctx context.Context, after int64, limit int) (domainrss.ChangePage, error) {
	scope := domainrss.SyncScope{WorkspaceID: domainrss.DefaultWorkspaceID, SubjectID: domainrss.DefaultSubjectID}
	overview, err := repo.GetSyncOverview(ctx, scope)
	if err != nil {
		return domainrss.ChangePage{}, err
	}
	page, err := repo.ListSyncChanges(ctx, domainrss.SyncChangeQuery{
		Scope: scope, Epoch: overview.Epoch, After: max(after, 0), Limit: limit,
	})
	if errors.Is(err, domainrss.ErrSyncResetRequired) {
		// The epoch can only rotate between these two transactions during a
		// restore. Retry once against the new generation for the legacy Wails
		// method; public callers always provide and validate their own epoch.
		overview, overviewErr := repo.GetSyncOverview(ctx, scope)
		if overviewErr != nil {
			return domainrss.ChangePage{}, overviewErr
		}
		return repo.ListSyncChanges(ctx, domainrss.SyncChangeQuery{
			Scope: scope, Epoch: overview.Epoch, After: max(after, 0), Limit: limit,
		})
	}
	return page, err
}

func (repo *SQLiteRepository) GetSyncOverview(ctx context.Context, scope domainrss.SyncScope) (domainrss.SyncOverview, error) {
	if repo == nil || repo.db == nil {
		return domainrss.SyncOverview{}, errors.New("rss repository unavailable")
	}
	var result domainrss.SyncOverview
	err := repo.db.RunInTx(ctx, &sql.TxOptions{ReadOnly: true}, func(ctx context.Context, tx bun.Tx) error {
		normalized, err := normalizeSyncScope(scope)
		if err != nil {
			return err
		}
		catalogID, err := validateSyncScopeTx(ctx, tx, normalized)
		if err != nil {
			return err
		}
		position, err := syncPositionTx(ctx, tx, normalized)
		if err != nil {
			return err
		}
		result = domainrss.SyncOverview{
			CatalogID: catalogID, WorkspaceID: normalized.WorkspaceID, SubjectID: normalized.SubjectID,
			Epoch: position.Epoch, HighWater: position.Cursor, RetainedFrom: position.RetainedFrom,
		}
		return nil
	})
	return result, err
}

func (repo *SQLiteRepository) ListSyncChanges(ctx context.Context, request domainrss.SyncChangeQuery) (domainrss.ChangePage, error) {
	if repo == nil || repo.db == nil {
		return domainrss.ChangePage{}, errors.New("rss repository unavailable")
	}
	limit := normalizeLimit(request.Limit, 200, 500)
	var page domainrss.ChangePage
	err := repo.db.RunInTx(ctx, &sql.TxOptions{ReadOnly: true}, func(ctx context.Context, tx bun.Tx) error {
		scope, err := normalizeSyncScope(request.Scope)
		if err != nil {
			return err
		}
		if _, err := validateSyncScopeTx(ctx, tx, scope); err != nil {
			return err
		}
		position, err := syncPositionTx(ctx, tx, scope)
		if err != nil {
			return err
		}
		if strings.TrimSpace(request.Epoch) != position.Epoch || request.After > position.Cursor || request.After < position.RetainedFrom {
			return &domainrss.SyncResetError{Position: position}
		}

		rows := make([]changeRow, 0, limit+1)
		if err := excludeDesktopLocalSourceChanges(tx.NewSelect().Model(&rows)).
			Where("workspace_id = ?", scope.WorkspaceID).
			Where("(subject_id IS NULL OR subject_id = '' OR subject_id = ?)", scope.SubjectID).
			Where("sequence > ? AND sequence <= ?", request.After, position.Cursor).
			OrderExpr("sequence ASC").Limit(limit + 1).Scan(ctx); err != nil {
			return err
		}
		hasMore := len(rows) > limit
		if hasMore {
			rows = rows[:limit]
		}
		page = domainrss.ChangePage{
			Changes: make([]domainrss.Change, 0, len(rows)), Epoch: position.Epoch,
			Cursor: request.After, HighWater: position.Cursor, RetainedFrom: position.RetainedFrom, HasMore: hasMore,
		}
		for _, row := range rows {
			payload, err := safeSyncChangePayload(row)
			if err != nil {
				return err
			}
			page.Changes = append(page.Changes, domainrss.Change{
				Sequence: row.Sequence, EntityType: row.EntityType, EntityID: row.EntityID,
				Operation: row.Operation, Revision: row.Revision,
				Payload: payload, ChangedAt: row.ChangedAt,
			})
			page.Cursor = row.Sequence
		}
		// A filtered local-only tail is still part of the journal high-water.
		// Advancing to it when there are no more public rows prevents callers
		// from polling the same invisible range forever.
		if !hasMore {
			page.Cursor = position.Cursor
		}
		return nil
	})
	return page, err
}

func excludeDesktopLocalSourceChanges(query *bun.SelectQuery) *bun.SelectQuery {
	// Current relationships cover live sources; local_* markers cover a source
	// after deletion without conflating it with a real public delete tombstone.
	return query.Where(`NOT (
		(entity_type = 'subscription' AND (
			entity_id IN (SELECT source.subscription_id FROM rss_sources source)
			OR entity_id IN (SELECT marker.entity_id FROM rss_tombstones marker WHERE marker.entity_type = 'local_subscription')
		))
		OR (entity_type IN ('entry', 'entry_state') AND (
			entity_id IN (
				SELECT entry.id
				FROM rss_entries entry
				JOIN rss_sources source ON source.subscription_id = entry.subscription_id
			)
			OR entity_id IN (SELECT marker.entity_id FROM rss_tombstones marker WHERE marker.entity_type = 'local_entry')
		))
		OR (entity_type = 'download' AND (
			entity_id IN (
				SELECT download.id
				FROM rss_entry_downloads download
				JOIN rss_entries entry ON entry.id = download.entry_id
				JOIN rss_sources source ON source.subscription_id = entry.subscription_id
			)
			OR entity_id IN (SELECT marker.entity_id FROM rss_tombstones marker WHERE marker.entity_type = 'local_download')
		))
	)`)
}

func (repo *SQLiteRepository) ListSyncSnapshot(ctx context.Context, request domainrss.SyncSnapshotQuery) (domainrss.SyncSnapshotPage, error) {
	if repo == nil || repo.db == nil {
		return domainrss.SyncSnapshotPage{}, errors.New("rss repository unavailable")
	}
	limit := normalizeLimit(request.Limit, 200, 500)
	var page domainrss.SyncSnapshotPage
	err := repo.db.RunInTx(ctx, &sql.TxOptions{ReadOnly: true}, func(ctx context.Context, tx bun.Tx) error {
		scope, err := normalizeSyncScope(request.Scope)
		if err != nil {
			return err
		}
		if _, err := validateSyncScopeTx(ctx, tx, scope); err != nil {
			return err
		}
		position, err := syncPositionTx(ctx, tx, scope)
		if err != nil {
			return err
		}
		if strings.TrimSpace(request.Epoch) != position.Epoch || request.HighWater > position.Cursor ||
			request.HighWater < position.RetainedFrom {
			return &domainrss.SyncResetError{Position: position}
		}
		stage := strings.TrimSpace(request.Stage)
		if stage == "" {
			stage = "subscriptions"
		}
		if stage != "subscriptions" && stage != "entries" {
			return errors.New("invalid RSS snapshot stage")
		}

		records := make([]domainrss.SyncSnapshotRecord, 0, limit+1)
		if stage == "subscriptions" {
			rows := make([]lightweightSyncSubscriptionRow, 0, limit+1)
			if err := applyPublicSyncSubscriptionEligibility(selectLightweightSyncSubscriptionColumns(
				tx.NewSelect().Model(&rows), "subscription_row",
			), "subscription_row").
				Where("subscription_row.workspace_id = ?", scope.WorkspaceID).
				Where("subscription_row.id > ?", strings.TrimSpace(request.AfterID)).
				OrderExpr("subscription_row.id ASC").Limit(limit + 1).Scan(ctx); err != nil {
				return err
			}
			for _, row := range rows {
				item := syncSubscriptionFromLightweightRow(row)
				payload, err := json.Marshal(item)
				if err != nil {
					return err
				}
				records = append(records, domainrss.SyncSnapshotRecord{
					EntityType: "subscription", EntityID: item.ID, Revision: item.Revision, Payload: payload,
				})
				if len(records) >= limit+1 {
					break
				}
			}
		}
		if len(records) < limit+1 {
			afterID := ""
			if stage == "entries" {
				afterID = strings.TrimSpace(request.AfterID)
			}
			rows := make([]lightweightSyncEntryRow, 0, limit+1-len(records))
			if err := applyPublicSyncEntryEligibility(selectLightweightSyncEntryColumns(tx.NewSelect().Model(&rows), "entry_row"), "entry_row").
				Join("JOIN rss_subscriptions AS subscription ON subscription.id = entry_row.subscription_id").
				Where("subscription.workspace_id = ?", scope.WorkspaceID).
				Where("entry_row.id > ?", afterID).
				OrderExpr("entry_row.id ASC").Limit(limit + 1 - len(records)).Scan(ctx); err != nil {
				return err
			}
			for _, row := range rows {
				item := syncEntryFromLightweightRow(row)
				payload, err := json.Marshal(item)
				if err != nil {
					return err
				}
				records = append(records, domainrss.SyncSnapshotRecord{
					EntityType: "entry", EntityID: item.ID, Revision: item.ContentRevision, Payload: payload,
				})
			}
		}
		hasMore := len(records) > limit
		if hasMore {
			records = records[:limit]
		}
		page = domainrss.SyncSnapshotPage{
			Records: records, Epoch: position.Epoch, HighWater: request.HighWater,
			RetainedFrom: position.RetainedFrom, HasMore: hasMore,
		}
		if hasMore && len(records) > 0 {
			last := records[len(records)-1]
			page.NextStage = "subscriptions"
			if last.EntityType == "entry" {
				page.NextStage = "entries"
			}
			page.NextID = last.EntityID
		}
		return nil
	})
	return page, err
}

func selectLightweightEntryColumns(query *bun.SelectQuery, alias string) *bun.SelectQuery {
	prefix := strings.TrimSpace(alias)
	if prefix != "" {
		prefix += "."
	}
	columns := []string{
		"id", "subscription_id", "external_id", "url", "title", "author", "summary", "kind",
		"image_urls_json", "media_json", "media_url", "media_type", "thumbnail_url", "platform", "platform_video_id",
		"published_at", "source_updated_at", "read_at", "starred_at",
		"article_progress_fraction", "article_progress_anchor", "article_progress_content_revision",
		"video_progress_seconds", "video_duration_seconds", "video_completed",
		"read_revision", "starred_revision", "article_progress_revision", "video_progress_seconds_revision",
		"state_revision", "read_state_updated_at", "read_state_device_id", "read_state_subject_id",
		"revision", "content_hash", "created_at", "modified_at",
	}
	qualified := make([]string, len(columns))
	for index, column := range columns {
		qualified[index] = prefix + column
	}
	return query.Column(qualified...)
}

func selectLightweightSyncEntryColumns(query *bun.SelectQuery, alias string) *bun.SelectQuery {
	prefix := strings.TrimSpace(alias)
	if prefix != "" {
		prefix += "."
	}
	columns := []string{
		"id", "subscription_id", "title", "author", "summary", "kind", "platform", "platform_video_id",
		"published_at", "source_updated_at", "read_at", "starred_at",
		"article_progress_fraction", "article_progress_anchor", "article_progress_content_revision",
		"video_progress_seconds", "video_duration_seconds", "video_completed",
		"read_revision", "starred_revision", "article_progress_revision", "video_progress_seconds_revision",
		"state_revision", "revision", "created_at", "modified_at",
	}
	qualified := make([]string, len(columns))
	for index, column := range columns {
		qualified[index] = prefix + column
	}
	return query.Column(qualified...).
		ColumnExpr("CASE WHEN " + prefix + "thumbnail_url <> '' THEN 1 ELSE 0 END AS thumbnail_available")
}

func syncEntryFromLightweightRow(row lightweightSyncEntryRow) domainrss.SyncEntry {
	var articleProgress *domainrss.ArticleProgress
	if row.ArticleProgressFraction != nil && row.ArticleProgressContentRevision != nil {
		articleProgress = &domainrss.ArticleProgress{
			Fraction: *row.ArticleProgressFraction, Anchor: row.ArticleProgressAnchor,
			ContentRevision: *row.ArticleProgressContentRevision,
		}
	}
	return domainrss.SyncEntry{
		ID: row.ID, SubscriptionID: row.SubscriptionID, Title: row.Title, Author: row.Author,
		Summary: row.Summary, Kind: row.Kind, ThumbnailAvailable: row.ThumbnailAvailable,
		Platform: row.Platform, PlatformVideoID: row.PlatformVideoID,
		PublishedAt: row.PublishedAt, SourceUpdatedAt: row.SourceUpdatedAt,
		Read: row.ReadAt != nil, ReadAt: row.ReadAt, Starred: row.StarredAt != nil, StarredAt: row.StarredAt,
		ArticleProgress: articleProgress, VideoProgressSeconds: cloneFloat(row.VideoProgressSeconds),
		VideoDurationSeconds: cloneFloat(row.VideoDurationSeconds), VideoCompleted: row.VideoCompleted,
		FieldRevisions: domainrss.StateFieldRevisions{
			Read: row.ReadRevision, Starred: row.StarredRevision,
			ArticleProgress: row.ArticleProgressRevision, VideoProgressSeconds: row.VideoProgressSecondsRevision,
		},
		StateRevision: row.StateRevision, ContentRevision: row.ContentRevision,
		CreatedAt: row.CreatedAt, ModifiedAt: row.ModifiedAt,
	}
}

func selectLightweightSyncSubscriptionColumns(query *bun.SelectQuery, alias string) *bun.SelectQuery {
	alias = strings.TrimSpace(alias)
	return query.
		Column(
			alias+".id", alias+".workspace_id", alias+".title", alias+".description",
			alias+".view_type", alias+".category_id", alias+".sort_order", alias+".enabled", alias+".created_at", alias+".updated_at", alias+".revision",
			alias+".source_access", alias+".public_feed_url",
		).
		ColumnExpr("CASE WHEN " + alias + ".icon_url <> '' THEN 1 ELSE 0 END AS icon_available").
		ColumnExpr("(SELECT COUNT(*) FROM rss_entries e WHERE e.subscription_id = " + alias + ".id AND e.read_at IS NULL) AS unread_count")
}

func syncSubscriptionFromLightweightRow(row lightweightSyncSubscriptionRow) domainrss.SyncSubscription {
	return domainrss.SyncSubscription{
		ID: row.ID, WorkspaceID: row.WorkspaceID, Title: row.Title, Description: row.Description,
		IconAvailable: row.IconAvailable, ViewType: row.ViewType, CategoryID: nullableStringValue(row.CategoryID), SortOrder: row.SortOrder,
		Enabled: row.Enabled, UnreadCount: row.UnreadCount,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Revision: row.Revision,
		SourceAccess: row.SourceAccess, PublicFeedURL: row.PublicFeedURL,
	}
}

func normalizeSyncScope(scope domainrss.SyncScope) (domainrss.SyncScope, error) {
	scope.WorkspaceID = strings.TrimSpace(scope.WorkspaceID)
	scope.SubjectID = strings.TrimSpace(scope.SubjectID)
	if scope.WorkspaceID == "" {
		scope.WorkspaceID = domainrss.DefaultWorkspaceID
	}
	if scope.SubjectID == "" {
		scope.SubjectID = domainrss.DefaultSubjectID
	}
	if scope.WorkspaceID == "" || scope.SubjectID == "" {
		return domainrss.SyncScope{}, errors.New("RSS synchronization scope is required")
	}
	return scope, nil
}

func validateSyncScopeTx(ctx context.Context, tx bun.Tx, scope domainrss.SyncScope) (string, error) {
	row := struct {
		CatalogID      string `bun:"catalog_id"`
		OwnerSubjectID string `bun:"owner_subject_id"`
	}{}
	if err := tx.NewSelect().Table("rss_workspaces").
		Column("catalog_id", "owner_subject_id").Where("id = ?", scope.WorkspaceID).Scan(ctx, &row); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", domainrss.ErrNotFound
		}
		return "", err
	}
	if strings.TrimSpace(row.OwnerSubjectID) != scope.SubjectID {
		return "", domainrss.ErrNotFound
	}
	return strings.TrimSpace(row.CatalogID), nil
}

func syncPositionTx(ctx context.Context, tx bun.Tx, scope domainrss.SyncScope) (domainrss.SyncPosition, error) {
	state := struct {
		Epoch        string `bun:"epoch"`
		RetainedFrom int64  `bun:"retained_from"`
	}{}
	if err := tx.NewSelect().Table("rss_sync_state").Column("epoch", "retained_from").
		Where("workspace_id = ?", scope.WorkspaceID).Scan(ctx, &state); err != nil {
		return domainrss.SyncPosition{}, err
	}
	var highWater int64
	if err := tx.NewSelect().Model((*changeRow)(nil)).ColumnExpr("COALESCE(MAX(sequence), 0)").
		Where("workspace_id = ?", scope.WorkspaceID).
		Where("(subject_id IS NULL OR subject_id = '' OR subject_id = ?)", scope.SubjectID).
		Scan(ctx, &highWater); err != nil {
		return domainrss.SyncPosition{}, err
	}
	retainedFrom := state.RetainedFrom
	if retainedFrom < 0 {
		retainedFrom = 0
	}
	if retainedFrom > highWater {
		retainedFrom = highWater
	}
	return domainrss.SyncPosition{Epoch: strings.TrimSpace(state.Epoch), Cursor: highWater, RetainedFrom: retainedFrom}, nil
}

func legacyReadMutationHash(mutation domainrss.ReadMutation) (string, error) {
	payload := struct {
		EntryID          string `json:"entryId"`
		Field            string `json:"field"`
		ExpectedRevision *int64 `json:"expectedRevision"`
		Value            bool   `json:"value"`
	}{
		EntryID: strings.TrimSpace(mutation.EntryID), Field: string(domainrss.EntryStateFieldRead),
		ExpectedRevision: mutation.ExpectedRevision, Value: mutation.Read,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return fmt.Sprintf("%x", digest[:]), nil
}

func stateFieldRevision(row entryRow, field domainrss.EntryStateField) int64 {
	switch field {
	case domainrss.EntryStateFieldRead:
		return row.ReadRevision
	case domainrss.EntryStateFieldStarred:
		return row.StarredRevision
	case domainrss.EntryStateFieldArticleProgress:
		return row.ArticleProgressRevision
	case domainrss.EntryStateFieldVideoProgressSeconds:
		return row.VideoProgressSecondsRevision
	default:
		return -1
	}
}

func entryStateFromRow(row entryRow, subjectID, mutationID string) domainrss.EntryState {
	updatedAt := row.ModifiedAt
	if row.ReadStateUpdatedAt != nil {
		updatedAt = row.ReadStateUpdatedAt.UTC()
	}
	return domainrss.EntryState{
		EntryID: row.ID, SubjectID: subjectID, Read: row.ReadAt != nil, ReadAt: row.ReadAt,
		Starred: row.StarredAt != nil, StarredAt: row.StarredAt,
		ArticleProgress:      articleProgressFromRow(row),
		VideoProgressSeconds: cloneFloat(row.VideoProgressSeconds),
		VideoDurationSeconds: cloneFloat(row.VideoDurationSeconds), VideoCompleted: row.VideoCompleted,
		FieldRevisions: domainrss.StateFieldRevisions{
			Read: row.ReadRevision, Starred: row.StarredRevision,
			ArticleProgress:      row.ArticleProgressRevision,
			VideoProgressSeconds: row.VideoProgressSecondsRevision,
		},
		Revision: row.StateRevision, UpdatedAt: updatedAt, UpdatedBy: row.ReadStateDeviceID,
		MutationID: mutationID,
	}
}

func articleProgressFromRow(row entryRow) *domainrss.ArticleProgress {
	if row.ArticleProgressFraction == nil || row.ArticleProgressContentRevision == nil {
		return nil
	}
	return &domainrss.ArticleProgress{
		Fraction: *row.ArticleProgressFraction, Anchor: row.ArticleProgressAnchor,
		ContentRevision: *row.ArticleProgressContentRevision,
	}
}

func articleProgressFraction(value *domainrss.ArticleProgress) *float64 {
	if value == nil {
		return nil
	}
	result := value.Fraction
	return &result
}

func articleProgressAnchor(value *domainrss.ArticleProgress) string {
	if value == nil {
		return ""
	}
	return value.Anchor
}

func articleProgressContentRevision(value *domainrss.ArticleProgress) *int64 {
	if value == nil {
		return nil
	}
	result := value.ContentRevision
	return &result
}

func cloneFloat(value *float64) *float64 {
	if value == nil {
		return nil
	}
	result := *value
	return &result
}

func applyStateRowToEntry(item *domainrss.Entry, row entryRow) {
	if item == nil {
		return
	}
	item.ReadAt = row.ReadAt
	item.StarredAt = row.StarredAt
	item.ArticleProgress = articleProgressFromRow(row)
	item.VideoProgressSeconds = cloneFloat(row.VideoProgressSeconds)
	item.VideoDurationSeconds = cloneFloat(row.VideoDurationSeconds)
	item.VideoCompleted = row.VideoCompleted
	item.FieldRevisions = domainrss.StateFieldRevisions{
		Read: row.ReadRevision, Starred: row.StarredRevision,
		ArticleProgress:      row.ArticleProgressRevision,
		VideoProgressSeconds: row.VideoProgressSecondsRevision,
	}
}

func syncSubscriptionProjection(item domainrss.Subscription) domainrss.SyncSubscription {
	return domainrss.SyncSubscription{
		ID: item.ID, WorkspaceID: item.WorkspaceID, Title: item.Title,
		Description: item.Description, IconAvailable: strings.TrimSpace(item.IconURL) != "", ViewType: item.ViewType,
		CategoryID: item.CategoryID, SortOrder: item.SortOrder,
		Enabled: item.Enabled, UnreadCount: item.UnreadCount, CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt, Revision: item.Revision,
		SourceAccess: normalizedSourceAccess(item.SourceAccess),
	}
}

func subscriptionContentChanged(existing, incoming subscriptionRow) bool {
	return existing.WorkspaceID != incoming.WorkspaceID ||
		existing.FeedURL != incoming.FeedURL ||
		existing.SourceAccess != incoming.SourceAccess ||
		existing.PublicFeedURL != incoming.PublicFeedURL ||
		existing.SiteURL != incoming.SiteURL ||
		existing.Title != incoming.Title ||
		existing.Description != incoming.Description ||
		existing.IconURL != incoming.IconURL ||
		existing.ViewType != incoming.ViewType ||
		nullableStringValue(existing.CategoryID) != nullableStringValue(incoming.CategoryID) ||
		existing.SortOrder != incoming.SortOrder ||
		existing.Enabled != incoming.Enabled
}

func syncEntryProjection(item domainrss.Entry) domainrss.SyncEntry {
	return domainrss.SyncEntry{
		ID: item.ID, SubscriptionID: item.SubscriptionID, Title: item.Title,
		Author: item.Author, Summary: item.Summary, Kind: item.Kind, ThumbnailAvailable: strings.TrimSpace(item.ThumbnailURL) != "",
		Platform: item.Platform, PlatformVideoID: item.PlatformVideoID,
		PublishedAt: item.PublishedAt, SourceUpdatedAt: item.SourceUpdatedAt,
		Read: item.ReadAt != nil, ReadAt: item.ReadAt, Starred: item.StarredAt != nil, StarredAt: item.StarredAt,
		ArticleProgress:      item.ArticleProgress,
		VideoProgressSeconds: item.VideoProgressSeconds, VideoDurationSeconds: item.VideoDurationSeconds,
		VideoCompleted: item.VideoCompleted, FieldRevisions: item.FieldRevisions,
		StateRevision: item.StateRevision, ContentRevision: item.Revision,
		CreatedAt: item.CreatedAt, ModifiedAt: item.ModifiedAt,
	}
}

func safeSyncChangePayload(row changeRow) (json.RawMessage, error) {
	if row.Operation == "delete" {
		return json.Marshal(map[string]string{"id": row.EntityID})
	}
	switch row.EntityType {
	case "subscription":
		var item domainrss.SyncSubscription
		if err := json.Unmarshal([]byte(row.PayloadJSON), &item); err != nil {
			return nil, fmt.Errorf("decode RSS subscription change: %w", err)
		}
		return json.Marshal(item)
	case "entry":
		var probe map[string]json.RawMessage
		if err := json.Unmarshal([]byte(row.PayloadJSON), &probe); err != nil {
			return nil, fmt.Errorf("decode RSS entry change: %w", err)
		}
		if _, projected := probe["contentRevision"]; projected {
			var item domainrss.SyncEntry
			if err := json.Unmarshal([]byte(row.PayloadJSON), &item); err != nil {
				return nil, err
			}
			return json.Marshal(item)
		}
		var item domainrss.Entry
		if err := json.Unmarshal([]byte(row.PayloadJSON), &item); err != nil {
			return nil, err
		}
		return json.Marshal(syncEntryProjection(item))
	case "entry_state":
		var item domainrss.EntryState
		if err := json.Unmarshal([]byte(row.PayloadJSON), &item); err != nil {
			return nil, err
		}
		return json.Marshal(item)
	default:
		// Downloads are represented by their entity identity/revision. Task or
		// extractor details belong to their separately-scoped public APIs.
		return json.Marshal(map[string]string{"id": row.EntityID})
	}
}

func appendChange(ctx context.Context, tx bun.Tx, entityType, entityID, operation string, revision int64, payload any, changedAt time.Time) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode RSS change: %w", err)
	}
	row := &changeRow{
		WorkspaceID: domainrss.DefaultWorkspaceID,
		SubjectID: func() string {
			if entityType == "entry_state" {
				return domainrss.DefaultSubjectID
			}
			return ""
		}(),
		EntityType: entityType, EntityID: entityID, Operation: operation,
		Revision: revision, PayloadJSON: string(encoded), ChangedAt: changedAt.UTC(),
	}
	_, err = tx.NewInsert().Model(row).Returning("sequence").Exec(ctx)
	if err != nil {
		return err
	}
	sequence := row.Sequence
	if sequence <= 0 || sequence%rssSyncJournalPruneInterval != 0 {
		return nil
	}
	return pruneRSSSyncJournalTx(
		ctx,
		tx,
		domainrss.DefaultWorkspaceID,
		maxRetainedRSSChanges,
		maxRetainedRSSMutationReceipts,
		time.Now().UTC().Add(-rssMutationReceiptTTL),
	)
}

func pruneRSSSyncJournalTx(
	ctx context.Context,
	tx bun.Tx,
	workspaceID string,
	maxChanges int,
	maxMutationReceipts int,
	mutationCutoff time.Time,
) error {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" || maxChanges < 1 || maxMutationReceipts < 1 || mutationCutoff.IsZero() {
		return errors.New("invalid RSS synchronization retention policy")
	}
	var changeCutoff int64
	err := tx.NewRaw(`
SELECT sequence
FROM rss_changes
WHERE workspace_id = ?
ORDER BY sequence DESC
LIMIT 1 OFFSET ?
`, workspaceID, maxChanges).Scan(ctx, &changeCutoff)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("select RSS change retention boundary: %w", err)
	}
	if err == nil && changeCutoff > 0 {
		if _, err := tx.ExecContext(ctx, `
DELETE FROM rss_changes
WHERE workspace_id = ? AND sequence <= ?
`, workspaceID, changeCutoff); err != nil {
			return fmt.Errorf("prune RSS changes: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
UPDATE rss_sync_state
SET retained_from = CASE WHEN retained_from < ? THEN ? ELSE retained_from END
WHERE workspace_id = ?
`, changeCutoff, changeCutoff, workspaceID); err != nil {
			return fmt.Errorf("advance RSS retained cursor: %w", err)
		}
	}
	return pruneRSSMutationReceiptsTx(ctx, tx, maxMutationReceipts, mutationCutoff)
}

func pruneRSSMutationReceiptsTx(
	ctx context.Context,
	tx bun.Tx,
	maxMutationReceipts int,
	mutationCutoff time.Time,
) error {
	if maxMutationReceipts < 1 || mutationCutoff.IsZero() {
		return errors.New("invalid RSS mutation receipt retention policy")
	}
	if _, err := tx.ExecContext(ctx, `
DELETE FROM rss_client_mutations
WHERE created_at < ?
`, mutationCutoff.UTC()); err != nil {
		return fmt.Errorf("expire RSS mutation receipts: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
DELETE FROM rss_client_mutations
WHERE rowid IN (
  SELECT rowid
  FROM rss_client_mutations
  ORDER BY created_at DESC, rowid DESC
  LIMIT -1 OFFSET ?
)
`, maxMutationReceipts); err != nil {
		return fmt.Errorf("bound RSS mutation receipts: %w", err)
	}
	return nil
}

func subscriptionToRow(item domainrss.Subscription) subscriptionRow {
	return subscriptionRow{
		ID: item.ID, WorkspaceID: item.WorkspaceID, FeedURL: item.FeedURL, SiteURL: item.SiteURL, Title: item.Title,
		SourceAccess: string(normalizedSourceAccess(item.SourceAccess)), PublicFeedURL: strings.TrimSpace(item.PublicFeedURL),
		Description: item.Description, IconURL: item.IconURL, ViewType: string(item.ViewType),
		CategoryID: nullableStringPointer(item.CategoryID), SortOrder: item.SortOrder,
		Enabled: item.Enabled, ETag: item.ETag, LastModified: item.LastModified, ValidatorURL: item.ValidatorURL,
		LastFetchedAt: item.LastFetchedAt, LastSuccessAt: item.LastSuccessAt,
		LastError: item.LastError, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
		Revision: item.Revision,
	}
}

func nullableStringPointer(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func nullableStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func subscriptionFromRow(row subscriptionRow) domainrss.Subscription {
	return domainrss.Subscription{
		ID: row.ID, WorkspaceID: row.WorkspaceID, FeedURL: row.FeedURL, SiteURL: row.SiteURL, Title: row.Title,
		SourceAccess: normalizedSourceAccess(domainrss.SubscriptionSourceAccess(row.SourceAccess)), PublicFeedURL: strings.TrimSpace(row.PublicFeedURL),
		Description: row.Description, IconURL: row.IconURL, ViewType: domainrss.ViewType(row.ViewType),
		ResolvedViewType: resolvedSubscriptionViewType(row),
		CategoryID:       nullableStringValue(row.CategoryID), SortOrder: row.SortOrder,
		Enabled: row.Enabled, UnreadCount: row.UnreadCount, ETag: row.ETag,
		LastModified: row.LastModified, ValidatorURL: row.ValidatorURL, LastFetchedAt: row.LastFetchedAt,
		LastSuccessAt: row.LastSuccessAt, LastError: row.LastError,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Revision: row.Revision,
	}
}

func normalizedSourceAccess(value domainrss.SubscriptionSourceAccess) domainrss.SubscriptionSourceAccess {
	switch value {
	case domainrss.SubscriptionSourceSharedPublic:
		return value
	default:
		return domainrss.SubscriptionSourceDesktopManaged
	}
}

func resolvedSubscriptionViewType(row subscriptionRow) domainrss.ViewType {
	resolved := domainrss.ViewType(strings.TrimSpace(row.ResolvedViewType))
	switch resolved {
	case domainrss.ViewTypeArticle, domainrss.ViewTypeSocial, domainrss.ViewTypeImage, domainrss.ViewTypeVideo:
		return resolved
	case domainrss.ViewTypeAuto:
		return domainrss.ViewTypeAuto
	}
	viewType := domainrss.ViewType(strings.TrimSpace(row.ViewType))
	if viewType != domainrss.ViewTypeAuto {
		return viewType
	}
	return domainrss.ViewTypeAuto
}

func entryToRow(item domainrss.Entry) entryRow {
	images, _ := json.Marshal(item.ImageURLs)
	media, _ := json.Marshal(item.Media)
	return entryRow{
		ID: item.ID, SubscriptionID: item.SubscriptionID, ExternalID: item.ExternalID,
		URL: item.URL, Title: item.Title, Author: item.Author, Summary: item.Summary,
		ContentHTML: item.ContentHTML, Kind: string(item.Kind), ImageURLsJSON: string(images),
		MediaJSON: string(media), MediaURL: item.MediaURL, MediaType: item.MediaType,
		ThumbnailURL: item.ThumbnailURL, Platform: item.Platform,
		PlatformVideoID: item.PlatformVideoID, PublishedAt: item.PublishedAt,
		SourceUpdatedAt: item.SourceUpdatedAt, ReadAt: item.ReadAt, StarredAt: item.StarredAt,
		ArticleProgressFraction:        articleProgressFraction(item.ArticleProgress),
		ArticleProgressAnchor:          articleProgressAnchor(item.ArticleProgress),
		ArticleProgressContentRevision: articleProgressContentRevision(item.ArticleProgress),
		VideoProgressSeconds:           cloneFloat(item.VideoProgressSeconds),
		VideoDurationSeconds:           cloneFloat(item.VideoDurationSeconds), VideoCompleted: item.VideoCompleted,
		ReadRevision: item.FieldRevisions.Read, StarredRevision: item.FieldRevisions.Starred,
		ArticleProgressRevision:      item.FieldRevisions.ArticleProgress,
		VideoProgressSecondsRevision: item.FieldRevisions.VideoProgressSeconds,
		StateRevision:                item.StateRevision, ReadStateUpdatedAt: item.ReadStateUpdatedAt,
		Revision: item.Revision, ContentHash: item.ContentHash, CreatedAt: item.CreatedAt,
		ModifiedAt: item.ModifiedAt,
	}
}

func entryFromRow(row entryRow) domainrss.Entry {
	images := make([]string, 0)
	_ = json.Unmarshal([]byte(row.ImageURLsJSON), &images)
	media := make([]domainrss.Media, 0)
	_ = json.Unmarshal([]byte(row.MediaJSON), &media)
	return domainrss.Entry{
		ID: row.ID, SubscriptionID: row.SubscriptionID, ExternalID: row.ExternalID,
		URL: row.URL, Title: row.Title, Author: row.Author, Summary: row.Summary,
		ContentHTML: row.ContentHTML, Kind: domainrss.EntryKind(row.Kind), ImageURLs: images,
		Media: media, MediaURL: row.MediaURL, MediaType: row.MediaType,
		ThumbnailURL: row.ThumbnailURL, Platform: row.Platform,
		PlatformVideoID: row.PlatformVideoID, DownloadTarget: downloadTargetFromRow(row), PublishedAt: row.PublishedAt,
		SourceUpdatedAt: row.SourceUpdatedAt, ReadAt: row.ReadAt, StarredAt: row.StarredAt,
		ArticleProgress: articleProgressFromRow(row), VideoProgressSeconds: cloneFloat(row.VideoProgressSeconds),
		VideoDurationSeconds: cloneFloat(row.VideoDurationSeconds), VideoCompleted: row.VideoCompleted,
		FieldRevisions: domainrss.StateFieldRevisions{
			Read: row.ReadRevision, Starred: row.StarredRevision,
			ArticleProgress:      row.ArticleProgressRevision,
			VideoProgressSeconds: row.VideoProgressSecondsRevision,
		},
		StateRevision: row.StateRevision, ReadStateUpdatedAt: row.ReadStateUpdatedAt,
		Revision: row.Revision, CreatedAt: row.CreatedAt, ModifiedAt: row.ModifiedAt,
		ContentHash: row.ContentHash,
	}
}

func downloadTargetFromRow(row entryRow) string {
	if platform := strings.TrimSpace(row.Platform); platform != "" && !strings.EqualFold(platform, "generic") {
		if pageURL := strings.TrimSpace(row.URL); pageURL != "" {
			return pageURL
		}
	}
	if mediaURL := strings.TrimSpace(row.MediaURL); mediaURL != "" {
		return mediaURL
	}
	return strings.TrimSpace(row.URL)
}

func discoveryRouteToRow(item domainrss.DiscoveryRoute) discoveryRouteRow {
	categories, _ := json.Marshal(item.Categories)
	parameters, _ := json.Marshal(item.Parameters)
	return discoveryRouteRow{
		ID: item.ID, Provider: item.Provider, Title: item.Title, URL: item.URL,
		Description: item.Description, SourceID: item.SourceID, SourceName: item.SourceName,
		SourceURL: item.SourceURL, SiteURL: item.SiteURL, RoutePath: item.RoutePath,
		ExamplePath: item.ExamplePath, CategoriesJSON: string(categories), Heat: item.Heat,
		Language: item.Language, Region: item.Region, ViewType: string(item.ViewType),
		RequiresConfig: item.RequiresConfig, RequiresPuppeteer: item.RequiresPuppeteer,
		NeedsParameters: item.NeedsParameters, ParametersJSON: string(parameters),
	}
}

func discoveryRouteFromRow(row discoveryRouteRow) domainrss.DiscoveryRoute {
	categories := make([]string, 0)
	_ = json.Unmarshal([]byte(row.CategoriesJSON), &categories)
	parameters := make([]domainrss.DiscoveryParameter, 0)
	_ = json.Unmarshal([]byte(row.ParametersJSON), &parameters)
	return domainrss.DiscoveryRoute{
		ID: row.ID, Provider: row.Provider, Title: row.Title, URL: row.URL,
		Description: row.Description, SourceID: row.SourceID, SourceName: row.SourceName,
		SourceURL: row.SourceURL, SiteURL: row.SiteURL, RoutePath: row.RoutePath,
		ExamplePath: row.ExamplePath, Categories: categories, Heat: row.Heat,
		Language: row.Language, Region: row.Region, ViewType: domainrss.ViewType(row.ViewType),
		RequiresConfig: row.RequiresConfig, RequiresPuppeteer: row.RequiresPuppeteer,
		NeedsParameters: row.NeedsParameters, Parameters: parameters,
	}
}

func normalizeLimit(value, fallback, maximum int) int {
	if value <= 0 {
		return fallback
	}
	if value > maximum {
		return maximum
	}
	return value
}
