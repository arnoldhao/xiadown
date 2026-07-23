package rss

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type parsedFeed struct {
	Title       string
	SiteURL     string
	Description string
	IconURL     string
	HistoryURL  string
	Entries     []parsedEntry
}

const (
	maxRSSFeedDocumentBytes     = 10 << 20
	maxRSSFeedEntries           = 2000
	maxRSSParsedEntryMediaItems = 64
	maxRSSPersistedEntryMedia   = 64
	maxRSSPersistedEntryImages  = 64
	maxRSSFeedTitleBytes        = 512
	maxRSSFeedDescriptionBytes  = 4096
	// The iOS public-feed parser bounds these identities by Unicode scalar
	// count (8,192 GUID / 4,096 title). Four bytes per UTF-8 scalar keeps the
	// Desktop origin-v1 input lossless while the document/body limits remain
	// the dominant memory bound.
	maxRSSEntryExternalIDBytes    = 32 << 10
	maxRSSEntryTitleBytes         = 16 << 10
	maxRSSEntryAuthorBytes        = 512
	maxRSSEntrySummaryBytes       = 8192
	maxRSSEntryContentHTMLBytes   = 1 << 20
	maxRSSMediaMIMETypeBytes      = 256
	maxRSSDurableURLBytes         = 4096
	maxRSSCleanTextInputExpansion = 8
	maxRSSMediaDimension          = 32768
	maxRSSMediaDurationMillis     = int64(7 * 24 * time.Hour / time.Millisecond)
)

type parsedEntry struct {
	ExternalID string
	URL        string
	Title      string
	Author     string
	Summary    string
	Content    string
	Published  *time.Time
	Updated    *time.Time
	Media      []parsedMedia
}

type parsedMedia struct {
	URL       string
	MIMEType  string
	Thumbnail string
	Width     int
	Height    int
	Duration  int64
}

type rssDocument struct {
	Channel rssChannel `xml:"channel"`
	Items   []rssItem  `xml:"item"`
}

type rssChannel struct {
	Title       string    `xml:"title"`
	Link        string    `xml:"link"`
	Description string    `xml:"description"`
	Image       rssImage  `xml:"image"`
	Items       []rssItem `xml:"item"`
	entryLimit  int       `xml:"-"`
	limitSet    bool      `xml:"-"`
}

type rssImage struct {
	URL string `xml:"url"`
}

type rssItem struct {
	GUID        string     `xml:"guid"`
	Title       string     `xml:"title"`
	Link        string     `xml:"link"`
	Description string     `xml:"description"`
	Encoded     string     `xml:"encoded"`
	Author      string     `xml:"author"`
	Creator     string     `xml:"creator"`
	PubDate     string     `xml:"pubDate"`
	Date        string     `xml:"date"`
	Updated     string     `xml:"updated"`
	Media       []xmlMedia `xml:"-"`
}

type xmlMedia struct {
	URL       string `xml:"-"`
	Type      string `xml:"-"`
	Width     int    `xml:"-"`
	Height    int    `xml:"-"`
	Duration  int64  `xml:"-"`
	Thumbnail string `xml:"-"`
}

type atomDocument struct {
	Title    atomText    `xml:"title"`
	Subtitle atomText    `xml:"subtitle"`
	Icon     string      `xml:"icon"`
	Logo     string      `xml:"logo"`
	Links    []atomLink  `xml:"link"`
	Entries  []atomEntry `xml:"entry"`
}

type atomText struct {
	Text  string `xml:",chardata"`
	Inner string `xml:",innerxml"`
}

func (value atomText) Value(limit int) string {
	inner := strings.TrimSpace(limitString(value.Inner, limit))
	if strings.HasPrefix(inner, "<![CDATA[") && strings.HasSuffix(inner, "]]>") {
		return limitString(strings.TrimSuffix(strings.TrimPrefix(inner, "<![CDATA["), "]]>"), limit)
	}
	if inner != "" {
		return limitString(html.UnescapeString(inner), limit)
	}
	return strings.TrimSpace(limitString(value.Text, limit))
}

type atomLink struct {
	Rel  string `xml:"rel,attr"`
	Type string `xml:"type,attr"`
	Href string `xml:"href,attr"`
}

type atomEntry struct {
	ID        string     `xml:"id"`
	Title     atomText   `xml:"title"`
	Summary   atomText   `xml:"summary"`
	Content   atomText   `xml:"content"`
	Published string     `xml:"published"`
	Updated   string     `xml:"updated"`
	Author    atomPerson `xml:"author"`
	Links     []atomLink `xml:"link"`
}

