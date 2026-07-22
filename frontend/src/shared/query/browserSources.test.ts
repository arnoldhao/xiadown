import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";

import {
  browserProfileAvailabilityReason,
  normalizeAppSessionBrowserScanResult,
  normalizeBrowserSourceProfile,
  normalizeDiscoveredBrowserProfiles,
} from "./browserSources";
import { resolveBrowserSourceSelection } from "../browser-source/BrowserSourcePicker";

describe("browser source adapters", () => {
  test("uses dedicated generated discovery bindings without probing missing methods", () => {
    const source = readFileSync(new URL("./browserSources.ts", import.meta.url), "utf8");
    const appSessionsSource = readFileSync(new URL("./appSessions.ts", import.meta.url), "utf8");
    const bindingsSource = readFileSync(
      new URL("./appSessionsBindings.ts", import.meta.url),
      "utf8",
    );

    expect(bindingsSource).toMatch(
      /appSessionsHandlerBindings \?\?= import\(\s*"\.\.\/\.\.\/\.\.\/bindings\/xiadown\/internal\/presentation\/wails\/appsessionshandler"\s*\)/,
    );
    expect(source).toContain('import { loadAppSessionsHandlerBindings } from "./appSessionsBindings"');
    expect(appSessionsSource).toContain(
      'import { loadAppSessionsHandlerBindings } from "./appSessionsBindings"',
    );
    expect(source).not.toMatch(/from ["'][^"']*\/appsessionshandler["']/);
    expect(appSessionsSource).not.toMatch(/from ["'][^"']*\/appsessionshandler["']/);
    expect(source).toContain("await bindings.ListBrowserProfileSources()");
    expect(source).toContain("await bindings.DiscoverBrowserProfiles({ browserId: input.browserId })");
    expect(source).toContain("await bindings.ScanBrowserProfile(selection)");
    expect(source).toContain("await bindings.ImportBrowserProfile(request)");
    expect(source).toMatch(/APP_SESSION_BROWSER_IDS[\s\S]*?"chrome",[\s\S]*?"safari",/);
    expect(source).toContain("state: stringValue(value.state)");
    expect(source).not.toContain("`${service}.ListBrowserProfiles`");
    expect(source).not.toContain("ListBrowserProfiles`, { intent }");
    expect(source).not.toContain("ListAppSessionBrowserProfiles");
    expect(source).not.toContain("`${APP_SESSIONS_HANDLER}.ScanBrowserProfile`");
    expect(source).not.toContain("`${APP_SESSIONS_HANDLER}.ImportBrowserProfile`");
  });

  test("normalizes profile aliases without depending on generated bindings", () => {
    expect(
      normalizeBrowserSourceProfile({
        profileId: "person-1",
        displayName: "Personal",
        browser: "chrome",
        displayPath: "~/Library/Application Support/Google/Chrome/Default",
        totalBytes: 2048,
      }),
    ).toMatchObject({
      id: "person-1",
      label: "Personal",
      browserId: "chrome",
      displayPath: "~/Library/Application Support/Google/Chrome/Default",
      sizeBytes: 2048,
      available: true,
    });
  });

  test("preserves the managed default role for Sniff Profile identity", () => {
    expect(
      normalizeBrowserSourceProfile({
        profileId: "managed-default",
        displayName: "XiaDown Chrome",
        browser: "chrome",
        isDefault: true,
        redundant: true,
      }),
    ).toMatchObject({
      id: "managed-default",
      label: "XiaDown Chrome",
      browserId: "chrome",
      isDefault: true,
      redundant: true,
    });
  });

  test("propagates managed-profile loading failures instead of forging an empty catalog", () => {
    const source = readFileSync(
      new URL("./browserSources.ts", import.meta.url),
      "utf8",
    );
    const profilesStart = source.indexOf(
      "export async function listSniffProfiles",
    );
    const catalogStart = source.indexOf(
      "export async function listBrowserSources",
      profilesStart,
    );
    const catalogEnd = source.indexOf(
      "export function useBrowserSources",
      catalogStart,
    );
    const profilesBody = source.slice(profilesStart, catalogStart);
    const catalogBody = source.slice(catalogStart, catalogEnd);

    expect(profilesBody).toContain(
      "await Call.ByName(`${SETTINGS_HANDLER}.ListSniffProfiles`)",
    );
    expect(profilesBody).not.toContain("catch");
    expect(catalogBody).toContain(
      "const [browserResult, sniffProfiles] = await Promise.all([",
    );
    expect(catalogBody).toContain("listSniffProfiles()");
    expect(catalogBody).toContain("const xiadownProfiles = sniffProfiles.length > 0");
    expect(catalogBody).toContain("virtual: true");
  });

  test("maps overwrite scan results to a replace review state", () => {
    const result = normalizeAppSessionBrowserScanResult(
      {
        snapshotToken: "opaque-scan-token",
        sessions: [
          {
            id: "youtube",
            state: "overwrite",
            accountName: "Creator",
            selectable: true,
            reason: "no_auth_cookies",
          },
          { id: "tiktok", state: "missing", selectable: false, reason: "private_backend_detail" },
        ],
      },
      { mode: "browser_profile", browserId: "chrome", profileId: "Default" },
    );

    expect(result.browserId).toBe("chrome");
    expect(result.profileId).toBe("Default");
    expect(result.snapshotToken).toBe("opaque-scan-token");
    expect(result.items.map((item) => item.status)).toEqual(["replace", "unavailable"]);
    expect(result.items.map((item) => item.selectable)).toEqual([true, false]);
    expect(result.items.map((item) => item.reason)).toEqual(["no_auth_cookies", ""]);
  });

  test("preserves protected-cookie source states without exposing backend details", () => {
    const result = normalizeAppSessionBrowserScanResult(
      {
        snapshotToken: "opaque-scan-token",
        items: [
          {
            appSessionId: "youtube",
            status: "unavailable",
            selectable: false,
            reason: "browser_cookie_access_required",
          },
          {
            appSessionId: "tiktok",
            status: "unavailable",
            selectable: false,
            reason: "protected_cookies_unsupported",
          },
        ],
      },
      { mode: "browser_profile", browserId: "chrome", profileId: "profile-default" },
    );

    expect(result.items.map((item) => item.reason)).toEqual([
      "browser_cookie_access_required",
      "protected_cookies_unsupported",
    ]);
  });

  test("keeps a browser permission denial distinct from an empty profile list", () => {
    expect(
      normalizeDiscoveredBrowserProfiles(
        [{
          id: "chrome-unavailable",
          browserId: "chrome",
          browserLabel: "Google Chrome",
          label: "Google Chrome",
          available: false,
          state: "permission_required",
          error: "permission required",
        }],
        "chrome",
        "Chrome",
      ),
    ).toMatchObject({
      id: "chrome",
      available: false,
      state: "permission_required",
      error: "permission required",
      profiles: [
        expect.objectContaining({
          id: "chrome-unavailable",
          available: false,
          state: "permission_required",
        }),
      ],
    });
  });

  test("preserves mixed available and unavailable profiles without escalating a browser-wide error", () => {
    const browser = normalizeDiscoveredBrowserProfiles(
      {
        browserId: "chrome",
        browserLabel: "Google Chrome",
        available: false,
        state: "permission_required",
        error: "one protected channel",
        profiles: [
          {
            id: "stable-default",
            browserId: "chrome",
            label: "Default",
            available: true,
            state: "ready",
          },
          {
            id: "beta-protected",
            browserId: "chrome",
            label: "Beta",
            available: false,
            state: "permission_required",
          },
          {
            id: "dev-empty",
            browserId: "chrome",
            label: "Dev",
            available: false,
            state: "no_profile_data",
          },
          {
            id: "canary-invalid",
            browserId: "chrome",
            label: "Canary",
            available: false,
            state: "invalid_profile_data",
          },
        ],
      },
      "chrome",
      "Chrome",
    );

    expect(browser).toMatchObject({
      available: true,
      state: "ready",
      error: "",
    });
    expect(browser.profiles.map((profile) => profile.id)).toEqual([
      "stable-default",
      "beta-protected",
      "dev-empty",
      "canary-invalid",
    ]);
    expect(browser.profiles.map(browserProfileAvailabilityReason)).toEqual([
      "",
      "permission_required",
      "no_profile_data",
      "invalid_profile_data",
    ]);
  });

  test("fails closed for unavailable states even when the boolean is omitted", () => {
    expect(
      normalizeBrowserSourceProfile({
        id: "profile-empty",
        browserId: "chrome",
        state: "no_profile_data",
      }),
    ).toMatchObject({
      available: false,
      state: "no_profile_data",
    });
  });

  test("keeps an in-use Windows browser profile unavailable and actionable", () => {
    const browser = normalizeDiscoveredBrowserProfiles(
      {
        browserId: "chrome",
        browserLabel: "Chrome",
        state: "browser_running",
        profiles: [{
          id: "locked-default",
          browserId: "chrome",
          label: "Default",
          state: "browser_running",
          error: "browser_running",
        }],
      },
      "chrome",
      "Chrome",
    );

    expect(browserProfileAvailabilityReason(browser.profiles[0])).toBe("browser_running");
    expect(browser).toMatchObject({
      available: false,
      state: "browser_running",
      profiles: [expect.objectContaining({ available: false })],
    });
  });

  test("keeps app-bound profiles unavailable before scan", () => {
    for (const state of ["access_required", "protected_unsupported"] as const) {
      const browser = normalizeDiscoveredBrowserProfiles(
        {
          browserId: state === "access_required" ? "chrome" : "edge",
          state,
          profiles: [{
            id: `protected-${state}`,
            browserId: state === "access_required" ? "chrome" : "edge",
            state,
            available: true,
          }],
        },
        state === "access_required" ? "chrome" : "edge",
        "Browser",
      );

      expect(browserProfileAvailabilityReason(browser.profiles[0])).toBe(state);
      expect(browser).toMatchObject({
        available: false,
        state,
        profiles: [expect.objectContaining({ available: false })],
      });
    }
  });

  test("keeps a managed profile scoped to the browser selected for this sniff", () => {
    const catalog = {
      browsers: [
        { id: "chrome", label: "Chrome", available: true, profiles: [] },
        { id: "edge", label: "Edge", available: true, profiles: [] },
      ],
      xiadownProfiles: [
        {
          id: "profile-chrome",
          label: "Chrome work",
          browserId: "chrome",
          available: true,
        },
        {
          id: "profile-edge",
          label: "Edge work",
          browserId: "edge",
          available: true,
        },
      ],
    };

    expect(
      resolveBrowserSourceSelection(
        catalog,
        {
          mode: "xiadown_profile",
          browserId: "edge",
          profileId: "",
        },
        true,
        false,
      ),
    ).toEqual({
      mode: "xiadown_profile",
      browserId: "edge",
      profileId: "profile-edge",
    });
  });
});
