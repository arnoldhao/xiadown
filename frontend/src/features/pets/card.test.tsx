import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import type { Pet } from "@/shared/contracts/pets";

import { LocalPetGalleryCard } from "./card";

const BUILTIN_PET: Pet = {
  id: "builtin-dream-cat",
  displayName: "Dream Cat",
  description: "A built-in pet",
  frameCount: 8,
  columns: 8,
  rows: 9,
  cellWidth: 32,
  cellHeight: 32,
  spritesheetFile: "dream-cat.png",
  spritesheetPath: "/pets/dream-cat.png",
  scope: "builtin",
  status: "ready",
  imageWidth: 256,
  imageHeight: 288,
  createdAt: "2026-07-19T00:00:00.000Z",
  updatedAt: "2026-07-19T00:00:00.000Z",
};

function renderCard(pet: Pet, isDefault = false) {
  return renderToStaticMarkup(
    <LocalPetGalleryCard
      pet={pet}
      imageUrl={pet.spritesheetPath}
      isDefault={isDefault}
      aria-label={`Select ${pet.displayName}`}
    />,
  );
}

describe("LocalPetGalleryCard", () => {
  test("preserves the button semantics while publishing stable Dream variants", () => {
    const markup = renderCard(BUILTIN_PET, true);

    expect(markup).toStartWith("<button");
    expect(markup).toContain('type="button"');
    expect(markup).toContain('aria-label="Select Dream Cat"');
    expect(markup).toContain('data-default="true"');
    expect(markup).toContain('data-lighting="default"');
    expect(markup).toContain("app-pets-gallery-card-light--primary");
    expect(markup).toContain("app-pets-gallery-card-light--directional");
    expect(markup).toContain("app-pets-gallery-card-light--rim");
    expect(markup).toContain("app-pets-gallery-card-light--spotlight");
    expect(markup).toContain('data-glow-variant="gallery-default"');
    expect(markup).toContain("app-pets-gallery-card-label");
    expect(markup).toContain("Dream Cat");
    expect(markup).not.toMatch(/(?:shadow-|bg-)\[/);
    expect(markup).not.toMatch(/(?:background|mask-image|box-shadow)\s*:/);
  });

  test("selects online and imported lighting without changing card anatomy", () => {
    const onlineMarkup = renderCard({
      ...BUILTIN_PET,
      id: "online-dream-cat",
      origin: "online",
    });
    const importedMarkup = renderCard({
      ...BUILTIN_PET,
      id: "imported-dream-cat",
      scope: "imported",
    });

    expect(onlineMarkup).toContain('data-lighting="online"');
    expect(importedMarkup).toContain('data-lighting="imported"');
    for (const markup of [onlineMarkup, importedMarkup]) {
      expect(markup).toStartWith("<button");
      expect(markup).toContain("app-pets-gallery-card-light--primary");
      expect(markup).toContain("app-pets-gallery-card-light--directional");
      expect(markup).toContain("app-pets-gallery-card-light--rim");
      expect(markup).not.toContain("app-pets-gallery-card-light--spotlight");
      expect(markup).not.toContain("data-glow-variant");
    }
  });
});
