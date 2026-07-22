package wails

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	applicationrss "xiadown/internal/application/rss"
	domainrss "xiadown/internal/domain/rss"

	xhtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

type rssService interface {
	ListSubscriptions(context.Context) ([]domainrss.Subscription, error)
	PreviewSubscription(context.Context, applicationrss.PreviewSubscriptionRequest) (applicationrss.PreviewSubscriptionResult, error)
	AddSubscription(context.Context, applicationrss.AddSubscriptionRequest) (domainrss.Subscription, error)
	UpdateSubscription(context.Context, applicationrss.UpdateSubscriptionRequest) (domainrss.Subscription, error)
	DeleteSubscription(context.Context, applicationrss.SubscriptionRequest) error
	Refresh(context.Context, applicationrss.RefreshRequest) (applicationrss.RefreshResult, error)
	BackfillHistory(context.Context, applicationrss.BackfillHistoryRequest) (applicationrss.BackfillHistoryResult, error)
	ListEntries(context.Context, applicationrss.ListEntriesRequest) (domainrss.EntryPage, error)
	GetEntry(context.Context, applicationrss.SubscriptionRequest) (domainrss.Entry, error)
	SetEntryRead(context.Context, applicationrss.SetEntryReadRequest) (domainrss.EntryState, error)
	MarkAllRead(context.Context, applicationrss.MarkAllReadRequest) (applicationrss.MarkAllReadResult, error)
	SetEntryState(context.Context, applicationrss.SetEntryStateRequest) (domainrss.EntryState, error)
	ListChanges(context.Context, applicationrss.ListChangesRequest) (domainrss.ChangePage, error)
	ListDiscovery(context.Context, applicationrss.DiscoveryRequest) (applicationrss.DiscoveryResult, error)
	ListCategories(context.Context) ([]domainrss.Category, error)
	CreateCategory(context.Context, applicationrss.CreateCategoryRequest) (domainrss.Category, error)
	UpdateCategory(context.Context, applicationrss.UpdateCategoryRequest) (domainrss.Category, error)
	DeleteCategory(context.Context, applicationrss.SubscriptionRequest) error
	ReorderCategories(context.Context, applicationrss.ReorderRequest) ([]domainrss.Category, error)
	ReorderSubscriptions(context.Context, applicationrss.ReorderSubscriptionsRequest) ([]domainrss.Subscription, error)
	ListCollections(context.Context) ([]domainrss.Collection, error)
	CreateCollection(context.Context, applicationrss.CreateCollectionRequest) (domainrss.Collection, error)
	UpdateCollection(context.Context, applicationrss.UpdateCollectionRequest) (domainrss.Collection, error)
	DeleteCollection(context.Context, applicationrss.SubscriptionRequest) error
	ListCollectionItems(context.Context, applicationrss.SubscriptionRequest) (domainrss.CollectionItems, error)
	ReplaceCollectionItems(context.Context, applicationrss.ReplaceCollectionItemsRequest) (domainrss.Collection, error)
	AddCollectionItems(context.Context, applicationrss.UpdateCollectionItemsRequest) (domainrss.Collection, error)
	RemoveCollectionItems(context.Context, applicationrss.UpdateCollectionItemsRequest) (domainrss.Collection, error)
	ListSources(context.Context) ([]domainrss.Source, error)
	CreateSource(context.Context, applicationrss.CreateSourceRequest) (domainrss.Source, error)
	UpdateSource(context.Context, applicationrss.UpdateSourceRequest) (domainrss.Source, error)
	DeleteSource(context.Context, applicationrss.SubscriptionRequest) error
	CreateSourceEntry(context.Context, applicationrss.CreateSourceEntryRequest) (domainrss.Entry, error)
}

type RSSDesktopEntryImageLoader interface {
	LoadDesktopEntryImage(context.Context, string, string) ([]byte, string, error)
}

type RSSSaveFileDialog interface {
	SaveMainFileDialog(SaveFileDialogOptions) (string, error)
}

