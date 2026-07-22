import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import {
  APP_USER_AVATAR_SHAPES,
  APP_USER_AVATAR_TONES,
  UserAvatar,
} from "./user-avatar";

describe("UserAvatar Dream appearance", () => {
  test("publishes semantic tone and anatomy without hidden visual utilities", async () => {
    const [source, dream] = await Promise.all([
      Bun.file(new URL("./user-avatar.tsx", import.meta.url)).text(),
      Bun.file(
        new URL("../styles/dream/components.css", import.meta.url),
      ).text(),
    ]);
    const markup = renderToStaticMarkup(
      <UserAvatar
        profile={{ displayName: "Xia Down" }}
        shape="circle"
        tone="theme"
      />,
    );

    expect(markup).toContain('class="app-user-avatar"');
    expect(markup).toContain('data-tone="theme"');
    expect(markup).toContain('data-shape="circle"');
    expect(markup).toContain("app-user-avatar__fallback");
    expect(source).not.toMatch(/(?:bg|shadow|ring)-\[/);
    expect(dream).toContain('.app-user-avatar[data-tone="theme"]');
    expect(dream).toContain('.app-user-avatar[data-shape="circle"]');
    expect(dream).toContain(".app-user-avatar__rim");
    expect(APP_USER_AVATAR_TONES).toEqual(["neutral", "theme"]);
    expect(APP_USER_AVATAR_SHAPES).toEqual(["rounded", "circle"]);
  });
});
