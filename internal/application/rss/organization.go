package rss

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode"

	domainrss "xiadown/internal/domain/rss"

	"github.com/google/uuid"
)

const (
	maxRSSOrganizationTitleBytes = 512
	maxRSSCollectionItems        = 10_000
	maxRSSSourceContentBytes     = 2 << 20
	maxRSSSortOrder              = 1_000_000_000
)

func (service *Service) ListCategories(ctx context.Context) ([]domainrss.Category, error) {
	repository, err := service.requireOrganizationRepository()
	if err != nil {
		return nil, err
	}
	return repository.ListCategories(ctx)
}

func (service *Service) CreateCategory(ctx context.Context, request CreateCategoryRequest) (domainrss.Category, error) {
	repository, err := service.requireOrganizationRepository()
	if err != nil {
		return domainrss.Category{}, err
	}
	title, err := normalizeOrganizationTitle(request.Title)
	if err != nil {
		return domainrss.Category{}, err
	}
	sortOrder := 0
	if request.SortOrder != nil {
		sortOrder = normalizeSortOrder(*request.SortOrder)
	} else if items, listErr := repository.ListCategories(ctx); listErr != nil {
		return domainrss.Category{}, listErr
	} else {
		sortOrder = nextCategorySortOrder(items)
	}
	now := service.now()
	return repository.CreateCategory(ctx, domainrss.Category{
		ID: "rss-category-" + uuid.NewString(), WorkspaceID: domainrss.DefaultWorkspaceID,
		Title: title, SortOrder: sortOrder, CreatedAt: now, UpdatedAt: now, Revision: 1,
	})
}

func (service *Service) UpdateCategory(ctx context.Context, request UpdateCategoryRequest) (domainrss.Category, error) {
	repository, err := service.requireOrganizationRepository()
	if err != nil {
		return domainrss.Category{}, err
	}
	item, err := repository.GetCategory(ctx, strings.TrimSpace(request.ID))
	if err != nil {
		return domainrss.Category{}, err
	}
	if request.ExpectedRevision != nil && item.Revision != *request.ExpectedRevision {
		return domainrss.Category{}, domainrss.ErrRevisionConflict
	}
	if strings.TrimSpace(request.Title) != "" {
		item.Title, err = normalizeOrganizationTitle(request.Title)
		if err != nil {
			return domainrss.Category{}, err
		}
	}
	if request.SortOrder != nil {
		item.SortOrder = normalizeSortOrder(*request.SortOrder)
	}
	item.Revision++
	item.UpdatedAt = service.now()
	return repository.UpdateCategory(ctx, item)
}

func (service *Service) DeleteCategory(ctx context.Context, request SubscriptionRequest) error {
	repository, err := service.requireOrganizationRepository()
	if err != nil {
		return err
	}
	return repository.DeleteCategory(ctx, strings.TrimSpace(request.ID))
}

func (service *Service) ReorderCategories(ctx context.Context, request ReorderRequest) ([]domainrss.Category, error) {
	repository, err := service.requireOrganizationRepository()
	if err != nil {
		return nil, err
	}
	ids, err := normalizeOrganizationIDs(request.IDs)
	if err != nil {
		return nil, err
	}
	return repository.ReorderCategories(ctx, ids, service.now())
}

func (service *Service) ReorderSubscriptions(ctx context.Context, request ReorderSubscriptionsRequest) ([]domainrss.Subscription, error) {
	repository, err := service.requireOrganizationRepository()
	if err != nil {
		return nil, err
	}
	categoryID := strings.TrimSpace(request.CategoryID)
	if categoryID != "" {
		if _, err := repository.GetCategory(ctx, categoryID); err != nil {
			return nil, err
		}
	}
	ids, err := normalizeOrganizationIDs(request.IDs)
	if err != nil {
		return nil, err
	}
	return repository.ReorderSubscriptions(ctx, categoryID, ids, service.now())
}

