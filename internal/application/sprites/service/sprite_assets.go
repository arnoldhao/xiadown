package service

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/HugoSmits86/nativewebp"
	"github.com/google/uuid"
	xdraw "golang.org/x/image/draw"
	"xiadown/internal/application/sprites/dto"
)

func normalizeSpriteID(scope string, seed string, current string) string {
	if parsed, err := uuid.Parse(strings.TrimSpace(current)); err == nil {
		return parsed.String()
	}
	if scope == scopeImported {
		return uuid.NewString()
	}
	seed = strings.TrimSpace(seed)
	if seed == "" {
		seed = uuid.NewString()
	}
	return uuid.NewSHA1(builtinSpriteNamespace, []byte(scope+":"+seed)).String()
}

func normalizeSpriteAuthorID(scope string, seed string, current string) string {
	if parsed, err := uuid.Parse(strings.TrimSpace(current)); err == nil {
		return parsed.String()
	}
	if scope == scopeImported {
		return uuid.NewString()
	}
	if seed == "" {
		seed = "builtin-author"
	}
	return uuid.NewSHA1(builtinSpriteNamespace, []byte(scope+":author:"+seed)).String()
}

func normalizeSpriteName(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "Sprite"
	}
	replacer := strings.NewReplacer("-", " ", "_", " ")
	return strings.TrimSpace(replacer.Replace(trimmed))
}

func normalizeSpriteVersion(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "1.0"
	}
	return trimmed
}

func normalizeSpriteSourceType(value string) string {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	switch trimmed {
	case "png", "zip":
		return trimmed
	default:
		return ""
	}
}

func normalizeSpriteOrigin(value string) string {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	switch trimmed {
	case "online":
		return trimmed
	default:
		return ""
	}
}

func normalizeSpriteSliceGrid(value spriteSliceGrid, fallbackWidth int, fallbackHeight int) spriteSliceGrid {
	if isValidSpriteBoundaryList(value.X, spriteColumns, fallbackWidth) && isValidSpriteBoundaryList(value.Y, spriteRows, fallbackHeight) {
		return cloneSpriteSliceGrid(value)
	}
	return defaultSpriteSliceGrid(fallbackWidth, fallbackHeight)
}

func defaultSpriteSliceGrid(width int, height int) spriteSliceGrid {
	return spriteSliceGrid{
		X: buildEqualBoundaries(maxInt(width, spriteColumns), spriteColumns),
		Y: buildEqualBoundaries(maxInt(height, spriteRows), spriteRows),
	}
}

func buildEqualBoundaries(total int, segments int) []int {
	safeSegments := maxInt(1, segments)
	safeTotal := maxInt(total, safeSegments)
	result := make([]int, safeSegments+1)
	for index := 0; index <= safeSegments; index++ {
		result[index] = int(math.Round(float64(safeTotal*index) / float64(safeSegments)))
	}
	result[0] = 0
	result[safeSegments] = safeTotal
	for index := 1; index < safeSegments; index++ {
		if result[index] <= result[index-1] {
			result[index] = result[index-1] + 1
		}
	}
	return result
}

func validateSpriteSliceGrid(value dto.SpriteSliceGrid, columns int, rows int, sheetWidth int, sheetHeight int) (spriteSliceGrid, error) {
	grid := spriteSliceGrid{
		X: append([]int(nil), value.X...),
		Y: append([]int(nil), value.Y...),
	}
	if !isValidSpriteBoundaryList(grid.X, columns, sheetWidth) {
		return spriteSliceGrid{}, fmt.Errorf("sprite slice x grid must contain %d ascending boundaries from 0 to %d", columns+1, sheetWidth)
	}
	if !isValidSpriteBoundaryList(grid.Y, rows, sheetHeight) {
		return spriteSliceGrid{}, fmt.Errorf("sprite slice y grid must contain %d ascending boundaries from 0 to %d", rows+1, sheetHeight)
	}
	return grid, nil
}