type RSSHandlerConfig struct {
	ResourceBaseURL string
	ImageLoader     RSSDesktopEntryImageLoader
	SaveDialog      RSSSaveFileDialog
}

type RSSHandler struct {
	service         rssService
	resourceBaseURL string
	imageLoader     RSSDesktopEntryImageLoader
	saveDialog      RSSSaveFileDialog
}

const (
	rssProjectedIndexedResourceLimit = 64
	maxRSSSavedImageBytes            = 12 << 20
	maxRSSSaveEntryIDRunes           = 255
	maxRSSSaveSuggestedNameRunes     = 160
	maxRSSSaveSuggestedNameBytes     = 200
	maxRSSSaveDialogTextRunes        = 128
)

func NewRSSHandler(service rssService, resourceBaseURL ...string) *RSSHandler {
	config := RSSHandlerConfig{}
	if len(resourceBaseURL) > 0 {
		config.ResourceBaseURL = resourceBaseURL[0]
	}
	return NewRSSHandlerWithConfig(service, config)
}

func NewRSSHandlerWithConfig(service rssService, config RSSHandlerConfig) *RSSHandler {
	return &RSSHandler{
		service:         service,
		resourceBaseURL: strings.TrimRight(strings.TrimSpace(config.ResourceBaseURL), "/"),
		imageLoader:     config.ImageLoader,
		saveDialog:      config.SaveDialog,
	}
}

func (handler *RSSHandler) ServiceName() string { return "RSSHandler" }

func (handler *RSSHandler) ListSubscriptions(ctx context.Context) ([]domainrss.Subscription, error) {
	items, err := handler.service.ListSubscriptions(ctx)
	if err != nil {
		return nil, err
	}
	for index := range items {
		items[index] = handler.projectSubscription(items[index])
	}
	return items, nil
}

func (handler *RSSHandler) PreviewSubscription(ctx context.Context, request applicationrss.PreviewSubscriptionRequest) (applicationrss.PreviewSubscriptionResult, error) {
	result, err := handler.service.PreviewSubscription(ctx, request)
	if err != nil {
		return applicationrss.PreviewSubscriptionResult{}, err
	}
	// Preview entities are intentionally not persisted, so entity-slot routes
	// would be broken. Keep preview text-only until the subscription is saved.
	result.Subscription.IconURL = ""
	for index := range result.Entries {
		result.Entries[index] = stripRSSPreviewResources(result.Entries[index])
	}
	return result, nil
}

func (handler *RSSHandler) AddSubscription(ctx context.Context, request applicationrss.AddSubscriptionRequest) (domainrss.Subscription, error) {
	item, err := handler.service.AddSubscription(ctx, request)
	return handler.projectSubscription(item), err
}

func (handler *RSSHandler) UpdateSubscription(ctx context.Context, request applicationrss.UpdateSubscriptionRequest) (domainrss.Subscription, error) {
	item, err := handler.service.UpdateSubscription(ctx, request)
	return handler.projectSubscription(item), err
}

func (handler *RSSHandler) DeleteSubscription(ctx context.Context, request applicationrss.SubscriptionRequest) error {
	return handler.service.DeleteSubscription(ctx, request)
}

func (handler *RSSHandler) Refresh(ctx context.Context, request applicationrss.RefreshRequest) (applicationrss.RefreshResult, error) {
	return handler.service.Refresh(ctx, request)
}

func (handler *RSSHandler) BackfillHistory(ctx context.Context, request applicationrss.BackfillHistoryRequest) (applicationrss.BackfillHistoryResult, error) {
	return handler.service.BackfillHistory(ctx, request)
}

func (handler *RSSHandler) ListEntries(ctx context.Context, request applicationrss.ListEntriesRequest) (domainrss.EntryPage, error) {
	page, err := handler.service.ListEntries(ctx, request)
	if err != nil {
		return domainrss.EntryPage{}, err
	}
	// Collection rows intentionally omit the potentially large article body.
	// The selected entry is hydrated through GetEntry, keeping scrolling and
	// search payloads bounded without changing the public sync projection.
	for index := range page.Items {
		page.Items[index].ContentHTML = ""
		page.Items[index] = handler.projectEntry(page.Items[index])
	}
	return page, nil
}

