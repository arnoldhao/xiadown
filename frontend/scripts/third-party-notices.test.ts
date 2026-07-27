import { describe, expect, test } from "bun:test";

import {
  requiredRuntimeDependencyNames,
  thirdPartyNoticesTextMatches,
} from "./third-party-notices.mjs";

describe("frontend runtime dependency inventory", () => {
  test("is host-independent by excluding optional platform packages", () => {
    expect(
      requiredRuntimeDependencyNames({
        dependencies: {
          react: "18.3.1",
          "pdfjs-dist": "6.1.200",
        },
        optionalDependencies: {
          "@napi-rs/canvas-darwin-arm64": "1.0.2",
          "@napi-rs/canvas-linux-x64-gnu": "1.0.2",
        },
      }),
    ).toEqual(["pdfjs-dist", "react"]);
  });
});

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
