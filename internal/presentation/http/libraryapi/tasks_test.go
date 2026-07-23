package libraryapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"xiadown/internal/application/apperrors"
	"xiadown/internal/application/library/access"
	"xiadown/internal/application/library/dto"
	"xiadown/internal/domain/library"
)

type taskServiceStub struct {
	listResult   []dto.OperationListItemDTO
	listErr      error
	getResult    dto.LibraryOperationDTO
	getErr       error
	createResult dto.LibraryOperationDTO
	createErr    error
	cancelResult dto.LibraryOperationDTO
	cancelErr    error
	resumeResult dto.LibraryOperationDTO
	resumeErr    error

	listRequest   dto.ListOperationsRequest
	getRequest    dto.GetOperationRequest
	createRequest dto.CreateYTDLPJobRequest
	cancelRequest dto.CancelOperationRequest
	resumeRequest dto.ResumeOperationRequest
	listCalls     int
	getCalls      int
	createCalls   int
	cancelCalls   int
	resumeCalls   int
}

func TestTaskSearchQueryMatchesOpenAPIUnicodeLengthContract(t *testing.T) {
	stub := &taskServiceStub{}
	api, err := NewTaskAPI(stub)
	if err != nil {
		t.Fatal(err)
	}
	validQuery := strings.Repeat("界", maxPublicSearchQueryLength)

	validRecorder := httptest.NewRecorder()
	api.list(validRecorder, httptest.NewRequest(http.MethodGet, "/api/v1/tasks?q="+url.QueryEscape(validQuery), nil))
	if validRecorder.Code != http.StatusOK || stub.listCalls != 1 || stub.listRequest.Query != validQuery {
		t.Fatalf("valid Unicode query status=%d calls=%d query=%q body=%s", validRecorder.Code, stub.listCalls, stub.listRequest.Query, validRecorder.Body.String())
	}

	for name, target := range map[string]string{
		"too many code points": "/api/v1/tasks?q=" + url.QueryEscape(validQuery+"界"),
		"invalid UTF-8":        "/api/v1/tasks?q=%FF",
	} {
		t.Run(name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			api.list(recorder, httptest.NewRequest(http.MethodGet, target, nil))
			if recorder.Code != http.StatusBadRequest || stub.listCalls != 1 || !strings.Contains(recorder.Body.String(), `"error":"invalid_request"`) {
				t.Fatalf("status=%d calls=%d body=%s", recorder.Code, stub.listCalls, recorder.Body.String())
			}
		})
	}
}

func (stub *taskServiceStub) GetOperation(_ context.Context, request dto.GetOperationRequest) (dto.LibraryOperationDTO, error) {
	stub.getCalls++
	stub.getRequest = request
	return stub.getResult, stub.getErr
}

func (stub *taskServiceStub) ListOperations(_ context.Context, request dto.ListOperationsRequest) ([]dto.OperationListItemDTO, error) {
	stub.listCalls++
	stub.listRequest = request
	return stub.listResult, stub.listErr
}

func (stub *taskServiceStub) CreateYTDLPJob(_ context.Context, request dto.CreateYTDLPJobRequest) (dto.LibraryOperationDTO, error) {
	stub.createCalls++
	stub.createRequest = request
	return stub.createResult, stub.createErr
}

func (stub *taskServiceStub) CancelOperation(_ context.Context, request dto.CancelOperationRequest) (dto.LibraryOperationDTO, error) {
	stub.cancelCalls++
	stub.cancelRequest = request
	return stub.cancelResult, stub.cancelErr
}

func (stub *taskServiceStub) ResumeOperation(_ context.Context, request dto.ResumeOperationRequest) (dto.LibraryOperationDTO, error) {
	stub.resumeCalls++
	stub.resumeRequest = request
	return stub.resumeResult, stub.resumeErr
}

