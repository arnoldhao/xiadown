import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import {
  formatListenLocalProbeWarning,
  ListenLocalProbeWarning,
} from "./LocalProbeWarning";

describe("local track probe warning", () => {
  test("renders a non-blocking accessible warning with compact detail", () => {
    const markup = renderToStaticMarkup(
      <ListenLocalProbeWarning
        error={"ffprobe failed\nwhile reading stream metadata"}
        message="Song info could not be refreshed"
      />,
    );
    expect(markup).toContain('role="note"');
    expect(markup).toContain('data-listen-local-probe-warning="true"');
    expect(markup).toContain(
      "Song info could not be refreshed: ffprobe failed while reading stream metadata",
    );
    expect(markup).not.toContain("disabled");
  });

  test("bounds an untrusted probe error before exposing it as a title", () => {
    const result = formatListenLocalProbeWarning(
      "Index warning",
      `  ${"x".repeat(400)}  `,
    );
    expect(result.startsWith("Index warning: ")).toBeTrue();
    expect(result.endsWith("…")).toBeTrue();
    expect(result.length).toBeLessThan(190);
  });

  test("is wired into both local track row implementations", async () => {
    const [pageView, libraryWorkspace] = await Promise.all([
      Bun.file(new URL("./PageView.tsx", import.meta.url)).text(),
      Bun.file(new URL("./LocalLibraryWorkspace.tsx", import.meta.url)).text(),
    ]);
    for (const source of [pageView, libraryWorkspace]) {
      expect(source).toContain("<ListenLocalProbeWarning");
      expect(source).toContain("formatListenLocalProbeWarning(");
      expect(source).toContain("track.probeError");
    }
  });
});
