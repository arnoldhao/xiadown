package rss

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	xhtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
	domainrss "xiadown/internal/domain/rss"
)

const maxLegacySyncSubscriptions = 2000

type lightweightSyncSubscriptionRepository interface {
	ListLightweightSyncSubscriptions(context.Context, int) ([]domainrss.SyncSubscription, error)
}

const (
	syncCursorVersion            = 1
	maxSyncContentHTMLBytes      = 1 << 20
	maxSyncSummaryBytes          = 8192
	maxSyncSubscriptionDescBytes = 4096
	maxSyncMediaItems            = 64
	maxSyncImageItems            = 64
	maxSyncURLBytes              = 4096
)

var publicRSSCapabilities = []string{
	"rss-sync-v1",
	"snapshot-keyset-v1",
	"changes-epoch-v1",
	"entry-state-v2",
	"entry-detail-on-demand-v1",
	"opaque-resource-slots-v1",
	"rss-subscription-mutations-v1",
	"rss-shared-public-fetch-v1",
	"rss-observations-v1",
	"rss-fetch-lease-v1",
}

type syncCursor struct {
	Version   int    `json:"v"`
	Epoch     string `json:"e"`
	HighWater int64  `json:"h"`
	Stage     string `json:"s"`
	AfterID   string `json:"a,omitempty"`
}

func (service *Service) GetSyncOverview(ctx context.Context, catalogID string) (domainrss.SyncOverview, error) {
	if service == nil || service.syncRepository == nil {
		return domainrss.SyncOverview{}, errors.New("RSS synchronization repository unavailable")
	}
	overview, err := service.syncRepository.GetSyncOverview(ctx, defaultSyncScope())
	if err != nil {
		return domainrss.SyncOverview{}, err
	}
	if value := strings.TrimSpace(catalogID); value != "" {
		overview.CatalogID = value
	}
	overview.Capabilities = append([]string(nil), publicRSSCapabilities...)
	return overview, nil
}

func (service *Service) ListSyncSubscriptions(ctx context.Context) ([]domainrss.SyncSubscription, error) {
	if repository, ok := service.repository.(lightweightSyncSubscriptionRepository); ok {
		items, err := repository.ListLightweightSyncSubscriptions(ctx, maxLegacySyncSubscriptions)
		if err != nil {
			return nil, err
		}
		for index := range items {
			items[index].WorkspaceID = domainrss.DefaultWorkspaceID
			items[index].Title = limitString(strings.TrimSpace(items[index].Title), 512)
			items[index].Description = limitString(strings.TrimSpace(items[index].Description), maxSyncSubscriptionDescBytes)
			items[index].SourceAccess = normalizeSyncSourceAccess(items[index].SourceAccess)
			items[index].PublicFeedURL = syncPublicFeedURL(items[index].SourceAccess, items[index].PublicFeedURL)
		}
		return items, nil
	}
	items, err := service.repository.ListSubscriptions(ctx)
	if err != nil {
		return nil, err
	}
	if len(items) > maxLegacySyncSubscriptions {
		items = items[:maxLegacySyncSubscriptions]
	}
	result := make([]domainrss.SyncSubscription, 0, len(items))
	for _, item := range items {
		if item.WorkspaceID != "" && item.WorkspaceID != domainrss.DefaultWorkspaceID {
			continue
		}
		result = append(result, syncSubscriptionFromDomain(item))
	}
	return result, nil
}

