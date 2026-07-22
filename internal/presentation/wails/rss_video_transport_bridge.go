package wails

import (
	"encoding/json"
	"fmt"
	"math"
)

type rssBilibiliBridgeConfig struct {
	SessionID       string  `json:"sessionId"`
	Adapter         string  `json:"adapter"`
	PlatformVideoID string  `json:"platformVideoId"`
	StartSeconds    float64 `json:"startSeconds"`
	Volume          float64 `json:"volume"`
	Muted           bool    `json:"muted"`
	PlaybackRate    float64 `json:"playbackRate"`
	Autoplay        bool    `json:"autoplay"`
}

func normalizeRSSBilibiliBridgeConfig(
	request RSSVideoPlayerPrepareRequest,
) (rssBilibiliBridgeConfig, error) {
	if math.IsNaN(request.StartSeconds) || math.IsInf(request.StartSeconds, 0) {
		return rssBilibiliBridgeConfig{}, fmt.Errorf("RSS Bilibili start position must be finite")
	}
	volume := 1.0
	if request.Volume != nil {
		if math.IsNaN(*request.Volume) || math.IsInf(*request.Volume, 0) {
			return rssBilibiliBridgeConfig{}, fmt.Errorf("RSS Bilibili volume must be finite")
		}
		volume = math.Max(0, math.Min(1, *request.Volume))
	}
	autoplay := true
	if request.Autoplay != nil {
		autoplay = *request.Autoplay
	}
	return rssBilibiliBridgeConfig{
		StartSeconds: math.Max(0, request.StartSeconds),
		Volume:       volume,
		Muted:        request.Muted,
		PlaybackRate: 1,
		Autoplay:     autoplay,
	}, nil
}

func rssBilibiliSimpleCommandScript(sessionID string, command string) string {
	session, _ := json.Marshal(sessionID)
	method, _ := json.Marshal(command)
	return fmt.Sprintf(
		`(function(){const p=window.__xiadownRSSBilibiliPlayer;if(p&&p.sessionId===%s&&typeof p[%s]==="function"){p[%s]();}})();`,
		session,
		method,
		method,
	)
}

func rssBilibiliNumberCommandScript(sessionID string, command string, value float64) string {
	session, _ := json.Marshal(sessionID)
	method, _ := json.Marshal(command)
	return fmt.Sprintf(
		`(function(){const p=window.__xiadownRSSBilibiliPlayer;if(p&&p.sessionId===%s&&typeof p[%s]==="function"){p[%s](%g);}})();`,
		session,
		method,
		method,
		value,
	)
}

func rssBilibiliVolumeCommandScript(sessionID string, volume float64, muted bool) string {
	session, _ := json.Marshal(sessionID)
	return fmt.Sprintf(
		`(function(){const p=window.__xiadownRSSBilibiliPlayer;if(p&&p.sessionId===%s&&typeof p.volume==="function"){p.volume(%g,%t);}})();`,
		session,
		volume,
		muted,
	)
}

func rssBilibiliStringCommandScript(sessionID string, command string, value string) string {
	session, _ := json.Marshal(sessionID)
	method, _ := json.Marshal(command)
	argument, _ := json.Marshal(value)
	return fmt.Sprintf(
		`(function(){const p=window.__xiadownRSSBilibiliPlayer;if(p&&p.sessionId===%s&&typeof p[%s]==="function"){p[%s](%s);}})();`,
		session,
		method,
		method,
		argument,
	)
}

func rssBilibiliBooleanCommandScript(sessionID string, command string, value bool) string {
	session, _ := json.Marshal(sessionID)
	method, _ := json.Marshal(command)
	return fmt.Sprintf(
		`(function(){const p=window.__xiadownRSSBilibiliPlayer;if(p&&p.sessionId===%s&&typeof p[%s]==="function"){p[%s](%t);}})();`,
		session,
		method,
		method,
		value,
	)
}

