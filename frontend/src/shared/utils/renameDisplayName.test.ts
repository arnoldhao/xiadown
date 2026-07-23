import { describe, expect, test } from "bun:test";

import {
  composeProtectedFileDisplayName,
  splitProtectedFileDisplayName,
  validateRenameDisplayName,
} from "./renameDisplayName";

const messages = {
  required: "required",
  invalid: "invalid",
  tooLong: "too long",
};

describe("rename display name", () => {
  test("shares the Completed and Library validation boundary", () => {
    expect(validateRenameDisplayName("  ", messages)).toBe("required");
    expect(validateRenameDisplayName("CON.mp3", messages)).toBe("invalid");
    expect(validateRenameDisplayName("bad/name", messages)).toBe("invalid");
    expect(validateRenameDisplayName("name.", messages)).toBe("invalid");
    expect(validateRenameDisplayName("a".repeat(161), messages)).toBe("too long");
    expect(validateRenameDisplayName("Field Recording", messages)).toBe("");
  });

  test("keeps the final file extension outside the editable stem", () => {
    expect(splitProtectedFileDisplayName("Field.Recording.flac")).toEqual({
      stem: "Field.Recording",
      extension: ".flac",
    });
    expect(splitProtectedFileDisplayName("README")).toEqual({
      stem: "README",
      extension: "",
    });
    expect(composeProtectedFileDisplayName("Morning", ".flac")).toBe(
      "Morning.flac",
    );
    expect(composeProtectedFileDisplayName("Morning.FLAC", ".flac")).toBe(
      "Morning.flac",
    );
  });
});
