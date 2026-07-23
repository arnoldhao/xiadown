package wails

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	listenEmbeddedVideoFullscreenResultType = "embedded-video-fullscreen-result"
	listenEmbeddedVideoFullscreenChangeType = "embedded-video-fullscreen-change"
	listenEmbeddedVideoFullscreenTimeout    = 4 * time.Second
)

func prepareListenNativeFullscreenWindow(
	window *application.WebviewWindow,
	host *application.WebviewWindow,
	width int,
	height int,
) {
	if window == nil {
		return
	}
	window.SetSize(width, height)
	if host == nil {
		return
	}
	bounds := host.Bounds()
	window.SetPosition(
		bounds.X+(bounds.Width-width)/2,
		bounds.Y+(bounds.Height-height)/2,
	)
}

func listenNativeWindowFullscreenPreparationDelay() time.Duration {
	// AppKit needs one run-loop turn after the WKWebView is restored to its
	// owning NSWindow. Windows' reparent/style restoration is synchronous and a
	// delay only creates a pre-fullscreen Escape race.
	if runtime.GOOS == "darwin" {
		return 250 * time.Millisecond
	}
	return 0
}

type listenEmbeddedVideoFullscreenResult struct {
	succeeded bool
	message   string
}

type listenEmbeddedVideoFullscreenRequests struct {
	mu      sync.Mutex
	nextID  uint64
	waiters map[uint64]chan listenEmbeddedVideoFullscreenResult
}

func (requests *listenEmbeddedVideoFullscreenRequests) register() (uint64, <-chan listenEmbeddedVideoFullscreenResult, func()) {
	requests.mu.Lock()
	requests.nextID++
	requestID := requests.nextID
	if requests.waiters == nil {
		requests.waiters = make(map[uint64]chan listenEmbeddedVideoFullscreenResult)
	}
	waiter := make(chan listenEmbeddedVideoFullscreenResult, 1)
	requests.waiters[requestID] = waiter
	requests.mu.Unlock()
	return requestID, waiter, func() {
		requests.mu.Lock()
		delete(requests.waiters, requestID)
		requests.mu.Unlock()
	}
}

func (requests *listenEmbeddedVideoFullscreenRequests) complete(requestID uint64, result listenEmbeddedVideoFullscreenResult) bool {
	if requests == nil || requestID == 0 {
		return false
	}
	requests.mu.Lock()
	waiter, ok := requests.waiters[requestID]
	if ok {
		delete(requests.waiters, requestID)
	}
	requests.mu.Unlock()
	if !ok {
		return false
	}
	select {
	case waiter <- result:
	default:
	}
	return true
}

func requestListenEmbeddedVideoFullscreen(
	window *application.WebviewWindow,
	source string,
	requests *listenEmbeddedVideoFullscreenRequests,
	enter bool,
) error {
	return requestListenEmbeddedVideoFullscreenForSession(window, source, "", requests, enter)
}

// requestListenEmbeddedVideoFullscreenForSession keeps the generic native
// fullscreen implementation while binding its asynchronous result/change
// messages to the React playback session that issued the command.
func requestListenEmbeddedVideoFullscreenForSession(
	window *application.WebviewWindow,
	source string,
	sessionID string,
	requests *listenEmbeddedVideoFullscreenRequests,
	enter bool,
) error {
	if window == nil || requests == nil {
		return fmt.Errorf("embedded video fullscreen unavailable")
	}
	requestID, waiter, unregister := requests.register()
	defer unregister()
	execListenYouTubeMusicJS(
		window,
		listenEmbeddedVideoFullscreenScriptForSession(source, sessionID, requestID, enter),
	)
	select {
	case result := <-waiter:
		if result.succeeded {
			return nil
		}
		if listenEmbeddedVideoNativeFullscreenMatches(window, enter) {
			return nil
		}
		message := strings.TrimSpace(result.message)
		if message == "" {
			message = "embedded video rejected fullscreen"
		}
		return fmt.Errorf("%s", message)
	case <-time.After(listenEmbeddedVideoFullscreenTimeout):
		if listenEmbeddedVideoNativeFullscreenMatches(window, enter) {
			return nil
		}
		return fmt.Errorf("embedded video fullscreen request timed out")
	}
}

