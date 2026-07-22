package libraryapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	"xiadown/internal/application/apperrors"
	"xiadown/internal/application/library/dto"
	"xiadown/internal/application/networkpolicy"
	"xiadown/internal/domain/library"
)

const maxTaskBodyBytes = 32 << 10

// TaskService is deliberately expressed in terms of the existing Library
// application service contract. The public API must not mutate operation
// repositories directly because doing so would bypass scheduler, history and
// realtime side effects owned by LibraryService.
type TaskService interface {
	ListOperations(context.Context, dto.ListOperationsRequest) ([]dto.OperationListItemDTO, error)
	GetOperation(context.Context, dto.GetOperationRequest) (dto.LibraryOperationDTO, error)
	CreateYTDLPJob(context.Context, dto.CreateYTDLPJobRequest) (dto.LibraryOperationDTO, error)
	CancelOperation(context.Context, dto.CancelOperationRequest) (dto.LibraryOperationDTO, error)
	ResumeOperation(context.Context, dto.ResumeOperationRequest) (dto.LibraryOperationDTO, error)
}

type TaskAPI struct{ service TaskService }

func NewTaskAPI(service TaskService) (*TaskAPI, error) {
	if service == nil {
		return nil, errors.New("Library public task API requires an application service")
	}
	return &TaskAPI{service: service}, nil
}

func (api *TaskAPI) Routes() []ProtectedRoute {
	return []ProtectedRoute{
		{Method: http.MethodGet, Path: "/api/v1/tasks", Scope: library.DeviceScopeTasksRead, Handler: http.HandlerFunc(api.list)},
		{Method: http.MethodGet, Path: "/api/v1/tasks/{id}", Scope: library.DeviceScopeTasksRead, Handler: http.HandlerFunc(api.get)},
		{Method: http.MethodPost, Path: "/api/v1/tasks", Scope: library.DeviceScopeTasksCreate, Handler: http.HandlerFunc(api.create)},
		{Method: http.MethodPost, Path: "/api/v1/tasks/{id}/cancel", Scope: library.DeviceScopeTasksControl, Handler: http.HandlerFunc(api.cancel)},
		{Method: http.MethodPost, Path: "/api/v1/tasks/{id}/resume", Scope: library.DeviceScopeTasksControl, Handler: http.HandlerFunc(api.resume)},
	}
}

func (api *TaskAPI) get(w http.ResponseWriter, request *http.Request) {
	operation, err := api.service.GetOperation(request.Context(), dto.GetOperationRequest{
		OperationID: strings.TrimSpace(request.PathValue("id")),
	})
	if err != nil {
		writeTaskError(w, err, false)
		return
	}
	writeJSON(w, http.StatusOK, publicTaskFromOperation(operation))
}

type publicTaskProgress struct {
	Stage       string                       `json:"stage,omitempty"`
	Percent     *int                         `json:"percent,omitempty"`
	Current     *int64                       `json:"current,omitempty"`
	Total       *int64                       `json:"total,omitempty"`
	Speed       string                       `json:"speed,omitempty"`
	SpeedMetric *dto.OperationSpeedMetricDTO `json:"speedMetric,omitempty"`
	UpdatedAt   string                       `json:"updatedAt,omitempty"`
}

// publicTask is an explicit allowlist. Do not embed LibraryOperationDTO or
// OperationListItemDTO here: both contain local paths, input/output JSON,
// internal correlation identifiers and process diagnostics.
type publicTask struct {
	ID          string                  `json:"id"`
	Name        string                  `json:"name"`
	Kind        string                  `json:"kind"`
	Status      string                  `json:"status"`
	Domain      string                  `json:"domain,omitempty"`
	Platform    string                  `json:"platform,omitempty"`
	Uploader    string                  `json:"uploader,omitempty"`
	PublishTime string                  `json:"publishTime,omitempty"`
	Progress    *publicTaskProgress     `json:"progress,omitempty"`
	Metrics     dto.OperationMetricsDTO `json:"metrics"`
	ErrorCode   string                  `json:"errorCode,omitempty"`
	CreatedAt   string                  `json:"createdAt"`
	StartedAt   string                  `json:"startedAt,omitempty"`
	FinishedAt  string                  `json:"finishedAt,omitempty"`
}

type publicTaskList struct {
	Tasks []publicTask `json:"tasks"`
}

