import { ChevronLeft, Loader2, RefreshCw } from "lucide-react";
import * as React from "react";

import { cn } from "@/lib/utils";
import { canUseCurrentChrome } from "@/app/main/currentChromeStatusGate";
import { BrowserBrandIcon } from "@/shared/browser-source/BrowserBrandIcon";
import type {
  BrowserSourceBrowser,
  BrowserSourceProfile,
  BrowserSourceSelection,
} from "@/shared/contracts/browserSources";
import type {
  CurrentResourceSniffBrowserState,
} from "@/shared/contracts/library";
import { useI18n } from "@/shared/i18n";
import { useBrowserSources } from "@/shared/query/browserSources";
import { useCurrentResourceSniffBrowserStatus } from "@/shared/query/library";
import { Button } from "@/shared/ui/button";
import { StatusBadge } from "@/shared/ui/status-badge";

type SniffSourceMode = "current_browser" | "xiadown_profile";

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

function defaultProfile(
  browser: BrowserSourceBrowser,
  label: string,
): BrowserSourceProfile {
  return {
    id: "",
    browserId: browser.id,
    browserLabel: browser.label,
    label,
    isDefault: true,
    available: true,
    virtual: true,
  };
}

export function NewSniffSourceSteps(props: {
  confirming?: boolean;
  onConfirm: (selection: BrowserSourceSelection) => void | Promise<void>;
}) {
  const { t } = useI18n();
  const sources = useBrowserSources("network_sniff", true);
  const [step, setStep] = React.useState<"browser" | "profile">("browser");
  const [browserId, setBrowserId] = React.useState("");
  const [sourceMode, setSourceMode] = React.useState<SniffSourceMode | null>(
    null,
  );
  const [currentChromeEntryDataUpdatedAt, setCurrentChromeEntryDataUpdatedAt] =
    React.useState<number | null>(null);

  const browsers =
    sources.data?.browsers.filter(
      (browser) => browser.available && browser.id !== "safari",
    ) ?? [];
  const selectedBrowser =
    browsers.find((browser) => browser.id === browserId) ?? null;
  const currentChromeStatus = useCurrentResourceSniffBrowserStatus(
    { browserId: "chrome" },
    step === "profile" && selectedBrowser?.id === "chrome",
  );
  const managedProfiles = selectedBrowser
    ? (sources.data?.xiadownProfiles ?? []).filter(
        (profile) =>
          profile.available &&
          !profile.redundant &&
          profile.browserId === selectedBrowser.id,
      )
    : [];
  const managedProfile = selectedBrowser
    ? managedProfiles.find((profile) => profile.isDefault) ??
      defaultProfile(selectedBrowser, t("browserSource.defaultProfile"))
    : null;
  const managedProfileLabel =
    managedProfile?.label?.trim() || t("browserSource.defaultProfile");
  const currentChromeReady = canUseCurrentChrome({
    data: currentChromeStatus.data,
    dataUpdatedAt: currentChromeStatus.dataUpdatedAt,
    entryDataUpdatedAt: currentChromeEntryDataUpdatedAt,
    isFetching: currentChromeStatus.isFetching,
    isError: currentChromeStatus.isError,
    isRefetchError: currentChromeStatus.isRefetchError,
  });
  const currentChromeStatusLabel = currentChromeStatus.isFetching
    ? t("browserSource.currentChromeChecking")
    : currentChromeStatus.isError || currentChromeStatus.isRefetchError
      ? t("browserSource.currentChromeStatusFailed")
      : currentChromeStatus.data
        ? t(CURRENT_CHROME_STATUS_KEYS[currentChromeStatus.data.state])
        : t("browserSource.currentChromeChecking");

  const selectBrowser = (browser: BrowserSourceBrowser) => {
    setBrowserId(browser.id);
    setSourceMode(null);
    setCurrentChromeEntryDataUpdatedAt(
      browser.id === "chrome" ? currentChromeStatus.dataUpdatedAt : null,
    );
    setStep("profile");
  };

  const goBack = () => {
    setStep("browser");
    setBrowserId("");
    setSourceMode(null);
    setCurrentChromeEntryDataUpdatedAt(null);
  };

  const startWithMode = (mode: SniffSourceMode) => {
    if (!selectedBrowser || props.confirming) {
      return;
    }
    if (mode === "current_browser") {
      if (!currentChromeReady) {
        return;
      }
      setSourceMode(mode);
      void props.onConfirm({
        mode: "current_browser",
        browserId: "chrome",
        profileId: "",
      });
      return;
    }
    if (!managedProfile) {
      return;
    }
    setSourceMode(mode);
    void props.onConfirm({
      mode: "xiadown_profile",
      browserId: selectedBrowser.id,
      profileId: managedProfile.id,
    });
  };

  return (
    <section
      className="app-new-task-sniff-source-steps overflow-hidden"
      data-step={step}
    >
      <div className="p-4">
        {sources.isLoading ? (
          <div
            className="app-new-task-sniff-source-loading flex min-h-28 items-center justify-center gap-2"
            role="status"
          >
            <Loader2 className="app-motion-spin h-4 w-4" />
            {t("browserSource.loading")}
          </div>
        ) : sources.isError ? (
          <div
            className="app-dream-status-message app-new-task-sniff-source-error p-4"
            data-intent="danger"
            role="alert"
          >
            <p>{t("browserSource.loadFailed")}</p>
            <Button
              className="mt-3"
              disabled={sources.isFetching}
              onClick={() => void sources.refetch()}
              size="compact"
              type="button"
              variant="outline"
            >
              <RefreshCw
                className={cn(
                  "h-3.5 w-3.5",
                  sources.isFetching && "app-motion-spin",
                )}
              />
              {t("browserSource.refresh")}
            </Button>
          </div>
        ) : step === "browser" ? (
          browsers.length === 0 ? (
            <div className="app-new-task-sniff-source-empty px-3 py-8">
              {t("browserSource.noBrowsers")}
            </div>
          ) : (
            <div
              aria-label={t("browserSource.chooseBrowser")}
              className="flex flex-wrap justify-center gap-x-6 gap-y-3"
              role="group"
            >
              {browsers.map((browser) => (
                <Button
                  aria-label={browser.label}
                  className="app-new-task-sniff-browser-option group"
                  data-browser-id={browser.id}
                  key={browser.id}
                  onClick={() => selectBrowser(browser)}
                  tone="neutral"
                  type="button"
                  variant="outline"
                >
                  <BrowserBrandIcon
                    browserId={browser.id}
                    className="app-new-task-sniff-browser-option__icon"
                  />
                  <span className="app-new-task-sniff-browser-option__label">
                    {browser.label}
                  </span>
                </Button>
              ))}
            </div>
          )
        ) : (
          <div>
            <div
              aria-label={t("browserSource.chooseMode")}
              className={cn(
                "mx-auto grid w-full gap-3",
                selectedBrowser?.id === "chrome"
                  ? "grid-cols-2"
                  : "max-w-sm grid-cols-1",
              )}
              role="radiogroup"
            >
              {selectedBrowser?.id === "chrome" ? (
                <div
                  className="app-new-task-sniff-profile-choice app-new-task-sniff-profile-choice--composite"
                  data-choice="browser-default"
                  data-disabled={!currentChromeReady ? "true" : "false"}
                  data-selected={sourceMode === "current_browser" ? "true" : "false"}
                >
                  <Button
                    aria-checked={sourceMode === "current_browser"}
                    aria-busy={
                      props.confirming && sourceMode === "current_browser"
                    }
                    className="app-new-task-sniff-profile-choice__primary"
                    disabled={!currentChromeReady || props.confirming}
                    onClick={() => startWithMode("current_browser")}
                    role="radio"
                    tone="neutral"
                    type="button"
                    variant="ghost"
                  >
                    {props.confirming && sourceMode === "current_browser" ? (
                      <Loader2 className="app-motion-spin h-8 w-8 shrink-0" />
                    ) : (
                      <BrowserBrandIcon
                        browserId="chrome"
                        className="h-8 w-8 shrink-0"
                      />
                    )}
                    <span className="min-w-0 flex-1">
                      <span className="app-new-task-sniff-profile-choice__title">
                        {t("browserSource.browserDefault")}
                      </span>
                      <StatusBadge
                        className="app-new-task-sniff-profile-choice__status"
                        tone={currentChromeReady ? "success" : "warning"}
                      >
                        {currentChromeStatusLabel}
                      </StatusBadge>
                    </span>
                  </Button>
                  <Button
                    aria-label={t("browserSource.refresh")}
                    className="my-auto mr-1.5 h-7 w-7 shrink-0"
                    disabled={currentChromeStatus.isFetching || props.confirming}
                    onClick={() => void currentChromeStatus.refetch()}
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
              ) : null}

              <Button
                aria-checked={sourceMode === "xiadown_profile"}
                aria-busy={
                  props.confirming && sourceMode === "xiadown_profile"
                }
                className="app-new-task-sniff-profile-choice"
                data-choice="xiadown-managed"
                disabled={props.confirming}
                onClick={() => startWithMode("xiadown_profile")}
                role="radio"
                tone={sourceMode === "xiadown_profile" ? "accent" : "neutral"}
                type="button"
                variant="outline"
              >
                {props.confirming && sourceMode === "xiadown_profile" ? (
                  <Loader2 className="app-motion-spin h-8 w-8 shrink-0" />
                ) : (
                  <img
                    alt=""
                    aria-hidden="true"
                    className="app-new-task-sniff-profile-choice__app-icon"
                    src="/appicon.png"
                  />
                )}
                <span className="min-w-0 flex-1">
                  <span className="app-new-task-sniff-profile-choice__title">
                    {t("browserSource.xiadownManaged")}
                  </span>
                  <span className="app-new-task-sniff-profile-choice__description">
                    {managedProfileLabel}
                  </span>
                </span>
              </Button>
            </div>
          </div>
        )}
      </div>

      {step === "profile" ? (
        <div className="flex items-center px-4 pb-4 pt-0">
          <Button
            disabled={props.confirming}
            onClick={goBack}
            size="compact"
            type="button"
            variant="ghost"
          >
            <ChevronLeft className="h-4 w-4" />
            {t("browserSource.back")}
          </Button>
        </div>
      ) : null}
    </section>
  );
}
