# Xia Surface Contract

Surface role describes **why a surface exists**, not how it currently looks.
Choose the role once; `Surface Style`, platform material, and accessibility CSS
decide its final fill, blur, and contrast.

## Roles

| Role | Use it for | Do not use it for |
| --- | --- | --- |
| `canvas` | The single window/workspace ambient substrate | A component, card, or scrolling region |
| `chrome` | Persistent app navigation, toolbars, sidebar, docked companion | Temporary menus or page content |
| `content` | The primary reading/work area | Nested cards or controls |
| `status` | Toasts, notifications, progress and compact state feedback | Modal prompts or content cards |
| `overlay` | Dropdowns, dialogs, sheets, popovers and secondary reveals | Persistent chrome |
| `card` | A discrete content grouping above a content surface | The whole page or a modal |
| `inset` | A recessed region inside another surface, such as an editor or list well | A floating surface |
| `control` | A standalone control or one grouped control surface | A content container |

The only role-to-material mapping lives in `XIA_SURFACE_ROLE_PRESETS`:
`status` is semantically `regular`, `overlay` is `panel`. This mapping selects
the primitive family; it does not mean `status` reuses the generic regular
optical recipe. Its denser fill, filter, rim, shadow, and specular tokens remain
centralized in shared CSS. Decorative status artwork must also consume the
shared `--app-surface-status-artwork-*` veil, filter, and opacity tokens so
cover text never competes with controls. Never reproduce either mapping or
artwork recipe in a feature.

## Glass and Contrast

- Roles do not change when the user switches Surface Style.
- Glass uses the centralized native/CSS material and tint recipes. Settings
  adds the shared translucent `--app-surface-window-glass-wash` above the
  native layer so an explicit in-App Light/Dark choice cannot disagree with
  the OS material appearance.
- Contrast maps `canvas`, persistent `chrome`, and `content` to the exact same
  opaque `--app-surface-canvas` token. That token resolves to the Settings
  window's canonical `--app-surface-window-canvas`, so Settings and every Main
  persistent pane share one recipe and one source of truth. Local roles such as
  `status` are remapped centrally in shared CSS/tokens. Do not conditionally
  change `material` in component code.
- Pixel remains a theme pack; it is not a surface role or material.

## Canonical API

Prefer `GlassSurface surfaceRole` for React surfaces. `surfaceRole` is visual
semantics and is independent from the element's accessibility `role`.

```tsx
<GlassSurface
  role="status"
  aria-live="polite"
  surfaceRole="status"
  elevation="floating"
  shape="panel"
>
  Download complete
</GlassSurface>
```

```tsx
<GlassSurface surfaceRole="card" elevation="embedded" shape="card">
  <LibrarySummary />
</GlassSurface>
```

Shared `DropdownMenuContent`, `DialogContent`, and `SheetContent` already own
the `overlay` contract. Callers should only provide content and placement:

```tsx
<DialogContent>
  <DialogTitle>Remove item?</DialogTitle>
  <DialogFooter>...</DialogFooter>
</DialogContent>
```

For a Radix primitive that cannot render `GlassSurface`, use the shared helper
and spread it last so the canonical attributes cannot be overridden:

```tsx
<Popover.Content
  {...props}
  className="app-glass-surface app-menu-content"
  data-elevation="floating"
  data-shape="control"
  {...getXiaSurfaceAttributes("overlay")}
/>
```

## Prohibited in feature code

- Do not set `material`, `data-material`, or recreate a role-to-material map.
- Do not add `backdrop-filter`, `-webkit-backdrop-filter`, or bespoke blur.
- Do not paint structural surfaces directly with theme colors such as
  `background`, `sidebar-background`, or `primary`; use the role contract and
  shared surface tokens.
- Do not detect macOS, Windows, native material, or transparency support in a
  feature to choose a recipe.

## Fallback and accessibility

- Always keep the semantic role when native glass is unavailable. The shared
  runtime chooses native, CSS, or solid material.
- Reduced transparency and unsupported backdrop filtering must become solid
  centrally; features must not implement their own fallback.
- Forced-colors and increased-contrast rules own final fill, border, focus, and
  text contrast. Do not override them with inline structural colors.
- Preserve the correct ARIA role, focus trap, labels, and live-region behavior;
  a `surfaceRole` never replaces accessibility semantics.
