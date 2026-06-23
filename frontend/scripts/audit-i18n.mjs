import fs from "node:fs";
import path from "node:path";
import ts from "typescript";

const rootDir = process.cwd();
const strict = process.argv.includes("--strict");
const jsonOnly = process.argv.includes("--json");
const pruneUnused = process.argv.includes("--prune-unused");
const fixEnglishStyle = process.argv.includes("--fix-english-style");

const localeDir = path.join(rootDir, "src", "shared", "i18n", "locales");
const sourceDir = path.join(rootDir, "src");
// Apple HIG alignment:
// - Keep user-visible text localizable, then test each supported language.
// - Prefer concise, direct labels; placeholders can hint input but must not be
//   the only localization surface for an interaction.
// - Use platform-consistent accessible labels for icon-only controls.
const englishStyleSkipKeys = new Set([
  "xiadown.welcome.readyHint",
  "xiadown.running.runningCountLine",
  "xiadown.running.queuedCountLine",
  "settings.language.option.en",
  "settings.language.option.zhCN",
  "settings.language.option.zhTW",
  "settings.language.option.jaJP",
  "settings.language.option.koKR",
  "settings.language.option.es419",
  "settings.language.option.ptBR",
  "settings.language.option.idID",
  "settings.language.option.viVN",
  "settings.appSessions.youtubeAccountFallbackName",
]);
const englishStylePreservePhrases = [
  "XiaDown",
  "DreamApp",
  "Pet Gallery",
  "Sniff Desk",
  "Codex",
  "codex-pets.net",
  "codexpet.xyz",
  "petdex.dev",
  "pet.json",
  "spritesheet.webp",
  "hatch-pet",
  "Apple",
  "macOS",
  "iOS",
  "Chrome",
  "Chromium",
  "Edge",
  "Brave",
  "Vivaldi",
  "Opera GX",
  "Yandex Browser",
  "Bun",
  "FFmpeg",
  "GitHub",
  "YouTube",
  "Bilibili",
  "TikTok",
  "Douyin",
  "Xiaohongshu",
  "Twitter",
  "Markdown",
  "Vibe Coding",
  "WebSocket",
  "JSON",
  "API",
  "VIP",
  "HTTP",
  "HTTPS",
  "URL",
  "PDF",
  "MIME",
  "ASS",
  "SSA",
  "SRT",
  "VTT",
  "TTML",
  "FCPXML",
  "CRF",
  "BOM",
  "UTC",
  "R&B",
  "N/A",
  "OS",
  "Nix",
  "Wails",
  "yt-dlp",
  "User-Agent",
  "Accept-Language",
  "Content-Type",
];
const englishStylePreserveWords = new Set(englishStylePreservePhrases.filter((item) => !item.includes(" ")));
const englishStyleAlwaysLowerWords = new Set([
  "a",
  "an",
  "the",
  "and",
  "or",
  "but",
  "for",
  "nor",
  "at",
  "by",
  "to",
  "from",
  "of",
  "in",
  "on",
  "off",
  "up",
  "out",
  "as",
  "is",
  "are",
  "be",
  "been",
  "being",
  "into",
  "over",
  "with",
  "without",
  "per",
  "same",
  "after",
  "before",
  "when",
  "while",
  "that",
  "this",
  "these",
  "those",
  "all",
  "no",
  "not",
  "now",
  "than",
  "once",
  "only",
  "via",
  "max",
  "min",
]);
const englishTitleCaseLowerWords = new Set([
  "a",
  "an",
  "the",
  "and",
  "or",
  "but",
  "nor",
  "for",
  "so",
  "yet",
  "as",
  "at",
  "by",
  "en",
  "from",
  "in",
  "of",
  "off",
  "on",
  "per",
  "to",
  "with",
  "up",
  "via",
]);
const englishSentenceBoundaryChars = new Set([".", "!", "?"]);
// Apple HIG defines title-style capitalization for compact structural elements
// like buttons, menu items, toolbar actions, tabs, navigation, page titles,
// section headings, and table columns. Descriptive text stays sentence-style.
const englishTitleStyleExplicitPatterns = [
  /^common\.(?:cancel|close)$/,
  /^common\.realtimeUnavailableTitle$/,
  /^library\.(?:status|progress|progressDetail)\./,
  /^library\.download\.subtitle\./,
  /^settings\.appSessions\.(?:headerRoot|openSite|connect|reconnect|clear|finish|cancel|status|account|handle|membership|badges|cookies|domains|lastVerifiedAt|actions|capabilities|connectTitle|openTitle|browserStatus|currentCookies|savedCookies|siteDetailTitle|youtubeIdentity|youtubeMembership|youtubeDomains|bilibiliIdentity|bilibiliMembership|bilibiliVip|bilibiliAnnualVip|bilibiliActiveVip|socialIdentity|genericIdentity|accountFallbackName|disconnectedName|signIn|signOut|verifyStatus|lastLoginAt|expiresAt|youtubeAccountFallbackName|youtubeDisconnectedName|youtubeSignIn|youtubeSignOut|youtubeVerifyStatus|youtubeLastLoginAt|youtubeExpiresAt|item\.)/,
  /^xiadown\.(?:views|actions|sidebar)\./,
  /^xiadown\.running\.(?:title|loading|overviewTitle|taskCount|overallProgress|throughput|elapsed|runningCount|queuedCount|downloadSpeed|transcodeSpeed|downloadBadge|transcodeBadge|source|createdAt|localSource|stage|status|eta|cancelConfirmTitle|stageLabels\.)/,
  /^xiadown\.completed\.(?:title|records|outputs|info|taskDetail|taskDtoTitle|openTaskDto|copyDownloadUrl|copyFailed|taskDataFields\.|fileDetail|searchTasks|searchFiles|searchFilter|total|taskCountLabel|fileCountLabel|perPage|itemUnit|page|taskStatus|source|fileType|fileFormat|fileInfo|selectFiles|selectAll|clearSelection|verifyFiles|clearMissingFiles|selectionSummary|selectionUnit|lineUnit|lineCount|deleteFiles|deleteTaskTitle|deleteTasksTitle|deleteFileTitle|deleteFilesTitle|previousPage|nextPage|preview|updatedAt|videoCount|subtitleCount|imageCount|type|resolution|frameRate|duration|channels|dpi|codec|bitrate|videoBitrate|audioBitrate|fileSize|originalFormat|succeeded|failed|canceled)/,
  /^xiadown\.welcome\.(?:title|language|proxy|dependencies|dependencyStatus|latestVersion|bgmOff|bgmOn|enterApp|installAll|installing|proxyNone|proxySystem|readyTitle|stage|systemProxyTitle|theme)/,
  /^xiadown\.settings\.(?:title|tabs\.|startup|tray|menuBar|syncedLyrics|romanizedLyrics|pinyinLyrics|downloadDirectory|ytdlpConcurrentDownloads|ytdlpConcurrentFragments|sniffBrowser|refreshBrowsers|browserData|browserDataSize|browserDataOpen|browserDataClear|language|logLevel|themePack|accent|accentColor|fontFamily|fontSize|colorScheme|appearanceMode|sidebarStyle|systemProxy|manualProxy|noProxy|proxy|host|port|username|password|timeout|scheme|noProxyList|status|checking|notConfigured|unavailable|systemSource|vpnSource|editProxy|proxyDialogTitle|otherSoftware|menuBarOptions\.|colorSchemeOptions\.|sidebarStyleOptions\.|accentOptions\.|equalizer\.(?:title|enable|preset|bands|preamp|reset|retry|openSettings|custom|status\.|presets\.))/,
  /^xiadown\.dependencies\.(?:title|installed|missing|invalid|idle|latestVersion|currentVersion|execPath|noRemoteVersionInfo|reinstall|downloading|extracting|verifying|installing|missingDependency)/,
  /^xiadown\.dialogs\.(?:downloadTitle|transcodeTitle|dependenciesRequiredTitle|requestDownload|quickMode|customMode|preset|format|quality|qualityBest|qualityAudio|selectFormat|size|container|codec|scaleOriginal|scaleCustom|subtitles|noSubtitle|noTranscode|keepOnlyTranscodedFile|parse|parseAgain|currentStatus|useAppSession|appSessionAvailable|appSessionUnavailable|noAvailableAppSession|appSessionNotConfigured|appSessionCanEnable|appSessionCookiesDownload|modifyLink|modifyFile|path|fileAddress|inspectingFile|fileInspectFailed|noCompatibleTranscodePreset)$/,
  /^xiadown\.dialogs\.formatGroup(?:Video|Audio)$/,
  /^xiadown\.listen\.(?:hush|muse|linger|localEmptyAction|localLoading|localRefresh|localClearMissing|localModified|randomStation|searchLive|searchOnline|searchLocal|searchSongs|searchArtists|searchPlaylists|source|group|liveStations|liveLoading|addChannel|editChannel|removeChannel|manageColumns|channelURL|channelTitle|channelName|channelColumn|noColumn|channelDescription|channelThumbnail|addColumn|editColumn|saveColumn|removeColumn|builtInColumns|customColumns|readonlyColumn|noColumns|retry|libraryArtists|likedMusic|playlistType|groupRecommendations|groupRadio|shelf|liveBadge|liveStatus|adBadge|browse|upNext|play|pause|stop|loading|idleStatus|nowPlaying|playingStatus|pausedStatus|loadingStatus|errorStatus|errorCodeLabel|previous|next|playbackMode|playMode|mute|unmute|volume|seek|favorite$|collapseList|openList|onlineLoading|openConnections|refresh|contentEmpty|searchLoading|loadMore|seeAll|playAll|playNext|addToQueue|addToLibrary|shuffleAll|clearQueue|undoQueue|redoQueue|editQueue|doneQueue|removeFromQueue|moveQueueItemUp|moveQueueItemDown|airPlay|video|fitWindow|fitVideo|lockVideo|unlockVideo|noVideo|lyrics|lyricsEmpty|more|openPage|copyLink|artistShuffle|artistMix|artistSubscribe|artistUnsubscribe|artistLoading|playlistLoading|savePlaylist|removePlaylist)/,
  /^xiadown\.whatsNew\.title$/,
  /^xiadown\.about\.(?:updateStatus|currentVersion|latestVersion|status|latestOk|latestAvailable|latestFailed|viewChangelog|viewReleaseNotes|noReleaseNotes|releaseNotesClose|checkUpdates|recheck|downloadAndInstall|restartAfterUpdate|downloading|installing|craftedBy|contact|feedback|sendFeedback|email|twitter|website|github|dreamCreator|hush)/,
  /^xiadown\.common\.(?:systemDefault|current|customColor|light|dark|auto|followSystem|ready|unknown|browse|colorOptions\.)/,
  /^xiadown\.petGallery\.(?:title|breadcrumb|gallery|detail|showMore|importAction|importTitle|importFailedTitle|petPackageFilter|exportTitle|exportAction|deleteAction|deleteTitle|activePet|setActive|loadingManifest|scopeLabel|sizeLabel|gridLabel|cellLabel|previewAnimation|scopes\.|origins\.localImport|animations\.|generationGuide\.action|generationGuide\.title|generationGuide\.steps\.[0-9]+\.title|generationGuide\.greatPetTitle|importDialog\.(?:online|local|onlineSite|browse|browserStatus|fileName|path|petName|importSelected|importedPets|finish|browserStatuses\.)|localPets)/,
];
const englishSentenceStyleExplicitPatterns = [
  /(?:^|\.)(?:description|subtitle|hint|placeholder|message|empty|prompt|toast)$/i,
  /^common\.realtimeUnavailableDescription$/,
  /^settings\.appSessions\.(?:searchPlaceholder|searchEmpty|empty|providerUnsupported|clearConfirmTitle|clearConfirmDescription|clearError|loginError|noCookiesRead|browserMissing|browserSessionEnded|sessionMissing|connected|expired|disconnected|unsupported)$/,
  /^xiadown\.running\.(?:empty|playgroundHint|downloadSpeedLine|transcodeSpeedLine|progressFallback|cancelConfirmDescription|units\.)/,
  /^xiadown\.completed\.(?:emptyTasks|emptyLibrary|emptyFiles|detailEmpty|downloadUrlCopied|verifyFilesMissingToast|verifyFilesValidToast|clearMissingFilesRemovedToast|clearMissingFilesNoneToast|fileMaintenanceFailed|deleteTaskMessage|deleteTasksMessage|deleteFileMessage|deleteFilesMessage|noPreview|noSelectedTask|noSelectedFile|taskNoFiles)$/,
  /^xiadown\.welcome\.(?:subtitle|noProxyDescription|systemProxyDescription)$/,
  /^xiadown\.settings\.(?:proxyDialogHint|aboutText|generalDescription|appearanceDescription|proxyDescription|systemProxyEmpty|proxyTestSucceeded|proxyTestFailed|ytdlpConcurrentDownloadsHelp|ytdlpConcurrentFragmentsHelp|equalizer\.(?:macOSOnly|messages\.))$/,
  /^xiadown\.dependencies\.subtitle$/,
  /^xiadown\.dialogs\.(?:dependenciesRequiredDescription|downloadPlaceholder|parseFailedWithoutAppSession|parseFailedWithAppSession|nameHint)$/,
  /^xiadown\.listen\.(?:subtitle|localEmpty|localEmptyPrompt|liveUnavailable|liveEmpty|confirmRemoveChannel|customCatalogSummary|addChannelPlaceholder|channelVideoIdRequired|channelNotLive|channelTitleRequired|confirmRemoveColumn|customColumnPlaceholder|columnTitleRequired|columnAlreadyExists|userCatalogSaveFailed|playlistTrackCount|playlistTrackCountMore|idleSubtitle|favoriteUnavailable|selectStation|onlineHint|onlineUnavailable|onlineConnectionPrompt|onlineAuthRequired|onlineAuthExpired|onlineNetworkUnavailable|onlineServiceUnavailable|onlineEmpty|searchUnavailable|searchEmpty|linkCopied|artistUnavailable|artistEmpty|playlistEmpty|upNextEmpty|playerUnavailable)$/,
  /^xiadown\.whatsNew\.empty$/,
  /^xiadown\.about\.(?:dreamCreatorDescription|hushDescription)$/,
  /^xiadown\.petGallery\.(?:empty|importInvalid|importSucceeded|errors\.|exportSucceeded|deleteMessage|deleteSucceeded|generationGuide\.steps\.[0-9]+\.description|generationGuide\.greatPetTips\.|importDialog\.(?:description|onlineDescription|validationReady|importedEmpty))$/,
];
const englishTitleStyleHeuristicPattern = /(\.button$|\.action$|columns\.|Tabs\.|sections\.|Section$|dialogTitle$|pickTitle$)/;
const englishTitleStyleSentencePattern = /\?|\.{1,3}$|\b(is|are|was|were|am|can't|cannot|need|needs|shows?\s+up|please)\b/i;
const englishImplementationDetailFindings = [
  /\bpolling\b/i,
  /\bWebSocket\b/i,
  /\bpipeline\b/i,
  /\bmanifest-driven\b/i,
  /\byt-dlp version\b/i,
  /\bFFmpeg transcode\b/i,
  /\bofficial account API integration\b/i,
  /\bplayer service\b/i,
  /\bonline import session\b/i,
];
const zhImplementationDetailFindings = [
  /依赖/,
  /依賴/,
  /相依/,
  /轮询/,
  /輪詢/,
  /WebSocket/i,
  /pipeline/i,
  /管线/,
  /流水线/,
  /更新清单/,
  /yt-dlp\s*版本/i,
  /官方账号\s*API/i,
  /官方帳號\s*API/i,
  /播放器服务/,
  /播放器服務/,
  /在线导入会话/,
  /線上匯入(?:工作階段|會話)/,
  /浏览器会话/,
  /瀏覽器(?:工作階段|會話)/,
  /登录会话/,
  /登入(?:工作階段|會話)/,
];
const zhGlossaryReplacements = [
  ["Sidebar", "侧边栏"],
  ["Connector", "连接"],
  ["External Tools", "依赖"],
  ["Runtime", "运行时"],
  ["Mono", "单语"],
  ["Bilingual", "双语"],
  ["Builtin", "内置"],
  ["Topic", "主题"],
  ["Toast", "提示"],
  ["Cloud Dancer", "暖云白"],
];
const zhAllowedEnglishKeyPatterns = [
  /^settings\.language\.option\./,
  /^settings\.appSessions\.youtubeAccountFallbackName$/,
  /^xiadown\.running\.units\.bytesPerSecond$/,
  /^xiadown\.running\.units\.framesPerSecond$/,
  /^settings\.appSessions\.item\./,
  /^xiadown\.welcome\.readyHint$/,
  /^xiadown\.petGallery\./,
];
const runningFramesPerSecondUnitKey = "xiadown.running.units.framesPerSecond";
const runningFramesPerSecondUnitValue = "{value} FPS";
const zhAllowedEnglishTokens = new Set([
  "PNG",
  "ZIP",
  "DPI",
  "VPN",
  "px",
  "x",
  "yt-dlp",
  "ffmpeg",
  "bun",
  "GitHub",
  "Issues",
  "YouTube",
  "Hush",
  "Music",
  "lo-fi",
  "Lo-fi",
  "Lo-Fi",
  "FM",
  "Mix",
  "YT-DLP",
  "FFMPEG",
  "BUN",
  "AI",
  "EP",
  "API",
  "Markdown",
  "Vibe",
  "Coding",
  "AirPlay",
  "macOS",
  "DreamCreator",
  "DreamApp",
  "WebM",
  "WebView",
  "Cookie",
  "Cookies",
  "cookies",
  "WASD",
]);
const knownNativeLanguageLabels = {
  "settings.language.option.en": "English",
  "settings.language.option.zhCN": "简体中文",
  "settings.language.option.zhTW": "繁體中文",
  "settings.language.option.jaJP": "日本語",
  "settings.language.option.koKR": "한국어",
  "settings.language.option.es419": "Español (LatAm)",
  "settings.language.option.ptBR": "Português (BR)",
  "settings.language.option.idID": "Bahasa Indonesia",
  "settings.language.option.viVN": "Tiếng Việt",
};
const hardcodedEnglishAllowedText = new Set([
  "HTTP",
  "HTTPS",
  "SOCKS5",
  "Listen",
  "DreamApp",
]);
const hardcodedEnglishUserFacingPropertyNames = new Set([
  "label",
  "title",
  "description",
  "message",
  "placeholder",
]);
const hardcodedEnglishPropertyFileSkips = [
  /(?:^|\/).*\.test\.[tj]sx?$/,
  /(?:^|\/)listen\/api\.ts$/,
  /(?:^|\/)listen\/catalog\.ts$/,
];
const hardcodedChineseFileSkips = [
  // Listen catalog entries preserve external channel titles and descriptions.
  /(?:^|\/)listen\/catalog\.ts$/,
];
const hardcodedChineseAllowedLinePatterns = [
  /LISTEN_RELEASE_YEAR_ARTIST_PATTERN/,
  /"专辑"|"專輯"|"单曲"|"單曲"/,
];
const hardcodedEnglishSkipAttributes = new Set([
  "className",
  "contentClassName",
  "fallbackClassName",
  "petClassName",
  "style",
  "id",
  "key",
  "src",
  "href",
  "to",
  "type",
  "role",
  "target",
  "rel",
  "value",
  "defaultValue",
  "name",
  "method",
  "topic",
  "window",
  "variant",
  "size",
  "align",
  "side",
  "sideOffset",
  "width",
  "height",
  "viewBox",
  "fill",
  "stroke",
  "strokeWidth",
  "strokeLinecap",
  "strokeLinejoin",
  "d",
  "xmlns",
  "tabIndex",
  "aria-hidden",
]);

function flatten(input, prefix = "", output = {}) {
  for (const [key, value] of Object.entries(input)) {
    const nextKey = prefix ? `${prefix}.${key}` : key;
    if (value && typeof value === "object" && !Array.isArray(value)) {
      flatten(value, nextKey, output);
    } else {
      output[nextKey] = String(value);
    }
  }
  return output;
}

function collectLocaleLeafTypeViolations(input, locale, prefix = "", output = []) {
  for (const [key, value] of Object.entries(input)) {
    const nextKey = prefix ? `${prefix}.${key}` : key;
    if (value && typeof value === "object" && !Array.isArray(value)) {
      collectLocaleLeafTypeViolations(value, locale, nextKey, output);
      continue;
    }
    if (typeof value !== "string") {
      output.push({
        locale,
        key: nextKey,
        valueType: Array.isArray(value) ? "array" : typeof value,
      });
    }
  }
  return output;
}

function walk(dir, output = []) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const next = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      if (next.includes(`${path.sep}bindings`) || next.includes(`${path.sep}shared${path.sep}i18n`)) {
        continue;
      }
      walk(next, output);
      continue;
    }
    if (!next.endsWith(".ts") && !next.endsWith(".tsx")) {
      continue;
    }
    output.push(next);
  }
  return output;
}

