import * as React from "react";

import { GlassSurface } from "@/shared/ui/glass-surface";
import { Input } from "@/shared/ui/input";

import {
  DREAM_STYLE_REGISTRY,
  filterDreamStyleRegistry,
  type DreamStyleEntry,
  type DreamStyleModuleInventory,
} from "./dream-style-registry";

function CatalogEntries({ entries, label }: { entries: DreamStyleEntry[]; label: string }) {
  if (entries.length === 0) return null;
  return (
    <section className="appearance-lab__catalog-group">
      <h4>{label} <span>{entries.length}</span></h4>
      <ol>
        {entries.map((entry, index) => (
          <li key={`${entry.kind}-${entry.line}-${entry.name}-${index}`}>
            <code>{entry.name}</code>
            {entry.value ? <span>{entry.value}</span> : null}
            <small>line {entry.line}</small>
          </li>
        ))}
      </ol>
    </section>
  );
}

function CatalogModule({
  module,
  forceOpen,
}: {
  module: DreamStyleModuleInventory;
  forceOpen: boolean;
}) {
  return (
    <GlassSurface asChild elevation="embedded" shape="control" surfaceRole="inset">
      <details className="appearance-lab__catalog-module" open={forceOpen || undefined}>
        <summary>
        <strong>{module.id}.css</strong>
        <span>{module.tokens.length} tokens</span>
        <span>{module.selectors.length} selectors</span>
        <span>{module.atRules.length} at-rules</span>
        <span>{module.keyframes.length} keyframes</span>
        </summary>
        <div className="appearance-lab__catalog-module-body">
          <CatalogEntries entries={module.tokens} label="Tokens" />
          <CatalogEntries entries={module.selectors} label="Selectors" />
          <CatalogEntries entries={module.atRules} label="At-rules" />
          <CatalogEntries entries={module.keyframes} label="Keyframes" />
        </div>
      </details>
    </GlassSurface>
  );
}

export function DreamStyleCatalog() {
  const [query, setQuery] = React.useState("");
  const filtered = React.useMemo(
    () => filterDreamStyleRegistry(DREAM_STYLE_REGISTRY, query),
    [query],
  );

  return (
    <section
      aria-labelledby="appearance-lab-style-catalog-title"
      className="appearance-lab__section appearance-lab__section--catalog"
      data-appearance-fixture="dream-style-catalog"
    >
      <div className="appearance-lab__section-heading">
        <span>09</span>
        <div>
          <h2 id="appearance-lab-style-catalog-title">Dream CSS registry</h2>
          <p>Vite discovers every Dream module; a pure parser inventories its public CSS facts.</p>
        </div>
      </div>

      <div className="appearance-lab__catalog-toolbar">
        <Input
          aria-label="Search Dream CSS registry"
          onChange={(event) => setQuery(event.currentTarget.value)}
          placeholder="Search module, token, selector or keyframe"
          type="search"
          value={query}
        />
        <div aria-live="polite" className="appearance-lab__catalog-totals">
          <span><strong>{filtered.totals.modules}</strong> modules</span>
          <span><strong>{filtered.totals.token}</strong> tokens</span>
          <span><strong>{filtered.totals.selector}</strong> selectors</span>
          <span><strong>{filtered.totals["at-rule"]}</strong> at-rules</span>
          <span><strong>{filtered.totals.keyframe}</strong> keyframes</span>
        </div>
      </div>

      {DREAM_STYLE_REGISTRY.modules.length === 0 ? (
        <p className="appearance-lab__catalog-empty">
          Registry sources are injected by Vite. The parser intentionally falls back to an empty registry under Bun.
        </p>
      ) : filtered.modules.length === 0 ? (
        <p className="appearance-lab__catalog-empty">No Dream CSS entry matches “{query}”.</p>
      ) : (
        <div className="appearance-lab__catalog-list">
          {filtered.modules.map((module) => (
            <CatalogModule forceOpen={Boolean(query.trim())} key={module.path} module={module} />
          ))}
        </div>
      )}
    </section>
  );
}
