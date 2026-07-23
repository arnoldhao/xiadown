import { describe, expect, test } from "bun:test";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

const frontendRoot = fileURLToPath(new URL("../../../", import.meta.url));

const SOURCE_GLOBS = [
  "src/app/main/completed/**/*.{ts,tsx}",
  "src/app/media/**/*.{ts,tsx}",
] as const;

const APPEARANCE_UTILITY =
  /(?:^|[\s"'`])(?:(?:dark|hover|focus|focus-visible|active|disabled|group-hover|group-focus|data-\[[^\]]+\]):)*(?:bg-|text-|font-|leading-|tracking-|rounded(?:-|(?=[\s"'`]))|border(?:-|(?=[\s"'`]))|shadow(?:-|(?=[\s"'`]))|ring(?:-|(?=[\s"'`]))|outline-|opacity-|cursor-|transition(?:-|(?=[\s"'`]))|duration-|ease-|animate-|filter(?:-|(?=[\s"'`]))|blur-|backdrop-)[^\s"'`]*/g;

describe("Completed and media style ownership", () => {
  test("keeps feature sources structural and routes appearance through Dream classes", async () => {
    const violations: string[] = [];
    const scannedFiles = new Set<string>();

    for (const pattern of SOURCE_GLOBS) {
      const glob = new Bun.Glob(pattern);
      for await (const file of glob.scan({ cwd: frontendRoot, onlyFiles: true })) {
        if (/\.(?:test|spec)\.[^/]+$/.test(file)) {
          continue;
        }

        scannedFiles.add(file);
        const source = await Bun.file(join(frontendRoot, file)).text();
        const utilityMatches = [...source.matchAll(APPEARANCE_UTILITY)].map(
          (match) => match[0].trim(),
        );

        for (const utility of utilityMatches) {
          violations.push(`${file}: ${utility}`);
        }

        if (
          /style=\{\{[^}]*\b(?:background(?:Color)?|color|border(?:Color|Radius)?|boxShadow|font(?:Family|Size|Weight)?|lineHeight|letterSpacing|opacity|textAlign|textShadow|filter|backdropFilter|animation|transition|cursor|outline)\s*:/s.test(
            source,
          )
        ) {
          violations.push(`${file}: static inline appearance style`);
        }
      }
    }

    expect(violations).toEqual([]);
    expect([...scannedFiles]).toEqual(expect.arrayContaining([
      "src/app/main/completed/CompletedPage.tsx",
      "src/app/media/MediaPreviewDialog.tsx",
      "src/app/media/index.ts",
    ]));
  });

  test("defines migrated recipes and reduced motion in Dream", async () => {
    const [entry, appearance] = await Promise.all([
      Bun.file(new URL("./dream.css", import.meta.url)).text(),
      Bun.file(new URL("./dream/completed.css", import.meta.url)).text(),
    ]);

    expect(entry).toContain('@import "./dream/completed.css";');
    expect(appearance).toContain(".app-completed-task-card");
    expect(appearance).toContain(".app-completed-detail-title-text");
    expect(appearance).toContain(".app-completed-loading-spinner");
    expect(appearance).toContain(".app-completed-preview-volume-popover");
    expect(appearance).toContain(".app-flv-preview-unavailable-overlay");
    expect(appearance).toContain(".app-media-preview-dialog-url");
    expect(appearance).toContain("@media (prefers-reduced-motion: reduce)");
    expect(appearance).toContain("var(--app-layer-fullscreen)");
    expect(appearance).not.toMatch(/z-index:\s*(?:200|300)\b/);
  });

  test("routes imperative clipboard fallback appearance through one Dream hook", async () => {
    const [anatomy, listen, completed, media] = await Promise.all([
      Bun.file(new URL("./dream/anatomy.css", import.meta.url)).text(),
      Bun.file(
        new URL("../../app/main/listen/playback-helpers.ts", import.meta.url),
      ).text(),
      Bun.file(
        new URL(
          "../../app/main/completed/detail-components.tsx",
          import.meta.url,
        ),
      ).text(),
      Bun.file(
        new URL("../../app/media/MediaPreviewDialog.tsx", import.meta.url),
      ).text(),
    ]);

    for (const source of [listen, completed, media]) {
      expect(source).toContain(
        'textarea.className = "app-clipboard-fallback-textarea";',
      );
      expect(source).not.toMatch(/textarea\.style\.(?:position|inset|top|left)/);
    }
    expect(anatomy).toMatch(
      /\.app-clipboard-fallback-textarea\s*\{[^}]*position:\s*fixed;[^}]*top:\s*0;[^}]*left:\s*-10000px;/s,
    );
  });
});
