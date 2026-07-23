package rss

import (
	"errors"
	"strconv"
	"strings"
	"time"

	domainrss "xiadown/internal/domain/rss"

	"github.com/google/uuid"
)

const (
	maxPreviewLeaseBytes          int64 = 32 << 20
	previewLeaseBaseOverheadBytes int64 = 1024
	previewEntryOverheadBytes     int64 = 512
	previewMediaOverheadBytes     int64 = 192
)

var errPreviewLeaseInUse = errors.New("RSS preview subscription is already being added")

type previewLeaseClaimStatus uint8

const (
	previewLeaseMissing previewLeaseClaimStatus = iota
	previewLeaseAcquired
	previewLeaseBusy
)

// previewFeedSnapshot contains only the sanitized fields needed to build the
// initial durable subscription and entries. Keeping parsed XML/JSON objects out
// of a lease prevents short string slices from retaining an entire feed body.
type previewFeedSnapshot struct {
	title       string
	siteURL     string
	description string
	iconURL     string
	entries     []previewEntrySnapshot
}

type previewEntrySnapshot struct {
	externalID      string
	url             string
	title           string
	author          string
	summary         string
	contentHTML     string
	media           []domainrss.Media
	publishedAt     *time.Time
	sourceUpdatedAt *time.Time
}

type previewLease struct {
	canonical string
	snapshot  previewFeedSnapshot
	metadata  fetchMetadata
	expiresAt time.Time
	bytes     int64
	claimed   bool
	timer     *time.Timer
}

func newPreviewFeedSnapshot(canonical string, feed parsedFeed, now time.Time) previewFeedSnapshot {
	subscription := subscriptionFromParsedFeed("", canonical, domainrss.ViewTypeAuto, feed, now)
	persistedEntries := entriesFromFeed("", domainrss.ViewTypeAuto, feed, now)
	snapshot := previewFeedSnapshot{
		title:       strings.Clone(subscription.Title),
		siteURL:     strings.Clone(subscription.SiteURL),
		description: strings.Clone(subscription.Description),
		iconURL:     strings.Clone(subscription.IconURL),
		entries:     make([]previewEntrySnapshot, 0, len(persistedEntries)),
	}
	for _, entry := range persistedEntries {
		snapshot.entries = append(snapshot.entries, previewEntrySnapshot{
			externalID:      strings.Clone(entry.ExternalID),
			url:             strings.Clone(entry.URL),
			title:           strings.Clone(entry.Title),
			author:          strings.Clone(entry.Author),
			summary:         strings.Clone(entry.Summary),
			contentHTML:     strings.Clone(entry.ContentHTML),
			media:           clonePreviewMedia(entry.Media),
			publishedAt:     clonePreviewTime(entry.PublishedAt),
			sourceUpdatedAt: clonePreviewTime(entry.SourceUpdatedAt),
		})
	}
	return snapshot
}

