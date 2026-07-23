package browsercdp

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	targetpkg "github.com/chromedp/cdproto/target"
)

func TestNormalizeBorrowedCookieAllowlistBuildsExplicitHTTPSURLs(t *testing.T) {
	domains, urls, err := normalizeBorrowedCookieAllowlist([]string{
		" .YouTube.com. ",
		"google.com",
		"youtube.com",
		"www.bilibili.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	wantDomains := []string{"google.com", "www.bilibili.com", "youtube.com"}
	wantURLs := []string{
		"https://google.com/",
		"https://www.google.com/",
		"https://www.bilibili.com/",
		"https://youtube.com/",
		"https://www.youtube.com/",
	}
	if !reflect.DeepEqual(domains, wantDomains) || !reflect.DeepEqual(urls, wantURLs) {
		t.Fatalf("allowlist = %#v / %#v, want %#v / %#v", domains, urls, wantDomains, wantURLs)
	}
}

func TestNormalizeBorrowedCookieAllowlistRejectsUnsafeDomains(t *testing.T) {
	for _, domains := range [][]string{
		nil,
		{"	"},
		{"localhost"},
		{"127.0.0.1"},
		{"https://youtube.com"},
		{"*.youtube.com"},
		{"youtube.com:443"},
		{"youtube.com/path"},
		{"-youtube.com"},
		{"youtube..com"},
	} {
		if _, _, err := normalizeBorrowedCookieAllowlist(domains); !errors.Is(err, ErrBorrowedCookieAllowlistInvalid) {
			t.Fatalf("domains %#v error = %v, want ErrBorrowedCookieAllowlistInvalid", domains, err)
		}
	}
}

func TestBorrowedCookiePageTargetIDsStayInApprovedContext(t *testing.T) {
	const approvedContext = cdp.BrowserContextID("approved-profile")
	runtimeBrowser := &Runtime{
		ownership:          RuntimeOwnershipBorrowed,
		borrowedContextID:  approvedContext,
		borrowedContextSet: true,
		targetManager: &PageTargetManager{targets: map[string]*targetpkg.Info{
			"approved-b": {TargetID: "approved-b", Type: "page", BrowserContextID: approvedContext},
			"approved-a": {TargetID: "approved-a", Type: "page", BrowserContextID: approvedContext},
			"other":      {TargetID: "other", Type: "page", BrowserContextID: "other-profile"},
			"worker":     {TargetID: "worker", Type: "service_worker", BrowserContextID: approvedContext},
		}},
	}
	if got, want := borrowedCookiePageTargetIDs(runtimeBrowser), []string{"approved-a", "approved-b"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("borrowed target IDs = %#v, want %#v", got, want)
	}
}

func TestReadBorrowedCookiesUsesNetworkURLFilterAndLocalDomainBoundary(t *testing.T) {
	executor := &borrowedCookieExecutorStub{cookies: []*network.Cookie{
		{Name: "allowed", Value: "allowed-secret", Domain: ".youtube.com", Path: "/"},
		{Name: "blocked", Value: "blocked-secret", Domain: ".example.com", Path: "/"},
	}}
	tabCtx := context.Background()
	urls := []string{"https://youtube.com/", "https://www.youtube.com/"}
	records, err := readBorrowedCookiesForURLs(context.Background(), tabCtx, executor, urls, []string{"youtube.com"})
	if err != nil {
		t.Fatal(err)
	}
	if executor.method != network.CommandGetCookies {
		t.Fatalf("CDP method = %q, want %q", executor.method, network.CommandGetCookies)
	}
	if !reflect.DeepEqual(executor.urls, urls) {
		t.Fatalf("Network.getCookies URLs = %#v, want %#v", executor.urls, urls)
	}
	if len(records) != 1 || records[0].Name != "allowed" || records[0].Domain != ".youtube.com" {
		t.Fatalf("filtered records = %#v", records)
	}
}

func TestReadBorrowedCookiesRejectsContextWithoutAttachedTargetExecutor(t *testing.T) {
	_, err := readBorrowedCookiesForURLs(
		context.Background(),
		context.Background(),
		nil,
		[]string{"https://youtube.com/"},
		[]string{"youtube.com"},
	)
	if !errors.Is(err, ErrBorrowedCookieTargetUnavailable) {
		t.Fatalf("missing target executor error = %v, want ErrBorrowedCookieTargetUnavailable", err)
	}
}

type borrowedCookieExecutorStub struct {
	method  string
	urls    []string
	cookies []*network.Cookie
}

func (stub *borrowedCookieExecutorStub) Execute(_ context.Context, method string, params any, result any) error {
	stub.method = method
	request, ok := params.(*network.GetCookiesParams)
	if !ok {
		return errors.New("unexpected CDP parameters")
	}
	stub.urls = append([]string(nil), request.URLs...)
	response, ok := result.(*network.GetCookiesReturns)
	if !ok {
		return errors.New("unexpected CDP result")
	}
	response.Cookies = append([]*network.Cookie(nil), stub.cookies...)
	return nil
}
