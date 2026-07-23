import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import { DialogMarkdown } from "./dialog-markdown";

describe("DialogMarkdown external links", () => {
  test("renders only HTTP and HTTPS destinations as actionable links", () => {
    const markup = renderToStaticMarkup(
      <DialogMarkdown
        content={[
          "[secure](https://xiadown.app/docs)",
          "[plain](http://example.com/help)",
          "[email](mailto:user@example.com)",
          "[script](javascript:alert(1))",
          "[relative](/local/path)",
        ].join("\n\n")}
      />,
    );

    expect(markup).toContain('href="https://xiadown.app/docs"');
    expect(markup).toContain('href="http://example.com/help"');
    expect(markup).not.toContain("mailto:");
    expect(markup).not.toContain("javascript:");
    expect(markup).not.toContain('href="/local/path"');
    expect(markup.match(/aria-disabled="true"/g)).toHaveLength(3);
  });

  test("uses semantic Dream code roles for inline and fenced code", async () => {
    const [components, source] = await Promise.all([
      Bun.file(
        new URL("../styles/dream/components.css", import.meta.url),
      ).text(),
      Bun.file(new URL("./dialog-markdown.tsx", import.meta.url)).text(),
    ]);
    const markup = renderToStaticMarkup(
      <DialogMarkdown content={'Use `inline` here.\n\n```sh\necho hello\n```'} />,
    );

    expect(markup).toContain('class="app-dialog-markdown-code"');
    expect(markup).toContain('data-block="false"');
    expect(markup).toContain('class="app-dialog-markdown-pre overflow-x-auto"');
    expect(markup).toContain('data-block="true"');
    expect(source).not.toMatch(/(?:rounded|bg-muted|font-mono|text-\[0\.85em\])/);
    expect(source).not.toMatch(
      /(?:bg-|text-(?:foreground|background|muted|primary|secondary|destructive)|border-(?!collapse)|ring-|shadow-|rounded-|backdrop-blur|blur-|font-(?:bold|semibold|medium|mono)|tracking-|uppercase)/,
    );
    expect(source).not.toMatch(/\b(?:text-left|list-disc|list-decimal|border-collapse)\b/);
    expect(source).toContain("app-dialog-markdown");
    expect(source).toContain("app-dialog-markdown-table");
    expect(components).toContain('.app-dialog-markdown-list[data-kind="ordered"]');
    expect(components).toContain('.app-dialog-markdown-code[data-block="false"]');
    expect(components).toContain(".app-dialog-markdown-pre {");
  });
});