func (handler *RSSHandler) GetEntry(ctx context.Context, request applicationrss.SubscriptionRequest) (domainrss.Entry, error) {
	item, err := handler.service.GetEntry(ctx, request)
	return handler.projectEntry(item), err
}

type RSSSaveEntryImageRequest struct {
	EntryID       string `json:"entryId"`
	Slot          string `json:"slot"`
	SuggestedName string `json:"suggestedName,omitempty"`
	DialogTitle   string `json:"dialogTitle,omitempty"`
	FilterName    string `json:"filterName,omitempty"`
	ButtonText    string `json:"buttonText,omitempty"`
}

type RSSSaveEntryImageResult struct {
	Saved bool `json:"saved"`
}

func (handler *RSSHandler) SaveEntryImage(
	ctx context.Context,
	request RSSSaveEntryImageRequest,
) (RSSSaveEntryImageResult, error) {
	if handler == nil || handler.imageLoader == nil || handler.saveDialog == nil {
		return RSSSaveEntryImageResult{}, fmt.Errorf("RSS image saving unavailable")
	}
	entryID := strings.TrimSpace(request.EntryID)
	slot := strings.TrimSpace(request.Slot)
	if !validRSSSaveEntryID(entryID) || !validRSSSaveImageSlot(slot) {
		return RSSSaveEntryImageResult{}, fmt.Errorf("invalid RSS image reference")
	}
	data, contentType, err := handler.imageLoader.LoadDesktopEntryImage(ctx, entryID, slot)
	if err != nil {
		return RSSSaveEntryImageResult{}, fmt.Errorf("load RSS image: %w", err)
	}
	if len(data) == 0 || len(data) > maxRSSSavedImageBytes {
		return RSSSaveEntryImageResult{}, fmt.Errorf("RSS image size is invalid")
	}
	extension, ok := rssSavedImageExtension(contentType)
	if !ok {
		return RSSSaveEntryImageResult{}, fmt.Errorf("RSS image type is unsupported")
	}
	title := sanitizedRSSSaveDialogText(
		request.DialogTitle,
		"Save image",
		maxRSSSaveDialogTextRunes,
	)
	filterName := sanitizedRSSSaveDialogText(
		request.FilterName,
		"Images",
		maxRSSSaveDialogTextRunes,
	)
	buttonText := sanitizedRSSSaveDialogText(
		request.ButtonText,
		"Save",
		maxRSSSaveDialogTextRunes,
	)
	destination, err := handler.saveDialog.SaveMainFileDialog(SaveFileDialogOptions{
		Title:      title,
		Message:    title,
		Filename:   rssSavedImageFilename(request.SuggestedName, extension),
		ButtonText: buttonText,
		Filters: []SaveFileDialogFilter{{
			DisplayName: filterName,
			Pattern:     "*" + extension,
		}},
	})
	if err != nil {
		return RSSSaveEntryImageResult{}, fmt.Errorf("open RSS image save dialog: %w", err)
	}
	if destination == "" {
		return RSSSaveEntryImageResult{Saved: false}, nil
	}
	if err := writeRSSSavedImage(destination, data); err != nil {
		return RSSSaveEntryImageResult{}, err
	}
	return RSSSaveEntryImageResult{Saved: true}, nil
}

func validRSSSaveEntryID(value string) bool {
	if value == "" || !utf8.ValidString(value) {
		return false
	}
	count := 0
	for _, item := range value {
		count++
		if count > maxRSSSaveEntryIDRunes || unicode.IsSpace(item) || unicode.IsControl(item) {
			return false
		}
	}
	return true
}