func isValidSpriteBoundaryList(values []int, segments int, total int) bool {
	if len(values) != segments+1 {
		return false
	}
	if values[0] != 0 {
		return false
	}
	for index := 1; index < len(values); index++ {
		if values[index] <= values[index-1] {
			return false
		}
	}
	return total > 0 && values[len(values)-1] == total
}

func cloneSpriteSliceGrid(value spriteSliceGrid) spriteSliceGrid {
	return spriteSliceGrid{
		X: append([]int(nil), value.X...),
		Y: append([]int(nil), value.Y...),
	}
}

func toDTOSpriteSliceGrid(value spriteSliceGrid) dto.SpriteSliceGrid {
	return dto.SpriteSliceGrid{
		X: append([]int(nil), value.X...),
		Y: append([]int(nil), value.Y...),
	}
}

func spriteSliceGridSize(value spriteSliceGrid) (int, int) {
	width := 0
	height := 0
	if len(value.X) > 0 {
		width = value.X[len(value.X)-1]
	}
	if len(value.Y) > 0 {
		height = value.Y[len(value.Y)-1]
	}
	return width, height
}

func parseSpriteTimestamp(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

func validateSpriteDimensions(width int, height int) spriteDimensionValidation {
	if width <= 0 || height <= 0 {
		return spriteDimensionValidation{validationMessage: "sprite image must have a positive size"}
	}
	if width != height {
		return spriteDimensionValidation{
			validationMessage: fmt.Sprintf("sprite image must be square, got %dx%d", width, height),
		}
	}
	if width < spriteMinWidth || height < spriteMinHeight {
		return spriteDimensionValidation{
			validationMessage: fmt.Sprintf(
				"sprite image %dx%d is too small; sprite sheet must be at least %dx%d",
				width,
				height,
				spriteMinWidth,
				spriteMinHeight,
			),
		}
	}
	if width > spriteMaxWidth || height > spriteMaxHeight {
		return spriteDimensionValidation{
			validationMessage: fmt.Sprintf(
				"sprite image %dx%d exceeds the maximum %dx%d sheet size",
				width,
				height,
				spriteMaxWidth,
				spriteMaxHeight,
			),
		}
	}

	return spriteDimensionValidation{
		sheetWidth:  width,
		sheetHeight: height,
	}
}

func validateManifestApp(manifest spriteManifest) error {
	app := strings.TrimSpace(manifest.App)
	if app == "" || app == spriteManifestAppID {
		return nil
	}
	return fmt.Errorf("sprite manifest app %q is not supported", app)
}

func readSpriteManifest(spriteDir string) spriteManifest {
	data, err := os.ReadFile(filepath.Join(spriteDir, spriteManifestFileName))
	if err != nil {
		return spriteManifest{}
	}
	var manifest spriteManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return spriteManifest{}
	}
	return manifest
}

func readSpriteManifestFromDirectory(spriteDir string) (spriteManifest, error) {
	data, err := os.ReadFile(filepath.Join(spriteDir, spriteManifestFileName))
	if err != nil {
		if os.IsNotExist(err) {
			return spriteManifest{}, nil
		}
		return spriteManifest{}, fmt.Errorf("read sprite manifest: %w", err)
	}
	var manifest spriteManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return spriteManifest{}, fmt.Errorf("decode sprite manifest: %w", err)
	}
	return manifest, nil
}

func writeSpriteManifest(spriteDir string, manifest spriteManifest) error {
	manifest.App = spriteManifestAppID
	manifest.FrameCount = spriteFrameCount
	manifest.Columns = spriteColumns
	manifest.Rows = spriteRows
	manifest.SpriteFile = spriteExportFileName

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal sprite manifest: %w", err)
	}
	data = append(data, '\n')
	if err := writeFileAtomic(filepath.Join(spriteDir, spriteManifestFileName), data, 0o644); err != nil {
		return fmt.Errorf("write sprite manifest: %w", err)
	}
	return nil
}

