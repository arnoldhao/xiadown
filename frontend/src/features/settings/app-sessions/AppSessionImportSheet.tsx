import {
  AlertTriangle,
  Check,
  CloudSync,
  Loader2,
  RefreshCw,
  ScanSearch,
  UserRound,
} from "lucide-react";
import * as React from "react";

import { cn } from "@/lib/utils";
import { BrowserBrandIcon } from "@/shared/browser-source/BrowserBrandIcon";
import { browserProfileDisplayLabel } from "@/shared/browser-source/browserProfileDisplayLabel";
import type { AppSession } from "@/shared/contracts/appSessions";
import type { CurrentResourceSniffBrowserState } from "@/shared/contracts/library";
import type {
  AppSessionBrowserScanItem,
  BrowserSourceBrowser,
  BrowserSourceProfile,
  BrowserSourceSelection,
} from "@/shared/contracts/browserSources";
import { useI18n } from "@/shared/i18n";
import { useCurrentResourceSniffBrowserStatus } from "@/shared/query/library";
import {
  browserProfileAvailabilityReason,
  scanBrowserAppSessions,
  useAppSessionBrowserProfileSources,
  useDiscoverAppSessionBrowserProfiles,
  useImportBrowserAppSessions,
  useOpenBrowserDataPermissionGuide,
} from "@/shared/query/browserSources";
import { Button } from "@/shared/ui/button";
import { Badge } from "@/shared/ui/badge";
import {
  Sheet,
  SheetBody,
  SheetCloseButton,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetHeading,
  SheetTitle,
} from "@/shared/ui/sheet";
import { SiteBrandIcon } from "@/shared/ui/site-brand-icon";
import {
  StatusBadge,
  type DreamStatusTone,
} from "@/shared/ui/status-badge";

import {
  browserScanReasonTranslationKey,
  isBrowserScanSourceNotice,
} from "./browserScanReason";
import {
  type AppSessionImportFlowSnapshot,
  type AppSessionImportOperation,
  type AppSessionImportFlowStep,
  browserProfileCanEnterPrerequisite,
  hasActiveAppSessionImportOperation,
  isCurrentBrowserDiscovery,
  isCurrentImportOperation,
  resolveBrowserProfilePrerequisite,
  tryBeginAppSessionImportOperation,
} from "./importFlowGuard";

const CURRENT_CHROME_REMOTE_DEBUGGING_URL =
  "chrome://inspect/#remote-debugging";

const CURRENT_CHROME_STATUS_KEYS: Record<
  CurrentResourceSniffBrowserState,
  string
> = {
  ready: "browserSource.currentChromeReady",
  not_installed: "browserSource.currentChromeNotInstalled",
  not_running: "browserSource.currentChromeNotRunning",
  remote_debugging_disabled:
    "browserSource.currentChromeRemoteDebuggingDisabled",
  permission_denied: "browserSource.currentChromePermissionDenied",
  unsupported_version: "browserSource.currentChromeUnsupportedVersion",
  endpoint_unavailable: "browserSource.currentChromeEndpointUnavailable",
  unsupported_browser: "browserSource.currentChromeUnsupportedBrowser",
};

function scanStatusTone(status: AppSessionBrowserScanItem["status"]): DreamStatusTone {
  switch (status) {
    case "new":
      return "success";
    case "replace":
      return "warning";
    default:
      return "muted";
  }
}

function currentChromeGuideKey(state?: CurrentResourceSniffBrowserState) {
  switch (state) {
    case "not_running":
      return "browserSource.currentChromeGuideNotRunning";
    case "remote_debugging_disabled":
      return "browserSource.currentChromeGuideRemoteDebugging";
    case "permission_denied":
      return "browserSource.currentChromeGuidePermission";
    case "unsupported_version":
    case "unsupported_browser":
      return "browserSource.currentChromeGuideUnsupportedBrowser";
    case "endpoint_unavailable":
      return "browserSource.currentChromeGuideEndpoint";
    default:
      return "";
  }
}

function formatCount(template: string, count: number) {
  return template.replace("{count}", String(count));
}

function scanStatusLabel(item: AppSessionBrowserScanItem, t: (key: string) => string) {
  switch (item.status) {
    case "new":
      return t("browserSource.statusNew");
    case "replace":
      return t("browserSource.statusReplace");
    case "unchanged":
      return t("browserSource.statusUnchanged");
    default:
      return t("browserSource.statusUnavailable");
  }
}

function scanReasonLabel(item: AppSessionBrowserScanItem, t: (key: string) => string) {
  const key = browserScanReasonTranslationKey(item.reason);
  return key ? t(key) : "";
}

function normalizedBrowserState(browser: BrowserSourceBrowser) {
  return browser.state?.trim().toLowerCase() ?? "";
}

function browserPermissionRequired(browser: BrowserSourceBrowser) {
  const state = normalizedBrowserState(browser);
  return state === "permission_required" || state === "permission_denied";
}

function browserProfileInUse(browser: BrowserSourceBrowser) {
  return normalizedBrowserState(browser) === "browser_running";
}

function browserHasAccessError(browser: BrowserSourceBrowser) {
  const state = normalizedBrowserState(browser);
  return (
    browserPermissionRequired(browser) ||
    Boolean(browser.error?.trim()) ||
    (!browser.available && state !== "no_profile_data")
  );
}

function profileAvailabilityLabel(
  profile: BrowserSourceProfile,
  t: (key: string) => string,
) {
  switch (browserProfileAvailabilityReason(profile)) {
    case "permission_required":
      return t("browserSource.profilePermissionRequired");
    case "no_profile_data":
      return t("browserSource.profileNoCookies");
    case "invalid_profile_data":
      return t("browserSource.profileInvalid");
    case "browser_running":
      return t("browserSource.profileInUse");
    case "access_required":
      return t("browserSource.profileProtectedUseCurrentChrome");
    case "protected_unsupported":
      return t("browserSource.profileProtectedUnsupported");
    case "unavailable":
      return t("browserSource.profileUnavailable");
    default:
      return "";
  }
}

