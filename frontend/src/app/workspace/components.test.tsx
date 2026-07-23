import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import { AccountDock, AccountDockProfile } from "./AccountDock";
import { ActivityDock } from "./ActivityDock";
import { AppShell } from "./AppShell";
import {
  CompanionPanel,
  resolveCompanionScrollChromeKey,
} from "./CompanionPanel";
import { PrimaryPane } from "./PrimaryPane";
import { WorkspaceSidebar } from "./WorkspaceSidebar";
import { WorkspaceStage } from "./WorkspaceStage";

describe("workspace layout components", () => {
  test("reports the sidebar + primary + companion minimum width", () => {
    const markup = renderToStaticMarkup(
      <AppShell
        companionOpen
        navigation={<WorkspaceSidebar />}
      >
        <WorkspaceStage companionOpen />
      </AppShell>,
    );

    expect(markup).toContain('data-required-width="1414"');
    expect(markup).toContain("min-width:1414px");
    expect(markup).toContain("--app-workspace-companion-width:390px");
    expect(markup).toContain("app-workspace-ambient-canvas");
    expect(markup).toContain('data-has-artwork="false"');
  });

  test("renders an optional artwork image inside the ambient canvas", () => {
    const markup = renderToStaticMarkup(
      <AppShell
        ambientArtworkURL="  https://media.example/cover.jpg  "
        navigation={<WorkspaceSidebar />}
      />,
    );

    expect(markup).toContain('data-has-artwork="true"');
    expect(markup).toContain("app-workspace-ambient-canvas__artwork");
    expect(markup).toContain('src="https://media.example/cover.jpg"');
    expect(markup).not.toContain("url(https://media.example/cover.jpg)");
  });

  test("marks primary density while chrome uses childless glass samplers", () => {
    const sidebarMarkup = renderToStaticMarkup(
      <WorkspaceSidebar>navigation</WorkspaceSidebar>,
    );
    const solidSidebarMarkup = renderToStaticMarkup(
      <WorkspaceSidebar glass={false}>navigation</WorkspaceSidebar>,
    );
    const primaryMarkup = renderToStaticMarkup(<PrimaryPane>content</PrimaryPane>);
    const dockedCompanionMarkup = renderToStaticMarkup(
      <CompanionPanel open>details</CompanionPanel>,
    );
    const overlayCompanionMarkup = renderToStaticMarkup(
      <CompanionPanel open presentation="overlay">details</CompanionPanel>,
    );

    expect(sidebarMarkup).toContain('data-glass-host="true"');
    expect(sidebarMarkup).toContain('data-surface-role="chrome"');
    expect(sidebarMarkup).toContain('data-glass-role="sidebar"');
    expect(sidebarMarkup).toContain('data-material="regular"');
    expect(sidebarMarkup).toContain('data-elevation="embedded"');
    expect(solidSidebarMarkup).toContain('data-glass-host="false"');
    expect(solidSidebarMarkup).not.toContain("app-workspace-chrome-material");
    expect(primaryMarkup).toContain('data-surface-role="content"');
    expect(primaryMarkup).toContain('data-surface-density="high"');
    expect(primaryMarkup).toContain('data-surface-style="glass"');
    expect(dockedCompanionMarkup).toContain('data-glass-role="companion"');
    expect(dockedCompanionMarkup).toContain('data-surface-role="chrome"');
    expect(dockedCompanionMarkup).toContain('data-material="regular"');
    expect(dockedCompanionMarkup).toContain('data-elevation="embedded"');
    expect(overlayCompanionMarkup).toContain('data-material="panel"');
    expect(overlayCompanionMarkup).toContain('data-surface-role="overlay"');
    expect(overlayCompanionMarkup).toContain('data-elevation="floating"');
  });

  test("propagates surface style while keeping overlay companion glass", () => {
    const contrastDockedMarkup = renderToStaticMarkup(
      <AppShell
        companionOpen
        navigation={<WorkspaceSidebar>navigation</WorkspaceSidebar>}
        surfaceStyle="contrast"
      >
        <PrimaryPane>content</PrimaryPane>
        <CompanionPanel open>details</CompanionPanel>
      </AppShell>,
    );
    const contrastOverlayMarkup = renderToStaticMarkup(
      <AppShell
        companionOpen
        companionPresentation="overlay"
        navigation={<WorkspaceSidebar>navigation</WorkspaceSidebar>}
        surfaceStyle="contrast"
      >
        <PrimaryPane>content</PrimaryPane>
        <CompanionPanel open presentation="overlay">
          details
        </CompanionPanel>
      </AppShell>,
    );

    expect(contrastDockedMarkup).toContain('data-surface-style="contrast"');
    expect(contrastDockedMarkup).toMatch(
      /app-workspace-sidebar[^>]*data-glass-host="false"[^>]*data-surface-style="contrast"/,
    );
    expect(contrastDockedMarkup).toMatch(
      /app-workspace-primary-pane[^>]*data-surface-density="high"[^>]*data-surface-role="content"[^>]*data-surface-style="contrast"/,
    );
    expect(contrastDockedMarkup).toMatch(
      /app-workspace-companion[^>]*data-glass-host="false"[^>]*data-presentation="docked"[^>]*data-surface-style="contrast"/,
    );
    expect(contrastDockedMarkup).not.toContain(
      "app-workspace-chrome-material",
    );

    expect(contrastOverlayMarkup).toMatch(
      /app-workspace-sidebar[^>]*data-glass-host="false"[^>]*data-surface-style="contrast"/,
    );
    expect(contrastOverlayMarkup).toMatch(
      /app-workspace-companion[^>]*data-glass-host="true"[^>]*data-presentation="overlay"[^>]*data-surface-style="contrast"/,
    );
    expect(contrastOverlayMarkup).toContain('data-glass-role="companion"');
    expect(contrastOverlayMarkup).toContain('data-material="panel"');
    expect(contrastOverlayMarkup).toContain('data-elevation="floating"');
  });

  test("wires the product surface style instead of sidebar-only structure", async () => {
    const source = await Bun.file(
      new URL("../main/MainApp.tsx", import.meta.url),
    ).text();

    expect(source).toContain(
      'data-surface-style={appearance.surfaceStyle}',
    );
    expect(source).toContain('surfaceStyle={appearance.surfaceStyle}');
    expect(source).toContain(
      "resolveSidebarSurface(theme.id, appearance.surfaceStyle, shellTheme)",
    );
    expect(source).toMatch(
      /scrollChrome=\{[\s\S]*?playerFullscreen \|\|[\s\S]*?companion\.destination\?\.id === "player" \|\|[\s\S]*?companion\.destination\?\.id === "lyrics"[\s\S]*?\? "off"/,
    );
    expect(source).not.toContain("data-sidebar-style=");
    expect(source).not.toContain("appearance.sidebarStyle");
  });

  test("removes the default Dock and keeps the wide status-card order", async () => {
    const source = await Bun.file(
      new URL("../main/MainApp.tsx", import.meta.url),
    ).text();

    expect(source).not.toContain("const compactActivityDock");
    expect(source).not.toContain("const defaultNavigation");
    expect(source).not.toContain("<StationDock");
    expect(source).toContain(
      "const expandedActivityDock = hasExpandedActivity ? (",
    );
    const expandedActivitySource = source.slice(
      source.indexOf("const expandedActivityDock"),
      source.indexOf("const workspaceHeader"),
    );
    expect(
      expandedActivitySource.indexOf("<WidePlaybackActivity"),
    ).toBeGreaterThan(
      expandedActivitySource.indexOf("<WideOperationActivity"),
    );
    expect(
      expandedActivitySource.indexOf("<SniffWorkspaceSessionActivity"),
    ).toBeGreaterThan(
      expandedActivitySource.indexOf("<WidePlaybackActivity"),
    );
  });

  test("does not add companion width when the panel overlays", () => {
    const markup = renderToStaticMarkup(
      <AppShell
        companionOpen
        companionPresentation="overlay"
        navigation={<WorkspaceSidebar />}
      >
        <WorkspaceStage companionOpen companionPresentation="overlay" />
      </AppShell>,
    );

    expect(markup).toContain('data-required-width="1024"');
    expect(markup).toContain('data-companion-presentation="overlay"');
  });

  test("keeps a closed companion mounted unless explicitly disabled", () => {
    const mounted = renderToStaticMarkup(
      <CompanionPanel open={false}>player-runtime</CompanionPanel>,
    );
    const unmounted = renderToStaticMarkup(
      <CompanionPanel keepMounted={false} open={false}>
        player-runtime
      </CompanionPanel>,
    );

    expect(mounted).toContain("player-runtime");
    expect(mounted).toContain("hidden");
    expect(unmounted).toBe("");
  });

  test("keeps companion header, content and footer in a stable layout order", () => {
    const markup = renderToStaticMarkup(
      <CompanionPanel
        open
        header={<span data-region="header">close</span>}
        footer={<span data-region="footer">actions</span>}
      >
        <span data-region="content">details</span>
      </CompanionPanel>,
    );

    const headerIndex = markup.indexOf('data-region="header"');
    const contentIndex = markup.indexOf('data-region="content"');
    const footerIndex = markup.indexOf('data-region="footer"');
    expect(headerIndex).toBeGreaterThan(-1);
    expect(contentIndex).toBeGreaterThan(headerIndex);
    expect(footerIndex).toBeGreaterThan(contentIndex);
    expect(markup).toContain("app-workspace-companion__header wails-drag");
    expect(markup).toContain("app-workspace-companion__content");
    expect(markup).toContain("app-workspace-companion__footer");
    expect(markup).toContain('data-has-footer="true"');
    expect(markup).toContain('data-scroll-state="top"');
    expect(markup).not.toContain("data-footer-state");
    const headerMaterial = (markup.match(/<div\b[^>]*>/g) ?? []).find(
      (tag) => tag.includes("app-workspace-companion__header-material"),
    );
    expect(headerMaterial).toContain('data-glass-role="header"');
    expect(headerMaterial).toContain('data-elevation="embedded"');
    expect(headerMaterial).toContain('data-surface-role="chrome"');
    expect(headerMaterial).toContain('data-material="regular"');
    expect(markup).not.toContain("app-workspace-companion__footer-material");
    expect(markup).not.toContain('data-glass-role="footer"');
  });

  test("arms scroll-aware chrome only for an active destination", () => {
    const destination = { id: "queue", scope: { kind: "global" as const } };
    const active = renderToStaticMarkup(
      <CompanionPanel
        open
        destination={destination}
        header={<span>Queue</span>}
        footer={<span>Transport</span>}
      >
        <div data-companion-scroll-owner="queue">tracks</div>
      </CompanionPanel>,
    );
    const disabled = renderToStaticMarkup(
      <CompanionPanel
        open
        destination={destination}
        scrollChrome="off"
      >
        player
      </CompanionPanel>,
    );

    expect(active).toContain('data-scroll-chrome="active"');
    expect(active).toContain('data-scroll-state="top"');
    expect(active).toContain('data-companion-scroll-owner="queue"');
    expect(active).toContain('data-glass-role="header"');
    expect(active).not.toContain("data-footer-state");
    expect(active).not.toContain('data-glass-role="footer"');
    expect(disabled).toContain('data-scroll-chrome="off"');
    expect(disabled).toContain('data-scroll-state="top"');
    expect(disabled).not.toContain("data-footer-state");
  });

  test("derives title chrome and safe insets from the active Companion scroll owner", async () => {
    const source = await Bun.file(
      new URL("./CompanionPanel.tsx", import.meta.url),
    ).text();

    expect(source).toContain("isWorkspacePageHeaderScrolled");
    expect(source).toContain("owner?.scrollTop");
    expect(source).toContain("new ResizeObserver(");
    expect(source).toContain(
      'data-scroll-state={headerIsScrolled ? "scrolled" : "top"}',
    );
    expect(source).toContain("--app-workspace-companion-header-inset");
    expect(source).toContain("--app-workspace-companion-footer-inset");
    expect(source).not.toContain("isWorkspacePageFooterCovered");
    expect(source).not.toContain("data-footer-state");
  });

  test("shares Companion title chrome between docked and overlay presentations", () => {
    const renderPresentation = (presentation: "docked" | "overlay") =>
      renderToStaticMarkup(
        <CompanionPanel
          open
          presentation={presentation}
          title="Queue"
          footer={<span>Transport</span>}
        >
          tracks
        </CompanionPanel>,
      );
    const docked = renderPresentation("docked");
    const overlay = renderPresentation("overlay");

    for (const markup of [docked, overlay]) {
      expect(markup.match(/data-glass-role="header"/g)).toHaveLength(1);
      expect(markup).not.toContain('data-glass-role="footer"');
      expect(markup).toContain("app-workspace-companion__header-material");
      expect(markup).not.toContain("app-workspace-companion__footer-material");
    }
    expect(docked).toContain('data-presentation="docked"');
    expect(docked).toContain('data-glass-role="companion"');
    expect(docked).toContain('data-elevation="embedded"');
    expect(overlay).toContain('data-presentation="overlay"');
    expect(overlay).toContain('data-surface-role="overlay"');
    expect(overlay).toContain('data-elevation="floating"');
  });

  test("keys Companion chrome by scope and context, not destination id alone", () => {
    const first = {
      id: "library-preview",
      scope: { kind: "route" as const, workspaceId: "library", routeId: "all" },
      context: { source: "catalog", itemId: "item-1" },
    };
    const reordered = {
      ...first,
      context: { itemId: "item-1", source: "catalog" },
    };
    const second = {
      ...first,
      context: { source: "catalog", itemId: "item-2" },
    };

    expect(resolveCompanionScrollChromeKey(first)).toBe(
      resolveCompanionScrollChromeKey(reordered),
    );
    expect(resolveCompanionScrollChromeKey(first)).not.toBe(
      resolveCompanionScrollChromeKey(second),
    );
  });

  test("renders the sidebar docks only when they have content", () => {
    expect(
      renderToStaticMarkup(<ActivityDock>activity-status</ActivityDock>),
    ).toContain("activity-status");
    expect(renderToStaticMarkup(<ActivityDock />)).toBe("");
    expect(
      renderToStaticMarkup(<AccountDock>account-details</AccountDock>),
    ).toContain("account-details");
    expect(renderToStaticMarkup(<AccountDock />)).toBe("");
  });

  test("combines account identity and disclosure without nesting actions", () => {
    const markup = renderToStaticMarkup(
      <AccountDock>
        <div className="app-workspace-account-row">
          <AccountDockProfile
            aria-label="workspace.switch"
            avatar={<span>profile.avatar</span>}
            disclosure={<span>station.switch</span>}
            name="profile.name"
            statusIndicator={<span>update.ready</span>}
            username="profile.username"
          />
          <button type="button">task.new</button>
        </div>
      </AccountDock>,
    );

    expect(markup).toContain("app-workspace-account-row");
    expect(markup).toContain('aria-label="workspace.switch"');
    expect(markup).toContain(
      'class="app-workspace-account-profile wails-no-drag"',
    );
    expect(markup).toContain('class="app-workspace-account-profile__avatar"');
    expect(markup).toContain('aria-hidden="true"');
    expect(markup).toContain("profile.name");
    expect(markup).toContain("profile.username");
    expect(markup).toContain("update.ready");
    expect(markup).toContain("station.switch");
    expect(markup).toContain("task.new");
    expect(markup.match(/<button/g)).toHaveLength(2);
  });
});