type atomPerson struct {
	Name string `xml:"name"`
}

type jsonFeed struct {
	Version     string               `json:"version"`
	Title       string               `json:"title"`
	HomePageURL string               `json:"home_page_url"`
	FeedURL     string               `json:"feed_url"`
	NextURL     string               `json:"next_url"`
	Description string               `json:"description"`
	Icon        string               `json:"icon"`
	Favicon     string               `json:"favicon"`
	Items       boundedJSONFeedItems `json:"items"`
}

type jsonFeedItem struct {
	ID            string                     `json:"id"`
	URL           string                     `json:"url"`
	ExternalURL   string                     `json:"external_url"`
	Title         string                     `json:"title"`
	ContentHTML   string                     `json:"content_html"`
	ContentText   string                     `json:"content_text"`
	Summary       string                     `json:"summary"`
	DatePublished string                     `json:"date_published"`
	DateModified  string                     `json:"date_modified"`
	Authors       boundedJSONFeedAuthors     `json:"authors"`
	Author        *jsonFeedAuthor            `json:"author"`
	Attachments   boundedJSONFeedAttachments `json:"attachments"`
}

type jsonFeedAuthor struct {
	Name string `json:"name"`
}

type jsonFeedAttachment struct {
	URL      string `json:"url"`
	MIMEType string `json:"mime_type"`
	Duration int64  `json:"duration_in_seconds"`
}

