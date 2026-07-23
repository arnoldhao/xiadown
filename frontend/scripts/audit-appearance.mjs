import { readdir, readFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import postcss from "postcss";
import ts from "typescript";

const scriptPath = fileURLToPath(import.meta.url);
const frontendRoot = path.resolve(path.dirname(scriptPath), "..");
const repoRoot = path.resolve(frontendRoot, "..");
const srcRoot = path.join(frontendRoot, "src");
const foreignProviderBridgePaths = [
  "internal/presentation/wails/listen_player_handler.go",
  "internal/presentation/wails/listen_live_player_handler.go",
  "internal/presentation/wails/rss_video_transport_bridge.go",
];
const glassContractPath = "shared/styles/dream/glass.css";
const tokenPath = "shared/styles/dream/tokens.css";
const statusContractPath = "shared/styles/dream/status-contract.css";
const layoutContractPath = "shared/styles/dream/layout-contract.css";
const surfaceContractPath = "shared/ui/surface-contract.ts";
const statusBadgePath = "shared/ui/status-badge.tsx";
const appearanceContractPath = "shared/ui/APPEARANCE_CONTRACT.md";
const settingsShellPath = "shared/styles/dream/shell.css";
const workspaceSurfacePath = "app/workspace/workspace.css";
const workspaceNavigationPath = "app/workspace/workspace-navigation.css";
const workspaceAppearancePath = "shared/styles/dream/workspace.css";
const mainAppPath = "app/main/MainApp.tsx";
const workspaceActivityPath = "app/main/WorkspaceActivitySurfaces.tsx";
const dreamControlsPath = "shared/styles/dream/controls.css";
const workspaceSearchControlPath = "shared/ui/workspace-search-control.tsx";
const workspaceSearchConsumerPaths = [
  "app/rss/RSSAddSubscriptionPage.tsx",
  "app/library/LibraryWorkspacePage.tsx",
  "app/main/listen/PageView.tsx",
  "app/youtube/YouTubeWorkspacePage.tsx",
];
const rssWorkspacePath = "app/rss/RSSWorkspacePage.tsx";
const rssDocumentSourcePath = "app/rss/workspace-utils.ts";
const rssDocumentCssPath = "shared/styles/dream/rss-documents.css";
const primaryHeaderConsumerPaths = [
  "app/library/LibraryWorkspacePage.tsx",
  "app/main/RunningPage.tsx",
  "features/settings/app-sessions/index.tsx",
  "app/pets-gallery/PetsGalleryPage.tsx",
  "app/rss/RSSWorkspacePage.tsx",
  "app/rss/RSSAddSubscriptionPage.tsx",
  "app/youtube/YouTubeWorkspacePage.tsx",
  "app/main/listen/PageView.tsx",
  "app/sniff-desk/SniffDeskPage.tsx",
  "app/settings/SettingsApp.tsx",
];

const workspacePageLayoutRootsByPath = new Map([
  ["app/library/library.css", ["app-workspace-page", "app-library-page"]],
  ["app/rss/rss-workspace.css", ["app-workspace-page", "rss-workspace-page"]],
  [
    "app/youtube/youtube-workspace.css",
    ["app-workspace-page", "youtube-workspace-page"],
  ],
  [
    "app/main/listen/listen.css",
    ["app-workspace-page", "listen-workspace-page"],
  ],
  ["shared/styles/dream/shell.css", ["app-settings-window"]],
  ["shared/styles/dream/pets.css", ["app-main-pets-page"]],
  [
    "shared/styles/dream/workflows.css",
    ["app-main-running-page", "app-main-app-sessions-page"],
  ],
]);

const workspaceStructuralRootClasses = [
  "app-workspace-primary-subpane",
  "app-workspace-primary-subpane--leading",
  "app-main-list-pane",
  "app-main-detail-pane",
  "app-library-page",
  "app-library-primary-surface",
  "rss-workspace-page",
  "rss-collection-list-pane",
  "rss-entry-list",
  "rss-entry-detail-pane",
  "youtube-workspace-page",
  "listen-list-surface",
  "listen-content-surface",
];

const migratedMaterialFiles = [
  "app/workspace/station-dock-editor.css",
  "app/workspace/workspace-navigation.css",
];

const canonicalSurfaceRoles = [
  "canvas",
  "chrome",
  "content",
  "status",
  "overlay",
  "card",
  "inset",
  "control",
];

const canonicalSurfaceTokens = [
  "--app-surface-window-canvas",
  "--app-surface-window-glass-wash",
  "--app-surface-canvas",
  "--app-surface-status-fill",
  "--app-surface-status-filter",
  "--app-surface-status-line",
  "--app-surface-status-shadow",
  "--app-surface-status-specular",
  "--app-surface-status-specular-opacity",
  "--app-surface-status-artwork-opacity",
  "--app-surface-status-artwork-filter",
  "--app-surface-status-artwork-veil",
  "--app-surface-overlay-fill",
  "--app-surface-overlay-line",
  "--app-surface-overlay-shadow",
  "--app-surface-card-fill",
  "--app-surface-card-line",
  "--app-surface-inset-fill",
  "--app-surface-control-fill",
];

const canonicalAppearanceTokens = [
  "--app-menu-action-height",
  "--app-workspace-search-height",
  "--app-workspace-search-gap",
  "--app-workspace-search-icon-size",
  "--app-workspace-search-clear-size",
  "--app-selection-list-inset",
  "--app-workspace-header-action-gap",
  "--app-workspace-header-group-gap",
  "--app-page-gutter",
  "--app-workspace-page-topbar-height",
  "--app-workspace-page-drag-region-min-width",
  "--app-workspace-page-footer-min-height",
  "--app-workspace-page-display-heading-size",
  "--app-workspace-page-hero-heading-size",
  "--app-status-tone-idle",
  "--app-status-tone-busy",
  "--app-status-tone-success",
  "--app-status-tone-error",
  "--app-status-tone-orphan",
  "--app-status-tone-neutral",
  "--app-status-surface-idle",
  "--app-status-surface-busy",
  "--app-status-surface-success",
  "--app-status-surface-error",
  "--app-status-surface-orphan",
  "--app-status-surface-neutral",
  "--app-workspace-divider-width",
  "--app-workspace-divider-color",
  "--app-workspace-primary-glass-opacity",
  "--app-workspace-primary-glass-surface",
  "--app-workspace-primary-surface",
  "--app-workspace-primary-subpane-surface",
];

const overlayRoleConsumers = [
  "shared/ui/dropdown-menu.tsx",
  "shared/ui/sheet.tsx",
  "shared/ui/dialog.tsx",
];

const roleOwnedFeatureFiles = [
  ...overlayRoleConsumers,
  workspaceActivityPath,
];

const featureCssAppearanceExemptPaths = new Set([
  "index.css",
  "app/dev/appearance-lab.css",
]);
const featureCssAppearanceProperty = /^(?:color|color-scheme|background(?:-.+)?|border(?:-.+)?|border-radius|outline(?:-.+)?|box-shadow|text-shadow|filter|-webkit-backdrop-filter|backdrop-filter|font|font-.+|line-height|letter-spacing|text-align|text-transform|text-decoration(?:-.+)?|accent-color|caret-color|cursor|fill|stroke(?:-.+)?|mix-blend-mode|transition(?:-.+)?|opacity|resize|forced-color-adjust|appearance|clip-path|mask(?:-.+)?|-webkit-mask(?:-.+)?)$/;
const featureCssVisualCustomProperty = /^--.*(?:accent|background|blur|color|fill|filter|font|foreground|material|motion|opacity|radius|shadow|surface|tone|typography|wash)/;

async function collectCssFiles(root, output = []) {
  const entries = await readdir(root, { withFileTypes: true });
  for (const entry of entries) {
    const absolute = path.join(root, entry.name);
    if (entry.isDirectory()) {
      await collectCssFiles(absolute, output);
    } else if (entry.name.endsWith(".css")) {
      output.push(absolute);
    }
  }
  return output;
}

async function collectTsxFiles(root, output = []) {
  const entries = await readdir(root, { withFileTypes: true });
  for (const entry of entries) {
    const absolute = path.join(root, entry.name);
    if (entry.isDirectory()) {
      await collectTsxFiles(absolute, output);
    } else if (entry.name.endsWith(".tsx")) {
      output.push(absolute);
    }
  }
  return output;
}

async function collectTypeScriptFiles(root, output = []) {
  const entries = await readdir(root, { withFileTypes: true });
  for (const entry of entries) {
    const absolute = path.join(root, entry.name);
    if (entry.isDirectory()) {
      await collectTypeScriptFiles(absolute, output);
    } else if (entry.name.endsWith(".ts") || entry.name.endsWith(".tsx")) {
      output.push(absolute);
    }
  }
  return output;
}

function relativeToSrc(absolute, sourceRoot = srcRoot) {
  return path.relative(sourceRoot, absolute).split(path.sep).join("/");
}

function lineNumber(content, offset) {
  return content.slice(0, offset).split("\n").length;
}

function cssRules(content) {
  return [...content.matchAll(/([^{}]+)\{([^{}]*)\}/g)].map((match) => ({
    body: match[2] ?? "",
    selector: (match[1] ?? "").replace(/\/\*[\s\S]*?\*\//g, " ").replace(/\s+/g, " ").trim(),
  }));
}

function selectorTargetsClass(selector, className) {
  const escaped = className.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  const target = new RegExp(`\\.${escaped}(?![\\w-])[^\\s>+~,]*$`);
  return selector
    .split(",")
    .some((branch) => target.test(branch.trim()));
}

function consumesSurfaceRole(source, role) {
  const quotedRole = `["']${role}["']`;
  const directRole = new RegExp(
    `(?:surfaceRole\\s*=\\s*(?:\\{\\s*)?${quotedRole}|` +
      `getXiaSurfaceAttributes\\(\\s*${quotedRole}\\s*\\)|` +
      `data-surface-role\\s*=\\s*(?:\\{\\s*)?${quotedRole})`,
  ).test(source);
  if (directRole) {
    return true;
  }

  const resolvedRole = new RegExp(
    `(?:resolvedSurfaceRole|surfaceRole)[^;]{0,160}${quotedRole}`,
    "s",
  ).test(source);
  return (
    resolvedRole &&
    /getXiaSurfaceAttributes\(\s*resolvedSurfaceRole\s*\)/.test(source)
  );
}

function functionSource(source, functionName) {
  const functionPattern = new RegExp(
    `(?:export\\s+)?function\\s+${functionName}\\s*\\(`,
  );
  const match = functionPattern.exec(source);
  if (!match || match.index === undefined) {
    return "";
  }
  const remainder = source.slice(match.index + match[0].length);
  const nextFunction = /\n\s*(?:export\s+)?function\s+[A-Za-z0-9_]+\s*\(/.exec(
    remainder,
  );
  return source.slice(
    match.index,
    nextFunction?.index === undefined
      ? source.length
      : match.index + match[0].length + nextFunction.index,
  );
}

export function auditCanonicalSurfaceTokens(content) {
  const findings = [];
  for (const token of canonicalSurfaceTokens) {
    if (!new RegExp(`${token.replaceAll("-", "\\-")}\\s*:`).test(content)) {
      findings.push(`${tokenPath}: missing canonical surface token ${token}`);
    }
  }
  return findings;
}

export function auditDreamAppearanceContracts(
  tokenSource,
  statusContractSource,
  statusBadgeSource,
  appearanceGuideSource,
) {
  const findings = [];
  for (const token of canonicalAppearanceTokens) {
    if (!new RegExp(`${token.replaceAll("-", "\\-")}\\s*:`).test(tokenSource)) {
      findings.push(`${tokenPath}: missing canonical appearance token ${token}`);
    }
  }
  if (
    !/--app-selection-list-inset\s*:\s*var\(--app-space-1\)\s*;/.test(tokenSource)
  ) {
    findings.push(
      `${tokenPath}: selection list inset must use the compact --app-space-1 geometry`,
    );
  }

  for (const tone of [
    "neutral",
    "accent",
    "busy",
    "success",
    "warning",
    "danger",
    "muted",
  ]) {
    if (!statusContractSource.includes(`[data-tone="${tone}"]`)) {
      findings.push(`${statusContractPath}: missing canonical status tone ${tone}`);
    }
  }

  for (const marker of [
    'data-app-status-badge="true"',
    "data-tone={tone}",
    "app-dream-status-badge__label",
  ]) {
    if (!statusBadgeSource.includes(marker)) {
      findings.push(`${statusBadgePath}: missing status badge contract ${marker}`);
    }
  }

  for (const topic of [
    "Primary page header",
    "Workspace page anatomy",
    "Search page",
    "Persistent pane geometry",
    "Account menu access control",
    "Selection lists",
    "Primary–Companion selection",
    "Status and health labels",
    "Navigation selection",
    "Feature CSS boundary",
    "Foreign provider document boundary",
    "dream/foundation.css",
  ]) {
    if (!appearanceGuideSource.includes(topic)) {
      findings.push(`${appearanceContractPath}: missing appearance guidance for ${topic}`);
    }
  }

  return findings;
}

export function auditPrimaryHeaderContract(layoutSource, sourceByPath) {
  const findings = [];
  for (const selector of [
    ".app-workspace-page {",
    ".app-workspace-page__topbar {",
    ".app-workspace-page__topbar-material {",
    ".app-workspace-page__topbar-drag-region {",
    ".app-workspace-page__content {",
    ".app-workspace-page__heading-title {",
    ".app-workspace-page__footer {",
    ".app-workspace-page__footer-material {",
    ".app-workspace-primary-header {",
    ".app-workspace-primary-header-action-group {",
    '.app-workspace-primary-header[data-window-controls="true"]',
    ".app-workspace-primary-header__actions {",
    ".app-workspace-primary-header__safe-area {",
    ".app-station-search-header {",
    ".app-station-search-content-search {",
  ]) {
    if (!layoutSource.includes(selector)) {
      findings.push(`${layoutContractPath}: missing canonical layout selector ${selector}`);
    }
  }

  for (const marker of [
    'data-page-header-layer="layered"',
    'data-page-header-state="scrolled"',
    'data-glass-role="header"',
  ]) {
    if (!layoutSource.includes(marker)) {
      findings.push(
        `${layoutContractPath}: missing scroll-aware Header contract ${marker}`,
      );
    }
  }

  for (const marker of [
    'data-page-footer-layer="layered"',
    'data-page-footer-state="content"',
    'data-glass-role="footer"',
  ]) {
    if (!layoutSource.includes(marker)) {
      findings.push(
        `${layoutContractPath}: missing content-aware Footer contract ${marker}`,
      );
    }
  }

  if (
    /\.app-workspace-page__footer\s*\{[^}]*border-block-start\s*:/s.test(
      layoutSource,
    ) ||
    !/\.app-workspace-page__footer\s*\{[^}]*border\s*:\s*0\s*;/s.test(
      layoutSource,
    )
  ) {
    findings.push(
      `${layoutContractPath}: page footers must use spacing without a content divider`,
    );
  }

  for (const relative of primaryHeaderConsumerPaths) {
    const source = sourceByPath.get(relative) ?? "";
    const usesWorkspacePage = source.includes("<WorkspacePage");

    if (usesWorkspacePage) {
      if (!source.includes("defineWorkspacePageContract")) {
        findings.push(
          `${relative}: WorkspacePage consumers must define an explicit page contract`,
        );
      }
      if (
        !source.includes("<WorkspacePageContent") &&
        !source.includes('heading: "host-owned"')
      ) {
        findings.push(
          `${relative}: WorkspacePage must render shared content or declare a host-owned heading`,
        );
      }
      if (
        !source.includes("<WorkspacePageTopBar") &&
        !source.includes('topBar: "host-owned"') &&
        !source.includes('topBar: "none"')
      ) {
        findings.push(
          `${relative}: WorkspacePage must render the shared TopBar or declare host-owned chrome`,
        );
      }
      continue;
    }

    // Transitional special pages may retain the legacy markup until their
    // bespoke host is migrated. New and migrated Station pages use the
    // WorkspacePage primitives above, which own their sole h1 internally.
    if (!source.includes("app-workspace-primary-header")) {
      findings.push(`${relative}: Primary page must consume app-workspace-primary-header`);
    }
    if (!source.includes('className="sr-only"')) {
      findings.push(`${relative}: Primary route name must remain available as an sr-only heading`);
    }
  }

  return findings;
}

export function auditSurfaceContractSource(content) {
  const findings = [];
  const rolesStart = content.indexOf("export const XIA_SURFACE_ROLES");
  const rolesEnd =
    rolesStart < 0 ? -1 : content.indexOf("as const", rolesStart);
  const rolesSource =
    rolesStart < 0 || rolesEnd < 0
      ? ""
      : content.slice(rolesStart, rolesEnd + "as const".length);
  const typeStart = content.indexOf("export type XiaSurfaceRole");
  const typeEnd = typeStart < 0 ? -1 : content.indexOf(";", typeStart);
  const typeSource =
    typeStart < 0 || typeEnd < 0 ? "" : content.slice(typeStart, typeEnd + 1);

  if (!typeSource) {
    findings.push(`${surfaceContractPath}: missing exported XiaSurfaceRole type`);
  } else if (
    rolesSource &&
    !/typeof\s+XIA_SURFACE_ROLES/.test(typeSource)
  ) {
    findings.push(
      `${surfaceContractPath}: XiaSurfaceRole must derive from XIA_SURFACE_ROLES`,
    );
  }

  if (!rolesSource) {
    findings.push(
      `${surfaceContractPath}: missing exported XIA_SURFACE_ROLES vocabulary`,
    );
  }

  const presetStart = content.indexOf("export const XIA_SURFACE_ROLE_PRESETS");
  const presetEnd =
    presetStart < 0
      ? -1
      : content.indexOf("export type XiaSurfaceAttributes", presetStart);
  const presetSource =
    presetStart < 0
      ? ""
      : content.slice(
          presetStart,
          presetEnd < 0 ? content.length : presetEnd,
        );

  if (!presetSource) {
    findings.push(
      `${surfaceContractPath}: missing canonical XIA_SURFACE_ROLE_PRESETS map`,
    );
  }

  for (const role of canonicalSurfaceRoles) {
    if (!(rolesSource || typeSource).includes(`"${role}"`)) {
      findings.push(`${surfaceContractPath}: XIA_SURFACE_ROLES is missing ${role}`);
    }
    if (!new RegExp(`(?:^|\\n)\\s*${role}\\s*:`).test(presetSource)) {
      findings.push(
        `${surfaceContractPath}: XIA_SURFACE_ROLE_PRESETS is missing ${role}`,
      );
    }
  }

  for (const [role, material] of [
    ["status", "regular"],
    ["overlay", "panel"],
  ]) {
    if (
      !new RegExp(
        `\\b${role}\\s*:\\s*\\{[^}]*\\bmaterial\\s*:\\s*["']${material}["']`,
        "s",
      ).test(presetSource)
    ) {
      findings.push(
        `${surfaceContractPath}: ${role} must resolve through the canonical ${material} preset`,
      );
    }
  }

  const helperStart = content.indexOf("export function getXiaSurfaceAttributes");
  const helperSource = helperStart < 0 ? "" : content.slice(helperStart);
  if (
    !helperSource.includes('"data-surface-role"') ||
    !helperSource.includes('"data-material"')
  ) {
    findings.push(
      `${surfaceContractPath}: getXiaSurfaceAttributes must emit role and resolved material attributes`,
    );
  }

  return findings;
}

export function auditContrastCanvasContract(content, tokenSource = "") {
  const findings = [];
  const rules = cssRules(content);
  const consumers = [
    [
      "Sidebar",
      '.app-main-shell[data-surface-style="contrast"] .app-workspace-sidebar',
      "--app-surface-canvas",
    ],
    [
      "Primary",
      '.app-main-shell[data-surface-style="contrast"] .app-workspace-primary-pane',
      "--app-workspace-primary-surface",
    ],
    [
      "Docked Companion",
      '.app-main-shell[data-surface-style="contrast"] .app-workspace-companion[data-presentation="docked"]',
      "--app-surface-canvas",
    ],
  ];

  for (const [label, selector, surfaceToken] of consumers) {
    const matchingRule = rules.find((rule) => rule.selector.includes(selector));
    if (
      !matchingRule ||
      !new RegExp(
        `\\bbackground\\s*:\\s*var\\(${surfaceToken.replaceAll("-", "\\-")}\\)\\s*;?`,
      ).test(matchingRule.body)
    ) {
      findings.push(
        `${workspaceSurfacePath}: Contrast ${label} must consume background: var(${surfaceToken})`,
      );
    }
  }

  if (
    tokenSource &&
    !/--app-workspace-primary-surface\s*:\s*var\(--app-surface-canvas\)\s*;/.test(
      tokenSource,
    )
  ) {
    findings.push(
      `${tokenPath}: Contrast Primary surface must alias --app-surface-canvas`,
    );
  }

  return findings;
}

export function auditWorkspacePaneAppearanceContract(
  tokenSource,
  layoutSource,
  workspaceSource,
  navigationSource,
  mainAppSource,
  appearanceGuideSource,
) {
  const findings = [];

  const requiredTokenRecipes = [
    [
      "--app-workspace-divider-color",
      /--app-workspace-divider-color\s*:\s*var\(--app-surface-separator\)\s*;/,
    ],
    [
      "--app-workspace-primary-glass-surface",
      /--app-workspace-primary-glass-surface\s*:[\s\S]*?var\(--app-workspace-primary-glass-opacity\)[\s\S]*?;/,
    ],
    [
      "--app-workspace-primary-surface",
      /--app-workspace-primary-surface\s*:\s*var\(--app-surface-canvas\)\s*;/,
    ],
    [
      "--app-workspace-primary-subpane-surface",
      /--app-workspace-primary-subpane-surface\s*:\s*transparent\s*;/,
    ],
    [
      "--app-menu-action-height",
      /--app-menu-action-height\s*:\s*2\.25rem\s*;/,
    ],
    [
      "--app-workspace-search-height",
      /--app-workspace-search-height\s*:\s*3rem\s*;/,
    ],
  ];
  for (const [token, pattern] of requiredTokenRecipes) {
    if (!pattern.test(tokenSource)) {
      findings.push(`${tokenPath}: invalid workspace appearance recipe ${token}`);
    }
  }

  if (
    !/:root\[data-xiadown-surface-style="glass"\],[\s\S]*?:where\(\[data-surface-style="glass"\]\)\s*\{[^}]*--app-workspace-primary-surface\s*:[^}]*var\(--app-workspace-primary-glass-surface\)/s.test(
      tokenSource,
    )
  ) {
    findings.push(
      `${tokenPath}: Glass must map the one Primary host surface to --app-workspace-primary-glass-surface`,
    );
  }
  if (
    !/\.app-workspace-primary-pane\s*\{[^}]*background\s*:\s*var\(--app-workspace-primary-surface\)/s.test(
      workspaceSource,
    )
  ) {
    findings.push(
      `${workspaceSurfacePath}: Primary host must be the only owner of --app-workspace-primary-surface`,
    );
  }
  if (
    !/\.app-main-shell\[data-surface-style="glass"\]\s*\.app-workspace-primary-pane\s*\{[^}]*box-shadow\s*:\s*none[^}]*backdrop-filter\s*:\s*none/s.test(
      workspaceSource,
    )
  ) {
    findings.push(
      `${workspaceSurfacePath}: Glass Primary host must not add a second shadow or backdrop pass`,
    );
  }
  if (
    !/\.app-workspace-primary-subpane--leading\s*\{[^}]*border-inline-end:[^}]*var\(--app-workspace-divider-width\) solid[^}]*var\(--app-workspace-divider-color\)/s.test(
      layoutSource,
    )
  ) {
    findings.push(
      `${layoutContractPath}: leading Primary subpanes must own exactly the shared workspace divider`,
    );
  }
  if (
    !/\.app-workspace-primary-subpane\s*\{[^}]*background:\s*var\(--app-workspace-primary-subpane-surface\)[^}]*box-shadow:\s*none[^}]*backdrop-filter:\s*none/s.test(
      layoutSource,
    )
  ) {
    findings.push(
      `${layoutContractPath}: Primary subpanes must stay transparent geometry-only regions`,
    );
  }

  const activeNavigationRule = cssRules(navigationSource).find(
    (rule) =>
      rule.selector.includes('.app-workspace-nav-button[data-active="true"]') &&
      /\bcolor\s*:\s*hsl\([\s\S]*?var\(--app-accent-on-solid/.test(rule.body) &&
      /\bbackground\s*:\s*hsl\(var\(--app-accent-solid/.test(rule.body),
  );
  if (!activeNavigationRule) {
    findings.push(
      `${workspaceNavigationPath}: selected navigation must use one accent-solid surface and accent-on-solid foreground`,
    );
  }
  if (
    !/\.app-workspace-nav-button\[data-active="true"\][\s\S]*?\.app-workspace-nav-button__icon\s*\{[^}]*color\s*:\s*inherit/s.test(
      navigationSource,
    ) ||
    !/\.app-workspace-nav-button__icon\s*\{[^}]*background\s*:\s*transparent/s.test(
      navigationSource,
    )
  ) {
    findings.push(
      `${workspaceNavigationPath}: selected navigation icon must inherit the row foreground without a nested tile`,
    );
  }

  if (
    !/app-workspace-account-menu__access-name[\s\S]{0,160}\{libraryAccessCopy\.remote\}/.test(
      mainAppSource,
    ) ||
    !/<DropdownMenuCheckboxItem[\s\S]{0,600}checked=\{libraryAccessDisplayRemote\}[\s\S]{0,900}onSelect=\{\(event\) => event\.preventDefault\(\)\}/.test(
      mainAppSource,
    ) ||
    !/<DreamInlineSwitchVisual[\s\S]{0,160}checked=\{libraryAccessDisplayRemote\}[\s\S]{0,160}className="app-workspace-account-menu__access-switch"/.test(
      mainAppSource,
    )
  ) {
    findings.push(
      `${mainAppPath}: Access must render Remote as a name/value row with the canonical inline switch`,
    );
  }
  if (
    mainAppSource.includes("app-workspace-account-menu__access-option") ||
    /const libraryAccessControlRow[\s\S]{0,1800}<DropdownMenuRadio(?:Group|Item)/.test(
      mainAppSource,
    )
  ) {
    findings.push(
      `${mainAppPath}: Access must not restore the Local/Remote segmented control`,
    );
  }
  for (const pattern of [
    /\.app-workspace-account-menu__access-row\.app-menu-item\s*\{(?=[^}]*\n\s*display\s*:\s*flex)(?=[^}]*\n\s*height\s*:\s*var\(--app-menu-action-height\))(?=[^}]*\n\s*min-height\s*:\s*var\(--app-menu-action-height\))(?=[^}]*\n\s*align-items\s*:\s*center)(?=[^}]*\n\s*justify-content\s*:\s*space-between)[^}]*\}/s,
    /\.app-workspace-account-menu__access-switch\.app-dream-inline-switch\s*\{[^}]*flex\s*:\s*0 0 auto/s,
    /\.app-workspace-account-menu__quick-action\.app-menu-item\s*\{(?=[^}]*\n\s*height\s*:\s*var\(--app-menu-action-height\))(?=[^}]*\n\s*min-height\s*:\s*var\(--app-menu-action-height\))[^}]*\}/s,
  ]) {
    if (!pattern.test(workspaceSource)) {
      findings.push(
        `${workspaceSurfacePath}: Access name/value row and account actions must share menu geometry`,
      );
      break;
    }
  }

  for (const marker of [
    "app-workspace-primary-subpane",
    "one edge owner",
    "accent-on-solid",
    "DreamInlineSwitchVisual",
  ]) {
    if (!appearanceGuideSource.includes(marker)) {
      findings.push(
        `${appearanceContractPath}: missing governed workspace appearance marker ${marker}`,
      );
    }
  }

  return findings;
}

