import { describe, expect, test } from "bun:test";

import en from "./locales/en.json";
import es419 from "./locales/es-419.json";
import idID from "./locales/id-ID.json";
import jaJP from "./locales/ja-JP.json";
import koKR from "./locales/ko-KR.json";
import ptBR from "./locales/pt-BR.json";
import viVN from "./locales/vi-VN.json";
import zhCN from "./locales/zh-CN.json";
import zhTW from "./locales/zh-TW.json";

const locales = {
  en,
  "es-419": es419,
  "id-ID": idID,
  "ja-JP": jaJP,
  "ko-KR": koKR,
  "pt-BR": ptBR,
  "vi-VN": viVN,
  "zh-CN": zhCN,
  "zh-TW": zhTW,
} as const;

describe("Music station branding", () => {
  test.each(Object.entries(locales))(
    "%s uses the current workspace names on every Music surface",
    (_locale, messages) => {
      const { listen, workspace } = messages.xiadown;

      expect(listen.hush).toBe(workspace.lofi);
      expect(listen.muse).toBe(workspace.youtubeMusic);
      expect(listen.linger).toBe(workspace.local);
      expect(listen.museGateTitle).toContain(workspace.youtubeMusic);

      const visibleMusicText = [
        listen.hush,
        listen.muse,
        listen.linger,
        listen.museGateTitle,
      ].join("\n");
      expect(visibleMusicText).not.toMatch(/\b(?:Hush|Muse|Linger)\b/i);
      expect(visibleMusicText).not.toMatch(/放空|思[绪緒]|[萦縈]绕|縈繞/);
    },
  );

  test("keeps Hush as a distinct DreamApp recommendation", () => {
    expect(zhCN.xiadown.about.hush).toBe("Hush / 放空");
    expect(zhCN.xiadown.about.hushDescription).toBe(
      "一款可自定义频道的 Lo-Fi 直播桌面播放器",
    );
    for (const messages of Object.values(locales)) {
      expect(messages.xiadown.about.hush).toContain("Hush");
    }
  });
});