function relative(filePath) {
  return path.relative(rootDir, filePath).split(path.sep).join("/");
}

function isI18nCallExpression(node) {
  if (!ts.isCallExpression(node)) {
    return false;
  }
  if (ts.isIdentifier(node.expression)) {
    return node.expression.text === "t" || node.expression.text === "translate";
  }
  if (ts.isPropertyAccessExpression(node.expression)) {
    return node.expression.name.text === "t" || node.expression.name.text === "translate";
  }
  return false;
}

function isExplicitLanguageArg(node, sourceFile) {
  const text = node.getText(sourceFile).trim();
  return /(^language\b)|(\blanguage\b\s+as\s+)|(as\s+"en"\s*\|\s*"zh-CN"\s*\|\s*"zh-TW")|(^"en"$)|(^"zh-CN"$)|(^"zh-TW"$)|(^"ja-JP"$)|(^"ko-KR"$)|(^"es-419"$)|(^"pt-BR"$)|(^"id-ID"$)|(^"vi-VN"$)/.test(text);
}

function splitWord(word) {
  const parts = [];
  let buffer = "";
  for (const char of word) {
    if (char === "-" || char === "/") {
      if (buffer) {
        parts.push(buffer);
      }
      parts.push(char);
      buffer = "";
      continue;
    }
    buffer += char;
  }
  if (buffer) {
    parts.push(buffer);
  }
  return parts;
}

