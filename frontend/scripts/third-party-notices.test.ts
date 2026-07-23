import { describe, expect, test } from "bun:test";

import { thirdPartyNoticesTextMatches } from "./third-party-notices.mjs";

describe("third-party notices text comparison", () => {
  const generated = "XiaDown Third-Party Notices\n\nComponent: example@1.0.0\n";

  test.each([
    ["LF", generated],
    ["CRLF", generated.replace(/\n/g, "\r\n")],
    ["CR", generated.replace(/\n/g, "\r")],
    ["UTF-8 BOM", `\uFEFF${generated}`],
    ["UTF-8 BOM and CRLF", `\uFEFF${generated.replace(/\n/g, "\r\n")}`],
  ])("accepts equivalent %s text", (_label, actual) => {
    expect(thirdPartyNoticesTextMatches(actual, generated)).toBe(true);
  });

  test("still rejects real notice changes", () => {
    expect(
      thirdPartyNoticesTextMatches(
        generated.replace("example@1.0.0", "example@2.0.0"),
        generated,
      ),
    ).toBe(false);
  });
});
