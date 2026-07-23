import {
  APP_BUTTON_SHAPES,
  APP_BUTTON_SIZES,
  APP_BUTTON_TONES,
  APP_BUTTON_VARIANTS,
} from "@/shared/ui/button";
import { APP_BADGE_VARIANTS } from "@/shared/ui/badge";
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

export type AppearancePrimitiveFixtureId =
  | "buttons"
  | "forms"
  | "content"
  | "toggles"
  | "menus"
  | "overlays";
export type AppearanceFixtureId =
  | "surface-roles"
  | AppearancePrimitiveFixtureId;

export interface AppearanceFixtureDefinition {
  id: AppearanceFixtureId;
  title: string;
  description: string;
  contracts: readonly string[];
  renderedBy: "surface-role-matrix" | "primitive-gallery";
}

export const APPEARANCE_SURFACE_ROLE_FIXTURE = {
  id: "surface-roles",
  title: "Surface roles",
  description: "Every public GlassSurface role and presentation axis.",
  contracts: [
    ...XIA_SURFACE_ROLES.map((value) => `surface.role.${value}`),
    ...XIA_GLASS_MATERIALS.map((value) => `surface.material.${value}`),
    ...GLASS_ELEVATIONS.map((value) => `surface.elevation.${value}`),
    ...GLASS_SHAPES.map((value) => `surface.shape.${value}`),
    ...GLASS_TINTS.map((value) => `surface.tint.${value}`),
  ],
  renderedBy: "surface-role-matrix",
} as const satisfies AppearanceFixtureDefinition;

/** Gallery fixtures are generated from the same public vocabularies as UI. */
export const APPEARANCE_PRIMITIVE_FIXTURE_REGISTRY = [
  {
    id: "buttons",
    title: "Buttons",
    description: "Every public variant, tone, size and shape axis.",
    contracts: [
      ...APP_BUTTON_VARIANTS.map((value) => `button.variant.${value}`),
      ...APP_BUTTON_TONES.map((value) => `button.tone.${value}`),
      ...APP_BUTTON_SIZES.map((value) => `button.size.${value}`),
      ...APP_BUTTON_SHAPES.map((value) => `button.shape.${value}`),
      ...FUN_BUTTON_EFFECTS.map((value) => `button.effect.${value}`),
      "button.state.disabled",
    ],
    renderedBy: "primitive-gallery",
  },
  {
    id: "forms",
    title: "Fields and choices",
    description: "Input, Select, Progress and separators in meaningful states.",
    contracts: [
      ...APP_INPUT_SIZES.map((value) => `input.size.${value}`),
      "input.state.placeholder",
      "input.state.readonly",
      "input.state.invalid",
      "input.state.disabled",
      "select.default",
      "select.disabled",
      "progress.determinate",
      "separator.horizontal",
      "separator.vertical",
    ],
    renderedBy: "primitive-gallery",
  },
  {
    id: "content",
    title: "Cards and badges",
    description: "Canonical content anatomy and every Badge variant.",
    contracts: [
      ...APP_CARD_SECTION_SIZES.map((value) => `card.section-size.${value}`),
      ...APP_BADGE_VARIANTS.map((value) => `badge.variant.${value}`),
      ...DREAM_STATUS_TONES.map((value) => `status-badge.tone.${value}`),
      "status-badge.state.icon-only",
      ...APP_USER_AVATAR_TONES.map((value) => `user-avatar.tone.${value}`),
      ...APP_USER_AVATAR_SHAPES.map((value) => `user-avatar.shape.${value}`),
      "user-avatar.branch.fallback",
      "user-avatar.branch.image",
      "user-avatar.branch.wash",
      ...PET_DISPLAY_GLOW_VARIANTS.map((value) => `pet.glow.${value}`),
      "surface.state.interactive",
      "surface.state.focus-ring",
    ],
    renderedBy: "primitive-gallery",
  },
  {
    id: "toggles",
    title: "Switches and segments",
    description: "Checked, unchecked, disabled and keyboard-driven selections.",
    contracts: [
      "inline-switch.checked",
      "inline-switch.unchecked",
      "inline-switch.disabled",
      "segment-switch.two",
      "segment-switch.three",
      "segment-switch.compact",
      "segment-switch.disabled",
    ],
    renderedBy: "primitive-gallery",
  },
  {
    id: "menus",
    title: "Dropdown menus",
    description: "Production item, choice, state, tone and shortcut branches.",
    contracts: [
      ...APP_DROPDOWN_MENU_ITEM_TONES.map(
        (value) => `dropdown-menu.item.tone.${value}`,
      ),
      "dropdown-menu.checkbox.checked",
      "dropdown-menu.checkbox.unchecked",
      "dropdown-menu.radio.selected",
      "dropdown-menu.radio.unselected",
      "dropdown-menu.item.disabled",
      "dropdown-menu.item.shortcut",
    ],
    renderedBy: "primitive-gallery",
  },
  {
    id: "overlays",
    title: "Transient overlays",
    description: "Real portaled Tooltip, Dialog, Sheet and SecondaryReveal.",
    contracts: [
      ...TOOLTIP_SIDES.flatMap((side) =>
        TOOLTIP_ALIGNS.map((align) => `tooltip.${side}.${align}`),
      ),
      "tooltip.multiline",
      "dialog",
      ...SHEET_SIDES.map((value) => `sheet.side.${value}`),
      ...SHEET_SIZES.map((value) => `sheet.size.${value}`),
      "sheet.centered",
      "sheet.window-chrome-safe-area",
      "secondary-reveal",
    ],
    renderedBy: "primitive-gallery",
  },
] as const satisfies readonly AppearanceFixtureDefinition[];

/** Complete machine-readable contract index across all Appearance sections. */
export const APPEARANCE_FIXTURE_REGISTRY = [
  APPEARANCE_SURFACE_ROLE_FIXTURE,
  ...APPEARANCE_PRIMITIVE_FIXTURE_REGISTRY,
] as const satisfies readonly AppearanceFixtureDefinition[];