func validRSSSaveImageSlot(slot string) bool {
	if slot == "thumbnail" {
		return true
	}
	number := ""
	switch {
	case strings.HasPrefix(slot, "image-"):
		number = strings.TrimPrefix(slot, "image-")
	case strings.HasPrefix(slot, "media-"):
		number = strings.TrimPrefix(slot, "media-")
		number = strings.TrimSuffix(number, "-thumbnail")
	default:
		return false
	}
	if number == "" || (len(number) > 1 && number[0] == '0') {
		return false
	}
	for index := 0; index < len(number); index++ {
		if number[index] < '0' || number[index] > '9' {
			return false
		}
	}
	index, err := strconv.Atoi(number)
	return err == nil && index >= 0 && index < rssProjectedIndexedResourceLimit
}

func rssSavedImageExtension(contentType string) (string, bool) {
	mediaType, _, err := mime.ParseMediaType(strings.TrimSpace(contentType))
	if err != nil {
		return "", false
	}
	switch strings.ToLower(mediaType) {
	case "image/jpeg":
		return ".jpg", true
	case "image/png":
		return ".png", true
	case "image/gif":
		return ".gif", true
	case "image/webp":
		return ".webp", true
	case "image/avif":
		return ".avif", true
	case "image/bmp":
		return ".bmp", true
	case "image/vnd.microsoft.icon", "image/x-icon":
		return ".ico", true
	default:
		return "", false
	}
}

func sanitizedRSSSaveDialogText(value string, fallback string, maximum int) string {
	value = strings.Map(func(item rune) rune {
		if unicode.IsControl(item) {
			return -1
		}
		if unicode.IsSpace(item) {
			return ' '
		}
		return item
	}, value)
	value = strings.Join(strings.Fields(value), " ")
	value = limitedRSSRunes(value, maximum)
	if value != "" {
		return value
	}
	return limitedRSSRunes(fallback, maximum)
}

func rssSavedImageFilename(value string, extension string) string {
	value = strings.Map(func(item rune) rune {
		if unicode.IsControl(item) {
			return -1
		}
		if unicode.IsSpace(item) {
			return ' '
		}
		switch item {
		case '<', '>', ':', '"', '/', '\\', '|', '?', '*':
			return '-'
		default:
			return item
		}
	}, value)
	value = strings.Trim(strings.Join(strings.Fields(value), " "), " .")
	if existing := strings.ToLower(filepath.Ext(value)); isRSSSavedImageExtension(existing) {
		value = strings.TrimSpace(strings.TrimSuffix(value, filepath.Ext(value)))
	}
	value = strings.Trim(limitedRSSFilename(value), " .")
	if value == "" || value == "." || value == ".." {
		value = "rss-image"
	}
	if isReservedRSSSavedImageFilename(value) {
		value = "_" + value
	}
	return value + extension
}

func isRSSSavedImageExtension(value string) bool {
	switch strings.ToLower(value) {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".avif", ".bmp", ".ico":
		return true
	default:
		return false
	}
}

func isReservedRSSSavedImageFilename(value string) bool {
	name := strings.ToUpper(strings.TrimSpace(value))
	if separator := strings.IndexRune(name, '.'); separator >= 0 {
		name = strings.TrimSpace(name[:separator])
	}
	switch name {
	case "CON", "PRN", "AUX", "NUL", "COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
		return true
	default:
		return false
	}
}

func limitedRSSFilename(value string) string {
	var result strings.Builder
	runeCount := 0
	for _, item := range value {
		itemBytes := utf8.RuneLen(item)
		if itemBytes < 1 ||
			runeCount >= maxRSSSaveSuggestedNameRunes ||
			result.Len()+itemBytes > maxRSSSaveSuggestedNameBytes {
			break
		}
		result.WriteRune(item)
		runeCount++
	}
	return result.String()
}

func limitedRSSRunes(value string, maximum int) string {
	if maximum <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) > maximum {
		runes = runes[:maximum]
	}
	return string(runes)
}

