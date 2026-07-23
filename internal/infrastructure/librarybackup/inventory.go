package librarybackup

import (
	"context"
	"database/sql"
	"path"
	"sort"
	"strings"

	domainbackup "xiadown/internal/domain/librarybackup"
)

type inventoryRoot struct {
	CatalogID string
	ID        string
	Name      string
	Path      string
	Mode      string
	Status    string
}

func buildInventory(ctx context.Context, db *sql.DB) ([]domainbackup.CatalogInventory, []domainbackup.FileInventory, error) {
	catalogs, catalogIndexes, err := readCatalogInventory(ctx, db)
	if err != nil {
		return nil, nil, err
	}
	roots, err := readStorageRootInventory(ctx, db, catalogs, catalogIndexes)
	if err != nil {
		return nil, nil, err
	}
	files, err := readFileInventory(ctx, db, roots, catalogs, catalogIndexes)
	if err != nil {
		return nil, nil, err
	}
	for index := range catalogs {
		sort.Slice(catalogs[index].StorageRoots, func(i, j int) bool {
			return catalogs[index].StorageRoots[i].ID < catalogs[index].StorageRoots[j].ID
		})
	}
	sort.Slice(catalogs, func(i, j int) bool { return catalogs[i].ID < catalogs[j].ID })
	sort.Slice(files, func(i, j int) bool {
		left, right := files[i], files[j]
		if left.FileID != right.FileID {
			return left.FileID < right.FileID
		}
		if left.AssetID != right.AssetID {
			return left.AssetID < right.AssetID
		}
		return left.ItemID < right.ItemID
	})
	return catalogs, files, nil
}

