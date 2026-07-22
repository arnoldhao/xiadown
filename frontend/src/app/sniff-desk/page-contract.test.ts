import { describe, expect, test } from "bun:test";

const sniffPageURL = new URL("./SniffDeskPage.tsx", import.meta.url);
const constellationURL = new URL(
  "./SniffFormatConstellation.tsx",
  import.meta.url,
);
const mainAppURL = new URL("../main/MainApp.tsx", import.meta.url);
const workflowsURL = new URL(
  "../../shared/styles/dream/workflows.css",
  import.meta.url,
);

describe("Sniff Desk workspace page contract", () => {
  test("uses a named custom page recipe with one assistive route heading", async () => {
    const source = await Bun.file(sniffPageURL).text();

    expect(source).toContain("defineWorkspacePageContract({");
    expect(source).toContain('presentation: "primary"');
    expect(source).toContain('recipe: "custom"');
    expect(source).toContain('customContractId: "sniff-desk-primary"');
    expect(source).toContain("routeLabel: text.sniffDesk.title");
    expect(source).toContain('topBar: "drag"');
    expect(source).toContain('heading: "assistive"');
    expect(source).toContain('contentLayout: "custom"');
    expect(source).toContain('footer: "none"');
    expect(source).toContain("<WorkspacePage");
    expect(source).toContain("<WorkspacePageContent");
    expect(source).not.toMatch(/<h1\b/);
  });

  test("delegates scrolling to the virtual result pane", async () => {
    const source = await Bun.file(sniffPageURL).text();

    expect(source).toContain('scroll: "panes"');
    expect(source).toContain(
      'className="app-sniff-desk-virtual-list h-full overflow-y-auto"',
    );
    expect(source).not.toContain("StructuredSniffView");
    expect(source).not.toContain("structuredModeActive");
    expect(source).not.toMatch(
      /app-sniff-desk-content[^"\n]*overflow-y-auto/,
    );
  });

  test("surfaces polling failures and bounds an unfinished start handoff", async () => {
    const source = await Bun.file(sniffPageURL).text();

    expect(source).toContain("resourcesQuery.isError && resources.length === 0");
    expect(source).toContain("sessionsQuery.isError && !currentSession");
    expect(source).toContain(
      'const currentSessionId = currentSession?.sessionId || ""',
    );
    expect(source).not.toContain(
      "currentSession?.sessionId || preferredSessionId",
    );
    expect(source).toContain("SNIFF_WORKSPACE_START_TIMEOUT_MS");
    expect(source).toContain("clearSniffWorkspaceStart(requestId)");
    expect(source).toContain("resourcesQuery.refetch()");
    expect(source).toContain("sessionsQuery.refetch()");
  });

  test("uses shared drag chrome only in workspace layout", async () => {
    const [source, mainSource] = await Promise.all([
      Bun.file(sniffPageURL).text(),
      Bun.file(mainAppURL).text(),
    ]);

    expect(source).toContain("<WorkspacePageTopBar");
    expect(source).toContain(
      "const sessionTitleActions = currentSession ? (",
    );
    expect(source).toContain(
      "<WorkspacePrimaryHeaderActionGroup label={text.sniffDesk.title}>",
    );
    expect(source).toContain("label={text.sniffDesk.clearResources}");
    expect(source).toContain("onClick={() => setClearConfirmOpen(true)}");
    expect(source).toContain("label={cancelSessionLabel}");
    expect(source).toContain("onClick={() => void handleCancelSession()}");
    expect(source).toContain('tone="destructive"');
    expect(source).toContain(
      "reserveWindowControls={props.reserveWindowControls}",
    );
    expect(mainSource).toContain(
      "reserveWindowControls={primaryWindowsChromeVisible}",
    );
    expect(source).not.toContain(
      "pt-[calc(var(--app-windows-caption-button-height)+1.25rem)]",
    );
    expect(source).toContain('!props.workspaceLayout && "pt-2"');
    expect(source).toContain("if (!props.contract)");
    expect(source).toContain("app-sniff-desk-page-toolbar wails-drag");
    expect(source.match(/\{sessionTitleActions\}/g)).toHaveLength(2);
  });

  test("keeps clear and stop in the title rail without duplicate toolbar actions", async () => {
    const source = await Bun.file(sniffPageURL).text();
    const toolbarStart = source.indexOf(
      '<section className="app-sniff-desk-toolbar',
    );
    const contentStart = source.indexOf("{workspaceStartPending ?", toolbarStart);
    const bottomControlStart = source.indexOf(
      "{!props.workspaceLayout && currentSession ? (",
    );
    const pageEnd = source.indexOf("</SniffDeskPageShell>", bottomControlStart);

    expect(toolbarStart).toBeGreaterThan(-1);
    expect(contentStart).toBeGreaterThan(toolbarStart);
    expect(bottomControlStart).toBeGreaterThan(contentStart);
    expect(pageEnd).toBeGreaterThan(bottomControlStart);
    expect(source.slice(toolbarStart, contentStart)).not.toContain(
      "setClearConfirmOpen(true)",
    );
    expect(source.slice(toolbarStart, contentStart)).toContain("<FilterX");
    expect(source.slice(bottomControlStart, pageEnd)).not.toContain(
      "handleCancelSession()",
    );
    expect(source.slice(bottomControlStart, pageEnd)).toContain(
      "setDetailsOpen((open) => !open)",
    );
    expect(source).not.toContain("app-sniff-desk-control-action-danger");
  });

  test("uses the canonical status badge without a feature-owned status palette", async () => {
    const [source, workflowsSource] = await Promise.all([
      Bun.file(sniffPageURL).text(),
      Bun.file(workflowsURL).text(),
    ]);

    expect(source).toContain('from "@/shared/ui/status-badge"');
    expect(source).toContain("<StatusBadge");
    expect(source).toContain("resolveSessionStatusTone(currentSession)");
    for (const tone of ["success", "busy", "warning", "muted", "danger"]) {
      expect(source).toContain(`return "${tone}";`);
    }
    expect(source).not.toContain("app-sniff-desk-status-badge");
    expect(workflowsSource).not.toContain(".app-sniff-desk-status-badge");
    expect(workflowsSource).not.toContain("hsl(142 72% 42% / 0.11)");
    expect(source).not.toMatch(
      /(?:bg-|text-(?:foreground|background|muted|primary|secondary|destructive)|border-|ring-|shadow-|rounded-|backdrop-blur|blur-|font-(?:bold|semibold|medium|mono)|tracking-|uppercase)/,
    );
    expect(source).not.toMatch(
      /style=\{\{[^}]*(?:background|color|boxShadow|borderRadius|filter|font)/s,
    );
  });

  test("keeps the idle primary content to one centered new-sniff action", async () => {
    const [source, mainSource] = await Promise.all([
      Bun.file(sniffPageURL).text(),
      Bun.file(mainAppURL).text(),
    ]);

    expect(source).toContain(
      'className="app-sniff-desk-start-entry flex min-h-0 flex-1 items-center justify-center px-4 py-6"',
    );
    expect(source).toContain("<SniffFormatConstellation");
    expect(source).toContain("burstKey={startBurstKey}");
    expect(source).toContain("onClick={handleStartSniff}");
    expect(source).toMatch(
      /setStartBurstKey\(\(current\) => current \+ 1\);\s+props\.onStartSniff\(\);/,
    );
    expect(mainSource).toContain(
      'onStartSniff={() => openNewTaskDialog("sniff")}',
    );
    expect(source).not.toContain("SniffDeskPetPrompt");
    expect(source).not.toContain("PetDisplay");
    expect(source).not.toContain("urlInputRef");
    expect(source).not.toContain("controlMode");
    expect(source).not.toContain("<BrowserSourceSheet");
    expect(source).not.toContain("useStartResourceSniff");
  });

  test("uses an accessible transparent format constellation instead of another surface", async () => {
    const [constellationSource, workflowsSource] = await Promise.all([
      Bun.file(constellationURL).text(),
      Bun.file(workflowsURL).text(),
    ]);
    const constellationStyles = workflowsSource.slice(
      workflowsSource.indexOf(".app-sniff-desk-start-stage"),
      workflowsSource.indexOf(
        ".app-sniff-desk-start-button.app-running-new-download-button",
      ),
    );

    expect(constellationSource).toContain('aria-hidden="true"');
    expect(constellationSource).toContain("key={burstKey}");
    expect(constellationSource).toContain(
      'className="app-sniff-desk-format-orientation"',
    );
    expect(constellationSource).toContain("resolveResourceKindIcon(item.kind)");
    expect(
      constellationSource.match(
        /kind: "(?:video|audio|subtitle|image|manifest|api|document|font|archive|other)"/g,
      ),
    ).toHaveLength(10);
    expect(constellationSource).not.toContain("resolveItemStyle");
    expect(constellationSource).not.toContain("style={");
    for (const tone of [
      "video",
      "audio",
      "image",
      "manifest",
      "api",
      "document",
      "archive",
      "subtitle",
      "font",
      "other",
    ]) {
      expect(workflowsSource).toContain(
        `.app-sniff-desk-format-item[data-tone="${tone}"]`,
      );
    }
    expect(workflowsSource).toContain("--sniff-format-item-opacity: 0.72");
    expect(workflowsSource).toContain("--sniff-format-scatter-delay: 162ms");
    expect(workflowsSource).toContain("pointer-events: none");
    expect(workflowsSource).toContain(
      "@keyframes app-sniff-desk-format-scatter",
    );
    expect(workflowsSource).toContain(
      "@keyframes app-sniff-desk-format-drift",
    );
    expect(workflowsSource).toContain(
      "animation: app-sniff-desk-format-field-orbit 36s linear infinite",
    );
    expect(workflowsSource).toContain(
      "animation: app-sniff-desk-format-counter-orbit 36s linear infinite",
    );
    expect(workflowsSource).toContain(
      "@media (prefers-reduced-motion: reduce)",
    );
    expect(workflowsSource).toContain("@media (forced-colors: active)");
    expect(constellationStyles).not.toMatch(
      /\b(?:background(?:-color)?|box-shadow|filter|backdrop-filter)\s*:/,
    );
  });

  test("labels every session teardown by the user's stop-sniff intent", async () => {
    const source = await Bun.file(sniffPageURL).text();

    expect(source).toContain(
      "const cancelSessionLabel = text.sniffDesk.stopSniff",
    );
    expect(source).not.toContain(
      'normalized(currentSession?.mode) === "current_browser"',
    );
    expect(source).toContain("title: cancelSessionLabel");
    expect(source).toContain("label={cancelSessionLabel}");
    expect(source).toContain('tone="destructive"');
  });

  test("delegates the browser and profile steps to the same new-sniff dialog", async () => {
    const [dialogSource, stepsSource, querySource, contractSource, buttonContract] =
      await Promise.all([
        Bun.file(new URL("../main/NewTaskDialog.tsx", import.meta.url)).text(),
        Bun.file(
          new URL("../main/NewSniffSourceSteps.tsx", import.meta.url),
        ).text(),
        Bun.file(
          new URL("../../shared/query/library.ts", import.meta.url),
        ).text(),
        Bun.file(
          new URL("../../shared/contracts/library.ts", import.meta.url),
        ).text(),
        Bun.file(
          new URL("../../shared/styles/dream/button-contract.css", import.meta.url),
        ).text(),
      ]);

    expect(dialogSource).toContain("<NewSniffSourceSteps");
    expect(dialogSource).toContain('{activeMode === "sniff" ? (');
    expect(dialogSource).toContain('url: ""');
    expect(dialogSource).not.toContain("pendingResourceSniffStart");
    expect(dialogSource).not.toContain("<BrowserSourceSheet");
    expect(stepsSource).toContain('useBrowserSources("network_sniff", true)');
    expect(stepsSource).toContain('data-step={step}');
    expect(stepsSource).toContain('setStep("profile")');
    expect(stepsSource).toContain("<BrowserBrandIcon");
    expect(stepsSource).toContain("app-new-task-sniff-browser-option");
    expect(stepsSource).not.toContain("app-new-task-sniff-browser-card");
    expect(stepsSource).toContain(
      "flex flex-wrap justify-center gap-x-6 gap-y-3",
    );
    expect(stepsSource).toContain('role="radiogroup"');
    expect(stepsSource).toContain('data-choice="browser-default"');
    expect(stepsSource).toContain('data-choice="xiadown-managed"');
    expect(stepsSource).toContain('browserSource.browserDefault');
    expect(stepsSource).toContain('browserSource.xiadownManaged');
    expect(stepsSource).toContain("app-new-task-sniff-profile-choice");
    expect(buttonContract).toContain(".app-new-task-sniff-profile-choice");
    expect(buttonContract).toContain("min-height: 4.5rem");
    expect(stepsSource).toContain(
      "managedProfiles.find((profile) => profile.isDefault)",
    );
    expect(stepsSource).toContain("currentChromeStatus.refetch()");
    expect(stepsSource).toContain(
      'onClick={() => startWithMode("current_browser")}',
    );
    expect(stepsSource).toContain(
      'onClick={() => startWithMode("xiadown_profile")}',
    );
    expect(stepsSource).not.toContain("confirmLabel");
    expect(stepsSource).toContain('mode: "current_browser"');
    expect(stepsSource).toContain('mode: "xiadown_profile"');
    expect(stepsSource).toContain("useCurrentResourceSniffBrowserStatus");
    expect(stepsSource).toContain(
      'disabled={!currentChromeReady || props.confirming}',
    );
    expect(stepsSource).not.toContain('browserSource.currentChromeScope');
    expect(stepsSource).not.toContain('currentChromeGuide(');
    expect(stepsSource).not.toContain('browserSource.chooseProfile');
    expect(stepsSource).not.toContain('profiles.map(');
    expect(stepsSource).not.toContain('selectedProfileKey');
    expect(stepsSource).not.toContain('min-h-36');
    expect(stepsSource).not.toContain('<ManagedProfileAvatar');
    expect(stepsSource).not.toContain("ChevronRight");
    expect(stepsSource).not.toContain("DialogDescription");
    expect(stepsSource).not.toContain("border-t border-border/60");
    expect(stepsSource).not.toContain(
      "profile.subtitle || selectedBrowser?.label",
    );
    expect(stepsSource).toContain('browser.id !== "safari"');
    expect(dialogSource).toContain(
      'currentBrowserMode ? "current_browser" : "managed_profile"',
    );
    expect(dialogSource).toContain("browserId: selection.browserId");
    expect(dialogSource).toContain(
      "currentBrowserMode ? {} : { profileId: selection.profileId }",
    );
    expect(querySource).toContain("`${LIBRARY_HANDLER_SERVICE}.StartResourceSniff`");
    expect(querySource).not.toContain(
      "LibraryBindings.StartResourceSniffRequest.createFrom(request)",
    );
    expect(contractSource).toContain(
      'mode?: "browser_profile" | "current_browser" | "managed_profile"',
    );
  });
});
