import { describe, expect, test } from "bun:test";

describe("settings data management sheet contract", () => {
  test("keeps core data read-only and separates reset from cleanup", async () => {
    const source = await Bun.file(
      new URL("./SettingsDataSheets.tsx", import.meta.url),
    ).text();

    expect(source).toContain(
      'const selectable = category.id !== "core" && item.clearable;',
    );
    expect(source).toContain("selectable ? (");
    expect(source).toContain("setConfirmingReset(true)");
    expect(source).toContain("confirmingReset ? (");
    expect(source).toContain("await reset.mutateAsync()");
    expect(source).toContain("if (!result.scheduled)");
    expect(source).toContain("windowChromeSafeArea");
    expect(source).toContain("const categories = snapshot.data?.categories ?? [];");
    expect(source).toContain("category.items.length === 0");
    expect(source).toContain('t("dataManagement.empty")');
    expect(source).toContain('case "active-logs"');
    expect(source).toContain('case "session-vault-key"');
    expect(source).toContain('"dataManagement.itemCountOne"');
    expect(source).toContain('"dataManagement.itemCount"');
  });

  test("retains failed cleanup selections and reports partial failures", async () => {
    const source = await Bun.file(
      new URL("./SettingsDataSheets.tsx", import.meta.url),
    ).text();

    expect(source).toContain("settleDataManagementCleanResults");
    expect(source).toContain("settlement.succeededIds.forEach");
    expect(source).toContain("settlement.failedIds.length > 0");
    expect(source).toContain('t("dataManagement.cleanPartialFailed")');
  });

  test("labels obsolete sessions and profiles as legacy data", async () => {
    const source = await Bun.file(
      new URL("./SettingsDataSheets.tsx", import.meta.url),
    ).text();

    expect(source).toContain('item.id === "legacy.app-sessions"');
    expect(source).toContain('item.id === "legacy.sniff-profiles"');
    expect(source).toContain("dataManagement.resource.legacyAppSessions.label");
    expect(source).toContain("dataManagement.resource.legacyBrowserProfiles.label");
  });

  test("does not describe protected personal content or staged updates as generic clearable files", async () => {
    const source = await Bun.file(
      new URL("./SettingsDataSheets.tsx", import.meta.url),
    ).text();

    expect(source).toContain('case "user-content":');
    expect(source).toContain("dataManagement.resource.userContent.label");
    expect(source).toContain('case "update-stage":');
    expect(source).toContain("dataManagement.resource.updateStage.label");
  });

  test("keeps one managed profile per browser without hiding old extra profiles", async () => {
    const source = await Bun.file(
      new URL("./SettingsDataSheets.tsx", import.meta.url),
    ).text();

    expect(source).not.toContain("createSniffProfile");
    expect(source).not.toContain("handleCreate");
    expect(source).not.toContain(
      ["dataManagement", "createProfile"].join("."),
    );
    expect(source).toContain("profiles.map((profile)");
    expect(source).toContain('profile.isDefault');
    expect(source).toContain('t("dataManagement.obsolete")');
    expect(source).toContain("openSniffProfile");
    expect(source).toContain("deleteSniffProfile");
  });
});
