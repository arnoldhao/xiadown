import { describe, expect, test } from "bun:test";
import { createElement, Fragment } from "react";
import { renderToStaticMarkup } from "react-dom/server";

import { APP_BADGE_VARIANTS, Badge } from "./badge";
import {
  APP_CARD_SECTION_SIZES,
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "./card";
import { APP_DROPDOWN_MENU_ITEM_TONES } from "./dropdown-menu";
import { APP_INPUT_SIZES, Input } from "./input";
import { PET_DISPLAY_GLOW_VARIANTS } from "./pet-player";
import { TOOLTIP_ALIGNS, TOOLTIP_SIDES } from "./tooltip";

const PRIMITIVE_WRAPPER_FILES = [
  "button.tsx",
  "card.tsx",
  "checkbox.tsx",
  "dialog.tsx",
  "dream-inline-switch.tsx",
  "dropdown-menu.tsx",
  "input.tsx",
  "select.tsx",
  "sheet.tsx",
  "tooltip.tsx",
] as const;

const tailwindAppearanceUtility = new RegExp(
  String.raw`\b(?:text-(?:xs|sm|base|lg|xl|[2-9]xl|primary|secondary|accent|destructive|muted|foreground|background|card|popover)|font-(?:thin|extralight|light|normal|medium|semibold|bold|extrabold|black)|leading-(?:none|tight|snug|normal|relaxed|loose)|tracking-(?:tighter|tight|normal|wide|wider|widest)|rounded(?:-(?:none|sm|md|lg|xl|2xl|3xl|full))?|shadow(?:-(?:sm|md|lg|xl|2xl|inner|none))?|(?:bg|border|ring|outline)-(?:primary|secondary|accent|destructive|muted|background|foreground|card|popover|input|border))\b`,
  "g",
);

const tailwindArbitraryUtility = /\b[-a-z0-9:]+-\[[^\]\n]+\]/gi;

describe("shared primitive appearance vocabulary", () => {
  test("publishes primitive axes for exhaustive fixtures", () => {
    expect(APP_BADGE_VARIANTS).toEqual([
      "default",
      "secondary",
      "destructive",
      "outline",
      "subtle",
      "ghost",
    ]);
    expect(APP_CARD_SECTION_SIZES).toEqual(["default", "compact"]);
    expect(APP_DROPDOWN_MENU_ITEM_TONES).toEqual([
      "neutral",
      "destructive",
    ]);
    expect(APP_INPUT_SIZES).toEqual(["default", "compact"]);
    expect(TOOLTIP_SIDES).toEqual(["top", "bottom", "left", "right"]);
    expect(TOOLTIP_ALIGNS).toEqual(["start", "center", "end"]);
    expect(PET_DISPLAY_GLOW_VARIANTS).toEqual([
      "gallery-default",
      "running-playground",
      "running-summary",
    ]);
  });

  test("publishes Badge, Card, and Input intent as stable DOM attributes", () => {
    const markup = renderToStaticMarkup(
      createElement(
        Fragment,
        null,
        createElement(Badge, { variant: "ghost" }, "Ghost"),
        createElement(Input, { size: "default" }),
        createElement(Input, { size: "compact" }),
        createElement(
          Card,
          null,
          createElement(
            CardHeader,
            { size: "compact" },
            createElement(CardTitle, null, "Title"),
            createElement(CardDescription, null, "Description"),
          ),
          createElement(CardContent, { size: "compact" }, "Content"),
          createElement(CardFooter, { size: "compact" }, "Footer"),
        ),
      ),
    );

    expect(markup).toContain('data-variant="ghost"');
    expect(markup).toContain('data-size="default"');
    expect(markup).toContain('data-size="compact"');
    expect(markup).toContain('data-app-card="true"');
    expect(markup.match(/data-section-size="compact"/g)).toHaveLength(3);
    expect(markup).toContain("app-dream-card__title");
    expect(markup).toContain("app-dream-card__description");
  });

  test("keeps visual vocabulary out of shared primitive wrappers", async () => {
    for (const file of PRIMITIVE_WRAPPER_FILES) {
      const source = await Bun.file(new URL(file, import.meta.url)).text();
      expect(source.match(tailwindAppearanceUtility) ?? []).toEqual([]);
      expect(source.match(tailwindArbitraryUtility) ?? []).toEqual([]);
    }
  });

  test("defines semantic primitive anatomy in Dream CSS", async () => {
    const [entry, anatomy, buttonContract] = await Promise.all([
      Bun.file(new URL("../styles/dream.css", import.meta.url)).text(),
      Bun.file(new URL("../styles/dream/anatomy.css", import.meta.url)).text(),
      Bun.file(
        new URL("../styles/dream/button-contract.css", import.meta.url),
      ).text(),
    ]);

    expect(entry).toContain('@import "./dream/anatomy.css"');
    for (const selector of [
      ".app-dream-card__header",
      '.app-dream-checkbox[data-app-checkbox="true"]',
      '.app-dream-input[data-size="compact"]',
      ".app-menu-label",
      ".app-dialog-title",
      '.app-sheet-content[data-size="sm"]',
      '.app-dream-tooltip[data-multiline="true"]',
    ]) {
      expect(anatomy).toContain(selector);
    }
    for (const size of [
      "default",
      "sm",
      "lg",
      "icon",
      "compact",
      "compactIcon",
    ]) {
      expect(buttonContract).toContain(`[data-size="${size}"]`);
    }
  });
});
