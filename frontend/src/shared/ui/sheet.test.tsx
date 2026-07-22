import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import {
  SHEET_SIDES,
  SHEET_SIZES,
  SheetBody,
  SheetFooter,
  SheetHeader,
  SheetHeading,
} from "./sheet";

describe("shared sheet", () => {
  test("publishes every placement and semantic width", () => {
    expect(SHEET_SIDES).toEqual(["left", "right"]);
    expect(SHEET_SIZES).toEqual(["sm", "md", "lg"]);
  });

  test("owns the modal panel contract instead of delegating it to features", async () => {
    const [source, anatomy] = await Promise.all([
      Bun.file(new URL("./sheet.tsx", import.meta.url)).text(),
      Bun.file(
        new URL("../styles/dream/anatomy.css", import.meta.url),
      ).text(),
    ]);

    expect(source).toContain('surfaceRole="overlay"');
    expect(source).not.toContain('data-material="panel"');
    expect(source).toContain('data-elevation="modal"');
    expect(source).toContain('data-shape="panel"');
    expect(source).toContain('data-tint="neutral"');
    expect(source).toContain("app-sheet-content");
    expect(anatomy).toMatch(
      /\.app-sheet-content\s*\{[^}]*z-index:\s*var\(--app-layer-modal\)/s,
    );
    expect(source).toContain('cn("app-sheet-overlay", overlayClassName)');
    expect(source).not.toMatch(/\b[-a-z0-9:]+-\[[^\]\n]+\]/i);
  });

  test("provides one header, scroll body, and footer anatomy", () => {
    const markup = renderToStaticMarkup(
      <>
        <SheetHeader>
          <SheetHeading>Heading</SheetHeading>
        </SheetHeader>
        <SheetBody>Body</SheetBody>
        <SheetFooter>Footer</SheetFooter>
      </>,
    );

    expect(markup).toContain("app-sheet-header");
    expect(markup).toContain("app-sheet-heading");
    expect(markup).toContain("app-sheet-body");
    expect(markup).toContain("app-sheet-footer");
    expect(markup).toContain("app-dialog-footer");
  });

  test("keeps header, body, and footer on one undivided glass surface", async () => {
    const [entry, anatomy, contract] = await Promise.all([
      Bun.file(new URL("../styles/dream.css", import.meta.url)).text(),
      Bun.file(new URL("../styles/dream/anatomy.css", import.meta.url)).text(),
      Bun.file(
        new URL("../styles/dream/dialog-contract.css", import.meta.url),
      ).text(),
    ]);

    expect(entry).toContain('@import "./dream/dialog-contract.css";');
    expect(entry.indexOf('dialog-contract.css')).toBeGreaterThan(
      entry.indexOf('library.css'),
    );
    expect(anatomy).toMatch(
      /\.app-sheet-header\s*\{[^}]*border-bottom:\s*0;/s,
    );
    expect(anatomy).toMatch(
      /\.app-sheet-footer\s*\{[^}]*border-top:\s*0;/s,
    );
    expect(contract).toMatch(
      /\.app-dialog-content\.app-glass-surface[\s\S]*?> :is\(\.app-dialog-header, \.app-dialog-footer\)[\s\S]*?border-block:\s*0;/,
    );
    expect(contract).toContain(
      ".app-sheet-content.app-glass-surface",
    );
    expect(contract).toMatch(
      /\.app-dialog-content\.app-glass-surface \.app-dialog-list-card,[\s\S]*?\.app-sheet-content\.app-glass-surface \.app-dialog-list-card\s*\{[^}]*border:\s*0;[^}]*background:\s*transparent;[^}]*box-shadow:\s*none;[^}]*backdrop-filter:\s*none;/,
    );
    expect(contract).toMatch(
      /@media \(forced-colors: active\)[\s\S]*?\.app-dialog-list-card[\s\S]*?border:\s*1px solid CanvasText;/,
    );
  });

  test("lets centered bodies size to content before they become scrollable", async () => {
    const anatomy = await Bun.file(
      new URL("../styles/dream/anatomy.css", import.meta.url),
    ).text();

    expect(anatomy).toMatch(
      /\.app-sheet-body\s*\{[^}]*min-height:\s*0;[^}]*flex:\s*1 1 auto;[^}]*overflow-y:\s*auto;/s,
    );
    expect(anatomy).not.toMatch(
      /\.app-sheet-body\s*\{[^}]*flex:\s*1 1 0;/s,
    );
  });

  test("offers an explicit native-window chrome safe area", async () => {
    const [source, anatomy] = await Promise.all([
      Bun.file(new URL("./sheet.tsx", import.meta.url)).text(),
      Bun.file(
        new URL("../styles/dream/anatomy.css", import.meta.url),
      ).text(),
    ]);

    expect(source).toContain("windowChromeSafeArea?: boolean");
    expect(source).toContain(
      'data-window-chrome-safe-area={windowChromeSafeArea ? "true" : undefined}',
    );
    expect(anatomy).toContain(
      '.app-sheet-content[data-centered="true"][data-window-chrome-safe-area="true"]',
    );
    expect(anatomy).toContain(
      "top: calc((100vh + var(--app-titlebar-height)) / 2)",
    );
    expect(anatomy).toContain(
      "max-height: calc(100vh - var(--app-titlebar-height) - 1.5rem)",
    );
    expect(anatomy).toContain(
      "top: calc(var(--app-titlebar-height) + 0.75rem)",
    );
  });
});
