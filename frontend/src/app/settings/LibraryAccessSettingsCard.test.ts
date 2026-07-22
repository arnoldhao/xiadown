import { describe, expect, test } from "bun:test";

describe("Library access settings presentation", () => {
  test("separates configuration, pairing, and device management into standard cards", async () => {
    const source = await Bun.file(new URL("./LibraryAccessSettingsCard.tsx", import.meta.url)).text();

    expect(source).toContain("data-library-access-settings");
    expect(source).toContain("data-library-pairing-settings");
    expect(source).toContain("data-library-paired-devices");
    expect(source).not.toContain("description={accessText.");
  });

  test("moves the one-time credential into the shared mobile pairing sheet", async () => {
    const source = await Bun.file(new URL("./LibraryAccessSettingsCard.tsx", import.meta.url)).text();

    expect(source).toContain("<LibraryPairingSheet");
    expect(source).toContain("setPairingSheetOpen(true)");
    expect(source).not.toContain("<QRCodeSVG");
    expect(source).not.toContain("pairing.tlsFingerprint");
  });

  test("keeps the paired-device card as a device list and moves permissions into a dialog", async () => {
    const [source, detailsDialog, detailsContent] = await Promise.all([
      Bun.file(new URL("./LibraryAccessSettingsCard.tsx", import.meta.url)).text(),
      Bun.file(new URL("./LibraryDeviceDetailsDialog.tsx", import.meta.url)).text(),
      Bun.file(new URL("./LibraryDeviceDetailsContent.tsx", import.meta.url)).text(),
    ]);
    const pairedCardStart = source.indexOf(
      '<SettingsCompactListCard data-library-paired-devices>',
    );
    const detailsDialogStart = source.indexOf(
      "<LibraryDeviceDetailsDialog",
      pairedCardStart,
    );

    expect(pairedCardStart).toBeGreaterThan(-1);
    expect(detailsDialogStart).toBeGreaterThan(pairedCardStart);

    const pairedCard = source.slice(pairedCardStart, detailsDialogStart);
    expect(pairedCard).toContain("<LibraryPairedDeviceRow");
    expect(pairedCard).toContain("setDeviceDetailsGrantId(device.grantId)");
    expect(detailsContent).toContain("data-library-device={props.device.grantId}");
    expect(detailsContent).toContain(
      'aria-label={`${text.actions.view}: ${name}`}',
    );
    expect(pairedCard).not.toContain("deviceScopes.map");
    expect(pairedCard).not.toContain("accessText.lastSeen");

    expect(detailsDialog).toContain("<LibraryDeviceDetailsContent");
    expect(detailsContent).toContain("data-library-device-details");
    expect(detailsContent).toContain("deviceScopes.map");
    expect(detailsContent).toContain('{ scope: "rss.manage", label: accessText.scopeRSSManage }');
    expect(detailsContent).toContain('{ scope: "rss.fetch", label: accessText.scopeRSSFetch }');
    expect(detailsContent).toContain("accessText.lastSeen");
    expect(detailsContent).toContain("<DialogScrollArea");
    expect(detailsDialog).toContain("<DialogClose asChild>");
  });

  test("never renders raw backend or Tailscale process errors", async () => {
    const [settings, details, main] = await Promise.all([
      Bun.file(new URL("./LibraryAccessSettingsCard.tsx", import.meta.url)).text(),
      Bun.file(new URL("./LibraryDeviceDetailsDialog.tsx", import.meta.url)).text(),
      Bun.file(new URL("../main/MainApp.tsx", import.meta.url)).text(),
    ]);

    expect(settings).toContain(
      "safeLibraryAccessBackendErrorMessage(status?.tailscale.lastError, accessText)",
    );
    expect(settings).toContain(
      "safeLibraryAccessBackendErrorMessage(pairedDevices.error, accessText)",
    );
    expect(settings).not.toContain("lastError?.trim()");
    expect(details).toContain(
      "safeLibraryAccessBackendErrorMessage(mutationError, accessText)",
    );
    expect(main).toContain(
      "safeLibraryAccessBackendErrorMessage(libraryAccessStatusQuery.data?.tailscale.lastError, libraryAccessCopy)",
    );
    expect(main).not.toContain("libraryAccessErrorMessage(");
  });
});
