package wails

import (
	"os"
	"strings"
	"testing"
)

func TestConnectorWindowsAsyncCompletionRootsFollowCOMReferenceLifetime(t *testing.T) {
	t.Parallel()

	sourceBytes, err := os.ReadFile("connector_app_session_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(sourceBytes)

	for _, test := range []struct {
		signature string
		required  []string
	}{
		{
			signature: "func readConnectorWindowsWebViewCookiesForURI(",
			required:  []string{"defer connectorWindowsGetCookiesCompletedHandlerRelease(handler)"},
		},
		{
			signature: "func connectorWindowsCallDevToolsProtocolMethod(",
			required:  []string{"defer connectorWindowsDevToolsCompletedHandlerRelease(handler)"},
		},
		{
			signature: "func connectorWindowsDevToolsCompletedHandlerRelease(",
			required:  []string{"remaining == 0", "connectorWindowsUntrackDevToolsHandler(this)"},
		},
		{
			signature: "func connectorWindowsGetCookiesCompletedHandlerRelease(",
			required:  []string{"remaining == 0", "connectorWindowsUntrackGetCookiesHandler(this)"},
		},
		{
			signature: "func connectorWindowsDevToolsCompletedHandlerInvoke(",
			required:  []string{"this.once.Do(func()", "close(this.done)"},
		},
		{
			signature: "func connectorWindowsGetCookiesCompletedHandlerInvoke(",
			required:  []string{"this.once.Do(func()", "close(this.done)"},
		},
	} {
		body := rssVideoFunctionSource(t, source, test.signature)
		for _, required := range test.required {
			if !strings.Contains(body, required) {
				t.Fatalf("%s is missing %q", test.signature, required)
			}
		}
	}

	for _, signature := range []string{
		"func connectorWindowsCallDevTools(",
		"func connectorWindowsGetCookies(",
	} {
		body := rssVideoFunctionSource(t, source, signature)
		if strings.Contains(body, "connectorWindowsUntrack") {
			t.Fatalf("%s unroots a handler before native Release", signature)
		}
	}
}
