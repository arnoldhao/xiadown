import { describe, expect, test } from "bun:test";
import { QRCodeSVG } from "qrcode.react";
import * as React from "react";
import { renderToStaticMarkup } from "react-dom/server";

import {
  LIBRARY_PAIRING_QR_OPTIONS,
  normalizeLibraryAccessPath,
  normalizeLibraryAccessPort,
  DEFAULT_LIBRARY_DEVICE_SCOPES,
  isValidLibraryPairingLink,
  isLibraryAccessRevisionConflict,
  resolveLibraryAccessStatusTone,
  resolveLibraryAccessTransportTone,
  safeLibraryAccessBackendErrorMessage,
  toggleLibraryDeviceScope,
} from "./library-access-ui";

describe("library access UI helpers", () => {
  test("normalizes a Tailscale Serve path without accepting an empty root", () => {
    expect(normalizeLibraryAccessPath(" library/mobile/ ")).toBe("/library/mobile");
    expect(normalizeLibraryAccessPath("/")).toBe("/xiadown");
  });

  test("keeps ports inside the TCP range", () => {
    expect(normalizeLibraryAccessPort("8443", 9443)).toBe(8443);
    expect(normalizeLibraryAccessPort("70000", 9443)).toBe(9443);
  });

  test("accepts only complete versioned pairing links for local QR rendering", () => {
    const link = new URL("xiadown://pair");
    link.searchParams.set("v", "1");
    link.searchParams.set("nonce", "abcdefghijklmnopqrstuvwxyz_12345");
    link.searchParams.set("code", "012345");
    link.searchParams.set("expires", "2026-07-13T18:30:00.000Z");
    link.searchParams.set("fingerprint", "AB".repeat(32));
    link.searchParams.append("lan", "https://192.168.1.20:43127/");
    link.searchParams.append("lan", "https://[fd00::20]:43127/");
    link.searchParams.append("remote", "https://studio.example.ts.net/xiadown/");

    expect(isValidLibraryPairingLink(link.toString(), 1)).toBeTrue();
    link.searchParams.delete("nonce");
    expect(isValidLibraryPairingLink(link.toString(), 1)).toBeFalse();
    expect(isValidLibraryPairingLink("https://example.test/?token=secret", 1)).toBeFalse();
  });

  test("keeps a scan-safe quiet zone and theme-independent QR contrast", () => {
    expect(LIBRARY_PAIRING_QR_OPTIONS.marginSize).toBeGreaterThanOrEqual(4);
    expect(LIBRARY_PAIRING_QR_OPTIONS.bgColor).toBe("#FFFFFF");
    expect(LIBRARY_PAIRING_QR_OPTIONS.fgColor).toBe("#111827");
  });

  test("renders the one-time pairing link as QR geometry without visible secret text", () => {
    const privateLink = "xiadown://pair?v=1&nonce=private-once-only";
    const markup = renderToStaticMarkup(React.createElement(QRCodeSVG, {
      value: privateLink,
      ...LIBRARY_PAIRING_QR_OPTIONS,
      title: "One-time pairing QR code",
    }));

    expect(markup).toContain("<svg");
    expect(markup).toContain("<title>One-time pairing QR code</title>");
    expect(markup).not.toContain(privateLink);
    expect(markup).not.toContain("xiadown://");
  });

  test("projects transport and aggregate status without exposing backend state codes", () => {
    expect(resolveLibraryAccessTransportTone({ desiredEnabled: true, state: "listening" })).toBe("success");
    expect(resolveLibraryAccessTransportTone({ desiredEnabled: true, state: "starting" })).toBe("pending");
    expect(resolveLibraryAccessTransportTone({ desiredEnabled: true, state: "error", lastError: "blocked" })).toBe("danger");
    expect(resolveLibraryAccessTransportTone({ desiredEnabled: true, state: "unavailable" })).toBe("danger");
    expect(resolveLibraryAccessStatusTone({
      desiredEnabled: true,
      lan: { desiredEnabled: true, state: "listening" },
      tailscale: { desiredEnabled: true, state: "error", lastError: "not installed", installed: false },
    })).toBe("success");
  });

  test("starts paired devices read-only and keeps at least one managed permission", () => {
    expect(DEFAULT_LIBRARY_DEVICE_SCOPES).toEqual(["library.read", "tasks.read"]);
    expect(toggleLibraryDeviceScope(DEFAULT_LIBRARY_DEVICE_SCOPES, "tasks.create")).toEqual([
      "library.read",
      "tasks.read",
      "tasks.create",
    ]);
    expect(toggleLibraryDeviceScope(DEFAULT_LIBRARY_DEVICE_SCOPES, "rss.state")).toEqual([
      "library.read",
      "rss.state",
      "tasks.read",
    ]);
    expect(toggleLibraryDeviceScope(["rss.read", "rss.manage"], "rss.fetch")).toEqual([
      "rss.read",
      "rss.manage",
      "rss.fetch",
    ]);
    expect(toggleLibraryDeviceScope(["library.read"], "library.read")).toEqual(["library.read"]);
  });

  test("recognizes optimistic revision conflicts for a forced refresh", () => {
    expect(isLibraryAccessRevisionConflict(new Error("catalog revision conflict"))).toBeTrue();
    expect(isLibraryAccessRevisionConflict(new Error("network unavailable"))).toBeFalse();
  });

  test("maps backend failures to localized safe copy without leaking diagnostics", () => {
    const text = {
      tailscaleServeNotEnabled: "serve-help",
      tailscaleCLIUnavailable: "cli-help",
      requestTimedOut: "timeout-help",
      networkUnavailable: "network-help",
      requestFailed: "generic-help",
    };
    const cases = [
      [
        new Error("enable Tailscale Serve route: Serve is not enabled; visit https://login.tailscale.com/f/serve"),
        "serve-help",
      ],
      [
        new Error('exec: "C:\\Program Files\\Tailscale\\tailscale.exe": executable file not found in %PATH%'),
        "cli-help",
      ],
      [
        new Error("tailscale command timed out after 10s: context deadline exceeded"),
        "timeout-help",
      ],
      [
        new Error('Get "https://private.example.ts.net/xiadown": dial tcp: network is unreachable'),
        "network-help",
      ],
      [
        new Error("read /Users/private/Library/state.json: permission denied"),
        "generic-help",
      ],
    ] as const;

    for (const [error, expected] of cases) {
      const result = safeLibraryAccessBackendErrorMessage(error, text);
      expect(result).toBe(expected);
      expect(result).not.toContain("tailscale serve");
      expect(result).not.toContain("context");
      expect(result).not.toContain("PATH");
      expect(result).not.toContain("/Users/");
      expect(result).not.toContain("https://");
    }
    expect(safeLibraryAccessBackendErrorMessage(null, text)).toBe("");
  });
});