func (service *Service) ListCollections(ctx context.Context) ([]domainrss.Collection, error) {
	repository, err := service.requireOrganizationRepository()
	if err != nil {
		return nil, err
	}
	return repository.ListCollections(ctx)
}

func (service *Service) CreateCollection(ctx context.Context, request CreateCollectionRequest) (domainrss.Collection, error) {
	repository, err := service.requireOrganizationRepository()
	if err != nil {
		return domainrss.Collection{}, err
	}
	title, err := normalizeOrganizationTitle(request.Title)
	if err != nil {
		return domainrss.Collection{}, err
	}
	kind, err := normalizeCollectionKind(request.Kind)
	if err != nil {
		return domainrss.Collection{}, err
	}
	viewType, err := normalizeViewType(request.ViewType)
	if err != nil {
		return domainrss.Collection{}, err
	}
	sortOrder := 0
	if request.SortOrder != nil {
		sortOrder = normalizeSortOrder(*request.SortOrder)
	} else if items, listErr := repository.ListCollections(ctx); listErr != nil {
		return domainrss.Collection{}, listErr
	} else {
		for _, item := range items {
			if item.SortOrder >= sortOrder {
				sortOrder = normalizeSortOrder(item.SortOrder + 1)
			}
		}
	}
	now := service.now()
	return repository.CreateCollection(ctx, domainrss.Collection{
		ID: "rss-collection-" + uuid.NewString(), WorkspaceID: domainrss.DefaultWorkspaceID,
		Title: title, Description: limitString(strings.TrimSpace(request.Description), 2048),
		Kind: kind, ViewType: viewType, SortOrder: sortOrder,
		CreatedAt: now, UpdatedAt: now, Revision: 1,
	})
}

func (service *Service) UpdateCollection(ctx context.Context, request UpdateCollectionRequest) (domainrss.Collection, error) {
	repository, err := service.requireOrganizationRepository()
	if err != nil {
		return domainrss.Collection{}, err
	}
	item, err := repository.GetCollection(ctx, strings.TrimSpace(request.ID))
	if err != nil {
		return domainrss.Collection{}, err
	}
	if request.ExpectedRevision != nil && item.Revision != *request.ExpectedRevision {
		return domainrss.Collection{}, domainrss.ErrRevisionConflict
	}
	if strings.TrimSpace(request.Title) != "" {
		item.Title, err = normalizeOrganizationTitle(request.Title)
		if err != nil {
			return domainrss.Collection{}, err
		}
	}
	if request.Description != nil {
		item.Description = limitString(strings.TrimSpace(*request.Description), 2048)
	}
	if strings.TrimSpace(request.ViewType) != "" {
		item.ViewType, err = normalizeViewType(request.ViewType)
		if err != nil {
			return domainrss.Collection{}, err
		}
	}
	if request.SortOrder != nil {
		item.SortOrder = normalizeSortOrder(*request.SortOrder)
	}
	item.Revision++
	item.UpdatedAt = service.now()
	return repository.UpdateCollection(ctx, item)
}

func (service *Service) DeleteCollection(ctx context.Context, request SubscriptionRequest) error {
	repository, err := service.requireOrganizationRepository()
	if err != nil {
		return err
	}
	return repository.DeleteCollection(ctx, strings.TrimSpace(request.ID))
}

func (service *Service) ListCollectionItems(ctx context.Context, request SubscriptionRequest) (domainrss.CollectionItems, error) {
	repository, err := service.requireOrganizationRepository()
	if err != nil {
		return domainrss.CollectionItems{}, err
	}
	return repository.ListCollectionItems(ctx, strings.TrimSpace(request.ID))
}

func (service *Service) ReplaceCollectionItems(ctx context.Context, request ReplaceCollectionItemsRequest) (domainrss.Collection, error) {
	repository, err := service.requireOrganizationRepository()
	if err != nil {
		return domainrss.Collection{}, err
	}
	item, err := repository.GetCollection(ctx, strings.TrimSpace(request.ID))
	if err != nil {
		return domainrss.Collection{}, err
	}
	ids, err := normalizeOrganizationIDs(request.ItemIDs)
	if err != nil {
		return domainrss.Collection{}, err
	}
	if len(ids) > maxRSSCollectionItems {
		return domainrss.Collection{}, fmt.Errorf("%w: collection contains too many items", domainrss.ErrInvalidRequest)
	}
	return repository.ReplaceCollectionItems(ctx, item.ID, item.Kind, ids, service.now())
}

