import { describe, expect, test } from "bun:test";

describe("Library mobile pairing sheet", () => {
  test("shows connections, a scan-safe credential, and the mobile release status", async () => {
    const source = await Bun.file(new URL("./LibraryPairingSheet.tsx", import.meta.url)).text();

    expect(source).toContain("usePairedLibraryDevices(props.open)");
    expect(source).toContain("useStartLibraryPairing()");
    expect(source).toContain("<QRCodeSVG");
    expect(source).toContain("{...LIBRARY_PAIRING_QR_OPTIONS}");
    expect(source).toContain("copyPairingLink");
    expect(source).toContain("Clipboard.SetText(pairing.pairingLink)");
    expect(source).toContain("navigator.clipboard.writeText(pairing.pairingLink)");
    expect(source).toContain("messageBus.publishToast");
    expect(source).toContain("accessText.comingSoon");
    expect(source).not.toContain("MOBILE_DOWNLOAD_URL");
    expect(source).not.toContain("openExternalURL");
  });

  test("opens as a compact centered modal and creates a QR code only on request", async () => {
    const source = await Bun.file(new URL("./LibraryPairingSheet.tsx", import.meta.url)).text();

    expect(source).toContain('<SheetContent centered size="sm">');
    expect(source).toContain('<SheetDescription className="sr-only">');
    expect(source).toContain('onClick={requestFreshPairing}');
    expect(source).toContain('<Plus className="h-4 w-4" />');
    expect(source).toContain('className="grid w-full max-w-xs grid-cols-2 gap-2"');
    expect(source).not.toContain("pairingRequestedRef");
    expect(source).not.toContain("accessText.pairingLinkDescription");
    expect(source).not.toContain("accessText.pairingQRPrivacy");
    expect(source).not.toContain("accessText.downloadMobileDescription");
  });

  test("does not expose diagnostic pairing fields as visible rows", async () => {
    const source = await Bun.file(new URL("./LibraryPairingSheet.tsx", import.meta.url)).text();

    expect(source).toContain("safeLibraryAccessBackendErrorMessage(startPairing.error, accessText)");
    expect(source).toContain("safeLibraryAccessBackendErrorMessage(statusQuery.data?.tailscale.lastError, accessText)");
    expect(source).not.toContain("statusQuery.data?.tailscale.lastError?.trim()");
    expect(source).not.toContain("pairing.tlsFingerprint");
    expect(source).not.toContain("pairing.lanEndpoints");
    expect(source).not.toContain("pairing.tailscaleURL");
    expect(source).not.toContain("pairing.code}");
  });

  test("shows local-only status with a neutral indicator", async () => {
    const source = await Bun.file(new URL("./LibraryPairingSheet.tsx", import.meta.url)).text();

    expect(source).toContain('remoteEnabled ? tone : "neutral"');
    expect(source).toContain("tone={statusBadgeTone(displayedTone)}");
    expect(source).toContain('className="w-full"');
    expect(source).toContain("marker={!statusRefreshing}");
    expect(source).not.toContain("app-library-pairing-status");
    expect(source).not.toContain("statusDotClass");
  });
});
