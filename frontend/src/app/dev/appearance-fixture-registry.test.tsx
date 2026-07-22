import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import { APP_BADGE_VARIANTS } from "@/shared/ui/badge";
import {
  APP_BUTTON_SHAPES,
  APP_BUTTON_SIZES,
  APP_BUTTON_TONES,
  APP_BUTTON_VARIANTS,
} from "@/shared/ui/button";
import { APP_CARD_SECTION_SIZES } from "@/shared/ui/card";
import { APP_DROPDOWN_MENU_ITEM_TONES } from "@/shared/ui/dropdown-menu";
import {
  GLASS_ELEVATIONS,
  GLASS_SHAPES,
  GLASS_TINTS,
} from "@/shared/ui/glass-surface";
import { APP_INPUT_SIZES } from "@/shared/ui/input";
import { FUN_BUTTON_EFFECTS } from "@/shared/ui/fun-button-effect";
import { PET_DISPLAY_GLOW_VARIANTS } from "@/shared/ui/pet-player";
import { SHEET_SIDES, SHEET_SIZES } from "@/shared/ui/sheet";
import { DREAM_STATUS_TONES } from "@/shared/ui/status-badge";
import { TOOLTIP_ALIGNS, TOOLTIP_SIDES } from "@/shared/ui/tooltip";
import {
  APP_USER_AVATAR_SHAPES,
  APP_USER_AVATAR_TONES,
} from "@/shared/ui/user-avatar";
import {
  XIA_GLASS_MATERIALS,
  XIA_SURFACE_ROLES,
} from "@/shared/ui/surface-contract";

import {
  APPEARANCE_FIXTURE_REGISTRY,
  APPEARANCE_PRIMITIVE_FIXTURE_REGISTRY,
} from "./appearance-fixture-registry";
import {
  APPEARANCE_FIXTURE_RENDERERS,
  PrimitiveFixtureGallery,
} from "./PrimitiveFixtureGallery";

