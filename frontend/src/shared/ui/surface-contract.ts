export const XIA_GLASS_MATERIALS = [
  "regular",
  "panel",
  "clear",
  "solid",
] as const;
export type GlassMaterial = (typeof XIA_GLASS_MATERIALS)[number];

/**
 * Product-level surface roles. Components declare intent through this type;
 * the material recipe remains centralized below so feature code never needs
 * to know whether a status or overlay uses regular or panel glass.
 */
export const XIA_SURFACE_ROLES = [
  "canvas",
  "chrome",
  "content",
  "status",
  "overlay",
  "card",
  "inset",
  "control",
] as const;
export type XiaSurfaceRole = (typeof XIA_SURFACE_ROLES)[number];

export interface XiaSurfaceRolePreset {
  readonly material: GlassMaterial;
}

export const XIA_SURFACE_ROLE_PRESETS = {
  canvas: { material: "clear" },
  chrome: { material: "regular" },
  content: { material: "regular" },
  status: { material: "regular" },
  overlay: { material: "panel" },
  card: { material: "regular" },
  inset: { material: "solid" },
  control: { material: "regular" },
} as const satisfies Record<XiaSurfaceRole, XiaSurfaceRolePreset>;

export type XiaSurfaceAttributes = {
  "data-surface-role": XiaSurfaceRole;
  "data-material": GlassMaterial;
};

export function getXiaSurfaceAttributes(
  role: XiaSurfaceRole,
): XiaSurfaceAttributes {
  return {
    "data-surface-role": role,
    "data-material": XIA_SURFACE_ROLE_PRESETS[role].material,
  };
}