func writeRSSSavedImage(destination string, data []byte) error {
	if destination == "" || strings.ContainsRune(destination, 0) || !filepath.IsAbs(destination) {
		return fmt.Errorf("save RSS image: invalid destination")
	}
	if len(data) == 0 || len(data) > maxRSSSavedImageBytes {
		return fmt.Errorf("save RSS image: invalid image size")
	}
	destination = filepath.Clean(destination)
	directory := filepath.Dir(destination)
	info, err := os.Stat(directory)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("save RSS image: destination directory unavailable")
	}
	if existing, statErr := os.Lstat(destination); statErr == nil && existing.IsDir() {
		return fmt.Errorf("save RSS image: destination is a directory")
	} else if statErr != nil && !os.IsNotExist(statErr) {
		return fmt.Errorf("save RSS image: inspect destination: %w", statErr)
	}
	temporary, err := os.CreateTemp(directory, ".xiadown-rss-image-*")
	if err != nil {
		return fmt.Errorf("save RSS image: create temporary file: %w", err)
	}
	temporaryPath := temporary.Name()
	published := false
	defer func() {
		_ = temporary.Close()
		if !published {
			_ = os.Remove(temporaryPath)
		}
	}()
	written, err := io.Copy(temporary, bytes.NewReader(data))
	if err != nil || written != int64(len(data)) {
		if err == nil {
			err = io.ErrShortWrite
		}
		return fmt.Errorf("save RSS image: write temporary file: %w", err)
	}
	if err := temporary.Chmod(0o644); err != nil {
		return fmt.Errorf("save RSS image: set file permissions: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("save RSS image: sync temporary file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("save RSS image: close temporary file: %w", err)
	}
	if err := replaceRSSSavedImageFile(temporaryPath, destination); err != nil {
		return fmt.Errorf("save RSS image: publish file: %w", err)
	}
	published = true
	return nil
}

func (handler *RSSHandler) SetEntryRead(ctx context.Context, request applicationrss.SetEntryReadRequest) (domainrss.EntryState, error) {
	return handler.service.SetEntryRead(ctx, request)
}

func (handler *RSSHandler) MarkAllRead(ctx context.Context, request applicationrss.MarkAllReadRequest) (applicationrss.MarkAllReadResult, error) {
	return handler.service.MarkAllRead(ctx, request)
}

func (handler *RSSHandler) SetEntryState(ctx context.Context, request applicationrss.SetEntryStateRequest) (domainrss.EntryState, error) {
	return handler.service.SetEntryState(ctx, request)
}

func (handler *RSSHandler) ListChanges(ctx context.Context, request applicationrss.ListChangesRequest) (domainrss.ChangePage, error) {
	return handler.service.ListChanges(ctx, request)
}

func (handler *RSSHandler) ListDiscovery(ctx context.Context, request applicationrss.DiscoveryRequest) (applicationrss.DiscoveryResult, error) {
	result, err := handler.service.ListDiscovery(ctx, request)
	if err != nil {
		return applicationrss.DiscoveryResult{}, err
	}
	// Categories are taxonomy, not publishers. The desktop uses the stable
	// category emoji set instead of presenting the favicon of an
	// arbitrary popular route as though it represented the whole category.
	for index := range result.Categories {
		result.Categories[index].IconURL = ""
	}
	for index := range result.Routes {
		result.Routes[index].IconURL = handler.discoveryResourceURL(
			applicationrss.DiscoveryResourceRouteIcon,
			result.Routes[index].ID,
		)
	}
	return result, nil
}

func (handler *RSSHandler) ListCategories(ctx context.Context) ([]domainrss.Category, error) {
	return handler.service.ListCategories(ctx)
}

func (handler *RSSHandler) CreateCategory(ctx context.Context, request applicationrss.CreateCategoryRequest) (domainrss.Category, error) {
	return handler.service.CreateCategory(ctx, request)
}

func (handler *RSSHandler) UpdateCategory(ctx context.Context, request applicationrss.UpdateCategoryRequest) (domainrss.Category, error) {
	return handler.service.UpdateCategory(ctx, request)
}

