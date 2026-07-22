import { describe, expect, test } from "bun:test";

describe("App Sessions page contract", () => {
  test("registers Douyin with localized labels and the shared TikTok appearance recipe", async () => {
    const [source, workflows] = await Promise.all([
      Bun.file(new URL("./index.tsx", import.meta.url)).text(),
      Bun.file(
        new URL("../../../shared/styles/dream/workflows.css", import.meta.url),
      ).text(),
    ]);

    expect(source).toContain('douyin: "settings.appSessions.item.douyin"');
    expect(source).toMatch(
      /ACCOUNT_VERIFIABLE_SITE_KEYS = new Set\(\[[\s\S]*?"douyin"/,
    );
    expect(source).toMatch(
      /localeCompare\(\s*resolveLabel\(right\),\s*language,?\s*\)/,
    );
    expect(workflows).toContain('[data-site="douyin"]');

    for (const fileName of [
      "en.json",
      "zh-CN.json",
      "zh-TW.json",
      "ja-JP.json",
      "ko-KR.json",
      "es-419.json",
      "pt-BR.json",
      "id-ID.json",
      "vi-VN.json",
    ]) {
      const locale = await Bun.file(
        new URL(`../../../shared/i18n/locales/${fileName}`, import.meta.url),
      ).json();
      expect(locale.settings.appSessions.item.douyin).toBe(
        fileName.startsWith("zh-") ? "\u6296\u97f3" : "Douyin",
      );
    }
  });

  test("registers Xiaohongshu with localized labels and the shared Dream appearance recipe", async () => {
    const [source, workflows] = await Promise.all([
      Bun.file(new URL("./index.tsx", import.meta.url)).text(),
      Bun.file(
        new URL("../../../shared/styles/dream/workflows.css", import.meta.url),
      ).text(),
    ]);

    expect(source).toContain(
      'xiaohongshu: "settings.appSessions.item.xiaohongshu"',
    );
    expect(source).toMatch(
      /ACCOUNT_VERIFIABLE_SITE_KEYS = new Set\(\[[\s\S]*?"xiaohongshu"/,
    );
    expect(workflows).toContain(".app-site-brand-surface");
    expect(workflows).toContain(".app-site-brand-icon");

    for (const fileName of [
      "en.json",
      "zh-CN.json",
      "zh-TW.json",
      "ja-JP.json",
      "ko-KR.json",
      "es-419.json",
      "pt-BR.json",
      "id-ID.json",
      "vi-VN.json",
    ]) {
      const locale = await Bun.file(
        new URL(`../../../shared/i18n/locales/${fileName}`, import.meta.url),
      ).json();
      expect(locale.settings.appSessions.item.xiaohongshu).toBe(
        fileName.startsWith("zh-") ? "\u5c0f\u7ea2\u4e66" : "Xiaohongshu",
      );
    }
  });

  test("maps session health to the canonical icon-only StatusBadge", async () => {
    const [source, workflows] = await Promise.all([
      Bun.file(new URL("./index.tsx", import.meta.url)).text(),
      Bun.file(
        new URL("../../../shared/styles/dream/workflows.css", import.meta.url),
      ).text(),
    ]);

    expect(source).toContain("<DreamStatusBadge");
    expect(source).toContain("iconOnly");
    expect(source).toContain('tone: "success"');
    expect(source).toContain('tone: "warning"');
    expect(source).not.toContain("app-sessions-status-connected");
    expect(workflows).not.toContain(".app-sessions-status-connected");
    expect(workflows).not.toContain(".app-sessions-status-expired");
  });

  test("uses an operational split page with explicit pane scrolling", async () => {
    const source = await Bun.file(
      new URL("./index.tsx", import.meta.url),
    ).text();

    expect(source).toContain("<WorkspacePage");
    expect(source).toContain("<WorkspacePageTopBar");
    expect(source).toContain("<WorkspacePageContent");
    expect(source).toContain('recipe: "operational"');
    expect(source).toContain('contentLayout: "split"');
    expect(source).toContain('heading: "assistive"');
    expect(source).toContain('scroll: "panes"');
    expect(source.match(/data-scroll-owner="true"/g)).toHaveLength(2);
    expect(source).not.toContain("<h1");
    expect(source).not.toContain("System.IsWindows()");
    expect(source).not.toContain("app-workspace-primary-header__safe-area");
  });

  test("continues the shared split divider through the top bar", async () => {
    const [source, css] = await Promise.all([
      Bun.file(new URL("./index.tsx", import.meta.url)).text(),
      Bun.file(
        new URL("../../../shared/styles/dream/workflows.css", import.meta.url),
      ).text(),
    ]);

    expect(source).toContain(
      "app-sessions-page-topbar-leading app-workspace-primary-subpane app-workspace-primary-subpane--leading",
    );
    expect(source).toMatch(
      /app-sessions-page-topbar-leading[\s\S]*?app-sessions-list-pane app-workspace-primary-subpane app-workspace-primary-subpane--leading/,
    );
    expect(source).not.toContain("w-[320px]");
    expect(css).toContain("--app-sessions-leading-pane-width: 20rem;");
    expect(css).toMatch(
      /\.app-sessions-list-pane,\s*\.app-sessions-page-topbar-leading\s*\{[^}]*inline-size:\s*var\(--app-sessions-leading-pane-width\);[^}]*flex:\s*0 0 var\(--app-sessions-leading-pane-width\);/s,
    );
    expect(css).toMatch(
      /\.app-sessions-page-topbar\s*\{[^}]*padding-inline:\s*0;/s,
    );
    expect(css).toMatch(
      /\.app-sessions-page-topbar[\s\S]*?> \.app-workspace-page__topbar-actions\s*\{[^}]*height:\s*100%;[^}]*align-self:\s*stretch;/s,
    );
    expect(css).toMatch(
      /\.app-sessions-page-topbar-leading\s*\{[^}]*align-self:\s*stretch;/s,
    );
  });

  test("uses the shared icon-only sync action and balanced empty-state actions", async () => {
    const source = await Bun.file(new URL("./index.tsx", import.meta.url)).text();
    const [sheet, workflows, buttonContract] = await Promise.all([
      Bun.file(new URL("./AppSessionImportSheet.tsx", import.meta.url)).text(),
      Bun.file(
        new URL("../../../shared/styles/dream/workflows.css", import.meta.url),
      ).text(),
      Bun.file(
        new URL("../../../shared/styles/dream/button-contract.css", import.meta.url),
      ).text(),
    ]);

    expect(source).toContain("<AppSessionImportSheet");
    expect(source).toMatch(
      /app-sessions-page-topbar-leading[^\n]*justify-start[\s\S]*?<WorkspacePrimaryHeaderAction[\s\S]*?label=\{t\("browserSource\.syncAction"\)\}[\s\S]*?<CloudSync[\s\S]*?<\/WorkspacePrimaryHeaderAction>/,
    );
    expect(source).not.toMatch(
      /app-sessions-page-topbar-leading[^\n]*justify-start[\s\S]*?<WorkspacePrimaryHeaderAction[\s\S]*?<span>\{t\("browserSource\.syncAction"\)\}<\/span>/,
    );
    expect(source).toContain('className="app-sessions-sync-title-action"');
    expect(source).not.toMatch(
      /<div className="flex min-w-0 flex-1 items-center justify-end[\s\S]*?browserSource\.syncAction/,
    );
    expect(source).not.toContain("<Search");
    expect(source).not.toContain("<Input");
    expect(source).toContain("const verifySession = useVerifyAppSession()");
    expect(source).not.toContain("useOpenAppSessionSite");
    expect(source).toContain("onConnect={() => void beginConnect(selected)}");
    expect(source).toContain("onVerify={() => void verifyAccount(selected)}");
    expect(source).toContain('<ShieldCheck className="h-4 w-4" />');
    expect(source).toContain("onBrowserSync={() => setImportSheetOpen(true)}");
    expect(source).toMatch(
      /<div className="grid w-full grid-cols-2 gap-2">[\s\S]*?variant="outline"[\s\S]*?props\.labels\.manualSignIn[\s\S]*?<CloudSync[\s\S]*?props\.labels\.browserSync/,
    );
    expect(
      source.match(/className="app-sessions-account-action min-w-0"/g)
        ?.length,
    ).toBeGreaterThanOrEqual(4);
    expect(source).toContain("props.labels.manualSignIn");
    expect(source).toContain('<LogIn className="h-4 w-4" />');
    expect(source).not.toContain("props.labels.youtubeSignIn");
    expect(source).toContain("props.labels.source");
    expect(sheet).toContain('<SheetContent centered size="lg">');
    expect(sheet).toContain('<SheetHeader className="pb-1">');
    expect(sheet).toContain('<SheetDescription className="sr-only">');
    expect(sheet).toContain('<SheetFooter className="pt-0">');
    expect(sheet).toContain("app-browser-source-card");
    expect(sheet).toContain('className="app-browser-source-card group"');
    expect(sheet).toContain('className="app-browser-profile-card"');
    expect(sheet).toContain("flex flex-wrap justify-center gap-x-6 gap-y-3");
    expect(sheet).toContain('className="h-11 w-11"');
    expect(sheet).toContain("<BrowserBrandIcon");
    expect(sheet).toContain("<Button");
    expect(sheet).toContain("<Badge");
    expect(sheet).toContain("<StatusBadge");
    expect(sheet).not.toContain("<button");
    expect(sheet).not.toMatch(/(?:bg|border|ring|shadow|text)-(?:amber|emerald|rose)-/);
    expect(sheet).not.toMatch(/(?:bg|border|ring|shadow)-\[/);
    expect(workflows).toContain(".app-browser-source-card__icon");
    expect(workflows).toContain(".app-browser-profile-card__selection[data-selected=\"true\"]");
    expect(workflows).toContain(".app-session-import-prerequisite-card");
    expect(workflows).toContain(".app-session-import-scan-item");
    expect(buttonContract).toContain(
      ".app-dream-button.app-motion-surface.app-browser-source-card",
    );
    expect(buttonContract).toMatch(
      /\.app-browser-source-card\[data-app-button\][\s\S]*?--app-button-border:\s*0;[\s\S]*?background:\s*transparent;/,
    );
    expect(buttonContract).toMatch(
      /\.app-browser-profile-card\[data-app-button\][\s\S]*?\[aria-checked="true"\]\s*\{[^}]*background:\s*var\(--app-glass-interactive-fill-selected\);[^}]*box-shadow:\s*none;/,
    );
    expect(workflows).toMatch(
      /\.app-session-import-prerequisite-card,[\s\S]*?\.app-session-import-scan-item\s*\{[^}]*border:\s*0;[^}]*background:\s*transparent;/,
    );
    expect(sheet).not.toContain("<ChevronRight");
    expect(sheet).not.toContain('t("browserSource.readOnlyNotice")');
    expect(sheet).not.toContain('t("browserSource.permissionHint")');
    expect(sheet).not.toContain("browserAccessErrorDescription");
    expect(sheet).toContain("useAppSessionBrowserProfileSources");
    expect(sheet).toContain("browserSources.data?.filter((browser) => browser.available)");
    expect(sheet).not.toContain("selectedBrowser?.error ||");
    expect(sheet).toContain("useDiscoverAppSessionBrowserProfiles");
    expect(sheet).toContain("useCurrentResourceSniffBrowserStatus");
    expect(sheet).toContain('mode: "current_browser"');
    expect(sheet).toContain('data-profile-source="current-browser"');
    expect(sheet).toContain('t("browserSource.authorizedRead")');
    expect(sheet).toContain('methodLabel={t("browserSource.copyAndParse")}');
    expect(sheet).toContain('t("browserSource.currentChromeCDPDescription")');
    expect(sheet).toContain('description={t("browserSource.copyAndParseDescription")}');
    expect(sheet).toContain("selectedProfile?.displayPath?.trim()");
    expect(sheet).toContain('data-profile-address={selectedProfileAddress ? "true" : undefined}');
    expect(sheet).toContain("CURRENT_CHROME_REMOTE_DEBUGGING_URL");
    expect(sheet).not.toContain("useBrowserSources");
    expect(sheet).toContain('role="radiogroup"');
    expect(sheet).toContain('data-profile-layout="single-column"');
    expect(sheet).toContain('className="grid grid-cols-1 gap-2"');
    expect(sheet).not.toContain('className="grid gap-2 sm:grid-cols-2"');
    expect(sheet).toContain('setStep("method")');
    expect(sheet).toContain('setStep("prerequisite")');
    expect(sheet).toContain('data-profile-method-selection="true"');
    expect(sheet).toContain('data-profile-prerequisite-step="true"');
    expect(sheet).toContain('data-profile-prerequisite="current-browser"');
    expect(sheet).toContain('data-profile-prerequisite="browser-profile"');
    expect(sheet).toContain('t("xiadown.actions.next")');
    expect(sheet).toContain('stepRef.current !== "prerequisite"');
    expect(sheet).toMatch(
      /step === "prerequisite"[\s\S]{0,200}selection\.mode === "current_browser"/,
    );
    const selectCurrentChromeStart = sheet.indexOf("const selectCurrentChrome = () => {");
    const selectCurrentChromeEnd = sheet.indexOf("\n  return (", selectCurrentChromeStart);
    expect(selectCurrentChromeStart).toBeGreaterThan(-1);
    expect(sheet.slice(selectCurrentChromeStart, selectCurrentChromeEnd)).not.toContain("currentChromeReady");
    expect(sheet).toContain("browserPermissionRequired");
    expect(sheet).toContain("const profiles = selectedBrowser?.profiles ?? []");
    expect(sheet).toContain("const protectedProfiles = profiles.filter((profile) => {");
    expect(sheet).toContain("const availableProfiles = directProfiles.filter((profile) => profile.available)");
    expect(sheet).toContain("availableProfiles.length === 0 && browserHasAccessError(selectedBrowser)");
    expect(sheet).toContain("{methodProfiles.map((profile) => (");
    expect(sheet).not.toContain("{profiles.map((profile) => (");
    expect(sheet).toContain('data-profile-protection-notice="true"');
    expect(sheet).toContain("disabled={disabled}");
    expect(sheet).toContain("aria-disabled={disabled}");
    expect(sheet).toContain("data-profile-state={browserProfileAvailabilityReason(props.profile) || \"ready\"}");
    expect(sheet).toContain("browserProfileCanEnterPrerequisite(profile)");
    expect(sheet).toContain("resolveBrowserProfilePrerequisite(");
    expect(sheet).toContain("selectedProfileSnapshot");
    expect(sheet).toContain("setSelectedProfileSnapshot(refreshedSelectedProfile)");
    expect(sheet).toContain("selectedProfileResolution.presentInDiscovery");
    expect(sheet).toContain("!selectedProfileReady ? (");
    expect(sheet).toContain("browserProfileDisplayLabel(");
    expect(sheet).not.toContain('rawLabel === "Default"');
    expect(sheet).not.toContain("{availableProfiles.map((profile) => (");
    expect(sheet).not.toContain("<BrowserSourcePicker");
    expect(sheet).toContain("scanBrowserAppSessions");
    expect(sheet).toContain("useImportBrowserAppSessions");
    expect(sheet).toContain("snapshotToken: requestSnapshotToken");
    expect(sheet).toContain("!scanSnapshotToken || importing");
    expect(sheet).toContain("browserScanReasonTranslationKey(item.reason)");
    expect(sheet).not.toContain("item.accountLabel || item.reason");
    expect(sheet).toContain('case "replace":');
    expect(sheet).toContain("scanStatusTone(item.status)");
    expect(sheet).toContain('result.importedIds.length === 0');
    expect(sheet).not.toContain('result.importedIds.length || appSessionIds.length');
  });

  test("invalidates stale discovery, scan, and import completions", async () => {
    const sheet = await Bun.file(
      new URL("./AppSessionImportSheet.tsx", import.meta.url),
    ).text();

    expect(sheet).toContain("discoveryEpochRef");
    expect(sheet).toContain("operationEpochRef");
    expect(sheet).toContain("tryBeginAppSessionImportOperation(operationBusyRef, \"scan\")");
    expect(sheet).toContain("tryBeginAppSessionImportOperation(operationBusyRef, \"import\")");
    expect(sheet.match(/hasActiveAppSessionImportOperation\(operationBusyRef\)/g)).toHaveLength(10);
    expect(sheet).toMatch(
      /const selectProfile = \(profileId: string\) => \{\s*if \(hasActiveAppSessionImportOperation\(operationBusyRef\)\)/,
    );
    expect(sheet).toMatch(
      /const selectCurrentChrome = \(\) => \{[\s\S]*?hasActiveAppSessionImportOperation\(operationBusyRef\)/,
    );
    expect(sheet).toMatch(
      /const returnToBrowserStep = \(\) => \{\s*if \(hasActiveAppSessionImportOperation\(operationBusyRef\)\)/,
    );
    expect(sheet).toMatch(
      /const returnToMethodStep = \(\) => \{\s*if \(hasActiveAppSessionImportOperation\(operationBusyRef\)\)/,
    );
    expect(sheet).toMatch(
      /const returnToPrerequisiteStep = \(\) => \{\s*if \(hasActiveAppSessionImportOperation\(operationBusyRef\)\)/,
    );
    expect(sheet).toMatch(
      /const handleSheetOpenChange = \(open: boolean\) => \{[\s\S]*?invalidatePendingRequests\(\);[\s\S]*?setScanning\(false\);[\s\S]*?setImporting\(false\);/,
    );
    expect(sheet).toContain("selectedBrowserIdRef");
    expect(sheet).toContain("isCurrentBrowserDiscovery(flowSnapshot(), request)");
    expect(sheet).toContain("isCurrentImportOperation(flowSnapshot(), request)");
    expect(sheet).toContain("onOpenChange={handleSheetOpenChange}");
    expect(sheet).toContain("onClick={returnToBrowserStep}");
    expect(sheet).toContain("onClick={returnToMethodStep}");
    expect(sheet).toContain("onClick={returnToPrerequisiteStep}");
    expect(sheet).toContain("interactionDisabled={scanning || importing}");
    expect(sheet).toContain("disabled={scanning}");
    expect(sheet).toContain("disabled={importing}");
    expect(sheet).not.toContain(
      'setScanError(t("browserSource.loadFailed"))',
    );
  });

  test("localizes the manual sign-in affordance explicitly", async () => {
    const expected = {
      "en.json": "Manual sign-in",
      "zh-CN.json": "\u624b\u52a8\u767b\u5f55",
      "zh-TW.json": "\u624b\u52d5\u767b\u5165",
      "ja-JP.json": "\u624b\u52d5\u3067\u30ed\u30b0\u30a4\u30f3",
      "ko-KR.json": "수동 로그인",
      "es-419.json": "Inicio de sesión manual",
      "pt-BR.json": "Login manual",
      "id-ID.json": "Masuk manual",
      "vi-VN.json": "Đăng nhập thủ công",
    } as const;

    for (const [fileName, label] of Object.entries(expected)) {
      const locale = await Bun.file(
        new URL(`../../../shared/i18n/locales/${fileName}`, import.meta.url),
      ).json();
      expect(locale.settings.appSessions.manualSignIn).toBe(label);
      expect(typeof locale.browserSource.browserSync).toBe("string");
      expect(locale.browserSource.browserSync.trim().length).toBeGreaterThan(0);
      expect(locale.settings.appSessions.signIn).toBeUndefined();
      expect(locale.settings.appSessions.youtubeSignIn).toBeUndefined();
    }

    const simplifiedChinese = await Bun.file(
      new URL("../../../shared/i18n/locales/zh-CN.json", import.meta.url),
    ).json();
    expect(simplifiedChinese.browserSource.browserSync).toBe("\u6d4f\u89c8\u5668\u540c\u6b65");
  });

  test("localizes every unavailable browser profile state", async () => {
    const files = [
      "en.json",
      "zh-CN.json",
      "zh-TW.json",
      "ja-JP.json",
      "ko-KR.json",
      "es-419.json",
      "pt-BR.json",
      "id-ID.json",
      "vi-VN.json",
    ];
    let englishLabels: string[] = [];
    for (const fileName of files) {
      const locale = await Bun.file(
        new URL(`../../../shared/i18n/locales/${fileName}`, import.meta.url),
      ).json();
      const labels = [
        locale.browserSource.authorizedRead,
        locale.browserSource.copyAndParse,
        locale.browserSource.currentChromeCDPDescription,
        locale.browserSource.copyAndParseDescription,
        locale.browserSource.prerequisiteDescription,
        locale.browserSource.profileReady,
        locale.browserSource.otherProfiles,
        locale.browserSource.profilePermissionRequired,
        locale.browserSource.profileNoCookies,
        locale.browserSource.profileInvalid,
        locale.browserSource.profileInUse,
        locale.browserSource.profileUnavailable,
      ];
      expect(labels.every((label) => typeof label === "string" && label.trim().length > 0)).toBe(true);
      if (fileName === "en.json") {
        englishLabels = labels;
      } else {
        expect(labels).not.toEqual(englishLabels);
      }
    }
    const simplifiedChinese = await Bun.file(
      new URL("../../../shared/i18n/locales/zh-CN.json", import.meta.url),
    ).json();
    expect(simplifiedChinese.browserSource.authorizedRead).toBe("\u6388\u6743\u8bfb\u53d6");
    expect(simplifiedChinese.browserSource.copyAndParse).toBe("\u590d\u5236\u89e3\u6790");
  });
});