export function auditWorkspaceStructuralOverrides(cssByPath) {
  const findings = [];
  const allowedBackground = (value) =>
    value.includes("var(--app-workspace-primary-subpane-surface") ||
    value.includes("var(--app-library-primary-surface") ||
    /^(?:transparent|none)(?:\s*!important)?$/.test(value.trim());
  const zeroEdge = (value) =>
    /^(?:0(?:px|rem|em)?|none|transparent)(?:\s*!important)?$/.test(
      value.trim(),
    );

  for (const [relative, source] of cssByPath) {
    for (const rule of cssRules(source)) {
      const targets = workspaceStructuralRootClasses.filter((className) =>
        selectorTargetsClass(rule.selector, className),
      );
      if (targets.length === 0) {
        continue;
      }

      if (
        relative !== layoutContractPath &&
        relative !== workspaceSurfacePath &&
        relative !== workspaceAppearancePath &&
        targets.some((className) =>
          className.startsWith("app-workspace-primary-subpane"),
        )
      ) {
        findings.push(
          `${relative}: canonical Primary subpane classes may only be styled by the shared layout/workspace contracts`,
        );
      }

      for (const match of rule.body.matchAll(
        /(?:^|;)\s*background(?:-color)?\s*:\s*([^;]+)/g,
      )) {
        if (!allowedBackground(match[1] ?? "")) {
          findings.push(
            `${relative}: ${targets.join("/")} must not repaint the Primary host background`,
          );
        }
      }

      if (/(?:^|;)\s*opacity\s*:/m.test(rule.body)) {
        findings.push(
          `${relative}: ${targets.join("/")} must not define local Primary opacity`,
        );
      }

      const canonicalLeadingDivider =
        relative === layoutContractPath &&
        targets.includes("app-workspace-primary-subpane--leading");
      for (const match of rule.body.matchAll(
        /(?:^|;)\s*(?:border(?:-(?:inline-end|right|left|width))?)\s*:\s*([^;]+)/g,
      )) {
        if (!zeroEdge(match[1] ?? "") && !canonicalLeadingDivider) {
          findings.push(
            `${relative}: ${targets.join("/")} must not own an extra Primary divider`,
          );
        }
      }

      for (const match of rule.body.matchAll(
        /(?:^|;)\s*(?:-webkit-)?backdrop-filter\s*:\s*([^;]+)|(?:^|;)\s*box-shadow\s*:\s*([^;]+)/g,
      )) {
        const value = (match[1] ?? match[2] ?? "").trim();
        if (!/^(?:none|0(?:\s+0)?(?:\s+transparent)?)$/.test(value)) {
          findings.push(
            `${relative}: ${targets.join("/")} must remain a geometry-only Primary region`,
          );
        }
      }
    }
  }

  const navigationSource = [
    cssByPath.get(workspaceNavigationPath) ?? "",
    cssByPath.get(workspaceAppearancePath) ?? "",
  ].join("\n");
  for (const rule of cssRules(navigationSource)) {
    if (
      !/\.app-workspace-nav-button\[data-active="true"\]\s+/.test(
        rule.selector,
      )
    ) {
      continue;
    }
    const descendantBackground = [
      ...rule.body.matchAll(
        /(?:^|;)\s*background(?:-color)?\s*:\s*([^;]+)/g,
      ),
    ].some((match) => !/^(?:transparent|none)$/.test((match[1] ?? "").trim()));
    const descendantShadow = [
      ...rule.body.matchAll(/(?:^|;)\s*box-shadow\s*:\s*([^;]+)/g),
    ].some((match) => !/^none$/.test((match[1] ?? "").trim()));
    if (descendantBackground || descendantShadow) {
      findings.push(
        `${workspaceNavigationPath}: selected navigation descendants must not add another active surface`,
      );
    }
  }

  return [...new Set(findings)];
}

