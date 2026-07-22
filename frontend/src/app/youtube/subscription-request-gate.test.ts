import { describe, expect, test } from "bun:test";

import {
  reconcileYouTubeSubscriptionDetails,
  resolveYouTubeUploaderSubscriptionSync,
  YouTubeSubscriptionRequestGate,
  youtubeSubscriptionIdentity,
} from "./subscription-request-gate";

describe("YouTube subscription request gate", () => {
  test("rejects a late result after the channel or video identity changes", () => {
    const channelA = youtubeSubscriptionIdentity("channel-a", "video-a");
    const channelB = youtubeSubscriptionIdentity("channel-b", "video-b");
    const gate = new YouTubeSubscriptionRequestGate(channelA);
    const stale = gate.begin(channelA);

    gate.activate(channelB);

    expect(gate.isCurrent(stale)).toBeFalse();
    const current = gate.begin(channelB);
    expect(gate.isCurrent(current)).toBeTrue();
  });

  test("only the newest request for one identity may settle", () => {
    const identity = youtubeSubscriptionIdentity("channel-a", "video-a");
    const gate = new YouTubeSubscriptionRequestGate(identity);
    const older = gate.begin(identity);
    const newer = gate.begin(identity);

    expect(gate.isCurrent(older)).toBeFalse();
    expect(gate.isCurrent(newer)).toBeTrue();
    expect(gate.canReconcile(older)).toBeFalse();
    expect(gate.canReconcile(newer)).toBeTrue();
  });

  test("invalidates an unresolved request when its view unmounts", () => {
    const identity = youtubeSubscriptionIdentity("channel-a");
    const gate = new YouTubeSubscriptionRequestGate(identity);
    const request = gate.begin(identity);

    gate.invalidate();

    expect(gate.isCurrent(request)).toBeFalse();
    expect(gate.canReconcile(request)).toBeFalse();
  });

  test("allows navigation-stale success unless a newer mutation targets the channel", () => {
    const firstView = youtubeSubscriptionIdentity("channel-a", "route-a");
    const secondView = youtubeSubscriptionIdentity("channel-a", "route-b");
    const gate = new YouTubeSubscriptionRequestGate(firstView);
    const first = gate.begin(firstView, "channel-a");

    gate.activate(secondView);
    expect(gate.isCurrent(first)).toBeFalse();
    expect(gate.canReconcile(first)).toBeTrue();

    const second = gate.begin(secondView, "channel-a");
    expect(gate.canReconcile(first)).toBeFalse();
    expect(gate.canReconcile(second)).toBeTrue();
  });

  test("does not apply an uploader result to a different watch channel", () => {
    expect(resolveYouTubeUploaderSubscriptionSync(
      "channel-a",
      "channel-b",
      "channel-a",
    )).toEqual({ accept: true, updateWatch: false });
    expect(resolveYouTubeUploaderSubscriptionSync(
      "channel-a",
      "channel-a",
      "channel-a",
    )).toEqual({ accept: true, updateWatch: true });
    expect(resolveYouTubeUploaderSubscriptionSync(
      "channel-a",
      "channel-a",
      "channel-b",
    )).toEqual({ accept: false, updateWatch: false });
  });

  test("reconciles a successful result by channel after the view identity changes", () => {
    const channelA = { channelId: "channel-a", isSubscribed: false, title: "A" };
    const channelB = { channelId: "channel-b", isSubscribed: false, title: "B" };

    expect(reconcileYouTubeSubscriptionDetails(channelA, "channel-a", true)).toEqual({
      ...channelA,
      isSubscribed: true,
    });
    expect(reconcileYouTubeSubscriptionDetails(channelB, "channel-a", true)).toBe(channelB);
  });
});