func (service *Service) ListSyncEntries(ctx context.Context, request ListEntriesRequest) (SyncEntryPage, error) {
	if service == nil || service.syncRepository == nil {
		return SyncEntryPage{}, errors.New("RSS synchronization repository unavailable")
	}
	kind := domainrss.EntryKind(strings.ToLower(strings.TrimSpace(request.Kind)))
	if kind != "" && kind != domainrss.EntryKindArticle && kind != domainrss.EntryKindSocial &&
		kind != domainrss.EntryKindImage && kind != domainrss.EntryKindVideo {
		return SyncEntryPage{}, errors.New("invalid RSS entry kind")
	}
	page, err := service.syncRepository.ListSyncEntries(ctx, domainrss.EntryQuery{
		SubscriptionID: strings.TrimSpace(request.SubscriptionID), Kind: kind,
		Query: strings.TrimSpace(request.Query), UnreadOnly: request.UnreadOnly, StarredOnly: request.StarredOnly,
		Limit: request.Limit, Offset: request.Offset,
	})
	if err != nil {
		return SyncEntryPage{}, err
	}
	for index := range page.Items {
		page.Items[index] = normalizeSyncEntryProjection(page.Items[index])
	}
	return page, nil
}

func (service *Service) GetSyncEntry(ctx context.Context, request SubscriptionRequest) (SyncEntryDetail, error) {
	if service == nil || service.syncRepository == nil {
		return SyncEntryDetail{}, errors.New("RSS synchronization repository unavailable")
	}
	item, err := service.syncRepository.GetSyncEntry(ctx, strings.TrimSpace(request.ID))
	if err != nil {
		return SyncEntryDetail{}, err
	}
	mediaSlots := make([]domainrss.SyncMediaSlot, min(len(item.Media), maxSyncMediaItems))
	for index := range mediaSlots {
		candidate := item.Media[index]
		mediaSlots[index] = domainrss.SyncMediaSlot{
			Available:          syncResourceAvailable(candidate.URL),
			ThumbnailAvailable: syncResourceAvailable(candidate.Thumbnail),
			MIMEType:           limitString(strings.TrimSpace(candidate.MIMEType), 256),
			Kind:               limitString(strings.TrimSpace(candidate.Kind), 64),
			// Old databases predate the ingestion bounds. Clamp again at the
			// public projection boundary so legacy negative/oversized metadata
			// cannot violate the documented device contract.
			Width:    boundedRSSMediaDimension(candidate.Width),
			Height:   boundedRSSMediaDimension(candidate.Height),
			Duration: boundedRSSMediaDurationMillis(candidate.Duration),
		}
	}
	imageSlots := make([]bool, min(len(item.ImageURLs), maxSyncImageItems))
	for index := range imageSlots {
		imageSlots[index] = syncResourceAvailable(item.ImageURLs[index])
	}
	return SyncEntryDetail{
		SyncEntry:   syncEntryFromDomain(item),
		ContentHTML: sanitizeSyncContentHTML(item.ContentHTML, item.URL, item.ImageURLs),
		ImageSlots:  imageSlots,
		MediaSlots:  mediaSlots,
	}, nil
}

func (service *Service) GetSyncSnapshot(ctx context.Context, request SyncSnapshotRequest) (SyncSnapshotResult, error) {
	if service == nil || service.syncRepository == nil {
		return SyncSnapshotResult{}, errors.New("RSS synchronization repository unavailable")
	}
	epoch := strings.TrimSpace(request.Epoch)
	if !validSyncEpoch(epoch) || request.HighWater < 0 {
		return SyncSnapshotResult{}, invalidRSSRequest("invalid RSS snapshot position")
	}
	stage, afterID := "subscriptions", ""
	if cursorValue := strings.TrimSpace(request.Cursor); cursorValue != "" {
		cursor, err := decodeSyncCursor(cursorValue)
		if err != nil || cursor.Epoch != epoch || cursor.HighWater != request.HighWater {
			return SyncSnapshotResult{}, invalidRSSRequest("invalid RSS snapshot cursor")
		}
		stage, afterID = cursor.Stage, cursor.AfterID
	}
	page, err := service.syncRepository.ListSyncSnapshot(ctx, domainrss.SyncSnapshotQuery{
		Scope: defaultSyncScope(), Epoch: epoch, HighWater: request.HighWater,
		Stage: stage, AfterID: afterID, Limit: request.Limit,
	})
	if err != nil {
		return SyncSnapshotResult{}, err
	}
	records, err := sanitizeSyncRecords(page.Records)
	if err != nil {
		return SyncSnapshotResult{}, err
	}
	result := SyncSnapshotResult{
		Records: records, Epoch: page.Epoch, HighWater: page.HighWater,
		RetainedFrom: page.RetainedFrom, HasMore: page.HasMore,
	}
	if page.HasMore {
		result.NextCursor, err = encodeSyncCursor(syncCursor{
			Version: syncCursorVersion, Epoch: page.Epoch, HighWater: page.HighWater,
			Stage: page.NextStage, AfterID: page.NextID,
		})
		if err != nil {
			return SyncSnapshotResult{}, err
		}
	}
	return result, nil
}