func findImportSpriteImageFile(spriteDir string, preferred string) (string, string, error) {
	if strings.TrimSpace(preferred) != "" {
		candidate, err := secureSpriteDirectoryFilePath(spriteDir, preferred)
		if err != nil {
			return "", "", err
		}
		if stat, err := os.Stat(candidate); err == nil && !stat.IsDir() && isSupportedSpriteImage(candidate) {
			return filepath.Base(candidate), candidate, nil
		}
		return "", "", fmt.Errorf("sprite image %q not found in %s", preferred, spriteDir)
	}

	preferredPath := filepath.Join(spriteDir, spriteExportFileName)
	if stat, err := os.Stat(preferredPath); err == nil && !stat.IsDir() && isSupportedSpriteImage(preferredPath) {
		return spriteExportFileName, preferredPath, nil
	}

	entries, err := os.ReadDir(spriteDir)
	if err != nil {
		return "", "", fmt.Errorf("read sprite directory: %w", err)
	}

	var fallback string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !isSupportedSpriteImage(name) {
			continue
		}
		if fallback == "" {
			fallback = name
		}
	}
	if fallback != "" {
		return fallback, filepath.Join(spriteDir, fallback), nil
	}
	return "", "", fmt.Errorf("sprite image not found in %s", spriteDir)
}

func secureSpriteDirectoryFilePath(spriteDir string, name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", fmt.Errorf("sprite file name is required")
	}
	if filepath.IsAbs(trimmed) {
		return "", fmt.Errorf("sprite file %q must be relative", name)
	}

	cleaned := filepath.Clean(trimmed)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("sprite file %q escapes the sprite directory", name)
	}

	candidate := filepath.Join(spriteDir, cleaned)
	rootEval, rootErr := filepath.EvalSymlinks(spriteDir)
	candidateEval, candidateErr := filepath.EvalSymlinks(candidate)
	if rootErr != nil || candidateErr != nil {
		return candidate, nil
	}

	relative, err := filepath.Rel(rootEval, candidateEval)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("sprite file %q escapes the sprite directory", name)
	}
	return candidate, nil
}

func readImageSize(path string) (int, int, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, 0, fmt.Errorf("open sprite image: %w", err)
	}
	defer file.Close()

	config, _, err := image.DecodeConfig(file)
	if err != nil {
		return 0, 0, fmt.Errorf("decode sprite image: %w", err)
	}
	return config.Width, config.Height, nil
}

func decodeImage(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open sprite image: %w", err)
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("decode sprite image: %w", err)
	}
	return img, nil
}

func writeStoredSpritePNG(sourcePath string, targetPath string) error {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open sprite image: %w", err)
	}
	defer sourceFile.Close()
	return writeFileAtomicFromReader(targetPath, sourceFile, 0o644)
}

func buildSpriteCoverImage(spritePath string) (*image.NRGBA, error) {
	if strings.TrimSpace(spritePath) == "" {
		return nil, nil
	}

	img, err := decodeImage(spritePath)
	if err != nil {
		return nil, err
	}
	bounds := img.Bounds()
	coverWidth := bounds.Dx() / maxInt(1, spriteColumns)
	coverHeight := bounds.Dy() / maxInt(1, spriteRows)
	if coverWidth <= 0 || coverHeight <= 0 {
		return nil, nil
	}

	cover := image.NewNRGBA(image.Rect(0, 0, coverWidth, coverHeight))
	xdraw.Draw(cover, cover.Bounds(), img, bounds.Min, xdraw.Src)
	removeMagentaToTransparent(cover)
	return cover, nil
}

func buildSpriteCoverPNG(sprite dto.Sprite) ([]byte, error) {
	cover, err := buildSpriteCoverImage(sprite.SpritePath)
	if err != nil || cover == nil {
		return nil, err
	}

	var buffer bytes.Buffer
	if err := png.Encode(&buffer, cover); err != nil {
		return nil, fmt.Errorf("encode sprite cover: %w", err)
	}
	return buffer.Bytes(), nil
}