describe("Appearance primitive fixture registry", () => {
  test("has exactly one renderer for every registered fixture", () => {
    expect(Object.keys(APPEARANCE_FIXTURE_RENDERERS).sort()).toEqual(
      APPEARANCE_PRIMITIVE_FIXTURE_REGISTRY.map(({ id }) => id).sort(),
    );
  });

  test("maps every canonical public vocabulary value to a fixture contract", () => {
    const vocabularies = [
      ["button.variant", APP_BUTTON_VARIANTS],
      ["button.tone", APP_BUTTON_TONES],
      ["button.size", APP_BUTTON_SIZES],
      ["button.shape", APP_BUTTON_SHAPES],
      ["button.effect", FUN_BUTTON_EFFECTS],
      ["input.size", APP_INPUT_SIZES],
      ["card.section-size", APP_CARD_SECTION_SIZES],
      ["badge.variant", APP_BADGE_VARIANTS],
      ["dropdown-menu.item.tone", APP_DROPDOWN_MENU_ITEM_TONES],
      ["status-badge.tone", DREAM_STATUS_TONES],
      ["user-avatar.tone", APP_USER_AVATAR_TONES],
      ["user-avatar.shape", APP_USER_AVATAR_SHAPES],
      ["pet.glow", PET_DISPLAY_GLOW_VARIANTS],
      ["surface.role", XIA_SURFACE_ROLES],
      ["surface.material", XIA_GLASS_MATERIALS],
      ["surface.elevation", GLASS_ELEVATIONS],
      ["surface.shape", GLASS_SHAPES],
      ["surface.tint", GLASS_TINTS],
      ["sheet.side", SHEET_SIDES],
      ["sheet.size", SHEET_SIZES],
    ] as const;
    const contracts = APPEARANCE_FIXTURE_REGISTRY.flatMap(
      (fixture) => fixture.contracts,
    );

    for (const [prefix, axis] of vocabularies) {
      expect(new Set(axis).size).toBe(axis.length);
      for (const value of axis) {
        expect(contracts).toContain(`${prefix}.${value}`);
      }
    }
    for (const side of TOOLTIP_SIDES) {
      for (const align of TOOLTIP_ALIGNS) {
        expect(contracts).toContain(`tooltip.${side}.${align}`);
      }
    }
    expect(new Set(contracts).size).toBe(contracts.length);

    const menuContracts = APPEARANCE_FIXTURE_REGISTRY.find(
      ({ id }) => id === "menus",
    )?.contracts;
    expect(menuContracts).toEqual([
      "dropdown-menu.item.tone.neutral",
      "dropdown-menu.item.tone.destructive",
      "dropdown-menu.checkbox.checked",
      "dropdown-menu.checkbox.unchecked",
      "dropdown-menu.radio.selected",
      "dropdown-menu.radio.unselected",
      "dropdown-menu.item.disabled",
      "dropdown-menu.item.shortcut",
    ]);

    const buttonContracts = APPEARANCE_FIXTURE_REGISTRY.find(
      ({ id }) => id === "buttons",
    )?.contracts;
    expect(buttonContracts).toHaveLength(
      APP_BUTTON_VARIANTS.length +
        APP_BUTTON_TONES.length +
        APP_BUTTON_SIZES.length +
        APP_BUTTON_SHAPES.length +
        FUN_BUTTON_EFFECTS.length +
        1,
    );
  });

  test("renders production primitives in semantic fixture cards", () => {
    const markup = renderToStaticMarkup(<PrimitiveFixtureGallery />);

    for (const { id } of APPEARANCE_PRIMITIVE_FIXTURE_REGISTRY) {
      expect(markup).toContain(`data-appearance-fixture="${id}"`);
    }
    expect(markup.match(/data-surface-role="card"/g)).toHaveLength(
      APPEARANCE_PRIMITIVE_FIXTURE_REGISTRY.length,
    );
    expect(markup).toContain("app-dream-button");
    expect(markup).toContain("app-dream-inline-switch");
    expect(markup).toContain("app-dream-segment-switch");
    expect(markup).toContain('role="progressbar"');
    for (const variant of APP_BUTTON_VARIANTS) {
      expect(markup).toContain(`data-variant="${variant}"`);
    }
    for (const tone of APP_BUTTON_TONES) {
      expect(markup).toContain(`data-tone="${tone}"`);
    }
    for (const size of APP_BUTTON_SIZES) {
      expect(markup).toContain(`data-size="${size}"`);
    }
    for (const shape of APP_BUTTON_SHAPES) {
      expect(markup).toContain(`data-shape="${shape}"`);
    }
    for (const effect of FUN_BUTTON_EFFECTS) {
      expect(markup).toContain(`data-fixture-button-effect="${effect}"`);
    }
    for (const size of APP_INPUT_SIZES) {
      expect(markup).toContain(`data-fixture-input-size="${size}"`);
    }
    for (const size of APP_CARD_SECTION_SIZES) {
      expect(markup).toContain(`data-fixture-card-section-size="${size}"`);
    }
    for (const variant of APP_BADGE_VARIANTS) {
      expect(markup).toContain(`data-fixture-badge-variant="${variant}"`);
    }
    expect(markup.match(/data-app-status-badge="true"/g)).toHaveLength(
      DREAM_STATUS_TONES.length + 1,
    );
    expect(markup).toContain('data-icon-only="true"');
    for (const tone of DREAM_STATUS_TONES) {
      expect(markup).toContain(`data-tone="${tone}"`);
    }
    for (const tone of APP_USER_AVATAR_TONES) {
      for (const shape of APP_USER_AVATAR_SHAPES) {
        expect(markup).toContain(
          `data-fixture-user-avatar="${tone}-${shape}"`,
        );
      }
    }
    expect(markup).toContain('data-fixture-user-avatar-image="theme-wash"');
    expect(markup).toContain('class="app-user-avatar__image"');
    expect(markup).toContain('class="app-user-avatar__wash"');
    for (const variant of PET_DISPLAY_GLOW_VARIANTS) {
      expect(markup).toContain(`data-fixture-pet-glow="${variant}"`);
      expect(markup).toContain(`data-glow-variant="${variant}"`);
    }
    for (const side of TOOLTIP_SIDES) {
      for (const align of TOOLTIP_ALIGNS) {
        expect(markup).toContain(`data-fixture-tooltip="${side}-${align}"`);
      }
    }
    expect(markup).toContain('data-fixture-tooltip="multiline"');
    expect(markup).toContain('data-compact="true"');
    expect(markup).toContain('data-fixture-glass-interactive="true"');
    expect(markup).toContain('data-interactive="true"');
    expect(markup).toContain('data-fixture-glass-focus-ring="true"');
    expect(markup).toContain('data-focus-ring="true"');
    for (const side of SHEET_SIDES) {
      expect(markup).toContain(`data-fixture-sheet-side="${side}"`);
    }
    for (const size of SHEET_SIZES) {
      expect(markup).toContain(`data-fixture-sheet-size="${size}"`);
    }
    expect(markup).toContain('data-fixture-sheet-centered="true"');
    expect(markup).toContain(
      'data-fixture-sheet-window-chrome-safe-area="true"',
    );
    expect(markup).toContain('data-fixture-dropdown-menu="true"');
    expect(markup).toContain("Open dialog");
    expect(markup).toContain("right sm sheet");
  });

  test("builds every menu branch from shared production primitives", async () => {
    const [gallerySource, primitiveSource, controls] = await Promise.all([
      Bun.file(new URL("./PrimitiveFixtureGallery.tsx", import.meta.url)).text(),
      Bun.file(
        new URL("../../shared/ui/dropdown-menu.tsx", import.meta.url),
      ).text(),
      Bun.file(
        new URL("../../shared/styles/dream/controls.css", import.meta.url),
      ).text(),
    ]);

    expect(gallerySource).toContain("<DropdownMenuCheckboxItem");
    expect(gallerySource).toContain("<DropdownMenuRadioGroup");
    expect(gallerySource).toContain("<DropdownMenuRadioItem");
    expect(gallerySource).toContain("<DropdownMenuShortcut>");
    expect(gallerySource).toContain('data-fixture-menu-branch="disabled"');
    expect(gallerySource).toContain('tone="destructive"');
    expect(primitiveSource).toContain("APP_DROPDOWN_MENU_ITEM_TONES");
    expect(primitiveSource).toContain('data-tone={tone}');
    expect(controls).toMatch(
      /:is\(\.app-dream-menu-item, \.app-menu-item\)\[data-tone="destructive"\]\s*\{[^}]*color:\s*hsl\(var\(--destructive\)\)/s,
    );
  });
});
