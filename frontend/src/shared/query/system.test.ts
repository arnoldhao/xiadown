import { describe, expect, test } from "bun:test";

import { isExternalHTTPURL } from "./system";

describe("system external URL policy", () => {
  test("accepts only absolute HTTP and HTTPS URLs", () => {
    expect(isExternalHTTPURL("https://xiadown.app/docs?q=network#proxy")).toBe(true);
    expect(isExternalHTTPURL(" HTTP://example.com:8080/path ")).toBe(true);

    for (const value of [
      "",
      "/relative",
      "example.com/path",
      "mailto:user@example.com",
      "javascript:alert(1)",
      "data:text/html,hello",
      "file:///tmp/example",
      "https://",
    ]) {
      expect(isExternalHTTPURL(value)).toBe(false);
    }
  });
});