func (service *Service) AddCollectionItems(ctx context.Context, request UpdateCollectionItemsRequest) (domainrss.Collection, error) {
	repository, item, ids, err := service.prepareCollectionItemMutation(ctx, request)
	if err != nil {
		return domainrss.Collection{}, err
	}
	return repository.AddCollectionItems(ctx, item.ID, item.Kind, ids, service.now())
}

func (service *Service) RemoveCollectionItems(ctx context.Context, request UpdateCollectionItemsRequest) (domainrss.Collection, error) {
	repository, item, ids, err := service.prepareCollectionItemMutation(ctx, request)
	if err != nil {
		return domainrss.Collection{}, err
	}
	return repository.RemoveCollectionItems(ctx, item.ID, item.Kind, ids, service.now())
}

func (service *Service) prepareCollectionItemMutation(
	ctx context.Context,
	request UpdateCollectionItemsRequest,
) (domainrss.OrganizationRepository, domainrss.Collection, []string, error) {
	repository, err := service.requireOrganizationRepository()
	if err != nil {
		return nil, domainrss.Collection{}, nil, err
	}
	item, err := repository.GetCollection(ctx, strings.TrimSpace(request.ID))
	if err != nil {
		return nil, domainrss.Collection{}, nil, err
	}
	ids, err := normalizeOrganizationIDs(request.ItemIDs)
	if err != nil {
		return nil, domainrss.Collection{}, nil, err
	}
	if len(ids) > maxRSSCollectionItems {
		return nil, domainrss.Collection{}, nil, fmt.Errorf("%w: collection contains too many items", domainrss.ErrInvalidRequest)
	}
	return repository, item, ids, nil
}

func (service *Service) ListSources(ctx context.Context) ([]domainrss.Source, error) {
	repository, err := service.requireOrganizationRepository()
	if err != nil {
		return nil, err
	}
	return repository.ListSources(ctx)
}

func (service *Service) CreateSource(ctx context.Context, request CreateSourceRequest) (domainrss.Source, error) {
	repository, err := service.requireOrganizationRepository()
	if err != nil {
		return domainrss.Source{}, err
	}
	kind, err := normalizeSourceKind(request.Kind)
	if err != nil {
		return domainrss.Source{}, err
	}
	handle, err := normalizeSourceHandle(request.Handle)
	if err != nil {
		return domainrss.Source{}, err
	}
	title, err := normalizeOrganizationTitle(request.Title)
	if err != nil {
		return domainrss.Source{}, err
	}
	sortOrder := 0
	if request.SortOrder != nil {
		sortOrder = normalizeSortOrder(*request.SortOrder)
	}
	now := service.now()
	id := "rss-source-" + uuid.NewString()
	subscription := domainrss.Subscription{
		ID: "rss-subscription-" + uuid.NewString(), WorkspaceID: domainrss.DefaultWorkspaceID,
		SourceAccess: domainrss.SubscriptionSourceDesktopManaged,
		FeedURL:      "xiadown-source://" + string(kind) + "/" + id, Title: title,
		ViewType: domainrss.ViewTypeArticle, ResolvedViewType: domainrss.ViewTypeArticle,
		Enabled: true, CreatedAt: now, UpdatedAt: now, Revision: 1,
	}
	return repository.CreateSource(ctx, domainrss.Source{
		ID: id, WorkspaceID: domainrss.DefaultWorkspaceID, SubscriptionID: subscription.ID,
		Kind: kind, Handle: handle, Title: title, Enabled: true, SortOrder: sortOrder,
		CreatedAt: now, UpdatedAt: now, Revision: 1,
	}, subscription)
}

