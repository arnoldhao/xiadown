import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import { COMPLETED_DEFAULT_COVER_IMAGE_URLS } from "@/shared/assets/default-cover";

import {
  CompletedFileArtwork,
  CompletedTaskArtwork,
  resolveCompletedArtworkImageURL,
} from "./CompletedArtwork";

describe("legacy Completed artwork boundary", () => {
  test("renders semantic task and file defaults as live placeholders", () => {
    const taskMarkup = renderToStaticMarkup(
      <CompletedTaskArtwork
        task={{ coverURL: COMPLETED_DEFAULT_COVER_IMAGE_URLS.other }}
        alt="Task"
      />,
    );
    const fileMarkup = renderToStaticMarkup(
      <CompletedFileArtwork
        file={{
          kind: "video",
          path: "/Library/clip.mp4",
          format: "MP4",
          media: null,
          coverURL: COMPLETED_DEFAULT_COVER_IMAGE_URLS.video,
        }}
        alt="Video"
      />,
    );

    expect(taskMarkup).toContain('data-artwork-kind="task"');
    expect(fileMarkup).toContain('data-artwork-kind="video"');
    expect(`${taskMarkup}${fileMarkup}`).not.toContain("<img");
    expect(`${taskMarkup}${fileMarkup}`).not.toContain(
      "xiadown-library-default:",
    );
  });

  test("keeps real artwork URLs and filters semantic tokens at media URL boundaries", () => {
    const artworkURL = "http://127.0.0.1/artwork/clip.webp";
    const markup = renderToStaticMarkup(
      <CompletedFileArtwork
        file={{
          kind: "video",
          path: "/Library/clip.mp4",
          format: "MP4",
          media: null,
          coverURL: artworkURL,
        }}
        alt="Video"
      />,
    );

    expect(markup).toContain(`<img src="${artworkURL}"`);
    expect(
      resolveCompletedArtworkImageURL(
        COMPLETED_DEFAULT_COVER_IMAGE_URLS.video,
        ` ${artworkURL} `,
      ),
    ).toBe(artworkURL);
    expect(
      resolveCompletedArtworkImageURL(
        COMPLETED_DEFAULT_COVER_IMAGE_URLS.video,
      ),
    ).toBe("");
  });

  test("keeps dormant Completed consumers on the shared artwork boundary", async () => {
    const pageSource = await Bun.file(
      new URL("./CompletedPage.tsx", import.meta.url),
    ).text();
    const detailSource = await Bun.file(
      new URL("./detail-components.tsx", import.meta.url),
    ).text();

    expect(pageSource).toContain("<CompletedTaskArtwork");
    expect(pageSource).toContain("<CompletedFileArtwork");
    expect(detailSource).toContain("<CompletedTaskArtwork");
    expect(detailSource).toContain("<CompletedFileArtwork");
    expect(detailSource).toContain("posterURL={previewCoverURL || undefined}");
    expect(pageSource).not.toMatch(/<img\s+src=\{entry\.coverURL\}/);
    expect(detailSource).not.toMatch(/<img\s+src=\{props\.coverURL\}/);
  });

  test("keeps the inline detail width stable when Companion changes the outer window", async () => {
    const [pageSource, css] = await Promise.all([
      Bun.file(new URL("./CompletedPage.tsx", import.meta.url)).text(),
      Bun.file(
        new URL("../../../shared/styles/dream/completed.css", import.meta.url),
      ).text(),
    ]);

    expect(pageSource).toContain(
      "app-completed-inline-detail my-3 flex w-[25rem]",
    );
    expect(pageSource).not.toContain("xl:w-[27rem]");
    expect(css).toContain("container: completed-page / inline-size");
    expect(css).toContain("@container completed-page (min-width: 66rem)");
    expect(css).toMatch(
      /\.app-main-completed-page \.app-completed-inline-detail\s*\{[^}]*width: 27rem;/,
    );
  });

  test("keeps the static task grid recipe in Dream CSS", async () => {
    const [pageSource, css] = await Promise.all([
      Bun.file(new URL("./CompletedPage.tsx", import.meta.url)).text(),
      Bun.file(
        new URL("../../../shared/styles/dream/completed.css", import.meta.url),
      ).text(),
    ]);

    expect(pageSource).not.toContain("gridTemplateColumns");
    expect(css).toMatch(
      /\.app-completed-task-grid\s*\{[^}]*grid-template-columns:\s*repeat\(/s,
    );
  });
});