function maskEnglishPreservePhrases(value) {
  let text = value;
  englishStylePreservePhrases.forEach((phrase, index) => {
    const escaped = phrase.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
    text = text.replace(new RegExp(escaped, "g"), `@@${index}@@`);
  });
  return text;
}

function unmaskEnglishPreservePhrases(value) {
  return value.replace(/@@(\d+)@@/g, (_, index) => englishStylePreservePhrases[Number(index)] ?? "");
}

function isEnglishStylePreservedWord(segment) {
  return (
    englishStylePreserveWords.has(segment) ||
    /^[A-Z0-9]+(?:_[A-Z0-9]+)*$/.test(segment) ||
    /^[a-z0-9]+(?:_[a-z0-9]+)+$/.test(segment) ||
    /^[A-Za-z]+=[A-Za-z0-9_-]+$/.test(segment) ||
    (/^[A-Za-z0-9-]+$/.test(segment) && /[A-Z].*[A-Z]/.test(segment) && /[a-z]/.test(segment))
  );
}

function splitAffixes(token) {
  const leading = token.match(/^[^A-Za-z{]*/)?.[0] ?? "";
  const trailing = token.match(/[^A-Za-z}]*$/)?.[0] ?? "";
  return {
    leading,
    trailing,
    core: token.slice(leading.length, token.length - trailing.length),
  };
}

