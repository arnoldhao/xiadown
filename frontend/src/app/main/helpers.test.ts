import { describe, expect, test } from "bun:test";

import {
  COMPLETED_TEXT_PREVIEW_MAX_BYTES,
  canPreviewCompletedFile,
  getAppErrorCode,
  isCompletedPreviewTooLarge,
  parseAppErrorMessage,
  resolveCompletedDefaultCoverImageKey,
  resolveCompletedFileCoverURL,
  resolveCompletedPreviewKind,
  resolveCompletedTaskCoverURL,
  resolveUnknownErrorMessage,
} from "@/app/main/helpers";

describe("app error helpers", () => {
  test("resolves Wails JSON error payloads to readable messages", () => {
    const error = new Error(
      JSON.stringify({
        code: "invalid_input",
        message: "invalid url or unsupported video path",
      }),
    );

    expect(getAppErrorCode(error)).toBe("invalid_input");
    expect(resolveUnknownErrorMessage(error, "Unknown")).toBe(
      "invalid url or unsupported video path",
    );
  });

  test("keeps bracketed app error codes available", () => {
    expect(
      parseAppErrorMessage(
        "[resource_unsupported_domain] resource sniff does not support youtube.com",
      ),
    ).toEqual({
      code: "resource_unsupported_domain",
      message: "resource sniff does not support youtube.com",
    });
  });

  test("extracts JSON error records embedded in transport messages", () => {
    const error = new Error(
      `request failed: ${JSON.stringify({
        code: "resource_resolve_failed",
        message: "resource sniff session not found",
      })}`,
    );

    expect(getAppErrorCode(error)).toBe("resource_resolve_failed");
    expect(resolveUnknownErrorMessage(error, "Unknown")).toBe(
      "resource sniff session not found",
    );
  });

  test("extracts nested Wails error records", () => {
    const error = {
      error: {
        code: "resource_resolve_failed",
        message: "resource sniff raw resource not found",
      },
    };

    expect(getAppErrorCode(error)).toBe("resource_resolve_failed");
    expect(resolveUnknownErrorMessage(error, "Unknown")).toBe(
      "resource sniff raw resource not found",
    );
  });
});

describe("completed preview helpers", () => {
  test("uses resource file format to correct historical sniff output kinds", () => {
    expect(resolveCompletedPreviewKind({ kind: "video", path: "", format: "WEBP" })).toBe(
      "image",
    );
    expect(resolveCompletedPreviewKind({ kind: "video", path: "", format: "VTT" })).toBe(
      "subtitle",
    );
    expect(resolveCompletedPreviewKind({ kind: "video", path: "", format: "ITT" })).toBe(
      "subtitle",
    );
  });

  test("rejects oversized text previews before fetching file content", () => {
    const file = {
      id: "subtitle-1",
      kind: "subtitle",
      path: "captions.vtt",
      format: "VTT",
      previewURL: "http://127.0.0.1/asset/captions.vtt",
      media: null,
      sizeBytes: COMPLETED_TEXT_PREVIEW_MAX_BYTES + 1,
    };

    expect(isCompletedPreviewTooLarge(file)).toBe(true);
    expect(canPreviewCompletedFile(file)).toBe(false);
  });

  test("does not treat unrelated files as previewable", () => {
    expect(
      canPreviewCompletedFile({
        id: "archive-1",
        kind: "file",
        path: "archive.zip",
        format: "ZIP",
        previewURL: "http://127.0.0.1/asset/archive.zip",
        media: null,
        sizeBytes: 1024,
      }),
    ).toBe(false);
  });

  test("selects completed default covers from task file combinations", () => {
    expect(
      resolveCompletedDefaultCoverImageKey([
        { kind: "video", path: "", format: "MP4" },
        { kind: "audio", path: "", format: "M4A" },
      ]),
    ).toBe("media");
    expect(
      resolveCompletedDefaultCoverImageKey([
        { kind: "video", path: "", format: "MP4" },
        { kind: "audio", path: "", format: "M4A" },
        { kind: "subtitle", path: "", format: "VTT" },
      ]),
    ).toBe("mediaSubtitle");
    expect(
      resolveCompletedDefaultCoverImageKey([
        { kind: "manifest", path: "stream.m3u8", format: "M3U8" },
      ]),
    ).toBe("manifest");
    expect(
      resolveCompletedDefaultCoverImageKey([
        { kind: "document", path: "report.pdf", format: "PDF" },
        { kind: "archive", path: "bundle.zip", format: "ZIP" },
      ]),
    ).toBe("mixed");
  });

  test("uses real image previews before completed default cover art", () => {
    expect(
      resolveCompletedTaskCoverURL([
        {
          kind: "thumbnail",
          path: "/downloads/cover.webp",
          format: "WEBP",
          previewURL: "http://127.0.0.1/asset/cover.webp",
        },
        { kind: "subtitle", path: "/downloads/caption.vtt", format: "VTT" },
      ]),
    ).toBe("http://127.0.0.1/asset/cover.webp");
    expect(
      resolveCompletedFileCoverURL({
        kind: "audio",
        path: "/downloads/song.mp3",
        format: "MP3",
      }),
    ).toBe("/completed-defaults/audio.jpg");
  });
});