func (service *Service) ListSyncChanges(ctx context.Context, request ListChangesRequest) (domainrss.ChangePage, error) {
	if service == nil || service.syncRepository == nil {
		return domainrss.ChangePage{}, errors.New("RSS synchronization repository unavailable")
	}
	epoch := strings.TrimSpace(request.Epoch)
	if !validSyncEpoch(epoch) || request.After < 0 {
		return domainrss.ChangePage{}, invalidRSSRequest("invalid RSS change cursor")
	}
	page, err := service.syncRepository.ListSyncChanges(ctx, domainrss.SyncChangeQuery{
		Scope: defaultSyncScope(), Epoch: epoch, After: request.After, Limit: request.Limit,
	})
	if err != nil {
		return domainrss.ChangePage{}, err
	}
	for index := range page.Changes {
		records, sanitizeErr := sanitizeSyncRecords([]domainrss.SyncSnapshotRecord{{
			EntityType: page.Changes[index].EntityType, EntityID: page.Changes[index].EntityID,
			Revision: page.Changes[index].Revision, Payload: page.Changes[index].Payload,
		}})
		if sanitizeErr != nil {
			return domainrss.ChangePage{}, sanitizeErr
		}
		page.Changes[index].Payload = records[0].Payload
	}
	return page, nil
}

func (service *Service) SetEntryStateForDevice(
	ctx context.Context,
	deviceID string,
	request SetEntryStateRequest,
) (domainrss.EntryState, error) {
	return service.setEntryStateForDevice(ctx, deviceID, request, false)
}

func (service *Service) setEntryStateForDevice(
	ctx context.Context,
	deviceID string,
	request SetEntryStateRequest,
	allowDesktopLocal bool,
) (domainrss.EntryState, error) {
	if service == nil || service.syncRepository == nil {
		return domainrss.EntryState{}, errors.New("RSS synchronization repository unavailable")
	}
	deviceID = strings.TrimSpace(deviceID)
	entryID := strings.TrimSpace(request.ID)
	mutationID := strings.TrimSpace(request.MutationID)
	if deviceID == "" || entryID == "" || mutationID == "" || request.ExpectedRevision == nil || *request.ExpectedRevision < 0 {
		return domainrss.EntryState{}, invalidRSSRequest("invalid RSS entry state mutation")
	}
	if request.ArticleProgress != nil {
		request.ArticleProgress = cloneArticleProgress(request.ArticleProgress)
		request.ArticleProgress.Anchor = strings.TrimSpace(request.ArticleProgress.Anchor)
	}

	field, value, err := validateStateMutationValue(request)
	if err != nil {
		return domainrss.EntryState{}, err
	}
	canonical, err := json.Marshal(canonicalStateMutation{
		EntryID: entryID, Field: field, ExpectedRevision: *request.ExpectedRevision, Value: value,
		VideoDurationSeconds: request.VideoDurationSeconds,
	})
	if err != nil {
		return domainrss.EntryState{}, err
	}
	digest := sha256.Sum256(canonical)
	return service.syncRepository.ApplyStateMutation(ctx, domainrss.StateMutation{
		Scope: defaultSyncScope(), EntryID: entryID, Field: field,
		Read: request.Read, Starred: request.Starred,
		ArticleProgress:      cloneArticleProgress(request.ArticleProgress),
		VideoProgressSeconds: cloneFloat64(request.VideoProgressSeconds),
		VideoDurationSeconds: cloneFloat64(request.VideoDurationSeconds),
		ExpectedRevision:     *request.ExpectedRevision, DeviceID: deviceID,
		MutationID: mutationID, RequestHash: hex.EncodeToString(digest[:]), ChangedAt: service.now(),
		AllowDesktopLocal: allowDesktopLocal,
	})
}

