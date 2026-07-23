import { describe, expect, test } from "bun:test";

describe("Welcome Dream style ownership", () => {
  test("loads the complete scene through the Dream entrypoint", async () => {
    const [source, entry, dreamWelcome] = await Promise.all([
      Bun.file(new URL("./WelcomeScreen.tsx", import.meta.url)).text(),
      Bun.file(
        new URL("../../shared/styles/dream.css", import.meta.url),
      ).text(),
      Bun.file(
        new URL("../../shared/styles/dream/welcome.css", import.meta.url),
      ).text(),
    ]);

    expect(source).not.toContain('import "./welcome.css"');
    expect(entry).toContain('@import "./dream/welcome.css";');
    expect(dreamWelcome).toContain(".welcome-screen {");
    expect(dreamWelcome).toContain("@keyframes");
    expect(dreamWelcome).toContain("@media (prefers-reduced-motion: reduce)");
  });

  test("keeps deterministic starfield appearance in Dream CSS", async () => {
    const [source, dreamWelcome] = await Promise.all([
      Bun.file(new URL("./WelcomeScreen.tsx", import.meta.url)).text(),
      Bun.file(
        new URL("../../shared/styles/dream/welcome.css", import.meta.url),
      ).text(),
    ]);

    expect(source).toContain("const WELCOME_STAR_COUNT = 72;");
    expect(source).toContain("const WELCOME_SHOOTING_STAR_COUNT = 3;");
    expect(source).toContain("data-star-index={index}");
    expect(source).toContain("data-shooting-star-index={index}");
    expect(source).not.toContain("function welcomeNoise");
    expect(source).not.toContain("const WELCOME_STARS");
    expect(source).not.toContain("const WELCOME_SHOOTING_STARS");
    expect(source).not.toContain('"--star-max-opacity"');
    expect(source).not.toContain('"--shoot-travel-x"');

    const starMarkup = source.match(
      /<div className="welcome-stars">[\s\S]*?<div className="welcome-cloud/,
    )?.[0];
    expect(starMarkup).toBeDefined();
    expect(starMarkup).not.toContain("style=");

    const starRecipes = Array.from(
      dreamWelcome.matchAll(
        /\.welcome-stars \[data-star-index="(\d+)"\] \{([^}]+)\}/g,
      ),
    );
    expect(starRecipes).toHaveLength(72);
    expect(starRecipes.map((recipe) => Number(recipe[1]))).toEqual(
      Array.from({ length: 72 }, (_, index) => index),
    );
    for (const recipe of starRecipes) {
      expect(recipe[2]).toContain("--welcome-star-delay:");
      expect(recipe[2]).toContain("--welcome-star-duration:");
      expect(recipe[2]).toContain("--welcome-star-left:");
      expect(recipe[2]).toContain("--welcome-star-size:");
      expect(recipe[2]).toContain("--welcome-star-top:");
      expect(recipe[2]).toContain("--star-max-opacity:");
      expect(recipe[2]).toContain("--star-min-opacity:");
    }

    const shootingStarRecipes = Array.from(
      dreamWelcome.matchAll(
        /\.welcome-shooting-star\[data-shooting-star-index="(\d+)"\] \{([^}]+)\}/g,
      ),
    );
    expect(shootingStarRecipes).toHaveLength(3);
    expect(shootingStarRecipes.map((recipe) => Number(recipe[1]))).toEqual([
      0, 1, 2,
    ]);
    for (const recipe of shootingStarRecipes) {
      expect(recipe[2]).toContain("--welcome-shooting-star-delay:");
      expect(recipe[2]).toContain("--welcome-shooting-star-duration:");
      expect(recipe[2]).toContain("--welcome-shooting-star-left:");
      expect(recipe[2]).toContain("--welcome-shooting-star-top:");
      expect(recipe[2]).toContain("--welcome-shooting-star-width:");
      expect(recipe[2]).toContain("--shoot-travel-x:");
      expect(recipe[2]).toContain("--shoot-travel-y:");
    }
  });
});