func (service *Service) UpdateSource(ctx context.Context, request UpdateSourceRequest) (domainrss.Source, error) {
	repository, err := service.requireOrganizationRepository()
	if err != nil {
		return domainrss.Source{}, err
	}
	item, err := repository.GetSource(ctx, strings.TrimSpace(request.ID))
	if err != nil {
		return domainrss.Source{}, err
	}
	if request.ExpectedRevision != nil && item.Revision != *request.ExpectedRevision {
		return domainrss.Source{}, domainrss.ErrRevisionConflict
	}
	subscription, err := service.repository.GetSubscription(ctx, item.SubscriptionID)
	if err != nil {
		return domainrss.Source{}, err
	}
	if strings.TrimSpace(request.Title) != "" {
		item.Title, err = normalizeOrganizationTitle(request.Title)
		if err != nil {
			return domainrss.Source{}, err
		}
		subscription.Title = item.Title
	}
	if request.Enabled != nil {
		item.Enabled = *request.Enabled
		subscription.Enabled = *request.Enabled
	}
	if request.SortOrder != nil {
		item.SortOrder = normalizeSortOrder(*request.SortOrder)
	}
	now := service.now()
	item.Revision++
	item.UpdatedAt = now
	subscription.Revision++
	subscription.UpdatedAt = now
	return repository.UpdateSource(ctx, item, subscription)
}

func (service *Service) DeleteSource(ctx context.Context, request SubscriptionRequest) error {
	repository, err := service.requireOrganizationRepository()
	if err != nil {
		return err
	}
	item, err := repository.GetSource(ctx, strings.TrimSpace(request.ID))
	if err != nil {
		return err
	}
	return service.repository.DeleteSubscription(ctx, item.SubscriptionID, service.now())
}

func (service *Service) CreateSourceEntry(ctx context.Context, request CreateSourceEntryRequest) (domainrss.Entry, error) {
	repository, err := service.requireOrganizationRepository()
	if err != nil {
		return domainrss.Entry{}, err
	}
	source, err := repository.GetSource(ctx, strings.TrimSpace(request.SourceID))
	if err != nil {
		return domainrss.Entry{}, err
	}
	if !source.Enabled {
		return domainrss.Entry{}, fmt.Errorf("%w: source is disabled", domainrss.ErrInvalidRequest)
	}
	title, err := normalizeOrganizationTitle(request.Title)
	if err != nil {
		return domainrss.Entry{}, err
	}
	entryURL, err := normalizeOptionalSourceEntryURL(request.URL)
	if err != nil {
		return domainrss.Entry{}, err
	}
	externalID := limitString(strings.TrimSpace(request.ExternalID), 1024)
	if externalID == "" {
		externalID = uuid.NewString()
	}
	now := service.now()
	publishedAt := request.PublishedAt
	if publishedAt == nil {
		publishedAt = &now
	}
	contentHTML := limitString(strings.TrimSpace(request.ContentHTML), maxRSSSourceContentBytes)
	entry := domainrss.Entry{
		ID:             "rss-entry-" + uuid.NewSHA1(uuid.NameSpaceURL, []byte(source.SubscriptionID+"\x00"+externalID)).String(),
		SubscriptionID: source.SubscriptionID, ExternalID: externalID, URL: entryURL,
		Title: title, Author: limitString(strings.TrimSpace(request.Author), 512),
		Summary: limitString(strings.TrimSpace(request.Summary), 4096), ContentHTML: contentHTML,
		Kind: domainrss.EntryKindArticle, ImageURLs: []string{}, Media: []domainrss.Media{},
		PublishedAt: publishedAt, SourceUpdatedAt: publishedAt,
		CreatedAt: now, ModifiedAt: now,
	}
	entry.ContentHash = sourceEntryContentHash(entry)
	subscription, err := service.repository.GetSubscription(ctx, source.SubscriptionID)
	if err != nil {
		return domainrss.Entry{}, err
	}
	subscription.Revision++
	subscription.UpdatedAt = now
	if _, err := service.repository.UpsertFeed(ctx, domainrss.FeedUpdate{
		Subscription: subscription, Entries: []domainrss.Entry{entry},
	}); err != nil {
		return domainrss.Entry{}, err
	}
	stored, err := repository.GetSourceEntry(ctx, source.ID, externalID)
	if err != nil {
		return domainrss.Entry{}, err
	}
	stored.ContentHTML = sanitizeEntryHTML(stored.ContentHTML, stored.URL)
	return stored, nil
}