func (handler *RSSHandler) DeleteCategory(ctx context.Context, request applicationrss.SubscriptionRequest) error {
	return handler.service.DeleteCategory(ctx, request)
}

func (handler *RSSHandler) ReorderCategories(ctx context.Context, request applicationrss.ReorderRequest) ([]domainrss.Category, error) {
	return handler.service.ReorderCategories(ctx, request)
}

func (handler *RSSHandler) ReorderSubscriptions(ctx context.Context, request applicationrss.ReorderSubscriptionsRequest) ([]domainrss.Subscription, error) {
	items, err := handler.service.ReorderSubscriptions(ctx, request)
	for index := range items {
		items[index] = handler.projectSubscription(items[index])
	}
	return items, err
}

func (handler *RSSHandler) ListCollections(ctx context.Context) ([]domainrss.Collection, error) {
	return handler.service.ListCollections(ctx)
}

func (handler *RSSHandler) CreateCollection(ctx context.Context, request applicationrss.CreateCollectionRequest) (domainrss.Collection, error) {
	return handler.service.CreateCollection(ctx, request)
}

func (handler *RSSHandler) UpdateCollection(ctx context.Context, request applicationrss.UpdateCollectionRequest) (domainrss.Collection, error) {
	return handler.service.UpdateCollection(ctx, request)
}

func (handler *RSSHandler) DeleteCollection(ctx context.Context, request applicationrss.SubscriptionRequest) error {
	return handler.service.DeleteCollection(ctx, request)
}

func (handler *RSSHandler) ListCollectionItems(ctx context.Context, request applicationrss.SubscriptionRequest) (domainrss.CollectionItems, error) {
	return handler.service.ListCollectionItems(ctx, request)
}

func (handler *RSSHandler) ReplaceCollectionItems(ctx context.Context, request applicationrss.ReplaceCollectionItemsRequest) (domainrss.Collection, error) {
	return handler.service.ReplaceCollectionItems(ctx, request)
}

func (handler *RSSHandler) AddCollectionItems(ctx context.Context, request applicationrss.UpdateCollectionItemsRequest) (domainrss.Collection, error) {
	return handler.service.AddCollectionItems(ctx, request)
}

func (handler *RSSHandler) RemoveCollectionItems(ctx context.Context, request applicationrss.UpdateCollectionItemsRequest) (domainrss.Collection, error) {
	return handler.service.RemoveCollectionItems(ctx, request)
}

func (handler *RSSHandler) ListSources(ctx context.Context) ([]domainrss.Source, error) {
	return handler.service.ListSources(ctx)
}

func (handler *RSSHandler) CreateSource(ctx context.Context, request applicationrss.CreateSourceRequest) (domainrss.Source, error) {
	return handler.service.CreateSource(ctx, request)
}

func (handler *RSSHandler) UpdateSource(ctx context.Context, request applicationrss.UpdateSourceRequest) (domainrss.Source, error) {
	return handler.service.UpdateSource(ctx, request)
}

func (handler *RSSHandler) DeleteSource(ctx context.Context, request applicationrss.SubscriptionRequest) error {
	return handler.service.DeleteSource(ctx, request)
}

func (handler *RSSHandler) CreateSourceEntry(ctx context.Context, request applicationrss.CreateSourceEntryRequest) (domainrss.Entry, error) {
	item, err := handler.service.CreateSourceEntry(ctx, request)
	return handler.projectEntry(item), err
}

func (handler *RSSHandler) projectSubscription(item domainrss.Subscription) domainrss.Subscription {
	if strings.TrimSpace(item.IconURL) == "" {
		return item
	}
	item.IconURL = handler.subscriptionResourceURL(item.ID, item.Revision)
	return item
}