func readCatalogInventory(ctx context.Context, db *sql.DB) ([]domainbackup.CatalogInventory, map[string]int, error) {
	rows, err := db.QueryContext(ctx, `
SELECT c.id, c.is_default,
       COUNT(DISTINCT i.id) AS item_count,
       COUNT(a.id) AS asset_count
FROM library_catalogs c
LEFT JOIN library_catalog_items i ON i.catalog_id = c.id
LEFT JOIN library_item_assets a ON a.item_id = i.id
GROUP BY c.id, c.is_default
ORDER BY c.id
`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	catalogs := make([]domainbackup.CatalogInventory, 0)
	indexes := make(map[string]int)
	for rows.Next() {
		var catalog domainbackup.CatalogInventory
		var isDefault int
		if err := rows.Scan(&catalog.ID, &isDefault, &catalog.ItemCount, &catalog.AssetCount); err != nil {
			return nil, nil, err
		}
		catalog.IsDefault = isDefault != 0
		catalog.StorageRoots = make([]domainbackup.StorageRootInventory, 0)
		indexes[catalog.ID] = len(catalogs)
		catalogs = append(catalogs, catalog)
	}
	return catalogs, indexes, rows.Err()
}

func readStorageRootInventory(
	ctx context.Context,
	db *sql.DB,
	catalogs []domainbackup.CatalogInventory,
	indexes map[string]int,
) ([]inventoryRoot, error) {
	rows, err := db.QueryContext(ctx, `
SELECT catalog_id, id, name, path, mode, status
FROM library_storage_roots
ORDER BY catalog_id, id
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	roots := make([]inventoryRoot, 0)
	for rows.Next() {
		var root inventoryRoot
		if err := rows.Scan(&root.CatalogID, &root.ID, &root.Name, &root.Path, &root.Mode, &root.Status); err != nil {
			return nil, err
		}
		catalogIndex, ok := indexes[root.CatalogID]
		if ok {
			catalogs[catalogIndex].StorageRoots = append(catalogs[catalogIndex].StorageRoots, domainbackup.StorageRootInventory{
				ID: root.ID, Name: root.Name, Mode: root.Mode, Status: root.Status,
			})
		}
		roots = append(roots, root)
	}
	return roots, rows.Err()
}

func readFileInventory(
	ctx context.Context,
	db *sql.DB,
	roots []inventoryRoot,
	catalogs []domainbackup.CatalogInventory,
	catalogIndexes map[string]int,
) ([]domainbackup.FileInventory, error) {
	rows, err := db.QueryContext(ctx, `
SELECT f.id, f.kind, f.storage_mode, COALESCE(f.storage_local_path, ''),
       COALESCE(i.catalog_id, ''), COALESCE(i.id, ''), COALESCE(a.id, ''),
       COALESCE(a.role, ''), COALESCE(a.position, 0)
FROM library_files f
LEFT JOIN library_item_assets a ON a.file_id = f.id
LEFT JOIN library_catalog_items i ON i.id = a.item_id
ORDER BY f.id, a.id
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	files := make([]domainbackup.FileInventory, 0)
	for rows.Next() {
		var file domainbackup.FileInventory
		var localPath string
		if err := rows.Scan(
			&file.FileID, &file.Kind, &file.StorageMode, &localPath,
			&file.CatalogID, &file.ItemID, &file.AssetID, &file.Role, &file.Position,
		); err != nil {
			return nil, err
		}
		rootID, relativePath := bestRelativeRoot(localPath, file.CatalogID, roots)
		file.StorageRoot = rootID
		file.RelativePath = relativePath
		if file.AssetID != "" && rootID != "" {
			incrementRootAssetCount(catalogs, catalogIndexes, file.CatalogID, rootID)
		}
		files = append(files, file)
	}
	return files, rows.Err()
}

func incrementRootAssetCount(
	catalogs []domainbackup.CatalogInventory,
	catalogIndexes map[string]int,
	catalogID string,
	rootID string,
) {
	catalogIndex, ok := catalogIndexes[catalogID]
	if !ok {
		return
	}
	for index := range catalogs[catalogIndex].StorageRoots {
		if catalogs[catalogIndex].StorageRoots[index].ID == rootID {
			catalogs[catalogIndex].StorageRoots[index].AssetCount++
			return
		}
	}
}

func bestRelativeRoot(localPath, catalogID string, roots []inventoryRoot) (string, string) {
	if strings.TrimSpace(localPath) == "" {
		return "", ""
	}
	bestRootID := ""
	bestRelative := ""
	bestLength := -1
	for _, root := range roots {
		if catalogID != "" && root.CatalogID != catalogID {
			continue
		}
		relative, ok := safeLexicalRelative(root.Path, localPath)
		if !ok {
			continue
		}
		normalizedRoot := normalizePortablePath(root.Path)
		if len(normalizedRoot) > bestLength {
			bestRootID = root.ID
			bestRelative = relative
			bestLength = len(normalizedRoot)
		}
	}
	return bestRootID, bestRelative
}

// safeLexicalRelative intentionally does not resolve symlinks or touch media.
// It only emits a slash-normalized relative path after proving path-boundary
// containment. This also keeps manifests portable when inspecting Windows
// paths on macOS and vice versa.
func safeLexicalRelative(rootValue, fileValue string) (string, bool) {
	root := normalizePortablePath(rootValue)
	file := normalizePortablePath(fileValue)
	if root == "" || file == "" || root == "." || file == "." {
		return "", false
	}
	comparisonRoot, comparisonFile := root, file
	if looksLikeWindowsPath(root) || looksLikeWindowsPath(file) {
		comparisonRoot = strings.ToLower(root)
		comparisonFile = strings.ToLower(file)
	}
	if comparisonFile == comparisonRoot {
		return ".", true
	}
	prefix := strings.TrimSuffix(comparisonRoot, "/") + "/"
	if !strings.HasPrefix(comparisonFile, prefix) {
		return "", false
	}
	rootWithoutSlash := strings.TrimSuffix(root, "/")
	if len(file) <= len(rootWithoutSlash) {
		return "", false
	}
	relative := strings.TrimPrefix(file[len(rootWithoutSlash):], "/")
	if relative == "" || relative == "." || relative == ".." || strings.HasPrefix(relative, "../") || path.IsAbs(relative) {
		return "", false
	}
	return relative, true
}

func normalizePortablePath(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if value == "" {
		return ""
	}
	return path.Clean(value)
}

func looksLikeWindowsPath(value string) bool {
	return len(value) >= 2 && ((value[0] >= 'A' && value[0] <= 'Z') || (value[0] >= 'a' && value[0] <= 'z')) && value[1] == ':'
}
