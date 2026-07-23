import { describe, expect, test } from "bun:test";

describe("settings Network tab composition", () => {
  test("keeps proxy and Library access together outside General", async () => {
    const source = await Bun.file(new URL("./SettingsApp.tsx", import.meta.url)).text();
    const generalStart = source.indexOf('{activeTab === "general"');
    const networkStart = source.indexOf('{activeTab === "network"');
    const appearanceStart = source.indexOf('{activeTab === "appearance"');

    expect(generalStart).toBeGreaterThan(-1);
    expect(networkStart).toBeGreaterThan(generalStart);
    expect(appearanceStart).toBeGreaterThan(networkStart);

    const general = source.slice(generalStart, networkStart);
    const network = source.slice(networkStart, appearanceStart);
    expect(general).not.toContain("{proxySettingsCard}");
    expect(general).not.toContain("<LibraryAccessSettingsCard");
    expect(network).toContain("{proxySettingsCard}");
    expect(network).toContain("<LibraryAccessSettingsCard");
  });

  test("renders seven tabs in one non-wrapping row", async () => {
    const source = await Bun.file(new URL("./SettingsApp.tsx", import.meta.url)).text();
    const tabsStart = source.indexOf("const tabs:");
    const tabsEnd = source.indexOf("const visibleTabs", tabsStart);
    const tabs = source.slice(tabsStart, tabsEnd);

    expect(tabs.match(/\{ id: "/g)).toHaveLength(7);
    expect(tabs).toContain('{ id: "network"');
    expect(tabs).toContain('{ id: "ai"');
    expect(source).toContain("app-dream-tabs-bar -mt-1 flex flex-nowrap");
  });

  test("keeps the AI tab intentionally limited to one construction card", async () => {
    const source = await Bun.file(new URL("./SettingsApp.tsx", import.meta.url)).text();
    const aiStart = source.indexOf('{activeTab === "ai"');
    const aboutStart = source.indexOf('{activeTab === "about"');

    expect(aiStart).toBeGreaterThan(-1);
    expect(aboutStart).toBeGreaterThan(aiStart);
    const ai = source.slice(aiStart, aboutStart);
    expect(ai.match(/<SettingsCompactListCard/g)).toHaveLength(1);
    expect(ai).toContain("text.settings.aiUnderConstruction");
  });
});