func (service *Service) requireOrganizationRepository() (domainrss.OrganizationRepository, error) {
	if service == nil || service.organizationRepository == nil {
		return nil, fmt.Errorf("rss organization repository unavailable")
	}
	return service.organizationRepository, nil
}

func normalizeOrganizationTitle(value string) (string, error) {
	value = limitString(strings.TrimSpace(value), maxRSSOrganizationTitleBytes)
	if value == "" {
		return "", fmt.Errorf("%w: title is required", domainrss.ErrInvalidRequest)
	}
	return value, nil
}

func normalizeSortOrder(value int) int {
	if value < 0 {
		return 0
	}
	if value > maxRSSSortOrder {
		return maxRSSSortOrder
	}
	return value
}

func nextCategorySortOrder(items []domainrss.Category) int {
	result := 0
	for _, item := range items {
		if item.SortOrder >= result {
			result = normalizeSortOrder(item.SortOrder + 1)
		}
	}
	return result
}

func normalizeOrganizationIDs(values []string) ([]string, error) {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			return nil, fmt.Errorf("%w: item id is required", domainrss.ErrInvalidRequest)
		}
		if _, ok := seen[value]; ok {
			return nil, fmt.Errorf("%w: duplicate item id", domainrss.ErrInvalidRequest)
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result, nil
}

func normalizeCollectionKind(value string) (domainrss.CollectionKind, error) {
	switch kind := domainrss.CollectionKind(strings.TrimSpace(value)); kind {
	case domainrss.CollectionKindSubscriptions, domainrss.CollectionKindEntries:
		return kind, nil
	default:
		return "", fmt.Errorf("%w: unsupported collection kind", domainrss.ErrInvalidRequest)
	}
}

func normalizeSourceKind(value string) (domainrss.SourceKind, error) {
	switch kind := domainrss.SourceKind(strings.TrimSpace(value)); kind {
	case domainrss.SourceKindInbox, domainrss.SourceKindNotification:
		return kind, nil
	default:
		return "", fmt.Errorf("%w: unsupported source kind", domainrss.ErrInvalidRequest)
	}
}

func normalizeSourceKindFilter(value string) (domainrss.SourceKind, error) {
	if strings.TrimSpace(value) == "" {
		return "", nil
	}
	return normalizeSourceKind(value)
}

func normalizeSourceHandle(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	for _, char := range value {
		if unicode.IsLetter(char) || unicode.IsDigit(char) || char == '.' || char == '_' || char == '-' {
			builder.WriteRune(char)
		}
	}
	result := limitString(builder.String(), 64)
	if result == "" {
		return "", fmt.Errorf("%w: source handle is required", domainrss.ErrInvalidRequest)
	}
	return result, nil
}

func normalizeOptionalSourceEntryURL(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return "", fmt.Errorf("%w: entry URL must use HTTP or HTTPS", domainrss.ErrInvalidRequest)
	}
	return limitString(parsed.String(), 4096), nil
}

func sourceEntryContentHash(entry domainrss.Entry) string {
	published := ""
	if entry.PublishedAt != nil {
		published = entry.PublishedAt.UTC().Format(time.RFC3339Nano)
	}
	digest := sha256.Sum256([]byte(strings.Join([]string{
		entry.ExternalID, entry.URL, entry.Title, entry.Author, entry.Summary, entry.ContentHTML, published,
	}, "\x00")))
	return hex.EncodeToString(digest[:])
}
