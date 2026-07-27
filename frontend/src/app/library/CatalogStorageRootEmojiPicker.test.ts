import { describe, expect, test } from "bun:test";

describe("Catalog storage root Emoji picker", () => {
  test("shares Radix dismissal state with the containing Dialog", async () => {
    const [packageJSON, lockfile, surfaceSource, dialogSource] =
      await Promise.all([
      Bun.file(new URL("../../../package.json", import.meta.url)).json(),
      Bun.file(new URL("../../../bun.lock", import.meta.url)).text(),
      Bun.file(new URL("./CatalogStorageSurfaces.tsx", import.meta.url)).text(),
      Bun.file(new URL("./CatalogManagementDialog.tsx", import.meta.url)).text(),
    ]);

    expect(packageJSON.dependencies["@radix-ui/react-popover"]).toBe("1.1.15");
    expect(lockfile).not.toContain(
      '"@radix-ui/react-popover/@radix-ui/react-dismissable-layer"',
    );
    expect(surfaceSource).toContain(
      `closest<HTMLElement>('[role="dialog"]')`,
    );
    expect(surfaceSource).toContain(
      "portalContainer={emojiPortalContainer}",
    );
    expect(surfaceSource).not.toContain("onEscapeKeyDown");
    expect(dialogSource).toContain(
      'const [emojiPickerRootID, setEmojiPickerRootID] = React.useState("")',
    );
    expect(dialogSource).toContain("onKeyDownCapture={(event) =>");
    expect(dialogSource).toContain(
      'if (event.key !== "Escape" || !emojiPickerRootID) return',
    );
    expect(dialogSource).toContain("event.preventDefault()");
    expect(dialogSource).toContain("event.stopPropagation()");
    expect(dialogSource).toContain('setEmojiPickerRootID("")');
    expect(dialogSource).toContain(
      "emojiPickerOpen={emojiPickerRootID === root.id}",
    );
  });
});