func (handler *RSSHandler) projectEntry(item domainrss.Entry) domainrss.Entry {
	original := item
	resourceURLs := make(map[string]string, len(original.ImageURLs)+len(original.Media)*2+1)
	// A feed commonly repeats one original image in thumbnail, content-image,
	// and image-media fields. Canonicalize those aliases to the first full-size
	// image slot so previews do not inherit the stricter thumbnail pixel budget.
	imageResourceURLs := make(map[string]string, len(original.ImageURLs)+len(original.Media)+1)
	projectImage := func(source, slot string) string {
		source = strings.TrimSpace(source)
		if source == "" {
			return ""
		}
		if projected := imageResourceURLs[source]; projected != "" {
			return projected
		}
		projected := handler.entryResourceURL(original.ID, slot, original.Revision)
		if projected == "" {
			return ""
		}
		imageResourceURLs[source] = projected
		resourceURLs[source] = projected
		return projected
	}
	item.ImageURLs = make([]string, 0, min(len(original.ImageURLs), rssProjectedIndexedResourceLimit))
	for index, source := range original.ImageURLs {
		if index >= rssProjectedIndexedResourceLimit {
			break
		}
		projected := projectImage(source, "image-"+strconv.Itoa(index))
		if projected == "" {
			continue
		}
		item.ImageURLs = append(item.ImageURLs, projected)
	}
	item.Media = make([]domainrss.Media, 0, min(len(original.Media), rssProjectedIndexedResourceLimit))
	for index, media := range original.Media {
		if index >= rssProjectedIndexedResourceLimit {
			break
		}
		projectedURL := ""
		if strings.EqualFold(strings.TrimSpace(media.Kind), "image") {
			projectedURL = projectImage(media.URL, "media-"+strconv.Itoa(index))
		} else {
			projectedURL = handler.entryResourceURL(original.ID, "media-"+strconv.Itoa(index), original.Revision)
		}
		if strings.TrimSpace(media.URL) == "" || projectedURL == "" {
			continue
		}
		resourceURLs[media.URL] = projectedURL
		media.URL = projectedURL
		if strings.TrimSpace(media.Thumbnail) != "" {
			projectedThumbnail := projectImage(
				original.Media[index].Thumbnail,
				"media-"+strconv.Itoa(index)+"-thumbnail",
			)
			media.Thumbnail = projectedThumbnail
		} else {
			media.Thumbnail = ""
		}
		item.Media = append(item.Media, media)
	}
	item.ThumbnailURL = projectImage(original.ThumbnailURL, "thumbnail")
	item.MediaURL = resourceURLs[original.MediaURL]
	item.ContentHTML = projectRSSContentHTML(original.ContentHTML, resourceURLs)
	return item
}

func stripRSSPreviewResources(item domainrss.Entry) domainrss.Entry {
	item.ContentHTML = projectRSSContentHTML(item.ContentHTML, nil)
	item.ImageURLs = []string{}
	item.Media = []domainrss.Media{}
	item.MediaURL = ""
	item.MediaType = ""
	item.ThumbnailURL = ""
	item.Platform = ""
	item.PlatformVideoID = ""
	item.PlaybackURL = ""
	item.DownloadTarget = ""
	return item
}

func (handler *RSSHandler) subscriptionResourceURL(id string, revisions ...int64) string {
	if handler == nil || handler.resourceBaseURL == "" || strings.TrimSpace(id) == "" {
		return ""
	}
	return versionRSSProjectedResourceURL(
		handler.resourceBaseURL+"/api/rss/subscriptions/"+url.PathEscape(strings.TrimSpace(id))+"/icon",
		revisions,
	)
}

func (handler *RSSHandler) entryResourceURL(id, slot string, revisions ...int64) string {
	if handler == nil || handler.resourceBaseURL == "" || strings.TrimSpace(id) == "" || strings.TrimSpace(slot) == "" {
		return ""
	}
	return versionRSSProjectedResourceURL(
		handler.resourceBaseURL+"/api/rss/entries/"+url.PathEscape(strings.TrimSpace(id))+"/resources/"+url.PathEscape(strings.TrimSpace(slot)),
		revisions,
	)
}

