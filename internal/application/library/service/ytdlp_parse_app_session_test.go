package service

import (
	"context"
	"errors"
	"testing"

	appsessionsdto "xiadown/internal/application/appsessions/dto"
	appsessionsservice "xiadown/internal/application/appsessions/service"
	"xiadown/internal/application/library/dto"
)

type failingYTDLPParseAppSessionReader struct {
	err error
}

func (stub failingYTDLPParseAppSessionReader) ListAppSessions(context.Context) ([]appsessionsdto.AppSession, error) {
	return nil, nil
}

func (stub failingYTDLPParseAppSessionReader) ExportAppSessionCookies(
	context.Context,
	string,
	appsessionsservice.CookiesExportFormat,
) (string, error) {
	return "", stub.err
}

func TestParseYTDLPDownloadDoesNotSilentlyIgnoreExplicitAppSessionExportFailure(t *testing.T) {
	want := errors.New("App Session credentials expired")
	service := &LibraryService{
		appSessions: failingYTDLPParseAppSessionReader{err: want},
	}
	_, err := service.ParseYTDLPDownload(context.Background(), dto.ParseYTDLPDownloadRequest{
		URL:           "https://www.bilibili.com/video/BV1xx411c7mD",
		UseAppSession: true,
		AppSessionID:  "site-app-session-bilibili",
	})
	if !errors.Is(err, want) {
		t.Fatalf("ParseYTDLPDownload error = %v, want %v", err, want)
	}
}
