package ytdlp

import (
	"encoding/xml"
	"io"
	"strings"
)

const unsupportedManifestReferenceReason = "manifest contains a non-HTTP(S) or credentialed URL reference"

func hlsManifestReferencesAllowed(manifestURL string, body string) bool {
	if ValidateNetworkURL(manifestURL) != nil {
		return false
	}
	for _, rawLine := range strings.Split(body, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "#") {
			if ResolveManifestReference(manifestURL, line) == "" {
				return false
			}
			continue
		}
		_, attributes, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		uri, hasURI := parseHLSAttributeList(attributes)["URI"]
		if hasURI && strings.TrimSpace(uri) != "" && ResolveManifestReference(manifestURL, uri) == "" {
			return false
		}
	}
	return true
}

func dashManifestReferencesAllowed(manifestURL string, body []byte) bool {
	if ValidateNetworkURL(manifestURL) != nil {
		return false
	}
	type referenceElement struct {
		capture bool
		text    strings.Builder
	}
	stack := []referenceElement{}
	decoder := xml.NewDecoder(strings.NewReader(string(body)))
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return true
		}
		if err != nil {
			return false
		}
		switch typed := token.(type) {
		case xml.StartElement:
			for _, attribute := range typed.Attr {
				if !dashURLAttribute(attribute.Name.Local) || strings.TrimSpace(attribute.Value) == "" {
					continue
				}
				if ResolveManifestReference(manifestURL, attribute.Value) == "" {
					return false
				}
			}
			stack = append(stack, referenceElement{capture: dashURLTextElement(typed.Name.Local)})
		case xml.CharData:
			if len(stack) > 0 && stack[len(stack)-1].capture {
				stack[len(stack)-1].text.Write([]byte(typed))
			}
		case xml.EndElement:
			if len(stack) == 0 {
				continue
			}
			current := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if current.capture {
				reference := strings.TrimSpace(current.text.String())
				if reference != "" && ResolveManifestReference(manifestURL, reference) == "" {
					return false
				}
			}
		}
	}
}

func dashURLAttribute(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "href", "index", "initialization", "media", "serverurl", "sourceurl", "url":
		return true
	default:
		return false
	}
}

func dashURLTextElement(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "baseurl", "location", "patchlocation":
		return true
	default:
		return false
	}
}