/**
 * The shared WorkspacePage grid owns root sizing, display, and overflow.
 * Feature roots may publish containers, tokens, and paint, but must compose
 * domain content inside the shared content slot instead of replacing the page
 * layout through a more specific selector.
 */
export function auditWorkspacePageLayoutOwnership(cssByPath) {
  const findings = [];
  const ownedProperty =
    /(?:^|;)\s*(display|grid(?:-template(?:-(?:areas|columns|rows))?|-auto-(?:columns|flow|rows))?|flex(?:-direction|-flow)?|overflow(?:-[xy])?|(?:min-|max-)?(?:width|height))\s*:\s*([^;]+)/g;

  for (const [relative, rootClasses] of workspacePageLayoutRootsByPath) {
    const source = cssByPath.get(relative) ?? "";
    for (const rule of cssRules(source)) {
      const rootClass = rootClasses.find((className) =>
        selectorTargetsClass(rule.selector, className),
      );
      if (!rootClass) {
        continue;
      }

      for (const match of rule.body.matchAll(ownedProperty)) {
        const property = match[1] ?? "";
        const value = (match[2] ?? "").trim();
        const isInactiveVisibilityRule =
          property === "display" &&
          value === "none" &&
          rule.selector.includes('[data-active="false"]');
        if (isInactiveVisibilityRule) {
          continue;
        }
        findings.push(
          `${relative}: .${rootClass} must not override WorkspacePage-owned ${property}`,
        );
      }
    }
  }

  return [...new Set(findings)];
}