func (snapshot previewFeedSnapshot) materialize(
	id, canonical string,
	view domainrss.ViewType,
	now time.Time,
) (domainrss.Subscription, []domainrss.Entry) {
	subscription := domainrss.Subscription{
		ID: id, WorkspaceID: domainrss.DefaultWorkspaceID, FeedURL: canonical,
		SourceAccess: domainrss.SubscriptionSourceDesktopManaged,
		SiteURL:      snapshot.siteURL, Title: snapshot.title, Description: snapshot.description, IconURL: snapshot.iconURL,
		ViewType: view, Enabled: true, CreatedAt: now, UpdatedAt: now, Revision: 1,
	}
	entries := make([]domainrss.Entry, 0, len(snapshot.entries))
	for index, source := range snapshot.entries {
		media := clonePreviewMedia(source.media)
		images := make([]string, 0, min(len(media), maxRSSPersistedEntryImages))
		imageSeen := make(map[string]struct{}, maxRSSPersistedEntryImages)
		mediaURL, mediaType, thumbnail := "", "", ""
		for _, candidate := range media {
			if candidate.Thumbnail != "" && thumbnail == "" {
				thumbnail = candidate.Thumbnail
			}
			switch candidate.Kind {
			case "image":
				images = appendUniqueString(images, imageSeen, candidate.URL, maxRSSPersistedEntryImages)
				if thumbnail == "" {
					thumbnail = candidate.URL
				}
			case "video":
				if mediaURL == "" {
					mediaURL, mediaType = candidate.URL, candidate.MIMEType
				}
			}
		}

		platform, platformID, playback := resolveVideoPlatform(source.url, media)
		kind := classifyEntry(view, source.url, media)
		if platform != "" {
			kind = domainrss.EntryKindVideo
		}
		title := limitString(strings.TrimSpace(source.title), maxRSSEntryTitleBytes)
		if title == "" {
			title = "Untitled"
		}
		externalID := boundedRSSExternalID(source.externalID)
		if externalID == "" {
			externalID = source.url
		}
		if externalID == "" {
			externalID = stableDigest(title, source.author, timeValue(source.publishedAt), source.contentHTML, strconv.Itoa(index))
		}
		entry := domainrss.Entry{
			ID:             "rss-entry-" + stableDigest(id, externalID)[:32],
			SubscriptionID: id, ExternalID: externalID, URL: source.url,
			Title: title, Author: limitString(strings.TrimSpace(source.author), maxRSSEntryAuthorBytes),
			Summary:     limitString(strings.TrimSpace(source.summary), maxRSSEntrySummaryBytes),
			ContentHTML: limitString(strings.TrimSpace(source.contentHTML), maxRSSEntryContentHTMLBytes),
			Kind:        kind, ImageURLs: images, Media: media, MediaURL: mediaURL, MediaType: mediaType, ThumbnailURL: thumbnail,
			Platform: platform, PlatformVideoID: platformID, PlaybackURL: playback,
			PublishedAt: clonePreviewTime(source.publishedAt), SourceUpdatedAt: clonePreviewTime(source.sourceUpdatedAt),
			Revision: 1, CreatedAt: now, ModifiedAt: now,
		}
		entry.DownloadTarget = downloadTargetForEntry(entry)
		entry.ContentHash = contentHashForEntry(entry)
		entries = append(entries, entry)
	}
	return subscription, entries
}

func clonePreviewMedia(items []domainrss.Media) []domainrss.Media {
	if len(items) == 0 {
		return nil
	}
	cloned := make([]domainrss.Media, len(items))
	for index, item := range items {
		cloned[index] = item
		cloned[index].URL = strings.Clone(item.URL)
		cloned[index].MIMEType = strings.Clone(item.MIMEType)
		cloned[index].Kind = strings.Clone(item.Kind)
		cloned[index].Thumbnail = strings.Clone(item.Thumbnail)
	}
	return cloned
}

func clonePreviewTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneFetchMetadata(metadata fetchMetadata) fetchMetadata {
	metadata.ResolvedURL = strings.Clone(metadata.ResolvedURL)
	metadata.ETag = strings.Clone(metadata.ETag)
	metadata.LastModified = strings.Clone(metadata.LastModified)
	metadata.ValidatorURL = strings.Clone(metadata.ValidatorURL)
	return metadata
}

func previewLeaseSize(canonical string, snapshot previewFeedSnapshot, metadata fetchMetadata) int64 {
	total := previewLeaseBaseOverheadBytes + int64(len(canonical)+len(snapshot.title)+len(snapshot.siteURL)+
		len(snapshot.description)+len(snapshot.iconURL)+len(metadata.ResolvedURL)+len(metadata.ETag)+
		len(metadata.LastModified)+len(metadata.ValidatorURL))
	for _, entry := range snapshot.entries {
		total += previewEntryOverheadBytes + int64(len(entry.externalID)+len(entry.url)+len(entry.title)+
			len(entry.author)+len(entry.summary)+len(entry.contentHTML))
		for _, media := range entry.media {
			total += previewMediaOverheadBytes + int64(len(media.URL)+len(media.MIMEType)+len(media.Kind)+len(media.Thumbnail))
		}
	}
	return total
}

