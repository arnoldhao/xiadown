package wails

func listenYouTubeAdBlockScript() string {
	return `
(function() {
  "use strict";
  const host = String(window.location && window.location.hostname || "").toLowerCase();
  if (host !== "music.youtube.com" && host !== "www.youtube.com") return;
  const DISABLE_UNTIL_KEY = "__xiadownYouTubeAdBlockDisabledUntil";
  function disabledUntil() {
    try {
      const value = Number(window.localStorage && window.localStorage.getItem(DISABLE_UNTIL_KEY));
      return Number.isFinite(value) ? value : 0;
    } catch (error) {
      return 0;
    }
  }
  window.__xiadownDisableYouTubeAdBlock = function(durationMs) {
    try {
      const duration = Number.isFinite(Number(durationMs)) ? Math.max(0, Number(durationMs)) : 0;
      window.localStorage.setItem(DISABLE_UNTIL_KEY, String(Date.now() + duration));
    } catch (error) {}
  };
  if (disabledUntil() > Date.now()) {
    window.__xiadownYouTubeAdBlockerActive = false;
    return;
  }
  if (window.__xiadownYouTubeAdBlockerInstalled) return;
  window.__xiadownYouTubeAdBlockerInstalled = true;
  window.__xiadownYouTubeAdBlockerActive = true;

  const AD_KEYS = new Set([
    "adPlacements",
    "adSlots",
    "adBreaks",
    "adBreakHeartbeatParams",
    "playerAds"
  ]);
  const GLOBAL_NAMES = [
    "ytInitialPlayerResponse",
    "ytInitialData",
    "playerResponse",
    "ytInitialPlayerConfig"
  ];
  const PRUNE_NEEDLE = /"?(adPlacements|adSlots|adBreaks|adBreakHeartbeatParams|playerAds|reelWatchSequenceResponse)"?\s*[:\[]/i;

  function isObject(value) {
    return value !== null && typeof value === "object";
  }

  function isAdReelEntry(value) {
    return Boolean(
      value &&
      value.command &&
      value.command.reelWatchEndpoint &&
      value.command.reelWatchEndpoint.adClientParams &&
      value.command.reelWatchEndpoint.adClientParams.isAd === true
    );
  }

  function prune(value, depth, seen) {
    if (!isObject(value) || depth > 18) return value;
    if (!seen) seen = new WeakSet();
    if (seen.has(value)) return value;
    seen.add(value);

    if (Array.isArray(value)) {
      for (let index = value.length - 1; index >= 0; index -= 1) {
        const item = value[index];
        if (isAdReelEntry(item)) {
          value.splice(index, 1);
          continue;
        }
        prune(item, depth + 1, seen);
      }
      return value;
    }

    for (const key of Object.keys(value)) {
      if (AD_KEYS.has(key)) {
        try { delete value[key]; } catch (error) { value[key] = undefined; }
        continue;
      }
      prune(value[key], depth + 1, seen);
    }
    return value;
  }

  function shouldInspectText(text) {
    return typeof text === "string" && text.length > 16 && PRUNE_NEEDLE.test(text);
  }

  function pruneGlobal(name) {
    try {
      prune(window[name], 0);
    } catch (error) {}
  }

  function pruneKnownGlobals() {
    for (const name of GLOBAL_NAMES) pruneGlobal(name);
    try {
      if (window.ytcfg && window.ytcfg.data_) prune(window.ytcfg.data_, 0);
    } catch (error) {}
  }

  function patchGlobal(name) {
    try {
      const descriptor = Object.getOwnPropertyDescriptor(window, name);
      if (descriptor && descriptor.configurable === false) {
        pruneGlobal(name);
        return;
      }
      const originalGet = descriptor && descriptor.get;
      const originalSet = descriptor && descriptor.set;
      let stored = descriptor && Object.prototype.hasOwnProperty.call(descriptor, "value")
        ? descriptor.value
        : undefined;
      if (originalGet) {
        try { stored = originalGet.call(window); } catch (error) {}
      }
      prune(stored, 0);
      Object.defineProperty(window, name, {
        configurable: true,
        enumerable: descriptor ? descriptor.enumerable : true,
        get() {
          if (originalGet) {
            try {
              const value = originalGet.call(window);
              return prune(value, 0);
            } catch (error) {}
          }
          return stored;
        },
        set(next) {
          const clean = prune(next, 0);
          if (originalSet) {
            try {
              originalSet.call(window, clean);
              return;
            } catch (error) {}
          }
          stored = clean;
        }
      });
    } catch (error) {}
  }

  function patchJSONParse() {
    try {
      const original = JSON.parse;
      if (typeof original !== "function") return;
      JSON.parse = new Proxy(original, {
        apply(target, thisArg, args) {
          const result = Reflect.apply(target, thisArg, args);
          if (shouldInspectText(args && args[0])) prune(result, 0);
          return result;
        }
      });
    } catch (error) {}
  }

  function patchResponseJSON() {
    try {
      if (!window.Response || !Response.prototype || typeof Response.prototype.json !== "function") return;
      const original = Response.prototype.json;
      Response.prototype.json = new Proxy(original, {
        apply(target, thisArg, args) {
          const value = Reflect.apply(target, thisArg, args);
          if (!value || typeof value.then !== "function") return prune(value, 0);
          return value.then((json) => prune(json, 0));
        }
      });
    } catch (error) {}
  }

  function patchXHRJSON() {
    try {
      if (!window.XMLHttpRequest || !XMLHttpRequest.prototype) return;
      const open = XMLHttpRequest.prototype.open;
      const send = XMLHttpRequest.prototype.send;
      if (typeof open === "function") {
        XMLHttpRequest.prototype.open = function(method, url) {
          try { this.__xiadownYouTubeURL = String(url || ""); } catch (error) {}
          return open.apply(this, arguments);
        };
      }
      if (typeof send === "function") {
        XMLHttpRequest.prototype.send = function() {
          try {
            this.addEventListener("load", function() {
              try {
                if (this.responseType === "json" && isObject(this.response)) {
                  prune(this.response, 0);
                }
              } catch (error) {}
            }, { once: true });
          } catch (error) {}
          return send.apply(this, arguments);
        };
      }
    } catch (error) {}
  }

  for (const name of GLOBAL_NAMES) patchGlobal(name);
  patchJSONParse();
  patchResponseJSON();
  patchXHRJSON();
  pruneKnownGlobals();

  let pruneCount = 0;
  const pruneTimer = window.setInterval(function() {
    pruneCount += 1;
    pruneKnownGlobals();
    if (pruneCount >= 80) window.clearInterval(pruneTimer);
  }, 250);
  document.addEventListener("DOMContentLoaded", pruneKnownGlobals, { once: true });
})();
`
}
