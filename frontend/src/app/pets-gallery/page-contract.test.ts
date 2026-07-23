import { describe, expect, test } from "bun:test";

describe("Pet Gallery page contract", () => {
  test("uses the shared browse recipe for the gallery and detail anatomy for a pet", async () => {
    const source = await Bun.file(
      new URL("./PetsGalleryPage.tsx", import.meta.url),
    ).text();

    expect(source).toContain("<WorkspacePage");
    expect(source).toContain("<WorkspacePageTopBar");
    expect(source).toContain("<WorkspacePageContent");
    expect(source).toContain('recipe: "browse"');
    expect(source).toContain('topBar: "drag"');
    expect(source).toContain('heading: "display"');
    expect(source).toContain('contentLayout: "card-grid"');
    expect(source).toContain('density: "comfortable"');
    expect(source).toContain('footer: "overlay"');
    expect(source).toContain('className="app-pets-gallery-grid"');
    expect(source).toContain('recipe: "detail"');
    expect(source).toContain('topBar: "navigation"');
    expect(source).toContain('heading: "assistive"');
    expect(source).toContain('scroll: "content"');
    expect(source).toContain('footer: "none"');
    expect(source).toContain("headingActions={");
    expect(source).toContain('"app-pets-gallery-heading"');
    expect(source).toContain("<WorkspacePrimaryHeaderAction");
    expect(source).toContain("<WorkspacePageFooter");
    expect(source).not.toContain('recipe: "custom"');
    expect(source).not.toContain("<h1");
    expect(source).not.toContain("app-workspace-primary-header__safe-area");
  });

  test("keeps Help beside the h1 and the Import Pet command centered at the bottom", async () => {
    const source = await Bun.file(
      new URL("./PetsGalleryPage.tsx", import.meta.url),
    ).text();
    const styles = await Bun.file(
      new URL("../../shared/styles/dream/pets.css", import.meta.url),
    ).text();

    const headingActionsStart = source.indexOf("headingActions={");
    const headingActionsEnd = source.indexOf("\n        }\n      >", headingActionsStart);
    const headingActions = source.slice(headingActionsStart, headingActionsEnd);
    const contentEnd = source.indexOf("</WorkspacePageContent>");
    const footerStart = source.indexOf("<WorkspacePageFooter", contentEnd);
    const footerEnd = source.indexOf("</WorkspacePageFooter>", footerStart);
    const footer = source.slice(footerStart, footerEnd);

    expect(headingActionsStart).toBeGreaterThan(-1);
    expect(headingActionsEnd).toBeGreaterThan(headingActionsStart);
    expect(headingActions).toContain("<HelpCircle");
    expect(headingActions).toContain('className="app-pets-gallery-heading-help"');
    expect(headingActions).not.toContain("<Upload");
    expect(headingActions).not.toContain("text.petGallery.importAction");

    expect(footerStart).toBeGreaterThan(contentEnd);
    expect(footerEnd).toBeGreaterThan(footerStart);
    expect(footer).toContain('className="app-pets-gallery-import-footer"');
    expect(footer).toContain("<Upload");
    expect(footer).toContain("text.petGallery.importAction");

    expect(styles).toMatch(
      /\.app-pets-gallery-heading\s*\{[^}]*justify-content:\s*flex-start/s,
    );
    expect(styles).toMatch(
      /\.app-pets-gallery-import-footer\s*\{[^}]*justify-content:\s*center[^}]*pointer-events:\s*none/s,
    );
    expect(styles).toMatch(
      /\.app-pets-gallery-import-footer\s*>\s*\.app-running-new-download-button\s*\{[^}]*pointer-events:\s*auto/s,
    );
  });

  test("adapts pet details to the Primary pane instead of the outer window", async () => {
    const styles = await Bun.file(
      new URL("../../shared/styles/dream/pets.css", import.meta.url),
    ).text();

    expect(styles).toMatch(
      /\.app-main-pets-page\s*\{[^}]*container: app-pets-page \/ inline-size;/s,
    );
    expect(styles).toMatch(
      /@container app-pets-page \(max-width: 900px\)[\s\S]*?\.app-pets-detail-grid\s*\{[^}]*grid-template-columns: minmax\(0, 1fr\);/,
    );
    expect(styles).not.toContain("@media (max-width: 900px)");
  });

  test("uses the shared equal-width Dream action group for pet details", async () => {
    const [source, styles, buttonContract] = await Promise.all([
      Bun.file(new URL("./PetsGalleryPage.tsx", import.meta.url)).text(),
      Bun.file(
        new URL("../../shared/styles/dream/pets.css", import.meta.url),
      ).text(),
      Bun.file(
        new URL("../../shared/styles/dream/button-contract.css", import.meta.url),
      ).text(),
    ]);

    expect(source).toContain(
      'className="app-dream-button-group app-dream-button-group--segmented app-pets-detail-actions mt-4 shrink-0"',
    );
    expect(source).not.toContain("app-pets-detail-actions mt-4 grid shrink-0 grid-cols-3");
    expect(styles).toMatch(
      /\.app-pets-detail-actions\s*\{[^}]*width:\s*100%[^}]*min-width:\s*0/s,
    );
    expect(buttonContract).toMatch(
      /\.app-dream-button-group\.app-dream-button-group--segmented\s*\{[^}]*display:\s*grid[^}]*grid-auto-columns:\s*minmax\(0, 1fr\)[^}]*grid-auto-flow:\s*column[^}]*gap:\s*1px/s,
    );
    expect(buttonContract).toMatch(
      /\.app-dream-button-group\.app-dream-button-group--segmented\s*>\s*\.app-dream-button[^\{]*\{[^}]*width:\s*100%[^}]*min-width:\s*0[^}]*border-radius:\s*var\(--dream-control-radius-inner\)[^}]*background:\s*transparent/s,
    );
  });

  test("publishes semantic pet lighting while Dream owns every optical recipe", async () => {
    const [cardSource, playerSource, styles, styleVocabulary] = await Promise.all([
      Bun.file(new URL("../../features/pets/card.tsx", import.meta.url)).text(),
      Bun.file(new URL("../../shared/ui/pet-player.tsx", import.meta.url)).text(),
      Bun.file(new URL("../../shared/styles/dream/pets.css", import.meta.url)).text(),
      Bun.file(new URL("../../shared/styles/xiadown.ts", import.meta.url)).text(),
    ]);

    expect(cardSource).toContain("data-lighting={lightingVariant}");
    expect(cardSource).toContain("app-pets-gallery-card-light--primary");
    expect(cardSource).toContain("app-pets-gallery-card-light--directional");
    expect(cardSource).toContain("app-pets-gallery-card-light--rim");
    expect(cardSource).toContain('glowVariant={isDefault ? "gallery-default" : undefined}');
    expect(cardSource).not.toMatch(/(?:shadow-|bg-)\[/);
    expect(cardSource).not.toContain("resolvePetCardLighting");
    expect(cardSource).not.toContain("glowStyle");

    expect(playerSource).toContain('className={cn("app-pet-display-glow"');
    expect(playerSource).toContain("data-glow-variant={glowVariant}");
    expect(playerSource).not.toContain("PET_DISPLAY_GLOW_STYLE");
    expect(playerSource).not.toContain("glowStyle");

    for (const variant of ["default", "imported", "online"]) {
      expect(styles).toContain(`data-lighting="${variant}"`);
    }
    expect(styles).toContain('[data-glow-variant="gallery-default"]');
    expect(styles).toContain("clip-path: polygon(22% 0, 78% 0, 98% 92%, 2% 92%)");
    expect(styles).toContain("background-image:");
    expect(styles).toContain("box-shadow:");
    expect(styleVocabulary).not.toMatch(
      /React\.CSSProperties|backgroundImage|(?:Webkit)?maskImage|shadow-\[|bg-\[/,
    );
  });
});