// SetEntryState exposes the same v2 state model to the desktop Wails bridge.
// The mutation ID remains caller-supplied so a renderer retry has the same
// idempotency guarantees as a paired mobile device.
func (service *Service) SetEntryState(ctx context.Context, request SetEntryStateRequest) (domainrss.EntryState, error) {
	return service.setEntryStateForDevice(ctx, "desktop", request, true)
}

func validateStateMutationValue(request SetEntryStateRequest) (domainrss.EntryStateField, json.RawMessage, error) {
	field := request.Field
	if request.VideoDurationSeconds != nil && field != domainrss.EntryStateFieldVideoProgressSeconds {
		return "", nil, invalidRSSRequest("RSS video duration is only valid with video progress")
	}
	setValues := 0
	if request.Read != nil {
		setValues++
	}
	if request.Starred != nil {
		setValues++
	}
	if request.ArticleProgress != nil {
		setValues++
	}
	if request.VideoProgressSeconds != nil {
		setValues++
	}
	if setValues != 1 {
		return "", nil, invalidRSSRequest("RSS state mutation must set exactly one field")
	}
	var value any
	switch field {
	case domainrss.EntryStateFieldRead:
		if request.Read == nil {
			return "", nil, invalidRSSRequest("RSS read state value is required")
		}
		value = *request.Read
	case domainrss.EntryStateFieldStarred:
		if request.Starred == nil {
			return "", nil, invalidRSSRequest("RSS starred state value is required")
		}
		value = *request.Starred
	case domainrss.EntryStateFieldArticleProgress:
		progress := request.ArticleProgress
		if progress == nil || math.IsNaN(progress.Fraction) || math.IsInf(progress.Fraction, 0) ||
			progress.Fraction < 0 || progress.Fraction > 1 || progress.ContentRevision < 1 ||
			!validProgressAnchor(progress.Anchor) {
			return "", nil, invalidRSSRequest("invalid RSS article progress")
		}
		progress = cloneArticleProgress(progress)
		progress.Anchor = strings.TrimSpace(progress.Anchor)
		request.ArticleProgress = progress
		value = progress
	case domainrss.EntryStateFieldVideoProgressSeconds:
		progress := request.VideoProgressSeconds
		if progress == nil || math.IsNaN(*progress) || math.IsInf(*progress, 0) || *progress < 0 ||
			(request.VideoDurationSeconds != nil && (math.IsNaN(*request.VideoDurationSeconds) ||
				math.IsInf(*request.VideoDurationSeconds, 0) || *request.VideoDurationSeconds < 0 || *progress > *request.VideoDurationSeconds)) {
			return "", nil, invalidRSSRequest("invalid RSS video progress")
		}
		value = *progress
	default:
		return "", nil, invalidRSSRequest("invalid RSS state field")
	}
	encoded, err := json.Marshal(value)
	return field, encoded, err
}

func syncSubscriptionFromDomain(item domainrss.Subscription) domainrss.SyncSubscription {
	return domainrss.SyncSubscription{
		ID: item.ID, WorkspaceID: domainrss.DefaultWorkspaceID,
		Title:       limitString(strings.TrimSpace(item.Title), 512),
		Description: limitString(strings.TrimSpace(item.Description), maxSyncSubscriptionDescBytes), IconAvailable: syncResourceAvailable(item.IconURL),
		ViewType: item.ViewType, Enabled: item.Enabled, UnreadCount: item.UnreadCount,
		CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt, Revision: item.Revision,
		SourceAccess:  normalizeSyncSourceAccess(item.SourceAccess),
		PublicFeedURL: syncPublicFeedURL(item.SourceAccess, item.PublicFeedURL),
	}
}

