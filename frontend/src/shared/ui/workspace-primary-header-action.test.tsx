import { describe, expect, test } from "bun:test";
import { RefreshCw } from "lucide-react";
import type { ComponentProps } from "react";
import { renderToStaticMarkup } from "react-dom/server";

import {
  WorkspacePrimaryHeaderAction,
  WorkspacePrimaryHeaderActionGroup,
} from "./workspace-primary-header-action";

describe("WorkspacePrimaryHeaderAction", () => {
  test("owns the shared icon-only Primary header contract", () => {
    const markup = renderToStaticMarkup(
      <WorkspacePrimaryHeaderAction
        aria-pressed
        label="Refresh"
        onClick={() => undefined}
      >
        <RefreshCw />
      </WorkspacePrimaryHeaderAction>,
    );

    expect(markup).toContain('aria-label="Refresh"');
    expect(markup).toContain('aria-pressed="true"');
    expect(markup).toContain('data-variant="ghost"');
    expect(markup).toContain('data-size="compactIcon"');
    expect(markup).toContain('data-shape="circle"');
    expect(markup).toContain("app-workspace-primary-header-action");
    expect(markup).toContain("wails-no-drag");
    expect(markup).toContain("<svg");
  });

  test("cannot be changed into a non-button through asChild", () => {
    const unsafeProps = { asChild: true } as unknown as ComponentProps<
      typeof WorkspacePrimaryHeaderAction
    >;
    const markup = renderToStaticMarkup(
      <WorkspacePrimaryHeaderAction {...unsafeProps} label="Refresh">
        <RefreshCw />
      </WorkspacePrimaryHeaderAction>,
    );

    expect(markup).toContain("<button");
    expect(markup).toContain('type="button"');
  });

  test("groups borderless title actions and centers their menus", async () => {
    const markup = renderToStaticMarkup(
      <WorkspacePrimaryHeaderActionGroup label="View controls">
        <WorkspacePrimaryHeaderAction label="Sort">
          <RefreshCw />
        </WorkspacePrimaryHeaderAction>
      </WorkspacePrimaryHeaderActionGroup>,
    );
    const [source, css] = await Promise.all([
      Bun.file(
        new URL("./workspace-primary-header-action.tsx", import.meta.url),
      ).text(),
      Bun.file(
        new URL("../styles/dream/layout-contract.css", import.meta.url),
      ).text(),
    ]);

    expect(markup).toContain('role="group"');
    expect(markup).toContain('aria-label="View controls"');
    expect(markup).toContain("app-workspace-primary-header-action-group");
    expect(source).toContain("app-workspace-primary-header-menu");
    expect(source).toContain('align="center"');
    expect(source).toContain('side="bottom"');
    expect(css).toMatch(
      /\.app-workspace-primary-header-action-group\s*\{[^}]*gap:\s*var\(--app-workspace-header-action-gap\)/s,
    );
    expect(css).toMatch(
      /\.app-workspace-primary-header-action-group\s*\+\s*\.app-workspace-primary-header-action-group\s*\{[^}]*margin-inline-start:\s*var\(--app-workspace-header-group-gap\)/s,
    );
    expect(css).toMatch(
      /\.app-workspace-primary-header-action\[aria-pressed="true"\]\s*\{[^}]*background:\s*var\(--dream-surface-pressed\)/s,
    );
  });

  test("keeps Radio and RSS on the same shared action primitive", async () => {
    const [radioSource, rssSource, pageSource, librarySource, localSource] =
      await Promise.all([
        Bun.file(
          new URL("../../app/main/listen/HushLiveList.tsx", import.meta.url),
        ).text(),
        Bun.file(
          new URL("../../app/rss/RSSWorkspacePage.tsx", import.meta.url),
        ).text(),
        Bun.file(
          new URL("../../app/main/listen/PageView.tsx", import.meta.url),
        ).text(),
        Bun.file(
          new URL(
            "../../app/library/LibraryWorkspacePage.tsx",
            import.meta.url,
          ),
        ).text(),
        Bun.file(
          new URL(
            "../../app/main/listen/LocalLibraryWorkspace.tsx",
            import.meta.url,
          ),
        ).text(),
      ]);

    expect(
      radioSource.match(/<WorkspacePrimaryHeaderAction(?:\s|>)/g),
    ).toHaveLength(3);
    expect(radioSource).not.toContain("HeaderToolbarButton");
    expect(radioSource).toContain("<WorkspacePrimaryHeaderActionGroup");
    expect(radioSource).not.toContain(
      'className="app-dream-button-group app-completed-toolbar-actions inline-flex h-9 shrink-0 items-center p-0.5"',
    );
    expect(rssSource).toContain(
      "WorkspacePrimaryHeaderAction as RSSHeaderAction",
    );
    expect(rssSource).toContain("<WorkspacePrimaryHeaderActionGroup");
    expect(rssSource).toContain("<WorkspacePrimaryHeaderMenuContent");
    expect(rssSource).not.toContain('<DropdownMenuContent align="start"');
    expect(rssSource).not.toContain("function RSSHeaderAction");
    expect(pageSource).toContain("LISTEN_HEADER_HUSH_ACTIONS_REM = 7.25");
    for (const source of [librarySource, localSource]) {
      expect(source).toContain("<WorkspacePrimaryHeaderActionGroup");
      expect(source).toContain("<WorkspacePrimaryHeaderMenuContent");
      expect(source).not.toMatch(/DropdownMenuContent align="(?:start|end)"/);
    }
  });
});
