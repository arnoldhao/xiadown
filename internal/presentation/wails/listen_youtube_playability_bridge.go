package wails

// listenYouTubePlayabilityBridgeScript is shared by the YouTube Music and
// regular YouTube players. YouTube's player response carries more reliable
// error information than the rendered, localized error page:
//
//   - playabilityStatus.status describes the broad failure class.
//   - reason/subreason preserve YouTube's detailed upstream explanation.
//   - playerCaptchaViewModel identifies the bot-verification challenge without
//     depending on the page language.
//
// The DOM text remains a final fallback because YouTube does not expose a
// stable public error enum for every watch-page failure (notably geo blocks).
const listenYouTubePlayabilityBridgeScript = `
  function listenYouTubeNormalizeErrorText(value) {
    return String(value || "")
      .split(/\r?\n+/)
      .map((line) => line.replace(/[ \t\f\v]+/g, " ").trim())
      .filter(Boolean)
      .join("\n")
      .slice(0, 500);
  }

  function listenYouTubeStructuredText(value) {
    if (typeof value === "string") {
      return listenYouTubeNormalizeErrorText(value);
    }
    if (!value || typeof value !== "object") return "";
    if (typeof value.simpleText === "string") {
      return listenYouTubeNormalizeErrorText(value.simpleText);
    }
    if (Array.isArray(value.runs)) {
      return listenYouTubeNormalizeErrorText(
        value.runs.map((run) => String(run && run.text || "")).join("")
      );
    }
    return "";
  }

  function listenYouTubeUnwrapPlayerResponse(value, depth) {
    const currentDepth = Number(depth || 0);
    if (currentDepth > 4) return null;
    let candidate = value;
    if (typeof candidate === "string") {
      try { candidate = JSON.parse(candidate); } catch (error) { return null; }
    }
    if (!candidate || typeof candidate !== "object") return null;
    if (candidate.playabilityStatus && typeof candidate.playabilityStatus === "object") {
      return candidate;
    }
    const nestedKeys = ["playerResponse", "player_response", "response"];
    for (const key of nestedKeys) {
      if (candidate[key] === candidate) continue;
      const nested = listenYouTubeUnwrapPlayerResponse(candidate[key], currentDepth + 1);
      if (nested) return nested;
    }
    if (candidate.args && typeof candidate.args === "object") {
      return listenYouTubeUnwrapPlayerResponse(
        candidate.args.player_response,
        currentDepth + 1
      );
    }
    return null;
  }

  function listenYouTubeResponseVideoId(response) {
    if (!response || typeof response !== "object") return "";
    const details = response.videoDetails;
    if (details && (details.videoId || details.video_id)) {
      return String(details.videoId || details.video_id);
    }
    return "";
  }

  function listenYouTubeActiveVideoId(api) {
    if (api && typeof api.getVideoData === "function") {
      try {
        const data = api.getVideoData();
        if (data && (data.video_id || data.videoId)) {
          return String(data.video_id || data.videoId);
        }
      } catch (error) {}
    }
    try {
      return new URL(window.location.href).searchParams.get("v") || "";
    } catch (error) {
      return "";
    }
  }

  function listenYouTubePlayerResponse() {
    const playerHost = listenYouTubeBridgePlayerHost();
    const api = listenYouTubeBridgePlayerApi();
    const activeVideoId = listenYouTubeActiveVideoId(api);
    const candidates = [];
    if (api) {
      if (typeof api.getPlayerResponse === "function") {
        try { candidates.push(api.getPlayerResponse()); } catch (error) {}
      }
      if (typeof api.getPlayerData === "function") {
        try { candidates.push(api.getPlayerData()); } catch (error) {}
      }
      candidates.push(api.playerResponse, api.player_response);
    }
    if (playerHost) {
      candidates.push(playerHost.playerResponse, playerHost.player_response);
    }
    let responseWithoutVideoId = null;
    for (const candidate of candidates) {
      const response = listenYouTubeUnwrapPlayerResponse(candidate);
      if (!response) continue;
      const responseVideoId = listenYouTubeResponseVideoId(response);
      if (!activeVideoId || !responseVideoId || activeVideoId === responseVideoId) {
        if (responseVideoId || !activeVideoId) return response;
        if (!responseWithoutVideoId) responseWithoutVideoId = response;
      }
    }
    if (responseWithoutVideoId) return responseWithoutVideoId;

    const initialResponse = listenYouTubeUnwrapPlayerResponse(window.ytInitialPlayerResponse);
    if (!initialResponse) return null;
    const initialResponseVideoId = listenYouTubeResponseVideoId(initialResponse);
    const initialRequestVideoId = String(INITIAL_REQUEST.videoId || "");
    if (activeVideoId && initialResponseVideoId && activeVideoId !== initialResponseVideoId) {
      return null;
    }
    if (activeVideoId && !initialResponseVideoId &&
        initialRequestVideoId && activeVideoId !== initialRequestVideoId) {
      return null;
    }
    return initialResponse;
  }

  function listenYouTubePlayerErrorCode(status) {
    const api = listenYouTubeBridgePlayerApi();
    if (api && typeof api.getErrorCode === "function") {
      try {
        const code = api.getErrorCode();
        if (code !== undefined && code !== null && String(code) !== "0") {
          return String(code);
        }
      } catch (error) {}
    }
    const statusCode = status && (status.errorCode || status.error_code);
    return statusCode === undefined || statusCode === null ? "" : String(statusCode);
  }

  function pageErrorSnapshot() {
    const pageError = listenYouTubeBridgePageError();
    const snapshot = {
      pageErrorText: pageError
        ? listenYouTubeNormalizeErrorText(pageError.innerText || pageError.textContent || "")
        : ""
    };
    const response = listenYouTubePlayerResponse();
    const status = response && response.playabilityStatus;
    if (!status || typeof status !== "object") return snapshot;

    const errorScreen = status.errorScreen && typeof status.errorScreen === "object"
      ? status.errorScreen
      : {};
    const renderer = errorScreen.playerErrorMessageRenderer &&
      typeof errorScreen.playerErrorMessageRenderer === "object"
      ? errorScreen.playerErrorMessageRenderer
      : {};
    const reason = listenYouTubeStructuredText(renderer.reason) ||
      listenYouTubeStructuredText(status.reason);
    const subreason = listenYouTubeStructuredText(renderer.subreason) ||
      (Array.isArray(status.messages)
        ? listenYouTubeNormalizeErrorText(
            status.messages.map(listenYouTubeStructuredText).filter(Boolean).join("\n")
          )
        : "");
    const statusName = String(status.status || "").trim();
    const playerErrorCode = (
      (statusName && statusName.toUpperCase() !== "OK") ||
      reason ||
      subreason ||
      snapshot.pageErrorText
    ) ? listenYouTubePlayerErrorCode(status) : "";

    snapshot.upstreamStatus = statusName;
    snapshot.upstreamReason = reason;
    snapshot.upstreamSubreason = subreason;
    snapshot.upstreamHasCaptcha = Boolean(errorScreen.playerCaptchaViewModel);
    if (typeof status.playableInEmbed === "boolean") {
      snapshot.upstreamPlayableInEmbed = status.playableInEmbed;
    }
    if (playerErrorCode) snapshot.playerErrorCode = playerErrorCode;
    return snapshot;
  }
`
