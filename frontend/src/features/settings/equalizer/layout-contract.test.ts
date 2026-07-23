import { describe, expect, test } from "bun:test";

const read = (relativePath: string) =>
  Bun.file(new URL(relativePath, import.meta.url)).text();

describe("equalizer Settings card spacing", () => {
  test("uses semantic Dream insets for the preset and preamp cards", async () => {
    const [source, cards, settings] = await Promise.all([
      read("./index.tsx"),
      read("./EqualizerControlCards.tsx"),
      read("../../../shared/styles/dream/settings.css"),
    ]);

    expect(source).toContain("<EqualizerControlCards");
    expect(cards).toContain(
      'contentClassName="app-equalizer-preset-card-content"',
    );
    expect(cards).toContain(
      'contentClassName="app-equalizer-preamp-card-content"',
    );
    expect(cards).not.toContain('contentClassName="p-4"');

    expect(settings).toMatch(
      /\.app-settings-list-card-content\.app-settings-list-card-content-compact\.app-equalizer-preset-card-content\s*\{[^}]*padding-block:\s*var\(--app-space-4\)/s,
    );
    expect(settings).toMatch(
      /\.app-settings-list-card-content\.app-settings-list-card-content-compact\.app-equalizer-preamp-card-content\s*\{[^}]*padding:\s*var\(--app-space-4\)/s,
    );
  });
});