function normalizeEnglishTitleSegment(segment, isFirst, isLast) {
  if (segment === "-" || segment === "/") {
    return segment;
  }
  if (!segment || !/[A-Za-z]/.test(segment)) {
    return segment;
  }
  if (isEnglishStylePreservedWord(segment)) {
    return segment;
  }
  if (segment.includes("/")) {
    const parts = splitWord(segment);
    const wordIndexes = parts.map((part, index) => (part !== "/" ? index : null)).filter((index) => index !== null);
    return parts
      .map((part, index) => {
        const position = wordIndexes.indexOf(index);
        if (position === -1) {
          return part;
        }
        return normalizeEnglishTitleSegment(part, isFirst && position === 0, isLast && position === wordIndexes.length - 1);
      })
      .join("");
  }
  if (segment.includes("-")) {
    const parts = splitWord(segment);
    const wordIndexes = parts.map((part, index) => (part !== "-" ? index : null)).filter((index) => index !== null);
    return parts
      .map((part, index) => {
        const position = wordIndexes.indexOf(index);
        if (position === -1) {
          return part;
        }
        return normalizeEnglishTitleSegment(part, isFirst && position === 0, isLast && position === wordIndexes.length - 1);
      })
      .join("");
  }
  const lower = segment.toLowerCase();
  if (!isFirst && !isLast && englishTitleCaseLowerWords.has(lower)) {
    return lower;
  }
  return `${lower.charAt(0).toUpperCase()}${lower.slice(1)}`;
}

function shouldUseEnglishTitleCase(key, value) {
  if (englishStyleSkipKeys.has(key) || typeof value !== "string" || !/[A-Za-z]/.test(value)) {
    return false;
  }
  if (englishSentenceStyleExplicitPatterns.some((pattern) => pattern.test(key))) {
    return false;
  }
  if (englishTitleStyleExplicitPatterns.some((pattern) => pattern.test(key))) {
    return !englishTitleStyleSentencePattern.test(value);
  }
  if (!englishTitleStyleHeuristicPattern.test(key)) {
    return false;
  }
  if (englishTitleStyleSentencePattern.test(value)) {
    return false;
  }
  return true;
}

