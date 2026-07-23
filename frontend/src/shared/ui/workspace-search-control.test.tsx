import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import {
  shouldSuppressWorkspaceSearchSubmit,
  WorkspaceSearchControl,
} from "./workspace-search-control";

describe("WorkspaceSearchControl", () => {
  test("does not submit while Enter is confirming an IME composition", () => {
    expect(
      shouldSuppressWorkspaceSearchSubmit({
        key: "Enter",
        isComposing: true,
      }),
    ).toBe(true);
    expect(
      shouldSuppressWorkspaceSearchSubmit({ key: "Enter", keyCode: 229 }),
    ).toBe(true);
    expect(
      shouldSuppressWorkspaceSearchSubmit({
        key: "Enter",
        isComposing: false,
        keyCode: 13,
      }),
    ).toBe(false);
  });

  test("keeps every station search landing on the shared taller target", async () => {
    const [tokens, controls] = await Promise.all([
      Bun.file(
        new URL("../styles/dream/tokens.css", import.meta.url),
      ).text(),
      Bun.file(
        new URL("../styles/dream/controls.css", import.meta.url),
      ).text(),
    ]);

    expect(tokens).toMatch(/--app-workspace-search-height:\s*3rem;/);
    expect(controls).toMatch(
      /\.app-dream-workspace-search\s*\{[^}]*min-height:\s*var\(--app-workspace-search-height\)/s,
    );
  });

  test("owns one canonical Dream Search-page structure", () => {
    const markup = renderToStaticMarkup(
      <WorkspaceSearchControl
        clearLabel="Clear"
        onSubmit={() => undefined}
        onValueChange={() => undefined}
        placeholder="Search Library"
        submitLabel="Search"
        value="dream"
      />,
    );

    expect(markup).toContain(
      "app-dream-workspace-search app-dream-search-control app-dream-control-shell app-station-search-content-search",
    );
    expect(markup).toContain('type="search"');
    expect(markup).toContain('placeholder="Search Library"');
    expect(markup).toContain('aria-label="Clear"');

    const icon = markup.indexOf("app-dream-workspace-search__icon");
    const input = markup.indexOf("app-dream-workspace-search__input");
    const clear = markup.indexOf("app-dream-workspace-search__clear");
    const submit = markup.indexOf("app-dream-workspace-search__submit");
    expect(icon).toBeGreaterThan(-1);
    expect(input).toBeGreaterThan(icon);
    expect(clear).toBeGreaterThan(input);
    expect(submit).toBeGreaterThan(clear);
  });

  test("keeps the submit action present but disabled on an empty landing", () => {
    const markup = renderToStaticMarkup(
      <WorkspaceSearchControl
        clearLabel="Clear"
        onValueChange={() => undefined}
        placeholder="Search Music"
        submitLabel="Search"
        value=""
      />,
    );

    expect(markup).not.toContain("app-dream-workspace-search__clear");
    expect(markup).toContain("app-dream-workspace-search__submit");
    expect(markup).toContain("disabled");
  });
});