function BrowserCard(props: {
  browser: BrowserSourceBrowser;
  onSelect: () => void;
}) {
  return (
    <Button
      aria-label={props.browser.label}
      className="app-browser-source-card group"
      data-browser-id={props.browser.id}
      onClick={props.onSelect}
      tone="neutral"
      type="button"
      variant="outline"
    >
      <span className="app-browser-source-card__icon">
        <BrowserBrandIcon browserId={props.browser.id} className="h-11 w-11" />
      </span>
      <span className="app-browser-source-card__label">
        {props.browser.label}
      </span>
    </Button>
  );
}

function ProfileCard(props: {
  browserId: string;
  profile: BrowserSourceProfile;
  selected: boolean;
  defaultLabel: string;
  otherProfilesLabel: string;
  methodLabel: string;
  description: string;
  selectable: boolean;
  interactionDisabled?: boolean;
  onSelect: () => void;
}) {
  const label = browserProfileDisplayLabel(
    props.profile,
    props.browserId,
    props.defaultLabel,
    props.otherProfilesLabel,
  );
  const disabled = !props.selectable || props.interactionDisabled === true;
  return (
    <Button
      aria-checked={props.selected}
      aria-disabled={disabled}
      className="app-browser-profile-card"
      disabled={disabled}
      tone={props.selected ? "accent" : "neutral"}
      variant="outline"
      data-profile-state={browserProfileAvailabilityReason(props.profile) || "ready"}
      onClick={props.onSelect}
      role="radio"
      type="button"
    >
      <span className="app-browser-profile-card__avatar">
        <UserRound aria-hidden="true" className="app-browser-profile-card__avatar-icon" />
        <span className="app-browser-profile-card__brand">
          <BrowserBrandIcon browserId={props.browserId} className="h-3.5 w-3.5" />
        </span>
      </span>
      <span className="min-w-0 flex-1">
        <span className="flex min-w-0 items-center gap-2">
          <span className="app-browser-profile-card__title">{label}</span>
          <Badge className="shrink-0" variant="secondary">
            {props.methodLabel}
          </Badge>
        </span>
        <span className="app-browser-profile-card__description">
          {props.description}
        </span>
      </span>
      <span className="app-browser-profile-card__selection" data-selected={props.selected ? "true" : "false"}>
        <Check aria-hidden="true" className="h-3 w-3" />
      </span>
    </Button>
  );
}