func normalizeSyncSourceAccess(value domainrss.SubscriptionSourceAccess) domainrss.SubscriptionSourceAccess {
	if value == domainrss.SubscriptionSourceSharedPublic {
		return value
	}
	return domainrss.SubscriptionSourceDesktopManaged
}

func syncPublicFeedURL(sourceAccess domainrss.SubscriptionSourceAccess, value string) string {
	if normalizeSyncSourceAccess(sourceAccess) != domainrss.SubscriptionSourceSharedPublic {
		return ""
	}
	return strings.TrimSpace(value)
}

// GetSyncSubscriptionSource returns the separately stored shared-public
// descriptor for scope-aware HTTP projection. The ordinary journal payload is
// deliberately URL-free at rest.
func (service *Service) GetSyncSubscriptionSource(ctx context.Context, id string) (domainrss.SubscriptionSourceAccess, string, error) {
	if service == nil || service.syncRepository == nil {
		return "", "", errors.New("RSS synchronization repository unavailable")
	}
	item, err := service.syncRepository.GetSyncSubscription(ctx, strings.TrimSpace(id))
	if err != nil {
		return "", "", err
	}
	access := normalizeSyncSourceAccess(item.SourceAccess)
	return access, syncPublicFeedURL(access, item.PublicFeedURL), nil
}

func syncEntryFromDomain(item domainrss.Entry) domainrss.SyncEntry {
	return normalizeSyncEntryProjection(domainrss.SyncEntry{
		ID: item.ID, SubscriptionID: item.SubscriptionID,
		Title: item.Title, Author: item.Author, Summary: item.Summary, Kind: item.Kind,
		ThumbnailAvailable: syncResourceAvailable(item.ThumbnailURL), Platform: limitString(strings.TrimSpace(item.Platform), 64),
		PlatformVideoID: limitString(strings.TrimSpace(item.PlatformVideoID), 256),
		PublishedAt:     item.PublishedAt, SourceUpdatedAt: item.SourceUpdatedAt,
		Read: item.ReadAt != nil, ReadAt: item.ReadAt, Starred: item.StarredAt != nil, StarredAt: item.StarredAt,
		ArticleProgress:      cloneArticleProgress(item.ArticleProgress),
		VideoProgressSeconds: cloneFloat64(item.VideoProgressSeconds), VideoDurationSeconds: cloneFloat64(item.VideoDurationSeconds),
		VideoCompleted: item.VideoCompleted,
		FieldRevisions: item.FieldRevisions, StateRevision: item.StateRevision, ContentRevision: item.Revision,
		CreatedAt: item.CreatedAt, ModifiedAt: item.ModifiedAt,
	})
}

func normalizeSyncEntryProjection(item domainrss.SyncEntry) domainrss.SyncEntry {
	item.Title = limitString(strings.TrimSpace(item.Title), 1024)
	item.Author = limitString(strings.TrimSpace(item.Author), 512)
	item.Summary = limitString(strings.TrimSpace(item.Summary), maxSyncSummaryBytes)
	item.Platform = limitString(strings.TrimSpace(item.Platform), 64)
	item.PlatformVideoID = limitString(strings.TrimSpace(item.PlatformVideoID), 256)
	if item.ArticleProgress != nil {
		item.ArticleProgress = cloneArticleProgress(item.ArticleProgress)
		item.ArticleProgress.Anchor = limitString(strings.TrimSpace(item.ArticleProgress.Anchor), 512)
	}
	return item
}

