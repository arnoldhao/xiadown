import { describe, expect, test } from "bun:test";
import { basename, join } from "node:path";
import { fileURLToPath } from "node:url";

type SourceScope = {
  name: string;
  root: URL;
  glob: string;
};

const sourceScopes: SourceScope[] = [
  { name: "Library", root: new URL("../../app/library/", import.meta.url), glob: "**/*.tsx" },
  { name: "RSS", root: new URL("../../app/rss/", import.meta.url), glob: "**/*.tsx" },
  { name: "YouTube", root: new URL("../../app/youtube/", import.meta.url), glob: "**/*.tsx" },
  { name: "Pets gallery", root: new URL("../../app/pets-gallery/", import.meta.url), glob: "**/*.tsx" },
  { name: "Pets feature", root: new URL("../../features/pets/", import.meta.url), glob: "**/*.tsx" },
  { name: "Workspace sidebars", root: new URL("../../app/workspace/", import.meta.url), glob: "*Sidebar*.tsx" },
];

const visualUtility = /^(?:[a-z0-9_-]+:)*(?:bg-[a-z0-9_[\]./%-]+|text-[a-z0-9_[\]./%-]+|font-[a-z0-9_[\]./%-]+|leading-[a-z0-9_[\]./%-]+|tracking-[a-z0-9_[\]./%-]+|border(?:-[a-z0-9_[\]./%-]+)?|divide-[a-z0-9_[\]./%-]+|from-[a-z0-9_[\]./%-]+|via-[a-z0-9_[\]./%-]+|to-[a-z0-9_[\]./%-]+|rounded(?:-[a-z0-9_[\]./%-]+)?|shadow(?:-[a-z0-9_[\]./%-]+)?|drop-shadow(?:-[a-z0-9_[\]./%-]+)?|ring(?:-[a-z0-9_[\]./%-]+)?|opacity-[a-z0-9_[\]./%-]+|outline-[a-z0-9_[\]./%-]+|fill-[a-z0-9_[\]./%-]+|stroke-[a-z0-9_[\]./%-]+|decoration-[a-z0-9_[\]./%-]+|accent-[a-z0-9_[\]./%-]+|caret-[a-z0-9_[\]./%-]+|placeholder-[a-z0-9_[\]./%-]+|cursor-[a-z0-9_[\]./%-]+|(?:backdrop-)?blur-[a-z0-9_[\]./%-]+|transition-colors|appearance-none|italic|not-italic|uppercase|lowercase|capitalize|normal-case|antialiased|subpixel-antialiased|tabular-nums)$/i;
const arbitraryUtility = /-\[[^\]\n]+\]/;
const testSource = /\.(?:test|spec)\.tsx$/;

function quotedTokens(source: string): string[] {
  const tokens: string[] = [];
  for (const match of source.matchAll(/"([^"\n]*)"|'([^'\n]*)'|`([^`\n]*)`/g)) {
    const value = match[1] ?? match[2] ?? match[3] ?? "";
    tokens.push(...value.split(/\s+/).filter(Boolean));
  }
  return tokens;
}

async function scopedSources() {
  const sources: Array<{ scope: string; path: string; source: string }> = [];

  for (const scope of sourceScopes) {
    const root = fileURLToPath(scope.root);
    const glob = new Bun.Glob(scope.glob);
    for await (const relativePath of glob.scan({ cwd: root, onlyFiles: true })) {
      if (testSource.test(relativePath)) continue;
      sources.push({
        scope: scope.name,
        path: basename(relativePath),
        source: await Bun.file(join(root, relativePath)).text(),
      });
    }
  }

  return sources;
}

describe("Dream product surface Tailwind boundary", () => {
  test("keeps visual utilities and arbitrary values out of migrated product surfaces", async () => {
    const violations: string[] = [];

    for (const entry of await scopedSources()) {
      for (const token of quotedTokens(entry.source)) {
        if (visualUtility.test(token) || arbitraryUtility.test(token)) {
          violations.push(`${entry.scope}/${entry.path}: ${token}`);
        }
      }
    }

    expect(violations).toEqual([]);
  });

  test("keeps the migrated semantic hooks discoverable in Dream", async () => {
    const [library, rss, youtube, pets, workspace] = await Promise.all([
      Bun.file(new URL("./dream/library.css", import.meta.url)).text(),
      Bun.file(new URL("./dream/rss.css", import.meta.url)).text(),
      Bun.file(new URL("./dream/youtube.css", import.meta.url)).text(),
      Bun.file(new URL("./dream/pets.css", import.meta.url)).text(),
      Bun.file(new URL("./dream/workspace.css", import.meta.url)).text(),
    ]);

    expect(library).toContain(".app-library-search");
    expect(rss).toContain(".app-rss-subscription-context-menu__anchor");
    expect(youtube).toContain(".youtube-workspace-video-card__open");
    expect(pets).toContain(".app-pets-animation-card");
    expect(workspace).toContain(".app-sniff-workspace-sidebar__waiting-pet");
  });
});