function normalizeEnglishSentenceValue(key, value) {
  if (englishStyleSkipKeys.has(key) || typeof value !== "string" || !/[A-Za-z]/.test(value)) {
    return value;
  }
  const text = maskEnglishPreservePhrases(value);
  const tokens = text.match(/\{[^}]+\}|@@\d+@@|[A-Za-z][A-Za-z0-9'/-]*|[^A-Za-z{@]+|[@][^@]+[@]?/g) ?? [text];
  let sentenceStart = true;
  const output = tokens
    .map((token) => {
      if (token.startsWith("@@")) {
        sentenceStart = false;
        return token;
      }
      if (token.startsWith("{") && token.endsWith("}")) {
        sentenceStart = false;
        return token;
      }
      if (!/[A-Za-z]/.test(token)) {
        const trimmed = token.trimEnd();
        const lastChar = trimmed.charAt(trimmed.length - 1);
        if (englishSentenceBoundaryChars.has(lastChar)) {
          sentenceStart = true;
        }
        return token;
      }
      const leading = token.match(/^[^A-Za-z{]*/)?.[0] ?? "";
      const trailing = token.match(/[^A-Za-z}]*$/)?.[0] ?? "";
      const core = token.slice(leading.length, token.length - trailing.length);
      const segments = splitWord(core);
      let nextSentenceStart = sentenceStart;
      const normalized = segments
        .map((segment) => {
          if (segment === "-" || segment === "/") {
            return segment;
          }
          if (!segment || !/[A-Za-z]/.test(segment)) {
            return segment;
          }
          if (
            isEnglishStylePreservedWord(segment)
          ) {
            nextSentenceStart = false;
            return segment;
          }
          if (/^[A-Z][a-z0-9]+(?:'[A-Za-z]+)?$/.test(segment) || /^[a-z][a-z0-9]+(?:'[A-Za-z]+)?$/.test(segment)) {
            const lower = segment.toLowerCase();
            const nextValue = nextSentenceStart ? `${lower.charAt(0).toUpperCase()}${lower.slice(1)}` : (englishStyleAlwaysLowerWords.has(lower) ? lower : lower);
            nextSentenceStart = false;
            return nextValue;
          }
          if (nextSentenceStart && /^[a-z]/.test(segment)) {
            nextSentenceStart = false;
            return `${segment.charAt(0).toUpperCase()}${segment.slice(1)}`;
          }
          nextSentenceStart = false;
          return segment;
        })
        .join("");
      sentenceStart = false;
      return `${leading}${normalized}${trailing}`;
    })
    .join("");
  return unmaskEnglishPreservePhrases(output);
}

function normalizeEnglishTitleValue(key, value) {
  if (englishStyleSkipKeys.has(key) || typeof value !== "string" || !/[A-Za-z]/.test(value)) {
    return value;
  }
  const text = maskEnglishPreservePhrases(value);
  const parts = text.split(/(\s+)/);
  const wordIndexes = parts
    .map((part, index) => {
      if (!part || /^\s+$/.test(part)) {
        return null;
      }
      if (/^@@\d+@@$/.test(part) || (part.startsWith("{") && part.endsWith("}"))) {
        return index;
      }
      const { core } = splitAffixes(part);
      return /[A-Za-z]/.test(core) ? index : null;
    })
    .filter((index) => index !== null);
  const output = parts
    .map((part, index) => {
      if (!part || /^\s+$/.test(part) || /^@@\d+@@$/.test(part) || (part.startsWith("{") && part.endsWith("}"))) {
        return part;
      }
      const position = wordIndexes.indexOf(index);
      if (position === -1) {
        return part;
      }
      const { leading, trailing, core } = splitAffixes(part);
      return `${leading}${normalizeEnglishTitleSegment(core, position === 0, position === wordIndexes.length - 1)}${trailing}`;
    })
    .join("");
  return unmaskEnglishPreservePhrases(output);
}

function normalizeEnglishLocaleValue(key, value) {
  if (shouldUseEnglishTitleCase(key, value)) {
    return normalizeEnglishTitleValue(key, value);
  }
  return normalizeEnglishSentenceValue(key, value);
}

function normalizeZhGlossaryValue(value) {
  if (typeof value !== "string") {
    return value;
  }
  let nextValue = value;
  for (const [from, to] of zhGlossaryReplacements) {
    nextValue = nextValue.split(from).join(to);
  }
  return nextValue;
}

function stripLocalePlaceholders(value) {
  return String(value)
    .replace(/\{[^}]+\}/g, "")
    .replace(/`[^`]*`/g, "")
    .replace(/#[0-9a-fA-F]{3,8}\b/g, "");
}

function isZhEnglishAllowedKey(key) {
  return zhAllowedEnglishKeyPatterns.some((pattern) => pattern.test(key));
}

function collectUnexpectedZhEnglishTokens(key, value) {
  if (isZhEnglishAllowedKey(key)) {
    return [];
  }
  const text = stripLocalePlaceholders(value);
  const tokens = (text.match(/[A-Za-z][A-Za-z0-9._+-]*/g) ?? [])
    .map((token) => token.replace(/[.。！？!?,，:：;；]+$/g, ""));
  return [...new Set(tokens.filter((token) => !zhAllowedEnglishTokens.has(token)))];
}

function visibleTextValue(value) {
  return String(value).replace(/\s+/g, " ").trim();
}

function looksLikeNonUserFacingVisibleText(value) {
  const text = visibleTextValue(value);
  return (
    !text ||
    !/[A-Za-z]/.test(text) ||
    hardcodedEnglishAllowedText.has(text) ||
    /^https?:/i.test(text) ||
    /^data:/i.test(text) ||
    /^#/.test(text) ||
    /^\/[A-Za-z0-9_./-]*$/.test(text) ||
    /^[a-z0-9_:/.[\]%-]+(?:\s+[a-z0-9_:/.[\]%-]+)*$/.test(text) ||
    /^[A-Z0-9_]+$/.test(text) ||
    /^[a-z]+(?:-[a-z]+)+$/.test(text) ||
    /^[a-z]+(?:\.[a-z0-9_.-]+)+$/.test(text) ||
    /^\d+(?:\.\d+)?(?:px|rem|%|s|ms)?$/.test(text)
  );
}

function hardcodedEnglishPropertyName(name) {
  if (ts.isIdentifier(name) || ts.isStringLiteral(name) || ts.isNumericLiteral(name)) {
    return name.text;
  }
  return "";
}

function shouldSkipHardcodedEnglishPropertyFile(filePath) {
  const rel = relative(filePath);
  return hardcodedEnglishPropertyFileSkips.some((pattern) => pattern.test(rel));
}

function shouldSkipHardcodedChineseFile(filePath) {
  const rel = relative(filePath);
  return hardcodedChineseFileSkips.some((pattern) => pattern.test(rel));
}

function expressionContainsLocalizedText(node, sourceFile) {
  if (!node) {
    return false;
  }
  if (isI18nCallExpression(node)) {
    return true;
  }
  if (ts.isIdentifier(node)) {
    return node.text === "text";
  }
  if (ts.isPropertyAccessExpression(node)) {
    if (node.name.text === "text") {
      return true;
    }
    return expressionContainsLocalizedText(node.expression, sourceFile);
  }
  let found = false;
  ts.forEachChild(node, (child) => {
    if (!found && expressionContainsLocalizedText(child, sourceFile)) {
      found = true;
    }
  });
  return found;
}

function collectEnglishImplementationDetails(en) {
  return Object.entries(en)
    .map(([key, value]) => {
      const matches = englishImplementationDetailFindings
        .filter((pattern) => pattern.test(value))
        .map((pattern) => pattern.source);
      return { key, value, matches };
    })
    .filter((item) => item.matches.length > 0);
}

function collectZhImplementationDetails(zh) {
  return Object.entries(zh)
    .map(([key, value]) => {
      const matches = zhImplementationDetailFindings
        .filter((pattern) => pattern.test(value))
        .map((pattern) => pattern.source);
      return { key, value, matches };
    })
    .filter((item) => item.matches.length > 0);
}

function isChineseLocale(locale) {
  return /^zh(?:-|$)/i.test(locale);
}

function collectPlaceholders(value) {
  return [...new Set([...String(value).matchAll(/\{([A-Za-z0-9_]+)\}/g)].map((match) => match[1]))].sort();
}

function collectLocaleParityViolations(localeFlats, baseLocale, baseValues) {
  const baseKeys = new Set(Object.keys(baseValues));
  const violations = [];
  for (const [locale, values] of Object.entries(localeFlats)) {
    if (locale === baseLocale) {
      continue;
    }
    const keys = new Set(Object.keys(values));
    for (const key of baseKeys) {
      if (!keys.has(key)) {
        violations.push({ locale, type: "missing", key });
      }
    }
    for (const key of keys) {
      if (!baseKeys.has(key)) {
        violations.push({ locale, type: "extra", key });
      }
    }
  }
  return violations;
}

function collectLocalePlaceholderViolations(localeFlats, baseLocale, baseValues) {
  const violations = [];
  for (const [locale, values] of Object.entries(localeFlats)) {
    if (locale === baseLocale) {
      continue;
    }
    for (const [key, baseValue] of Object.entries(baseValues)) {
      if (!(key in values)) {
        continue;
      }
      const expected = collectPlaceholders(baseValue);
      const actual = collectPlaceholders(values[key]);
      if (expected.join("\u0000") !== actual.join("\u0000")) {
        violations.push({
          locale,
          key,
          value: values[key],
          expected,
          actual,
        });
      }
    }
  }
  return violations;
}

function collectRunningFPSUnitViolations(localeFlats) {
  return Object.entries(localeFlats)
    .map(([locale, values]) => ({
      locale,
      key: runningFramesPerSecondUnitKey,
      value: values[runningFramesPerSecondUnitKey] ?? "",
      expected: runningFramesPerSecondUnitValue,
    }))
    .filter((item) => item.value !== item.expected);
}

function filterLocaleTree(input, usedSet, prefix = "") {
  const output = {};
  for (const [key, value] of Object.entries(input)) {
    const nextKey = prefix ? `${prefix}.${key}` : key;
    if (value && typeof value === "object" && !Array.isArray(value)) {
      const nextValue = filterLocaleTree(value, usedSet, nextKey);
      if (Object.keys(nextValue).length > 0) {
        output[key] = nextValue;
      }
      continue;
    }
    if (usedSet.has(nextKey)) {
      output[key] = value;
    }
  }
  return output;
}

function mapLocaleTree(input, mapper, prefix = "") {
  const output = {};
  for (const [key, value] of Object.entries(input)) {
    const nextKey = prefix ? `${prefix}.${key}` : key;
    if (value && typeof value === "object" && !Array.isArray(value)) {
      output[key] = mapLocaleTree(value, mapper, nextKey);
      continue;
    }
    output[key] = mapper(nextKey, String(value));
  }
  return output;
}

const baseLocale = "en";
const localeFileNames = fs.readdirSync(localeDir).filter((fileName) => fileName.endsWith(".json")).sort();
const localeSources = Object.fromEntries(
  localeFileNames.map((fileName) => [
    path.basename(fileName, ".json"),
    JSON.parse(fs.readFileSync(path.join(localeDir, fileName), "utf8")),
  ])
);
let enSource = localeSources[baseLocale];
if (!enSource) {
  throw new Error(`Missing base locale file: ${baseLocale}.json`);
}
if (fixEnglishStyle) {
  enSource = mapLocaleTree(enSource, normalizeEnglishLocaleValue);
  localeSources[baseLocale] = enSource;
  fs.writeFileSync(path.join(localeDir, "en.json"), `${JSON.stringify(enSource, null, 2)}\n`);
}
const localeFlats = Object.fromEntries(
  Object.entries(localeSources).map(([locale, source]) => [locale, flatten(source)])
);
const en = flatten(enSource);
const zh = localeFlats["zh-CN"] ?? {};
const chineseLocales = Object.entries(localeFlats).filter(([locale]) => isChineseLocale(locale));
const files = walk(sourceDir);
const localeKeys = new Set(Object.values(localeFlats).flatMap((values) => Object.keys(values)));
const sortedLocaleKeys = [...localeKeys];

const usedKeys = new Set();
const hardcodedChinese = [];
const hardcodedEnglish = [];
const i18nCopyNamingViolations = [];
const unresolvedDynamicKeys = [];
const i18nCallViolations = [];
const localizedCasingViolations = [];
const localizedCasingMethods = new Set(["toUpperCase", "toLocaleUpperCase"]);
const localizedCasingFunctionPattern = /^(?:capitalize|titleCase|toTitleCase|uppercase)$/i;
const keyPattern = /\b(?:t|translate)\(\s*(["'`])([^"'`]+)\1/g;
const propertyKeyPattern = /\b(?:labelKey|descriptionKey|reasonKey)\s*:\s*(["'`])([^"'`]+)\1/g;
const stringLiteralPattern = /(["'`])([A-Za-z0-9._-]+)\1/g;

function resolveDynamicTemplate(raw) {
  const match = /^([A-Za-z0-9._-]*)\$\{[^}\n]+\}([A-Za-z0-9._-]*)$/.exec(raw);
  if (!match) {
    return false;
  }
  const [, prefix, suffix] = match;
  if (!`${prefix}${suffix}`.includes(".")) {
    return false;
  }
  const matchedKeys = sortedLocaleKeys.filter((key) => key.startsWith(prefix) && key.endsWith(suffix));
  if (matchedKeys.length === 0) {
    unresolvedDynamicKeys.push(raw);
    return false;
  }
  for (const key of matchedKeys) {
    usedKeys.add(key);
  }
  return true;
}

for (const file of files) {
  const text = fs.readFileSync(file, "utf8");
  const sourceFile = ts.createSourceFile(
    file,
    text,
    ts.ScriptTarget.Latest,
    true,
    file.endsWith(".tsx") ? ts.ScriptKind.TSX : ts.ScriptKind.TS
  );

  function visit(node) {
    if (ts.isJsxText(node)) {
      const content = visibleTextValue(node.getText(sourceFile));
      if (!looksLikeNonUserFacingVisibleText(content)) {
        const line = sourceFile.getLineAndCharacterOfPosition(node.getStart(sourceFile)).line + 1;
        hardcodedEnglish.push({
          file: relative(file),
          line,
          content,
        });
      }
    }
    if (
      ts.isJsxAttribute(node) &&
      node.initializer &&
      ts.isStringLiteral(node.initializer) &&
      !hardcodedEnglishSkipAttributes.has(node.name.getText(sourceFile))
    ) {
      const content = visibleTextValue(node.initializer.text);
      if (!looksLikeNonUserFacingVisibleText(content)) {
        const line = sourceFile.getLineAndCharacterOfPosition(node.getStart(sourceFile)).line + 1;
        hardcodedEnglish.push({
          file: relative(file),
          line,
          content: `${node.name.getText(sourceFile)}="${content}"`,
        });
      }
    }
    if (
      !shouldSkipHardcodedEnglishPropertyFile(file) &&
      ts.isPropertyAssignment(node) &&
      hardcodedEnglishUserFacingPropertyNames.has(hardcodedEnglishPropertyName(node.name)) &&
      (ts.isStringLiteral(node.initializer) || ts.isNoSubstitutionTemplateLiteral(node.initializer))
    ) {
      const content = visibleTextValue(node.initializer.text);
      if (!looksLikeNonUserFacingVisibleText(content)) {
        const line = sourceFile.getLineAndCharacterOfPosition(node.getStart(sourceFile)).line + 1;
        hardcodedEnglish.push({
          file: relative(file),
          line,
          content: `${hardcodedEnglishPropertyName(node.name)}="${content}"`,
        });
      }
    }
    if (isI18nCallExpression(node)) {
      const calleeName = ts.isPropertyAccessExpression(node.expression) ? node.expression.name.text : node.expression.text;
      const secondArg = node.arguments[1];
      const hasFallbackLikeArg =
        calleeName === "translate" ||
        (node.arguments.length >= 2 && secondArg && !isExplicitLanguageArg(secondArg, sourceFile));
      if (hasFallbackLikeArg) {
        const line = sourceFile.getLineAndCharacterOfPosition(node.getStart(sourceFile)).line + 1;
        i18nCallViolations.push({
          file: relative(file),
          line,
          content: node.getText(sourceFile),
        });
      }
    }
    if (
      ts.isCallExpression(node) &&
      ts.isPropertyAccessExpression(node.expression) &&
      localizedCasingMethods.has(node.expression.name.text) &&
      expressionContainsLocalizedText(node.expression.expression, sourceFile)
    ) {
      const line = sourceFile.getLineAndCharacterOfPosition(node.getStart(sourceFile)).line + 1;
      localizedCasingViolations.push({
        file: relative(file),
        line,
        content: node.getText(sourceFile),
      });
    }
    if (
      ts.isCallExpression(node) &&
      ts.isIdentifier(node.expression) &&
      localizedCasingFunctionPattern.test(node.expression.text) &&
      node.arguments.some((argument) => expressionContainsLocalizedText(argument, sourceFile))
    ) {
      const line = sourceFile.getLineAndCharacterOfPosition(node.getStart(sourceFile)).line + 1;
      localizedCasingViolations.push({
        file: relative(file),
        line,
        content: node.getText(sourceFile),
      });
    }
    ts.forEachChild(node, visit);
  }
  visit(sourceFile);

  for (const match of text.matchAll(keyPattern)) {
    const key = match[2];
    if (key.includes("${")) {
      resolveDynamicTemplate(key);
    } else {
      usedKeys.add(key);
    }
  }

  for (const match of text.matchAll(propertyKeyPattern)) {
    const key = match[2];
    if (key.includes("${")) {
      resolveDynamicTemplate(key);
    } else if (localeKeys.has(key)) {
      usedKeys.add(key);
    }
  }

  for (const match of text.matchAll(stringLiteralPattern)) {
    const candidate = match[2];
    if (localeKeys.has(candidate)) {
      usedKeys.add(candidate);
    }
  }

  for (const [index, line] of text.split(/\r?\n/).entries()) {
    const trimmed = line.trim();
    if (!trimmed) {
      continue;
    }
    if (/\bcopy\b/.test(line)) {
      i18nCopyNamingViolations.push({
        file: relative(file),
        line: index + 1,
        content: trimmed,
      });
    }
    if (/^\/\/|^\/\*|^\*|^\{\/\*/.test(trimmed)) {
      continue;
    }
    if (/\b(?:t|translate)\(/.test(line)) {
      continue;
    }
    if (
      !shouldSkipHardcodedChineseFile(file) &&
      /[\u4e00-\u9fff]/.test(line) &&
      !hardcodedChineseAllowedLinePatterns.some((pattern) => pattern.test(line))
    ) {
      hardcodedChinese.push({
        file: relative(file),
        line: index + 1,
        content: trimmed,
      });
    }
  }
}

const enKeys = new Set(Object.keys(en));
const zhKeys = new Set(Object.keys(zh));
const localeCounts = Object.fromEntries(
  Object.entries(localeFlats).map(([locale, values]) => [locale, Object.keys(values).length])
);
const localeLeafTypeViolations = Object.entries(localeSources).flatMap(([locale, source]) =>
  collectLocaleLeafTypeViolations(source, locale)
);
const localeParityViolations = collectLocaleParityViolations(localeFlats, baseLocale, en);
const localePlaceholderViolations = collectLocalePlaceholderViolations(localeFlats, baseLocale, en);
const runningFPSUnitViolations = collectRunningFPSUnitViolations(localeFlats);

const missingInZh = [...enKeys].filter((key) => !zhKeys.has(key));
const extraInZh = [...zhKeys].filter((key) => !enKeys.has(key));
const unusedInEn = [...enKeys].filter((key) => !usedKeys.has(key));
const missingDefs = [...usedKeys].filter((key) => !enKeys.has(key));
const concreteMissingDefs = missingDefs.filter((key) => !key.includes("${"));
const dynamicMissingDefs = [...new Set(unresolvedDynamicKeys)];
const englishStyleViolations = Object.entries(en)
  .filter(([key, value]) => normalizeEnglishLocaleValue(key, value) !== value)
  .map(([key, value]) => ({ key, value, expected: normalizeEnglishLocaleValue(key, value) }));
const englishImplementationDetailViolations = collectEnglishImplementationDetails(en);
const zhImplementationDetailViolations = chineseLocales.flatMap(([locale, values]) =>
  collectZhImplementationDetails(values).map((item) => ({ locale, ...item }))
);
const zhGlossaryViolations = chineseLocales.flatMap(([locale, values]) =>
  Object.entries(values)
    .filter(([, value]) => normalizeZhGlossaryValue(value) !== value)
    .map(([key, value]) => ({ locale, key, value, expected: normalizeZhGlossaryValue(value) }))
);
const zhUnexpectedEnglishViolations = chineseLocales
  .flatMap(([locale, values]) =>
    Object.entries(values).map(([key, value]) => ({
      locale,
      key,
      value,
      tokens: collectUnexpectedZhEnglishTokens(key, value),
    }))
  )
  .filter((item) => item.tokens.length > 0);
const zhSameAsEnglishViolations = chineseLocales.flatMap(([locale, values]) =>
  Object.entries(values)
    .filter(([key, value]) => en[key] === value && collectUnexpectedZhEnglishTokens(key, value).length > 0)
    .map(([key, value]) => ({ locale, key, value }))
);
const nativeLanguageLabelViolations = [];
const nativeLanguageLabelKeys = new Set([
  ...Object.keys(knownNativeLanguageLabels),
  ...Object.keys(en).filter((item) => item.startsWith("settings.language.option.")),
]);
for (const key of nativeLanguageLabelKeys) {
  const expected = knownNativeLanguageLabels[key] ?? en[key];
  for (const [locale, values] of Object.entries(localeFlats)) {
    if (values[key] !== expected) {
      nativeLanguageLabelViolations.push({
        locale,
        key,
        value: values[key] ?? "",
        expected,
      });
    }
  }
}

const summary = {
  locale: {
    baseLocale,
    localeCounts,
    enCount: enKeys.size,
    zhCount: zhKeys.size,
    usedKeyCount: usedKeys.size,
    localeParityViolationCount: localeParityViolations.length,
    localeMissingKeyCount: localeParityViolations.filter((item) => item.type === "missing").length,
    localeExtraKeyCount: localeParityViolations.filter((item) => item.type === "extra").length,
    localePlaceholderViolationCount: localePlaceholderViolations.length,
    runningFPSUnitViolationCount: runningFPSUnitViolations.length,
    localeLeafTypeViolationCount: localeLeafTypeViolations.length,
    missingInZhCount: missingInZh.length,
    extraInZhCount: extraInZh.length,
    unusedInEnCount: unusedInEn.length,
    missingDefinitionCount: missingDefs.length,
    concreteMissingDefinitionCount: concreteMissingDefs.length,
    dynamicMissingDefinitionCount: dynamicMissingDefs.length,
    i18nCallViolationCount: i18nCallViolations.length,
    localizedCasingViolationCount: localizedCasingViolations.length,
    englishStyleViolationCount: englishStyleViolations.length,
    englishImplementationDetailViolationCount: englishImplementationDetailViolations.length,
    zhImplementationDetailViolationCount: zhImplementationDetailViolations.length,
    zhGlossaryViolationCount: zhGlossaryViolations.length,
    zhUnexpectedEnglishViolationCount: zhUnexpectedEnglishViolations.length,
    zhSameAsEnglishViolationCount: zhSameAsEnglishViolations.length,
    nativeLanguageLabelViolationCount: nativeLanguageLabelViolations.length,
    hardcodedEnglishCount: hardcodedEnglish.length,
    hardcodedChineseCount: hardcodedChinese.length,
    i18nCopyNamingViolationCount: i18nCopyNamingViolations.length,
  },
  samples: {
    localeParityViolations: localeParityViolations.slice(0, 80),
    localePlaceholderViolations: localePlaceholderViolations.slice(0, 40),
    runningFPSUnitViolations: runningFPSUnitViolations.slice(0, 40),
    localeLeafTypeViolations: localeLeafTypeViolations.slice(0, 40),
    concreteMissingDefinitions: concreteMissingDefs.slice(0, 40),
    dynamicMissingDefinitions: dynamicMissingDefs.slice(0, 20),
    unusedInEn: unusedInEn.slice(0, 40),
    hardcodedChinese: hardcodedChinese.slice(0, 40),
    i18nCallViolations: i18nCallViolations.slice(0, 40),
    localizedCasingViolations: localizedCasingViolations.slice(0, 40),
    englishStyleViolations: englishStyleViolations.slice(0, 40),
    englishImplementationDetailViolations: englishImplementationDetailViolations.slice(0, 40),
    zhImplementationDetailViolations: zhImplementationDetailViolations.slice(0, 40),
    zhGlossaryViolations: zhGlossaryViolations.slice(0, 40),
    zhUnexpectedEnglishViolations: zhUnexpectedEnglishViolations.slice(0, 40),
    zhSameAsEnglishViolations: zhSameAsEnglishViolations.slice(0, 40),
    nativeLanguageLabelViolations: nativeLanguageLabelViolations.slice(0, 40),
    hardcodedEnglish: hardcodedEnglish.slice(0, 40),
    i18nCopyNamingViolations: i18nCopyNamingViolations.slice(0, 40),
  },
};

if (jsonOnly) {
  console.log(JSON.stringify(summary, null, 2));
} else {
  console.log("i18n audit summary");
  console.log(`- locale keys: ${Object.entries(localeCounts).map(([locale, count]) => `${locale}=${count}`).join(", ")}`);
  console.log(`- base locale: ${baseLocale}`);
  console.log(`- locale parity violations: ${summary.locale.localeParityViolationCount} (missing=${summary.locale.localeMissingKeyCount}, extra=${summary.locale.localeExtraKeyCount})`);
  console.log(`- locale placeholder violations: ${summary.locale.localePlaceholderViolationCount}`);
  console.log(`- running FPS unit violations: ${summary.locale.runningFPSUnitViolationCount}`);
  console.log(`- locale leaf type violations: ${summary.locale.localeLeafTypeViolationCount}`);
  console.log(`- used keys in source: ${summary.locale.usedKeyCount}`);
  console.log(`- missing definitions: ${summary.locale.missingDefinitionCount} (concrete=${summary.locale.concreteMissingDefinitionCount}, dynamic=${summary.locale.dynamicMissingDefinitionCount})`);
  console.log(`- unused en keys: ${summary.locale.unusedInEnCount}`);
  console.log(`- missing zh-CN keys: ${summary.locale.missingInZhCount}`);
  console.log(`- extra zh-CN keys: ${summary.locale.extraInZhCount}`);
  console.log(`- hardcoded Chinese lines: ${hardcodedChinese.length}`);
  console.log(`- hardcoded English lines: ${hardcodedEnglish.length}`);
  console.log(`- invalid i18n calls: ${i18nCallViolations.length}`);
  console.log(`- localized casing violations: ${localizedCasingViolations.length}`);
  console.log(`- english style violations: ${englishStyleViolations.length}`);
  console.log(`- english implementation detail violations: ${englishImplementationDetailViolations.length}`);
  console.log(`- zh implementation detail violations: ${zhImplementationDetailViolations.length}`);
  console.log(`- zh glossary violations: ${zhGlossaryViolations.length}`);
  console.log(`- zh unexpected English violations: ${zhUnexpectedEnglishViolations.length}`);
  console.log(`- zh same-as-English violations: ${zhSameAsEnglishViolations.length}`);
  console.log(`- native language label violations: ${nativeLanguageLabelViolations.length}`);
  console.log(`- i18n copy naming violations: ${i18nCopyNamingViolations.length}`);

  if (concreteMissingDefs.length > 0) {
    console.log("\nmissing concrete keys:");
    for (const key of concreteMissingDefs.slice(0, 20)) {
      console.log(`- ${key}`);
    }
  }

  if (localeParityViolations.length > 0) {
    console.log("\nlocale parity samples:");
    for (const item of localeParityViolations.slice(0, 20)) {
      console.log(`- ${item.locale}: ${item.type} ${item.key}`);
    }
  }

  if (localePlaceholderViolations.length > 0) {
    console.log("\nlocale placeholder samples:");
    for (const item of localePlaceholderViolations.slice(0, 20)) {
      console.log(`- ${item.locale}:${item.key}: expected {${item.expected.join(", ")}} got {${item.actual.join(", ")}}`);
    }
  }

  if (runningFPSUnitViolations.length > 0) {
    console.log("\nrunning FPS unit samples:");
    for (const item of runningFPSUnitViolations.slice(0, 20)) {
      console.log(`- ${item.locale}:${item.key}: ${item.value} -> ${item.expected}`);
    }
  }

  if (localeLeafTypeViolations.length > 0) {
    console.log("\nlocale leaf type samples:");
    for (const item of localeLeafTypeViolations.slice(0, 20)) {
      console.log(`- ${item.locale}:${item.key}: ${item.valueType}`);
    }
  }

  if (dynamicMissingDefs.length > 0) {
    console.log("\ndynamic keys that need manual review:");
    for (const key of dynamicMissingDefs.slice(0, 10)) {
      console.log(`- ${key}`);
    }
  }

  if (hardcodedChinese.length > 0) {
    console.log("\nhardcoded Chinese samples:");
    for (const item of hardcodedChinese.slice(0, 20)) {
      console.log(`- ${item.file}:${item.line} ${item.content}`);
    }
  }

  if (hardcodedEnglish.length > 0) {
    console.log("\nhardcoded English samples:");
    for (const item of hardcodedEnglish.slice(0, 20)) {
      console.log(`- ${item.file}:${item.line} ${item.content}`);
    }
  }

  if (i18nCallViolations.length > 0) {
    console.log("\ninvalid i18n call samples:");
    for (const item of i18nCallViolations.slice(0, 20)) {
      console.log(`- ${item.file}:${item.line} ${item.content}`);
    }
  }

  if (localizedCasingViolations.length > 0) {
    console.log("\nlocalized casing samples:");
    for (const item of localizedCasingViolations.slice(0, 20)) {
      console.log(`- ${item.file}:${item.line} ${item.content}`);
    }
  }

  if (englishStyleViolations.length > 0) {
    console.log("\nenglish style samples:");
    for (const item of englishStyleViolations.slice(0, 20)) {
      console.log(`- ${item.key}: ${item.value} -> ${item.expected}`);
    }
  }

  if (englishImplementationDetailViolations.length > 0) {
    console.log("\nenglish implementation detail samples:");
    for (const item of englishImplementationDetailViolations.slice(0, 20)) {
      console.log(`- ${item.key}: ${item.value}`);
    }
  }

  if (zhImplementationDetailViolations.length > 0) {
    console.log("\nzh implementation detail samples:");
    for (const item of zhImplementationDetailViolations.slice(0, 20)) {
      console.log(`- ${item.locale}:${item.key}: ${item.value}`);
    }
  }

  if (zhGlossaryViolations.length > 0) {
    console.log("\nzh glossary samples:");
    for (const item of zhGlossaryViolations.slice(0, 20)) {
      console.log(`- ${item.locale}:${item.key}: ${item.value} -> ${item.expected}`);
    }
  }

  if (zhUnexpectedEnglishViolations.length > 0) {
    console.log("\nzh unexpected English samples:");
    for (const item of zhUnexpectedEnglishViolations.slice(0, 20)) {
      console.log(`- ${item.locale}:${item.key}: ${item.value} (${item.tokens.join(", ")})`);
    }
  }

  if (zhSameAsEnglishViolations.length > 0) {
    console.log("\nzh same-as-English samples:");
    for (const item of zhSameAsEnglishViolations.slice(0, 20)) {
      console.log(`- ${item.locale}:${item.key}: ${item.value}`);
    }
  }

  if (nativeLanguageLabelViolations.length > 0) {
    console.log("\nnative language label samples:");
    for (const item of nativeLanguageLabelViolations.slice(0, 20)) {
      console.log(`- ${item.locale}:${item.key}: ${item.value} -> ${item.expected}`);
    }
  }

  if (i18nCopyNamingViolations.length > 0) {
    console.log("\ni18n copy naming samples:");
    for (const item of i18nCopyNamingViolations.slice(0, 20)) {
      console.log(`- ${item.file}:${item.line} ${item.content}`);
    }
  }
}

if (pruneUnused) {
  for (const [locale, source] of Object.entries(localeSources)) {
    const nextSource = filterLocaleTree(source, usedKeys);
    fs.writeFileSync(path.join(localeDir, `${locale}.json`), `${JSON.stringify(nextSource, null, 2)}\n`);
  }
}

if (strict) {
  const hasBlockingIssues =
    localeParityViolations.length > 0 ||
    localePlaceholderViolations.length > 0 ||
    runningFPSUnitViolations.length > 0 ||
    localeLeafTypeViolations.length > 0 ||
    concreteMissingDefs.length > 0 ||
    dynamicMissingDefs.length > 0 ||
    unusedInEn.length > 0 ||
    hardcodedChinese.length > 0 ||
    hardcodedEnglish.length > 0 ||
    i18nCallViolations.length > 0 ||
    localizedCasingViolations.length > 0 ||
    englishStyleViolations.length > 0 ||
    englishImplementationDetailViolations.length > 0 ||
    zhImplementationDetailViolations.length > 0 ||
    zhGlossaryViolations.length > 0 ||
    zhUnexpectedEnglishViolations.length > 0 ||
    zhSameAsEnglishViolations.length > 0 ||
    nativeLanguageLabelViolations.length > 0 ||
    i18nCopyNamingViolations.length > 0;
  if (hasBlockingIssues) {
    process.exitCode = 1;
  }
}