func sanitizeSyncRecords(records []domainrss.SyncSnapshotRecord) ([]domainrss.SyncSnapshotRecord, error) {
	result := make([]domainrss.SyncSnapshotRecord, 0, len(records))
	for _, record := range records {
		switch record.EntityType {
		case "subscription":
			var item domainrss.SyncSubscription
			if err := json.Unmarshal(record.Payload, &item); err != nil {
				return nil, err
			}
			item.WorkspaceID = domainrss.DefaultWorkspaceID
			item.Title = limitString(strings.TrimSpace(item.Title), 512)
			item.Description = limitString(strings.TrimSpace(item.Description), maxSyncSubscriptionDescBytes)
			item.SourceAccess = normalizeSyncSourceAccess(item.SourceAccess)
			// A stored journal is readable with rss.read, so it must never become a
			// credential-adjacent URL store. rss.fetch handlers enrich this field
			// from the separate subscription column after authorization.
			item.PublicFeedURL = ""
			payload, err := json.Marshal(item)
			if err != nil {
				return nil, err
			}
			record.Payload = payload
		case "entry":
			var item domainrss.SyncEntry
			if err := json.Unmarshal(record.Payload, &item); err != nil {
				return nil, err
			}
			item = normalizeSyncEntryProjection(item)
			payload, err := json.Marshal(item)
			if err != nil {
				return nil, err
			}
			record.Payload = payload
		case "entry_state":
			var item domainrss.EntryState
			if err := json.Unmarshal(record.Payload, &item); err != nil {
				return nil, err
			}
			item.SubjectID = domainrss.DefaultSubjectID
			payload, err := json.Marshal(item)
			if err != nil {
				return nil, err
			}
			record.Payload = payload
		default:
			record.Payload = json.RawMessage(`{"id":` + strconv.Quote(record.EntityID) + `}`)
		}
		result = append(result, record)
	}
	return result, nil
}

func defaultSyncScope() domainrss.SyncScope {
	return domainrss.SyncScope{WorkspaceID: domainrss.DefaultWorkspaceID, SubjectID: domainrss.DefaultSubjectID}
}

func validSyncEpoch(value string) bool {
	if len(value) != 32 {
		return false
	}
	for _, item := range value {
		if (item < '0' || item > '9') && (item < 'a' || item > 'f') {
			return false
		}
	}
	return true
}

func validProgressAnchor(value string) bool {
	value = strings.TrimSpace(value)
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) > 512 {
		return false
	}
	for _, item := range value {
		if unicode.IsControl(item) {
			return false
		}
	}
	return true
}

func encodeSyncCursor(cursor syncCursor) (string, error) {
	encoded, err := json.Marshal(cursor)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(encoded), nil
}

func decodeSyncCursor(value string) (syncCursor, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(value))
	if err != nil || len(decoded) > 2048 {
		return syncCursor{}, errors.New("invalid RSS snapshot cursor")
	}
	var cursor syncCursor
	decoder := json.NewDecoder(strings.NewReader(string(decoded)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cursor); err != nil || cursor.Version != syncCursorVersion || !validSyncEpoch(cursor.Epoch) ||
		cursor.HighWater < 0 || (cursor.Stage != "subscriptions" && cursor.Stage != "entries") || !validCursorID(cursor.AfterID) {
		return syncCursor{}, errors.New("invalid RSS snapshot cursor")
	}
	return cursor, nil
}

func validCursorID(value string) bool {
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) > 255 {
		return false
	}
	for _, item := range value {
		if unicode.IsControl(item) {
			return false
		}
	}
	return true
}

