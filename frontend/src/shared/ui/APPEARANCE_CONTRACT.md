# XiaDown Dream CSS Appearance Contract

Dream CSS is the only product appearance system. A feature describes **what an
element is** and **where it belongs**; shared tokens, contracts, and primitives
decide how that element looks in a theme, Surface Style, platform, and
accessibility mode.

This document governs all new UI and every migrated screen. The more focused
[Surface Contract](./SURFACE_CONTRACT.md) remains authoritative for Glass and
Contrast materials.

## Ownership layers

| Layer | Owns | Location |
| --- | --- | --- |
| Theme palette and document foundation | Raw theme colors, light/dark values, global typography, focus, scrollbars, and platform document chrome | `dream/foundation.css` |
| Foundation tokens | Spacing, typography, radii, motion, semantic state colors, shared geometry | `dream/tokens.css` |
| Visual contracts | Surface, layout, control, status, dialog, button, motion recipes | `dream/glass.css`, `dream/layout-contract.css`, `dream/controls.css`, `dream/status-contract.css`, `dream/dialog-contract.css`, `dream/button-contract.css`, `dream/motion.css` |
| Shared primitives | Stable DOM, ARIA, and `data-*` semantics | `shared/ui/*` |
| Feature layout | Content flow, responsive placement, domain-specific composition | `app/**` feature CSS |

The dependency direction is one way: feature code consumes shared primitives
and tokens. A feature must never redefine a foundation token or copy a shared
component recipe.

## Canonical semantic vocabulary

- Accent: `--app-accent-brand`, `--app-accent-text`,
  `--app-accent-solid`, `--app-accent-on-solid`,
  `--app-accent-surface`, `--app-accent-ring`.
- Text: `--app-text-primary`, `--app-text-secondary`,
  `--app-text-tertiary`, `--app-text-disabled`.
- State: `--app-status-tone-*` and `--app-status-surface-*`.
- Geometry: `--app-space-*`, `--app-radius-*`, control/menu/search/list tokens.
- Surface: `--app-surface-*` and the roles in `surface-contract.ts`.

Never hard-code `white` as an active foreground. Use the semantic
`on-solid`/`on-accent` token so a selected icon remains legible in Light, Dark,
custom themes, forced colors, Glass, and Contrast.

## Required shared patterns

### Dialogs and sheets

- `DialogContent` and `SheetContent` own one modal glass surface. Header, body,
  and footer are layout regions within that surface and never add divider
  rules, independent backgrounds, shadows, or another backdrop sample.
- Interactive choices inside a modal use transparent hit areas with tonal
  hover and selection states. Do not wrap every browser, profile, or scan row
  in a nested card surface. Semantic warning and danger messages may retain
  their status tint so state remains unambiguous.

### Primary page header

- Every route declares a `WorkspacePageContract` and has exactly one `h1`.
  `heading="display"` and `heading="hero"` render that heading visibly at the
  start of the scrolling content; `heading="assistive"` renders the same route
  label as `h1.app-visually-hidden`; `heading="host-owned"` means the presentation host is
  solely responsible for the heading. Never render a second Station-owned h1.
- Page actions begin at the leading edge inside
  `app-workspace-primary-header__actions`.
- A Primary header that hosts native Windows caption controls sets
  `data-window-controls="true"` and consumes the shared trailing safe-area
  token. Do not place product actions in that area.
- On ordinary Primary pages, the Header is transparent at the scroll origin.
  When the shared Content viewport passes beneath it, the shared page contract
  reveals one light chrome material and a short bottom fade. The Header never
  adds a divider or strong shadow, and the visible route title remains in
  Content rather than moving into chrome. Search, split, canvas, host-scrolled,
  immersive, and named custom layouts keep their reviewed special geometry.
- Icon-only title actions use `WorkspacePrimaryHeaderAction`. The shared
  primitive owns the circular `compactIcon` Ghost button, tooltip, accessible
  name, and native-window drag exclusion; Stations provide only the icon,
  localized label, state, and behavior.
- Related borderless title actions use `WorkspacePrimaryHeaderActionGroup`.
  Proximity is the visual separator: actions stay compact within a semantic
  group and groups receive a larger gap. Do not add decorative vertical rules
  to the drag rail. Title-action dropdowns use
  `WorkspacePrimaryHeaderMenuContent`, which opens below and centered on its
  trigger; Stations must not choose leading or trailing alignment.

### Workspace page anatomy

