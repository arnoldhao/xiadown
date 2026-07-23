import { describe, expect, test } from "bun:test";

import { TASK_DIALOG_DEPENDENCIES_BY_MODE } from "./main-constants";

describe("main dialogs", () => {
  test("gates each new-task mode only on tools used by that entry flow", () => {
    expect(TASK_DIALOG_DEPENDENCIES_BY_MODE).toEqual({
      download: ["yt-dlp", "ffmpeg"],
      transcode: ["ffmpeg"],
      sniff: [],
    });
  });

  test("keeps What's New on the shared semantic panel material", async () => {
    const [source, dreamComponents] = await Promise.all([
      Bun.file(new URL("./dialogs.tsx", import.meta.url)).text(),
      Bun.file(new URL("../../shared/styles/dream/components.css", import.meta.url)).text(),
    ]);

    expect(source).toContain("<DialogContent");
    expect(source).toContain("app-whats-new-eyebrow");
    expect(source).toContain("app-whats-new-changelog");
    expect(dreamComponents).toContain(".app-whats-new-eyebrow");
    expect(dreamComponents).toContain(".app-whats-new-changelog");
    expect(source).not.toContain("bg-transparent");
    expect(source).not.toContain("linear-gradient");
    expect(source).not.toContain("text-slate-");
    expect(source).not.toContain("border-white/");
  });

  test("opens the three new-task surfaces from the sidebar menu without dialog tabs", async () => {
    const [source, mainApp, constants] = await Promise.all([
      Bun.file(new URL("./NewTaskDialog.tsx", import.meta.url)).text(),
      Bun.file(new URL("./MainApp.tsx", import.meta.url)).text(),
      Bun.file(new URL("./main-constants.tsx", import.meta.url)).text(),
    ]);

    expect(mainApp).toContain("<DropdownMenuContent");
    expect(mainApp).toContain("className={SIDEBAR_DROPDOWN_CONTENT_CLASS_NAME}");
    expect(mainApp).toContain('openNewTaskDialog("download")');
    expect(mainApp).toContain('openNewTaskDialog("transcode")');
    expect(mainApp).toContain('openNewTaskDialog("sniff")');
    expect(mainApp).toContain('className={SIDEBAR_DROPDOWN_ITEM_CLASS_NAME}');
    expect(mainApp).toContain('<Download className="h-4 w-4 shrink-0" />');
    expect(mainApp).toContain('<FileCog className="h-4 w-4 shrink-0" />');
    expect(mainApp).toContain('<Radar className="h-4 w-4 shrink-0" />');
    expect(mainApp).toContain('className="min-w-0 flex-1 truncate"');
    expect(source).not.toContain("app-new-task-mode-switch");
    expect(source).toContain("handlePasteDownloadURL");
    expect(source).toContain("transcodeLibrarySources");
    expect(source).toContain("handleChooseLibrarySource");
    expect(mainApp).toContain(
      'const transcodeCatalogItemsQuery = useCompleteCatalogItems(\n    { category: "video", status: "active" },\n    newTaskDialogOpen && newTaskDialogMode === "transcode",\n  );',
    );
    expect(mainApp).toContain(
      'const catalogItemsQuery = useCompleteCatalogItems(\n    { status: "all", excludeTrashed: true },\n    completeCatalogNeeded,\n  );',
    );
    expect(mainApp).toContain(
      'category: libraryCatalogCategory(libraryContentRoute),\n    status: "all",\n    excludeTrashed: true,',
    );
    const transcodeSourcesStart = mainApp.indexOf(
      "const transcodeLibrarySources",
    );
    const transcodeSourcesEnd = mainApp.indexOf(
      "const librariesById",
      transcodeSourcesStart,
    );
    const transcodeSources = mainApp.slice(
      transcodeSourcesStart,
      transcodeSourcesEnd,
    );
    expect(transcodeSources).toContain(
      "transcodeCatalogItemsQuery.data?.items",
    );
    expect(transcodeSources).not.toContain("catalogItemsQuery.data");
    expect(mainApp).toContain(
      "transcodeLibraryLoading={transcodeCatalogItemsQuery.isFetching}",
    );
    expect(mainApp).toContain("transcodeLibraryError={");
    expect(mainApp).toContain("onRetryTranscodeLibrary={() => void transcodeCatalogItemsQuery.refetch()}");
    expect(source).toContain("props.transcodeLibraryLoading");
    expect(source).toContain('role="status"');
    expect(source).toContain("props.transcodeLibraryError");
    expect(source).toContain('role="alert"');
    expect(source).toContain("props.onRetryTranscodeLibrary");
    expect(source).toContain("TASK_DIALOG_DEPENDENCIES_BY_MODE[activeMode]");
    expect(constants).toContain('download: ["yt-dlp", "ffmpeg"]');
    expect(constants).toContain('transcode: ["ffmpeg"]');
    expect(constants).toContain("sniff: []");
    expect(source).toContain('aria-label={text.dialogs.selectFromLibrary}');
    expect(source).toContain('title={text.actions.close}');
    expect(source).toContain('const activeDownloadMode = activeMode === "download"');
    expect(source).toContain('{activeMode === "sniff" ? (');
    expect(source).toContain("<NewSniffSourceSteps");
    expect(source).not.toContain("pendingResourceSniffStart");
    expect(source).toContain("transferResourceSniffToDesk(startVersion)");
    expect(source).toContain("beginSniffWorkspaceStart()");
    expect(source).toContain("attachSniffWorkspaceStartSession(");
    expect(source).toContain("messageBus.publishToast");
  });

  test("chooses current Chrome or a XiaDown-managed profile before every new-task sniff launch", async () => {
    const [source, steps] = await Promise.all([
      Bun.file(new URL("./NewTaskDialog.tsx", import.meta.url)).text(),
      Bun.file(new URL("./NewSniffSourceSteps.tsx", import.meta.url)).text(),
    ]);

    expect(source).toContain("<NewSniffSourceSteps");
    expect(source).not.toContain("<BrowserSourceSheet");
    expect(source).not.toContain("resourceSniffSourceSheetOpen");
    expect(steps).toContain('useBrowserSources("network_sniff", true)');
    expect(steps).toContain('data-step={step}');
    expect(steps).toContain('setStep("profile")');
    expect(steps).toContain("<BrowserBrandIcon");
    expect(steps).toContain("app-new-task-sniff-browser-option");
    expect(steps).not.toContain("app-new-task-sniff-browser-card");
    expect(steps).toContain(
      'className="app-new-task-sniff-source-steps overflow-hidden"',
    );
    expect(steps).not.toContain(
      'app-new-task-sniff-source-steps overflow-hidden rounded-',
    );
    expect(steps).not.toContain("<ManagedProfileAvatar");
    expect(steps).toContain("!profile.redundant");
    expect(steps).toContain(
      "managedProfiles.find((profile) => profile.isDefault)",
    );
    expect(steps).toContain(
      'defaultProfile(selectedBrowser, t("browserSource.defaultProfile"))',
    );
    expect(steps).not.toContain("managedProfiles[0]");
    expect(steps).toContain("managedProfileLabel");
    expect(steps).toContain("useCurrentResourceSniffBrowserStatus");
    expect(steps).toContain("canUseCurrentChrome({");
    expect(steps).toContain("currentChromeEntryDataUpdatedAt");
    expect(steps).toContain(
      'browser.id === "chrome" ? currentChromeStatus.dataUpdatedAt : null',
    );
    expect(steps).toContain('grid-cols-2');
    expect(steps).toContain('role="radiogroup"');
    expect(steps).toContain('data-choice="browser-default"');
    expect(steps).toContain('data-choice="xiadown-managed"');
    expect(steps).toContain('browserSource.browserDefault');
    expect(steps).toContain('browserSource.xiadownManaged');
    expect(steps).toContain("app-new-task-sniff-profile-choice");
    expect(steps).toContain("<Button");
    expect(steps).toContain("<StatusBadge");
    expect(steps).not.toContain("<button");
    expect(steps).toContain("currentChromeStatus.refetch()");
    expect(steps).toContain('disabled={!currentChromeReady || props.confirming}');
    expect(steps).toContain('mode: "current_browser"');
    expect(steps).toContain('mode: "xiadown_profile"');
    expect(steps).not.toContain('browserSource.currentChromeScope');
    expect(steps).not.toContain('currentChromeGuide(');
    expect(steps).not.toContain('browserSource.chooseProfile');
    expect(steps).not.toContain('profiles.map(');
    expect(steps).not.toContain('selectedProfileKey');
    expect(steps).not.toContain('min-h-36');
    expect(steps).toContain("flex flex-wrap justify-center gap-x-6 gap-y-3");
    expect(steps).toContain("app-new-task-sniff-browser-option__icon");
    expect(steps).toContain("app-new-task-sniff-browser-option__label");
    expect(steps).not.toContain("flex h-14 w-14");
    expect(steps).toContain('role="radiogroup"');
    expect(steps).toContain('onClick={() => startWithMode("current_browser")}');
    expect(steps).toContain('onClick={() => startWithMode("xiadown_profile")}');
    expect(steps).not.toContain("confirmLabel");
    expect(steps).not.toContain("canConfirm");
    expect(steps).not.toContain("ChevronRight");
    expect(steps).not.toContain("DialogDescription");
    expect(steps).not.toContain("useRefreshBrowserSources");
    expect(steps).not.toContain("border-t border-border/60");
    expect(steps).not.toContain(
      "profile.subtitle || selectedBrowser?.label",
    );
    expect(steps).toContain("onClick={() => void sources.refetch()}");
    expect(steps).toContain('browser.id !== "safari"');
    expect(source).toContain("await startResourceSniffFromSelection(selection, {");
    expect(source).toContain('url: ""');
    expect(source).not.toContain("requestResourceSniffStart");
    expect(source).not.toContain("startResourceSniffURL");
    expect(source).toContain('currentBrowserMode ? "current_browser" : "managed_profile"');
    expect(source).toContain("browserId: selection.browserId");
    expect(source).toContain("currentBrowserMode ? {} : { profileId: selection.profileId }");
    expect(source).not.toContain(
      "startResourceSniff.mutateAsync({\n        url,\n      })",
    );

    const confirmStart = source.indexOf(
      "const handleConfirmResourceSniffSource",
    );
    const confirmEnd = source.indexOf(
      "const resolveDownloadErrorMessage",
      confirmStart,
    );
    const confirmBody = source.slice(confirmStart, confirmEnd);
    expect(confirmBody).toContain('selection.mode === "current_browser"');
    expect(confirmBody).toContain('selection.mode === "xiadown_profile"');
    expect(confirmBody).toContain('selection.browserId === "chrome"');
    expect(confirmBody).toContain("await startResourceSniffFromSelection(");
    expect(confirmBody).toContain("resourceSniffConfirmingRef.current");
    const latchSet = confirmBody.indexOf(
      "resourceSniffConfirmingRef.current = true",
    );
    const startCall = confirmBody.indexOf("await startResourceSniffFromSelection(");
    const latchReset = confirmBody.indexOf(
      "resourceSniffConfirmingRef.current = false",
    );
    expect(latchSet).toBeGreaterThanOrEqual(0);
    expect(startCall).toBeGreaterThan(latchSet);
    expect(latchReset).toBeGreaterThan(startCall);
  });
});
