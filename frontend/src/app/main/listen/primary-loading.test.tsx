import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import {
  ListenPrimaryLoadingBoundary,
  ListenPrimaryLoadingOverlay,
  ListenPrimaryStatusOverlay,
} from "./PrimaryLoadingOverlay";

const loadingLabel = "Loading music";
const underlyingActionLabel = "action.underlying";
const backLabel = "action.back";

describe("Listen primary loading treatment", () => {
  test("announces one centered content scrim without replacing navigation", () => {
    const markup = renderToStaticMarkup(
      <ListenPrimaryLoadingOverlay
        label={loadingLabel}
        pet={null}
        petImageURL=""
      />,
    );

    expect(markup).toContain('data-listen-primary-status="loading"');
    expect(markup).toContain('data-surface-role="status"');
    expect(markup).toContain('data-material="regular"');
    expect(markup).toContain('role="status"');
    expect(markup).toContain('aria-live="polite"');
    expect(markup).toContain(loadingLabel);
    expect(markup).toContain("listen-primary-status-pet");
    expect(markup).not.toContain("lucide-loader-circle");
  });

  test("uses the same pet-first centered treatment for actionable errors", () => {
    const errorLabel = "Music service unavailable";
    const actionLabel = "Try again";
    const markup = renderToStaticMarkup(
      <ListenPrimaryStatusOverlay
        kind="error"
        label={errorLabel}
        pet={null}
        petImageURL=""
        animation="failed"
        actionLabel={actionLabel}
        onAction={() => undefined}
      />,
    );

    expect(markup).toContain('data-listen-primary-status="error"');
    expect(markup).toContain('role="alert"');
    expect(markup).toContain('aria-live="assertive"');
    expect(markup.indexOf("listen-primary-status-pet")).toBeLessThan(
      markup.indexOf("listen-primary-status-message"),
    );
    expect(markup.indexOf("listen-primary-status-message")).toBeLessThan(
      markup.indexOf("listen-primary-status-action"),
    );
  });

  test("does not invent an action for a non-retryable error", () => {
    const markup = renderToStaticMarkup(
      <ListenPrimaryStatusOverlay
        kind="error"
        label="Unavailable in this region"
        pet={null}
        petImageURL=""
        animation="failed"
      />,
    );

    expect(markup).toContain("Unavailable in this region");
    expect(markup).not.toContain("listen-primary-status-action");
  });

  test("makes the covered page inert and hidden from assistive technology only while busy", () => {
    const busyMarkup = renderToStaticMarkup(
      <ListenPrimaryLoadingBoundary loading>
        <button type="button">{underlyingActionLabel}</button>
      </ListenPrimaryLoadingBoundary>,
    );
    const idleMarkup = renderToStaticMarkup(
      <ListenPrimaryLoadingBoundary loading={false}>
        <button type="button">{underlyingActionLabel}</button>
      </ListenPrimaryLoadingBoundary>,
    );

    expect(busyMarkup).toContain('inert=""');
    expect(busyMarkup).toContain('aria-busy="true"');
    expect(busyMarkup).toContain('aria-hidden="true"');
    expect(busyMarkup).toContain('data-listen-primary-loading-boundary="busy"');
    expect(idleMarkup).not.toContain("inert=");
    expect(idleMarkup).not.toContain("aria-busy=");
    expect(idleMarkup).not.toContain("aria-hidden=");
    expect(idleMarkup).toContain('data-listen-primary-loading-boundary="idle"');
  });

  test("covers error content without announcing it as busy", () => {
    const markup = renderToStaticMarkup(
      <ListenPrimaryLoadingBoundary loading={false} covered>
        <button type="button">{underlyingActionLabel}</button>
      </ListenPrimaryLoadingBoundary>,
    );

    expect(markup).toContain('data-listen-primary-loading-boundary="covered"');
    expect(markup).toContain('inert=""');
    expect(markup).toContain('aria-hidden="true"');
    expect(markup).not.toContain("aria-busy=");
  });

  test("keeps an optional detail back action outside the inert page", () => {
    const markup = renderToStaticMarkup(
      <ListenPrimaryLoadingOverlay
        label={loadingLabel}
        pet={null}
        petImageURL=""
        backLabel={backLabel}
        onBack={() => undefined}
      />,
    );

    expect(markup).toContain('data-listen-primary-loading-back="true"');
    expect(markup).toContain(`aria-label="${backLabel}"`);
    expect(markup).toContain("listen-primary-status-back wails-no-drag");
    expect(markup).not.toContain("inert=");
  });

  test("covers the full primary viewport with a centered pet status", async () => {
    const [layoutCss, appearanceCss, pageSource] = await Promise.all([
      Bun.file(new URL("./listen.css", import.meta.url)).text(),
      Bun.file(
        new URL("../../../shared/styles/dream/listen.css", import.meta.url),
      ).text(),
      Bun.file(new URL("./PageView.tsx", import.meta.url)).text(),
    ]);
    const scrimRule = layoutCss.match(
      /\.listen-primary-status-scrim\s*\{([\s\S]*?)\n\}/,
    )?.[1];
    const scrimAppearanceRule = appearanceCss.match(
      /\.listen-primary-status-scrim\s*\{([\s\S]*?)\n\}/,
    )?.[1];
    const surfaceRule = layoutCss.match(
      /\.listen-primary-status-scrim__surface\s*\{([\s\S]*?)\n\}/,
    )?.[1];
    const surfaceAppearanceRule = appearanceCss.match(
      /\.app-glass-surface\.listen-primary-status-scrim__surface\s*\{([\s\S]*?)\n\}/,
    )?.[1];

    expect(scrimRule).toContain("inset: 0");
    expect(scrimRule).toContain("place-items: center");
    expect(scrimRule).toContain("z-index: var(--app-layer-floating-controls)");
    expect(scrimAppearanceRule).toContain("background: transparent");
    expect(scrimRule).not.toContain("backdrop-filter:");
    expect(surfaceRule).toContain("position: absolute");
    expect(surfaceRule).toContain("inset: 0");
    expect(surfaceAppearanceRule).toContain(
      "--app-glass-surface-radius: var(--app-radius-none)",
    );
    expect(scrimRule).not.toContain("border-radius");
    expect(layoutCss).not.toMatch(/(?:-webkit-)?backdrop-filter\s*:/);
    expect(layoutCss).not.toMatch(/z-index:\s*(?:[2-9]\d|[1-9]\d{2,})\s*;/);
    expect(layoutCss).toContain(".listen-primary-status-content");
    expect(appearanceCss).toContain(".listen-primary-status-pet");
    expect(layoutCss).toContain(".listen-primary-status-back");
    expect(appearanceCss).toMatch(/\.listen-primary-status-pet\s*\{[^}]*filter:\s*none/s);
    expect(pageSource).toContain(
      "listen-primary-viewport listen-primary-viewport-enter relative",
    );
    expect(pageSource).toContain("<ListenPrimaryLoadingBoundary");
    expect(pageSource).toContain("loading={musePrimaryLoading}");
    expect(pageSource).toContain("covered={pagePromptIsError}");
    expect(pageSource).toContain(
      "const primaryStatusOverlay = musePrimaryLoading",
    );
    expect(pageSource).toContain(
      '<ListenPrimaryStatusOverlay\n      kind="error"',
    );
    expect(pageSource).toContain("overlay={primaryStatusOverlay}");
    expect(pageSource).toContain("onBack={musePrimaryLoadingBackAction}");
    expect(pageSource).toContain("pet={props.pet}");
    expect(pageSource).toContain("petImageURL={props.petImageURL}");
    expect(pageSource).toContain(
      '!normalizedQuery && libraryViewPhase === "loading"',
    );
    expect(pageSource).toContain(
      "return { label: props.text.listen.onlineLoading }",
    );
  });
});
