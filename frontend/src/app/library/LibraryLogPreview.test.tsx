import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import { t } from "@/shared/i18n";

import { LibraryLogPreview } from "./LibraryLogPreview";
import { createLibraryWorkspaceLabels } from "./types";

describe("LibraryLogPreview", () => {
  test("uses the shared iPod shell and defers the bounded text dialog request", async () => {
    const labels = createLibraryWorkspaceLabels((key) => t(key, "en"), "en");
    const markup = renderToStaticMarkup(
      <LibraryLogPreview
        labels={labels}
        sourceURL="http://127.0.0.1:43127/api/library/card-preview/log/item-log?v=1"
        title="download.log"
      />,
    );
    const source = await Bun.file(
      new URL("./LibraryLogPreview.tsx", import.meta.url),
    ).text();
    const workflows = await Bun.file(
      new URL("../../shared/styles/dream/workflows.css", import.meta.url),
    ).text();

    expect(markup).toContain('data-media-kind="log"');
    expect(markup).toContain("app-library-ipod__screen");
    expect(markup).toContain("app-library-ipod__wheel");
    expect(markup.match(/app-library-ipod__wheel-button/g)).toHaveLength(4);
    expect(markup).toContain("app-library-log-ipod__display");
    expect(source).toContain('url.searchParams.set("detail", "1")');
    expect(source).toContain("if (!dialogOpen || !sourceURL || detail) return");
    expect(source).toContain(
      'className="app-library-log-dialog app-media-preview-dialog min-w-0 max-w-none"',
    );
    expect(source).toContain(
      'className="app-library-log-dialog__stage app-media-preview-dialog-stage"',
    );
    expect(workflows).toMatch(
      /\.app-library-log-dialog\.app-media-preview-dialog\s*\{[^}]*height:\s*auto;[^}]*max-height:\s*calc\(100vh - 2rem\);/s,
    );
    expect(workflows).not.toMatch(
      /\.app-library-log-dialog\.app-media-preview-dialog\s*\{[^}]*height:\s*min\(44rem,/s,
    );
  });
});
