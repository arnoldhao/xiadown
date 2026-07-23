import { describe, expect, test } from "bun:test";

describe("music browse artwork layout", () => {
  test("keeps Online cards and scroll controls on the ten-rem geometry", async () => {
    const source = await Bun.file(new URL("./ui.tsx", import.meta.url)).text();

    expect(source).toContain(
      'className="listen-muse-card group/muse-card relative w-[10rem]',
    );
    expect(source).toContain("snap-x snap-mandatory gap-4");
    expect(source).toContain(
      '"listen-horizontal-scroll-fade absolute top-0 z-30 h-[10rem] w-20"',
    );
    expect(source).toContain("!mt-7 space-y-3");
  });

  test("keeps Lofi cards on the same artwork and section geometry", async () => {
    const source = await Bun.file(
      new URL("./HushLiveList.tsx", import.meta.url),
    ).text();

    expect(source).toContain('className="space-y-7"');
    expect(source).toContain(
      'className="listen-hush-card group/hush-card relative w-[10rem]',
    );
    expect(source).toContain("snap-x snap-mandatory gap-4");
    expect(source).toContain(
      '"listen-horizontal-scroll-fade absolute top-0 z-30 h-[10rem] w-20"',
    );
  });

  test("routes square Online and Lofi cards through poster fallback candidates", async () => {
    const [onlineSource, lofiSource] = await Promise.all([
      Bun.file(new URL("./ui.tsx", import.meta.url)).text(),
      Bun.file(new URL("./HushLiveList.tsx", import.meta.url)).text(),
    ]);

    expect(onlineSource).toContain(
      ': buildListenPosterCandidates(props.httpBaseURL, {',
    );
    expect(lofiSource).toContain(
      "buildListenPosterCandidates(props.httpBaseURL, props.item)",
    );
  });
});