func buildSpritePreviewWebP(spritePath string) ([]byte, error) {
	cover, err := buildSpriteCoverImage(spritePath)
	if err != nil || cover == nil {
		return nil, err
	}

	preview := image.NewNRGBA(image.Rect(0, 0, spritePreviewSize, spritePreviewSize))
	xdraw.CatmullRom.Scale(preview, preview.Bounds(), cover, cover.Bounds(), xdraw.Src, nil)

	var buffer bytes.Buffer
	if err := nativewebp.Encode(&buffer, preview, nil); err != nil {
		return nil, fmt.Errorf("encode sprite preview: %w", err)
	}
	return buffer.Bytes(), nil
}

func removeMagentaToTransparent(img *image.NRGBA) {
	if img == nil {
		return
	}

	background, ok := sampleBackgroundColor(img)
	if ok {
		removeEdgeConnectedBackground(img, func(r uint8, g uint8, b uint8) bool {
			return matchesBackgroundColor(r, g, b, background) || isNearPureMagenta(r, g, b)
		})
		removeMagentaKeyPixels(img)
		return
	}
	removeEdgeConnectedBackground(img, isNearPureMagenta)
	removeMagentaKeyPixels(img)
}

func sampleBackgroundColor(img *image.NRGBA) (backgroundColor, bool) {
	if img == nil || img.Bounds().Dx() <= 0 || img.Bounds().Dy() <= 0 {
		return backgroundColor{}, false
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	samples := make([]backgroundColor, 0, len(backgroundSamplePoints))
	for _, point := range backgroundSamplePoints {
		x := clampInt(int(math.Round(float64(width-1)*point[0])), 0, width-1)
		y := clampInt(int(math.Round(float64(height-1)*point[1])), 0, height-1)
		offset := img.PixOffset(bounds.Min.X+x, bounds.Min.Y+y)
		alpha := img.Pix[offset+3]
		if alpha == 0 {
			continue
		}
		r := img.Pix[offset]
		g := img.Pix[offset+1]
		b := img.Pix[offset+2]
		if !isMagentaLike(r, g, b) {
			continue
		}
		samples = append(samples, backgroundColor{r: r, g: g, b: b})
	}
	if len(samples) == 0 {
		return backgroundColor{}, false
	}

	var totalR, totalG, totalB int
	for _, sample := range samples {
		totalR += int(sample.r)
		totalG += int(sample.g)
		totalB += int(sample.b)
	}

	return backgroundColor{
		r: uint8(totalR / len(samples)),
		g: uint8(totalG / len(samples)),
		b: uint8(totalB / len(samples)),
	}, true
}

func matchesBackgroundColor(r uint8, g uint8, b uint8, background backgroundColor) bool {
	if !isMagentaLike(r, g, b) {
		return false
	}

	totalDistance := absInt(int(r)-int(background.r)) + absInt(int(g)-int(background.g)) + absInt(int(b)-int(background.b))
	return absInt(int(r)-int(background.r)) <= 120 &&
		absInt(int(g)-int(background.g)) <= 120 &&
		absInt(int(b)-int(background.b)) <= 120 &&
		totalDistance <= 232
}

func isMagentaLike(r uint8, g uint8, b uint8) bool {
	minChannel := minInt(int(r), int(b))
	return int(r) >= 140 &&
		int(b) >= 140 &&
		minChannel-int(g) >= 20 &&
		absInt(int(r)-int(b)) <= 120
}

func removeEdgeConnectedBackground(img *image.NRGBA, matcher func(r uint8, g uint8, b uint8) bool) {
	if img == nil || matcher == nil {
		return
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= 0 || height <= 0 {
		return
	}

	visited := make([]uint8, width*height)
	queue := make([]int, 0, width*2+height*2)

	enqueue := func(x int, y int) {
		if x < 0 || y < 0 || x >= width || y >= height {
			return
		}
		index := y*width + x
		if visited[index] != 0 {
			return
		}
		offset := img.PixOffset(bounds.Min.X+x, bounds.Min.Y+y)
		if img.Pix[offset+3] == 0 || !matcher(img.Pix[offset], img.Pix[offset+1], img.Pix[offset+2]) {
			return
		}
		visited[index] = 1
		queue = append(queue, index)
	}

	for x := 0; x < width; x++ {
		enqueue(x, 0)
		enqueue(x, height-1)
	}
	for y := 0; y < height; y++ {
		enqueue(0, y)
		enqueue(width-1, y)
	}

	for cursor := 0; cursor < len(queue); cursor++ {
		index := queue[cursor]
		x := index % width
		y := index / width
		offset := img.PixOffset(bounds.Min.X+x, bounds.Min.Y+y)
		img.Pix[offset] = 0
		img.Pix[offset+1] = 0
		img.Pix[offset+2] = 0
		img.Pix[offset+3] = 0

		enqueue(x-1, y)
		enqueue(x+1, y)
		enqueue(x, y-1)
		enqueue(x, y+1)
	}
}

func removeMagentaKeyPixels(img *image.NRGBA) {
	if img == nil {
		return
	}

	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			offset := img.PixOffset(x, y)
			if img.Pix[offset+3] == 0 || !isMagentaKeyPixel(img.Pix[offset], img.Pix[offset+1], img.Pix[offset+2]) {
				continue
			}
			img.Pix[offset] = 0
			img.Pix[offset+1] = 0
			img.Pix[offset+2] = 0
			img.Pix[offset+3] = 0
		}
	}
}

