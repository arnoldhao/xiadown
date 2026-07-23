import { describe, expect, test } from "bun:test";

describe("Main dialog Dream appearance contract", () => {
  test("uses a semantic Whats New eyebrow without an inline pill recipe", async () => {
    const [source, components] = await Promise.all([
      Bun.file(new URL("./dialogs.tsx", import.meta.url)).text(),
      Bun.file(
        new URL("../../shared/styles/dream/components.css", import.meta.url),
      ).text(),
    ]);

    expect(source).toContain("app-whats-new-eyebrow");
    expect(source).toContain("app-whats-new-title");
    expect(source).toContain("app-whats-new-changelog");
    expect(source).toContain("app-whats-new-empty");
    expect(source).not.toMatch(
      /(?:bg-|text-(?:foreground|background|muted|primary|secondary|destructive)|border-|ring-|shadow-|rounded-|backdrop-blur|blur-|font-(?:bold|semibold|medium|mono)|tracking-|uppercase)/,
    );
    expect(source).not.toContain("text-left");
    expect(source).toContain("app-whats-new-header");
    expect(components).toContain(".app-whats-new-eyebrow {");
    expect(components).toContain("border-radius: var(--app-radius-capsule)");
    expect(components).toContain("color: var(--app-text-secondary)");
    expect(components).toContain(".app-whats-new-changelog {");
  });

  test("keeps sniff source controls semantic and Dream-owned", async () => {
    const [source, workflows, buttons] = await Promise.all([
      Bun.file(new URL("./NewSniffSourceSteps.tsx", import.meta.url)).text(),
      Bun.file(
        new URL("../../shared/styles/dream/workflows.css", import.meta.url),
      ).text(),
      Bun.file(
        new URL("../../shared/styles/dream/button-contract.css", import.meta.url),
      ).text(),
    ]);

    expect(source).toContain("<Button");
    expect(source).toContain("<StatusBadge");
    expect(source).not.toContain("<button");
    expect(source).not.toMatch(
      /(?:bg-|border-(?:primary|destructive|emerald|amber)|ring-|rounded-\[|text-(?:emerald|amber)|shadow-\[)/,
    );
    expect(source).not.toMatch(/\b(?:animate-spin|text-center)\b/);
    expect(source).toContain("app-motion-spin");
    expect(workflows).toContain(".app-new-task-sniff-source-empty");
    expect(workflows).toContain(
      '.app-new-task-sniff-profile-choice--composite[data-selected="true"]',
    );
    expect(buttons).toContain(
      ".app-new-task-sniff-browser-option[data-app-button]",
    );
    expect(buttons).toContain(
      ".app-new-task-sniff-profile-choice[data-app-button]",
    );
  });
});
