import { describe, expect, test } from "bun:test";
import postcss, { type Declaration, type Root } from "postcss";
import { renderToStaticMarkup } from "react-dom/server";

import { getXiaText } from "@/features/xiadown/shared";

import { ListenPlayerTransport } from "./playback-controls";

const text = getXiaText("en");

function declarationsFor(root: Root, selector: string) {
  const declarations = new Map<string, Declaration>();
  root.walkRules((rule) => {
    if (rule.selector !== selector) {
      return;
    }
    rule.walkDecls((declaration) => {
      declarations.set(declaration.prop, declaration);
    });
  });
  return declarations;
}

function expectCustomProperty(
  root: Root,
  selector: string,
  property: string,
  value: string,
) {
  expect(declarationsFor(root, selector).get(property)?.value).toBe(value);
}

describe("Now Playing control sizing", () => {
  test("keeps mode, skip, and primary transport controls in three size tiers", () => {
    const markup = renderToStaticMarkup(
      <ListenPlayerTransport
        playing={false}
        loading={false}
        playMode="order"
        text={text}
        onPrevious={() => undefined}
        onNext={() => undefined}
        onPlayModeChange={() => undefined}
        onTogglePlayback={() => undefined}
      />,
    );
    const buttons = markup.match(/<button[^>]*>/g) ?? [];
    const transportButtons = buttons.filter((button) =>
      button.includes("listen-transport-icon-button"),
    );
    const primaryButton = buttons.find((button) =>
      button.includes("listen-primary-play-button"),
    );

    expect(buttons).toHaveLength(5);
    expect(transportButtons).toHaveLength(4);
    expect(
      transportButtons.filter((button) =>
        button.includes('data-transport-size="small"'),
      ),
    ).toHaveLength(2);
    expect(
      transportButtons.filter((button) =>
        button.includes('data-transport-size="normal"'),
      ),
    ).toHaveLength(2);
    expect(primaryButton).toContain("listen-primary-play-button--medium");
    expect(markup).toContain("listen-primary-play-icon--medium");
  });

  test("publishes the 32/40/48 transport hierarchy through Dream variables", async () => {
    const root = postcss.parse(
      await Bun.file(
        new URL("../../../shared/styles/dream/listen.css", import.meta.url),
      ).text(),
    );
    const transportSmall =
      ':root .app-dream-button.listen-transport-icon-button[data-app-button][data-size][data-transport-size="small"]';
    const transportNormal =
      ':root .app-dream-button.listen-transport-icon-button[data-app-button][data-size][data-transport-size="normal"]';

    expectCustomProperty(
      root,
      ".listen-player-icon-button",
      "--app-button-inline-size",
      "2.5rem",
    );
    expectCustomProperty(
      root,
      ".listen-player-icon-button",
      "--app-button-block-size",
      "2.5rem",
    );
    expect(declarationsFor(root, ".listen-player-icon-button").get("border-radius")?.value).toBe(
      "50%",
    );
    expectCustomProperty(
      root,
      transportSmall,
      "--app-button-inline-size",
      "2rem",
    );
    expectCustomProperty(
      root,
      transportNormal,
      "--app-button-inline-size",
      "2.5rem",
    );
    expectCustomProperty(
      root,
      transportNormal,
      "--app-button-icon-size",
      "1.25rem",
    );

    for (const [size, buttonSize, iconSize] of [
      ["small", "2.5rem", "1rem"],
      ["medium", "3rem", "1.25rem"],
      ["large", "3.5rem", "1.5rem"],
    ] as const) {
      expectCustomProperty(
        root,
        `.listen-primary-play-button--${size}`,
        "--app-button-inline-size",
        buttonSize,
      );
      expectCustomProperty(
        root,
        `.listen-primary-play-button--${size}`,
        "--app-button-block-size",
        buttonSize,
      );
      expectCustomProperty(
        root,
        `.listen-primary-play-icon--${size}`,
        "--app-button-icon-size",
        iconSize,
      );
    }
    expect(declarationsFor(root, ".listen-primary-play-button").get("border-radius")?.value).toBe(
      "50%",
    );
    expect(declarationsFor(root, transportNormal).get("border-radius")?.value).toBeUndefined();
    expect(
      declarationsFor(
        root,
        ":root .app-dream-button.listen-transport-icon-button[data-app-button][data-size]",
      ).get("border-radius")?.value,
    ).toBe("50%");
  });

  test("connects semantic player sizes to the shared Button contract", async () => {
    const [buttonRoot, anatomyRoot] = await Promise.all([
      Bun.file(
        new URL(
          "../../../shared/styles/dream/button-contract.css",
          import.meta.url,
        ),
      )
        .text()
        .then((source) => postcss.parse(source)),
      Bun.file(
        new URL("../../../shared/styles/dream/anatomy.css", import.meta.url),
      )
        .text()
        .then((source) => postcss.parse(source)),
    ]);
    const iconButton = declarationsFor(
      buttonRoot,
      ':root .app-dream-button.app-motion-surface[data-app-button][data-size="icon"]',
    );
    const icon = declarationsFor(anatomyRoot, ".app-base-button > svg");

    expect(
      iconButton.get("inline-size")?.value ?? iconButton.get("width")?.value,
    ).toContain(
      "--app-button-inline-size",
    );
    expect(
      iconButton.get("block-size")?.value ?? iconButton.get("height")?.value,
    ).toContain(
      "--app-button-block-size",
    );
    expect(iconButton.get("padding")?.value).toContain(
      "--app-button-padding",
    );
    expect(icon.get("width")?.value).toContain("--app-button-icon-size");
    expect(icon.get("height")?.value).toContain("--app-button-icon-size");
  });

  test("keeps compact volume and native-video controls scoped", async () => {
    const [listenRoot, componentsRoot] = await Promise.all([
      Bun.file(
        new URL("../../../shared/styles/dream/listen.css", import.meta.url),
      )
        .text()
        .then((source) => postcss.parse(source)),
      Bun.file(
        new URL("../../../shared/styles/dream/components.css", import.meta.url),
      )
        .text()
        .then((source) => postcss.parse(source)),
    ]);
    const volumeSelector =
      ":root .app-dream-button.listen-player-volume__button[data-app-button][data-size]";

    expectCustomProperty(
      listenRoot,
      volumeSelector,
      "--app-button-inline-size",
      "2rem",
    );
    expectCustomProperty(
      listenRoot,
      volumeSelector,
      "--app-button-icon-size",
      "1rem",
    );

    const videoAction = declarationsFor(
      componentsRoot,
      ".listen-video-action-button",
    );
    const videoPrimary = declarationsFor(
      componentsRoot,
      ".listen-video-action-button-primary",
    );
    const videoExpand = declarationsFor(
      componentsRoot,
      ".listen-video-expand-button",
    );
    expect(videoAction.get("width")?.value).toBe("36px");
    expect(videoAction.get("width")?.important).toBe(true);
    expect(videoPrimary.get("width")?.value).toBe("40px");
    expect(videoPrimary.get("width")?.important).toBe(true);
    expect(videoExpand.get("width")?.value).toBe("36px");
    expect(videoExpand.get("width")?.important).toBe(true);
  });
});