func TestTaskRoutesUseLeastPrivilegeScopes(t *testing.T) {
	api, err := NewTaskAPI(&taskServiceStub{})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]library.DeviceScope{
		"GET /api/v1/tasks":              library.DeviceScopeTasksRead,
		"GET /api/v1/tasks/{id}":         library.DeviceScopeTasksRead,
		"POST /api/v1/tasks":             library.DeviceScopeTasksCreate,
		"POST /api/v1/tasks/{id}/cancel": library.DeviceScopeTasksControl,
		"POST /api/v1/tasks/{id}/resume": library.DeviceScopeTasksControl,
	}
	for _, route := range api.Routes() {
		key := route.Method + " " + route.Path
		if route.Scope != want[key] {
			t.Fatalf("route %s scope = %q, want %q", key, route.Scope, want[key])
		}
		delete(want, key)
	}
	if len(want) != 0 {
		t.Fatalf("missing task routes: %#v", want)
	}
}

func TestCreateTaskUsesStrictAllowlistAndForcesAuthenticatedDeviceCaller(t *testing.T) {
	secretPath := "/Users/arnold/Private/download.mp4"
	stub := &taskServiceStub{createResult: sensitiveOperationDTO(secretPath)}
	api, err := NewTaskAPI(stub)
	if err != nil {
		t.Fatal(err)
	}
	body := `{"url":"https://example.com/video","title":"Episode","mode":"quick","quality":"best","writeThumbnail":true,"subtitleLangs":["en"],"subtitleAuto":true}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/tasks", strings.NewReader(body))
	request = request.WithContext(context.WithValue(request.Context(), principalContextKey{}, access.Principal{DeviceID: "iphone-15"}))
	recorder := httptest.NewRecorder()
	api.create(recorder, request)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	if stub.createRequest.Caller != "iphone-15" || stub.createRequest.Source != "public_api" {
		t.Fatalf("caller/source not forced from principal: %#v", stub.createRequest)
	}
	if stub.createRequest.LibraryID != "" || stub.createRequest.CookiesPath != "" ||
		stub.createRequest.AppSessionID != "" || stub.createRequest.ResourceSessionID != "" ||
		stub.createRequest.ResourceMediaID != "" || stub.createRequest.RunID != "" || stub.createRequest.RetryOf != "" {
		t.Fatalf("public request populated private execution context: %#v", stub.createRequest)
	}
	assertTaskResponseSanitized(t, recorder.Body.String(), secretPath)
}

func TestCreateTaskRejectsUnknownOrLocalPathFieldsBeforeApplicationService(t *testing.T) {
	for name, body := range map[string]string{
		"input path":    `{"url":"https://example.com/video","inputPath":"/etc/passwd"}`,
		"library id":    `{"url":"https://example.com/video","libraryId":"legacy-library"}`,
		"caller":        `{"url":"https://example.com/video","caller":"spoofed-device"}`,
		"cookies path":  `{"url":"https://example.com/video","cookiesPath":"C:\\\\secret.txt"}`,
		"unix path url": `{"url":"/Users/arnold/Private/video.mp4"}`,
		"file url":      `{"url":"file:///etc/passwd"}`,
		"windows path":  `{"url":"C:\\\\Users\\\\private.mp4"}`,
		"loopback url":  `{"url":"http://127.0.0.1/admin"}`,
		"private url":   `{"url":"http://10.0.0.8/video"}`,
		"metadata url":  `{"url":"http://169.254.169.254/latest/meta-data"}`,
		"metadata name": `{"url":"http://metadata.google.internal/computeMetadata/v1"}`,
		"ipv6 local":    `{"url":"http://[fe80::1]/video"}`,
	} {
		t.Run(name, func(t *testing.T) {
			stub := &taskServiceStub{}
			api, err := NewTaskAPI(stub)
			if err != nil {
				t.Fatal(err)
			}
			request := httptest.NewRequest(http.MethodPost, "/api/v1/tasks", strings.NewReader(body))
			request = request.WithContext(context.WithValue(request.Context(), principalContextKey{}, access.Principal{DeviceID: "windows-1"}))
			recorder := httptest.NewRecorder()
			api.create(recorder, request)
			if recorder.Code != http.StatusBadRequest || stub.createCalls != 0 {
				t.Fatalf("status=%d createCalls=%d body=%s", recorder.Code, stub.createCalls, recorder.Body.String())
			}
		})
	}
}

func TestTaskListAndControlResponsesNeverExposeOperationInternals(t *testing.T) {
	secretPath := `C:\Users\arnold\Private\movie.mkv`
	operation := sensitiveOperationDTO(secretPath)
	stub := &taskServiceStub{
		listResult: []dto.OperationListItemDTO{{
			OperationID: operation.ID, LibraryID: operation.LibraryID, LibraryName: secretPath,
			Name: operation.DisplayName, Kind: operation.Kind, Status: operation.Status,
			Correlation: operation.Correlation, Domain: operation.SourceDomain, SourceIcon: secretPath,
			Platform: operation.Meta.Platform, Uploader: operation.Meta.Uploader, PublishTime: operation.Meta.PublishTime,
			Request: operation.Request, Progress: operation.Progress, OutputFiles: operation.OutputFiles,
			ThumbnailPreviewPath: secretPath, Metrics: operation.Metrics,
			ErrorCode: operation.ErrorCode, ErrorMessage: "process failed at " + secretPath,
			CreatedAt: operation.CreatedAt, StartedAt: operation.StartedAt, FinishedAt: operation.FinishedAt,
		}},
		getResult:    operation,
		cancelResult: operation,
		resumeResult: operation,
	}
	api, err := NewTaskAPI(stub)
	if err != nil {
		t.Fatal(err)
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/tasks?limit=20&offset=2&status=running&kind=download&q=episode", nil)
	listRecorder := httptest.NewRecorder()
	api.list(listRecorder, listRequest)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", listRecorder.Code, listRecorder.Body.String())
	}
	if stub.listRequest.Limit != 20 || stub.listRequest.Offset != 2 || stub.listRequest.Query != "episode" ||
		len(stub.listRequest.Status) != 1 || len(stub.listRequest.Kinds) != 1 {
		t.Fatalf("unexpected application list request: %#v", stub.listRequest)
	}
	assertTaskResponseSanitized(t, listRecorder.Body.String(), secretPath)

	getRequest := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/task-1", nil)
	getRequest.SetPathValue("id", "task-1")
	getRecorder := httptest.NewRecorder()
	api.get(getRecorder, getRequest)
	if getRecorder.Code != http.StatusOK {
		t.Fatalf("get status=%d body=%s", getRecorder.Code, getRecorder.Body.String())
	}
	assertTaskResponseSanitized(t, getRecorder.Body.String(), secretPath)
	if stub.getRequest.OperationID != "task-1" {
		t.Fatalf("get ID not delegated: %#v", stub.getRequest)
	}

	for action, handler := range map[string]func(http.ResponseWriter, *http.Request){
		"cancel": api.cancel,
		"resume": api.resume,
	} {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/task-1/"+action, nil)
		request.SetPathValue("id", "task-1")
		recorder := httptest.NewRecorder()
		handler(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s status=%d body=%s", action, recorder.Code, recorder.Body.String())
		}
		assertTaskResponseSanitized(t, recorder.Body.String(), secretPath)
	}
	if stub.cancelRequest.OperationID != "task-1" || stub.resumeRequest.OperationID != "task-1" {
		t.Fatalf("control IDs not delegated: cancel=%#v resume=%#v", stub.cancelRequest, stub.resumeRequest)
	}
}

func TestTaskRouterEnforcesReadCreateAndControlIndependently(t *testing.T) {
	stub := &taskServiceStub{createResult: sensitiveOperationDTO("/secret"), cancelResult: sensitiveOperationDTO("/secret"), resumeResult: sensitiveOperationDTO("/secret")}
	stub.getResult = sensitiveOperationDTO("/secret")
	taskAPI, err := NewTaskAPI(stub)
	if err != nil {
		t.Fatal(err)
	}
	authenticator := &fakeAuthenticator{principal: access.Principal{GrantID: "grant-1", CatalogID: "catalog-1", DeviceID: "device-1"}}
	router, err := NewRouter(Config{Version: "test", CatalogID: "catalog-1", Authenticator: authenticator, Pairer: &fakePairer{}, Routes: taskAPI.Routes()})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		method string
		path   string
		body   string
		scope  library.DeviceScope
	}{
		{name: "read", method: http.MethodGet, path: "/api/v1/tasks", scope: library.DeviceScopeTasksRead},
		{name: "get", method: http.MethodGet, path: "/api/v1/tasks/task-1", scope: library.DeviceScopeTasksRead},
		{name: "create", method: http.MethodPost, path: "/api/v1/tasks", body: `{"url":"https://example.com/video"}`, scope: library.DeviceScopeTasksCreate},
		{name: "cancel", method: http.MethodPost, path: "/api/v1/tasks/task-1/cancel", scope: library.DeviceScopeTasksControl},
		{name: "resume", method: http.MethodPost, path: "/api/v1/tasks/task-1/resume", scope: library.DeviceScopeTasksControl},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			authenticator.principal.Scopes = []library.DeviceScope{library.DeviceScopeLibraryRead}
			forbidden := performRequest(router, test.method, test.path, "Bearer token", test.body)
			if forbidden.Code != http.StatusForbidden {
				t.Fatalf("wrong scope status=%d body=%s", forbidden.Code, forbidden.Body.String())
			}
			authenticator.principal.Scopes = []library.DeviceScope{test.scope}
			allowed := performRequest(router, test.method, test.path, "Bearer token", test.body)
			if allowed.Code < 200 || allowed.Code >= 300 {
				t.Fatalf("correct scope status=%d body=%s", allowed.Code, allowed.Body.String())
			}
		})
	}
}

func TestPublicTaskNameProjectionPreservesTitlesAndGeneralizesSecretsEverywhere(t *testing.T) {
	for _, ordinary := range []string{"AC/DC — Live at River Plate", "Monkey=Business", "A 64-character title can still be perfectly ordinary and descriptive"} {
		if got := safePublicTaskName(ordinary, "download"); got != ordinary {
			t.Fatalf("ordinary title = %q, want %q", got, ordinary)
		}
	}

	secretNames := []string{
		"https://cdn.example/video.mp4?X-Amz-Signature=secret",
		`C:\\Users\\arnold\\Private\\movie.mkv`,
		"/Users/arnold/Private/movie.mkv",
		"Downloads/private/movie.mkv",
		`..\Private\movie.mkv`,
		"eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJkZXZpY2UxIn0.abcdefghijklmnopqrstuvwxyz012345",
		"0123456789abcdef0123456789abcdef",
	}
	for _, secretName := range secretNames {
		t.Run(secretName[:min(len(secretName), 24)], func(t *testing.T) {
			operation := sensitiveOperationDTO("/not-returned")
			operation.DisplayName = secretName
			stub := &taskServiceStub{
				listResult: []dto.OperationListItemDTO{{OperationID: operation.ID, Name: secretName, Kind: "download", Status: operation.Status}},
				getResult:  operation, createResult: operation,
			}
			api, err := NewTaskAPI(stub)
			if err != nil {
				t.Fatal(err)
			}
			for name, invoke := range map[string]func(*httptest.ResponseRecorder){
				"list": func(recorder *httptest.ResponseRecorder) {
					api.list(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil))
				},
				"get": func(recorder *httptest.ResponseRecorder) {
					request := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/task-1", nil)
					request.SetPathValue("id", "task-1")
					api.get(recorder, request)
				},
				"create": func(recorder *httptest.ResponseRecorder) {
					request := httptest.NewRequest(http.MethodPost, "/api/v1/tasks", strings.NewReader(`{"url":"https://example.com/video"}`))
					request = request.WithContext(context.WithValue(request.Context(), principalContextKey{}, access.Principal{DeviceID: "device-1"}))
					api.create(recorder, request)
				},
			} {
				recorder := httptest.NewRecorder()
				invoke(recorder)
				if recorder.Code < 200 || recorder.Code >= 300 {
					t.Fatalf("%s status=%d body=%s", name, recorder.Code, recorder.Body.String())
				}
				if strings.Contains(recorder.Body.String(), secretName) || !strings.Contains(recorder.Body.String(), `"name":"Download"`) {
					t.Fatalf("%s leaked unsafe task name: %s", name, recorder.Body.String())
				}
			}
		})
	}
}

func TestTaskErrorsAreOpaqueAndUseStableStatuses(t *testing.T) {
	stub := &taskServiceStub{
		createErr: apperrors.New(apperrors.CodeDownloadURLInvalid, "private details /Users/secret"),
		cancelErr: errors.New("process 812 failed at /Users/secret"),
		resumeErr: sql.ErrNoRows,
	}
	api, err := NewTaskAPI(stub)
	if err != nil {
		t.Fatal(err)
	}

	createRequest := httptest.NewRequest(http.MethodPost, "/api/v1/tasks", strings.NewReader(`{"url":"https://example.com/video"}`))
	createRequest = createRequest.WithContext(context.WithValue(createRequest.Context(), principalContextKey{}, access.Principal{DeviceID: "device-1"}))
	createRecorder := httptest.NewRecorder()
	api.create(createRecorder, createRequest)
	if createRecorder.Code != http.StatusBadRequest || strings.Contains(createRecorder.Body.String(), "private") {
		t.Fatalf("create error leaked details: %d %s", createRecorder.Code, createRecorder.Body.String())
	}

	cancelRequest := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/task-1/cancel", nil)
	cancelRequest.SetPathValue("id", "task-1")
	cancelRecorder := httptest.NewRecorder()
	api.cancel(cancelRecorder, cancelRequest)
	if cancelRecorder.Code != http.StatusConflict || strings.Contains(cancelRecorder.Body.String(), "812") {
		t.Fatalf("cancel error leaked details: %d %s", cancelRecorder.Code, cancelRecorder.Body.String())
	}

	resumeRequest := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/missing/resume", nil)
	resumeRequest.SetPathValue("id", "missing")
	resumeRecorder := httptest.NewRecorder()
	api.resume(resumeRecorder, resumeRequest)
	if resumeRecorder.Code != http.StatusNotFound {
		t.Fatalf("resume status=%d body=%s", resumeRecorder.Code, resumeRecorder.Body.String())
	}
}

func sensitiveOperationDTO(secretPath string) dto.LibraryOperationDTO {
	percent := 42
	return dto.LibraryOperationDTO{
		ID: "task-1", LibraryID: "legacy-library", Kind: "download", Status: "running", DisplayName: "Episode",
		Correlation: dto.OperationCorrelationDTO{RequestID: "process-request", RunID: "process-run", ParentOperationID: "parent"},
		InputJSON:   `{"inputPath":"` + secretPath + `"}`, OutputJSON: `{"outputPath":"` + secretPath + `"}`,
		SourceDomain: "example.com", SourceIcon: secretPath,
		Meta:                 dto.OperationMetaDTO{Platform: "youtube", Uploader: "Author", PublishTime: "2026-07-13"},
		Request:              &dto.OperationRequestPreviewDTO{URL: "https://example.com/video", Caller: "internal", InputPath: secretPath},
		Progress:             &dto.OperationProgressDTO{Stage: "downloading", Percent: &percent, Message: "writing " + secretPath, UpdatedAt: "2026-07-13T12:00:00Z"},
		OutputFiles:          []dto.OperationOutputFileDTO{{FileID: "private-file-id", Kind: "video"}},
		ThumbnailPreviewPath: secretPath, Metrics: dto.OperationMetricsDTO{FileCount: 1},
		ErrorCode: "download_failed", ErrorMessage: "process 812 failed at " + secretPath,
		CreatedAt: "2026-07-13T12:00:00Z", StartedAt: "2026-07-13T12:00:01Z",
	}
}

func assertTaskResponseSanitized(t *testing.T, body string, secretPath string) {
	t.Helper()
	escapedSecret, _ := json.Marshal(secretPath)
	for _, forbidden := range []string{
		secretPath, strings.Trim(string(escapedSecret), `"`), "inputJson", "outputJson", "libraryId", "libraryName", "correlation",
		"requestId", "runId", "parentOperationId", "thumbnailPreviewPath", "sourceIcon",
		"inputPath", "outputPath", "outputFiles", "private-file-id", "errorMessage", "process 812", "writing ",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("public task response contains %q: %s", forbidden, body)
		}
	}
	if !strings.Contains(body, `"id":"task-1"`) || !strings.Contains(body, `"status":"running"`) {
		t.Fatalf("public task response omitted safe identity/state: %s", body)
	}
}
