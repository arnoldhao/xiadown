import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import { ListenMuseArtistHero } from "./ArtistDetailHero";

const artistHeroFixture = {
  title: "Artist name",
  subtitle: "2M subscribers",
  description: "An artist introduction returned by YouTube Music.",
  backLabel: "Back",
  infoLabel: "Artist Info",
  biographyLabel: "Biography",
  closeLabel: "Close",
  shuffleLabel: "Shuffle Artist",
  mixLabel: "Play Mix",
  subscribeLabel: "Subscribe",
  unsubscribeLabel: "Unsubscribe",
};

describe("Online artist primary UI", () => {
  test("centers only circular artist card labels", async () => {
    const [source, appearanceCss] = await Promise.all([
      Bun.file(new URL("./ui.tsx", import.meta.url)).text(),
      Bun.file(
        new URL("../../../shared/styles/dream/listen.css", import.meta.url),
      ).text(),
    ]);

    expect(source.match(/align="center"/g)).toHaveLength(1);
    expect(source).toContain('shape="circle"');
    expect(source).toContain(
      'data-listen-card-align={centered ? "center" : "start"}',
    );
    expect(source).toContain('centered && "justify-center"');
    expect(source).not.toMatch(/\btext-(?:center|left)\b/);
    expect(appearanceCss).toContain(
      '.listen-muse-card-text[data-listen-card-align="center"]',
    );
    expect(appearanceCss).toContain("text-align: center");
    expect(appearanceCss).toContain(
      '.listen-muse-card-text[data-listen-card-align="start"]',
    );
    expect(appearanceCss).toContain("text-align: left");
  });

  test("renders a centered artist hero without leaking biography into the header", () => {
    const markup = renderToStaticMarkup(
      <ListenMuseArtistHero
        httpBaseURL="http://127.0.0.1:34115"
        title={artistHeroFixture.title}
        subtitle={artistHeroFixture.subtitle}
        description={artistHeroFixture.description}
        thumbnailUrl="https://example.test/wide-artist.jpg"
        heroThumbnailUrl="https://example.test/artist-banner.jpg"
        backLabel={artistHeroFixture.backLabel}
        infoLabel={artistHeroFixture.infoLabel}
        biographyLabel={artistHeroFixture.biographyLabel}
        closeLabel={artistHeroFixture.closeLabel}
        shuffleLabel={artistHeroFixture.shuffleLabel}
        mixLabel={artistHeroFixture.mixLabel}
        subscribeLabel={artistHeroFixture.subscribeLabel}
        unsubscribeLabel={artistHeroFixture.unsubscribeLabel}
        showActions={false}
        subscribed
        actionBusy=""
        shuffleDisabled={false}
        mixDisabled={false}
        subscribeDisabled={false}
        onBack={() => undefined}
        onShuffle={() => undefined}
        onMix={() => undefined}
        onToggleSubscription={() => undefined}
      />,
    );

    expect(markup).toContain('data-listen-artist-hero="true"');
    expect(markup).toContain("listen-muse-artist-hero__artwork");
    expect(markup).toContain("listen-muse-artist-hero__artwork-image");
    expect(markup).toContain("listen-muse-artist-hero__veil");
    expect(markup).not.toContain(artistHeroFixture.description);
    expect(markup).not.toContain("data-artist-action");
  });

  test("keeps the artist backdrop responsive and full-bleed in the workspace", async () => {
    const [layoutCss, appearanceCss, pageSource, heroSource] = await Promise.all([
      Bun.file(new URL("./listen.css", import.meta.url)).text(),
      Bun.file(
        new URL("../../../shared/styles/dream/listen.css", import.meta.url),
      ).text(),
      Bun.file(new URL("./PageView.tsx", import.meta.url)).text(),
      Bun.file(new URL("./ArtistDetailHero.tsx", import.meta.url)).text(),
    ]);
    const css = `${layoutCss}\n${appearanceCss}`;

    expect(css).toContain(".listen-muse-artist-hero__body");
    expect(css).toContain("min-height: clamp(22rem, 52cqw, 36rem)");
    expect(css).toContain("font-size: clamp(2rem, 5cqw, 3.75rem)");
    expect(css).not.toContain("min-height: clamp(22rem, 52vw, 36rem)");
    expect(css).toContain(
      "@container workspace-page-content (max-width: 720px)",
    );
    expect(css).toContain("min-height: clamp(24rem, 72cqw, 31rem)");
    expect(css).not.toContain("min-height: clamp(24rem, 72vw, 31rem)");
    expect(css).toContain("max-width: 100%");
    expect(css).toContain("object-fit: cover");
    expect(css).toContain("flex-direction: column");
    expect(css).toContain("align-items: center");
    expect(css).toContain('data-size="large"');
    expect(css).toContain(".listen-artist-info-dialog");
    expect(css).toContain("white-space: pre-wrap");
    expect(css).toContain("hsl(var(--background) / 0.92) 87%");
    expect(heroSource.match(/variant="glass"/g)).toHaveLength(2);
    expect(heroSource.match(/shape="circle"/g)).toHaveLength(2);
    expect(layoutCss).not.toMatch(/(?:-webkit-)?backdrop-filter\s*:/);
    expect(css).not.toContain(".listen-artist-info-dialog__photos");
    expect(css).toContain(
      "@container workspace-page-content (max-width: 540px)",
    );
    expect(pageSource).toContain("listen-muse-artist-detail space-y-4");
    expect(css).toContain(".listen-muse-artist-detail");
    expect(css).toContain(
      "calc(-1 * var(--app-workspace-page-content-padding-block))",
    );
    expect(css).toContain("calc(-1 * var(--app-page-gutter))");
    expect(appearanceCss).toMatch(
      /\.listen-workspace-page\[data-page-recipe="detail"\][\s\S]*?\.listen-muse-artist-hero__body\s*\{[^}]*border-radius:\s*var\(--app-radius-none\)/s,
    );
  });

  test("keeps biography beneath one fading hero image in the info dialog", async () => {
    const [dialogSource, heroSource] = await Promise.all([
      Bun.file(new URL("./ArtistInfoDialog.tsx", import.meta.url)).text(),
      Bun.file(new URL("./ArtistDetailHero.tsx", import.meta.url)).text(),
    ]);

    expect(dialogSource).toContain("<DialogTrigger asChild>");
    expect(dialogSource).toContain("<DialogTitle");
    expect(dialogSource).toContain("<DialogDescription");
    expect(dialogSource).toContain("props.biographyLabel");
    expect(dialogSource).toContain("buildListenImageCandidates");
    expect(dialogSource).toContain("showCloseButton={false}");
    expect(dialogSource).not.toContain("props.photosLabel");
    expect(dialogSource).not.toContain("props.openPageLabel");

    expect(dialogSource).toContain('data-artist-action="info"');
    expect(heroSource).toContain('action="shuffle"');
    expect(heroSource).toContain('action="mix"');
    expect(heroSource).toContain('action="subscribe"');
    expect(heroSource).toContain('size="large"');
    expect(heroSource).toContain('size="small"');
    expect(heroSource).toContain("aria-pressed={props.active}");
    expect(heroSource).toContain("aria-disabled={props.disabled}");
    expect(heroSource).toContain(
      "onClick={props.disabled ? undefined : props.onClick}",
    );
    expect(heroSource).not.toContain("\n          disabled={props.disabled}");
    expect(heroSource).toContain('side="bottom" sideOffset={10}');
    expect(dialogSource).toContain('side="bottom" sideOffset={10}');
    expect(dialogSource).toContain('<DialogDescription className="sr-only">');
    const actionIndexes = [
      heroSource.indexOf("<ListenArtistInfoDialog"),
      heroSource.indexOf('action="shuffle"'),
      heroSource.indexOf('action="mix"'),
      heroSource.indexOf('action="subscribe"'),
    ];
    expect(actionIndexes.every((index) => index >= 0)).toBeTrue();
    expect(actionIndexes).toEqual(
      [...actionIndexes].sort((left, right) => left - right),
    );
  });
});
