import { describe, expect, test } from "bun:test";

import { getXiaText } from "@/features/xiadown/shared";
import { resolveSniffDeskErrorDescription } from "./error-prompts";

describe("sniff desk error prompts", () => {
  test("localizes structured resource sniff resolve errors", () => {
    const text = getXiaText("zh-CN");
    const error = new Error(
      JSON.stringify({
        code: "resource_resolve_failed",
        message: "resource sniff raw resource not found",
      }),
    );

    expect(resolveSniffDeskErrorDescription(text, error)).toBe(
      text.sniffDesk.errors.resourceNotFound,
    );
  });

  test("localizes bracketed unsupported-domain errors with the domain", () => {
    const text = getXiaText("zh-CN");

    expect(
      resolveSniffDeskErrorDescription(
        text,
        "[resource_unsupported_domain] resource sniff does not support youtube.com",
      ),
    ).toBe(text.sniffDesk.urlUnsupportedDomain.replace("{domain}", "youtube.com"));
  });

  test("localizes unavailable browser errors", () => {
    const text = getXiaText("zh-CN");

    expect(
      resolveSniffDeskErrorDescription(
        text,
        "[resource_browser_unavailable] resource sniff browser unavailable: no supported browser detected",
      ),
    ).toBe(text.sniffDesk.errors.browserUnavailable);
  });
});
