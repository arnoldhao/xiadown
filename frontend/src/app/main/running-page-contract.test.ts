import { describe, expect, test } from "bun:test";

describe("Running page contract", () => {
  test("uses the operational page shell and one content scroll owner", async () => {
    const source = await Bun.file(
      new URL("./RunningPage.tsx", import.meta.url),
    ).text();

    expect(source).toContain("<WorkspacePage");
    expect(source).toContain("<WorkspacePageTopBar");
    expect(source).toContain("<WorkspacePageContent");
    expect(source).toContain('recipe: "operational"');
    expect(source).toContain('contentLayout: "canvas"');
    expect(source).toContain('footer: "none"');
    expect(source).toContain('heading: "assistive"');
    expect(source).toContain('scroll: "content"');
    expect(source).toMatch(/<WorkspacePageContent[\s\S]{0,180}ref=\{scrollRef\}/);
    expect(source).not.toContain("<h1");
    expect(source).not.toContain("app-workspace-primary-header__safe-area");
  });

  test("keeps the interactive empty state full-height inside the canvas", async () => {
    const source = await Bun.file(
      new URL("./RunningPage.tsx", import.meta.url),
    ).text();
    const emptyStart = source.indexOf("if (operations.length === 0)");
    const taskStart = source.indexOf(
      '\n  return renderShell(\n    <div className="relative',
      emptyStart,
    );
    const emptyState = source.slice(emptyStart, taskStart);

    expect(emptyStart).toBeGreaterThan(-1);
    expect(taskStart).toBeGreaterThan(emptyStart);
    expect(emptyState).toContain('data-running-empty-state="true"');
    expect(emptyState).toContain('className="h-full min-h-0 px-5 pb-5"');
    expect(emptyState).toContain("<RunningPetPlayground");
    expect(emptyState).toContain("label={text.running.emptyAction}");
    expect(emptyState).toContain("onClick={props.onNewDownload}");
    expect(emptyState).not.toContain('className="min-h-full px-5 pb-5"');
  });

  test("declares artwork layers while Dream CSS owns their visual recipe", async () => {
    const [source, dreamWorkflowStyles] = await Promise.all([
      Bun.file(new URL("./RunningPage.tsx", import.meta.url)).text(),
      Bun.file(
        new URL("../../shared/styles/dream/workflows.css", import.meta.url),
      ).text(),
    ]);

    const artworkLayers = [
      "app-running-thumbnail-stage",
      "app-running-thumbnail-blur",
      "app-running-thumbnail-detail",
      "app-running-thumbnail-glass-veil",
      "app-running-thumbnail-grain-field",
      "app-running-thumbnail-texture",
      "app-running-thumbnail-sweep",
      "app-running-thumbnail-fallback",
      "app-running-card-ring",
    ];

    for (const className of artworkLayers) {
      expect(source).toContain(`className="${className}"`);
      expect(dreamWorkflowStyles).toContain(`.${className}`);
    }

    expect(source).not.toMatch(
      /(?:backdropFilter|WebkitBackdropFilter|maskImage|WebkitMaskImage)/,
    );
    expect(source).not.toContain("bg-[linear-gradient");
    expect(dreamWorkflowStyles).toContain(
      "backdrop-filter: var(--running-thumbnail-veil-filter)",
    );
  });

  test("publishes semantic Pet glow variants while Dream owns gradients and masks", async () => {
    const [pageSource, playgroundSource, playerSource, petStyles, styleVocabulary] =
      await Promise.all([
        Bun.file(new URL("./RunningPage.tsx", import.meta.url)).text(),
        Bun.file(new URL("./RunningPetPlayground.tsx", import.meta.url)).text(),
        Bun.file(new URL("../../shared/ui/pet-player.tsx", import.meta.url)).text(),
        Bun.file(new URL("../../shared/styles/dream/pets.css", import.meta.url)).text(),
        Bun.file(new URL("../../shared/styles/xiadown.ts", import.meta.url)).text(),
      ]);

    expect(pageSource).toContain(
      'glowVariant={useRunningPetGlow ? "running-summary" : undefined}',
    );
    expect(playgroundSource).toContain('glowVariant="running-playground"');
    expect(`${pageSource}\n${playgroundSource}`).not.toContain("glowStyle");
    expect(`${pageSource}\n${playgroundSource}`).not.toContain(
      "RUNNING_PET_GLOW_STYLE",
    );
    const pagePet = pageSource.slice(
      pageSource.indexOf("<PetDisplay"),
      pageSource.indexOf("/>", pageSource.indexOf("<PetDisplay")) + 2,
    );
    const playgroundPet = playgroundSource.slice(
      playgroundSource.indexOf("<PetDisplay"),
      playgroundSource.indexOf(
        "/>",
        playgroundSource.indexOf("<PetDisplay"),
      ) + 2,
    );
    expect(`${pagePet}\n${playgroundPet}`).not.toMatch(/(?:blur|h|w)-\[/);
    expect(playerSource).toContain("data-glow-variant={glowVariant}");
    expect(playerSource).not.toContain("style={glow");
    expect(petStyles).toContain('[data-glow-variant="running-summary"]');
    expect(petStyles).toContain('[data-glow-variant="running-playground"]');
    expect(petStyles).toContain("-webkit-mask-image: radial-gradient(");
    expect(petStyles).toContain("mask-image: radial-gradient(");
    expect(styleVocabulary).not.toMatch(
      /React\.CSSProperties|backgroundImage|(?:Webkit)?maskImage|shadow-\[|bg-\[/,
    );
  });
});