func isMagentaKeyPixel(r uint8, g uint8, b uint8) bool {
	return r >= spriteMagentaKeyRedMin && g <= spriteMagentaKeyGreenMax && b >= spriteMagentaKeyBlueMin
}

func isNearPureMagenta(r uint8, g uint8, b uint8) bool {
	return r >= 236 && g <= 24 && b >= 236
}

func coverPNGToDataURL(payload []byte) string {
	if len(payload) == 0 {
		return ""
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(payload)
}

func clampInt(value int, minimum int, maximum int) int {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}

func minInt(left int, right int) int {
	if left < right {
		return left
	}
	return right
}

func maxInt(left int, right int) int {
	if left > right {
		return left
	}
	return right
}

func maxTime(left time.Time, right time.Time) time.Time {
	if left.After(right) {
		return left
	}
	return right
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func writeFileAtomic(targetPath string, data []byte, perm fs.FileMode) error {
	return writeFileAtomicFromReader(targetPath, bytes.NewReader(data), perm)
}

func writeFileAtomicFromReader(targetPath string, reader io.Reader, perm fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("create sprite file directory: %w", err)
	}
	tempFile, err := os.CreateTemp(filepath.Dir(targetPath), "."+filepath.Base(targetPath)+".*.tmp")
	if err != nil {
		return fmt.Errorf("create sprite file: %w", err)
	}
	tempPath := tempFile.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tempPath)
		}
	}()

	if _, err := io.Copy(tempFile, reader); err != nil {
		tempFile.Close()
		return fmt.Errorf("write sprite file: %w", err)
	}
	if err := tempFile.Chmod(perm); err != nil {
		tempFile.Close()
		return fmt.Errorf("chmod sprite file: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("close sprite file: %w", err)
	}
	if err := replaceFile(tempPath, targetPath); err != nil {
		return err
	}
	cleanup = false
	return nil
}

func replaceFile(sourcePath string, targetPath string) error {
	err := os.Rename(sourcePath, targetPath)
	if err == nil {
		return nil
	}
	if runtime.GOOS != "windows" {
		return fmt.Errorf("replace sprite file: %w", err)
	}
	if _, err := os.Stat(sourcePath); err != nil {
		return fmt.Errorf("replace sprite file: %w", err)
	}
	if _, err := os.Stat(targetPath); err != nil {
		return fmt.Errorf("replace sprite file: %w", err)
	}
	if err := os.Remove(targetPath); err != nil {
		return fmt.Errorf("replace sprite file: %w", err)
	}
	if err := os.Rename(sourcePath, targetPath); err != nil {
		return fmt.Errorf("replace sprite file: %w", err)
	}
	return nil
}

