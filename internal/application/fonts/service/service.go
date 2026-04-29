package service

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
)

const maxFontFileSizeBytes int64 = 100 << 20 // 100 MiB

type FontService struct {
	mu sync.RWMutex

	loaded   bool
	families []string
	err      error
}

func NewFontService() *FontService {
	return &FontService{}
}

func (service *FontService) ListFontFamilies(ctx context.Context) ([]string, error) {
	if err := service.ensureFamilies(ctx); err != nil {
		return nil, err
	}

	service.mu.RLock()
	defer service.mu.RUnlock()

	result := make([]string, 0, len(service.families))
	result = append(result, service.families...)
	return result, nil
}

func (service *FontService) ensureFamilies(ctx context.Context) error {
	service.mu.RLock()
	if service.loaded {
		err := service.err
		service.mu.RUnlock()
		return err
	}
	service.mu.RUnlock()

	service.mu.Lock()
	defer service.mu.Unlock()
	if service.loaded {
		return service.err
	}

	service.families, service.err = scanFontFamilies(ctx)
	service.loaded = true
	return service.err
}

func scanFontFamilies(ctx context.Context) ([]string, error) {
	dirs := fontDirectories()
	if len(dirs) == 0 {
		return []string{}, nil
	}

	displayFamilies := make(map[string]string, 512)
	for _, dir := range dirs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if strings.TrimSpace(dir) == "" {
			continue
		}

		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			continue
		}

		err = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return nil
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			if !isFontFile(path) {
				return nil
			}

			stat, err := entry.Info()
			if err != nil || stat.Size() <= 0 || stat.Size() > maxFontFileSizeBytes {
				return nil
			}

			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}

			for _, family := range fontFamiliesFromFontData(data) {
				if strings.HasPrefix(family, ".") {
					continue
				}
				key := normalizeFontFamilyKey(family)
				if key == "" {
					continue
				}
				if _, exists := displayFamilies[key]; !exists {
					displayFamilies[key] = family
				}
			}

			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	families := make([]string, 0, len(displayFamilies))
	for _, family := range displayFamilies {
		families = append(families, family)
	}
	sort.Strings(families)
	if families == nil {
		return []string{}, nil
	}
	return families, nil
}

func fontFamiliesFromFontData(data []byte) []string {
	if len(data) == 0 {
		return nil
	}

	results := make([]string, 0, 2)
	seen := make(map[string]struct{}, 2)
	push := func(family string) {
		trimmed := strings.TrimSpace(family)
		if trimmed == "" {
			return
		}
		key := normalizeFontFamilyKey(trimmed)
		if key == "" {
			return
		}
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		results = append(results, trimmed)
	}

	if isFontCollectionData(data) {
		collection, err := opentype.ParseCollection(data)
		if err != nil {
			return nil
		}
		for index := 0; index < collection.NumFonts(); index++ {
			font, err := collection.Font(index)
			if err != nil {
				continue
			}
			push(fontFamilyName(font))
		}
		return results
	}

	font, err := opentype.Parse(data)
	if err != nil {
		return nil
	}
	push(fontFamilyName(font))
	return results
}

func fontFamilyName(font *sfnt.Font) string {
	var buf sfnt.Buffer
	typographicFamily := readFontName(font, &buf, sfnt.NameIDTypographicFamily)
	wwsFamily := readFontName(font, &buf, sfnt.NameIDWWSFamily)
	legacyFamily := readFontName(font, &buf, sfnt.NameIDFamily)
	fullName := readFontName(font, &buf, sfnt.NameIDFull)
	postScript := readFontName(font, &buf, sfnt.NameIDPostScript)
	return resolveCatalogFontFamily(typographicFamily, wwsFamily, legacyFamily, fullName, postScript)
}

func readFontName(font *sfnt.Font, buf *sfnt.Buffer, id sfnt.NameID) string {
	name, err := font.Name(buf, id)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(name)
}

func resolveCatalogFontFamily(typographicFamily, wwsFamily, legacyFamily, fullName, postScript string) string {
	if isHiddenCatalogFontFamily(legacyFamily) || isHiddenCatalogFontFamily(wwsFamily) {
		return ""
	}
	return firstPublicCatalogFontFamily(legacyFamily, wwsFamily, typographicFamily, fullName, postScript)
}

func firstPublicCatalogFontFamily(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" || isHiddenCatalogFontFamily(trimmed) {
			continue
		}
		return trimmed
	}
	return ""
}

func isHiddenCatalogFontFamily(value string) bool {
	return strings.HasPrefix(strings.TrimSpace(value), ".")
}

func isFontFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".ttf", ".otf", ".ttc", ".otc":
		return true
	default:
		return false
	}
}

func isFontCollectionData(data []byte) bool {
	return len(data) >= 4 && string(data[:4]) == "ttcf"
}

func fontDirectories() []string {
	home, _ := os.UserHomeDir()
	return platformFontDirectories(home)
}

func normalizeFontFamilyKey(value string) string {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.Trim(trimmed, `"'`)
	trimmed = strings.NewReplacer("-", " ", "_", " ").Replace(trimmed)
	if trimmed == "" {
		return ""
	}
	return strings.ToLower(strings.Join(strings.Fields(trimmed), " "))
}