- Use `WorkspacePage`, `WorkspacePageTopBar`, `WorkspacePageContent`, and, when
  the route has a true page footer, `WorkspacePageFooter`. The root publishes
  presentation, recipe, chrome, heading, layout, scrolling, density, and
  immersion as stable `data-page-*` attributes.
- Content has one vertical scroll owner. `scroll="content"` makes the shared
  content viewport own it; `scroll="panes"` delegates it to explicit split
  panes; `scroll="host"` delegates it to the presentation host. Do not add a
  nested `overflow: auto` to ordinary content. Shared scroll-aware Header
  state listens only to that declared content owner and resets across routes.
- `browse` pages use a visible content heading and card/shelf composition;
  `collection` and `feed` pages normally use an action TopBar and an assistive
  heading. These are recipes, not Station-specific skins.
- `footer` means pagination, commands, status, or an intentional overlay. The
  Music/YouTube transport and Companion chrome are host-owned controls, not a
  page footer. A page footer is separated by spacing, never by a rule against
  content. Do not render an empty footer to complete a visual template.
- On an ordinary Primary page with the shared Content scroller, pagination,
  command, and status Footers are bottom chrome layers. Content continues
  beneath the Footer while it has more material to reveal, using a short
  upward fade and the same chrome recipe as the Header; the Footer returns to
  the canvas at the content end. Search keeps its special Header geometry but
  may still use this shared Footer. Split, canvas, host-scrolled, custom, and
  explicitly overlaid Footers retain their reviewed geometry. Never add a
  divider, strong shadow, or feature-owned blur to simulate this layer.
- A `custom` recipe, TopBar, or content layout requires a stable
  `customContractId` naming the reviewed exception.
- The contract classifies navigable route roots. Companion content, portaled
  players, and an embedded fullscreen state remain presentation-host chrome;
  they do not nest a second `WorkspacePage`. Their host owns its accessible
  name, modal/focus behavior, header, scrolling, and controls. A Companion
  scroller must explicitly name its active destination with
  `data-companion-scroll-owner`; the host may then tint its existing Header
  after scrolling, without adding a second glass sampler or moving the Header
  out of flow. Fullscreen presentation always disables this tint.

### Search page

Use `WorkspaceSearchControl`. Stations provide query behavior, placeholder,
and submit/clear labels only. The shared primitive owns icon → input → clear →
submit order, geometry, focus, disabled state, and Dream roles. Its height is
owned by `--app-workspace-search-height` so empty Library, Music, YouTube, and
RSS search pages keep the same comfortable entry target. Never fork a station
search shell in feature CSS.

### Menus and compact rows

- Shared menu items use `--app-menu-item-min-height`.
- Direct menu artwork icons share one optical size. Trailing state glyphs and
  other semantic indicators use `data-menu-indicator="true"` so they retain
  their own size; the shared Checkbox and Radio primitives publish this marker.
- Compact menu actions use `--app-menu-action-height`; related controls must
  consume the token instead of repeating its current pixel value.
- A dropdown that represents a full-width trigger uses Radix's
  `--radix-dropdown-menu-trigger-width`; do not guess a fixed width.

### Semantic button geometry

- Shared `Button` owns its base recipe. A domain control that needs reviewed
  geometry publishes `--app-button-inline-size`, `--app-button-block-size`,
  `--app-button-padding`, `--app-button-gap`, and `--app-button-icon-size`
  instead of fighting the contract with width/height utility overrides.
- Artwork triggers that must let an image fill the complete square set
  `--app-button-border: 0`; a transparent default border still consumes one
  pixel of the image's content box.
- Size tiers must express real hierarchy. Mode actions may be smaller than
  previous/next, and the primary play action may be larger; unrelated footer
  regions use one common button geometry.

### Account menu access control

- Access is a single name/value row: the leading name is “Remote” and the
  trailing value uses the canonical `DreamInlineSwitchVisual`. Off means
  local-only; on means remote access.
- The complete row remains a Radix `menuitemcheckbox`, so menu arrow keys can
  reach it and Space/Enter can toggle it. Prevent selection from closing the
  menu; do not nest an independent raw button inside the menu item.
- The row and adjacent Settings/actions use `--app-menu-action-height`. The
  row uses the same leading/trailing alignment and type scale as Settings
  name/value rows without changing the trigger-owned menu width.
- Loading and mutation states disable the switch. Its accessible description
  continues to expose the current status or error, even though no extra status
  copy is painted in the compact menu.

### Persistent pane geometry

