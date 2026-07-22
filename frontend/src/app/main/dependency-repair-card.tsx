import {
Loader2,
Wrench
} from "lucide-react";

import {
getXiaText
} from "@/features/xiadown/shared";
import { cn } from "@/lib/utils";
import type {
Dependency,
DependencyUpdateInfo,
} from "@/shared/contracts/dependencies";
import { Button } from "@/shared/ui/button";
import {
  StatusBadge,
  type DreamStatusTone,
} from "@/shared/ui/status-badge";

export function dependencyStatusLabel(
  text: ReturnType<typeof getXiaText>,
  status?: string,
) {
  switch ((status ?? "").trim().toLowerCase()) {
    case "installed":
      return text.dependencies.installed;
    case "invalid":
      return text.dependencies.invalid;
    default:
      return text.dependencies.missing;
  }
}

export function formatDependencyDisplayName(value: string) {
  return value
    .trim()
    .toUpperCase()
    .replace(/[!-~]/g, (char) =>
      String.fromCharCode(char.charCodeAt(0) + 0xfee0),
    );
}

export function formatDependencyInstallStage(
  text: ReturnType<typeof getXiaText>,
  stage?: string,
) {
  switch ((stage ?? "").trim().toLowerCase()) {
    case "downloading":
      return text.dependencies.downloading;
    case "extracting":
      return text.dependencies.extracting;
    case "verifying":
      return text.dependencies.verifying;
    default:
      return text.dependencies.installing;
  }
}

export function isDependencyInstallActive(stage?: string) {
  switch ((stage ?? "").trim().toLowerCase()) {
    case "downloading":
    case "extracting":
    case "verifying":
    case "installing":
      return true;
    default:
      return false;
  }
}

export function clampProgress(value: number | undefined) {
  if (typeof value !== "number" || Number.isNaN(value)) {
    return 0;
  }
  return Math.min(Math.max(value, 0), 100);
}

export function resolveDependencyTone(status?: string): DreamStatusTone {
  switch ((status ?? "").trim().toLowerCase()) {
    case "installed":
      return "success";
    case "invalid":
      return "warning";
    default:
      return "danger";
  }
}

export type DependencyRepairCardProps = {
  text: ReturnType<typeof getXiaText>;
  dependencyNames: readonly string[];
  toolsByName: Map<string, Dependency>;
  updatesByName: Map<string, DependencyUpdateInfo>;
  installStagesByName: Map<string, string>;
  installProgressByName: Map<string, number>;
  installPending: boolean;
  onInstallAll: () => Promise<void>;
  title: string;
  description: string;
};

export function DependencyRepairCard(props: DependencyRepairCardProps) {
  const dependencyCards = props.dependencyNames.map((name) => ({
    name,
    tool: props.toolsByName.get(name),
    update: props.updatesByName.get(name),
    installStage: props.installStagesByName.get(name) ?? "idle",
    installProgress: props.installProgressByName.get(name) ?? 0,
  }));
  const missingCount = dependencyCards.filter(
    (item) =>
      (item.tool?.status ?? "missing").trim().toLowerCase() !== "installed",
  ).length;
  const hasMissingDependencies = missingCount > 0;
  const installActive =
    props.installPending ||
    dependencyCards.some((item) =>
      isDependencyInstallActive(item.installStage),
    );

  return (
    <div
      className={cn(
        "app-dependency-repair-card",
        hasMissingDependencies && "app-dependency-repair-card-needed",
      )}
    >
      <div className="space-y-4 p-5 sm:p-6">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div className="flex min-w-0 items-start gap-3">
            <div className="app-dependency-repair-icon flex h-10 w-10 shrink-0 items-center justify-center">
              <Wrench className="h-3.5 w-3.5" />
            </div>
            <div className="min-w-0 space-y-1.5">
              <div className="app-dependency-repair-title">
                {props.title}
              </div>
              <div className="app-dependency-repair-description max-w-2xl">
                {props.description}
              </div>
            </div>
          </div>
          <StatusBadge
            className="w-fit shrink-0"
            marker
            tone={hasMissingDependencies ? "warning" : "success"}
          >
            {hasMissingDependencies
              ? props.text.dependencies.missing
              : props.text.dependencies.installed}
          </StatusBadge>
        </div>

        <div className="space-y-3">
          {dependencyCards.map((item) => {
            const status = (item.tool?.status ?? "missing")
              .trim()
              .toLowerCase();
            const installStage = item.installStage.trim().toLowerCase();
            const isInstalling = isDependencyInstallActive(installStage);
            const installProgress = clampProgress(item.installProgress);

            return (
              <div
                key={item.name}
                className="app-dependency-item px-4 py-3"
                data-installing={isInstalling ? "true" : undefined}
              >
                <div className="flex min-w-0 items-center justify-between gap-3">
                  <div className="min-w-0">
                    <div className="app-dependency-item-title truncate">
                      {formatDependencyDisplayName(item.name)}
                    </div>
                    <div className="app-dependency-item-meta mt-1 flex flex-wrap items-center gap-2">
                      <span>
                        {props.text.dependencies.currentVersion}:{" "}
                        {item.tool?.version || "-"}
                      </span>
                      <span className="app-dependency-item-separator">/</span>
                      <span>
                        {props.text.dependencies.latestVersion}:{" "}
                        {item.update?.latestVersion ||
                          props.text.common.unknown}
                      </span>
                    </div>
                  </div>
                  <StatusBadge
                    className="shrink-0"
                    marker
                    tone={resolveDependencyTone(status)}
                  >
                    {dependencyStatusLabel(props.text, status)}
                  </StatusBadge>
                </div>

                {isInstalling ? (
                  <div className="mt-3 space-y-1.5">
                    <div className="app-dream-progress-track h-2 w-full">
                      <div
                        className="app-dream-progress-value"
                        style={{ width: `${installProgress}%` }}
                      />
                    </div>
                    <div className="app-dependency-item-progress-meta flex items-center justify-between gap-3">
                      <span className="truncate">
                        {formatDependencyInstallStage(props.text, installStage)}
                      </span>
                      <span className="app-dependency-progress-value shrink-0">
                        {Math.round(installProgress)}%
                      </span>
                    </div>
                  </div>
                ) : null}
              </div>
            );
          })}
        </div>

        {missingCount > 0 ? (
          <div className="flex items-center justify-end">
            <Button
              type="button"
              onClick={() => void props.onInstallAll()}
              disabled={installActive}
            >
              {installActive ? (
                <Loader2 className="h-4 w-4 app-motion-spin" />
              ) : (
                <Wrench className="h-4 w-4" />
              )}
              {props.text.actions.installAll}
            </Button>
          </div>
        ) : null}
      </div>
    </div>
  );
}