export function AppSessionImportSheet(props: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  sessions: AppSession[];
  resolveLabel: (session: AppSession) => string;
  onImported: () => void;
}) {
  const { t } = useI18n();
  const browserSources = useAppSessionBrowserProfileSources(props.open);
  const discoverProfiles = useDiscoverAppSessionBrowserProfiles();
  const importSessions = useImportBrowserAppSessions();
  const openPermissionGuide = useOpenBrowserDataPermissionGuide();
  const [step, setStep] = React.useState<AppSessionImportFlowStep>("browser");
  const [selection, setSelection] = React.useState<BrowserSourceSelection>({
    mode: "browser_profile",
    browserId: "",
    profileId: "",
  });
  const [scanItems, setScanItems] = React.useState<AppSessionBrowserScanItem[]>([]);
  const [scanSnapshotToken, setScanSnapshotToken] = React.useState("");
  const [selectedIds, setSelectedIds] = React.useState<Set<string>>(new Set());
  const [scanError, setScanError] = React.useState("");
  const [scanning, setScanning] = React.useState(false);
  const [importing, setImporting] = React.useState(false);
  const [discoveringBrowserId, setDiscoveringBrowserId] = React.useState("");
  const [importedCount, setImportedCount] = React.useState(0);
  const [selectedBrowser, setSelectedBrowser] = React.useState<BrowserSourceBrowser | null>(null);
  const [selectedProfileSnapshot, setSelectedProfileSnapshot] =
    React.useState<BrowserSourceProfile | null>(null);
  const [currentChromeEntryDataUpdatedAt, setCurrentChromeEntryDataUpdatedAt] =
    React.useState<number | null>(null);
  const currentChromeStatus = useCurrentResourceSniffBrowserStatus(
    { browserId: "chrome" },
    props.open &&
      step === "prerequisite" &&
      selectedBrowser?.id === "chrome" &&
      selection.mode === "current_browser",
  );
  const openRef = React.useRef(props.open);
  const stepRef = React.useRef<AppSessionImportFlowStep>(step);
  const selectionRef = React.useRef(selection);
  const snapshotTokenRef = React.useRef(scanSnapshotToken);
  const selectedBrowserIdRef = React.useRef("");
  const discoveryEpochRef = React.useRef(0);
  const operationEpochRef = React.useRef(0);
  const operationBusyRef = React.useRef<AppSessionImportOperation>("");

  openRef.current = props.open;
  stepRef.current = step;
  selectionRef.current = selection;
  snapshotTokenRef.current = scanSnapshotToken;

  const flowSnapshot = (): AppSessionImportFlowSnapshot => ({
    open: openRef.current,
    step: stepRef.current,
    discoveryEpoch: discoveryEpochRef.current,
    operationEpoch: operationEpochRef.current,
    selectedBrowserId: selectedBrowserIdRef.current,
    selection: selectionRef.current,
    snapshotToken: snapshotTokenRef.current,
  });

  const invalidatePendingRequests = () => {
    discoveryEpochRef.current += 1;
    operationEpochRef.current += 1;
    operationBusyRef.current = "";
  };

  React.useEffect(() => {
    invalidatePendingRequests();
    selectedBrowserIdRef.current = "";
    stepRef.current = "browser";
    selectionRef.current = { mode: "browser_profile", browserId: "", profileId: "" };
    snapshotTokenRef.current = "";
    setStep("browser");
    setSelection({ mode: "browser_profile", browserId: "", profileId: "" });
    setSelectedBrowser(null);
    setSelectedProfileSnapshot(null);
    setDiscoveringBrowserId("");
    setScanItems([]);
    setScanSnapshotToken("");
    setSelectedIds(new Set());
    setScanError("");
    setScanning(false);
    setImporting(false);
    setImportedCount(0);
    setCurrentChromeEntryDataUpdatedAt(null);
  }, [props.open]);

  const browsers = browserSources.data?.filter((browser) => browser.available) ?? [];
  const profiles = selectedBrowser?.profiles ?? [];
  const protectedProfiles = profiles.filter((profile) => {
    const reason = browserProfileAvailabilityReason(profile);
    return reason === "access_required" || reason === "protected_unsupported";
  });
  const directProfiles = profiles.filter((profile) => {
    const reason = browserProfileAvailabilityReason(profile);
    return reason !== "access_required" && reason !== "protected_unsupported";
  });
  const methodProfiles = directProfiles.filter(browserProfileCanEnterPrerequisite);
  const availableProfiles = directProfiles.filter((profile) => profile.available);
  const onlyProtectedProfiles =
    profiles.length > 0 && directProfiles.length === 0;
  // A v20 Chrome profile has no usable copied-profile path. Omit it entirely;
  // the current-session CDP row above is the only actionable Chrome choice.
  // Browsers without an authorized live-session path still need an explanation.
  const protectedProfileNoticeKey = selectedBrowser?.id === "chrome"
    ? ""
    : protectedProfiles.length > 0
      ? "browserSource.profileProtectedUnsupported"
      : "";
  const selectedProfileResolution = selection.mode === "browser_profile"
    ? resolveBrowserProfilePrerequisite(
        profiles,
        selection.profileId,
        selectedProfileSnapshot,
      )
    : {
        profile: null,
        presentInDiscovery: false,
        ready: false,
      };
  const selectedProfile = selectedProfileResolution.profile;
  const selectedProfileReady = selectedProfileResolution.ready;
  const selectedProfileCanEnterPrerequisite = Boolean(
    selectedProfile && browserProfileCanEnterPrerequisite(selectedProfile),
  );
  const selectedProfileLabel = selectedProfile
    ? browserProfileDisplayLabel(
        selectedProfile,
        selectedBrowser?.id ?? selection.browserId,
        t("browserSource.defaultProfile"),
        t("browserSource.otherProfiles"),
      )
    : "";
  const selectedProfileAddress = selectedProfile?.displayPath?.trim() ?? "";
  const selectedProfileDetail =
    selectedProfileAddress || selectedProfile?.subtitle?.trim() || "";
  const selectedProfileStatusLabel = selectedProfile
    ? selectedProfileReady
      ? t("browserSource.profileReady")
      : selectedProfileResolution.presentInDiscovery
        ? profileAvailabilityLabel(selectedProfile, t) ||
          t("browserSource.profileUnavailable")
        : t("browserSource.profileUnavailable")
    : "";
  const currentBrowserSelected = selection.mode === "current_browser";
  const currentChromeStatusFresh = Boolean(
    currentChromeEntryDataUpdatedAt !== null &&
      currentChromeStatus.dataUpdatedAt > currentChromeEntryDataUpdatedAt,
  );
  const currentChromeReady = Boolean(
    currentChromeStatusFresh &&
      currentChromeStatus.data?.ready === true &&
      !currentChromeStatus.isFetching &&
      !currentChromeStatus.isError &&
      !currentChromeStatus.isRefetchError,
  );
  const currentChromeStatusLabel = currentChromeStatus.isFetching
    ? t("browserSource.currentChromeChecking")
    : currentChromeStatus.isError || currentChromeStatus.isRefetchError
      ? t("browserSource.currentChromeStatusFailed")
      : currentChromeStatusFresh && currentChromeStatus.data
        ? t(CURRENT_CHROME_STATUS_KEYS[currentChromeStatus.data.state])
        : t("browserSource.currentChromeChecking");
  const currentChromeGuide = currentChromeStatusFresh && currentChromeStatus.data
    ? currentChromeGuideKey(currentChromeStatus.data.state)
    : "";
  const discovering = Boolean(
    discoveringBrowserId && discoveringBrowserId === selectedBrowser?.id,
  );
  const selectableItems = scanItems.filter(
    (item) => item.selectable && item.status !== "unchanged" && item.status !== "unavailable",
  );
  const selectedCount = selectableItems.filter((item) => selectedIds.has(item.appSessionId)).length;
  const scanSourceNotice = scanItems.find((item) =>
    isBrowserScanSourceNotice(item.reason)
  );

  const discoverBrowser = async (browser: BrowserSourceBrowser) => {
    const requestStep = stepRef.current;
    if (requestStep !== "method" && requestStep !== "prerequisite") {
      return;
    }
    const request = {
      epoch: ++discoveryEpochRef.current,
      browserId: browser.id,
      step: requestStep,
    };
    setDiscoveringBrowserId(browser.id);
    setScanError("");
    try {
      const discovered = await discoverProfiles.mutateAsync({
        browserId: browser.id,
        browserLabel: browser.label,
      });
      if (!isCurrentBrowserDiscovery(flowSnapshot(), request)) {
        return;
      }
      setSelectedBrowser(discovered);
      const selectedProfileId = selectionRef.current.mode === "browser_profile"
        ? selectionRef.current.profileId
        : "";
      const refreshedSelectedProfile = discovered.profiles.find(
        (profile) => profile.id === selectedProfileId,
      );
      if (refreshedSelectedProfile) {
        setSelectedProfileSnapshot(refreshedSelectedProfile);
      }
    } catch {
      if (!isCurrentBrowserDiscovery(flowSnapshot(), request)) {
        return;
      }
      setSelectedBrowser({
        ...browser,
        available: false,
        state: "unavailable",
        error: "profile_discovery_failed",
        profiles: [],
      });
    } finally {
      if (isCurrentBrowserDiscovery(flowSnapshot(), request)) {
        setDiscoveringBrowserId("");
      }
    }
  };

  const refreshSources = () => {
    if (hasActiveAppSessionImportOperation(operationBusyRef)) {
      return;
    }
    if (selectedBrowser) {
      void discoverBrowser(selectedBrowser);
    }
  };

  const selectBrowser = (browser: BrowserSourceBrowser) => {
    if (hasActiveAppSessionImportOperation(operationBusyRef)) {
      return;
    }
    operationEpochRef.current += 1;
    selectedBrowserIdRef.current = browser.id;
    stepRef.current = "method";
    const nextSelection: BrowserSourceSelection = {
      mode: "browser_profile",
      browserId: browser.id,
      profileId: "",
    };
    selectionRef.current = nextSelection;
    setSelectedBrowser(browser);
    setSelectedProfileSnapshot(null);
    setSelection(nextSelection);
    setCurrentChromeEntryDataUpdatedAt(
      browser.id === "chrome" ? currentChromeStatus.dataUpdatedAt : null,
    );
    setScanError("");
    setScanning(false);
    setImporting(false);
    setScanItems([]);
    snapshotTokenRef.current = "";
    setScanSnapshotToken("");
    setSelectedIds(new Set());
    setStep("method");
    void discoverBrowser(browser);
  };

  const handleScan = async () => {
    const requestSelection = { ...selectionRef.current };
    if (
      stepRef.current !== "prerequisite" ||
      !requestSelection.browserId ||
      (requestSelection.mode !== "current_browser" && !requestSelection.profileId) ||
      (requestSelection.mode === "current_browser"
        ? !currentChromeReady
        : !selectedProfileReady) ||
      !tryBeginAppSessionImportOperation(operationBusyRef, "scan")
    ) {
      return;
    }
    const request = {
      epoch: ++operationEpochRef.current,
      step: "prerequisite" as const,
      selection: requestSelection,
    };
    setScanning(true);
    setScanError("");
    snapshotTokenRef.current = "";
    setScanSnapshotToken("");
    try {
      const result = await scanBrowserAppSessions(requestSelection);
      if (!isCurrentImportOperation(flowSnapshot(), request)) {
        return;
      }
      if (!result.snapshotToken) {
        throw new Error("missing browser scan snapshot");
      }
      setScanItems(result.items);
      snapshotTokenRef.current = result.snapshotToken;
      setScanSnapshotToken(result.snapshotToken);
      setSelectedIds(
        new Set(
          result.items
            .filter(
              (item) =>
                item.selectable &&
                item.status !== "unchanged" &&
                item.status !== "unavailable",
            )
            .map((item) => item.appSessionId),
        ),
      );
      stepRef.current = "review";
      setStep("review");
    } catch {
      if (!isCurrentImportOperation(flowSnapshot(), request)) {
        return;
      }
      setScanItems([]);
      setSelectedIds(new Set());
      setScanError(t("browserSource.scanFailed"));
    } finally {
      if (openRef.current && operationEpochRef.current === request.epoch) {
        operationBusyRef.current = "";
        setScanning(false);
      }
    }
  };

  const handleImport = async () => {
    const appSessionIds = selectableItems
      .filter((item) => selectedIds.has(item.appSessionId))
      .map((item) => item.appSessionId);
    const requestSelection = { ...selectionRef.current };
    const requestSnapshotToken = snapshotTokenRef.current;
    if (
      appSessionIds.length === 0 ||
      !requestSnapshotToken ||
      !tryBeginAppSessionImportOperation(operationBusyRef, "import")
    ) {
      return;
    }
    const request = {
      epoch: ++operationEpochRef.current,
      step: "review" as const,
      selection: requestSelection,
      snapshotToken: requestSnapshotToken,
    };
    setImporting(true);
    setScanError("");
    try {
      const result = await importSessions.mutateAsync({
        ...requestSelection,
        snapshotToken: requestSnapshotToken,
        appSessionIds,
      });
      if (!isCurrentImportOperation(flowSnapshot(), request)) {
        return;
      }
      if (result.importedIds.length === 0) {
        setScanError(t("browserSource.importNone"));
        return;
      }
      setImportedCount(result.importedIds.length);
      stepRef.current = "complete";
      setStep("complete");
      props.onImported();
    } catch {
      if (!isCurrentImportOperation(flowSnapshot(), request)) {
        return;
      }
      setScanError(t("browserSource.importFailed"));
    } finally {
      // The backend atomically consumes every valid token on the first import
      // attempt. A retry must start from a fresh scan.
      if (openRef.current && operationEpochRef.current === request.epoch) {
        operationBusyRef.current = "";
        snapshotTokenRef.current = "";
        setScanSnapshotToken("");
        setImporting(false);
      }
    }
  };

  const toggleAll = () => {
    if (hasActiveAppSessionImportOperation(operationBusyRef)) {
      return;
    }
    if (selectedCount === selectableItems.length) {
      setSelectedIds(new Set());
      return;
    }
    setSelectedIds(new Set(selectableItems.map((item) => item.appSessionId)));
  };

  const sessionById = React.useMemo(
    () => new Map(props.sessions.map((session) => [session.id, session])),
    [props.sessions],
  );

  const description = step === "browser"
    ? t("browserSource.chooseBrowserDescription")
    : step === "method"
      ? t("browserSource.chooseProfileDescription")
      : step === "prerequisite"
        ? t("browserSource.prerequisiteDescription")
        : step === "review"
          ? t("browserSource.reviewDescription")
          : t("browserSource.appSessionDescription");

  const browserAccessError = selectedBrowser && availableProfiles.length === 0 && browserHasAccessError(selectedBrowser);
  const browserPermissionNeeded = Boolean(browserAccessError && selectedBrowser && browserPermissionRequired(selectedBrowser));
  const browserIsRunning = Boolean(browserAccessError && selectedBrowser && browserProfileInUse(selectedBrowser));

  const handleSheetOpenChange = (open: boolean) => {
    if (!open) {
      openRef.current = false;
      invalidatePendingRequests();
      selectedBrowserIdRef.current = "";
      setDiscoveringBrowserId("");
      setScanning(false);
      setImporting(false);
    }
    props.onOpenChange(open);
  };

  const returnToBrowserStep = () => {
    if (hasActiveAppSessionImportOperation(operationBusyRef)) {
      return;
    }
    invalidatePendingRequests();
    selectedBrowserIdRef.current = "";
    stepRef.current = "browser";
    const emptySelection: BrowserSourceSelection = {
      mode: "browser_profile",
      browserId: "",
      profileId: "",
    };
    selectionRef.current = emptySelection;
    snapshotTokenRef.current = "";
    setStep("browser");
    setSelection(emptySelection);
    setSelectedBrowser(null);
    setSelectedProfileSnapshot(null);
    setCurrentChromeEntryDataUpdatedAt(null);
    setDiscoveringBrowserId("");
    setScanItems([]);
    setScanSnapshotToken("");
    setSelectedIds(new Set());
    setScanError("");
    setScanning(false);
    setImporting(false);
  };

  const continueToPrerequisiteStep = () => {
    if (hasActiveAppSessionImportOperation(operationBusyRef)) {
      return;
    }
    const currentSelection = selectionRef.current;
    if (
      discovering ||
      !currentSelection.browserId ||
      (currentSelection.mode !== "current_browser" &&
        (currentSelection.mode !== "browser_profile" ||
          !selectedProfileCanEnterPrerequisite))
    ) {
      return;
    }
    operationEpochRef.current += 1;
    stepRef.current = "prerequisite";
    snapshotTokenRef.current = "";
    if (currentSelection.mode === "current_browser") {
      setCurrentChromeEntryDataUpdatedAt(currentChromeStatus.dataUpdatedAt);
    }
    setStep("prerequisite");
    setScanSnapshotToken("");
    setScanItems([]);
    setSelectedIds(new Set());
    setScanError("");
    setImporting(false);
  };

  const returnToMethodStep = () => {
    if (hasActiveAppSessionImportOperation(operationBusyRef)) {
      return;
    }
    discoveryEpochRef.current += 1;
    operationEpochRef.current += 1;
    stepRef.current = "method";
    snapshotTokenRef.current = "";
    setStep("method");
    setDiscoveringBrowserId("");
    setScanSnapshotToken("");
    setScanItems([]);
    setSelectedIds(new Set());
    setScanError("");
    setScanning(false);
    setImporting(false);
  };

  const returnToPrerequisiteStep = () => {
    if (hasActiveAppSessionImportOperation(operationBusyRef)) {
      return;
    }
    operationEpochRef.current += 1;
    stepRef.current = "prerequisite";
    snapshotTokenRef.current = "";
    if (selectionRef.current.mode === "current_browser") {
      setCurrentChromeEntryDataUpdatedAt(currentChromeStatus.dataUpdatedAt);
    }
    setStep("prerequisite");
    setScanSnapshotToken("");
    setScanItems([]);
    setSelectedIds(new Set());
    setScanError("");
    setImporting(false);
  };

  const selectProfile = (profileId: string) => {
    if (hasActiveAppSessionImportOperation(operationBusyRef)) {
      return;
    }
    const profile = directProfiles.find((item) => item.id === profileId);
    if (!profile || !browserProfileCanEnterPrerequisite(profile)) {
      return;
    }
    operationEpochRef.current += 1;
    const nextSelection: BrowserSourceSelection = {
      ...selectionRef.current,
      mode: "browser_profile",
      profileId,
    };
    selectionRef.current = nextSelection;
    snapshotTokenRef.current = "";
    setSelectedProfileSnapshot(profile);
    setSelection(nextSelection);
    setScanItems([]);
    setScanSnapshotToken("");
    setSelectedIds(new Set());
    setScanError("");
  };

  const selectCurrentChrome = () => {
    if (hasActiveAppSessionImportOperation(operationBusyRef)) {
      return;
    }
    operationEpochRef.current += 1;
    const nextSelection: BrowserSourceSelection = {
      mode: "current_browser",
      browserId: "chrome",
      profileId: "",
    };
    selectionRef.current = nextSelection;
    snapshotTokenRef.current = "";
    setSelectedProfileSnapshot(null);
    setSelection(nextSelection);
    setScanItems([]);
    setScanSnapshotToken("");
    setSelectedIds(new Set());
    setScanError("");
  };

  return (
    <Sheet open={props.open} onOpenChange={handleSheetOpenChange}>
      <SheetContent centered size="lg">
        <SheetHeader className="pb-1">
          <SheetHeading>
            <SheetTitle className="app-session-import-heading">
              <CloudSync aria-hidden="true" className="app-session-import-heading__icon" />
              <span>{t("browserSource.appSessionTitle")}</span>
            </SheetTitle>
            <SheetDescription className="sr-only">{description}</SheetDescription>
          </SheetHeading>
          <SheetCloseButton aria-label={t("common.close")} />
        </SheetHeader>

        <SheetBody className="pt-3">
          {step === "browser" ? (
            <div>
              {browserSources.isLoading ? (
                <div className="flex flex-wrap justify-center gap-x-6 gap-y-3" role="status" aria-label={t("browserSource.loading")}>
                  {[0, 1, 2].map((item) => (
                    <div className="app-session-import-skeleton-card" key={item}>
                      <span className="app-session-import-skeleton-card__icon" />
                      <span className="app-session-import-skeleton-card__label" />
                    </div>
                  ))}
                </div>
              ) : browserSources.isError ? (
                <div className="app-dream-status-message app-session-import-centered-message p-4" data-intent="danger" role="alert">
                  <p>{t("browserSource.loadFailed")}</p>
                  <Button className="mt-3" onClick={() => void browserSources.refetch()} size="compact" type="button" variant="outline">
                    <RefreshCw className="h-3.5 w-3.5" />
                    {t("browserSource.refresh")}
                  </Button>
                </div>
              ) : browsers.length === 0 ? (
                <div className="app-session-import-empty-card px-4 py-10">
                  {t("browserSource.noBrowsers")}
                </div>
              ) : (
                <div
                  aria-label={t("browserSource.chooseBrowser")}
                  className="flex flex-wrap justify-center gap-x-6 gap-y-3"
                  role="group"
                >
                  {browsers.map((browser) => (
                    <BrowserCard
                      browser={browser}
                      key={browser.id}
                      onSelect={() => selectBrowser(browser)}
                    />
                  ))}
                </div>
              )}
            </div>
          ) : step === "method" ? (
            <div className="space-y-4">
              {selectedBrowser ? (
                <div className="app-session-import-browser-heading">
                  <span className="flex h-12 w-12 shrink-0 items-center justify-center">
                    <BrowserBrandIcon browserId={selectedBrowser.id} className="h-10 w-10" />
                  </span>
                  <span className="app-session-import-browser-heading__label">{selectedBrowser.label}</span>
                </div>
              ) : null}

              {discovering ? (
                <div className="app-session-import-loading" role="status">
                  <Loader2 aria-hidden="true" className="h-4 w-4 app-motion-spin" />
                  <span className="sr-only">{t("browserSource.loading")}</span>
                </div>
              ) : (
                <>
                  <div
                    aria-label={t("browserSource.chooseProfile")}
                    className="grid grid-cols-1 gap-2"
                    data-profile-layout="single-column"
                    data-profile-method-selection="true"
                    role="radiogroup"
                  >
                    {selectedBrowser?.id === "chrome" ? (
                      <Button
                        aria-checked={currentBrowserSelected}
                        className="app-browser-profile-card"
                        data-profile-source="current-browser"
                        disabled={scanning || importing}
                        onClick={selectCurrentChrome}
                        role="radio"
                        tone={currentBrowserSelected ? "accent" : "neutral"}
                        type="button"
                        variant="outline"
                      >
                        <span className="app-browser-profile-card__avatar">
                          <BrowserBrandIcon browserId="chrome" className="h-6 w-6" />
                        </span>
                        <span className="min-w-0 flex-1">
                          <span className="flex min-w-0 items-center gap-2">
                            <span className="app-browser-profile-card__title">
                              {t("browserSource.currentChromeSession")}
                            </span>
                            <Badge className="shrink-0" variant="secondary">
                              {t("browserSource.authorizedRead")}
                            </Badge>
                          </span>
                          <span className="app-browser-profile-card__description">
                            {t("browserSource.currentChromeCDPDescription")}
                          </span>
                        </span>
                        <span className="app-browser-profile-card__selection" data-selected={currentBrowserSelected ? "true" : "false"}>
                          <Check aria-hidden="true" className="h-3 w-3" />
                        </span>
                      </Button>
                    ) : null}
                    {methodProfiles.map((profile) => (
                      <ProfileCard
                        browserId={selectedBrowser?.id ?? selection.browserId}
                        defaultLabel={t("browserSource.defaultProfile")}
                        description={t("browserSource.copyAndParseDescription")}
                        interactionDisabled={scanning || importing}
                        key={profile.id}
                        methodLabel={t("browserSource.copyAndParse")}
                        onSelect={() => selectProfile(profile.id)}
                        otherProfilesLabel={t("browserSource.otherProfiles")}
                        profile={profile}
                        selectable={browserProfileCanEnterPrerequisite(profile)}
                        selected={selectedProfile?.id === profile.id}
                      />
                    ))}
                  </div>
                  {protectedProfileNoticeKey ? (
                    <div
                      className="app-dream-status-message app-session-import-prominent-message px-3 py-2.5"
                      data-intent="warning"
                      data-profile-protection-notice="true"
                      role="status"
                    >
                      {t(protectedProfileNoticeKey)}
                    </div>
                  ) : null}
                  {methodProfiles.length === 0 &&
                    selectedBrowser?.id !== "chrome" &&
                    protectedProfiles.length === 0 ? (
                    <div className="app-session-import-empty-card px-4 py-8">
                      <p>{t("browserSource.noProfiles")}</p>
                      <Button
                        className="mt-3"
                        disabled={discovering}
                        onClick={refreshSources}
                        size="compact"
                        type="button"
                        variant="outline"
                      >
                        <RefreshCw className={cn("h-3.5 w-3.5", discovering && "app-motion-spin")} />
                        {t("browserSource.refresh")}
                      </Button>
                    </div>
                  ) : null}
                </>
              )}
            </div>
          ) : step === "prerequisite" ? (
            <div className="space-y-4" data-profile-prerequisite-step="true">
              {selectedBrowser ? (
                <div className="app-session-import-browser-heading">
                  <span className="flex h-12 w-12 shrink-0 items-center justify-center">
                    <BrowserBrandIcon browserId={selectedBrowser.id} className="h-10 w-10" />
                  </span>
                  <span className="app-session-import-browser-heading__label">{selectedBrowser.label}</span>
                </div>
              ) : null}

              {currentBrowserSelected ? (
                <div
                  className="app-session-import-prerequisite-card"
                  data-profile-prerequisite="current-browser"
                >
                  <span className="app-browser-profile-card__avatar">
                    <BrowserBrandIcon browserId="chrome" className="h-6 w-6" />
                  </span>
                  <span className="min-w-0 flex-1">
                    <span className="flex min-w-0 items-center gap-2">
                      <span className="app-browser-profile-card__title">
                        {t("browserSource.currentChromeSession")}
                      </span>
                      <Badge className="shrink-0" variant="secondary">
                        {t("browserSource.authorizedRead")}
                      </Badge>
                    </span>
                    <span className="app-session-import-prerequisite-status" data-intent={currentChromeReady ? "success" : "warning"}>
                      {currentChromeStatusLabel}
                      {currentChromeGuide ? (
                        <span className="app-session-import-guide-detail ml-1">
                          {t(currentChromeGuide)}
                          {currentChromeStatus.data?.state ===
                          "remote_debugging_disabled" ? (
                            <code className="app-session-import-guide-code ml-1 select-all px-1 py-0.5">
                              {CURRENT_CHROME_REMOTE_DEBUGGING_URL}
                            </code>
                          ) : null}
                        </span>
                      ) : null}
                    </span>
                  </span>
                  <Button
                    aria-label={t("browserSource.refresh")}
                    className="h-8 w-8 shrink-0"
                    disabled={currentChromeStatus.isFetching || scanning || importing}
                    onClick={() => {
                      setCurrentChromeEntryDataUpdatedAt(currentChromeStatus.dataUpdatedAt);
                      void currentChromeStatus.refetch();
                    }}
                    size="compactIcon"
                    title={t("browserSource.refresh")}
                    type="button"
                    variant="ghost"
                  >
                    <RefreshCw
                      className={cn(
                        "h-3.5 w-3.5",
                        currentChromeStatus.isFetching && "app-motion-spin",
                      )}
                    />
                  </Button>
                </div>
              ) : selectedProfile ? (
                <>
                  <div
                    className="app-session-import-prerequisite-card"
                    data-profile-prerequisite="browser-profile"
                    data-profile-state={
                      selectedProfileResolution.presentInDiscovery
                        ? browserProfileAvailabilityReason(selectedProfile) || "ready"
                        : "unavailable"
                    }
                  >
                    <span className="app-browser-profile-card__avatar">
                      <UserRound aria-hidden="true" className="app-browser-profile-card__avatar-icon" />
                      <span className="app-browser-profile-card__brand">
                        <BrowserBrandIcon browserId={selectedBrowser?.id ?? selection.browserId} className="h-3.5 w-3.5" />
                      </span>
                    </span>
                    <span className="min-w-0 flex-1">
                      <span className="flex min-w-0 items-center gap-2">
                        <span className="app-browser-profile-card__title">{selectedProfileLabel}</span>
                        <Badge className="shrink-0" variant="secondary">
                          {t("browserSource.copyAndParse")}
                        </Badge>
                      </span>
                      {selectedProfileDetail ? (
                        <span
                          className="app-browser-profile-card__detail"
                          data-profile-address={selectedProfileAddress ? "true" : undefined}
                          title={selectedProfileAddress || undefined}
                        >
                          {selectedProfileDetail}
                        </span>
                      ) : null}
                      <span className="app-session-import-prerequisite-status" data-intent={selectedProfileReady ? "success" : "warning"}>
                        {selectedProfileReady ? (
                          <Check aria-hidden="true" className="h-3.5 w-3.5 shrink-0" />
                        ) : (
                          <AlertTriangle aria-hidden="true" className="h-3.5 w-3.5 shrink-0" />
                        )}
                        <span>{selectedProfileStatusLabel}</span>
                      </span>
                    </span>
                    {!selectedProfileReady ? (
                      <Button
                        aria-label={t("browserSource.refresh")}
                        className="h-8 w-8 shrink-0"
                        disabled={discovering}
                        onClick={refreshSources}
                        size="compactIcon"
                        title={t("browserSource.refresh")}
                        type="button"
                        variant="ghost"
                      >
                        <RefreshCw className={cn("h-3.5 w-3.5", discovering && "app-motion-spin")} />
                      </Button>
                    ) : null}
                  </div>
                  {browserAccessError && !onlyProtectedProfiles ? (
                    <div className="app-session-import-prerequisite-alert" role="alert">
                      <StatusBadge icon={<AlertTriangle />} tone="warning">
                        <span>
                          {browserPermissionNeeded
                            ? t("browserSource.accessRequired")
                            : browserIsRunning
                              ? t("browserSource.profileInUse")
                              : t("browserSource.statusUnavailable")}
                        </span>
                      </StatusBadge>
                      {browserPermissionNeeded ? (
                        <div className="flex shrink-0 items-center gap-2">
                          <Button
                            disabled={openPermissionGuide.isPending}
                            onClick={() => {
                              setScanError("");
                              void openPermissionGuide.mutateAsync()
                                .catch(() => setScanError(t("browserSource.permissionGuideFailed")));
                            }}
                            size="compact"
                            type="button"
                            variant="outline"
                          >
                            {openPermissionGuide.isPending ? (
                              <Loader2 className="h-3.5 w-3.5 app-motion-spin" />
                            ) : null}
                            {t("browserSource.openPermissionSettings")}
                          </Button>
                          <Button
                            aria-label={t("browserSource.refresh")}
                            disabled={discovering}
                            onClick={refreshSources}
                            size="compactIcon"
                            title={t("browserSource.refresh")}
                            type="button"
                            variant="outline"
                          >
                            <RefreshCw className={cn("h-3.5 w-3.5", discovering && "app-motion-spin")} />
                          </Button>
                        </div>
                      ) : (
                        <Button
                          disabled={discovering}
                          onClick={refreshSources}
                          size="compact"
                          type="button"
                          variant="outline"
                        >
                          <RefreshCw className={cn("h-3.5 w-3.5", discovering && "app-motion-spin")} />
                          {t("browserSource.refresh")}
                        </Button>
                      )}
                    </div>
                  ) : null}
                </>
              ) : (
                <div className="app-session-import-empty-card px-4 py-8">
                  {t("browserSource.noProfiles")}
                </div>
              )}

              {scanError ? (
                <div className="app-dream-status-message px-3 py-2" data-intent="danger" role="alert">
                  {scanError}
                </div>
              ) : null}
            </div>
          ) : step === "review" ? (
            <div className="space-y-3">
              {scanSourceNotice ? (
                <div
                  className="app-dream-status-message app-session-import-prominent-message px-3 py-2.5"
                  data-intent="warning"
                  role="alert"
                >
                  {scanReasonLabel(scanSourceNotice, t)}
                </div>
              ) : null}
              <div className="flex items-center justify-between gap-3">
                <span className="app-session-import-selection-count">
                  {formatCount(t("browserSource.selectedCount"), selectedCount)}
                </span>
                <Button
                  disabled={selectableItems.length === 0 || importing}
                  onClick={toggleAll}
                  size="compact"
                  type="button"
                  variant="ghost"
                >
                  {t("browserSource.selectAll")}
                </Button>
              </div>
              <div className="grid gap-2">
                {scanItems.length === 0 ? (
                  <div className="app-session-import-empty-card px-4 py-8">
                    {t("browserSource.noSessionsFound")}
                  </div>
                ) : (
                  scanItems.map((item) => {
                    const session = sessionById.get(item.appSessionId);
                    const secondaryLabel = item.accountLabel || scanReasonLabel(item, t);
                    const disabled =
                      !item.selectable || item.status === "unchanged" || item.status === "unavailable";
                    const checked = selectedIds.has(item.appSessionId) && !disabled;
                    const label = item.label || (session ? props.resolveLabel(session) : item.siteKey || item.appSessionId);
                    return (
                      <label
                        className="app-session-import-scan-item"
                        data-disabled={disabled ? "true" : "false"}
                        key={item.appSessionId}
                      >
                        <input
                          checked={checked}
                          className="app-session-import-checkbox"
                          disabled={disabled || importing}
                          onChange={(event) => {
                            if (hasActiveAppSessionImportOperation(operationBusyRef)) {
                              return;
                            }
                            setSelectedIds((current) => {
                              const next = new Set(current);
                              if (event.target.checked) {
                                next.add(item.appSessionId);
                              } else {
                                next.delete(item.appSessionId);
                              }
                              return next;
                            });
                          }}
                          type="checkbox"
                        />
                        <span className="app-session-import-site-icon">
                          <SiteBrandIcon className="h-5 w-5" siteKey={item.siteKey || session?.siteKey || ""} />
                        </span>
                        <span className="min-w-0 flex-1">
                          <span className="app-session-import-scan-item__title">{label}</span>
                          {secondaryLabel ? (
                            <span className="app-session-import-scan-item__subtitle">
                              {secondaryLabel}
                            </span>
                          ) : null}
                        </span>
                        <StatusBadge className="shrink-0" tone={scanStatusTone(item.status)}>
                          {scanStatusLabel(item, t)}
                        </StatusBadge>
                      </label>
                    );
                  })
                )}
              </div>
            </div>
          ) : (
            <div className="app-session-import-complete flex min-h-64 flex-col items-center justify-center">
              <span className="app-session-import-success-icon mb-4">
                <Check className="h-6 w-6" />
              </span>
              <h3 className="app-session-import-complete__title">
                {t("browserSource.importComplete")}
              </h3>
              <p className="app-session-import-complete__copy">
                {formatCount(t("browserSource.importedCount"), importedCount)}
              </p>
            </div>
          )}
          {scanError && step === "review" ? (
            <div className="app-dream-status-message mt-3 px-3 py-2" data-intent="danger" role="alert">
              {scanError}
            </div>
          ) : null}
        </SheetBody>

        {step !== "browser" ? (
          <SheetFooter className="pt-0">
            {step === "method" ? (
              <>
                <Button
                  type="button"
                  variant="outline"
                  onClick={returnToBrowserStep}
                >
                  {t("browserSource.back")}
                </Button>
                <Button
                  disabled={
                    discovering ||
                    (!currentBrowserSelected && !selectedProfileCanEnterPrerequisite)
                  }
                  onClick={continueToPrerequisiteStep}
                  type="button"
                >
                  {t("xiadown.actions.next")}
                </Button>
              </>
            ) : step === "prerequisite" ? (
              <>
                <Button
                  disabled={scanning}
                  onClick={returnToMethodStep}
                  type="button"
                  variant="outline"
                >
                  {t("browserSource.back")}
                </Button>
                <Button
                  disabled={
                    scanning ||
                    discovering ||
                    (currentBrowserSelected
                      ? !currentChromeReady
                      : !selectedProfileReady)
                  }
                  onClick={() => void handleScan()}
                  type="button"
                >
                  {scanning ? <Loader2 className="h-4 w-4 app-motion-spin" /> : <ScanSearch className="h-4 w-4" />}
                  {t("browserSource.scan")}
                </Button>
              </>
            ) : step === "review" ? (
              <>
                <Button
                  disabled={importing}
                  type="button"
                  variant="outline"
                  onClick={returnToPrerequisiteStep}
                >
                  {t("browserSource.back")}
                </Button>
                <Button
                  disabled={selectedCount === 0 || !scanSnapshotToken || importing}
                  onClick={() => void handleImport()}
                  type="button"
                >
                  {importing ? <Loader2 className="h-4 w-4 app-motion-spin" /> : null}
                  {t("browserSource.import")}
                </Button>
              </>
            ) : (
              <Button type="button" onClick={() => handleSheetOpenChange(false)}>
                {t("common.close")}
              </Button>
            )}
          </SheetFooter>
        ) : null}
      </SheetContent>
    </Sheet>
  );
}
