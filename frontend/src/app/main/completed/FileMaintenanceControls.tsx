import { useQueryClient } from "@tanstack/react-query";
import { Loader2, RefreshCw, Trash2 } from "lucide-react";
import * as React from "react";

import { getXiaText } from "@/features/xiadown/shared";
import { cn } from "@/lib/utils";
import { messageBus } from "@/shared/message";
import { invalidateLibraryQueries } from "@/shared/query/library";
import { Button } from "@/shared/ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/shared/ui/tooltip";

import { resolveUnknownErrorMessage } from "@/app/main/helpers";

type VerifyFilesResponse = {
  checked?: number;
  missing?: number;
};

type ClearMissingFilesResponse = {
  checked?: number;
  removed?: number;
};

function formatCountMessage(template: string, count: number) {
  return template.replace("{count}", String(count));
}

export function CompletedFileMaintenanceControls(props: {
  text: ReturnType<typeof getXiaText>;
  httpBaseURL: string;
}) {
  const [action, setAction] = React.useState<"" | "verify" | "clear">("");
  const queryClient = useQueryClient();

  const runMaintenance = async (nextAction: "verify" | "clear") => {
    const baseURL = props.httpBaseURL.trim().replace(/\/+$/, "");
    if (!baseURL || action) {
      return;
    }
    setAction(nextAction);
    try {
      const endpoint =
        nextAction === "verify"
          ? "/api/library/files/verify"
          : "/api/library/files/clear-missing";
      const response = await fetch(`${baseURL}${endpoint}`, {
        method: "POST",
        headers: { Accept: "application/json" },
      });
      if (!response.ok) {
        throw new Error(`library file maintenance failed: ${response.status}`);
      }
      const result = (await response.json()) as VerifyFilesResponse & ClearMissingFilesResponse;
      if (nextAction === "verify") {
        const missing = Number.isFinite(result.missing) ? Number(result.missing) : 0;
        messageBus.publishToast({
          intent: missing > 0 ? "warning" : "success",
          description:
            missing > 0
              ? formatCountMessage(props.text.completed.verifyFilesMissingToast, missing)
              : props.text.completed.verifyFilesValidToast,
        });
        return;
      }
      const removed = Number.isFinite(result.removed) ? Number(result.removed) : 0;
      messageBus.publishToast({
        intent: removed > 0 ? "success" : "info",
        description:
          removed > 0
            ? formatCountMessage(props.text.completed.clearMissingFilesRemovedToast, removed)
            : props.text.completed.clearMissingFilesNoneToast,
      });
      invalidateLibraryQueries(queryClient);
    } catch (error) {
      messageBus.publishToast({
        intent: "danger",
        description: `${props.text.completed.fileMaintenanceFailed}: ${resolveUnknownErrorMessage(error, props.text.common.unknown)}`,
      });
    } finally {
      setAction("");
    }
  };

  return (
    <>
      <Tooltip>
        <TooltipTrigger asChild>
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="app-completed-toolbar-button h-8 w-8 p-0"
            aria-label={props.text.completed.verifyFiles}
            disabled={action !== ""}
            onClick={() => void runMaintenance("verify")}
          >
            <RefreshCw
              className={cn("h-4 w-4", action === "verify" ? "animate-spin" : "")}
            />
          </Button>
        </TooltipTrigger>
        <TooltipContent side="bottom">
          {props.text.completed.verifyFiles}
        </TooltipContent>
      </Tooltip>
      <Tooltip>
        <TooltipTrigger asChild>
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="app-completed-toolbar-button h-8 w-8 p-0"
            aria-label={props.text.completed.clearMissingFiles}
            disabled={action !== ""}
            onClick={() => void runMaintenance("clear")}
          >
            {action === "clear" ? (
              <Loader2 className="h-4 w-4 animate-spin" />
            ) : (
              <Trash2 className="h-4 w-4" />
            )}
          </Button>
        </TooltipTrigger>
        <TooltipContent side="bottom">
          {props.text.completed.clearMissingFiles}
        </TooltipContent>
      </Tooltip>
    </>
  );
}
