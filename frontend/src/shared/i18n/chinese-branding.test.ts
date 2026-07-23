import { describe, expect, test } from "bun:test";

import zhCN from "./locales/zh-CN.json";
import zhTW from "./locales/zh-TW.json";

function translationValues(value: unknown): string[] {
  if (typeof value === "string") {
    return [value];
  }
  if (Array.isArray(value)) {
    return value.flatMap(translationValues);
  }
  if (value && typeof value === "object") {
    return Object.values(value).flatMap(translationValues);
  }
  return [];
}

describe("Chinese locale branding", () => {
  test.each([
    ["zh-CN", zhCN],
    ["zh-TW", zhTW],
  ] as const)("uses the localized brand in %s translation values", (_locale, messages) => {
    const values = translationValues(messages);

    expect(values.some((value) => value.includes("下蛋"))).toBe(true);
    expect(values.filter((value) => /xiadown/i.test(value))).toEqual([]);
  });
});

describe("Chinese Library terminology", () => {
  test("uses 资源库 consistently for the simplified Chinese Library product surface", async () => {
    const recoveryPage = await Bun.file(
      new URL("../../../index.html", import.meta.url),
    ).text();

    expect(JSON.stringify(zhCN)).not.toContain("资料库");
    expect(recoveryPage).not.toContain("资料库");
    expect(zhCN.xiadown.workspace.libraryStation).toBe("资源库");
    expect(zhCN.xiadown.libraryCatalog.library).toBe("资源库");
    expect(zhCN.xiadown.settings.libraryAccess.title).toBe("资源库访问");
  });

  test("keeps resource-library wording distinct from database wording in traditional Chinese", () => {
    expect(zhTW.xiadown.workspace.libraryStation).toBe("資源庫");
    expect(zhTW.xiadown.libraryCatalog.library).toBe("資源庫");
    expect(zhTW.xiadown.settings.libraryAccess.title).toBe("資源庫存取");
    expect(zhTW.xiadown.libraryData.databaseIntegrityFailedTitle).toContain("資料庫");
  });
});