- Sidebar → Primary, Primary leading subpane → trailing subpane, and Primary →
  docked Companion all consume `--app-workspace-divider-width` and
  `--app-workspace-divider-color`.
- Every boundary has one edge owner. A host and its Glass child must never both
  paint the same divider, inset line, or structural shadow.
- The Primary host paints `--app-workspace-primary-surface` exactly once.
  Single-column and split-column routes use the same opacity; descendants use
  `app-workspace-primary-subpane` and remain transparent.
- Add `app-workspace-primary-subpane--leading` only when that element is truly
  adjacent to a trailing Primary subpane. Portaled Companion content is not a
  Primary column.

### Selection lists

The scrolling host uses `app-dream-selection-list`; direct selectable rows use
`app-dream-selection-item`. The shared role owns a compact symmetric inline
inset and one stable gutter on the scrollbar edge. Feature lists may add block
spacing so rounded selected backgrounds stay detached from adjacent rows.

### Primary–Companion selection

- A Primary item is selected for Companion presentation only while the
  matching Companion destination is open and its context names that item.
  Use `defineCompanionSelectionContract` and
  `resolveActiveCompanionSelection`; a feature-local object may cache render
  data but must never remain the authority for selected styling.
- Closing the Companion, navigating out of its scope, replacing its
  destination, or losing/mismatching its context releases the Primary selected
  state immediately. The same resolved active selection drives the card,
  Companion body, and Companion footer.

### Status and health labels

Use `StatusBadge` with a localized label, semantic `tone`, and optional icon or
marker. Raw backend values such as `missing`, `trashed`, or `needs_review` must
be mapped before rendering. A feature can decide the tone but cannot define
its hue, fill, border, icon size, or pill geometry.

### Navigation selection

- Unselected navigation icons use the semantic accent text color.
- A selected navigation item has exactly one `accent-solid` row surface. Its
  icon, label, count, and favicon inherit `accent-on-solid`; no descendant may
  add another selected background or icon tile.
- Selection is never represented by a literal white icon. Dark and custom
  themes resolve their legible foreground through `accent-on-solid`.

### Artwork and placeholders

- Render real artwork as an image.
- Render a missing/default asset with a runtime SVG icon and Dream tokens.
  Do not add theme-colored bitmap placeholders that cannot respond to theme or
  accessibility changes.
- Keep imagery free of ambiguous state text. Duration, size, availability, and
  health belong in explicit metadata or a semantic badge.

### Persistent presentation preferences

Presentation choices that change information density, such as Library grid vs
list, persist under a versioned key. Storage failure must fall back safely and
must never make the view unusable.

## Feature CSS boundary

Feature CSS may define:

- grid/flex composition and responsive breakpoints;
- content-specific aspect ratios and intrinsic sizes;
- placement of shared roles;
- domain animation that is not a general interaction state.

Feature CSS must not define:

- a new palette, status color, blur, material, shadow hierarchy, or focus ring;
- a duplicate search, button, menu, badge, dialog, sheet, dropdown, or selected
  row recipe;
- raw structural backgrounds for persistent panes;
- platform checks for appearance;
- copied pixel values when a shared geometry token exists.

## Foreign provider document boundary

Dream governs every XiaDown-owned DOM tree and every generated document that
XiaDown presents as product UI, including RSS reader and print documents.
Remote provider WebViews are a separate transport boundary: their document-start
bridges cannot load the app stylesheet and may inject narrowly scoped provider
CSS only to isolate or expose the provider's media element.

That bridge CSS may control visibility, geometry, clipping, overflow,
pointer-events, `object-fit`, and the black media canvas required to prevent a
remote-page flash. It may target provider-owned chrome solely to hide it or to
restore provider controls during provider fullscreen. It must not define a
XiaDown component class, Dream token, palette, control, typography, focus,
shadow, or material recipe. Product controls remain in the XiaDown host DOM and
therefore remain Dream-owned.

## Implementation checklist

1. Choose semantic surface, control, state, and layout roles.
2. Reuse a shared primitive before adding markup.
3. Add a foundation token only when the value is truly cross-feature.
4. Keep feature CSS limited to composition.
5. Verify Light/Dark, Glass/Contrast, macOS/Windows safe areas, keyboard focus,
   reduced transparency/motion, forced colors, and a narrow content width.
6. Add an Appearance audit rule when a contract can be checked statically.
7. Run `bun run audit:appearance`, relevant UI tests, and a desktop visual QA.
