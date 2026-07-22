package rss

import (
	"bytes"
	"errors"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	xhtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

var safeHTMLIDPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_:.-]{0,127}$`)

const (
	rssVideoEmbedProviderAttribute = "data-xiadown-rss-video-provider"
	rssVideoEmbedIDAttribute       = "data-xiadown-rss-video-id"
	rssVideoEmbedWidthAttribute    = "data-xiadown-rss-video-width"
	rssVideoEmbedHeightAttribute   = "data-xiadown-rss-video-height"
)

var (
	rssYouTubeEmbedIDPattern  = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)
	rssBilibiliEmbedBVPattern = regexp.MustCompile(`(?i)^BV[A-Za-z0-9]{10}$`)
)

var allowedEntryHTMLTags = map[string]struct{}{
	"a": {}, "abbr": {}, "address": {}, "article": {}, "aside": {},
	"b": {}, "blockquote": {}, "br": {}, "caption": {}, "cite": {},
	"code": {}, "col": {}, "colgroup": {}, "dd": {}, "del": {},
	"details": {}, "div": {}, "dl": {}, "dt": {}, "em": {},
	"figcaption": {}, "figure": {}, "h1": {}, "h2": {}, "h3": {},
	"h4": {}, "h5": {}, "h6": {}, "hr": {}, "i": {}, "img": {},
	"kbd": {}, "li": {}, "main": {}, "mark": {}, "ol": {}, "p": {},
	"picture": {}, "pre": {}, "q": {}, "s": {}, "section": {},
	"small": {}, "source": {}, "span": {}, "strong": {}, "sub": {},
	"summary": {}, "sup": {}, "table": {}, "tbody": {}, "td": {},
	"tfoot": {}, "th": {}, "thead": {}, "time": {}, "tr": {}, "u": {},
	"ul": {}, "video": {}, "audio": {},
}

var droppedEntryHTMLTags = map[string]struct{}{
	"applet": {}, "base": {}, "button": {}, "canvas": {}, "embed": {},
	"form": {}, "frame": {}, "frameset": {}, "iframe": {}, "input": {},
	"link": {}, "math": {}, "meta": {}, "noscript": {}, "object": {},
	"script": {}, "select": {}, "style": {}, "svg": {}, "template": {},
	"textarea": {},
}

const (
	maxRSSEntryHTMLNodes         = 20_000
	maxRSSEntryHTMLDepth         = 128
	maxRSSEntryHTMLAttributes    = 64
	maxRSSEntryPlainTextFallback = maxRSSEntryContentHTMLBytes / 6
)

var errRSSEntryHTMLTooLarge = errors.New("RSS entry HTML exceeds persisted byte limit")

type boundedEntryHTMLWriter struct {
	bytes.Buffer
	limit int
}

func (writer *boundedEntryHTMLWriter) Write(value []byte) (int, error) {
	if len(value) > writer.limit-writer.Len() {
		return 0, errRSSEntryHTMLTooLarge
	}
	return writer.Buffer.Write(value)
}

func (writer *boundedEntryHTMLWriter) WriteByte(value byte) error {
	if writer.Len() >= writer.limit {
		return errRSSEntryHTMLTooLarge
	}
	return writer.Buffer.WriteByte(value)
}

func (writer *boundedEntryHTMLWriter) WriteString(value string) (int, error) {
	return writer.Write([]byte(value))
}

type sanitizeEntryHTMLFrame struct {
	node   *xhtml.Node
	parent *xhtml.Node
	depth  int
}

// sanitizeEntryHTML is the single content boundary used both at ingestion and
// at read time. Read-time sanitization protects entries saved by older builds;
// ingestion-time sanitization keeps unsafe markup out of change payloads and
// the desktop database going forward.
func sanitizeEntryHTML(markup, baseURL string) string {
	markup = limitString(markup, maxRSSEntryContentHTMLBytes)
	markup = strings.TrimSpace(strings.ReplaceAll(markup, "\x00", ""))
	if markup == "" {
		return ""
	}
	contextNode := &xhtml.Node{Type: xhtml.ElementNode, DataAtom: atom.Div, Data: "div"}
	nodes, err := xhtml.ParseFragment(strings.NewReader(markup), contextNode)
	if err != nil {
		return safeEntryHTMLPlainText(markup)
	}

	root := &xhtml.Node{Type: xhtml.ElementNode, DataAtom: atom.Div, Data: "div"}
	stack := make([]sanitizeEntryHTMLFrame, 0, min(len(nodes), 64))
	for index := len(nodes) - 1; index >= 0; index-- {
		stack = append(stack, sanitizeEntryHTMLFrame{node: nodes[index], parent: root, depth: 1})
	}
	visited := 0
	for len(stack) > 0 {
		frame := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if frame.node == nil {
			continue
		}
		visited++
		if visited > maxRSSEntryHTMLNodes || frame.depth > maxRSSEntryHTMLDepth {
			return safeEntryHTMLPlainText(markup)
		}
		switch frame.node.Type {
		case xhtml.TextNode:
			frame.parent.AppendChild(&xhtml.Node{Type: xhtml.TextNode, Data: frame.node.Data})
		case xhtml.ElementNode:
			name := strings.ToLower(strings.TrimSpace(frame.node.Data))
			// Remote frames never survive the durable content boundary. A strict
			// video embed becomes an inert provider/id marker so the reader can
			// restore its position without persisting or executing feed-owned URLs.
			if name == "iframe" || (name == "figure" && hasRawHTMLAttribute(
				frame.node.Attr,
				rssVideoEmbedProviderAttribute,
			)) {
				if embed := sanitizedRSSVideoEmbedMarker(frame.node, baseURL); embed != nil {
					frame.parent.AppendChild(embed)
				}
				continue
			}
			if _, drop := droppedEntryHTMLTags[name]; drop {
				continue
			}
			nextParent := frame.parent
			if _, allowed := allowedEntryHTMLTags[name]; allowed {
				clone := &xhtml.Node{Type: xhtml.ElementNode, Data: name}
				clone.Attr = sanitizeEntryHTMLAttributes(name, frame.node.Attr, baseURL)
				if (name == "img" || name == "source") && !hasHTMLAttribute(clone.Attr, "src") {
					continue
				}
				frame.parent.AppendChild(clone)
				nextParent = clone
			}
			for child := frame.node.LastChild; child != nil; child = child.PrevSibling {
				stack = append(stack, sanitizeEntryHTMLFrame{node: child, parent: nextParent, depth: frame.depth + 1})
			}
		}
	}

	output := &boundedEntryHTMLWriter{limit: maxRSSEntryContentHTMLBytes}
	for node := root.FirstChild; node != nil; node = node.NextSibling {
		if err := xhtml.Render(output, node); err != nil {
			return safeEntryHTMLPlainText(markup)
		}
	}
	return strings.TrimSpace(output.String())
}

func safeEntryHTMLPlainText(markup string) string {
	plainText := cleanText(markup, maxRSSEntryPlainTextFallback)
	return boundedHTMLEscape(plainText, maxRSSEntryContentHTMLBytes)
}

func sanitizeEntryHTMLAttributes(tag string, attributes []xhtml.Attribute, baseURL string) []xhtml.Attribute {
	values := make(map[string]string, min(len(attributes), maxRSSEntryHTMLAttributes))
	for index, attribute := range attributes {
		if index >= maxRSSEntryHTMLAttributes {
			break
		}
		if attribute.Namespace != "" {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(attribute.Key))
		if name == "" || strings.HasPrefix(name, "on") {
			continue
		}
		values[name] = strings.TrimSpace(attribute.Val)
	}

	result := make([]xhtml.Attribute, 0, 10)
	appendValue := func(name, value string) {
		value = strings.TrimSpace(value)
		if value != "" {
			result = append(result, xhtml.Attribute{Key: name, Val: value})
		}
	}
	appendValue("title", limitString(values["title"], 512))
	if id := values["id"]; safeHTMLIDPattern.MatchString(id) {
		appendValue("id", id)
	}
	switch strings.ToLower(values["dir"]) {
	case "ltr", "rtl", "auto":
		appendValue("dir", strings.ToLower(values["dir"]))
	}

	switch tag {
	case "a":
		if href := safeEntryLinkURL(baseURL, values["href"]); href != "" {
			appendValue("href", href)
			appendValue("target", "_blank")
			appendValue("rel", "noopener noreferrer")
			appendValue("referrerpolicy", "no-referrer")
		}
	case "img":
		appendValue("src", firstSafeRSSImageCandidate(baseURL, values))
		appendValue("alt", limitString(values["alt"], 1024))
		appendPositiveHTMLInteger(&result, "width", values["width"])
		appendPositiveHTMLInteger(&result, "height", values["height"])
		appendValue("loading", "lazy")
		appendValue("decoding", "async")
		appendValue("referrerpolicy", "no-referrer")
	case "video", "audio":
		appendValue("src", safeEntryResourceURL(baseURL, values["src"]))
		if tag == "video" {
			appendValue("poster", safeEntryResourceURL(baseURL, values["poster"]))
			appendPositiveHTMLInteger(&result, "width", values["width"])
			appendPositiveHTMLInteger(&result, "height", values["height"])
		}
		result = append(result,
			xhtml.Attribute{Key: "controls", Val: ""},
			xhtml.Attribute{Key: "preload", Val: "metadata"},
		)
	case "source":
		appendValue("src", firstSafeRSSImageCandidate(baseURL, values))
		appendValue("type", limitString(values["type"], 128))
		appendValue("media", limitString(values["media"], 256))
	case "blockquote", "q":
		appendValue("cite", safeEntryLinkURL(baseURL, values["cite"]))
	case "time":
		appendValue("datetime", limitString(values["datetime"], 128))
	case "ol":
		appendSignedHTMLInteger(&result, "start", values["start"])
	case "li":
		appendSignedHTMLInteger(&result, "value", values["value"])
	case "td", "th":
		appendPositiveHTMLInteger(&result, "colspan", values["colspan"])
		appendPositiveHTMLInteger(&result, "rowspan", values["rowspan"])
	}
	return result
}

func sanitizedRSSVideoEmbedMarker(node *xhtml.Node, baseURL string) *xhtml.Node {
	if node == nil || node.Type != xhtml.ElementNode {
		return nil
	}
	values := make(map[string]string, min(len(node.Attr), maxRSSEntryHTMLAttributes))
	for index, attribute := range node.Attr {
		if index >= maxRSSEntryHTMLAttributes {
			break
		}
		if attribute.Namespace == "" {
			values[strings.ToLower(strings.TrimSpace(attribute.Key))] = strings.TrimSpace(attribute.Val)
		}
	}
	provider, videoID := "", ""
	if strings.EqualFold(node.Data, "iframe") {
		provider, videoID = rssVideoEmbedIdentity(baseURL, values["src"])
	} else {
		provider, videoID = normalizedRSSVideoEmbedIdentity(
			values[rssVideoEmbedProviderAttribute],
			values[rssVideoEmbedIDAttribute],
		)
	}
	if provider == "" || videoID == "" {
		return nil
	}
	width, height := values["width"], values["height"]
	if !strings.EqualFold(node.Data, "iframe") {
		width = values[rssVideoEmbedWidthAttribute]
		height = values[rssVideoEmbedHeightAttribute]
	}
	attributes := []xhtml.Attribute{
		{Key: rssVideoEmbedProviderAttribute, Val: provider},
		{Key: rssVideoEmbedIDAttribute, Val: videoID},
	}
	appendPositiveHTMLInteger(&attributes, rssVideoEmbedWidthAttribute, width)
	appendPositiveHTMLInteger(&attributes, rssVideoEmbedHeightAttribute, height)
	if title := limitString(strings.TrimSpace(values["title"]), 512); title != "" {
		attributes = append(attributes, xhtml.Attribute{Key: "title", Val: title})
	}
	return &xhtml.Node{
		Type: xhtml.ElementNode,
		Data: "figure",
		Attr: attributes,
	}
}

func rssVideoEmbedIdentity(baseURL, candidate string) (string, string) {
	resolved := safeEntryResourceURL(baseURL, candidate)
	parsed, err := url.Parse(resolved)
	if err != nil || parsed.Scheme != "https" || parsed.User != nil || parsed.RawPath != "" ||
		(parsed.Port() != "" && parsed.Port() != "443") {
		return "", ""
	}
	host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	switch host {
	case "youtube.com", "www.youtube.com", "youtube-nocookie.com", "www.youtube-nocookie.com":
		if len(segments) == 2 && segments[0] == "embed" {
			return normalizedRSSVideoEmbedIdentity("youtube", segments[1])
		}
	case "player.vimeo.com":
		if len(segments) == 2 && segments[0] == "video" {
			return normalizedRSSVideoEmbedIdentity("vimeo", segments[1])
		}
	case "player.bilibili.com":
		if len(segments) == 1 && segments[0] == "player.html" {
			if bvid := parsed.Query().Get("bvid"); bvid != "" {
				return normalizedRSSVideoEmbedIdentity("bilibili", bvid)
			}
			if aid := parsed.Query().Get("aid"); aid != "" {
				return normalizedRSSVideoEmbedIdentity("bilibili", "av"+aid)
			}
		}
	}
	return "", ""
}

func normalizedRSSVideoEmbedIdentity(provider, videoID string) (string, string) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	videoID = strings.TrimSpace(videoID)
	switch provider {
	case "youtube":
		if rssYouTubeEmbedIDPattern.MatchString(videoID) {
			return provider, videoID
		}
	case "vimeo":
		value, err := strconv.ParseUint(videoID, 10, 64)
		if err == nil && value > 0 {
			return provider, strconv.FormatUint(value, 10)
		}
	case "bilibili":
		if rssBilibiliEmbedBVPattern.MatchString(videoID) {
			return provider, "BV" + videoID[2:]
		}
		if matches := bilibiliAVPattern.FindStringSubmatch(videoID); len(matches) == 2 {
			value, err := strconv.ParseUint(matches[1], 10, 64)
			if err == nil && value > 0 {
				return provider, "av" + strconv.FormatUint(value, 10)
			}
		}
	}
	return "", ""
}

func hasRawHTMLAttribute(attributes []xhtml.Attribute, name string) bool {
	for _, attribute := range attributes {
		if attribute.Namespace == "" && strings.EqualFold(strings.TrimSpace(attribute.Key), name) {
			return true
		}
	}
	return false
}

func firstSafeRSSImageCandidate(baseURL string, values map[string]string) string {
	for _, candidate := range rssImageCandidates(values) {
		if resolved := safeEntryResourceURL(baseURL, candidate); resolved != "" {
			return resolved
		}
	}
	return ""
}

// rssImageCandidates preserves the preference used by common lazy-image
// implementations while leaving trust decisions to safeEntryResourceURL.
// Srcset URLs are tokens, not comma-separated strings: data URLs contain a
// comma themselves, so splitting on every comma can turn a rejected data URL
// payload into an apparently valid relative URL.
func rssImageCandidates(values map[string]string) []string {
	result := make([]string, 0, 8)
	seen := make(map[string]struct{}, 8)
	appendCandidate := func(candidate string) bool {
		candidate = boundedRawRSSURL(candidate)
		if candidate == "" {
			return true
		}
		if _, exists := seen[candidate]; exists {
			return true
		}
		seen[candidate] = struct{}{}
		result = append(result, candidate)
		return len(result) < maxRSSPersistedEntryMedia
	}

	for _, name := range []string{"data-original", "data-lazy-src", "data-src", "src"} {
		if !appendCandidate(values[name]) {
			return result
		}
	}
	for _, name := range []string{"srcset", "data-srcset"} {
		for _, candidate := range parseRSSSrcsetCandidates(values[name], maxRSSPersistedEntryMedia-len(result)) {
			if !appendCandidate(candidate) {
				return result
			}
		}
	}
	return result
}

func parseRSSSrcsetCandidates(raw string, limit int) []string {
	if limit <= 0 {
		return nil
	}
	raw = strings.TrimSpace(raw)
	result := make([]string, 0, min(limit, 8))
	for cursor := 0; cursor < len(raw) && len(result) < limit; {
		for cursor < len(raw) && (isRSSASCIISpace(raw[cursor]) || raw[cursor] == ',') {
			cursor++
		}
		if cursor >= len(raw) {
			break
		}

		start := cursor
		for cursor < len(raw) && !isRSSASCIISpace(raw[cursor]) {
			cursor++
		}
		candidateToken := raw[start:cursor]
		candidate := strings.TrimRight(candidateToken, ",")
		if candidate != "" {
			result = append(result, candidate)
		}
		if len(candidate) < len(candidateToken) {
			continue
		}

		// Consume density/width descriptors until the candidate delimiter.
		// Parentheses are allowed in descriptors, and commas inside them do
		// not start a new candidate.
		parentheses := 0
		for cursor < len(raw) {
			switch raw[cursor] {
			case '(':
				parentheses++
			case ')':
				if parentheses > 0 {
					parentheses--
				}
			case ',':
				if parentheses == 0 {
					cursor++
					goto nextCandidate
				}
			}
			cursor++
		}
	nextCandidate:
	}
	return result
}

func isRSSASCIISpace(value byte) bool {
	switch value {
	case '\t', '\n', '\f', '\r', ' ':
		return true
	default:
		return false
	}
}

func safeEntryLinkURL(baseURL, candidate string) string {
	candidate = boundedRawRSSURL(candidate)
	if candidate == "" {
		return ""
	}
	if strings.HasPrefix(candidate, "#") {
		if safeHTMLIDPattern.MatchString(strings.TrimPrefix(candidate, "#")) {
			return candidate
		}
		return ""
	}
	parsed, err := url.Parse(candidate)
	if err == nil && strings.EqualFold(parsed.Scheme, "mailto") && parsed.Opaque != "" && parsed.User == nil {
		return boundedRawRSSURL(parsed.String())
	}
	return safeEntryResourceURL(baseURL, candidate)
}

func safeEntryResourceURL(baseURL, candidate string) string {
	candidate = boundedRawRSSURL(candidate)
	if candidate == "" || len(strings.TrimSpace(baseURL)) > maxRSSDurableURLBytes {
		return ""
	}
	resolved := resolveURL(baseURL, candidate)
	if resolved == "" || len(resolved) > maxRSSDurableURLBytes {
		return ""
	}
	parsed, err := url.Parse(resolved)
	if err != nil || parsed.User != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" {
		return ""
	}
	host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	if ip := net.ParseIP(host); ip != nil {
		if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return ""
		}
	} else {
		if !strings.Contains(host, ".") || isLocalEntryHostname(host) {
			return ""
		}
	}
	return parsed.String()
}

func isLocalEntryHostname(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	for _, suffix := range []string{"localhost", "local", "internal", "lan", "home", "home.arpa", "localdomain"} {
		if host == suffix || strings.HasSuffix(host, "."+suffix) {
			return true
		}
	}
	return false
}

func appendPositiveHTMLInteger(attributes *[]xhtml.Attribute, name, raw string) {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err == nil && value > 0 && value <= 100000 {
		*attributes = append(*attributes, xhtml.Attribute{Key: name, Val: strconv.Itoa(value)})
	}
}

func appendSignedHTMLInteger(attributes *[]xhtml.Attribute, name, raw string) {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err == nil && value >= -100000 && value <= 100000 {
		*attributes = append(*attributes, xhtml.Attribute{Key: name, Val: strconv.Itoa(value)})
	}
}

func hasHTMLAttribute(attributes []xhtml.Attribute, name string) bool {
	for _, attribute := range attributes {
		if attribute.Key == name && strings.TrimSpace(attribute.Val) != "" {
			return true
		}
	}
	return false
}