// rssBilibiliHTMLMediaBridgeScript keeps transport on the standard
// HTMLMediaElement surface. Optional quality/danmaku capabilities are
// feature-detected from the canonical full-page nano port, with real DOM
// controls as fallbacks; XiaDown never creates or replaces Bilibili's player.
func rssBilibiliHTMLMediaBridgeScript(config rssBilibiliBridgeConfig) string {
	payload, _ := json.Marshal(config)
	return fmt.Sprintf(`(function(){
  "use strict";
  // WebView2 installs document-created scripts in child frames as well. The
  // native transport contract is top-level only on every platform.
  if (window.top !== window) return;
  const CONFIG = %s;
  const DOCUMENT_PATH_MATCHES_ADAPTER = CONFIG.adapter === "video"
    ? /^\/video\/(?:BV[0-9A-Za-z]{10}|av[1-9][0-9]*)\/?$/i.test(window.location.pathname)
    : CONFIG.adapter === "bangumi"
      ? /^\/bangumi\/play\/(?:ep[1-9][0-9]*|ss[1-9][0-9]*)\/?$/i.test(window.location.pathname)
      : false;
  if (window.location.origin !== "https://www.bilibili.com" || !DOCUMENT_PATH_MATCHES_ADAPTER) return;
  const SOURCE = "rss-bilibili-video-player";
  const FULLSCREEN_STORAGE_KEY = "__xiadownRSSBilibiliFullscreen:" + CONFIG.sessionId;
  const ACTIVE_VIDEO_ATTRIBUTE = "data-xiadown-rss-bilibili-active-video";
  const CAPTION_OFF_ID = "off";
  const BILIBILI_SUBTITLE_ITEM_SELECTOR = ".bpx-player-ctrl-subtitle-major-content .bpx-player-ctrl-subtitle-language-item[data-lan]";
  const BILIBILI_SUBTITLE_CLOSE_SELECTOR = ".bpx-player-ctrl-subtitle-close-switch";
  const BILIBILI_QUALITY_ITEM_SELECTOR = ".bpx-player-ctrl-quality-menu-item[data-value]";
  const BILIBILI_DANMAKU_INPUT_SELECTOR = ".bpx-player-dm-switch .bui-danmaku-switch-input";
  const QUALITY_LABELS = {
    "127": "8K 超高清", "126": "杜比视界", "125": "HDR 真彩", "120": "4K 超清",
    "116": "1080P 60帧", "112": "1080P 高码率", "80": "1080P 高清",
    "74": "720P 60帧", "64": "720P 高清", "32": "480P 清晰", "16": "360P 流畅", "6": "240P 极速"
  };
  const RATE_OPTIONS = [
    { id: "0.5", label: "0.5×" },
    { id: "0.75", label: "0.75×" },
    { id: "1", label: "1×" },
    { id: "1.25", label: "1.25×" },
    { id: "1.5", label: "1.5×" },
    { id: "2", label: "2×" }
  ];
  const previous = window.__xiadownRSSBilibiliPlayer;
  if (previous && previous.sessionId === CONFIG.sessionId && previous.disposed !== true && typeof previous.reconcile === "function") {
    try { previous.reconcile(); } catch (_) {}
    return;
  }
  if (previous && typeof previous.dispose === "function") {
    try { previous.dispose(); } catch (_) {}
  }

  let media = null;
  let mediaListeners = [];
  let observer = null;
  let documentRootObserver = null;
  let pollTimer = 0;
  let officialPlayerRef = null;
  let officialEventTypesRef = null;
  let officialPlayerListeners = [];
  let playerProbeTimer = 0;
  let playerProbeDeadline = Date.now() + 15000;
  let requestedStartApplied = false;
  let initialAutoplayAttempted = false;
  let disposed = false;
  let identityFailed = false;
  let identityViolationReported = false;
  let removeHistoryIdentityGuard = null;
  let terminalEnded = false;
  let lastEmptySnapshot = "";
  let lastCaptionId = CAPTION_OFF_ID;
  let structuredMetadataNode = null;
  let structuredMetadataText = "";
  let structuredMetadata = { publisher: "", publishedAt: "", viewCount: 0, likeCount: 0 };
  let pageMetadata = { publisher: "", publishedAt: "", viewCount: 0, likeCount: 0 };

  function setFullscreenPresentation(active) {
    const root = document.documentElement;
    if (!root) return;
    if (active) {
      root.setAttribute("data-xiadown-rss-bilibili-fullscreen", "true");
      try { window.sessionStorage.setItem(FULLSCREEN_STORAGE_KEY, "true"); } catch (_) {}
    } else {
      root.removeAttribute("data-xiadown-rss-bilibili-fullscreen");
      try { window.sessionStorage.removeItem(FULLSCREEN_STORAGE_KEY); } catch (_) {}
    }
  }

  function restoreFullscreenPresentation() {
    let active = false;
    try { active = window.sessionStorage.getItem(FULLSCREEN_STORAGE_KEY) === "true"; } catch (_) {}
    if (active) setFullscreenPresentation(true);
  }

  function finite(value, fallback) {
    const number = Number(value);
    return Number.isFinite(number) ? number : fallback;
  }

  function clamp(value, minimum, maximum) {
    return Math.min(maximum, Math.max(minimum, value));
  }

  function normalizedText(value) {
    return String(value == null ? "" : value).replace(/\s+/g, " ").trim();
  }

  function normalizedVideoIdentity(value) {
    const text = normalizedText(value);
    if (/^BV[0-9A-Za-z]{10}$/i.test(text)) {
      return "BV" + text.slice(2);
    }
    if (/^av[1-9][0-9]*$/i.test(text)) {
      return "av" + text.slice(2);
    }
    return "";
  }

  function normalizedBangumiIdentity(value) {
    const text = normalizedText(value);
    if (/^ep[1-9][0-9]*$/i.test(text)) {
      return "ep" + text.slice(2);
    }
    if (/^ss[1-9][0-9]*$/i.test(text)) {
      return "ss" + text.slice(2);
    }
    return "";
  }

  function videoIdentityFromPath(pathname) {
    const match = /^\/video\/(BV[0-9A-Za-z]{10}|av[1-9][0-9]*)\/?$/i.exec(
      normalizedText(pathname),
    );
    return normalizedVideoIdentity(match ? match[1] : "");
  }

  function bangumiIdentityFromPath(pathname) {
    const match = /^\/bangumi\/play\/(ep[1-9][0-9]*|ss[1-9][0-9]*)\/?$/i.exec(
      normalizedText(pathname),
    );
    return normalizedBangumiIdentity(match ? match[1] : "");
  }

  function configuredIdentity(value) {
    if (CONFIG.adapter === "video") return normalizedVideoIdentity(value);
    if (CONFIG.adapter === "bangumi") return normalizedBangumiIdentity(value);
    return "";
  }

  function configuredIdentityFromPath(pathname) {
    if (CONFIG.adapter === "video") return videoIdentityFromPath(pathname);
    if (CONFIG.adapter === "bangumi") return bangumiIdentityFromPath(pathname);
    return "";
  }

  function configuredVideoIdentityState() {
    const expected = configuredIdentity(CONFIG.platformVideoId);
    const actual = configuredIdentityFromPath(window.location.pathname);
    if (!expected || !actual) return "mismatch";
    return expected === actual ? "match" : "mismatch";
  }

  function reportIdentityViolation() {
    if (identityViolationReported || disposed) return;
    identityViolationReported = true;
    // Native owns the authenticated player window. Asking it to tear down the
    // exact session is the only durable response to a SPA identity escape:
    // merely pausing today's media element would not stop Bilibili from
    // creating and autoplaying another one on the next route.
    failConfiguredVideoIdentity();
    // Publish the fail-closed state before native teardown resets the session;
    // raw-message ordering then prevents React from retaining a stale playing
    // snapshot while the window is being closed.
    post({ type: "identity-violation" });
  }

  function ensureConfiguredVideoIdentity() {
    if (identityFailed || disposed) return false;
    const state = configuredVideoIdentityState();
    if (state === "match") return true;
    if (state === "mismatch") reportIdentityViolation();
    return false;
  }

  function installHistoryIdentityGuard() {
    const historyPort = window.history;
    if (!historyPort || typeof URL !== "function") return;
    const expected = configuredIdentity(CONFIG.platformVideoId);
    const originalPushState = typeof historyPort.pushState === "function"
      ? historyPort.pushState
      : null;
    const originalReplaceState = typeof historyPort.replaceState === "function"
      ? historyPort.replaceState
      : null;

    function allowsHistoryTarget(value) {
      let target;
      try {
        target = new URL(value == null ? window.location.href : String(value), window.location.href);
      } catch (_) {
        // Preserve the platform's own TypeError/SecurityError contract for an
        // invalid URL by delegating to the original history method.
        return true;
      }
      return target.origin === "https://www.bilibili.com" &&
        configuredIdentityFromPath(target.pathname) === expected;
    }

    function guardedHistoryMethod(original) {
      return function(...args) {
        const target = args.length >= 3 ? args[2] : null;
        if (!allowsHistoryTarget(target)) {
          reportIdentityViolation();
          return undefined;
        }
        return Reflect.apply(original, this, args);
      };
    }

    const guardedPushState = originalPushState ? guardedHistoryMethod(originalPushState) : null;
    const guardedReplaceState = originalReplaceState ? guardedHistoryMethod(originalReplaceState) : null;
    if (guardedPushState) historyPort.pushState = guardedPushState;
    if (guardedReplaceState) historyPort.replaceState = guardedReplaceState;
    const verifyCurrentIdentity = () => { ensureConfiguredVideoIdentity(); };
    if (typeof window.addEventListener === "function") {
      window.addEventListener("popstate", verifyCurrentIdentity, true);
      window.addEventListener("pageshow", verifyCurrentIdentity, true);
    }
    removeHistoryIdentityGuard = () => {
      // Restore only wrappers owned by this session. A later document-start
      // installation may already have replaced them with a newer guard.
      if (guardedPushState && historyPort.pushState === guardedPushState) {
        historyPort.pushState = originalPushState;
      }
      if (guardedReplaceState && historyPort.replaceState === guardedReplaceState) {
        historyPort.replaceState = originalReplaceState;
      }
      if (typeof window.removeEventListener === "function") {
        window.removeEventListener("popstate", verifyCurrentIdentity, true);
        window.removeEventListener("pageshow", verifyCurrentIdentity, true);
      }
      removeHistoryIdentityGuard = null;
    };
  }

  function positiveCount(value) {
    if (typeof value === "number") {
      return Number.isFinite(value) && value > 0
        ? Math.floor(Math.min(value, Number.MAX_SAFE_INTEGER))
        : 0;
    }
    const text = normalizedText(value).replace(/,/g, "");
    const match = text.match(/([0-9]+(?:\.[0-9]+)?)\s*(万|亿|[kKmMbB])?/);
    if (!match) return 0;
    const suffix = String(match[2] || "").toLowerCase();
    const multiplier = suffix === "万" ? 10000
      : suffix === "亿" ? 100000000
      : suffix === "k" ? 1000
      : suffix === "m" ? 1000000
      : suffix === "b" ? 1000000000
      : 1;
    const number = Number(match[1]) * multiplier;
    return Number.isFinite(number) && number > 0
      ? Math.floor(Math.min(number, Number.MAX_SAFE_INTEGER))
      : 0;
  }

  function jsonLDHasType(value, expected) {
    const types = Array.isArray(value) ? value : [value];
    return types.some((type) => normalizedText(type).toLowerCase() === expected.toLowerCase());
  }

  function findVideoObject(value) {
    if (Array.isArray(value)) {
      for (const item of value) {
        const match = findVideoObject(item);
        if (match) return match;
      }
      return null;
    }
    if (!value || typeof value !== "object") return null;
    if (jsonLDHasType(value["@type"], "VideoObject")) return value;
    return findVideoObject(value["@graph"]);
  }

  function publisherName(value) {
    if (Array.isArray(value)) {
      for (const author of value) {
        const name = publisherName(author);
        if (name) return name;
      }
      return "";
    }
    if (typeof value === "string") return normalizedText(value);
    if (!value || typeof value !== "object") return "";
    return normalizedText(value.name || value.alternateName);
  }

  function metadataFromVideoObject(video) {
    const metadata = {
      publisher: publisherName(video && (video.author || video.publisher)),
      publishedAt: normalizedText(video && (video.uploadDate || video.datePublished)),
      viewCount: 0,
      likeCount: 0
    };
    const statistics = Array.isArray(video && video.interactionStatistic)
      ? video.interactionStatistic
      : video && video.interactionStatistic
        ? [video.interactionStatistic]
        : [];
    for (const statistic of statistics) {
      if (!statistic || typeof statistic !== "object") continue;
      const interaction = statistic.interactionType;
      const type = normalizedText(
        interaction && typeof interaction === "object"
          ? interaction["@type"] || interaction.type
          : interaction,
      ).toLowerCase();
      const count = positiveCount(statistic.userInteractionCount || statistic.interactionCount);
      if (type.includes("watchaction")) metadata.viewCount = count;
      if (type.includes("likeaction")) metadata.likeCount = count;
    }
    return metadata;
  }

  function readStructuredMetadata() {
    if (
      structuredMetadataNode &&
      structuredMetadataNode.isConnected !== false &&
      normalizedText(structuredMetadataNode.textContent) === structuredMetadataText
    ) {
      return structuredMetadata;
    }
    structuredMetadataNode = null;
    structuredMetadataText = "";
    structuredMetadata = { publisher: "", publishedAt: "", viewCount: 0, likeCount: 0 };
    const scripts = document.querySelectorAll('script[type="application/ld+json"]');
    for (const script of scripts) {
      const text = normalizedText(script && script.textContent);
      if (!text || text.length > 2000000) continue;
      try {
        const video = findVideoObject(JSON.parse(text));
        if (!video) continue;
        structuredMetadataNode = script;
        structuredMetadataText = text;
        structuredMetadata = metadataFromVideoObject(video);
        return structuredMetadata;
      } catch (_) {}
    }
    return structuredMetadata;
  }

  function selectorText(selector, attribute) {
    try {
      const node = document.querySelector(selector);
      if (!node) return "";
      if (attribute && typeof node.getAttribute === "function") {
        return normalizedText(node.getAttribute(attribute));
      }
      return normalizedText(node.textContent);
    } catch (_) {
      return "";
    }
  }

  function normalizedPublishedAt(value) {
    const text = normalizedText(value);
    const match = text.match(/\d{4}-\d{1,2}-\d{1,2}(?:[T ][0-9:.+Z-]+)?/);
    return match ? match[0].replace(" ", "T") : text;
  }

  function readPageMetadata() {
    const structured = readStructuredMetadata();
    return {
      publisher: structured.publisher ||
        selectorText('meta[itemprop="author"][content]', "content") ||
        selectorText(".up-name"),
      publishedAt: normalizedPublishedAt(
        structured.publishedAt ||
          selectorText('meta[itemprop="uploadDate"][content]', "content") ||
          selectorText('meta[itemprop="datePublished"][content]', "content") ||
          selectorText(".pubdate-ip-text"),
      ),
      viewCount: structured.viewCount || positiveCount(selectorText(".view-text")),
      likeCount: structured.likeCount || positiveCount(
        selectorText(".video-like-info.video-toolbar-item-text"),
      )
    };
  }

  function option(id, label) {
    return { id: String(id), label: normalizedText(label) || String(id) };
  }

  function uniqueOptions(options) {
    const seen = new Set();
    return options.filter((entry) => {
      if (!entry || !entry.id || seen.has(entry.id)) return false;
      seen.add(entry.id);
      return true;
    });
  }

  // This is the public port installed by Bilibili's canonical full-page nano
  // player. XiaDown only feature-detects it; it never creates or replaces it.
  function officialPlayer() {
    const candidate = window.player;
    if (!candidate || (typeof candidate !== "object" && typeof candidate !== "function")) return null;
    return candidate;
  }

  function removeOfficialPlayerListeners() {
    for (const remove of officialPlayerListeners.splice(0)) {
      try { remove(); } catch (_) {}
    }
  }

  function bindOfficialPlayer() {
    const next = officialPlayer();
    const eventTypes = window.nano && window.nano.EventType;
    if (next === officialPlayerRef && eventTypes === officialEventTypesRef) return;
    removeOfficialPlayerListeners();
    officialPlayerRef = next;
    officialEventTypesRef = eventTypes || null;
    if (!next) return;
    if (!eventTypes) return;
    const listener = () => reconcile();
    const attach = (method, key) => {
      const eventName = eventTypes[key];
      if (!eventName || typeof next[method] !== "function") return;
      try {
        next[method](eventName, listener);
        if (typeof next.off === "function") {
          officialPlayerListeners.push(() => next.off(eventName, listener));
        }
      } catch (_) {}
    };
    let initialized = true;
    if (typeof next.isInitialized === "function") {
      try { initialized = !!next.isInitialized(); } catch (_) { initialized = false; }
    }
    if (!initialized) attach("once", "Player_Initialized");
    for (const key of [
      "Player_Connected",
      "Player_LoadedMetadata",
      "Player_Quality_Changed",
      "Player_Quality_Rendered",
      "Player_Danmaku_Change"
    ]) {
      attach("on", key);
    }
  }

  function probeOfficialPlayer() {
    if (disposed) return;
    bindOfficialPlayer();
    if ((officialPlayerRef && officialEventTypesRef) || Date.now() >= playerProbeDeadline || playerProbeTimer) return;
    playerProbeTimer = window.setTimeout(() => {
      playerProbeTimer = 0;
      probeOfficialPlayer();
      reconcile();
    }, 150);
  }

  function scheduleReconcile() {
    try { window.requestAnimationFrame(() => reconcile()); } catch (_) {}
    window.setTimeout(reconcile, 80);
    window.setTimeout(reconcile, 350);
    window.setTimeout(reconcile, 1000);
  }

  function bilibiliCaptionEntries() {
    return Array.from(document.querySelectorAll(BILIBILI_SUBTITLE_ITEM_SELECTOR))
      .filter((node) => node && node.isConnected && normalizedText(node.getAttribute("data-lan")) !== "")
      .map((node) => {
        const language = normalizedText(node.getAttribute("data-lan"));
        const labelNode = node.querySelector(".bpx-player-ctrl-subtitle-language-item-text");
        return {
          id: "bili:" + encodeURIComponent(language),
          label: normalizedText(labelNode ? labelNode.textContent : node.textContent) || language,
          node,
          active: node.classList.contains("bpx-state-active") || node.getAttribute("aria-selected") === "true"
        };
      });
  }

  function bilibiliCaptionState() {
    const entries = bilibiliCaptionEntries();
    if (!entries.length) return null;
    const close = document.querySelector(BILIBILI_SUBTITLE_CLOSE_SELECTOR);
    const closed = !!(close && (
      close.classList.contains("bpx-state-active") ||
      close.getAttribute("aria-checked") === "true"
    ));
    const selected = closed ? null : entries.find((entry) => entry.active);
    const selectedId = selected ? selected.id : CAPTION_OFF_ID;
    if (selected) lastCaptionId = selected.id;
    return {
      source: "bilibili-dom",
      available: entries.length > 0 && typeof entries[0].node.click === "function",
      selectedId,
      options: entries.map((entry) => option(entry.id, entry.label)),
      entries,
      close
    };
  }

  function textTrackEntries(video) {
    if (!video || !video.textTracks) return [];
    try {
      return Array.from(video.textTracks).map((track, index) => {
        const identity = normalizedText(track.language || track.id || track.label || String(index));
        return {
          id: "track:" + String(index) + ":" + encodeURIComponent(identity),
          label: normalizedText(track.label) || normalizedText(track.language) || "Track " + String(index + 1),
          track,
          active: track.mode === "showing"
        };
      });
    } catch (_) {
      return [];
    }
  }

  function textTrackCaptionState(video) {
    const entries = textTrackEntries(video);
    if (!entries.length) return null;
    const selected = entries.find((entry) => entry.active);
    if (selected) lastCaptionId = selected.id;
    return {
      source: "text-tracks",
      available: true,
      selectedId: selected ? selected.id : CAPTION_OFF_ID,
      options: entries.map((entry) => option(entry.id, entry.label)),
      entries,
      close: null
    };
  }

  function captionState(video) {
    return bilibiliCaptionState() || textTrackCaptionState(video) || {
      source: "none",
      available: false,
      selectedId: CAPTION_OFF_ID,
      options: []
    };
  }

  function clickConnected(node) {
    if (!node || !node.isConnected || typeof node.click !== "function") return false;
    if (node.matches("[disabled],[aria-disabled='true']") || node.closest(".bui-disabled")) return false;
    const link = node.closest("a[href]");
    if (link) return false;
    try { node.click(); return true; } catch (_) { return false; }
  }

  function selectCaption(value) {
    const requested = normalizedText(value) || CAPTION_OFF_ID;
    const bili = bilibiliCaptionState();
    if (bili) {
      if (requested === CAPTION_OFF_ID) {
        if (bili.selectedId === CAPTION_OFF_ID || clickConnected(bili.close)) scheduleReconcile();
        return;
      }
      const entry = bili.entries.find((candidate) => candidate.id === requested);
      if (!entry) return;
      if (bili.selectedId !== requested && !clickConnected(entry.node)) return;
      lastCaptionId = requested;
      scheduleReconcile();
      return;
    }
    const tracks = textTrackCaptionState(media);
    if (!tracks) return;
    if (requested !== CAPTION_OFF_ID && !tracks.entries.some((entry) => entry.id === requested)) return;
    for (const entry of tracks.entries) {
      try { entry.track.mode = requested === entry.id ? "showing" : "disabled"; } catch (_) {}
    }
    if (requested !== CAPTION_OFF_ID) lastCaptionId = requested;
    scheduleReconcile();
  }

  function toggleCaptions() {
    const state = captionState(media);
    if (!state.available) return;
    if (state.selectedId !== CAPTION_OFF_ID) {
      selectCaption(CAPTION_OFF_ID);
      return;
    }
    const target = state.options.some((entry) => entry.id === lastCaptionId) && lastCaptionId !== CAPTION_OFF_ID
      ? lastCaptionId
      : (state.options[0] || {}).id;
    if (target) selectCaption(target);
  }

  function playInfoQualityLabels() {
    const labels = Object.assign({}, QUALITY_LABELS);
    let data = null;
    try { data = window.__playinfo__ && (window.__playinfo__.data || window.__playinfo__); } catch (_) {}
    if (!data || typeof data !== "object") return labels;
    const formats = data.support_formats || data.supportFormats;
    if (Array.isArray(formats)) {
      for (const format of formats) {
        if (!format || format.quality == null) continue;
        const label = format.new_description || format.newDescription || format.display_desc ||
          format.displayDesc || format.description;
        if (normalizedText(label)) labels[String(format.quality)] = normalizedText(label);
      }
    }
    const qualities = data.accept_quality || data.acceptQuality;
    const descriptions = data.accept_description || data.acceptDescription;
    if (Array.isArray(qualities) && Array.isArray(descriptions)) {
      qualities.forEach((quality, index) => {
        if (descriptions[index] != null) labels[String(quality)] = normalizedText(descriptions[index]);
      });
    }
    return labels;
  }

  function officialQualityState() {
    const player = officialPlayer();
    if (!player || typeof player.getSupportedQualityList !== "function" ||
        typeof player.getQuality !== "function" || typeof player.requestQuality !== "function") return null;
    let supported = null;
    let current = null;
    try { supported = player.getSupportedQualityList(); } catch (_) { return null; }
    try { current = player.getQuality(); } catch (_) { return null; }
    if (!Array.isArray(supported) || supported.length === 0) return null;
    const labels = playInfoQualityLabels();
    const values = supported.map((entry) => {
      if (entry && typeof entry === "object") return entry.quality == null ? entry.id : entry.quality;
      return entry;
    }).map((entry) => Number(entry)).filter((entry) => Number.isFinite(entry) && entry >= 0);
    const nowQuality = current && typeof current === "object" ? current.nowQ : current;
    const nowQualityNumber = Number(nowQuality);
    if (Number.isFinite(nowQualityNumber) && nowQualityNumber >= 0 && !values.some((entry) => entry === nowQualityNumber)) {
      values.push(nowQualityNumber);
    }
    const options = uniqueOptions(values.map((quality) => {
      const id = String(quality);
      return option(id, labels[id] || (id === "0" ? "Auto" : id));
    }));
    return {
      source: "bilibili-port",
      available: options.length > 0,
      selectedId: Number.isFinite(nowQualityNumber) && nowQualityNumber >= 0 ? String(nowQualityNumber) : "",
      options,
      player
    };
  }

  function domQualityState() {
    const selectors = [
      BILIBILI_QUALITY_ITEM_SELECTOR,
      ".bpx-player-ctrl-quality-bubble .bpx-player-ctrl-quality-menu-item[data-value]",
      ".bilibili-player-video-btn-quality-menu-item[data-value]"
    ];
    const nodes = Array.from(document.querySelectorAll(selectors.join(",")))
      .filter((node, index, all) => node && node.isConnected && all.indexOf(node) === index);
    if (!nodes.length) return null;
    const entries = nodes.map((node) => {
      const id = normalizedText(node.getAttribute("data-value"));
      return {
        id,
        label: normalizedText(node.textContent) || id,
        node,
        active: node.classList.contains("bpx-state-active") || node.getAttribute("aria-selected") === "true"
      };
    }).filter((entry) => entry.id !== "");
    if (!entries.length) return null;
    const selected = entries.find((entry) => entry.active);
    return {
      source: "bilibili-dom",
      available: entries.some((entry) => typeof entry.node.click === "function"),
      selectedId: selected ? selected.id : "",
      options: uniqueOptions(entries.map((entry) => option(entry.id, entry.label))),
      entries
    };
  }

  function qualityState() {
    return officialQualityState() || domQualityState() || {
      source: "none",
      available: false,
      selectedId: "",
      options: []
    };
  }

  function selectQuality(value) {
    const requested = normalizedText(value);
    if (!requested) return;
    const port = officialQualityState();
    if (port && port.options.some((entry) => entry.id === requested)) {
      if (port.selectedId === requested) {
        scheduleReconcile();
        return;
      }
      const quality = Number(requested);
      if (!Number.isFinite(quality)) return;
      try {
        const pending = port.player.requestQuality(quality);
        if (pending && typeof pending.then === "function") {
          pending.then(scheduleReconcile, scheduleReconcile);
        } else {
          scheduleReconcile();
        }
        return;
      } catch (_) {}
    }
    const dom = domQualityState();
    if (!dom) return;
    const entry = dom.entries.find((candidate) => candidate.id === requested);
    if (!entry) return;
    if (dom.selectedId === requested || clickConnected(entry.node)) scheduleReconcile();
  }

  function officialDanmakuState() {
    const player = officialPlayer();
    const danmaku = player && player.danmaku;
    if (!danmaku || typeof danmaku.isOpen !== "function" || typeof danmaku.isDisabled !== "function" ||
        typeof danmaku.open !== "function" || typeof danmaku.close !== "function") return null;
    try {
      const disabled = !!danmaku.isDisabled();
      return {
        source: "bilibili-port",
        available: !disabled,
        enabled: disabled ? false : !!danmaku.isOpen(),
        danmaku
      };
    } catch (_) {
      return null;
    }
  }

  function domDanmakuState() {
    const input = document.querySelector(BILIBILI_DANMAKU_INPUT_SELECTOR) ||
      document.querySelector(".bilibili-player-video-danmaku-switch input[type='checkbox']");
    if (!input || !input.isConnected) return null;
    const disabled = input.matches("[disabled],[aria-disabled='true']") || !!input.closest(".bui-disabled");
    let enabled = false;
    if (typeof input.checked === "boolean") enabled = input.checked;
    else if (input.getAttribute("aria-checked") != null) enabled = input.getAttribute("aria-checked") === "true";
    else enabled = input.classList.contains("bpx-state-active") || !!input.closest(".bpx-state-active,.bui-checked");
    return {
      source: "bilibili-dom",
      available: !disabled && typeof input.click === "function",
      enabled,
      input
    };
  }

  function danmakuState() {
    return officialDanmakuState() || domDanmakuState() || {
      source: "none",
      available: false,
      enabled: false
    };
  }

  function toggleDanmaku() {
    const port = officialDanmakuState();
    if (port) {
      if (!port.available) return;
      try {
        const result = port.enabled ? port.danmaku.close() : port.danmaku.open();
        if (result && typeof result.then === "function") result.then(scheduleReconcile, scheduleReconcile);
        else scheduleReconcile();
      } catch (_) {}
      return;
    }
    const dom = domDanmakuState();
    if (!dom || !dom.available) return;
    if (clickConnected(dom.input)) scheduleReconcile();
  }

  function post(payload) {
    const message = JSON.stringify(Object.assign({
      source: SOURCE,
      sessionId: CONFIG.sessionId,
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
    } catch (_) {}
  }

  function failConfiguredVideoIdentity() {
    if (identityFailed || disposed) return;
    identityFailed = true;
    const root = document.documentElement;
    const fullscreenPresentationActive = !!(
      root && root.getAttribute("data-xiadown-rss-bilibili-fullscreen") === "true"
    );
    removeMediaListeners();
    removeOfficialPlayerListeners();
    officialPlayerRef = null;
    officialEventTypesRef = null;
    let videos = [];
    try { videos = Array.from(document.querySelectorAll("video")); } catch (_) {}
    for (const video of videos) {
      try { video.pause(); } catch (_) {}
      unmarkMedia(video);
    }
    media = null;
    if (observer) {
      observer.disconnect();
      observer = null;
    }
    if (documentRootObserver) {
      documentRootObserver.disconnect();
      documentRootObserver = null;
    }
    if (pollTimer) {
      window.clearInterval(pollTimer);
      pollTimer = 0;
    }
    if (playerProbeTimer) {
      window.clearTimeout(playerProbeTimer);
      playerProbeTimer = 0;
    }
    document.removeEventListener("fullscreenchange", reconcile, true);
    document.removeEventListener("webkitfullscreenchange", reconcile, true);
    document.removeEventListener("click", requestHostFullscreenExit, true);
    document.removeEventListener("ended", blockAutomaticAdvance, true);
    setFullscreenPresentation(false);
    post({
      type: "state",
      provider: "bilibili",
      platformVideoId: CONFIG.platformVideoId,
      available: false,
      state: "error",
      title: "",
      publisher: "",
      publishedAt: "",
      viewCount: 0,
      likeCount: 0,
      currentTime: 0,
      duration: 0,
      bufferedTime: 0,
      volume: CONFIG.volume,
      muted: !!CONFIG.muted,
      playbackRate: CONFIG.playbackRate,
      fullscreen: false,
      controls: {
        playPause: false, seek: false, volume: false, playbackRate: false, fullscreen: false,
        captions: false, quality: false, danmaku: false
      },
      playbackRateOptions: RATE_OPTIONS,
      captionOptions: [],
      qualityOptions: [],
      danmakuEnabled: false,
      selections: { playbackRateId: String(CONFIG.playbackRate), captionId: "", qualityId: "" },
      errorMessage: ""
    });
    if (fullscreenPresentationActive) post({ type: "fullscreen-exit-request" });
  }

  function installVideoOnlyStyle() {
    const styleID = "xiadown-rss-bilibili-video-only";
    if (document.getElementById(styleID)) return true;
    const root = document.head || document.documentElement;
    if (!root) return false;
    const style = document.createElement("style");
    style.id = styleID;
    style.textContent = [
      "html,body{margin:0!important;padding:0!important;width:100%%!important;height:100%%!important;overflow:hidden!important;background:#000!important;pointer-events:none!important;user-select:none!important;}",
      "body>*{visibility:hidden!important;pointer-events:none!important;}",
      ".bpx-player-control-wrap,.bpx-player-control-mask,.bpx-player-top-wrap,.bpx-player-sending-area,.bpx-player-toast-wrap,.bpx-player-dialog-wrap,.bpx-player-dialog-area,.bpx-player-tooltip-area,.bpx-player-context-area,.bpx-player-ending-wrap,.bpx-player-ending-panel,.bpx-player-state-wrap,.bpx-player-loading-panel,.bpx-player-video-poster,.bpx-player-business-wrap,.bpx-player-music-wrap,.bpx-player-cmd-dm-wrap{display:none!important;visibility:hidden!important;pointer-events:none!important;}",
      "video[" + ACTIVE_VIDEO_ATTRIBUTE + "]{display:block!important;visibility:visible!important;opacity:1!important;position:fixed!important;inset:0!important;top:0!important;right:0!important;bottom:0!important;left:0!important;width:100vw!important;height:100vh!important;min-width:0!important;min-height:0!important;max-width:none!important;max-height:none!important;margin:0!important;padding:0!important;border:0!important;border-radius:0!important;transform:none!important;clip:auto!important;clip-path:none!important;object-fit:contain!important;background:#000!important;z-index:2147483640!important;pointer-events:none!important;}",
      ".bpx-player-render-dm-wrap{visibility:visible!important;position:fixed!important;inset:0!important;width:100vw!important;height:100vh!important;z-index:2147483645!important;overflow:hidden!important;pointer-events:none!important;}",
      ".bpx-player-subtitle-wrap{visibility:visible!important;position:fixed!important;inset:0!important;width:100vw!important;height:100vh!important;z-index:2147483646!important;pointer-events:none!important;}",
      ".bpx-player-render-dm-wrap *,.bpx-player-subtitle-wrap *{pointer-events:none!important;}",
      "video[" + ACTIVE_VIDEO_ATTRIBUTE + "]::-webkit-media-controls{display:none!important;}",
      "html[data-xiadown-rss-bilibili-fullscreen='true'],html[data-xiadown-rss-bilibili-fullscreen='true'] body{pointer-events:auto!important;user-select:auto!important;}",
      "html[data-xiadown-rss-bilibili-fullscreen='true'] .bpx-player-container,html[data-xiadown-rss-bilibili-fullscreen='true'] .bpx-player-control-wrap,html[data-xiadown-rss-bilibili-fullscreen='true'] .bpx-player-control-mask,html[data-xiadown-rss-bilibili-fullscreen='true'] .bpx-player-tooltip-area,html[data-xiadown-rss-bilibili-fullscreen='true'] .bpx-player-dialog-area,html[data-xiadown-rss-bilibili-fullscreen='true'] .bpx-player-toast-wrap{visibility:visible!important;pointer-events:auto!important;}",
      "html[data-xiadown-rss-bilibili-fullscreen='true'] .bpx-player-container,html[data-xiadown-rss-bilibili-fullscreen='true'] .bpx-player-primary-area,html[data-xiadown-rss-bilibili-fullscreen='true'] .bpx-player-video-area{display:block!important;visibility:visible!important;position:fixed!important;inset:0!important;width:100vw!important;height:100vh!important;min-width:0!important;min-height:0!important;max-width:none!important;max-height:none!important;margin:0!important;padding:0!important;transform:none!important;overflow:hidden!important;background:#000!important;}",
      "html[data-xiadown-rss-bilibili-fullscreen='true'] .bpx-player-control-wrap,html[data-xiadown-rss-bilibili-fullscreen='true'] .bpx-player-control-mask,html[data-xiadown-rss-bilibili-fullscreen='true'] .bpx-player-tooltip-area,html[data-xiadown-rss-bilibili-fullscreen='true'] .bpx-player-dialog-area,html[data-xiadown-rss-bilibili-fullscreen='true'] .bpx-player-toast-wrap{display:block!important;z-index:2147483647!important;}",
      "html[data-xiadown-rss-bilibili-fullscreen='true'] .bpx-player-control-bottom,html[data-xiadown-rss-bilibili-fullscreen='true'] .bpx-player-control-mask{opacity:1!important;visibility:visible!important;}",
      "html[data-xiadown-rss-bilibili-fullscreen='true'] .bpx-player-ctrl-full,html[data-xiadown-rss-bilibili-fullscreen='true'] .bpx-player-ctrl-web,html[data-xiadown-rss-bilibili-fullscreen='true'] .bpx-player-ctrl-next,html[data-xiadown-rss-bilibili-fullscreen='true'] .bpx-player-ctrl-setting-autoplay,html[data-xiadown-rss-bilibili-fullscreen='true'] .bpx-player-ctrl-setting-auto-play,html[data-xiadown-rss-bilibili-fullscreen='true'] .bpx-player-setting-autoplay{display:none!important;visibility:hidden!important;pointer-events:none!important;}",
      "html[data-xiadown-rss-bilibili-fullscreen='true'] .bpx-player-sending-area{display:block!important;visibility:visible!important;position:fixed!important;inset:0!important;z-index:2147483647!important;pointer-events:none!important;background:transparent!important;}",
      "html[data-xiadown-rss-bilibili-fullscreen='true'] .bpx-player-sending-area *{visibility:hidden!important;pointer-events:none!important;}",
      "html[data-xiadown-rss-bilibili-fullscreen='true'] .bpx-player-sending-area .bpx-player-dm-switch{display:inline-flex!important;visibility:visible!important;opacity:1!important;position:fixed!important;left:20px!important;bottom:64px!important;pointer-events:auto!important;}",
      "html[data-xiadown-rss-bilibili-fullscreen='true'] .bpx-player-sending-area .bpx-player-dm-switch *{visibility:visible!important;opacity:1!important;pointer-events:auto!important;}",
      "html[data-xiadown-rss-bilibili-fullscreen='true'] video[" + ACTIVE_VIDEO_ATTRIBUTE + "]{pointer-events:auto!important;}",
      "*{scrollbar-width:none!important;}*::-webkit-scrollbar{display:none!important;}"
    ].join("");
    root.appendChild(style);
    return true;
  }

  function installVideoOnlyStyleAtDocumentStart() {
    if (installVideoOnlyStyle() || documentRootObserver) return;
    documentRootObserver = new MutationObserver(() => {
      if (!installVideoOnlyStyle()) return;
      documentRootObserver.disconnect();
      documentRootObserver = null;
    });
    // Observing the Document works before <html>/<head> exists, avoiding the
    // remote control flash that a DOMContentLoaded-only injection permits.
    documentRootObserver.observe(document, { childList: true, subtree: true });
  }

  function selectMedia() {
    const player = officialPlayer();
    if (player && typeof player.mediaElement === "function") {
      try {
        const officialMedia = player.mediaElement();
        if (officialMedia && officialMedia.tagName === "VIDEO" && officialMedia.isConnected) return officialMedia;
      } catch (_) {}
    }
    const videos = Array.from(document.querySelectorAll("video"));
    return videos.find((video) => !video.paused && !video.ended) ||
      videos.find((video) => video.readyState > 0) ||
      videos[0] ||
      null;
  }

  function bufferedTime(video) {
    let value = 0;
    try {
      for (let index = 0; index < video.buffered.length; index += 1) {
        value = Math.max(value, finite(video.buffered.end(index), 0));
      }
    } catch (_) {}
    return value;
  }

  function fullscreenActive(video) {
    return Boolean(
      document.fullscreenElement ||
      document.webkitFullscreenElement ||
      (video && video.webkitDisplayingFullscreen)
    );
  }

  function stateFor(video, preferred) {
    if (!video) return "loading";
    if (video.error) return "error";
    // Bilibili may replace its media element while processing an ended event.
    // Keep the transport terminal until the user explicitly asks to replay;
    // otherwise the replacement reports paused and can be auto-started by the
    // official player on the next reconciliation pass.
    if (terminalEnded || preferred === "ended" || video.ended) return "ended";
    if (preferred === "buffering" || preferred === "loading") return preferred;
    if (video.seeking || (video.readyState < 3 && !video.paused)) return "buffering";
    return video.paused ? "paused" : "playing";
  }

  function controlsFor(video, captions, quality, danmaku) {
    if (!video) {
      return {
        playPause: false, seek: false, volume: false, playbackRate: false, fullscreen: false,
        captions: false, quality: false, danmaku: false
      };
    }
    const root = document.documentElement;
    const fullscreen = Boolean(
      video.requestFullscreen ||
      video.webkitRequestFullscreen ||
      video.webkitEnterFullscreen ||
      (root && (root.requestFullscreen || root.webkitRequestFullscreen))
    );
    return {
      playPause: typeof video.play === "function" && typeof video.pause === "function",
      seek: Number.isFinite(Number(video.duration)) && Number(video.duration) > 0,
      volume: "volume" in video && "muted" in video,
      playbackRate: "playbackRate" in video,
      fullscreen,
      captions: !!(captions && captions.available),
      quality: !!(quality && quality.available),
      danmaku: !!(danmaku && danmaku.available)
    };
  }

  function snapshot(preferred, errorMessage) {
    if (!ensureConfiguredVideoIdentity()) return;
    const video = media;
    if (!video) {
      const empty = JSON.stringify({
        preferred: preferred || "loading",
        errorMessage: errorMessage || "",
        pageMetadata
      });
      if (empty === lastEmptySnapshot) return;
      lastEmptySnapshot = empty;
      post({
        type: "state",
        provider: "bilibili",
        platformVideoId: CONFIG.platformVideoId,
        available: false,
        state: preferred || "loading",
        title: document.title || "",
        publisher: pageMetadata.publisher,
        publishedAt: pageMetadata.publishedAt,
        viewCount: pageMetadata.viewCount,
        likeCount: pageMetadata.likeCount,
        currentTime: 0,
        duration: 0,
        bufferedTime: 0,
        volume: CONFIG.volume,
        muted: !!CONFIG.muted,
        playbackRate: CONFIG.playbackRate,
        fullscreen: false,
        controls: controlsFor(null, null, null, null),
        playbackRateOptions: RATE_OPTIONS,
        captionOptions: [],
        qualityOptions: [],
        danmakuEnabled: false,
        selections: { playbackRateId: String(CONFIG.playbackRate), captionId: "", qualityId: "" },
        errorMessage: errorMessage || ""
      });
      return;
    }
    lastEmptySnapshot = "";
    const state = stateFor(video, preferred);
    let mediaError = errorMessage || "";
    if (!mediaError && video.error) {
      mediaError = "HTMLMediaElement error " + String(video.error.code || "unknown");
    }
    const captions = captionState(video);
    const quality = qualityState();
    const danmaku = danmakuState();
    post({
      type: "state",
      provider: "bilibili",
      platformVideoId: CONFIG.platformVideoId,
      available: true,
      state,
      title: document.title || "",
      publisher: pageMetadata.publisher,
      publishedAt: pageMetadata.publishedAt,
      viewCount: pageMetadata.viewCount,
      likeCount: pageMetadata.likeCount,
      currentTime: Math.max(0, finite(video.currentTime, 0)),
      duration: Math.max(0, finite(video.duration, 0)),
      bufferedTime: Math.max(0, bufferedTime(video)),
      volume: clamp(finite(video.volume, CONFIG.volume), 0, 1),
      muted: !!video.muted,
      playbackRate: Math.max(0.01, finite(video.playbackRate, CONFIG.playbackRate)),
      fullscreen: fullscreenActive(video),
      controls: controlsFor(video, captions, quality, danmaku),
      playbackRateOptions: RATE_OPTIONS,
      captionOptions: captions.options,
      qualityOptions: quality.options,
      danmakuEnabled: !!danmaku.enabled,
      selections: {
        playbackRateId: String(video.playbackRate || CONFIG.playbackRate),
        captionId: captions.selectedId === CAPTION_OFF_ID ? "" : captions.selectedId,
        qualityId: quality.selectedId
      },
      errorMessage: mediaError
    });
  }

  function removeMediaListeners() {
    for (const remove of mediaListeners.splice(0)) {
      try { remove(); } catch (_) {}
    }
  }

  function unmarkMedia(video) {
    if (!video || typeof video.getAttribute !== "function") return;
    if (video.getAttribute(ACTIVE_VIDEO_ATTRIBUTE) !== CONFIG.sessionId) return;
    try { video.removeAttribute(ACTIVE_VIDEO_ATTRIBUTE); } catch (_) {}
  }

  function listen(target, name, listener) {
    if (!target || typeof target.addEventListener !== "function") return;
    target.addEventListener(name, listener, true);
    mediaListeners.push(() => target.removeEventListener(name, listener, true));
  }

  function markMedia(video) {
    if (!video || typeof video.getAttribute !== "function") return;
    if (video.getAttribute(ACTIVE_VIDEO_ATTRIBUTE) === CONFIG.sessionId) return;
    try { video.setAttribute(ACTIVE_VIDEO_ATTRIBUTE, CONFIG.sessionId); } catch (_) {}
  }

  function applyRequestedStart(video) {
    if (requestedStartApplied || CONFIG.startSeconds <= 0 || video.readyState < 1) return;
    const duration = finite(video.duration, CONFIG.startSeconds);
    const target = duration > 0 ? Math.min(CONFIG.startSeconds, duration) : CONFIG.startSeconds;
    try {
      video.currentTime = Math.max(0, target);
      requestedStartApplied = true;
    } catch (_) {}
  }

  function play(reportFailure) {
    if (!ensureConfiguredVideoIdentity()) return;
    if (!media) return;
    if (!reportFailure && terminalEnded) return;
    if (reportFailure && terminalEnded) {
      terminalEnded = false;
      try { media.currentTime = 0; } catch (_) {}
    }
    const pending = media.play();
    if (pending && typeof pending.catch === "function") {
      pending.catch((error) => {
        if (reportFailure) {
          snapshot("error", String(error && error.message ? error.message : error));
        } else {
          snapshot("paused", "");
        }
      });
    }
  }

  function attachMedia(nextMedia) {
    if (!ensureConfiguredVideoIdentity()) return;
    if (nextMedia === media) {
      if (media) {
        if (media.controls) media.controls = false;
        markMedia(media);
      }
      return;
    }
    removeMediaListeners();
    unmarkMedia(media);
    media = nextMedia || null;
    if (!media) {
      snapshot("loading", "");
      return;
    }
    media.controls = false;
    markMedia(media);
    media.playsInline = true;
    try { media.volume = clamp(finite(CONFIG.volume, 1), 0, 1); } catch (_) {}
    try { media.muted = !!CONFIG.muted; } catch (_) {}
    try { media.playbackRate = CONFIG.playbackRate; } catch (_) {}

    listen(media, "loadstart", () => snapshot("loading", ""));
    listen(media, "loadedmetadata", () => { applyRequestedStart(media); snapshot("", ""); });
    listen(media, "durationchange", () => { applyRequestedStart(media); snapshot("", ""); });
    listen(media, "canplay", () => snapshot("", ""));
    listen(media, "playing", () => {
      if (terminalEnded) {
        try { media.pause(); } catch (_) {}
        snapshot("ended", "");
        return;
      }
      snapshot("playing", "");
    });
    listen(media, "pause", () => snapshot("paused", ""));
    listen(media, "waiting", () => snapshot("buffering", ""));
    listen(media, "stalled", () => snapshot("buffering", ""));
    listen(media, "seeking", () => snapshot("buffering", ""));
    listen(media, "seeked", () => snapshot("", ""));
    listen(media, "timeupdate", () => snapshot("", ""));
    listen(media, "progress", () => snapshot("", ""));
    listen(media, "volumechange", () => snapshot("", ""));
    listen(media, "ratechange", () => snapshot("", ""));
    listen(media, "ended", (event) => blockAutomaticAdvance(event));
    listen(media, "error", () => snapshot("error", ""));
    listen(media, "webkitbeginfullscreen", () => snapshot("", ""));
    listen(media, "webkitendfullscreen", () => snapshot("", ""));
    if (media.textTracks) {
      listen(media.textTracks, "addtrack", () => snapshot("", ""));
      listen(media.textTracks, "removetrack", () => snapshot("", ""));
      listen(media.textTracks, "change", () => snapshot("", ""));
    }
    applyRequestedStart(media);
    if (terminalEnded) {
      try { media.pause(); } catch (_) {}
      snapshot("ended", "");
      return;
    }
    snapshot("", "");
    if (CONFIG.autoplay && !initialAutoplayAttempted) {
      // Autoplay belongs only to the first valid media element for this
      // prepared session. Quality changes and Bilibili player rebuilds keep
      // their own playback state and must never trigger another host play().
      initialAutoplayAttempted = true;
      window.setTimeout(() => play(false), 0);
    }
  }

  function reconcile() {
    if (disposed || !ensureConfiguredVideoIdentity()) return;
    installVideoOnlyStyleAtDocumentStart();
    probeOfficialPlayer();
    pageMetadata = readPageMetadata();
    const nextMedia = selectMedia();
    if (nextMedia !== media) attachMedia(nextMedia);
    else if (media) attachMedia(media);
    snapshot("", "");
  }

  function install() {
    if (disposed) return;
    installVideoOnlyStyleAtDocumentStart();
    reconcile();
    if (identityFailed) return;
    observer = new MutationObserver(reconcile);
    if (document.documentElement) {
      observer.observe(document.documentElement, {
        childList: true,
        subtree: true,
        attributes: true,
        attributeFilter: ["class", "checked", "aria-checked", "aria-selected", "data-value", "data-lan"]
      });
    }
    pollTimer = window.setInterval(reconcile, 500);
    document.addEventListener("fullscreenchange", reconcile, true);
    document.addEventListener("webkitfullscreenchange", reconcile, true);
    document.addEventListener("click", requestHostFullscreenExit, true);
  }

  function requestHostFullscreenExit(event) {
    const root = document.documentElement;
    if (!root || root.getAttribute("data-xiadown-rss-bilibili-fullscreen") !== "true") return;
    const eventTarget = event && event.target && typeof event.target.closest === "function"
      ? event.target
      : null;
    const advance = eventTarget && eventTarget.closest(
      ".bpx-player-ctrl-next, .bpx-player-ctrl-setting-autoplay, .bpx-player-ctrl-setting-auto-play, .bpx-player-setting-autoplay",
    );
    if (advance) {
      try { event.preventDefault(); } catch (_) {}
      try { event.stopImmediatePropagation(); } catch (_) {}
      return;
    }
    const target = eventTarget && eventTarget.closest(
      ".bpx-player-ctrl-full, .bpx-player-ctrl-web",
    );
    if (!target) return;
    try { event.preventDefault(); } catch (_) {}
    try { event.stopImmediatePropagation(); } catch (_) {}
    post({ type: "fullscreen-exit-request" });
  }

  function blockAutomaticAdvance(event) {
    const target = event && event.target;
    if (!target || String(target.tagName || "").toUpperCase() !== "VIDEO") return;
    terminalEnded = true;
    try { target.pause(); } catch (_) {}
    try { event.preventDefault(); } catch (_) {}
    try { event.stopImmediatePropagation(); } catch (_) {}
    if (media === target) snapshot("ended", "");
  }

  window.__xiadownRSSBilibiliPlayer = {
    sessionId: CONFIG.sessionId,
    get disposed() { return disposed; },
    play() { play(true); },
    pause() {
      if (!ensureConfiguredVideoIdentity()) return;
      if (media) media.pause();
    },
    seek(seconds) {
      if (!ensureConfiguredVideoIdentity()) return;
      if (!media) return;
      const requested = Math.max(0, finite(seconds, 0));
      const duration = finite(media.duration, requested);
      try { media.currentTime = duration > 0 ? Math.min(requested, duration) : requested; } catch (_) {}
      snapshot("", "");
    },
    volume(value, muted) {
      if (!ensureConfiguredVideoIdentity()) return;
      if (!media) return;
      try { media.volume = clamp(finite(value, 1), 0, 1); } catch (_) {}
      try { media.muted = !!muted; } catch (_) {}
      snapshot("", "");
    },
    rate(value) {
      if (!ensureConfiguredVideoIdentity()) return;
      if (!media) return;
      const rate = finite(value, 1);
      if (![0.5, 0.75, 1, 1.25, 1.5, 2].includes(rate)) return;
      try { media.playbackRate = rate; } catch (_) {}
      snapshot("", "");
    },
    toggleCaptions() {
      if (ensureConfiguredVideoIdentity()) toggleCaptions();
    },
    selectCaption(value) {
      if (ensureConfiguredVideoIdentity()) selectCaption(value);
    },
    selectQuality(value) {
      if (ensureConfiguredVideoIdentity()) selectQuality(value);
    },
    toggleDanmaku() {
      if (ensureConfiguredVideoIdentity()) toggleDanmaku();
    },
    fullscreenPresentation(active) {
      if (active && !ensureConfiguredVideoIdentity()) return;
      setFullscreenPresentation(active === true);
    },
    snapshot() { snapshot("", ""); },
    reconcile() { reconcile(); },
    dispose() {
      disposed = true;
      removeMediaListeners();
      removeOfficialPlayerListeners();
      officialPlayerRef = null;
      officialEventTypesRef = null;
      unmarkMedia(media);
      setFullscreenPresentation(false);
      media = null;
      if (observer) observer.disconnect();
      if (documentRootObserver) documentRootObserver.disconnect();
      if (pollTimer) window.clearInterval(pollTimer);
      if (playerProbeTimer) window.clearTimeout(playerProbeTimer);
      document.removeEventListener("fullscreenchange", reconcile, true);
      document.removeEventListener("webkitfullscreenchange", reconcile, true);
      document.removeEventListener("click", requestHostFullscreenExit, true);
      document.removeEventListener("ended", blockAutomaticAdvance, true);
      if (removeHistoryIdentityGuard) removeHistoryIdentityGuard();
    }
  };

  // Hide the remote transport at the earliest document phase possible; the
  // install/reconcile path repeats this after DOMContentLoaded and mutations.
  installHistoryIdentityGuard();
  restoreFullscreenPresentation();
  document.addEventListener("ended", blockAutomaticAdvance, true);
  installVideoOnlyStyleAtDocumentStart();
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", install, { once: true });
  } else {
    install();
  }
})();`, payload)
}