func (api *TaskAPI) list(w http.ResponseWriter, request *http.Request) {
	query := request.URL.Query()
	searchQuery := query.Get("q")
	limit, limitErr := optionalInteger(query.Get("limit"))
	offset, offsetErr := optionalInteger(query.Get("offset"))
	if limitErr != nil || offsetErr != nil || limit < 0 || limit > 200 || offset < 0 {
		writeError(w, http.StatusBadRequest, "invalid_pagination")
		return
	}
	if !validPublicSearchQuery(searchQuery) {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	if limit == 0 {
		limit = 100
	}
	items, err := api.service.ListOperations(request.Context(), dto.ListOperationsRequest{
		Status: query["status"], Kinds: query["kind"], Query: searchQuery, Limit: limit, Offset: offset,
	})
	if err != nil {
		writeTaskError(w, err, false)
		return
	}
	result := publicTaskList{Tasks: make([]publicTask, 0, len(items))}
	for _, item := range items {
		result.Tasks = append(result.Tasks, publicTaskFromListItem(item))
	}
	writeJSON(w, http.StatusOK, result)
}

// createTaskPayload is the complete public task creation vocabulary. Fields
// that can name a local file or alter desktop execution context (libraryId,
// cookiesPath, app/resource sessions, run/retry IDs, source and caller) are not
// accepted. json.Decoder.DisallowUnknownFields enforces that boundary.
type createTaskPayload struct {
	URL                            string   `json:"url"`
	Title                          string   `json:"title,omitempty"`
	Mode                           string   `json:"mode,omitempty"`
	Quality                        string   `json:"quality,omitempty"`
	FormatID                       string   `json:"formatId,omitempty"`
	AudioFormatID                  string   `json:"audioFormatId,omitempty"`
	WriteThumbnail                 bool     `json:"writeThumbnail,omitempty"`
	SubtitleLangs                  []string `json:"subtitleLangs,omitempty"`
	SubtitleAuto                   bool     `json:"subtitleAuto,omitempty"`
	SubtitleAll                    bool     `json:"subtitleAll,omitempty"`
	SubtitleFormat                 string   `json:"subtitleFormat,omitempty"`
	TranscodePresetID              string   `json:"transcodePresetId,omitempty"`
	DeleteSourceFileAfterTranscode bool     `json:"deleteSourceFileAfterTranscode,omitempty"`
}

func (api *TaskAPI) create(w http.ResponseWriter, request *http.Request) {
	principal, ok := PrincipalFromContext(request.Context())
	if !ok || strings.TrimSpace(principal.DeviceID) == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	request.Body = http.MaxBytesReader(w, request.Body, maxTaskBodyBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var payload createTaskPayload
	if err := decoder.Decode(&payload); err != nil || ensureJSONEOF(decoder) != nil || payload.validate() != nil {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	operation, err := api.service.CreateYTDLPJob(request.Context(), dto.CreateYTDLPJobRequest{
		URL: payload.URL, Title: payload.Title, Mode: payload.Mode, Quality: payload.Quality,
		FormatID: payload.FormatID, AudioFormatID: payload.AudioFormatID,
		WriteThumbnail: payload.WriteThumbnail, SubtitleLangs: append([]string(nil), payload.SubtitleLangs...),
		SubtitleAuto: payload.SubtitleAuto, SubtitleAll: payload.SubtitleAll, SubtitleFormat: payload.SubtitleFormat,
		TranscodePresetID:              payload.TranscodePresetID,
		DeleteSourceFileAfterTranscode: payload.DeleteSourceFileAfterTranscode,
		Source:                         "public_api", Caller: strings.TrimSpace(principal.DeviceID),
	})
	if err != nil {
		writeTaskError(w, err, false)
		return
	}
	writeJSON(w, http.StatusCreated, publicTaskFromOperation(operation))
}

func (api *TaskAPI) cancel(w http.ResponseWriter, request *http.Request) {
	operation, err := api.service.CancelOperation(request.Context(), dto.CancelOperationRequest{
		OperationID: strings.TrimSpace(request.PathValue("id")),
	})
	if err != nil {
		writeTaskError(w, err, true)
		return
	}
	writeJSON(w, http.StatusOK, publicTaskFromOperation(operation))
}

func (api *TaskAPI) resume(w http.ResponseWriter, request *http.Request) {
	operation, err := api.service.ResumeOperation(request.Context(), dto.ResumeOperationRequest{
		OperationID: strings.TrimSpace(request.PathValue("id")),
	})
	if err != nil {
		writeTaskError(w, err, true)
		return
	}
	writeJSON(w, http.StatusOK, publicTaskFromOperation(operation))
}

func (payload createTaskPayload) validate() error {
	if invalidPublicTaskString(payload.URL, 8192, false) ||
		invalidPublicTaskString(payload.Title, 512, true) ||
		invalidPublicTaskString(payload.FormatID, 256, true) ||
		invalidPublicTaskString(payload.AudioFormatID, 256, true) ||
		invalidPublicTaskString(payload.SubtitleFormat, 128, true) ||
		invalidPublicTaskString(payload.TranscodePresetID, 256, true) {
		return errors.New("invalid task field")
	}
	parsedURL, err := url.Parse(strings.TrimSpace(payload.URL))
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") ||
		strings.TrimSpace(parsedURL.Hostname()) == "" || parsedURL.User != nil {
		return errors.New("invalid task url")
	}
	// This catches unsafe literals and special-use/metadata names synchronously.
	// DNS is intentionally resolved and pinned again at the download transport
	// boundary, where redirects and rebinding can actually be contained.
	if _, err := networkpolicy.ValidatePublicHTTPURL(parsedURL.String()); err != nil {
		return errors.New("task url destination is not public")
	}
	mode := strings.ToLower(strings.TrimSpace(payload.Mode))
	if mode != "" && mode != "quick" && mode != "custom" {
		return errors.New("invalid task mode")
	}
	quality := strings.ToLower(strings.TrimSpace(payload.Quality))
	if quality != "" && quality != "best" && quality != "bitrate" && quality != "audio" {
		return errors.New("invalid task quality")
	}
	if len(payload.SubtitleLangs) > 32 {
		return errors.New("too many subtitle languages")
	}
	for _, language := range payload.SubtitleLangs {
		if invalidPublicTaskString(language, 64, false) {
			return errors.New("invalid subtitle language")
		}
	}
	return nil
}

func invalidPublicTaskString(value string, limit int, allowEmpty bool) bool {
	trimmed := strings.TrimSpace(value)
	if (!allowEmpty && trimmed == "") || !utf8.ValidString(value) || utf8.RuneCountInString(value) > limit {
		return true
	}
	return strings.ContainsRune(value, '\x00')
}

func publicTaskFromListItem(item dto.OperationListItemDTO) publicTask {
	return publicTask{
		ID: item.OperationID, Name: safePublicTaskName(item.Name, item.Kind), Kind: item.Kind, Status: item.Status,
		Domain: item.Domain, Platform: item.Platform, Uploader: item.Uploader, PublishTime: item.PublishTime,
		Progress: publicProgress(item.Progress), Metrics: item.Metrics, ErrorCode: item.ErrorCode,
		CreatedAt: item.CreatedAt, StartedAt: item.StartedAt, FinishedAt: item.FinishedAt,
	}
}

func publicTaskFromOperation(item dto.LibraryOperationDTO) publicTask {
	return publicTask{
		ID: item.ID, Name: safePublicTaskName(item.DisplayName, item.Kind), Kind: item.Kind, Status: item.Status,
		Domain: item.SourceDomain, Platform: item.Meta.Platform, Uploader: item.Meta.Uploader, PublishTime: item.Meta.PublishTime,
		Progress: publicProgress(item.Progress), Metrics: item.Metrics, ErrorCode: item.ErrorCode,
		CreatedAt: item.CreatedAt, StartedAt: item.StartedAt, FinishedAt: item.FinishedAt,
	}
}

// safePublicTaskName is the single projection boundary for task names returned
// to remote clients. Desktop operations created without a title historically
// used their complete request URL as DisplayName; older databases can therefore
// contain signed URLs, local paths or opaque credentials in this field.
func safePublicTaskName(value, kind string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || !utf8.ValidString(trimmed) || sensitiveTaskName(trimmed) {
		return publicTaskFallbackName(kind)
	}
	for _, character := range trimmed {
		if unicode.IsControl(character) {
			return publicTaskFallbackName(kind)
		}
	}
	const maxRunes = 160
	runes := []rune(trimmed)
	if len(runes) > maxRunes {
		return string(runes[:maxRunes-1]) + "…"
	}
	return trimmed
}

func sensitiveTaskName(value string) bool {
	lower := strings.ToLower(value)
	if parsed, err := url.Parse(value); err == nil && parsed.IsAbs() && parsed.Host != "" {
		return true
	}
	if strings.Contains(lower, "://") || strings.HasPrefix(lower, "file:") ||
		strings.HasPrefix(value, "/") || strings.HasPrefix(value, `\\`) ||
		strings.HasPrefix(value, "~/") || looksLikeWindowsAbsolutePath(value) || looksLikeRelativeLocalPath(value) {
		return true
	}
	for _, marker := range []string{
		"token=", "access_token=", "id_token=", "api_key=", "apikey=", "key=", "secret=",
		"signature=", "sig=", "credential=", "authorization=", "x-amz-credential=", "x-amz-signature=",
	} {
		if containsSensitiveAssignment(lower, marker) {
			return true
		}
	}
	parts := strings.Split(value, ".")
	if len(parts) == 3 && len(parts[0]) >= 8 && len(parts[1]) >= 8 && len(parts[2]) >= 8 &&
		isTokenAlphabet(parts[0]) && isTokenAlphabet(parts[1]) && isTokenAlphabet(parts[2]) {
		return true
	}
	if len(value) >= 32 && isHexString(value) {
		return true
	}
	if len(value) >= 48 && !strings.ContainsAny(value, " \t\r\n") && isOpaqueToken(value) {
		return true
	}
	return false
}

func looksLikeRelativeLocalPath(value string) bool {
	normalized := strings.ReplaceAll(strings.TrimSpace(value), `\`, "/")
	if strings.HasPrefix(normalized, "./") || strings.HasPrefix(normalized, "../") {
		return true
	}
	parts := strings.Split(normalized, "/")
	if len(parts) < 2 {
		return false
	}
	for _, directory := range []string{
		"users", "home", "private", "var", "tmp", "downloads", "documents", "desktop", "appdata", "library",
	} {
		if strings.EqualFold(strings.TrimSpace(parts[0]), directory) {
			return true
		}
	}
	last := strings.TrimSpace(parts[len(parts)-1])
	hasFileExtension := strings.LastIndex(last, ".") > 0 && !strings.HasSuffix(last, ".")
	return (len(parts) >= 3 && hasFileExtension) || (strings.Contains(value, `\`) && hasFileExtension)
}

func containsSensitiveAssignment(value, marker string) bool {
	for offset := 0; offset < len(value); {
		index := strings.Index(value[offset:], marker)
		if index < 0 {
			return false
		}
		index += offset
		if index == 0 || !taskIdentifierByte(value[index-1]) {
			return true
		}
		offset = index + 1
	}
	return false
}

func taskIdentifierByte(value byte) bool {
	return (value >= 'a' && value <= 'z') || (value >= 'A' && value <= 'Z') ||
		(value >= '0' && value <= '9') || value == '_'
}

func looksLikeWindowsAbsolutePath(value string) bool {
	return len(value) >= 3 && ((value[0] >= 'A' && value[0] <= 'Z') || (value[0] >= 'a' && value[0] <= 'z')) &&
		value[1] == ':' && (value[2] == '\\' || value[2] == '/')
}

func isTokenAlphabet(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') || character == '-' || character == '_' || character == '=' {
			continue
		}
		return false
	}
	return true
}

func isHexString(value string) bool {
	for _, character := range value {
		if (character >= '0' && character <= '9') || (character >= 'a' && character <= 'f') ||
			(character >= 'A' && character <= 'F') {
			continue
		}
		return false
	}
	return true
}

func isOpaqueToken(value string) bool {
	hasLetter, hasDigit, hasPunctuation := false, false, false
	for _, character := range value {
		switch {
		case (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z'):
			hasLetter = true
		case character >= '0' && character <= '9':
			hasDigit = true
		case strings.ContainsRune("-_=+/", character):
			hasPunctuation = true
		default:
			return false
		}
	}
	return hasLetter && hasDigit && hasPunctuation
}

func publicTaskFallbackName(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "download":
		return "Download"
	case "transcode":
		return "Transcode"
	case "import":
		return "Import"
	case "delete":
		return "Library task"
	default:
		return "Task"
	}
}

func publicProgress(progress *dto.OperationProgressDTO) *publicTaskProgress {
	if progress == nil {
		return nil
	}
	return &publicTaskProgress{
		Stage: progress.Stage, Percent: progress.Percent, Current: progress.Current, Total: progress.Total,
		Speed: progress.Speed, SpeedMetric: progress.SpeedMetric, UpdatedAt: progress.UpdatedAt,
	}
}

func writeTaskError(w http.ResponseWriter, err error, control bool) {
	switch {
	case errors.Is(err, sql.ErrNoRows):
		writeError(w, http.StatusNotFound, "not_found")
	case apperrors.CodeOf(err) != "", errors.Is(err, library.ErrInvalidLibraryOperation):
		writeError(w, http.StatusBadRequest, "invalid_request")
	case control:
		// LibraryService intentionally owns the task state machine. Its detailed
		// errors can contain implementation state, so the public response is opaque.
		writeError(w, http.StatusConflict, "task_state_conflict")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error")
	}
}
