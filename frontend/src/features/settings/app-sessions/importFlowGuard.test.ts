import { describe, expect, test } from "bun:test";

import type { BrowserSourceSelection } from "@/shared/contracts/browserSources";

import {
  type AppSessionImportOperation,
  type AppSessionImportFlowSnapshot,
  browserProfileCanEnterPrerequisite,
  hasActiveAppSessionImportOperation,
  isCurrentBrowserDiscovery,
  isCurrentImportOperation,
  resolveBrowserProfilePrerequisite,
  tryBeginAppSessionImportOperation,
} from "./importFlowGuard";

const chromeSelection: BrowserSourceSelection = {
  mode: "browser_profile",
  browserId: "chrome",
  profileId: "chrome-profile",
};

function flow(
  patch: Partial<AppSessionImportFlowSnapshot> = {},
): AppSessionImportFlowSnapshot {
  return {
    open: true,
    step: "method",
    discoveryEpoch: 1,
    operationEpoch: 1,
    selectedBrowserId: "chrome",
    selection: chromeSelection,
    snapshotToken: "snapshot-1",
    ...patch,
  };
}

describe("App Session import flow guards", () => {
  test("allows repairable profile prerequisites without exposing terminal failures", () => {
    expect(browserProfileCanEnterPrerequisite({ available: true })).toBe(true);
    expect(
      browserProfileCanEnterPrerequisite({
        available: false,
        state: "permission_required",
      }),
    ).toBe(true);
    expect(
      browserProfileCanEnterPrerequisite({
        available: false,
        state: "browser_running",
      }),
    ).toBe(true);
    expect(
      browserProfileCanEnterPrerequisite({
        available: false,
        state: "no_profile_data",
      }),
    ).toBe(false);
    expect(
      browserProfileCanEnterPrerequisite({
        available: false,
        state: "invalid_profile_data",
      }),
    ).toBe(false);
  });

  test("retains a selected profile snapshot when refresh temporarily omits it", () => {
    const snapshot = {
      id: "profile-default",
      browserId: "chrome",
      label: "Default",
      displayPath: "~/Library/Application Support/Google/Chrome/Default",
      available: true,
      state: "ready",
    };

    expect(
      resolveBrowserProfilePrerequisite([], snapshot.id, snapshot),
    ).toEqual({
      profile: snapshot,
      presentInDiscovery: false,
      ready: false,
    });
  });

  test("uses a refreshed protected profile without losing its prerequisite card", () => {
    const snapshot = {
      id: "profile-default",
      browserId: "chrome",
      label: "Default",
      available: true,
      state: "ready",
    };
    const protectedProfile = {
      ...snapshot,
      available: false,
      state: "access_required",
    };

    expect(
      resolveBrowserProfilePrerequisite(
        [protectedProfile],
        snapshot.id,
        snapshot,
      ),
    ).toEqual({
      profile: protectedProfile,
      presentInDiscovery: true,
      ready: false,
    });
  });

  test("locks same-tick profile interactions as soon as a scan begins", () => {
    const operation = { current: "" as AppSessionImportOperation };
    let selectedProfileId = "profile-a";

    expect(tryBeginAppSessionImportOperation(operation, "scan")).toBe(true);
    expect(hasActiveAppSessionImportOperation(operation)).toBe(true);

    // This models a profile-card event delivered before React has rendered
    // scanning=true. The synchronous latch must already reject it.
    if (!hasActiveAppSessionImportOperation(operation)) {
      selectedProfileId = "profile-b";
    }

    expect(selectedProfileId).toBe("profile-a");
    expect(tryBeginAppSessionImportOperation(operation, "import")).toBe(false);
    expect(operation.current).toBe("scan");
  });

  test("rejects Chrome discovery after back navigation and a later Safari selection", () => {
    const safariFlow = flow({
      discoveryEpoch: 3,
      selectedBrowserId: "safari",
      selection: {
        mode: "browser_profile",
        browserId: "safari",
        profileId: "",
      },
    });

    expect(
      isCurrentBrowserDiscovery(safariFlow, {
        epoch: 1,
        browserId: "chrome",
        step: "method",
      }),
    ).toBe(false);
    expect(
      isCurrentBrowserDiscovery(safariFlow, {
        epoch: 3,
        browserId: "safari",
        step: "method",
      }),
    ).toBe(true);
  });

  test("rejects discovery from a prior close and reopen cycle", () => {
    const reopened = flow({ discoveryEpoch: 5 });

    expect(
      isCurrentBrowserDiscovery(reopened, {
        epoch: 2,
        browserId: "chrome",
        step: "method",
      }),
    ).toBe(false);
    expect(
      isCurrentBrowserDiscovery(
        { ...reopened, open: false },
        { epoch: 5, browserId: "chrome", step: "method" },
      ),
    ).toBe(false);
  });

  test("rejects discovery results that cross the method prerequisite boundary", () => {
    const prerequisite = flow({ step: "prerequisite" });

    expect(
      isCurrentBrowserDiscovery(prerequisite, {
        epoch: 1,
        browserId: "chrome",
        step: "method",
      }),
    ).toBe(false);
    expect(
      isCurrentBrowserDiscovery(prerequisite, {
        epoch: 1,
        browserId: "chrome",
        step: "prerequisite",
      }),
    ).toBe(true);
  });

  test("rejects stale scan and import completions after navigation or reopen", () => {
    expect(
      isCurrentImportOperation(
        flow({ step: "browser", operationEpoch: 2 }),
        { epoch: 1, step: "prerequisite", selection: chromeSelection },
      ),
    ).toBe(false);

    expect(
      isCurrentImportOperation(
        flow({
          step: "review",
          operationEpoch: 4,
          snapshotToken: "new-snapshot",
        }),
        {
          epoch: 3,
          step: "review",
          selection: chromeSelection,
          snapshotToken: "old-snapshot",
        },
      ),
    ).toBe(false);

    expect(
      isCurrentImportOperation(
        flow({
          step: "prerequisite",
          selection: {
            mode: "current_browser",
            browserId: "chrome",
            profileId: chromeSelection.profileId,
          },
        }),
        { epoch: 1, step: "prerequisite", selection: chromeSelection },
      ),
    ).toBe(false);

    expect(
      isCurrentImportOperation(flow({ step: "review" }), {
        epoch: 1,
        step: "review",
        selection: chromeSelection,
        snapshotToken: "snapshot-1",
      }),
    ).toBe(true);
  });
});