func versionRSSProjectedResourceURL(rawURL string, revisions []int64) string {
	if len(revisions) == 0 || revisions[0] <= 0 {
		return rawURL
	}
	// The query contains only an entity revision, never a remote source. This
	// keeps the opaque endpoint stable within a revision while giving Chromium
	// and the renderer session cache a new identity when a slot's source changes.
	return rawURL + "?v=" + strconv.FormatInt(revisions[0], 10)
}

func (handler *RSSHandler) discoveryResourceURL(kind applicationrss.DiscoveryResourceKind, id string) string {
	if handler == nil || handler.resourceBaseURL == "" || strings.TrimSpace(id) == "" {
		return ""
	}
	switch kind {
	case applicationrss.DiscoveryResourceCategoryIcon, applicationrss.DiscoveryResourceRouteIcon:
	default:
		return ""
	}
	return handler.resourceBaseURL + "/api/rss/discovery/" + string(kind) + "/" +
		url.PathEscape(strings.TrimSpace(id)) + "/icon"
}

func projectRSSContentHTML(markup string, resourceURLs map[string]string) string {
	markup = strings.TrimSpace(markup)
	if markup == "" {
		return ""
	}
	contextNode := &xhtml.Node{Type: xhtml.ElementNode, DataAtom: atom.Div, Data: "div"}
	nodes, err := xhtml.ParseFragment(strings.NewReader(markup), contextNode)
	if err != nil {
		return ""
	}
	var project func(*xhtml.Node) bool
	project = func(node *xhtml.Node) bool {
		if node.Type == xhtml.ElementNode {
			tag := strings.ToLower(node.Data)
			switch tag {
			case "img", "source":
				if !projectRSSResourceAttribute(node, "src", resourceURLs) {
					return false
				}
				if tag == "img" {
					setRSSResourceAttribute(node, "loading", "lazy")
					setRSSResourceAttribute(node, "decoding", "async")
				}
			case "video", "audio":
				projectRSSResourceAttribute(node, "src", resourceURLs)
				if tag == "video" {
					projectRSSResourceAttribute(node, "poster", resourceURLs)
				}
			case "figure":
				if hasRSSContentAttribute(node, "data-xiadown-rss-video-provider") && resourceURLs == nil {
					return false
				}
			}
		}
		for child := node.FirstChild; child != nil; {
			next := child.NextSibling
			if !project(child) {
				node.RemoveChild(child)
			}
			child = next
		}
		return true
	}
	var output bytes.Buffer
	for _, node := range nodes {
		if !project(node) {
			continue
		}
		if err := xhtml.Render(&output, node); err != nil {
			return ""
		}
	}
	return strings.TrimSpace(output.String())
}

func hasRSSContentAttribute(node *xhtml.Node, name string) bool {
	if node == nil {
		return false
	}
	for _, attribute := range node.Attr {
		if attribute.Namespace == "" && strings.EqualFold(attribute.Key, name) && strings.TrimSpace(attribute.Val) != "" {
			return true
		}
	}
	return false
}

func setRSSResourceAttribute(node *xhtml.Node, name, value string) {
	for index := range node.Attr {
		if node.Attr[index].Namespace == "" && strings.EqualFold(node.Attr[index].Key, name) {
			node.Attr[index].Key = name
			node.Attr[index].Val = value
			return
		}
	}
	node.Attr = append(node.Attr, xhtml.Attribute{Key: name, Val: value})
}

func projectRSSResourceAttribute(node *xhtml.Node, name string, resourceURLs map[string]string) bool {
	found := false
	attributes := node.Attr[:0]
	for _, attribute := range node.Attr {
		if attribute.Namespace != "" || !strings.EqualFold(attribute.Key, name) {
			attributes = append(attributes, attribute)
			continue
		}
		projected := resourceURLs[strings.TrimSpace(attribute.Val)]
		if projected != "" {
			attribute.Val = projected
			attributes = append(attributes, attribute)
			found = true
		}
	}
	node.Attr = attributes
	return found
}