func listenEmbeddedVideoNativeFullscreenMatches(window *application.WebviewWindow, enter bool) bool {
	if window == nil {
		return false
	}
	active, known := listenNativeEmbeddedVideoFullscreenOwnsPresentation(window.NativeWindow())
	return known && active == enter
}

func listenEmbeddedVideoFullscreenScript(source string, requestID uint64, enter bool) string {
	return listenEmbeddedVideoFullscreenScriptForSession(source, "", requestID, enter)
}

func listenEmbeddedVideoFullscreenScriptForSession(source string, sessionID string, requestID uint64, enter bool) string {
	action := "exit"
	if enter {
		action = "enter"
	}
	return fmt.Sprintf(`
(function() {
  "use strict";
  const SOURCE = %s;
  const SESSION_ID = %s;
  const REQUEST_ID = %d;
  const ACTION = %s;
  const CHANGE_TYPE = %s;
  const WANT_FULLSCREEN = ACTION === "enter";
  const errors = [];
  let finished = false;
  let pollTimer = 0;
  let timeoutTimer = 0;

  function post(payload) {
    const message = JSON.stringify(Object.assign({
      source: SOURCE,
      sessionId: SESSION_ID,
      mainFrame: window.top === window
    }, payload));
    try {
      if (window._wails && typeof window._wails.invoke === "function") {
        window._wails.invoke(message);
        return;
      }
      if (window.webkit && window.webkit.messageHandlers && window.webkit.messageHandlers.external) {
        window.webkit.messageHandlers.external.postMessage(message);
        return;
      }
      if (window.chrome && window.chrome.webview && typeof window.chrome.webview.postMessage === "function") {
        window.chrome.webview.postMessage(message);
        return;
      }
      if (window.wails && typeof window.wails.invoke === "function") {
        window.wails.invoke(message);
      }
    } catch (error) {}
  }

  function videoElement() {
    const videos = Array.from(document.querySelectorAll("video"));
    return videos.find((video) => !video.paused && !video.ended) ||
      videos.find((video) => video.readyState > 0) ||
      videos[0] ||
      null;
  }

  const video = videoElement();
  const player = document.querySelector("#movie_player, .html5-video-player, ytmusic-player");
  const fullscreenButton = document.querySelector(
    "#movie_player .ytp-fullscreen-button, .html5-video-player .ytp-fullscreen-button, button.ytp-fullscreen-button"
  );
  const isYouTubeStationWatchPage =
    SOURCE === "listen-youtube-live-player" &&
    location.hostname === "www.youtube.com" &&
    location.pathname === "/watch";
  const isRSSBilibiliDocument =
    SOURCE === "rss-bilibili-video-player" &&
    location.hostname === "www.bilibili.com";
  const isRSSBilibiliVideoPage =
    isRSSBilibiliDocument &&
    /^\/video\/(?:BV[0-9A-Za-z]+|av[0-9]+)(?:\/|$)/i.test(location.pathname);
  const isRSSBilibiliBangumiPage =
    isRSSBilibiliDocument &&
    /^\/bangumi\/play\/(?:ep|ss)[1-9][0-9]*(?:\/|$)/i.test(location.pathname);
  const isRSSBilibiliPage = isRSSBilibiliVideoPage || isRSSBilibiliBangumiPage;

  function fullscreenActive() {
    return Boolean(
      document.fullscreenElement ||
      document.webkitFullscreenElement ||
      (video && video.webkitDisplayingFullscreen)
    );
  }

  function installFullscreenMonitor() {
    const key = "__xiadownEmbeddedVideoFullscreenMonitor";
    const previous = window[key];
    if (previous && previous.video === video && typeof previous.expect === "function") {
      return previous;
    }
    if (previous && typeof previous.dispose === "function") {
      try { previous.dispose(); } catch (error) {}
    }
    let lastActive = fullscreenActive();
    let authoritativeFullscreenActive = lastActive;
    let exitExpected = false;
    let ignoreSnapshotExitUntil = 0;
    let monitorTimer = 0;
    const publish = (active, reason) => {
      const nextActive = active === true;
      const now = Date.now();
      const explicitExit = reason === "webkitendfullscreen" || exitExpected ||
        reason === "fullscreenchange" || reason === "webkitfullscreenchange";
      // WebKit can dispatch webkitbeginfullscreen before
      // webkitDisplayingFullscreen flips to true. Captions make that hand-off
      // slower on some videos. A resolved request or any positive event owns
      // presentation until an explicit exit; a poll is never allowed to undo
      // that ownership from a stale WebKit snapshot.
      if (nextActive) {
        authoritativeFullscreenActive = true;
        exitExpected = false;
        ignoreSnapshotExitUntil = Math.max(
          ignoreSnapshotExitUntil,
          now + (reason === "webkitbeginfullscreen" ? 900 : 300)
        );
      } else {
        if (!explicitExit && authoritativeFullscreenActive) return;
        if (reason !== "webkitendfullscreen" && !exitExpected && now < ignoreSnapshotExitUntil) {
          return;
        }
        if (explicitExit) authoritativeFullscreenActive = false;
      }
      if (nextActive === lastActive) return;
      lastActive = nextActive;
      post({
        type: CHANGE_TYPE,
        active: nextActive,
        reason: reason || ""
      });
    };
    const reconcile = (reason) => publish(fullscreenActive(), reason);
    const documentChange = () => reconcile("fullscreenchange");
    const webkitDocumentChange = () => reconcile("webkitfullscreenchange");
    const webkitBegin = () => publish(true, "webkitbeginfullscreen");
    const webkitEnd = () => publish(false, "webkitendfullscreen");
    document.addEventListener("fullscreenchange", documentChange, true);
    document.addEventListener("webkitfullscreenchange", webkitDocumentChange, true);
    video?.addEventListener("webkitbeginfullscreen", webkitBegin, true);
    video?.addEventListener("webkitendfullscreen", webkitEnd, true);
    monitorTimer = window.setInterval(() => {
      if (!authoritativeFullscreenActive && (lastActive || Date.now() < ignoreSnapshotExitUntil)) {
        reconcile("poll");
      }
    }, 200);
    const monitor = {
      video,
      expect: (enter) => {
        if (enter === true) {
          exitExpected = false;
          ignoreSnapshotExitUntil = Math.max(ignoreSnapshotExitUntil, Date.now() + 2200);
        } else {
          exitExpected = true;
          authoritativeFullscreenActive = false;
          ignoreSnapshotExitUntil = 0;
        }
      },
      reportKnown: publish,
      dispose: () => {
        if (monitorTimer) window.clearInterval(monitorTimer);
        document.removeEventListener("fullscreenchange", documentChange, true);
        document.removeEventListener("webkitfullscreenchange", webkitDocumentChange, true);
        video?.removeEventListener("webkitbeginfullscreen", webkitBegin, true);
        video?.removeEventListener("webkitendfullscreen", webkitEnd, true);
      }
    };
    window[key] = monitor;
    return monitor;
  }

  const fullscreenMonitor = installFullscreenMonitor();
  fullscreenMonitor?.expect(WANT_FULLSCREEN);

  function cleanup() {
    if (pollTimer) window.clearInterval(pollTimer);
    if (timeoutTimer) window.clearTimeout(timeoutTimer);
    document.removeEventListener("fullscreenchange", checkSnapshot, true);
    document.removeEventListener("webkitfullscreenchange", checkSnapshot, true);
    video?.removeEventListener("webkitbeginfullscreen", checkBegin, true);
    video?.removeEventListener("webkitendfullscreen", checkEnd, true);
  }

  function finish(succeeded, message) {
    if (finished) return;
    finished = true;
    cleanup();
    post({
      type: %s,
      requestId: String(REQUEST_ID),
      succeeded: succeeded === true,
      message: message || ""
    });
  }

  function check(activeOverride) {
    const active = typeof activeOverride === "boolean" ? activeOverride : fullscreenActive();
    if ((WANT_FULLSCREEN && active) || (!WANT_FULLSCREEN && !active)) {
      finish(true, "");
    }
  }

  function checkSnapshot() {
    check();
  }

  function checkBegin() {
    fullscreenMonitor?.reportKnown(true, "webkitbeginfullscreen");
    check(true);
  }

  function checkEnd() {
    fullscreenMonitor?.reportKnown(false, "webkitendfullscreen");
    check(false);
  }

  function record(error) {
    const message = error && error.message ? error.message : String(error || "");
    if (message && !errors.includes(message)) errors.push(message);
  }

  document.addEventListener("fullscreenchange", checkSnapshot, true);
  document.addEventListener("webkitfullscreenchange", checkSnapshot, true);
  video?.addEventListener("webkitbeginfullscreen", checkBegin, true);
  video?.addEventListener("webkitendfullscreen", checkEnd, true);

  if (!WANT_FULLSCREEN && !fullscreenActive()) {
    finish(true, "");
    return;
  }

  let attempted = false;

  function clickFullscreenButton() {
    if (!fullscreenButton || fullscreenActive() === WANT_FULLSCREEN) return false;
    attempted = true;
    try {
      fullscreenButton.click();
      return true;
    } catch (error) {
      record(error);
      return false;
    }
  }

  function fallbackEnterToButton(error) {
    record(error);
    if (finished || fullscreenActive()) return;
    if (!clickFullscreenButton()) {
      fallbackToNativeVideo();
    }
  }

  function fallbackToNativeVideo(error) {
    record(error);
    if (finished || fullscreenActive()) return false;
    try {
      if (video && typeof video.webkitEnterFullscreen === "function") {
        attempted = true;
        video.webkitEnterFullscreen();
        return true;
      }
    } catch (nativeError) {
      record(nativeError);
    }
    return false;
  }

  function fallbackExitToButton(error) {
    record(error);
    if (finished || !fullscreenActive()) return;
    const fullscreenTarget = document.fullscreenElement || document.webkitFullscreenElement;
    if (isYouTubeStationWatchPage && fullscreenTarget === document.documentElement) {
      // The watch-page player does not own XiaDown's document-root fullscreen
      // state. Its button would try to enter player fullscreen instead of
      // recovering a failed document exit.
      return;
    }
    if (clickFullscreenButton()) return;
    try {
      if (video && typeof video.webkitExitFullscreen === "function") {
        attempted = true;
        video.webkitExitFullscreen();
      }
    } catch (nativeError) {
      record(nativeError);
    }
  }

  if (WANT_FULLSCREEN) {
    let elementRequested = false;
    try {
      // YouTube's watch-page player owns an internal fullscreen state machine.
      // Requesting element fullscreen on #movie_player succeeds and is then
      // immediately undone by the page. Fullscreen the stable document root
      // for the dedicated YouTube Station watch page instead; video-mode CSS
      // already limits that page to the player, including captions and overlays.
      // Music Station and the Hush /embed player keep their player target.
      const target = (isYouTubeStationWatchPage || isRSSBilibiliPage)
        ? document.documentElement
        : (player || video || document.documentElement);
      const request = target && (target.requestFullscreen || target.webkitRequestFullscreen);
      if (typeof request === "function") {
        attempted = true;
        const result = request.call(target);
        elementRequested = true;
        if (result && typeof result.then === "function") {
          result
            .then(() => {
              fullscreenMonitor?.reportKnown(true, "requestfullscreen-resolved");
              check(true);
            })
            .catch(fallbackEnterToButton);
        }
      }
    } catch (error) {
      record(error);
    }
    if (!elementRequested && !clickFullscreenButton()) {
      fallbackToNativeVideo();
    }
  } else {
    let nativeRequested = false;
    try {
      const exit = document.exitFullscreen || document.webkitExitFullscreen;
      if (fullscreenActive() && typeof exit === "function" &&
          (document.fullscreenElement || document.webkitFullscreenElement)) {
        attempted = true;
        const result = exit.call(document);
        nativeRequested = true;
        if (result && typeof result.catch === "function") result.catch(fallbackExitToButton);
      }
    } catch (error) {
      record(error);
    }
    if (!nativeRequested) {
      try {
        if (video && typeof video.webkitExitFullscreen === "function") {
          attempted = true;
          video.webkitExitFullscreen();
          nativeRequested = true;
        }
      } catch (error) {
        record(error);
      }
    }
    if (!nativeRequested) clickFullscreenButton();
  }

  check();
  if (finished) return;
  if (!attempted) {
    finish(false, "This video does not expose a fullscreen control.");
    return;
  }
  pollTimer = window.setInterval(check, 50);
  timeoutTimer = window.setTimeout(() => {
    check();
    if (!finished) {
      finish(false, errors.join("; ") || "The video did not enter fullscreen. Try again from the player controls.");
    }
  }, 3000);
})();
`, strconv.Quote(source), strconv.Quote(sessionID), requestID, strconv.Quote(action), strconv.Quote(listenEmbeddedVideoFullscreenChangeType), strconv.Quote(listenEmbeddedVideoFullscreenResultType))
}
