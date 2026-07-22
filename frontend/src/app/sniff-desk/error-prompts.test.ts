import { describe, expect, test } from "bun:test";

import { getXiaText } from "@/features/xiadown/shared";
import { resolveSniffDeskErrorDescription } from "./error-prompts";

describe("Sniff error prompts", () => {
  test("localizes raw resource sniff resolve errors", () => {
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

  test("localizes unavailable browser errors", () => {
    const text = getXiaText("zh-CN");

    expect(
      resolveSniffDeskErrorDescription(
        text,
        "[resource_browser_unavailable] resource sniff browser unavailable: no supported browser detected",
      ),
    ).toBe(text.sniffDesk.errors.browserUnavailable);
  });

  test("localizes current Chrome connection races", () => {
    const text = getXiaText("zh-CN");
    expect(
      resolveSniffDeskErrorDescription(
        text,
        new Error(
          "[resource_current_browser_remote_debugging_required] enable Chrome Remote Debugging before connecting",
        ),
      ),
    ).toBe(text.sniffDesk.errors.currentBrowserRemoteDebugging);
  });
});
