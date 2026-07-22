import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import { TaskFolderArtwork } from "./TaskFolderArtwork";
import {
  LIBRARY_PAPER_GEOMETRY,
  LIBRARY_PAPER_NOTCH_POSITIONS,
  PLACEHOLDER_PAPER_HORIZONTAL_NOTCH_OFFSETS,
  TASK_FOLDER_PAPER_PLACEMENT,
  TASK_FOLDER_PAPER_TRANSFORM,
} from "./library-paper-geometry";
import type { LibraryTaskPreviewItem } from "./types";

const previewItems: LibraryTaskPreviewItem[] = [
  {
    id: "cover",
    kind: "thumbnail",
    previewURL: "http://127.0.0.1:43127/assets/cover.jpg",
    label: "JPG",
  },
  { id: "audio", kind: "transcode", label: "AAC M4A" },
  { id: "subtitle", kind: "subtitle", label: "VTT" },
  { id: "archive", kind: "archive", label: "ZIP" },
];

describe("TaskFolderArtwork", () => {
  test("renders the measured closed archive layers with one frosted preview", () => {
    const markup = renderToStaticMarkup(
      <TaskFolderArtwork items={previewItems} totalCount={5} view="grid" />,
    );
    const back = markup.indexOf("app-library-task-folder__back");
    const contents = markup.indexOf("app-library-task-folder__contents");
    const front = markup.indexOf("app-library-task-folder__front");

    expect(markup).toContain('class="app-library-task-folder"');
    expect(markup).toContain('data-task-folder-artwork="true"');
    expect(markup).toContain('data-view="grid"');
    expect(markup).toContain('data-preview-count="3"');
    expect(markup).toContain('data-total-count="5"');
    expect(markup).toContain('aria-hidden="true"');
    expect(back).toBeGreaterThan(0);
    expect(contents).toBeGreaterThan(back);
    expect(front).toBeGreaterThan(contents);
    expect(markup.match(/class="app-library-task-folder__page"/g)).toHaveLength(2);
    expect(markup).not.toContain("app-library-task-folder__peek");
    expect(markup).toContain('href="http://127.0.0.1:43127/assets/cover.jpg"');
    expect(markup.match(/http:\/\/127\.0\.0\.1:43127\/assets\/cover\.jpg/g)).toHaveLength(1);
    expect(markup).toContain("lucide-music-2");
    expect(markup.match(/<filter /g)).toHaveLength(1);
    expect(markup.match(/<mask /g)).toHaveLength(1);
    expect(markup.match(/<feGaussianBlur /g)).toHaveLength(1);
    expect(markup.match(/<image /g)).toHaveLength(1);
    expect(markup).toContain('viewBox="0 0 100 120"');
    expect(markup).toContain(`transform="${TASK_FOLDER_PAPER_TRANSFORM}"`);
    expect(markup.match(/<circle[^>]*cx="0"/g)).toHaveLength(
      LIBRARY_PAPER_NOTCH_POSITIONS.length,
    );
    expect(markup).toContain(`width="${LIBRARY_PAPER_GEOMETRY.width}"`);
    expect(markup).toContain(`height="${LIBRARY_PAPER_GEOMETRY.height}"`);
    expect(markup).toContain('width="94.5"');
    expect(markup).toContain('height="120"');
    expect(markup).toContain('result="underCover"');
    expect(markup).toContain('result="clearReveal"');
    expect(Number(markup.match(/stdDeviation="([^"]+)"/)?.[1])).toBeGreaterThan(0);
    expect(markup).not.toContain("app-library-task-folder__tab");
    expect(markup).not.toContain("app-library-task-folder__overflow");
    expect(markup).not.toContain("app-library-task-folder__front-highlight");
    expect(markup).not.toContain("<button");
    expect(markup).not.toContain("<a");
    expect(markup).not.toContain("tabindex=");
    expect(markup).toContain('focusable="false"');
  });

  test("puts a real image before default pages when List exposes one file edge", () => {
    const markup = renderToStaticMarkup(
      <TaskFolderArtwork
        items={[
          previewItems[1]!,
          previewItems[2]!,
          previewItems[0]!,
          previewItems[3]!,
        ]}
        totalCount={5}
        view="list"
      />,
    );

    expect(markup).toContain('data-view="list"');
    expect(markup).toContain('data-preview-count="1"');
    expect(markup).toContain('data-total-count="5"');
    expect(markup.match(/class="app-library-task-folder__page"/g) ?? []).toHaveLength(0);
    expect(markup).toContain('href="http://127.0.0.1:43127/assets/cover.jpg"');
    expect(markup).not.toContain("lucide-music-2");
    expect(markup).not.toContain("app-library-task-folder__overflow");
  });

  test("keeps the smaller paper below the folder bottom while exposing its right corner", () => {
    const angle = TASK_FOLDER_PAPER_PLACEMENT.rotation * Math.PI / 180;
    const rightTopX = TASK_FOLDER_PAPER_PLACEMENT.translateX;
    const rightBottomX = rightTopX - Math.sin(angle) * LIBRARY_PAPER_GEOMETRY.height;
    const rightBottomY = TASK_FOLDER_PAPER_PLACEMENT.translateY +
      Math.cos(angle) * LIBRARY_PAPER_GEOMETRY.height;

    expect(LIBRARY_PAPER_GEOMETRY.width).toBeLessThan(65);
    expect(LIBRARY_PAPER_GEOMETRY.height).toBeLessThan(98.4);
    expect(rightTopX).toBeGreaterThan(94.5);
    expect(rightBottomX).toBeLessThan(94.5);
    expect(rightBottomY).toBeLessThan(120);
  });

  test("renders an empty opaque folder without a dangling frost reference", () => {
    const markup = renderToStaticMarkup(
      <TaskFolderArtwork items={[]} totalCount={0} view="grid" />,
    );

    expect(markup).toContain('data-preview-count="0"');
    expect(markup).toContain('data-total-count="0"');
    expect(markup).toContain("app-library-task-folder__page--empty");
    expect(markup).not.toContain("app-library-task-folder__peek");
    expect(markup).not.toContain("app-library-task-folder__unified-preview");
    expect(markup).not.toContain("<filter");
    expect(markup).not.toContain("url(#");
    expect(markup).not.toContain("<image");
  });

  test("opens sideways in Companion with at most two complete postage stamps", () => {
    const markup = renderToStaticMarkup(
      <TaskFolderArtwork
        items={previewItems}
        presentation="companion-open"
        totalCount={5}
      />,
    );

    expect(markup).toContain('data-presentation="companion-open"');
    expect(markup).toContain('data-preview-count="2"');
    expect(markup).toContain('data-total-count="5"');
    expect(markup.match(/class="app-library-task-folder__page app-library-task-folder__page--/g))
      .toHaveLength(2);
    expect(markup).toContain("app-library-task-folder__page-image");
    expect(markup.match(/data-stamp-recipe="library-paper"/g)).toHaveLength(2);
    expect(markup.match(/class="app-library-task-folder__page-stamp"/g)).toHaveLength(2);
    expect(markup).toContain('data-placement="inside"');
    expect(markup).toContain('data-placement="outside"');
    expect(markup).toContain(
      'app-library-task-folder__page--outside" data-placement="outside" data-preview-kind="thumbnail" data-preview-source="asset"',
    );
    expect(markup).toContain(
      'app-library-task-folder__page--inside" data-placement="inside" data-preview-kind="transcode" data-preview-source="fallback"',
    );
    expect(markup.match(/<mask /g)).toHaveLength(2);
    expect(markup.match(/class="app-library-artwork__placeholder-paper-face /g))
      .toHaveLength(2);
    expect(markup.match(/class="app-library-artwork__placeholder-paper-border /g))
      .toHaveLength(2);
    expect(markup.match(/<circle[^>]+fill="black"/g)).toHaveLength(
      2 * (
        LIBRARY_PAPER_NOTCH_POSITIONS.length * 2 +
        PLACEHOLDER_PAPER_HORIZONTAL_NOTCH_OFFSETS.length * 2
      ),
    );
    expect(markup).not.toContain("app-library-task-folder__unified-preview");
    expect(markup).not.toContain("<filter");
  });

  test("never promotes default artwork ahead of a real outside preview", () => {
    const realPreviewURL = "http://127.0.0.1:43127/assets/real-preview.webp";
    const markup = renderToStaticMarkup(
      <TaskFolderArtwork
        items={[
          {
            id: "default",
            kind: "audio",
            previewURL: "xiadown-library-default:audio",
          },
          { id: "real", kind: "thumbnail", previewURL: realPreviewURL },
        ]}
        presentation="companion-open"
      />,
    );

    expect(markup).toContain(
      'app-library-task-folder__page--outside" data-placement="outside" data-preview-kind="thumbnail" data-preview-source="asset"',
    );
    expect(markup).toContain(`href="${realPreviewURL}"`);
    expect(markup).not.toContain('href="xiadown-library-default:audio"');
    expect(markup).toContain(
      'app-library-task-folder__page--inside" data-placement="inside" data-preview-kind="audio" data-preview-source="fallback"',
    );
  });

  test("omits the outside slot when only default/type pages are available", () => {
    const markup = renderToStaticMarkup(
      <TaskFolderArtwork
        items={[
          { id: "audio", kind: "audio" },
          {
            id: "default",
            kind: "image",
            previewURL: "xiadown-library-default:image",
          },
        ]}
        presentation="companion-open"
      />,
    );

    expect(markup).toContain('data-placement="inside"');
    expect(markup).toContain('data-preview-source="fallback"');
    expect(markup).not.toContain('data-placement="outside"');
    expect(markup).not.toContain("app-library-task-folder__page-image");
  });

  test("uses format evidence for transcoded video and audio type pages", () => {
    const markup = renderToStaticMarkup(
      <TaskFolderArtwork
        items={[
          { id: "video", kind: "transcode", label: "MP4" },
          { id: "audio", kind: "transcode", label: "FLAC" },
        ]}
        totalCount={2}
      />,
    );

    expect(markup).toContain("lucide-video");
    expect(markup).toContain("lucide-music-2");
    expect(markup).not.toContain("<image");
  });

  test("creates SSR-safe filter and sheet-mask IDs for repeated instances", () => {
    const markup = renderToStaticMarkup(
      <>
        <TaskFolderArtwork items={[previewItems[0]!]} />
        <TaskFolderArtwork items={[previewItems[0]!]} />
      </>,
    );
    const ids = [...markup.matchAll(/<filter id="([^"]+)"/g)].map((match) => match[1]!);
    const references = [...markup.matchAll(/filter="url\(#([^\)]+)\)"/g)].map(
      (match) => match[1]!,
    );
    const maskIds = [...markup.matchAll(/<mask id="([^"]+)"/g)].map(
      (match) => match[1]!,
    );
    const maskReferences = [...markup.matchAll(/mask="url\(#([^\)]+)\)"/g)].map(
      (match) => match[1]!,
    );

    expect(ids).toHaveLength(2);
    expect(new Set(ids).size).toBe(2);
    expect(references).toHaveLength(2);
    expect(references.every((id) => ids.includes(id))).toBe(true);
    expect(ids.every((id) => /^app-task-folder-frost-[a-zA-Z0-9_-]+$/.test(id))).toBe(true);
    expect(maskIds).toHaveLength(2);
    expect(new Set(maskIds).size).toBe(2);
    expect(maskReferences.every((id) => maskIds.includes(id))).toBe(true);
    expect(maskIds.every((id) => /^app-task-folder-sheet-[a-zA-Z0-9_-]+$/.test(id))).toBe(true);
  });

  test("locks the folder's physical opening model and accessibility fallbacks in CSS", async () => {
    const [layoutCss, appearanceCss] = await Promise.all([
      Bun.file(new URL("./library.css", import.meta.url)).text(),
      Bun.file(
        new URL("../../shared/styles/dream/library.css", import.meta.url),
      ).text(),
    ]);
    const layoutStart = layoutCss.indexOf(".app-library-task-folder {");
    const layoutEnd = layoutCss.indexOf(
      ".app-library-artwork--placeholder",
      layoutStart,
    );
    const materialStart = appearanceCss.indexOf(":where(");
    const appearanceStart = appearanceCss.indexOf(
      ".app-library-task-folder {",
      materialStart,
    );
    const appearanceEnd = appearanceCss.indexOf(
      ".app-library-artwork--placeholder",
      appearanceStart,
    );
    const taskCardStart = layoutCss.indexOf(
      '.app-library-item[data-category="task"] .app-library-item__artwork',
    );
    const sharedCardStart = layoutCss.indexOf(".app-library-item__artwork {");
    const taskCardLayout = layoutCss.slice(taskCardStart, layoutStart);
    const sharedCardLayout = layoutCss.slice(sharedCardStart, taskCardStart);
    const materialAppearance = appearanceCss.slice(
      materialStart,
      appearanceStart,
    );
    const folderLayout = layoutCss.slice(layoutStart, layoutEnd);
    const folderAppearance = appearanceCss.slice(
      appearanceStart,
      appearanceEnd,
    );

    expect(sharedCardStart).toBeGreaterThan(0);
    expect(taskCardStart).toBeGreaterThan(0);
    expect(layoutStart).toBeGreaterThan(0);
    expect(layoutEnd).toBeGreaterThan(layoutStart);
    expect(appearanceStart).toBeGreaterThan(0);
    expect(appearanceEnd).toBeGreaterThan(appearanceStart);
    expect(sharedCardLayout).toContain("aspect-ratio: 5 / 6");
    expect(sharedCardLayout).toContain("width: min(84%, 10.75rem)");
    expect(taskCardLayout).toContain("height: auto");
    expect(taskCardLayout).toContain("pointer-events: none");
    expect(taskCardLayout).not.toMatch(/\b(?:background|box-shadow)\s*:/);
    expect(materialAppearance).toContain("--app-library-material-face");
    expect(materialAppearance).toContain("color-mix(");
    expect(materialAppearance).toContain(
      "hsl(var(--app-accent-brand, var(--primary)))",
    );
    expect(materialAppearance).toContain("hsl(var(--card))");
    expect(materialAppearance).toContain("hsl(var(--muted))");
    expect(materialAppearance).not.toContain("hsl(var(--secondary))");
    expect(folderLayout).not.toContain("--app-accent-brand");
    expect(folderLayout).not.toContain("hsl(var(--primary)");
    expect(folderLayout).toContain(
      "--app-library-task-folder-edge-reveal: 5.6%",
    );
    expect(folderLayout).toContain("width: 94.5%");
    expect(folderLayout).toContain("height: 100%");
    expect(folderAppearance).toContain("background:");
    expect(folderAppearance).toContain("box-shadow:");
    expect(folderAppearance).not.toContain("app-library-task-folder__front-highlight");
    expect(folderAppearance).not.toContain("app-library-task-folder__tab");
    expect(folderAppearance).not.toContain("app-library-task-folder__overflow");
    expect(folderAppearance).toContain('[data-surface-style="contrast"]');
    expect(folderAppearance).toContain('[data-reduce-transparency="true"]');
    expect(folderAppearance).toContain("prefers-reduced-transparency");
    expect(folderLayout).toContain(".app-library-task-folder__unified-preview");
    expect(folderLayout).toContain(".app-library-task-folder__page-stamp");
    expect(folderLayout).toContain(
      "--app-library-task-folder-open-cover-angle: 40deg",
    );
    expect(folderLayout).toContain(
      "--app-library-task-folder-open-perspective: 29rem",
    );
    expect(folderLayout).toMatch(
      /\[data-presentation="companion-open"\]\s*\{[^}]*perspective:\s*var\(--app-library-task-folder-open-perspective\)[^}]*perspective-origin:\s*var\(--app-library-task-folder-open-shell-left\) 50%/s,
    );
    expect(folderLayout).toMatch(
      /\[data-presentation="companion-open"\][\s\S]*?\.app-library-task-folder__front\s*\{[^}]*transform:\s*rotateY\(var\(--app-library-task-folder-open-cover-angle\)\)[^}]*transform-origin:\s*100% 50%[^}]*transform-style:\s*preserve-3d/s,
    );
    expect(folderAppearance).toMatch(
      /\[data-presentation="companion-open"\][\s\S]*?\.app-library-task-folder__front\s*\{[^}]*transform:\s*rotateY\(var\(--app-library-task-folder-open-cover-angle\)\)\s*scaleX\(var\(--app-library-task-folder-open-cover-visual-scale\)\)/s,
    );
    expect(folderLayout).toMatch(
      /\.app-library-task-folder__front-cover,[\s\S]*?\.app-library-task-folder__front-film\s*\{[^}]*inset:\s*0[^}]*width:\s*100%[^}]*height:\s*100%[^}]*backface-visibility:\s*hidden[^}]*transform:\s*none/s,
    );
    expect(folderLayout).not.toContain("skewY(");
    expect(folderLayout).toContain(
      "--app-library-task-folder-open-shell-width: 23.5%",
    );
    expect(folderLayout).toContain(
      "--app-library-task-folder-open-shell-height: 84%",
    );
    expect(folderLayout.match(
      /width:\s*var\(--app-library-task-folder-open-shell-width\)/g,
    )).toHaveLength(2);
    expect(folderLayout.match(
      /height:\s*var\(--app-library-task-folder-open-shell-height\)/g,
    )).toHaveLength(2);
    const percentVariable = (name: string) => Number(
      folderLayout.match(new RegExp(`${name}:\\s*(-?[\\d.]+)%`))?.[1],
    );
    const shellLeft = percentVariable("--app-library-task-folder-open-shell-left");
    const shellWidth = percentVariable("--app-library-task-folder-open-shell-width");
    const shellTop = percentVariable("--app-library-task-folder-open-shell-top");
    const shellHeight = percentVariable("--app-library-task-folder-open-shell-height");
    const stampWidth = percentVariable("--app-library-task-folder-open-stamp-width");
    const insideLeft = percentVariable("--app-library-task-folder-open-inside-left");
    const outsideLeft = percentVariable("--app-library-task-folder-open-outside-left");
    const coverAngle = Number(folderLayout.match(
      /--app-library-task-folder-open-cover-angle:\s*([\d.]+)deg/,
    )?.[1]);
    const perspectiveRem = Number(folderLayout.match(
      /--app-library-task-folder-open-perspective:\s*([\d.]+)rem/,
    )?.[1]);
    const coverVisualScale = Number(folderAppearance.match(
      /--app-library-task-folder-open-cover-visual-scale:\s*([\d.]+)/,
    )?.[1]);
    const frontLeft = shellLeft - shellWidth;
    const frontHinge = frontLeft + shellWidth;
    const shellRight = shellLeft + shellWidth;
    const stampViewBoxWidth = LIBRARY_PAPER_GEOMETRY.width + 4;
    const stampPaperMargin = 2 / stampViewBoxWidth * stampWidth;
    const visibleStampWidth = LIBRARY_PAPER_GEOMETRY.width / stampViewBoxWidth * stampWidth;
    const insidePaperRight = insideLeft + stampWidth - stampPaperMargin;
    const visibleInsideOverhangRatio =
      (insidePaperRight - shellRight) / visibleStampWidth;
    const outsidePaperLeft = outsideLeft + stampPaperMargin;
    const outsidePaperRight = outsideLeft + stampWidth - stampPaperMargin;
    const outsideBoxRight = outsideLeft + stampWidth;
    const stampToShellWidthRatio = stampWidth / shellWidth;
    const visibleStampGapRatio =
      (outsidePaperLeft - insidePaperRight) / visibleStampWidth;

    // At the measured 300x168.75 Companion artwork contract, a positive Y rotation
    // around the cover's right hinge brings its free edge toward the viewer.
    // The free edge must therefore be taller than the hinge while remaining
    // fully inside the artwork bounds, matching a real V-shaped open folder.
    const artworkWidth = 300;
    const artworkHeight = 168.75;
    const rootFontSize = 16;
    const coverWidthPx = artworkWidth * shellWidth / 100;
    const coverHeightPx = artworkHeight * shellHeight / 100;
    const scaledCoverWidthPx = coverWidthPx * coverVisualScale;
    const coverAngleRadians = coverAngle * Math.PI / 180;
    const perspectivePx = perspectiveRem * rootFontSize;
    const freeEdgeDepthPx = scaledCoverWidthPx * Math.sin(coverAngleRadians);
    const freeEdgeScale = perspectivePx / (perspectivePx - freeEdgeDepthPx);
    const projectedCoverWidthPx =
      scaledCoverWidthPx * Math.cos(coverAngleRadians) * freeEdgeScale;
    const projectedFrontLeft = shellLeft - projectedCoverWidthPx / artworkWidth * 100;
    const hingeTopPx = artworkHeight * shellTop / 100;
    const hingeBottomPx = hingeTopPx + coverHeightPx;
    const perspectiveCenterYPx = artworkHeight / 2;
    const freeEdgeTopPx = perspectiveCenterYPx +
      (hingeTopPx - perspectiveCenterYPx) * freeEdgeScale;
    const freeEdgeBottomPx = perspectiveCenterYPx +
      (hingeBottomPx - perspectiveCenterYPx) * freeEdgeScale;
    const firstExteriorShadow = (rules: string) => {
      const match = rules.match(
        /box-shadow:\s*(-?[\d.]+)px\s+(-?[\d.]+)px\s+([\d.]+)px\s+(-?[\d.]+)px/,
      );
      return {
        x: Number(match?.[1]),
        y: Number(match?.[2]),
        blur: Number(match?.[3]),
        spread: Number(match?.[4]),
      };
    };
    const companionBackMaterial = [...folderAppearance.matchAll(
      /\.app-library-task-folder\[data-presentation="companion-open"\]\s*\.app-library-task-folder__back\s*\{([^}]*)\}/g,
    )].map((match) => match[1]!).find((rules) => rules.includes("box-shadow:")) ?? "";
    const companionFrontMaterial = folderAppearance.match(
      /:where\(\.app-library-task-folder\[data-presentation="companion-open"\]\)\s*\.app-library-task-folder__front-cover\s*\{([^}]*)\}/,
    )?.[1] ?? "";
    const backShadow = firstExteriorShadow(companionBackMaterial);
    const frontShadow = firstExteriorShadow(companionFrontMaterial);
    const frontShadowFreeEdgePx = Math.max(
      0,
      -frontShadow.x + frontShadow.blur + frontShadow.spread,
    );
    const frontPaintLocalWidthPx =
      (coverWidthPx + frontShadowFreeEdgePx) * coverVisualScale;
    const frontPaintDepthPx = frontPaintLocalWidthPx * Math.sin(coverAngleRadians);
    const frontPaintScale = perspectivePx / (perspectivePx - frontPaintDepthPx);
    const frontPaintWidthPx =
      frontPaintLocalWidthPx * Math.cos(coverAngleRadians) * frontPaintScale;
    const backShadowLeftPx = Math.max(
      0,
      -backShadow.x + backShadow.blur + backShadow.spread,
    );
    const backShadowRightPx = Math.max(
      0,
      backShadow.x + backShadow.blur + backShadow.spread,
    );
    const backSpineLeftPx = 2.5;
    const backPaintWidthPx =
      coverWidthPx + backSpineLeftPx + backShadowLeftPx + backShadowRightPx;
    const symmetricBackRevealPx = (coverWidthPx - frontPaintWidthPx) / 2;
    const frontPaintLeftPx = artworkWidth * shellLeft / 100 - frontPaintWidthPx;
    const backPaintRightPx =
      artworkWidth * shellRight / 100 + backShadowRightPx;
    const companionFrameWidthPx = 390;
    const artworkFrameInsetPx = (companionFrameWidthPx - artworkWidth) / 2;

    expect(visibleInsideOverhangRatio).toBeGreaterThanOrEqual(0.22);
    expect(visibleInsideOverhangRatio).toBeLessThanOrEqual(0.25);
    expect(outsidePaperLeft).toBeGreaterThan(insidePaperRight);
    expect(stampToShellWidthRatio).toBeGreaterThanOrEqual(1.2);
    expect(stampToShellWidthRatio).toBeLessThanOrEqual(1.4);
    expect(visibleStampGapRatio).toBeGreaterThanOrEqual(0.07);
    expect(visibleStampGapRatio).toBeLessThanOrEqual(0.12);
    expect(coverAngle).toBeGreaterThan(0);
    expect(coverAngle).toBeLessThan(90);
    expect(coverVisualScale).toBeGreaterThanOrEqual(0.92);
    expect(coverVisualScale).toBeLessThanOrEqual(0.95);
    expect(frontHinge).toBeCloseTo(shellLeft, 8);
    expect(freeEdgeScale).toBeGreaterThanOrEqual(1.06);
    expect(freeEdgeScale).toBeLessThanOrEqual(1.14);
    expect(projectedCoverWidthPx / coverWidthPx).toBeGreaterThanOrEqual(0.77);
    expect(projectedCoverWidthPx / coverWidthPx).toBeLessThanOrEqual(0.81);
    expect(frontPaintWidthPx).toBeLessThan(coverWidthPx);
    expect(backPaintWidthPx).toBeGreaterThan(coverWidthPx);
    expect(symmetricBackRevealPx).toBeGreaterThanOrEqual(3);
    expect(artworkFrameInsetPx + frontPaintLeftPx).toBeGreaterThanOrEqual(0);
    expect(artworkFrameInsetPx + backPaintRightPx)
      .toBeLessThanOrEqual(companionFrameWidthPx);
    expect(projectedFrontLeft).toBeGreaterThanOrEqual(0);
    expect(freeEdgeTopPx).toBeGreaterThanOrEqual(0);
    expect(freeEdgeBottomPx).toBeLessThanOrEqual(artworkHeight);
    expect(freeEdgeBottomPx - freeEdgeTopPx).toBeGreaterThan(coverHeightPx);
    expect(frontLeft).toBeGreaterThanOrEqual(0);
    expect(shellRight).toBeLessThanOrEqual(100);
    expect(insideLeft).toBeGreaterThanOrEqual(0);
    expect(insideLeft + stampWidth).toBeLessThanOrEqual(100);
    expect(outsideLeft).toBeGreaterThanOrEqual(0);
    expect(outsideBoxRight).toBeLessThanOrEqual(100);
    expect(outsidePaperRight).toBeLessThanOrEqual(96);
    expect(folderLayout).not.toContain("--app-library-task-folder-open-outside-right");
    expect(folderLayout).toMatch(
      /\.app-library-task-folder__back\s*\{[^}]*z-index:\s*1/s,
    );
    expect(folderLayout).toMatch(
      /\.app-library-task-folder__contents\s*\{[^}]*z-index:\s*2/s,
    );
    expect(folderLayout).toMatch(
      /\.app-library-task-folder__front\s*\{[^}]*z-index:\s*4/s,
    );
    expect(folderLayout).toMatch(
      /\[data-presentation="companion-open"\]\s*\{[^}]*overflow:\s*clip/,
    );
    expect(folderLayout).toMatch(
      /\[data-presentation="companion-open"\][\s\S]*?\.app-library-task-folder__page\s*\{[^}]*overflow:\s*visible[^}]*padding:\s*0/,
    );
    expect(folderLayout).toMatch(
      /\.app-library-task-folder__page--inside\s*\{[^}]*top:\s*10%[^}]*left:\s*var\(--app-library-task-folder-open-inside-left\)/,
    );
    expect(folderLayout).toMatch(
      /\.app-library-task-folder__page--outside\s*\{[^}]*top:\s*10%[^}]*left:\s*var\(--app-library-task-folder-open-outside-left\)/,
    );
    expect(folderAppearance).toContain(
      "border-radius: 2% 10% 10% 2% / 2% 7% 8% 2%",
    );
    expect(folderAppearance).toContain(
      "border-radius: 10% 2% 2% 10% / 7% 2% 2% 7%",
    );
    expect(folderAppearance).toContain(
      ".app-library-task-folder__back::before",
    );
    expect(folderAppearance).toMatch(
      /:where\(\.app-library-task-folder\[data-presentation="companion-open"\]\)\s*\.app-library-task-folder__front-cover\s*\{[^}]*background:[^}]*--app-library-task-folder-frost-highlight[^}]*--app-library-task-folder-frost-face[^}]*--app-library-task-folder-frost-edge/s,
    );
    expect(folderAppearance).toContain(
      "4px 7px 12px -11px rgb(0 0 0 / 0.36)",
    );
    expect(folderAppearance).toContain(
      "-8px 9px 14px -13px rgb(0 0 0 / 0.38)",
    );
    expect(folderAppearance).toContain(".app-library-task-folder__front-film");
    expect(folderAppearance).toContain(".app-library-task-folder__page-stamp");
    expect(folderAppearance).toContain("drop-shadow(2px 6px 3px");
    const companionStampShadow = folderAppearance.match(
      /\[data-presentation="companion-open"\][\s\S]*?\.app-library-task-folder__page-stamp\s*\{[^}]*drop-shadow\(([\d.]+)px\s+([\d.]+)px\s+([\d.]+)px/,
    );
    const shadowOffsetXPx = Number(companionStampShadow?.[1]);
    const shadowBlurPx = Number(companionStampShadow?.[3]);
    const conservativePaintRight = outsidePaperRight +
      (shadowOffsetXPx + shadowBlurPx * 3) / artworkWidth * 100;
    expect(conservativePaintRight).toBeLessThanOrEqual(100);
    expect(
      artworkFrameInsetPx + conservativePaintRight / 100 * artworkWidth,
    ).toBeLessThanOrEqual(companionFrameWidthPx);
    expect(folderAppearance).toContain("Library postage-paper");
    expect(folderAppearance).toContain(
      "--app-library-task-folder-frost-highlight:",
    );
    expect(folderAppearance).toContain(
      "color-mix(in srgb, var(--app-library-material-face) 60%, transparent)",
    );
    expect(folderAppearance).toContain(
      "color-mix(in srgb, var(--app-library-material-face) 42%, transparent)",
    );
    expect(folderAppearance).toContain(
      "color-mix(in srgb, var(--app-library-material-edge) 62%, transparent)",
    );
    expect(`${folderLayout}\n${folderAppearance}`).not.toContain(
      ".app-library-task-folder__peek",
    );
    expect(folderAppearance).not.toMatch(
      /data-reduce-transparency[^}]+app-library-task-folder__unified-preview[^}]+display:\s*none/s,
    );
    expect(`${folderLayout}\n${folderAppearance}`).not.toMatch(
      /\b(?:animation|transition|backdrop-filter)\s*:/,
    );
    expect(folderAppearance).not.toMatch(/\bblur\s*\(/);
    expect(folderAppearance).not.toContain("repeating-linear-gradient");
    expect(folderAppearance).not.toContain(
      ".app-library-item:hover .app-library-task-folder",
    );
  });
});
