import { describe, expect, test } from "bun:test";
import {
  FileArchive,
  FileBraces,
  FileCode,
  FileText,
  FileType,
  FileVideo,
  ImageIcon,
  Languages,
  Link2,
  Music2,
} from "lucide-react";

import { resolveResourceKindIcon } from "./resource-kind-icon";

describe("resource kind icon", () => {
  test.each([
    ["video", FileVideo],
    ["audio", Music2],
    ["subtitle", Languages],
    ["image", ImageIcon],
    ["manifest", FileCode],
    ["api", FileBraces],
    ["document", FileText],
    ["font", FileType],
    ["archive", FileArchive],
    ["other", Link2],
  ] as const)("maps %s to its shared Library icon", (kind, icon) => {
    expect(resolveResourceKindIcon(kind)).toBe(icon);
  });

  test("normalizes known values and falls back to the other icon", () => {
    expect(resolveResourceKindIcon("  VIDEO ")).toBe(FileVideo);
    expect(resolveResourceKindIcon("unknown")).toBe(Link2);
    expect(resolveResourceKindIcon(null)).toBe(Link2);
  });
});