func (service *Service) storePreviewLease(
	canonical string,
	snapshot previewFeedSnapshot,
	metadata fetchMetadata,
	now time.Time,
) string {
	leaseBytes := previewLeaseSize(canonical, snapshot, metadata)
	if leaseBytes <= 0 || leaseBytes > maxPreviewLeaseBytes {
		return ""
	}
	service.previewMu.Lock()
	defer service.previewMu.Unlock()
	if service.previewLeases == nil {
		service.previewLeases = make(map[string]previewLease)
	}
	service.prunePreviewLeasesLocked(now)
	for len(service.previewLeases) >= maxPreviewLeases || service.previewLeaseBytes+leaseBytes > maxPreviewLeaseBytes {
		token := service.oldestEvictablePreviewLeaseLocked()
		if token == "" {
			return ""
		}
		service.removePreviewLeaseLocked(token)
	}
	token := uuid.NewString()
	expiresAt := now.Add(previewLeaseTTL)
	lease := previewLease{
		canonical: strings.Clone(canonical), snapshot: snapshot, metadata: cloneFetchMetadata(metadata),
		expiresAt: expiresAt, bytes: leaseBytes,
	}
	lease.timer = time.AfterFunc(previewLeaseTTL, func() {
		service.expirePreviewLease(token, expiresAt)
	})
	service.previewLeases[token] = lease
	service.previewLeaseBytes += leaseBytes
	return token
}

func (service *Service) claimPreviewLease(
	token, canonical string,
	now time.Time,
) (previewFeedSnapshot, fetchMetadata, previewLeaseClaimStatus) {
	token = strings.TrimSpace(token)
	if token == "" || len(token) > maxPreviewTokenBytes {
		return previewFeedSnapshot{}, fetchMetadata{}, previewLeaseMissing
	}
	service.previewMu.Lock()
	defer service.previewMu.Unlock()
	service.prunePreviewLeasesLocked(now)
	lease, ok := service.previewLeases[token]
	if !ok || lease.canonical != canonical {
		return previewFeedSnapshot{}, fetchMetadata{}, previewLeaseMissing
	}
	if lease.claimed {
		return previewFeedSnapshot{}, fetchMetadata{}, previewLeaseBusy
	}
	lease.claimed = true
	service.previewLeases[token] = lease
	return lease.snapshot, lease.metadata, previewLeaseAcquired
}

func (service *Service) releasePreviewLease(token, canonical string, now time.Time) {
	token = strings.TrimSpace(token)
	service.previewMu.Lock()
	defer service.previewMu.Unlock()
	service.prunePreviewLeasesLocked(now)
	lease, ok := service.previewLeases[token]
	if !ok || lease.canonical != canonical || !lease.claimed {
		return
	}
	lease.claimed = false
	service.previewLeases[token] = lease
}

func (service *Service) consumePreviewLease(token, canonical string) {
	token = strings.TrimSpace(token)
	service.previewMu.Lock()
	defer service.previewMu.Unlock()
	lease, ok := service.previewLeases[token]
	if !ok || lease.canonical != canonical || !lease.claimed {
		return
	}
	service.removePreviewLeaseLocked(token)
}

func (service *Service) expirePreviewLease(token string, expiresAt time.Time) {
	service.previewMu.Lock()
	defer service.previewMu.Unlock()
	lease, ok := service.previewLeases[token]
	if !ok || !lease.expiresAt.Equal(expiresAt) {
		return
	}
	// Expiry removes claimed leases too. An in-flight AddSubscription owns its
	// local snapshot copy; deleting the map entry prevents an abandoned claim
	// from pinning the global byte budget indefinitely.
	service.removePreviewLeaseLocked(token)
}

func (service *Service) prunePreviewLeasesLocked(now time.Time) {
	for token, lease := range service.previewLeases {
		if !now.Before(lease.expiresAt) {
			service.removePreviewLeaseLocked(token)
		}
	}
}

func (service *Service) oldestEvictablePreviewLeaseLocked() string {
	var oldestToken string
	var oldestExpiry time.Time
	for token, lease := range service.previewLeases {
		if lease.claimed {
			continue
		}
		if oldestToken == "" || lease.expiresAt.Before(oldestExpiry) {
			oldestToken, oldestExpiry = token, lease.expiresAt
		}
	}
	return oldestToken
}

func (service *Service) removePreviewLeaseLocked(token string) {
	lease, ok := service.previewLeases[token]
	if !ok {
		return
	}
	delete(service.previewLeases, token)
	service.previewLeaseBytes -= lease.bytes
	if service.previewLeaseBytes < 0 {
		service.previewLeaseBytes = 0
	}
	if lease.timer != nil {
		lease.timer.Stop()
	}
}
