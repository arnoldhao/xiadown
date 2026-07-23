import { describe, expect, test } from "bun:test";

import {
  clearListenLyricsManualOverride,
  getListenLyricsFocusStylePreferenceSnapshot,
  getListenLyricsOffsetPreferenceSnapshot,
  getListenLyricsRendererPreferenceSnapshot,
  listenLyricsTrackPreferenceKey,
  listenLyricsVersionPreferenceKey,
  readListenLyricsManualOverride,
  readListenLyricsOffset,
  readListenLyricsFocusStylePreference,
  readListenLyricsRendererPreference,
  saveListenLyricsManualOverride,
  saveListenLyricsOffset,
  saveListenLyricsFocusStylePreference,
  saveListenLyricsRendererPreference,
  subscribeListenLyricsPreferences,
} from "@/app/main/listen/lyrics-preferences";

const LYRICS_PREFERENCES_KEY = "xiadown.listen.lyrics.preferences.v1";

function memoryStorage(initial: Record<string, string> = {}) {
  const values = new Map(Object.entries(initial));
  return {
    getItem(key: string) {
      return values.get(key) ?? null;
    },
    setItem(key: string, value: string) {
      values.set(key, value);
    },
  };
}

function installLyricsPreferenceWindow(
  storage: ReturnType<typeof memoryStorage>,
) {
  const listeners = new Set<(event: StorageEvent) => void>();
  const previousWindow = Object.getOwnPropertyDescriptor(globalThis, "window");
  const fakeWindow = {
    localStorage: storage,
    addEventListener(type: string, listener: EventListener) {
      if (type === "storage") {
        listeners.add(listener as (event: StorageEvent) => void);
      }
    },
    removeEventListener(type: string, listener: EventListener) {
      if (type === "storage") {
        listeners.delete(listener as (event: StorageEvent) => void);
      }
    },
  } as unknown as Window;
  Object.defineProperty(globalThis, "window", {
    configurable: true,
    value: fakeWindow,
  });
  return {
    dispatchStorage(key: string | null) {
      for (const listener of Array.from(listeners)) {
        listener({ key, storageArea: storage } as unknown as StorageEvent);
      }
    },
    restore() {
      if (previousWindow) {
        Object.defineProperty(globalThis, "window", previousWindow);
      } else {
        Reflect.deleteProperty(globalThis, "window");
      }
    },
  };
}