func copyFileFS(sourceFS fs.FS, sourcePath string, targetPath string) error {
	sourceFile, err := sourceFS.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open embedded sprite file: %w", err)
	}
	defer sourceFile.Close()

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("create sprite file directory: %w", err)
	}
	targetFile, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("create sprite file: %w", err)
	}
	defer targetFile.Close()

	if _, err := io.Copy(targetFile, sourceFile); err != nil {
		return fmt.Errorf("copy embedded sprite file: %w", err)
	}
	return nil
}

func extractZipFile(file *zip.File, targetPath string, maxBytes int64) (int64, error) {
	if maxBytes <= 0 {
		return 0, fmt.Errorf("sprite archive contents exceed the %s limit", formatByteLimit(spriteMaxZipSizeBytes))
	}

	readCloser, err := file.Open()
	if err != nil {
		return 0, fmt.Errorf("open archived sprite image: %w", err)
	}
	defer readCloser.Close()

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return 0, fmt.Errorf("create archived sprite directory: %w", err)
	}
	outputFile, err := os.Create(targetPath)
	if err != nil {
		return 0, fmt.Errorf("create archived sprite image: %w", err)
	}
	defer outputFile.Close()

	limitedReader := &io.LimitedReader{R: readCloser, N: maxBytes + 1}
	written, err := io.Copy(outputFile, limitedReader)
	if err != nil {
		return written, fmt.Errorf("extract archived sprite image: %w", err)
	}
	if written > maxBytes {
		return written, fmt.Errorf("sprite archive contents exceed the %s limit", formatByteLimit(spriteMaxZipSizeBytes))
	}
	return written, nil
}

func readZipFile(file *zip.File, maxBytes int64) ([]byte, int64, error) {
	if maxBytes <= 0 {
		return nil, 0, fmt.Errorf("sprite archive contents exceed the %s limit", formatByteLimit(spriteMaxZipSizeBytes))
	}

	readCloser, err := file.Open()
	if err != nil {
		return nil, 0, fmt.Errorf("open archived sprite file: %w", err)
	}
	defer readCloser.Close()

	var buffer bytes.Buffer
	limitedReader := &io.LimitedReader{R: readCloser, N: maxBytes + 1}
	written, err := io.Copy(&buffer, limitedReader)
	if err != nil {
		return nil, written, fmt.Errorf("read archived sprite file: %w", err)
	}
	if written > maxBytes {
		return nil, written, fmt.Errorf("sprite archive contents exceed the %s limit", formatByteLimit(spriteMaxZipSizeBytes))
	}
	return buffer.Bytes(), written, nil
}

func formatByteLimit(bytes int64) string {
	if bytes > 0 && bytes%(1024*1024) == 0 {
		return fmt.Sprintf("%dMB", bytes/(1024*1024))
	}
	return fmt.Sprintf("%d bytes", bytes)
}

func sanitizeImportFileName(value string) string {
	baseName := strings.TrimSpace(filepath.Base(value))
	if baseName == "" {
		return spriteExportFileName
	}
	baseName = strings.ReplaceAll(baseName, string(filepath.Separator), "_")
	return baseName
}

func detectSpriteSourceType(sourcePath string) string {
	if sourcePath == "" {
		return ""
	}
	if stat, err := os.Stat(sourcePath); err == nil && stat.IsDir() {
		return "directory"
	}
	switch strings.ToLower(filepath.Ext(sourcePath)) {
	case ".png":
		return "png"
	case ".zip":
		return "zip"
	default:
		return ""
	}
}

func isSupportedSpriteImportFile(path string) bool {
	if isSupportedSpriteImage(path) {
		return true
	}
	return strings.EqualFold(filepath.Ext(path), ".zip")
}

func isSupportedSpriteImage(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		return true
	default:
		return false
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func errorsIsNotExist(err error) bool {
	return err != nil && (os.IsNotExist(err) || strings.Contains(strings.ToLower(err.Error()), "does not exist"))
}
