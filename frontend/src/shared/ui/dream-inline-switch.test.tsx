import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import {
  DreamInlineSwitch,
  DreamInlineSwitchVisual,
} from "./dream-inline-switch";

describe("DreamInlineSwitch", () => {
  test("exposes checked, disabled, and described state through one switch primitive", () => {
    const markup = renderToStaticMarkup(
      <DreamInlineSwitch
        aria-describedby="remote-status"
        aria-busy="true"
        ariaLabel="Remote access"
        checked
        disabled
        onCheckedChange={() => undefined}
      />,
    );

    expect(markup).toContain('role="switch"');
    expect(markup).toContain('aria-checked="true"');
    expect(markup).toContain('aria-describedby="remote-status"');
    expect(markup).toContain('aria-busy="true"');
    expect(markup).toContain('data-state="checked"');
    expect(markup).toContain("disabled");
    expect(markup).toContain("app-dream-inline-switch-knob");
  });

  test("offers the same non-interactive visual for composite controls", () => {
    const markup = renderToStaticMarkup(
      <DreamInlineSwitchVisual checked className="access-switch" />,
    );

    expect(markup).toContain('aria-hidden="true"');
    expect(markup).toContain('data-state="checked"');
    expect(markup).toContain("app-dream-inline-switch");
    expect(markup).toContain("app-dream-inline-switch-knob");
    expect(markup).not.toContain('role="switch"');
  });

  test("keeps product switch consumers on the shared primitive", async () => {
    const [settings, newTask, equalizer, workflows] = await Promise.all([
      Bun.file(
        new URL("../../app/settings/settings-helpers.tsx", import.meta.url),
      ).text(),
      Bun.file(
        new URL("../../app/main/NewTaskDialog.tsx", import.meta.url),
      ).text(),
      Bun.file(
        new URL("../../features/settings/equalizer/index.tsx", import.meta.url),
      ).text(),
      Bun.file(
        new URL("../styles/dream/workflows.css", import.meta.url),
      ).text(),
    ]);

    for (const source of [settings, newTask, equalizer]) {
      expect(source).toContain("<DreamInlineSwitch");
      expect(source).not.toContain('role="switch"');
      expect(source).not.toContain("app-dream-inline-switch-knob");
    }

    expect(workflows).toMatch(
      /@media \(prefers-reduced-motion:\s*reduce\)[\s\S]*?\.app-dream-inline-switch,[\s\S]*?\.app-dream-inline-switch-knob\s*\{[^}]*transition:\s*none/s,
    );
    expect(workflows).toMatch(
      /@media \(forced-colors:\s*active\)[\s\S]*?\.app-dream-inline-switch\s*\{[^}]*border:\s*1px solid CanvasText[^}]*background:\s*Canvas/s,
    );

    const mainAppSource = await Bun.file(
      new URL("../../app/main/MainApp.tsx", import.meta.url),
    ).text();
    expect(mainAppSource).toContain("<DreamInlineSwitchVisual");
    expect(mainAppSource).not.toContain("app-dream-inline-switch-knob");
  });
});