describe("listen lyrics preferences", () => {
  test("builds stable identities with strong ids before metadata", () => {
    expect(
      listenLyricsTrackPreferenceKey({
        id: "local-one",
        videoId: "video-one",
        title: "Ignored",
      }),
    ).toBe("video:video-one");
    expect(
      listenLyricsTrackPreferenceKey({
        lyricsId: "local:track-one",
        title: "Local Song",
      }),
    ).toBe("local:track-one");
    expect(
      listenLyricsTrackPreferenceKey({
        title: "  Héllo　World ",
        artist: " Artist ",
        durationSeconds: 212.6,
      }),
    ).toBe("meta:h%C3%A9llo%20world:artist:213");
  });

  test("persists renderer, canonical focus flow, manual override and per-version offset", () => {
    const storage = memoryStorage();
    const track = { videoId: "video-one", title: "Song", artist: "Artist" };
    const lyrics = {
      providerId: "lrclib",
      providerTrackId: "42",
      source: "LRCLib",
      timingQuality: "word" as const,
    };

    expect(readListenLyricsRendererPreference(storage)).toBe("scroll");
    saveListenLyricsRendererPreference("focus", storage);
    expect(readListenLyricsRendererPreference(storage)).toBe("focus");
    expect(readListenLyricsFocusStylePreference(storage)).toBe("prism");
    saveListenLyricsFocusStylePreference("facet", storage);
    expect(readListenLyricsFocusStylePreference(storage)).toBe("prism");

    expect(
      saveListenLyricsManualOverride(
        track,
        {
          providerId: " LRCLIB ",
          providerTrackId: " 42 ",
          title: " Song ",
          artist: " Artist ",
          confidence: 120,
        },
        storage,
        10,
      ),
    ).toBe(true);
    expect(readListenLyricsManualOverride(track, storage)).toEqual({
      providerId: "lrclib",
      providerTrackId: "42",
      title: "Song",
      artist: "Artist",
      confidence: 100,
      updatedAt: 10,
    });

    expect(saveListenLyricsOffset(track, lyrics, 5_500, storage, 20)).toBe(
      true,
    );
    expect(readListenLyricsOffset(track, lyrics, storage)).toBe(5_000);
    expect(listenLyricsVersionPreferenceKey(track, lyrics)).toContain(
      "video:video-one|lrclib|42|word",
    );

    expect(clearListenLyricsManualOverride(track, storage)).toBe(true);
    expect(readListenLyricsManualOverride(track, storage)).toBeNull();
  });

  test("recovers from corrupt storage without throwing", () => {
    const storage = memoryStorage({
      [LYRICS_PREFERENCES_KEY]: "{broken",
    });
    expect(readListenLyricsRendererPreference(storage)).toBe("scroll");
    expect(
      readListenLyricsOffset(
        { id: "local:one" },
        { source: "sidecar", timingQuality: "line" },
        storage,
      ),
    ).toBe(0);
  });

  test("reads the persisted renderer in the render snapshot and synchronizes windows", () => {
    const storage = memoryStorage({
      [LYRICS_PREFERENCES_KEY]: JSON.stringify({
        version: 1,
        renderer: "focus",
      }),
    });
    const fakeWindow = installLyricsPreferenceWindow(storage);
    const snapshots: string[] = [];
    let unsubscribe = () => {};
    try {
      expect(getListenLyricsRendererPreferenceSnapshot()).toBe("focus");
      unsubscribe = subscribeListenLyricsPreferences(() => {
        snapshots.push(getListenLyricsRendererPreferenceSnapshot());
      });

      saveListenLyricsRendererPreference("scroll");
      expect(snapshots).toEqual(["scroll"]);

      storage.setItem(
        LYRICS_PREFERENCES_KEY,
        JSON.stringify({ version: 1, renderer: "focus" }),
      );
      fakeWindow.dispatchStorage(LYRICS_PREFERENCES_KEY);
      fakeWindow.dispatchStorage("unrelated.preference");
      expect(snapshots).toEqual(["scroll", "focus"]);

      unsubscribe();
      saveListenLyricsRendererPreference("scroll");
      expect(snapshots).toEqual(["scroll", "focus"]);
    } finally {
      unsubscribe();
      fakeWindow.restore();
    }
  });

  test("reads versioned offsets immediately and synchronizes same-window and storage writes", () => {
    const storage = memoryStorage();
    const track = { videoId: "offset-video", title: "Offset Song" };
    const firstLyrics = {
      providerId: "lrclib",
      providerTrackId: "42",
      source: "LRCLib",
      timingQuality: "line" as const,
    };
    const secondLyrics = {
      ...firstLyrics,
      providerTrackId: "84",
    };
    const fakeWindow = installLyricsPreferenceWindow(storage);
    const snapshots: number[] = [];
    let unsubscribe = () => {};
    try {
      expect(getListenLyricsOffsetPreferenceSnapshot(track, null)).toBe(0);
      expect(
        getListenLyricsOffsetPreferenceSnapshot(track, firstLyrics),
      ).toBe(0);
      unsubscribe = subscribeListenLyricsPreferences(() => {
        snapshots.push(
          getListenLyricsOffsetPreferenceSnapshot(track, firstLyrics),
        );
      });

      saveListenLyricsOffset(track, firstLyrics, 750);
      expect(snapshots).toEqual([750]);
      expect(
        getListenLyricsOffsetPreferenceSnapshot(track, secondLyrics),
      ).toBe(0);

      const secondKey = listenLyricsVersionPreferenceKey(track, secondLyrics);
      storage.setItem(
        LYRICS_PREFERENCES_KEY,
        JSON.stringify({
          version: 2,
          renderer: "scroll",
          offsets: {
            [listenLyricsVersionPreferenceKey(track, firstLyrics)]: {
              offsetMs: -500,
              updatedAt: 20,
            },
            [secondKey]: { offsetMs: 1250, updatedAt: 30 },
          },
        }),
      );
      fakeWindow.dispatchStorage(LYRICS_PREFERENCES_KEY);
      expect(snapshots).toEqual([750, -500]);
      expect(
        getListenLyricsOffsetPreferenceSnapshot(track, secondLyrics),
      ).toBe(1250);
    } finally {
      unsubscribe();
      fakeWindow.restore();
    }
  });

  test("migrates legacy offset direction without changing audible timing", () => {
    const track = { videoId: "legacy-offset" };
    const lyrics = {
      providerId: "lrclib",
      providerTrackId: "legacy-42",
      source: "LRCLib",
      timingQuality: "line" as const,
    };
    const key = listenLyricsVersionPreferenceKey(track, lyrics);
    const storage = memoryStorage({
      [LYRICS_PREFERENCES_KEY]: JSON.stringify({
        version: 1,
        renderer: "scroll",
        offsets: {
          [key]: { offsetMs: 750, updatedAt: 10 },
        },
      }),
    });

    expect(readListenLyricsOffset(track, lyrics, storage)).toBe(-750);
  });

  test("migrates and persists every former focus style as Prism", () => {
    const storage = memoryStorage({
      [LYRICS_PREFERENCES_KEY]: JSON.stringify({
        version: 2,
        renderer: "focus",
        focusStyle: "editorial",
        overrides: {},
        offsets: {},
      }),
    });

    expect(readListenLyricsRendererPreference(storage)).toBe("focus");
    expect(readListenLyricsFocusStylePreference(storage)).toBe("prism");
    saveListenLyricsFocusStylePreference("pendulum", storage);
    const persisted = JSON.parse(
      storage.getItem(LYRICS_PREFERENCES_KEY) ?? "{}",
    ) as Record<string, unknown>;
    expect(persisted).toMatchObject({
      version: 3,
      renderer: "focus",
      focusStyle: "prism",
    });
  });

  test("synchronizes one canonical focus snapshot across windows", () => {
    const storage = memoryStorage();
    const fakeWindow = installLyricsPreferenceWindow(storage);
    const snapshots: string[] = [];
    let unsubscribe = () => {};
    try {
      expect(getListenLyricsFocusStylePreferenceSnapshot()).toBe("prism");
      unsubscribe = subscribeListenLyricsPreferences(() => {
        snapshots.push(getListenLyricsFocusStylePreferenceSnapshot());
      });
      saveListenLyricsFocusStylePreference("splice");
      storage.setItem(
        LYRICS_PREFERENCES_KEY,
        JSON.stringify({
          version: 3,
          renderer: "focus",
          focusStyle: "facet",
        }),
      );
      fakeWindow.dispatchStorage(LYRICS_PREFERENCES_KEY);
      expect(snapshots).toEqual(["prism", "prism"]);
    } finally {
      unsubscribe();
      fakeWindow.restore();
    }
  });
});
