import { describe, expect, test } from "bun:test";

import {
  formatAudioTrackLabel,
  resolveResourceSniffStartResolution,
  selectAudioFormatId,
} from "@/app/main/new-task-dialog-helpers";

describe("new task dialog resource sniff lifecycle helpers", () => {
  test("preserves a sniff start that was transferred to Sniff Desk", () => {
    expect(
      resolveResourceSniffStartResolution({
        requestVersion: 4,
        currentVersion: 9,
        dialogOpen: true,
        transferRequestVersion: 4,
      }),
    ).toBe("preserve");
  });

  test("cancels a stale sniff start after dialog state moved on", () => {
    expect(
      resolveResourceSniffStartResolution({
        requestVersion: 4,
        currentVersion: 5,
        dialogOpen: true,
        transferRequestVersion: null,
      }),
    ).toBe("cancel");
  });

  test("cancels a completed sniff start after the dialog closed", () => {
    expect(
      resolveResourceSniffStartResolution({
        requestVersion: 4,
        currentVersion: 4,
        dialogOpen: false,
        transferRequestVersion: null,
      }),
    ).toBe("cancel");
  });

  test("attaches a current sniff start while the dialog stays open", () => {
    expect(
      resolveResourceSniffStartResolution({
        requestVersion: 4,
        currentVersion: 4,
        dialogOpen: true,
        transferRequestVersion: null,
      }),
    ).toBe("attach");
  });
});

describe("new task dialog yt-dlp audio format helpers", () => {
  test("selects the audio track with the highest available bitrate", () => {
    expect(
      selectAudioFormatId([
        {
          id: "low",
          label: "low",
          hasVideo: false,
          hasAudio: true,
          abr: 96,
          filesize: 2000,
        },
        {
          id: "high",
          label: "high",
          hasVideo: false,
          hasAudio: true,
          abr: 160,
          filesize: 1000,
        },
      ]),
    ).toBe("high");
  });

  test("formats audio track labels with language and bitrate metadata", () => {
    expect(
      formatAudioTrackLabel({
        id: "30280",
        label: "fallback",
        hasVideo: false,
        hasAudio: true,
        ext: "m4a",
        acodec: "mp4a.40.2",
        formatNote: "Chinese",
        language: "zh-Hans",
        abr: 132.5,
        audioChannels: 2,
        filesize: 1234567,
      }),
    ).toBe("zh-Hans · Chinese · 133k · 2ch · m4a · AAC · 1.2 MB");
  });
});