func cloneArticleProgress(value *domainrss.ArticleProgress) *domainrss.ArticleProgress {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func cloneFloat64(value *float64) *float64 {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func syncResourceAvailable(value string) bool {
	return boundedSyncURL(value) != ""
}

// sanitizeSyncContentHTML keeps inert reading structure and replaces inline
// images with stable, resource-neutral image-N placeholders. Paired DTOs never
// serialize reusable signed image/media/link URLs; clients resolve a declared
// placeholder only through the authenticated entry resource-slot route.
func sanitizeSyncContentHTML(markup, baseURL string, imageURLs []string) string {
	markup = limitString(sanitizeEntryHTML(markup, baseURL), maxSyncContentHTMLBytes)
	if strings.TrimSpace(markup) == "" {
		return ""
	}
	imageSlotByURL := make(map[string]int, min(len(imageURLs), maxSyncImageItems))
	for index, rawURL := range imageURLs[:min(len(imageURLs), maxSyncImageItems)] {
		if normalized := boundedSyncURL(rawURL); normalized != "" {
			if _, exists := imageSlotByURL[normalized]; !exists {
				imageSlotByURL[normalized] = index
			}
		}
	}
	contextNode := &xhtml.Node{Type: xhtml.ElementNode, DataAtom: atom.Div, Data: "div"}
	nodes, err := xhtml.ParseFragment(strings.NewReader(markup), contextNode)
	if err != nil {
		return ""
	}
	var sanitize func(*xhtml.Node) bool
	sanitize = func(node *xhtml.Node) bool {
		if node.Type == xhtml.ElementNode {
			name := strings.ToLower(node.Data)
			if name == "img" || name == "source" {
				slot, available := syncInlineImageSlot(node, imageSlotByURL)
				if !available {
					return false
				}
				if name == "source" {
					// A picture's img fallback owns the visual position. A lone
					// source is promoted so source-only feed markup is not lost.
					if pictureHasSyncImageFallback(node.Parent, imageSlotByURL) {
						return false
					}
					node.Data = "img"
					node.DataAtom = atom.Img
				}
				attributes := node.Attr[:0]
				for _, attribute := range node.Attr {
					if attribute.Namespace != "" {
						continue
					}
					switch strings.ToLower(attribute.Key) {
					case "alt", "height", "title", "width":
						attributes = append(attributes, attribute)
					}
				}
				node.Attr = append(attributes, xhtml.Attribute{
					Key: "data-xiadown-slot", Val: "image-" + strconv.Itoa(slot),
				})
			} else {
				attributes := node.Attr[:0]
				for _, attribute := range node.Attr {
					if attribute.Namespace == "" {
						switch strings.ToLower(attribute.Key) {
						case "action", "cite", "formaction", "href", "poster", "src", "srcset":
							continue
						}
					}
					attributes = append(attributes, attribute)
				}
				node.Attr = attributes
			}
		}
		for child := node.FirstChild; child != nil; {
			next := child.NextSibling
			if !sanitize(child) {
				node.RemoveChild(child)
			}
			child = next
		}
		return true
	}
	var output bytes.Buffer
	for _, node := range nodes {
		if !sanitize(node) {
			continue
		}
		if err := xhtml.Render(&output, node); err != nil {
			return ""
		}
	}
	return limitString(strings.TrimSpace(output.String()), maxSyncContentHTMLBytes)
}

func syncInlineImageSlot(node *xhtml.Node, imageSlotByURL map[string]int) (int, bool) {
	if node == nil {
		return 0, false
	}
	for _, attribute := range node.Attr {
		if attribute.Namespace == "" && strings.EqualFold(attribute.Key, "src") {
			slot, ok := imageSlotByURL[boundedSyncURL(attribute.Val)]
			return slot, ok
		}
	}
	return 0, false
}

func pictureHasSyncImageFallback(parent *xhtml.Node, imageSlotByURL map[string]int) bool {
	if parent == nil || parent.Type != xhtml.ElementNode || !strings.EqualFold(parent.Data, "picture") {
		return false
	}
	for child := parent.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == xhtml.ElementNode && strings.EqualFold(child.Data, "img") {
			if _, ok := syncInlineImageSlot(child, imageSlotByURL); ok {
				return true
			}
		}
	}
	return false
}

func boundedSyncURL(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > maxSyncURLBytes {
		return ""
	}
	result := safeEntryResourceURL("", value)
	if len(result) > maxSyncURLBytes {
		return ""
	}
	return result
}

func invalidRSSRequest(message string) error {
	return fmt.Errorf("%w: %s", domainrss.ErrInvalidRequest, message)
}