func (document *rssDocument) UnmarshalXML(decoder *xml.Decoder, start xml.StartElement) error {
	for {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		switch typed := token.(type) {
		case xml.StartElement:
			switch strings.ToLower(typed.Name.Local) {
			case "channel":
				document.Channel.entryLimit = maxRSSFeedEntries - len(document.Items)
				document.Channel.limitSet = true
				if err := decoder.DecodeElement(&document.Channel, &typed); err != nil {
					return err
				}
			case "item":
				if len(document.Channel.Items)+len(document.Items) >= maxRSSFeedEntries {
					if err := decoder.Skip(); err != nil {
						return err
					}
					continue
				}
				var item rssItem
				if err := decoder.DecodeElement(&item, &typed); err != nil {
					return err
				}
				document.Items = append(document.Items, item)
			default:
				if err := decoder.Skip(); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if typed.Name == start.Name {
				return nil
			}
		}
	}
}

func (channel *rssChannel) UnmarshalXML(decoder *xml.Decoder, start xml.StartElement) error {
	entryLimit := maxRSSFeedEntries
	if channel.limitSet {
		entryLimit = max(channel.entryLimit, 0)
	}
	for {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		switch typed := token.(type) {
		case xml.StartElement:
			switch strings.ToLower(typed.Name.Local) {
			case "title":
				if err := decoder.DecodeElement(&channel.Title, &typed); err != nil {
					return err
				}
				channel.Title = limitString(channel.Title, maxRSSFeedTitleBytes*maxRSSCleanTextInputExpansion)
			case "link":
				if err := decoder.DecodeElement(&channel.Link, &typed); err != nil {
					return err
				}
				channel.Link = boundedRawRSSURL(channel.Link)
			case "description":
				if err := decoder.DecodeElement(&channel.Description, &typed); err != nil {
					return err
				}
				channel.Description = limitString(channel.Description, maxRSSFeedDescriptionBytes*maxRSSCleanTextInputExpansion)
			case "image":
				if err := decoder.DecodeElement(&channel.Image, &typed); err != nil {
					return err
				}
				channel.Image.URL = boundedRawRSSURL(channel.Image.URL)
			case "item":
				if len(channel.Items) >= entryLimit {
					if err := decoder.Skip(); err != nil {
						return err
					}
					continue
				}
				var item rssItem
				if err := decoder.DecodeElement(&item, &typed); err != nil {
					return err
				}
				channel.Items = append(channel.Items, item)
			default:
				if err := decoder.Skip(); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if typed.Name == start.Name {
				return nil
			}
		}
	}
}

func (item *rssItem) UnmarshalXML(decoder *xml.Decoder, start xml.StartElement) error {
	for {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		switch typed := token.(type) {
		case xml.StartElement:
			name := strings.ToLower(typed.Name.Local)
			switch name {
			case "guid", "title", "link", "description", "encoded", "author", "creator", "pubdate", "date", "updated":
				var value string
				if err := decoder.DecodeElement(&value, &typed); err != nil {
					return err
				}
				switch name {
				case "guid":
					item.GUID = boundedRSSExternalID(value)
				case "title":
					item.Title = limitString(value, maxRSSEntryTitleBytes*maxRSSCleanTextInputExpansion)
				case "link":
					item.Link = boundedRawRSSURL(value)
				case "description":
					item.Description = limitString(value, maxRSSEntryContentHTMLBytes)
				case "encoded":
					item.Encoded = limitString(value, maxRSSEntryContentHTMLBytes)
				case "author":
					item.Author = limitString(value, maxRSSEntryAuthorBytes*maxRSSCleanTextInputExpansion)
				case "creator":
					item.Creator = limitString(value, maxRSSEntryAuthorBytes*maxRSSCleanTextInputExpansion)
				case "pubdate":
					item.PubDate = limitString(value, 128)
				case "date":
					item.Date = limitString(value, 128)
				case "updated":
					item.Updated = limitString(value, 128)
				}
			case "enclosure", "content", "thumbnail":
				if len(item.Media) >= maxRSSParsedEntryMediaItems {
					if err := decoder.Skip(); err != nil {
						return err
					}
					continue
				}
				var media xmlMedia
				if err := decoder.DecodeElement(&media, &typed); err != nil {
					return err
				}
				item.Media = append(item.Media, media)
			default:
				if err := decoder.Skip(); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if typed.Name == start.Name {
				return nil
			}
		}
	}
}

func (media *xmlMedia) UnmarshalXML(decoder *xml.Decoder, start xml.StartElement) error {
	for _, attribute := range start.Attr {
		switch strings.ToLower(attribute.Name.Local) {
		case "url":
			media.URL = boundedRawRSSURL(attribute.Value)
		case "type":
			media.Type = limitString(strings.TrimSpace(attribute.Value), maxRSSMediaMIMETypeBytes)
		case "width":
			value, _ := strconv.Atoi(strings.TrimSpace(attribute.Value))
			media.Width = boundedRSSMediaDimension(value)
		case "height":
			value, _ := strconv.Atoi(strings.TrimSpace(attribute.Value))
			media.Height = boundedRSSMediaDimension(value)
		case "duration":
			value, _ := strconv.ParseInt(strings.TrimSpace(attribute.Value), 10, 64)
			if value > maxRSSMediaDurationMillis/1000 {
				value = maxRSSMediaDurationMillis / 1000
			}
			if value > 0 {
				media.Duration = value
			}
		}
	}
	for {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		switch typed := token.(type) {
		case xml.StartElement:
			if strings.EqualFold(typed.Name.Local, "thumbnail") && media.Thumbnail == "" {
				for _, attribute := range typed.Attr {
					if strings.EqualFold(attribute.Name.Local, "url") {
						media.Thumbnail = boundedRawRSSURL(attribute.Value)
						break
					}
				}
			}
			if err := decoder.Skip(); err != nil {
				return err
			}
		case xml.EndElement:
			if typed.Name == start.Name {
				return nil
			}
		}
	}
}

func (document *atomDocument) UnmarshalXML(decoder *xml.Decoder, start xml.StartElement) error {
	for {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		switch typed := token.(type) {
		case xml.StartElement:
			switch strings.ToLower(typed.Name.Local) {
			case "title":
				if err := decoder.DecodeElement(&document.Title, &typed); err != nil {
					return err
				}
				boundAtomText(&document.Title, maxRSSFeedTitleBytes*maxRSSCleanTextInputExpansion)
			case "subtitle":
				if err := decoder.DecodeElement(&document.Subtitle, &typed); err != nil {
					return err
				}
				boundAtomText(&document.Subtitle, maxRSSFeedDescriptionBytes*maxRSSCleanTextInputExpansion)
			case "icon", "logo":
				var value string
				if err := decoder.DecodeElement(&value, &typed); err != nil {
					return err
				}
				if strings.EqualFold(typed.Name.Local, "icon") {
					document.Icon = boundedRawRSSURL(value)
				} else {
					document.Logo = boundedRawRSSURL(value)
				}
			case "link":
				var link atomLink
				if err := decoder.DecodeElement(&link, &typed); err != nil {
					return err
				}
				boundAtomLink(&link)
				if len(document.Links) < maxRSSParsedEntryMediaItems {
					document.Links = append(document.Links, link)
				}
			case "entry":
				if len(document.Entries) >= maxRSSFeedEntries {
					if err := decoder.Skip(); err != nil {
						return err
					}
					continue
				}
				var entry atomEntry
				if err := decoder.DecodeElement(&entry, &typed); err != nil {
					return err
				}
				document.Entries = append(document.Entries, entry)
			default:
				if err := decoder.Skip(); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if typed.Name == start.Name {
				return nil
			}
		}
	}
}

func (entry *atomEntry) UnmarshalXML(decoder *xml.Decoder, start xml.StartElement) error {
	for {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		switch typed := token.(type) {
		case xml.StartElement:
			name := strings.ToLower(typed.Name.Local)
			switch name {
			case "id", "published", "updated":
				var value string
				if err := decoder.DecodeElement(&value, &typed); err != nil {
					return err
				}
				switch name {
				case "id":
					entry.ID = boundedRSSExternalID(value)
				case "published":
					entry.Published = limitString(value, 128)
				case "updated":
					entry.Updated = limitString(value, 128)
				}
			case "title", "summary", "content":
				var value atomText
				if err := decoder.DecodeElement(&value, &typed); err != nil {
					return err
				}
				limit := maxRSSEntryContentHTMLBytes
				if name == "title" {
					limit = maxRSSEntryTitleBytes * maxRSSCleanTextInputExpansion
				} else if name == "summary" {
					limit = maxRSSEntrySummaryBytes * maxRSSCleanTextInputExpansion
				}
				boundAtomText(&value, limit)
				switch name {
				case "title":
					entry.Title = value
				case "summary":
					entry.Summary = value
				case "content":
					entry.Content = value
				}
			case "author":
				if err := decoder.DecodeElement(&entry.Author, &typed); err != nil {
					return err
				}
				entry.Author.Name = limitString(entry.Author.Name, maxRSSEntryAuthorBytes*maxRSSCleanTextInputExpansion)
			case "link":
				if len(entry.Links) >= maxRSSParsedEntryMediaItems {
					if err := decoder.Skip(); err != nil {
						return err
					}
					continue
				}
				var link atomLink
				if err := decoder.DecodeElement(&link, &typed); err != nil {
					return err
				}
				boundAtomLink(&link)
				entry.Links = append(entry.Links, link)
			default:
				if err := decoder.Skip(); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if typed.Name == start.Name {
				return nil
			}
		}
	}
}

func boundAtomText(value *atomText, limit int) {
	value.Text = limitString(value.Text, limit)
	value.Inner = limitString(value.Inner, limit)
}

func boundAtomLink(link *atomLink) {
	link.Rel = limitString(strings.TrimSpace(link.Rel), 64)
	link.Type = limitString(strings.TrimSpace(link.Type), maxRSSMediaMIMETypeBytes)
	link.Href = boundedRawRSSURL(link.Href)
}

type boundedJSONFeedItems []jsonFeedItem
type boundedJSONFeedAuthors []jsonFeedAuthor
type boundedJSONFeedAttachments []jsonFeedAttachment

func (items *boundedJSONFeedItems) UnmarshalJSON(data []byte) error {
	result, err := decodeBoundedJSONArray[jsonFeedItem](data, maxRSSFeedEntries)
	*items = result
	return err
}

func (items *boundedJSONFeedAuthors) UnmarshalJSON(data []byte) error {
	result, err := decodeBoundedJSONArray[jsonFeedAuthor](data, 1)
	*items = result
	return err
}

func (items *boundedJSONFeedAttachments) UnmarshalJSON(data []byte) error {
	result, err := decodeBoundedJSONArray[jsonFeedAttachment](data, maxRSSParsedEntryMediaItems)
	*items = result
	return err
}

func decodeBoundedJSONArray[T any](data []byte, limit int) ([]T, error) {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return nil, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	if delimiter, ok := token.(json.Delim); !ok || delimiter != '[' {
		return nil, errors.New("expected JSON array")
	}
	items := make([]T, 0, min(limit, 16))
	for decoder.More() {
		if len(items) >= limit {
			var discarded json.RawMessage
			if err := decoder.Decode(&discarded); err != nil {
				return nil, err
			}
			continue
		}
		var item T
		if err := decoder.Decode(&item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if _, err := decoder.Token(); err != nil {
		return nil, err
	}
	return items, nil
}

func parseFeed(body []byte, contentType string) (parsedFeed, error) {
	if len(body) > maxRSSFeedDocumentBytes {
		return parsedFeed{}, fmt.Errorf("feed document exceeds %d bytes", maxRSSFeedDocumentBytes)
	}
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return parsedFeed{}, errors.New("feed response is empty")
	}
	if trimmed[0] == '{' || strings.Contains(strings.ToLower(contentType), "json") {
		if feed, err := parseJSONFeed(trimmed); err == nil {
			return feed, nil
		}
	}
	decoder := xml.NewDecoder(bytes.NewReader(trimmed))
	for {
		token, err := decoder.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return parsedFeed{}, fmt.Errorf("parse feed XML: %w", err)
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		switch strings.ToLower(start.Name.Local) {
		case "rss", "rdf":
			var document rssDocument
			if err := decoder.DecodeElement(&document, &start); err != nil {
				return parsedFeed{}, fmt.Errorf("parse RSS: %w", err)
			}
			return mapRSS(document), nil
		case "feed":
			var document atomDocument
			if err := decoder.DecodeElement(&document, &start); err != nil {
				return parsedFeed{}, fmt.Errorf("parse Atom: %w", err)
			}
			return mapAtom(document), nil
		default:
			return parsedFeed{}, fmt.Errorf("unsupported feed root %q", start.Name.Local)
		}
	}
	return parsedFeed{}, errors.New("feed document has no root element")
}

func mapRSS(document rssDocument) parsedFeed {
	items := document.Channel.Items
	if len(items) == 0 && len(document.Items) > 0 {
		items = document.Items
	}
	if len(items) > maxRSSFeedEntries {
		items = items[:maxRSSFeedEntries]
	}
	feed := parsedFeed{
		Title: cleanText(document.Channel.Title, maxRSSFeedTitleBytes), SiteURL: boundedRawRSSURL(document.Channel.Link),
		Description: cleanText(document.Channel.Description, maxRSSFeedDescriptionBytes), IconURL: boundedRawRSSURL(document.Channel.Image.URL),
		Entries: make([]parsedEntry, 0, len(items)),
	}
	for _, item := range items {
		content := strings.TrimSpace(limitString(item.Encoded, maxRSSEntryContentHTMLBytes))
		if content == "" {
			content = strings.TrimSpace(limitString(item.Description, maxRSSEntryContentHTMLBytes))
		}
		entry := parsedEntry{
			ExternalID: boundedRSSExternalID(item.GUID), URL: boundedRawRSSURL(item.Link),
			Title: cleanText(item.Title, maxRSSEntryTitleBytes), Author: cleanText(firstNonEmpty(item.Creator, item.Author), maxRSSEntryAuthorBytes),
			Summary: cleanText(item.Description, maxRSSEntrySummaryBytes), Content: content,
			Published: parseFeedTime(firstNonEmpty(item.PubDate, item.Date)), Updated: parseFeedTime(item.Updated),
		}
		for _, media := range item.Media {
			entry.Media = appendMedia(entry.Media, media)
		}
		feed.Entries = append(feed.Entries, entry)
	}
	return feed
}

func mapAtom(document atomDocument) parsedFeed {
	entries := document.Entries
	if len(entries) > maxRSSFeedEntries {
		entries = entries[:maxRSSFeedEntries]
	}
	feed := parsedFeed{
		Title:       cleanText(document.Title.Value(maxRSSFeedTitleBytes*maxRSSCleanTextInputExpansion), maxRSSFeedTitleBytes),
		Description: cleanText(document.Subtitle.Value(maxRSSFeedDescriptionBytes*maxRSSCleanTextInputExpansion), maxRSSFeedDescriptionBytes),
		IconURL:     boundedRawRSSURL(firstNonEmpty(document.Icon, document.Logo)),
		SiteURL:     atomLinkURL(document.Links, "alternate"),
		HistoryURL:  atomHistoryURL(document.Links),
		Entries:     make([]parsedEntry, 0, len(entries)),
	}
	for _, item := range entries {
		content := item.Content.Value(maxRSSEntryContentHTMLBytes)
		if content == "" {
			content = item.Summary.Value(maxRSSEntryContentHTMLBytes)
		}
		entry := parsedEntry{
			ExternalID: boundedRSSExternalID(item.ID), URL: atomLinkURL(item.Links, "alternate"),
			Title:   cleanText(item.Title.Value(maxRSSEntryTitleBytes*maxRSSCleanTextInputExpansion), maxRSSEntryTitleBytes),
			Author:  cleanText(item.Author.Name, maxRSSEntryAuthorBytes),
			Summary: cleanText(item.Summary.Value(maxRSSEntrySummaryBytes*maxRSSCleanTextInputExpansion), maxRSSEntrySummaryBytes), Content: content,
			Published: parseFeedTime(item.Published), Updated: parseFeedTime(item.Updated),
		}
		for _, link := range item.Links {
			if len(entry.Media) >= maxRSSParsedEntryMediaItems {
				break
			}
			if strings.EqualFold(link.Rel, "enclosure") {
				entry.Media = append(entry.Media, parsedMedia{
					URL: boundedRawRSSURL(link.Href), MIMEType: limitString(strings.TrimSpace(link.Type), maxRSSMediaMIMETypeBytes),
				})
			}
		}
		feed.Entries = append(feed.Entries, entry)
	}
	return feed
}

func parseJSONFeed(body []byte) (parsedFeed, error) {
	var document jsonFeed
	if err := json.Unmarshal(body, &document); err != nil {
		return parsedFeed{}, err
	}
	if !strings.Contains(strings.ToLower(document.Version), "jsonfeed.org") && document.Title == "" {
		return parsedFeed{}, errors.New("unsupported JSON feed")
	}
	items := document.Items
	if len(items) > maxRSSFeedEntries {
		items = items[:maxRSSFeedEntries]
	}
	feed := parsedFeed{
		Title: cleanText(document.Title, maxRSSFeedTitleBytes), SiteURL: boundedRawRSSURL(document.HomePageURL),
		Description: cleanText(document.Description, maxRSSFeedDescriptionBytes), IconURL: boundedRawRSSURL(firstNonEmpty(document.Icon, document.Favicon)),
		HistoryURL: boundedRawRSSURL(document.NextURL), Entries: make([]parsedEntry, 0, len(items)),
	}
	for _, item := range items {
		author := ""
		if len(item.Authors) > 0 {
			author = item.Authors[0].Name
		} else if item.Author != nil {
			author = item.Author.Name
		}
		entry := parsedEntry{
			ExternalID: boundedRSSExternalID(item.ID), URL: boundedRawRSSURL(firstNonEmpty(item.URL, item.ExternalURL)),
			Title: cleanText(item.Title, maxRSSEntryTitleBytes), Author: cleanText(author, maxRSSEntryAuthorBytes),
			Summary:   cleanText(firstNonEmpty(item.Summary, item.ContentText), maxRSSEntrySummaryBytes),
			Content:   boundedJSONFeedContent(item),
			Published: parseFeedTime(item.DatePublished), Updated: parseFeedTime(item.DateModified),
		}
		attachments := item.Attachments
		if len(attachments) > maxRSSParsedEntryMediaItems {
			attachments = attachments[:maxRSSParsedEntryMediaItems]
		}
		for _, attachment := range attachments {
			entry.Media = append(entry.Media, parsedMedia{
				URL: boundedRawRSSURL(attachment.URL), MIMEType: limitString(strings.TrimSpace(attachment.MIMEType), maxRSSMediaMIMETypeBytes),
				Duration: rssMediaDurationFromSeconds(attachment.Duration),
			})
		}
		feed.Entries = append(feed.Entries, entry)
	}
	return feed, nil
}

func appendMedia(items []parsedMedia, value xmlMedia) []parsedMedia {
	return append(items, parsedMedia{
		URL: boundedRawRSSURL(value.URL), MIMEType: limitString(strings.TrimSpace(value.Type), maxRSSMediaMIMETypeBytes),
		Thumbnail: boundedRawRSSURL(value.Thumbnail), Width: boundedRSSMediaDimension(value.Width), Height: boundedRSSMediaDimension(value.Height),
		Duration: rssMediaDurationFromSeconds(value.Duration),
	})
}

func boundedRSSMediaDimension(value int) int {
	if value <= 0 {
		return 0
	}
	return min(value, maxRSSMediaDimension)
}

func boundedRSSMediaDurationMillis(value int64) int64 {
	if value <= 0 {
		return 0
	}
	return min(value, maxRSSMediaDurationMillis)
}

func rssMediaDurationFromSeconds(value int64) int64 {
	if value <= 0 {
		return 0
	}
	if value > maxRSSMediaDurationMillis/1000 {
		return maxRSSMediaDurationMillis
	}
	return value * 1000
}

func atomLinkURL(links []atomLink, relation string) string {
	for _, link := range links {
		rel := strings.TrimSpace(strings.ToLower(link.Rel))
		if rel == strings.ToLower(relation) || (relation == "alternate" && rel == "") {
			return boundedRawRSSURL(link.Href)
		}
	}
	return ""
}

// RFC 5005 subscription/archive documents walk toward older immutable
// archives through prev-archive. Atom paged feeds use next for the following
// page, which is the convention used by chronological feed publishers for
// older entries. Prefer the lossless archive relation when both are present.
func atomHistoryURL(links []atomLink) string {
	if archived := atomLinkURL(links, "prev-archive"); archived != "" {
		return archived
	}
	return atomLinkURL(links, "next")
}

func parseFeedTime(value string) *time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	for _, layout := range []string{
		time.RFC3339Nano, time.RFC3339, time.RFC1123Z, time.RFC1123,
		time.RFC822Z, time.RFC822, time.RFC850, time.ANSIC,
		"Mon, 02 Jan 2006 15:04:05 -0700", "2006-01-02 15:04:05",
	} {
		if parsed, err := time.Parse(layout, value); err == nil {
			utc := parsed.UTC()
			return &utc
		}
	}
	return nil
}

func cleanText(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	inputLimit := limit * maxRSSCleanTextInputExpansion
	if inputLimit < limit || inputLimit > maxRSSEntryContentHTMLBytes {
		inputLimit = maxRSSEntryContentHTMLBytes
	}
	value = limitString(value, inputLimit)
	value = html.UnescapeString(strings.TrimSpace(stripMarkup(value)))
	value = strings.Join(strings.Fields(value), " ")
	value = strings.NewReplacer(
		" .", ".", " ,", ",", " !", "!", " ?", "?", " :", ":", " ;", ";",
	).Replace(value)
	return limitString(value, limit)
}

var markupTagPattern = regexp.MustCompile(`(?s)</?[A-Za-z][^>]*>|<!--.*?-->`)

func stripMarkup(value string) string {
	return markupTagPattern.ReplaceAllString(strings.ReplaceAll(value, "\x00", ""), " ")
}

func limitString(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	value = strings.ToValidUTF8(strings.ReplaceAll(value, "\x00", ""), "�")
	if len(value) <= limit {
		return value
	}
	for limit > 0 && !utf8.ValidString(value[:limit]) {
		limit--
	}
	return value[:limit]
}

func boundedRawRSSURL(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > maxRSSDurableURLBytes {
		return ""
	}
	value = strings.ToValidUTF8(strings.ReplaceAll(value, "\x00", ""), "�")
	if len(value) > maxRSSDurableURLBytes {
		return ""
	}
	return value
}

func boundedRSSExternalID(value string) string {
	value = strings.TrimSpace(strings.ToValidUTF8(strings.ReplaceAll(value, "\x00", ""), "�"))
	if len(value) <= maxRSSEntryExternalIDBytes {
		return value
	}
	return "sha256:" + stableDigest(value)
}

func boundedJSONFeedContent(item jsonFeedItem) string {
	if content := strings.TrimSpace(item.ContentHTML); content != "" {
		return limitString(content, maxRSSEntryContentHTMLBytes)
	}
	return boundedHTMLEscape(item.ContentText, maxRSSEntryContentHTMLBytes)
}

func boundedHTMLEscape(value string, limit int) string {
	value = limitString(value, limit)
	var builder strings.Builder
	builder.Grow(min(len(value), limit))
	for _, character := range value {
		replacement := string(character)
		switch character {
		case '&':
			replacement = "&amp;"
		case '\'':
			replacement = "&#39;"
		case '<':
			replacement = "&lt;"
		case '>':
			replacement = "&gt;"
		case '"':
			replacement = "&#34;"
		}
		if builder.Len()+len(replacement) > limit {
			break
		}
		builder.WriteString(replacement)
	}
	return builder.String()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func resolveURL(baseURL, candidate string) string {
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return ""
	}
	parsed, err := url.Parse(candidate)
	if err != nil {
		return ""
	}
	if parsed.IsAbs() {
		if parsed.Scheme == "http" || parsed.Scheme == "https" {
			return parsed.String()
		}
		return ""
	}
	base, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || !base.IsAbs() {
		return ""
	}
	return base.ResolveReference(parsed).String()
}