export function auditSharedWindowCanvasContract(tokenSource, settingsSource) {
  const findings = [];
  if (
    !/--app-surface-window-canvas\s*:[\s\S]*?var\(--dream-shell-top\)[\s\S]*?var\(--dream-shell-mid\)[\s\S]*?var\(--dream-shell-bottom\)[\s\S]*?;/.test(
      tokenSource,
    )
  ) {
    findings.push(
      `${tokenPath}: --app-surface-window-canvas must preserve the theme-tinted Settings canvas recipe`,
    );
  }
  if (
    !/--app-main-shell-aba-surface\s*:\s*var\(--app-surface-window-canvas\)\s*;/.test(
      tokenSource,
    )
  ) {
    findings.push(
      `${tokenPath}: legacy ABA surface must alias --app-surface-window-canvas instead of duplicating the recipe`,
    );
  }

  const contrastCanvasRule = cssRules(tokenSource).find(
    (rule) =>
      rule.selector.includes('[data-surface-style="contrast"]') &&
      /--app-surface-canvas\s*:\s*var\(--app-surface-window-canvas\)\s*;?/.test(
        rule.body,
      ),
  );
  if (!contrastCanvasRule) {
    findings.push(
      `${tokenPath}: Contrast must alias --app-surface-canvas to the Settings window canvas recipe`,
    );
  }

  const settingsCanvasRule = cssRules(settingsSource).find(
    (rule) =>
      rule.selector.includes(
        '.app-settings-window.app-dream-window[data-surface-style="contrast"]',
      ) &&
      /\bbackground\s*:\s*var\(--app-surface-canvas\)\s*;?/.test(rule.body),
  );
  if (!settingsCanvasRule) {
    findings.push(
      `${settingsShellPath}: Settings Contrast must consume background: var(--app-surface-canvas)`,
    );
  }

  return findings;
}

export function auditOverlayRoleConsumers(sourceByPath) {
  const findings = [];
  for (const relative of overlayRoleConsumers) {
    const source = sourceByPath.get(relative) ?? "";
    if (!consumesSurfaceRole(source, "overlay")) {
      findings.push(`${relative}: must consume the canonical overlay surface role`);
    }
  }
  return findings;
}

export function auditWorkspaceStatusRole(content) {
  const findings = [];
  const statusBoundary = functionSource(content, "WorkspaceStatusSurface");
  if (!statusBoundary) {
    findings.push(
      `${workspaceActivityPath}: missing the shared WorkspaceStatusSurface boundary`,
    );
  } else if (!consumesSurfaceRole(statusBoundary, "status")) {
    findings.push(
      `${workspaceActivityPath}: WorkspaceStatusSurface must consume the canonical status role`,
    );
  }

  for (const functionName of [
    "WideSniffActivity",
    "SniffWorkspaceSessionActivity",
    "WidePlaybackActivity",
    "WideOperationActivity",
  ]) {
    const source = functionSource(content, functionName);
    if (!source || !/<WorkspaceStatusSurface\b/.test(source)) {
      findings.push(
        `${workspaceActivityPath}: ${functionName} must render through WorkspaceStatusSurface`,
      );
    }
  }

  return findings;
}

export function auditWorkspaceContentControlContracts(sourceByPath, controlsSource) {
  const findings = [];
  const canonicalSearch = sourceByPath.get(workspaceSearchControlPath) ?? "";
  const requiredSearchRoles = [
    "app-dream-workspace-search",
    "app-dream-search-control",
    "app-dream-control-shell",
    "app-station-search-content-search",
  ];

  if (!canonicalSearch.includes("export const WorkspaceSearchControl")) {
    findings.push(`${workspaceSearchControlPath}: missing canonical Search-page component`);
  }
  for (const role of requiredSearchRoles) {
    if (!canonicalSearch.includes(role)) {
      findings.push(`${workspaceSearchControlPath}: missing canonical Dream role ${role}`);
    }
  }
  for (const part of ["__icon", "__input", "__clear", "__submit"]) {
    if (!canonicalSearch.includes(`app-dream-workspace-search${part}`)) {
      findings.push(`${workspaceSearchControlPath}: missing canonical Search part ${part}`);
    }
  }

  for (const relative of workspaceSearchConsumerPaths) {
    const source = sourceByPath.get(relative) ?? "";
    if (!/<WorkspaceSearchControl\b/.test(source)) {
      findings.push(`${relative}: Search route must render WorkspaceSearchControl`);
    }
  }

  const rssWorkspace = sourceByPath.get(rssWorkspacePath) ?? "";
  if (!rssWorkspace.includes("rss-entry-list app-dream-selection-list")) {
    findings.push(`${rssWorkspacePath}: RSS split list must consume app-dream-selection-list`);
  }
  if (!rssWorkspace.includes("rss-entry-row app-dream-selection-item")) {
    findings.push(`${rssWorkspacePath}: RSS selectable rows must consume app-dream-selection-item`);
  }

  const selectionRule = cssRules(controlsSource).find((rule) =>
    rule.selector.includes(".app-dream-selection-list"),
  );
  if (
    !selectionRule ||
    !/padding-inline\s*:\s*var\(--app-selection-list-inset\)/.test(selectionRule.body) ||
    !/scroll-padding-inline\s*:\s*var\(--app-selection-list-inset\)/.test(selectionRule.body) ||
    !/scrollbar-gutter\s*:\s*stable\s*;/.test(selectionRule.body)
  ) {
    findings.push(
      `${dreamControlsPath}: selection list must own compact inline inset and one stable scrollbar gutter`,
    );
  }

  return findings;
}

