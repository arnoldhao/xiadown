import { describe, expect, test } from "bun:test";

import {
  resolveListenLocalPlayableQueue,
  resolveListenLocalPlaybackCapability,
  type ListenLocalCanPlayType,
} from "./local-format";
import { mapListenLocalTrackDTO } from "./local-library";
import type { ListenLocalItem } from "./types";

const acceptKnownAudio: ListenLocalCanPlayType = (mimeType) =>
  /audio\/(aac|flac|mpeg|mp4|ogg|wav|webm)/.test(mimeType)
    ? "probably"
    : "";

function track(
  id: string,
  overrides: Partial<ListenLocalItem> = {},
): ListenLocalItem {
  return {
    id,
    title: id,
    author: "",
    album: "",
    albumArtist: "",
    genre: "",
    trackNumber: 0,
    discNumber: 0,
    year: 0,
    lyricsTitle: id,
    lyricsArtist: "",
    path: `/music/${id}.mp3`,
    previewURL: "",
    durationLabel: "3:00",
    durationSeconds: 180,
    coverURL: "",
    format: "mp3",
    audioCodec: "mp3",
    sizeBytes: 1024,
    metadataWritable: true,
    playbackSupported: true,
    playbackUnsupportedReason: "",
    probeError: "",
    modTimeUnix: 0,
    createdAtUnix: 0,
    ...overrides,
  };
}

describe("local audio format capability", () => {
  test.each([
    ["/music/song.m4a", "mov,mp4,m4a,3gp,3g2,mj2", "aac", "audio/mp4"],
    ["/music/song.aac", "aac", "aac", "audio/aac"],
    ["/music/song.mp3", "mp3", "mp3", "audio/mpeg"],
    ["/music/song.flac", "flac", "flac", "audio/flac"],
    ["/music/song.ogg", "ogg", "vorbis", "audio/ogg"],
    ["/music/song.opus", "ogg", "opus", "audio/ogg"],
    ["/music/song.webm", "matroska,webm", "opus", "audio/webm"],
    ["/music/song.weba", "matroska,webm", "vorbis", "audio/webm"],
  ])("accepts %s when the current WebView reports support", (path, format, codec, mime) => {
    const capability = resolveListenLocalPlaybackCapability(
      { path, format, audioCodec: codec },
      { canPlayType: acceptKnownAudio },
    );
    expect(capability.supported).toBeTrue();
    expect(capability.mimeType).toContain(mime);
    expect(capability.unsupportedReason).toBe("");
  });

  test.each([
    ["/music/song.ape", "ape"],
    ["/music/song.wma", "wmav2"],
  ])("keeps %s in the library but never advertises WebView playback", (path, codec) => {
    const capability = resolveListenLocalPlaybackCapability(
      { path, format: codec, audioCodec: codec },
      { canPlayType: () => "probably" },
    );
    expect(capability.supported).toBeFalse();
    expect(capability.unsupportedReason).toMatch(/^unsupported-/);
  });

  test("uses the active WebView result instead of a global optimistic list", () => {
    const capability = resolveListenLocalPlaybackCapability(
      { path: "/music/song.ogg", format: "ogg", audioCodec: "vorbis" },
      { canPlayType: () => "" },
    );
    expect(capability).toMatchObject({
      supported: false,
      unsupportedReason: "webview-rejected",
    });
  });

  test("does not probe an unrelated playable MIME for an invalid container-codec pair", () => {
    const seen: string[] = [];
    const capability = resolveListenLocalPlaybackCapability(
      { path: "/music/mislabeled.webm", format: "matroska,webm", audioCodec: "aac" },
      {
        canPlayType: (mimeType) => {
          seen.push(mimeType);
          return "probably";
        },
      },
    );
    expect(capability).toMatchObject({
      supported: false,
      unsupportedReason: "unknown-format",
    });
    expect(seen).toEqual([]);
  });

  test("keeps M4A/AAC and WebM/Opus bound to their real containers", () => {
    const m4a = resolveListenLocalPlaybackCapability(
      { path: "/music/song.m4a", format: "mov,mp4,m4a", audioCodec: "aac" },
      { canPlayType: () => "probably" },
    );
    const webm = resolveListenLocalPlaybackCapability(
      { path: "/music/song.webm", format: "matroska,webm", audioCodec: "opus" },
      { canPlayType: () => "probably" },
    );
    expect(m4a.mimeType).toBe('audio/mp4; codecs="mp4a.40.2"');
    expect(webm.mimeType).toBe('audio/webm; codecs="opus"');
  });

  test.each([
    ["/music/song.m4a", "aac"],
    ["/music/song.mp3", "mp3"],
    ["/music/song.flac", "flac"],
    ["/music/song.ogg", "vorbis"],
    ["/music/song.opus", "opus"],
  ])("has a deterministic non-DOM decision for %s", (path, codec) => {
    expect(
      resolveListenLocalPlaybackCapability(
        { path, audioCodec: codec },
        { canPlayType: null },
      ).supported,
    ).toBeTrue();
  });

  test("recognizes a supported codec even when an imported file has no useful extension", () => {
    const capability = resolveListenLocalPlaybackCapability(
      { path: "/music/recording.bin", format: "mp3", audioCodec: "mp3" },
      { canPlayType: acceptKnownAudio },
    );
    expect(capability).toMatchObject({
      supported: true,
      mimeType: "audio/mpeg",
    });
  });

  test("filters unsupported items before building a local playback queue", () => {
    const items = [
      track("mp3"),
      track("ape", {
        path: "/music/song.ape",
        format: "ape",
        audioCodec: "ape",
        playbackSupported: false,
        playbackUnsupportedReason: "unsupported-container",
      }),
      track("flac", { path: "/music/song.flac", format: "flac", audioCodec: "flac" }),
    ];
    expect(resolveListenLocalPlayableQueue(items).map((item) => item.id)).toEqual([
      "mp3",
      "flac",
    ]);
  });

  test("maps probe failures and deterministic playback status into local tracks", () => {
    const item = mapListenLocalTrackDTO(
      {
        fileId: "song",
        localPath: "/music/song.m4a",
        title: "Song",
        format: "mov,mp4,m4a,3gp,3g2,mj2",
        audioCodec: "aac",
        probeError: "ffprobe temporarily unavailable",
      },
      "http://127.0.0.1:1234",
    );
    expect(item).toMatchObject({
      playbackSupported: true,
      playbackUnsupportedReason: "",
      probeError: "ffprobe temporarily unavailable",
    });
  });

  test("keeps playback and embedded-tag editing as independent capabilities", () => {
    const wav = mapListenLocalTrackDTO(
      {
        fileId: "wav",
        localPath: "/music/song.wav",
        format: "wav",
        audioCodec: "pcm_s16le",
        metadataWritable: false,
      },
      "http://127.0.0.1:1234",
    );
    const flac = mapListenLocalTrackDTO(
      {
        fileId: "flac",
        localPath: "/music/song.flac",
        format: "flac",
        audioCodec: "flac",
        metadataWritable: true,
      },
      "http://127.0.0.1:1234",
    );
    const ape = mapListenLocalTrackDTO(
      {
        fileId: "ape",
        localPath: "/music/song.ape",
        format: "ape",
        audioCodec: "ape",
        metadataWritable: false,
      },
      "http://127.0.0.1:1234",
    );

    expect(wav).toMatchObject({ playbackSupported: true, metadataWritable: false });
    expect(flac).toMatchObject({ playbackSupported: true, metadataWritable: true });
    expect(ape).toMatchObject({ playbackSupported: false, metadataWritable: false });
  });
});