export function auditRoleOwnedFeatureSource(content, relative) {
  const findings = [];
  const directMaterial = /\b(?:data-material\s*=|material\s*(?:=|:))/g;
  for (const match of content.matchAll(directMaterial)) {
    findings.push(
      `${relative}:${lineNumber(content, match.index ?? 0)}: feature surfaces must declare a canonical role, not choose a material`,
    );
  }

  const localBlur = /(?:-webkit-)?backdrop-filter\b|backdrop-blur|\bblur\s*\(/g;
  for (const match of content.matchAll(localBlur)) {
    findings.push(
      `${relative}:${lineNumber(content, match.index ?? 0)}: feature surfaces must not define blur; the role preset owns its recipe`,
    );
  }

  return findings;
}

export function auditFeatureMaterialFilters(content, relative) {
  if (relative.startsWith("shared/styles/dream/")) {
    return [];
  }

  const findings = [];
  const materialFilter = /(?:-webkit-)?backdrop-filter\s*:\s*([^;{}]+)/g;
  for (const match of content.matchAll(materialFilter)) {
    const value = (match[1] ?? "").trim();
    // `none` is a composition-level opt-out used to prevent nested sampling;
    // every positive filter/material recipe belongs to Dream CSS.
    if (value !== "none") {
      findings.push(
        `${relative}:${lineNumber(content, match.index ?? 0)}: feature CSS must not choose a backdrop material filter; move the recipe to Dream CSS or consume a surface role`,
      );
    }
  }
  return findings;
}

function declarationBelongsToKeyframes(declaration) {
  for (let parent = declaration.parent; parent; parent = parent.parent) {
    if (parent.type === "atrule" && /keyframes$/i.test(parent.name)) {
      return true;
    }
  }
  return false;
}

/** Feature stylesheets own composition only. Product appearance declarations
 * belong to Dream so the Appearance Lab can inventory and render them. */
export function auditFeatureCssAppearanceBoundary(content, relative) {
  if (
    relative.startsWith("shared/styles/dream/") ||
    relative === "shared/styles/dream.css" ||
    featureCssAppearanceExemptPaths.has(relative)
  ) {
    return [];
  }

  const findings = [];
  const root = postcss.parse(content, { from: relative });
  root.walkDecls((declaration) => {
    if (declarationBelongsToKeyframes(declaration)) {
      return;
    }

    const line = declaration.source?.start?.line ?? 0;
    if (featureCssAppearanceProperty.test(declaration.prop)) {
      findings.push(
        `${relative}:${line}: feature CSS appearance property ${declaration.prop} must move to Dream CSS`,
      );
    }
    if (featureCssVisualCustomProperty.test(declaration.prop)) {
      findings.push(
        `${relative}:${line}: feature CSS visual custom property ${declaration.prop} must be defined in Dream CSS`,
      );
    }
  });
  return findings;
}

/** index.css is an assembly boundary only. Product declarations belong to a
 * Dream module so Appearance can inventory them. */
export function auditStyleEntrypointBoundary(content, relative = "index.css") {
  const findings = [];
  let root;
  try {
    root = postcss.parse(content, { from: relative });
  } catch (error) {
    return [
      `${relative}: failed to parse stylesheet entrypoint (${error instanceof Error ? error.message : String(error)})`,
    ];
  }
  root.walkRules((rule) => {
    findings.push(
      `${relative}:${rule.source?.start?.line ?? 1}: stylesheet entrypoint must not define CSS rules; move them to Dream CSS`,
    );
  });
  root.walkDecls((declaration) => {
    findings.push(
      `${relative}:${declaration.source?.start?.line ?? 1}: stylesheet entrypoint must not define ${declaration.prop}; move it to Dream CSS`,
    );
  });
  root.walkAtRules((atRule) => {
    if (atRule.name !== "import" && atRule.name !== "tailwind") {
      findings.push(
        `${relative}:${atRule.source?.start?.line ?? 1}: stylesheet entrypoint allows only @import and @tailwind, not @${atRule.name}`,
      );
    }
  });
  if (!content.includes('@import "./shared/styles/dream.css"')) {
    findings.push(`${relative}: stylesheet entrypoint must import Dream CSS`);
  }
  return findings;
}

/** Tailwind is a composition utility compiler in XiaDown; product appearance
 * extensions and visual plugins would create a second theme authority. */
export function auditTailwindCompositionConfigSource(
  content,
  relative = "tailwind.config.js",
) {
  const findings = [];
  if (!/\btheme\s*:\s*\{\s*\}/s.test(content)) {
    findings.push(
      `${relative}: Tailwind theme must stay empty; visual values belong to Dream CSS`,
    );
  }
  if (!/\bplugins\s*:\s*\[\s*\]/s.test(content)) {
    findings.push(
      `${relative}: Tailwind plugins must stay empty unless the Dream contract explicitly adopts them`,
    );
  }
  if (/\bnode_modules\b/.test(content)) {
    findings.push(
      `${relative}: Tailwind content discovery must stay scoped to XiaDown source`,
    );
  }
  if (/^\s*import\s/m.test(content) || /\brequire\s*\(/.test(content)) {
    findings.push(
      `${relative}: Tailwind config must not import a second visual style system`,
    );
  }
  return findings;
}

/** Provider WebViews may isolate remote media, but they must never project
 * XiaDown or Dream appearance into provider-owned documents. */
export function auditForeignProviderAppearanceBoundary(sourceByPath) {
  const findings = [];
  const forbidden = [
    "--app-",
    ".app-",
    "app-dream",
    "dream-",
    "hsl(var(",
    "var(--",
  ];
  for (const relative of foreignProviderBridgePaths) {
    const source = sourceByPath.get(relative) ?? "";
    if (!source) {
      findings.push(`${relative}: missing foreign-provider bridge source`);
      continue;
    }
    for (const marker of forbidden) {
      if (source.includes(marker)) {
        findings.push(
          `${relative}: foreign-provider transport CSS must not contain Dream appearance marker ${JSON.stringify(marker)}`,
        );
      }
    }
  }
  return findings;
}

export function auditInlineBackdropBlurSource(content, relative) {
  if (/(?:^|\.)test\.(?:ts|tsx)$/.test(relative)) {
    return [];
  }

  const findings = [];
  const staticInlineBackdropBlur =
    /\b(?:WebkitBackdropFilter|backdropFilter)\s*:\s*(?:"[^"]*\bblur\s*\(|'[^']*\bblur\s*\(|`[^`]*\bblur\s*\()/g;
  for (const match of content.matchAll(staticInlineBackdropBlur)) {
    findings.push(
      `${relative}:${lineNumber(content, match.index ?? 0)}: static inline backdrop blur recipes must be defined in Dream CSS`,
    );
  }

  return findings;
}

function unwrapStyleExpression(node) {
  let current = node;
  while (
    ts.isParenthesizedExpression(current) ||
    ts.isAsExpression(current) ||
    ts.isSatisfiesExpression(current) ||
    ts.isNonNullExpression(current)
  ) {
    current = current.expression;
  }
  return current;
}

function isStaticStyleValue(node) {
  const value = unwrapStyleExpression(node);
  if (
    ts.isStringLiteral(value) ||
    ts.isNoSubstitutionTemplateLiteral(value) ||
    ts.isNumericLiteral(value) ||
    value.kind === ts.SyntaxKind.TrueKeyword ||
    value.kind === ts.SyntaxKind.FalseKeyword ||
    (ts.isPrefixUnaryExpression(value) &&
      ts.isNumericLiteral(value.operand))
  ) {
    return true;
  }
  return (
    ts.isConditionalExpression(value) &&
    isStaticStyleValue(value.whenTrue) &&
    isStaticStyleValue(value.whenFalse)
  );
}

function appendsHexAlphaInStyleValue(node) {
  const value = unwrapStyleExpression(node);
  if (!ts.isTemplateExpression(value) || value.templateSpans.length === 0) {
    return false;
  }
  const tail = value.templateSpans.at(-1)?.literal.text ?? "";
  return /^[0-9a-f]{2}$/i.test(tail);
}

function styleObjectContext(node, sourceFile) {
  let current = node;
  while (current.parent) {
    const parent = current.parent;
    if (
      ts.isJsxExpression(parent) &&
      ts.isJsxAttribute(parent.parent) &&
      parent.parent.name.text === "style"
    ) {
      return "inline style";
    }
    if (ts.isVariableDeclaration(parent)) {
      const name = parent.name.getText(sourceFile);
      const type = parent.type?.getText(sourceFile) ?? "";
      if (/style/i.test(name) || /CSSProperties/.test(type)) {
        return `style object ${name}`;
      }
      return "";
    }
    if (
      (ts.isAsExpression(parent) || ts.isSatisfiesExpression(parent)) &&
      /CSSProperties/.test(parent.type.getText(sourceFile))
    ) {
      return "CSSProperties object";
    }
    if (ts.isFunctionDeclaration(parent) || ts.isFunctionExpression(parent)) {
      const name = parent.name?.getText(sourceFile) ?? "";
      return /style/i.test(name) ? `style helper ${name}` : "";
    }
    if (ts.isArrowFunction(parent)) {
      const owner = parent.parent;
      if (ts.isVariableDeclaration(owner)) {
        const name = owner.name.getText(sourceFile);
        return /style/i.test(name) ? `style helper ${name}` : "";
      }
    }
    current = parent;
  }
  return "";
}

/** Static values in React style objects are hidden appearance definitions.
 * Data-driven values may remain inline; constants belong to semantic classes
 * whose recipes are discoverable through Dream CSS and Appearance. */
export function auditInlineStaticStyleSource(content, relative) {
  if (/(?:^|\.)test\.(?:ts|tsx)$/.test(relative)) {
    return [];
  }

  const sourceFile = ts.createSourceFile(
    relative,
    content,
    ts.ScriptTarget.Latest,
    true,
    relative.endsWith(".tsx") ? ts.ScriptKind.TSX : ts.ScriptKind.TS,
  );
  const findings = [];
  const audited = new Set();

  function visit(node) {
    if (ts.isObjectLiteralExpression(node)) {
      const context = styleObjectContext(node, sourceFile);
      if (context && !audited.has(node.pos)) {
        audited.add(node.pos);
        for (const property of node.properties) {
          if (
            !ts.isPropertyAssignment(property) ||
            (!isStaticStyleValue(property.initializer) &&
              !appendsHexAlphaInStyleValue(property.initializer))
          ) {
            continue;
          }
          const propertyName = property.name.getText(sourceFile);
          const { line } = sourceFile.getLineAndCharacterOfPosition(
            property.getStart(sourceFile),
          );
          const reason = appendsHexAlphaInStyleValue(property.initializer)
            ? `encodes alpha in dynamic ${propertyName}`
            : `contains static ${propertyName}`;
          findings.push(
            `${relative}:${line + 1}: ${context} ${reason}; move the recipe to a Dream CSS semantic class`,
          );
        }
      }
    }
    ts.forEachChild(node, visit);
  }
  visit(sourceFile);
  return findings;
}

function unwrapClassExpression(node) {
  let current = node;
  while (
    ts.isParenthesizedExpression(current) ||
    ts.isAsExpression(current) ||
    ts.isSatisfiesExpression(current) ||
    ts.isNonNullExpression(current)
  ) {
    current = current.expression;
  }
  return current;
}

function classContextName(node, sourceFile) {
  if (!node) {
    return "";
  }
  const name = ts.isIdentifier(node) || ts.isStringLiteralLike(node)
    ? node.text
    : node.getText(sourceFile).replace(/^['"]|['"]$/g, "");
  return name;
}

function isClassContextName(name) {
  return (
    /(?:class|classes|className|classNames)$/i.test(name) ||
    /(?:^|_)(?:CLASS|CLASSES|CLASS_NAME|CLASS_NAMES)(?:_|$)/.test(name)
  );
}

function stripTailwindVariants(token) {
  let squareDepth = 0;
  let roundDepth = 0;
  let lastVariantSeparator = -1;
  for (let index = 0; index < token.length; index += 1) {
    const character = token[index];
    if (character === "[") squareDepth += 1;
    else if (character === "]") squareDepth = Math.max(0, squareDepth - 1);
    else if (character === "(") roundDepth += 1;
    else if (character === ")") roundDepth = Math.max(0, roundDepth - 1);
    else if (character === ":" && squareDepth === 0 && roundDepth === 0) {
      lastVariantSeparator = index;
    }
  }

  let utility = token.slice(lastVariantSeparator + 1);
  while (utility.startsWith("!")) utility = utility.slice(1);
  if (utility.startsWith("-")) utility = utility.slice(1);
  return utility;
}

const tailwindTextAppearanceUtility = new RegExp(
  "^text-(?:" +
    [
      "\\[",
      "\\(",
      "(?:left|center|right|justify|start|end)(?:$|/)",
      "(?:2xs|xs|sm|base|lg|xl|[2-9]xl)(?:$|/)",
      "(?:inherit|current|transparent|black|white)(?:$|[-/])",
      "(?:slate|gray|zinc|neutral|stone|red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose)(?:$|[-/])",
      "(?:border|input|ring|background|foreground|primary|secondary|destructive|muted|accent|popover|card|sidebar|light|dark)(?:$|[-/])",
      "\\$\\{",
      "__DYNAMIC__",
    ].join("|") +
    ")",
);

const tailwindAppearanceUtility = new RegExp(
  "^(?:" +
    [
      "bg(?:-|$)",
      "font(?:-|$)",
      "leading(?:-|$)",
      "tracking(?:-|$)",
      "rounded(?:-|$)",
      "border(?:-|$)",
      "divide(?:-|$)",
      "shadow(?:-|$)",
      "ring(?:-|$)",
      "outline(?:-|$)",
      "opacity(?:-|$)",
      "cursor(?:-|$)",
      "transition(?:-|$)",
      "duration(?:-|$)",
      "ease(?:-|$)",
      "delay(?:-|$)",
      "animate(?:-|$)",
      "filter(?:-none)?$",
      "backdrop(?:-|$)",
      "blur(?:-|$)",
      "brightness(?:-|$)",
      "contrast(?:-|$)",
      "drop-shadow(?:-|$)",
      "grayscale(?:-|$)",
      "hue-rotate(?:-|$)",
      "invert(?:-|$)",
      "saturate(?:-|$)",
      "sepia(?:-|$)",
      "fill-(?:none|current|inherit|transparent|black|white|\\[|\\(|slate|gray|zinc|neutral|stone|red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose|primary|secondary|destructive|muted|accent|foreground|background)(?:$|[-/])?",
      "stroke-(?:none|current|inherit|transparent|black|white|\\[|\\(|[0-9]|slate|gray|zinc|neutral|stone|red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose|primary|secondary|destructive|muted|accent|foreground|background)(?:$|[-/])?",
      "accent-(?:auto|inherit|current|transparent|black|white|\\[|\\(|slate|gray|zinc|neutral|stone|red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose|primary|secondary|destructive|muted|accent|foreground|background)(?:$|[-/])?",
      "caret-(?:inherit|current|transparent|black|white|\\[|\\(|slate|gray|zinc|neutral|stone|red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose|primary|secondary|destructive|muted|accent|foreground|background)(?:$|[-/])?",
      "decoration(?:-|$)",
      "placeholder(?:-|$)",
      "selection(?:-|$)",
      "(?:from|via|to)(?:-|$)",
      "mix-blend(?:-|$)",
      "bg-blend(?:-|$)",
      "box-decoration(?:-|$)",
      "appearance-(?:none|auto)$",
      "forced-color-adjust(?:-|$)",
      "list-(?:none|disc|decimal|inside|outside|\\[|\\()",
      "indent(?:-|$)",
      "underline-offset(?:-|$)",
      "content-(?:none|\\[|\\()",
      "(?:italic|not-italic|antialiased|subpixel-antialiased|uppercase|lowercase|capitalize|normal-case|underline|overline|line-through|no-underline)$",
    ].join("|") +
    ")",
);

const arbitraryAppearanceProperty = new RegExp(
  "^\\[(?:" +
    [
      "color",
      "background(?:-color|-image)?",
      "border(?:-.+)?",
      "border-radius",
      "outline(?:-.+)?",
      "box-shadow",
      "text-shadow",
      "font(?:-.+)?",
      "line-height",
      "letter-spacing",
      "text-align",
      "text-transform",
      "text-decoration(?:-.+)?",
      "opacity",
      "cursor",
      "transition(?:-.+)?",
      "animation",
      "filter",
      "backdrop-filter",
      "-webkit-backdrop-filter",
      "fill",
      "stroke(?:-.+)?",
      "accent-color",
      "caret-color",
      "mix-blend-mode",
      "appearance",
    ].join("|") +
    "):",
);

function isTailwindAppearanceUtility(token) {
  const utility = stripTailwindVariants(token);
  return (
    tailwindTextAppearanceUtility.test(utility) ||
    tailwindAppearanceUtility.test(utility) ||
    arbitraryAppearanceProperty.test(utility)
  );
}

function callName(node) {
  if (ts.isIdentifier(node)) return node.text;
  if (ts.isPropertyAccessExpression(node)) return node.name.text;
  return "";
}

/** Product TS/TSX may retain structural Tailwind utilities, but appearance
 * recipes must be named semantically and defined in Dream CSS. This walks
 * class-bearing AST contexts only, so prose and unrelated string data do not
 * become false positives. */
export function auditTailwindAppearanceUtilitiesSource(content, relative) {
  if (/(?:^|\.)(?:test|spec)\.(?:ts|tsx)$/.test(relative)) {
    return [];
  }

  const sourceFile = ts.createSourceFile(
    relative,
    content,
    ts.ScriptTarget.Latest,
    true,
    relative.endsWith(".tsx") ? ts.ScriptKind.TSX : ts.ScriptKind.TS,
  );
  const findings = [];
  const auditedUtilities = new Set();
  const classCombinerNames = new Set(["cn", "clsx"]);
  const cvaNames = new Set(["cva"]);

  for (const statement of sourceFile.statements) {
    if (!ts.isImportDeclaration(statement) || !ts.isStringLiteral(statement.moduleSpecifier)) {
      continue;
    }
    const moduleName = statement.moduleSpecifier.text;
    const clause = statement.importClause;
    if (!clause) continue;
    if (moduleName === "clsx" && clause.name) {
      classCombinerNames.add(clause.name.text);
    }
    const bindings = clause.namedBindings;
    if (!bindings || !ts.isNamedImports(bindings)) continue;
    for (const specifier of bindings.elements) {
      const importedName = specifier.propertyName?.text ?? specifier.name.text;
      if (importedName === "clsx" || importedName === "cn") {
        classCombinerNames.add(specifier.name.text);
      }
      if (moduleName === "class-variance-authority" && importedName === "cva") {
        cvaNames.add(specifier.name.text);
      }
    }
  }

  function recordLiteral(node, value, context) {
    for (const token of value.split(/\s+/).filter(Boolean)) {
      if (!isTailwindAppearanceUtility(token)) continue;
      const key = `${node.pos}:${token}`;
      if (auditedUtilities.has(key)) continue;
      auditedUtilities.add(key);
      const { line } = sourceFile.getLineAndCharacterOfPosition(node.getStart(sourceFile));
      findings.push(
        `${relative}:${line + 1}: ${context} contains visual Tailwind utility ${JSON.stringify(token)}; replace it with a semantic class defined in Dream CSS`,
      );
    }
  }

  function auditClassExpression(expression, context, objectValuesAreClasses = false) {
    if (!expression) return;
    const node = unwrapClassExpression(expression);
    if (ts.isStringLiteralLike(node)) {
      recordLiteral(node, node.text, context);
      return;
    }
    if (ts.isTemplateExpression(node)) {
      const synthesized = [node.head.text];
      for (const span of node.templateSpans) {
        synthesized.push("${…}", span.literal.text);
      }
      recordLiteral(node, synthesized.join(""), context);
      return;
    }
    if (ts.isConditionalExpression(node)) {
      auditClassExpression(node.whenTrue, context, objectValuesAreClasses);
      auditClassExpression(node.whenFalse, context, objectValuesAreClasses);
      return;
    }
    if (ts.isBinaryExpression(node)) {
      if (
        node.operatorToken.kind === ts.SyntaxKind.PlusToken ||
        node.operatorToken.kind === ts.SyntaxKind.AmpersandAmpersandToken ||
        node.operatorToken.kind === ts.SyntaxKind.BarBarToken ||
        node.operatorToken.kind === ts.SyntaxKind.QuestionQuestionToken
      ) {
        auditClassExpression(node.left, context, objectValuesAreClasses);
        auditClassExpression(node.right, context, objectValuesAreClasses);
      }
      return;
    }
    if (ts.isArrayLiteralExpression(node)) {
      for (const element of node.elements) {
        if (!ts.isSpreadElement(element)) {
          auditClassExpression(element, context, objectValuesAreClasses);
        }
      }
      return;
    }
    if (ts.isObjectLiteralExpression(node)) {
      for (const property of node.properties) {
        if (ts.isPropertyAssignment(property)) {
          const name = classContextName(property.name, sourceFile);
          if (!objectValuesAreClasses) {
            if (!property.questionToken) {
              recordLiteral(property.name, name, context);
            }
          } else {
            auditClassExpression(property.initializer, context, true);
          }
        } else if (ts.isShorthandPropertyAssignment(property) && objectValuesAreClasses) {
          auditClassExpression(property.name, context, true);
        }
      }
      return;
    }
    if (ts.isCallExpression(node) && classCombinerNames.has(callName(node.expression))) {
      for (const argument of node.arguments) {
        auditClassExpression(argument, context);
      }
    }
  }

  function auditCvaConfig(config, context) {
    const node = unwrapClassExpression(config);
    if (!ts.isObjectLiteralExpression(node)) return;
    for (const property of node.properties) {
      if (!ts.isPropertyAssignment(property)) continue;
      const propertyName = classContextName(property.name, sourceFile);
      const value = unwrapClassExpression(property.initializer);
      if (propertyName === "variants" && ts.isObjectLiteralExpression(value)) {
        for (const variant of value.properties) {
          if (!ts.isPropertyAssignment(variant)) continue;
          auditClassExpression(variant.initializer, context, true);
        }
      } else if (
        propertyName === "compoundVariants" &&
        ts.isArrayLiteralExpression(value)
      ) {
        for (const element of value.elements) {
          const compound = unwrapClassExpression(element);
          if (!ts.isObjectLiteralExpression(compound)) continue;
          for (const entry of compound.properties) {
            if (!ts.isPropertyAssignment(entry)) continue;
            const entryName = classContextName(entry.name, sourceFile);
            if (entryName === "class" || entryName === "className") {
              auditClassExpression(entry.initializer, context);
            }
          }
        }
      }
    }
  }

  function visit(node) {
    if (ts.isJsxAttribute(node)) {
      const name = classContextName(node.name, sourceFile);
      if (isClassContextName(name) && node.initializer) {
        if (ts.isStringLiteral(node.initializer)) {
          auditClassExpression(node.initializer, `JSX ${name}`);
        } else if (ts.isJsxExpression(node.initializer)) {
          auditClassExpression(node.initializer.expression, `JSX ${name}`);
        }
      }
    } else if (ts.isVariableDeclaration(node)) {
      const name = classContextName(node.name, sourceFile);
      if (isClassContextName(name)) {
        auditClassExpression(node.initializer, `class constant ${name}`, true);
      }
    } else if (ts.isPropertyAssignment(node)) {
      const name = classContextName(node.name, sourceFile);
      if (isClassContextName(name)) {
        auditClassExpression(node.initializer, `class property ${name}`, true);
      }
    } else if (ts.isCallExpression(node)) {
      const name = callName(node.expression);
      if (classCombinerNames.has(name)) {
        for (const argument of node.arguments) {
          auditClassExpression(argument, `${name}(...)`);
        }
      } else if (cvaNames.has(name)) {
        auditClassExpression(node.arguments[0], `${name}(...) base`);
        if (node.arguments[1]) auditCvaConfig(node.arguments[1], `${name}(...) variant`);
      }
    }
    ts.forEachChild(node, visit);
  }
  visit(sourceFile);
  return findings;
}

function isStaticStyleAssignmentValue(node) {
  const value = unwrapStyleExpression(node);
  if (isStaticStyleValue(value)) return true;
  return (
    ts.isConditionalExpression(value) &&
    isStaticStyleAssignmentValue(value.whenTrue) &&
    isStaticStyleAssignmentValue(value.whenFalse)
  );
}

function assignedStyleProperty(node, sourceFile) {
  const target = unwrapStyleExpression(node);
  if (!ts.isPropertyAccessExpression(target) && !ts.isElementAccessExpression(target)) {
    return "";
  }
  const owner = unwrapStyleExpression(target.expression);
  const ownsStyle =
    (ts.isPropertyAccessExpression(owner) && owner.name.text === "style") ||
    (ts.isElementAccessExpression(owner) &&
      owner.argumentExpression &&
      ts.isStringLiteralLike(owner.argumentExpression) &&
      owner.argumentExpression.text === "style");
  if (!ownsStyle) return "";
  if (ts.isPropertyAccessExpression(target)) return target.name.text;
  return target.argumentExpression?.getText(sourceFile) ?? "style property";
}

function staticStyleSetProperty(node, sourceFile) {
  if (
    !ts.isCallExpression(node) ||
    !ts.isPropertyAccessExpression(node.expression) ||
    node.expression.name.text !== "setProperty" ||
    node.arguments.length < 2 ||
    !isStaticStyleAssignmentValue(node.arguments[1])
  ) {
    return "";
  }
  const owner = unwrapStyleExpression(node.expression.expression);
  const ownsStyle =
    (ts.isPropertyAccessExpression(owner) && owner.name.text === "style") ||
    (ts.isElementAccessExpression(owner) &&
      owner.argumentExpression &&
      ts.isStringLiteralLike(owner.argumentExpression) &&
      owner.argumentExpression.text === "style");
  if (!ownsStyle) return "";
  return node.arguments[0]?.getText(sourceFile) ?? "custom property";
}

/** Static CSSStyleDeclaration writes hide appearance recipes from Dream CSS.
 * Theme token projection and the Appearance Lab's controlled preview bridge are
 * the only intentional direct-style interfaces. */
export function auditStaticElementStyleAssignmentSource(content, relative) {
  if (/(?:^|\.)(?:test|spec)\.(?:ts|tsx)$/.test(relative)) {
    return [];
  }
  const sourceFile = ts.createSourceFile(
    relative,
    content,
    ts.ScriptTarget.Latest,
    true,
    relative.endsWith(".tsx") ? ts.ScriptKind.TSX : ts.ScriptKind.TS,
  );
  const findings = [];
  function visit(node) {
    if (
      ts.isBinaryExpression(node) &&
      node.operatorToken.kind === ts.SyntaxKind.EqualsToken &&
      isStaticStyleAssignmentValue(node.right)
    ) {
      const property = assignedStyleProperty(node.left, sourceFile);
      if (property) {
        const isThemeColorSchemeProjection =
          relative === "shared/styles/theme-runtime.ts" &&
          property === "colorScheme" &&
          node.left.getText(sourceFile) ===
            "document.documentElement.style.colorScheme";
        if (isThemeColorSchemeProjection) {
          ts.forEachChild(node, visit);
          return;
        }
        const { line } = sourceFile.getLineAndCharacterOfPosition(node.getStart(sourceFile));
        findings.push(
          `${relative}:${line + 1}: element.style.${property} receives a static appearance value; move the recipe to Dream CSS`,
        );
      }
    } else {
      const property = staticStyleSetProperty(node, sourceFile);
      if (property) {
        const { line } = sourceFile.getLineAndCharacterOfPosition(node.getStart(sourceFile));
        findings.push(
          `${relative}:${line + 1}: element.style.setProperty(${property}, ...) receives a static appearance value; move the recipe to Dream CSS`,
        );
      }
    }
    ts.forEachChild(node, visit);
  }
  visit(sourceFile);
  return findings;
}

function staticStringSource(node) {
  const value = unwrapClassExpression(node);
  if (ts.isStringLiteralLike(value)) return value.text;
  if (!ts.isTemplateExpression(value)) return "";
  return [
    value.head.text,
    ...value.templateSpans.flatMap((span) => ["${…}", span.literal.text]),
  ].join("");
}

function looksLikeStaticCssRecipe(value) {
  return /(?:^|[>}])\s*(?:[.#:[*]|[a-zA-Z])[^{]{0,180}\{[^{}]{0,240}:[^{}]*/s.test(
    value,
  );
}

/** Rejects stylesheet payloads assembled in TypeScript strings. Raw CSS
 * imports are allowed because the source stays discoverable in Dream. */
export function auditStaticCssStringSource(content, relative) {
  if (/(?:^|\.)(?:test|spec)\.(?:ts|tsx)$/.test(relative)) return [];
  const sourceFile = ts.createSourceFile(
    relative,
    content,
    ts.ScriptTarget.Latest,
    true,
    relative.endsWith(".tsx") ? ts.ScriptKind.TSX : ts.ScriptKind.TS,
  );
  const findings = [];
  const audited = new Set();
  function record(node, context) {
    const value = staticStringSource(node);
    if (!value || !looksLikeStaticCssRecipe(value) || audited.has(node.pos)) return;
    audited.add(node.pos);
    const { line } = sourceFile.getLineAndCharacterOfPosition(node.getStart(sourceFile));
    findings.push(
      `${relative}:${line + 1}: ${context} contains a static CSS recipe; move it to an imported Dream CSS raw module`,
    );
  }
  function recordStyleCollection(node, context) {
    const value = unwrapClassExpression(node);
    if (ts.isArrayLiteralExpression(value)) {
      for (const element of value.elements) recordStyleCollection(element, context);
      return;
    }
    record(value, context);
  }
  function visit(node) {
    if (ts.isVariableDeclaration(node)) {
      const name = classContextName(node.name, sourceFile);
      if (/(?:css|styles?|styleSheet)$/i.test(name)) {
        recordStyleCollection(node.initializer, `style source ${name}`);
      }
    }
    if (ts.isStringLiteralLike(node) || ts.isTemplateExpression(node)) {
      const value = staticStringSource(node);
      const styleStart = value.search(/<style\b[^>]*>/i);
      if (styleStart >= 0 && looksLikeStaticCssRecipe(value.slice(styleStart))) {
        record(node, "inline <style> payload");
      }
    }
    ts.forEachChild(node, visit);
  }
  visit(sourceFile);
  return findings;
}

export function auditRSSDocumentStyleContract(source, cssSource) {
  const findings = [];
  if (!source.includes('rss-documents.css?raw"')) {
    findings.push(
      `${rssDocumentSourcePath}: reader and print documents must import their raw Dream CSS module`,
    );
  }
  if ((source.match(/<style>\$\{rssDocumentStyles\}<\/style>/g) ?? []).length !== 2) {
    findings.push(
      `${rssDocumentSourcePath}: reader and print documents must embed only rssDocumentStyles`,
    );
  }
  for (const marker of [
    'class="app-rss-reader-document"',
    'class="app-rss-print-document"',
    'style="${readerStyleProperties}"',
    "--app-rss-reader-font-size",
    "--app-rss-reader-line-height",
    "--app-rss-reader-paragraph-spacing",
  ]) {
    if (!source.includes(marker)) {
      findings.push(`${rssDocumentSourcePath}: missing raw-document contract ${marker}`);
    }
  }
  let stylesheet;
  try {
    stylesheet = postcss.parse(cssSource, { from: rssDocumentCssPath });
  } catch (error) {
    return [
      ...findings,
      `${rssDocumentCssPath}: failed to parse scoped raw document CSS (${error instanceof Error ? error.message : String(error)})`,
    ];
  }
  stylesheet.walkRules((rule) => {
    if (
      !rule.selectors.every((selector) =>
        selector.trim().startsWith("html.app-rss-"),
      )
    ) {
      findings.push(
        `${rssDocumentCssPath}:${rule.source?.start?.line ?? 1}: raw document selectors must stay scoped to html.app-rss-*`,
      );
    }
  });
  for (const marker of [
    "html.app-rss-reader-document",
    "html.app-rss-print-document",
    "var(--app-rss-reader-font-size)",
    "var(--app-rss-reader-line-height)",
    "var(--app-rss-reader-paragraph-spacing)",
  ]) {
    if (!cssSource.includes(marker)) {
      findings.push(`${rssDocumentCssPath}: missing scoped raw recipe ${marker}`);
    }
  }
  return findings;
}

function maskAppearanceLabMaterialSpecimens(content) {
  return content.replace(
    /<GlassSurface\b(?=[^>]*\bdata-appearance-material-specimen=)[^>]*>/gs,
    (openingTag) => openingTag.replace(/[^\n]/g, " "),
  );
}

export function auditProductSurfaceEntries(content, relative) {
  if (
    relative === "shared/ui/glass-surface.tsx" ||
    /(?:^|\.)test\.tsx$/.test(relative)
  ) {
    return [];
  }

  const findings = [];
  const source =
    relative === "app/dev/AppearanceLab.tsx"
      ? maskAppearanceLabMaterialSpecimens(content)
      : content;

  if (!relative.startsWith("shared/ui/")) {
    const auditedTagStarts = new Set();
    let markerOffset = source.indexOf("app-glass-surface");
    while (markerOffset >= 0) {
      const tagStart = source.lastIndexOf("<", markerOffset);
      if (tagStart >= 0 && !auditedTagStarts.has(tagStart)) {
        auditedTagStarts.add(tagStart);
        let quote = "";
        let braceDepth = 0;
        let tagEnd = -1;
        for (let index = tagStart + 1; index < source.length; index += 1) {
          const character = source[index];
          if (quote) {
            if (character === quote && source[index - 1] !== "\\") {
              quote = "";
            }
            continue;
          }
          if (character === '"' || character === "'") {
            quote = character;
          } else if (character === "{") {
            braceDepth += 1;
          } else if (character === "}") {
            braceDepth = Math.max(0, braceDepth - 1);
          } else if (character === ">" && braceDepth === 0) {
            tagEnd = index;
            break;
          }
        }

        if (tagEnd >= 0) {
          const openingTag = source.slice(tagStart, tagEnd + 1);
          const isRawGlassSurface =
            /^<[a-z][\w:.-]*\b/.test(openingTag) &&
            openingTag.includes("app-glass-surface");
          const resolvesMaterial =
            /getXiaSurfaceAttributes\s*\(/.test(openingTag) ||
            /\bdata-material\s*=/.test(openingTag);
          if (isRawGlassSurface && !resolvesMaterial) {
            findings.push(
              `${relative}:${lineNumber(source, tagStart)}: raw app-glass-surface entries must resolve their material through GlassSurface, getXiaSurfaceAttributes, or an explicit data-material attribute`,
            );
          }
        }
      }
      markerOffset = source.indexOf("app-glass-surface", markerOffset + 1);
    }
  }

  const directMaterial =
    /\b(?:data-material|material)\s*=\s*(?:\{\s*)?["'](?:regular|panel|clear|solid)["']/g;
  for (const match of source.matchAll(directMaterial)) {
    findings.push(
      `${relative}:${lineNumber(source, match.index ?? 0)}: product code must declare surfaceRole instead of a literal material`,
    );
  }

  const glassEntry = /<Glass(?:Surface|Group)\b[\s\S]*?>/g;
  for (const match of source.matchAll(glassEntry)) {
    if (!/\bsurfaceRole\s*=/.test(match[0])) {
      findings.push(
        `${relative}:${lineNumber(source, match.index ?? 0)}: GlassSurface and GlassGroup entries must declare surfaceRole`,
      );
    }
  }

  return findings;
}

export async function auditAppearance({ sourceRoot = srcRoot } = {}) {
  const findings = [];
  findings.push(
    ...auditTailwindCompositionConfigSource(
      await readFile(path.join(frontendRoot, "tailwind.config.js"), "utf8"),
    ),
  );
  const foreignProviderSources = new Map(
    await Promise.all(
      foreignProviderBridgePaths.map(async (relative) => [
        relative,
        await readFile(path.join(repoRoot, relative), "utf8"),
      ]),
    ),
  );
  findings.push(
    ...auditForeignProviderAppearanceBoundary(foreignProviderSources),
  );
  const cssFiles = await collectCssFiles(sourceRoot);
  const cssByPath = new Map();

  for (const absolute of cssFiles) {
    const relative = relativeToSrc(absolute, sourceRoot);
    const content = await readFile(absolute, "utf8");
    cssByPath.set(relative, content);

    findings.push(...auditFeatureCssAppearanceBoundary(content, relative));
    findings.push(...auditFeatureMaterialFilters(content, relative));
    if (relative === "index.css") {
      findings.push(...auditStyleEntrypointBoundary(content, relative));
    }

    if (
      relative !== layoutContractPath &&
      /(?:^|\n)\s*\.app-workspace-primary-header\s*\{/.test(content)
    ) {
      findings.push(
        `${relative}: the Primary header recipe may only be defined in ${layoutContractPath}`,
      );
    }

    if (relative !== tokenPath) {
      for (const token of canonicalAppearanceTokens) {
        const definition = new RegExp(`${token.replaceAll("-", "\\-")}\\s*:`);
        if (definition.test(content)) {
          findings.push(
            `${relative}: canonical appearance token ${token} may only be defined in ${tokenPath}`,
          );
        }
      }
    }

    if (relative !== tokenPath) {
      const rawBackdrop = /(?:-webkit-)?backdrop-filter\s*:\s*[^;{}]*\bblur\(/g;
      for (const match of content.matchAll(rawBackdrop)) {
        findings.push(
          `${relative}:${lineNumber(content, match.index ?? 0)}: raw backdrop blur must use a semantic material filter token`,
        );
      }
    }

    if (relative !== glassContractPath) {
      const recipeDefinition = /\.app-glass-surface(?:\[data-material\])?\s*\{/g;
      for (const match of content.matchAll(recipeDefinition)) {
        findings.push(
          `${relative}:${lineNumber(content, match.index ?? 0)}: the core glass recipe may only be defined in ${glassContractPath}`,
        );
      }
    }
  }

  for (const relative of migratedMaterialFiles) {
    const content = cssByPath.get(relative) ?? "";
    if (/--app-glass-(?:regular|panel)-(?:surface|filter|shadow)\s*:/.test(content)) {
      findings.push(
        `${relative}: migrated components must consume, not redefine, semantic material tokens`,
      );
    }
  }

  findings.push(
    ...auditWorkspaceStructuralOverrides(cssByPath),
    ...auditWorkspacePageLayoutOwnership(cssByPath),
  );

  const readSource = async (relative) =>
    readFile(path.join(sourceRoot, relative), "utf8");
  const entry = await readSource("shared/styles/dream.css");
  const contractOrder = [
    './dream/tokens.css',
    './dream/glass.css',
    './dream/components.css',
    './dream/layout-contract.css',
    './dream/completed.css',
    './dream/status-contract.css',
    './dream/dialog-contract.css',
    './dream/button-contract.css',
  ];
  let previousIndex = -1;
  for (const item of contractOrder) {
    const index = entry.indexOf(item);
    if (index < 0 || index <= previousIndex) {
      findings.push(
        `shared/styles/dream.css: expected stable import order ${contractOrder.join(" -> ")}`,
      );
      break;
    }
    previousIndex = index;
  }

  const glass = cssByPath.get(glassContractPath) ?? "";
  for (const selector of [
    '.app-glass-surface[data-material]',
    '[data-material="panel"]',
    '[data-material="clear"]',
    '[data-material="solid"]',
    '.app-glass-group',
  ]) {
    if (!glass.includes(selector)) {
      findings.push(`${glassContractPath}: missing semantic contract ${selector}`);
    }
  }

  findings.push(
    ...auditCanonicalSurfaceTokens(cssByPath.get(tokenPath) ?? ""),
    ...auditDreamAppearanceContracts(
      cssByPath.get(tokenPath) ?? "",
      cssByPath.get(statusContractPath) ?? "",
      await readSource(statusBadgePath),
      await readSource(appearanceContractPath),
    ),
    ...auditSharedWindowCanvasContract(
      cssByPath.get(tokenPath) ?? "",
      cssByPath.get(settingsShellPath) ?? "",
    ),
    ...auditContrastCanvasContract(
      cssByPath.get(workspaceAppearancePath) ?? "",
      cssByPath.get(tokenPath) ?? "",
    ),
  );

  const sourcePaths = new Set([
    surfaceContractPath,
    ...overlayRoleConsumers,
    workspaceActivityPath,
    mainAppPath,
    workspaceSearchControlPath,
    ...workspaceSearchConsumerPaths,
    rssWorkspacePath,
    rssDocumentSourcePath,
    ...primaryHeaderConsumerPaths,
  ]);
  const sourceByPath = new Map(
    await Promise.all(
      [...sourcePaths].map(async (relative) => [relative, await readSource(relative)]),
    ),
  );

  findings.push(
    ...auditSurfaceContractSource(sourceByPath.get(surfaceContractPath) ?? ""),
    ...auditOverlayRoleConsumers(sourceByPath),
    ...auditWorkspaceStatusRole(sourceByPath.get(workspaceActivityPath) ?? ""),
    ...auditWorkspaceContentControlContracts(
      sourceByPath,
      cssByPath.get(dreamControlsPath) ?? "",
    ),
    ...auditPrimaryHeaderContract(
      cssByPath.get(layoutContractPath) ?? "",
      sourceByPath,
    ),
    ...auditWorkspacePaneAppearanceContract(
      cssByPath.get(tokenPath) ?? "",
      cssByPath.get(layoutContractPath) ?? "",
      [
        cssByPath.get(workspaceSurfacePath) ?? "",
        cssByPath.get(workspaceAppearancePath) ?? "",
      ].join("\n"),
      cssByPath.get(workspaceAppearancePath) ?? "",
      sourceByPath.get(mainAppPath) ?? "",
      await readSource(appearanceContractPath),
    ),
    ...auditRSSDocumentStyleContract(
      sourceByPath.get(rssDocumentSourcePath) ?? "",
      cssByPath.get(rssDocumentCssPath) ?? "",
    ),
  );

  for (const relative of roleOwnedFeatureFiles) {
    findings.push(
      ...auditRoleOwnedFeatureSource(sourceByPath.get(relative) ?? "", relative),
    );
  }

  const typeScriptFiles = await collectTypeScriptFiles(sourceRoot);
  for (const absolute of typeScriptFiles) {
    const relative = relativeToSrc(absolute, sourceRoot);
    const content = await readFile(absolute, "utf8");
    findings.push(
      ...auditInlineBackdropBlurSource(
        content,
        relative,
      ),
      ...auditInlineStaticStyleSource(
        content,
        relative,
      ),
      ...auditTailwindAppearanceUtilitiesSource(content, relative),
      ...auditStaticElementStyleAssignmentSource(content, relative),
      ...auditStaticCssStringSource(content, relative),
    );
  }

  const tsxFiles = await collectTsxFiles(sourceRoot);
  for (const absolute of tsxFiles) {
    const relative = relativeToSrc(absolute, sourceRoot);
    findings.push(
      ...auditProductSurfaceEntries(await readFile(absolute, "utf8"), relative),
    );
  }

  return findings;
}

async function main() {
  const findings = await auditAppearance();
  if (findings.length > 0) {
    console.error("Appearance audit failed:");
    findings.forEach((finding) => console.error(`- ${finding}`));
    process.exitCode = 1;
    return;
  }

  console.log(
    "Appearance audit passed: canvas, pane geometry, status, overlay, navigation selection, material recipes, inline styles, Tailwind utilities, and blur ownership are governed by shared contracts.",
  );
}

if (process.argv[1] && path.resolve(process.argv[1]) === scriptPath) {
  await main();
}
