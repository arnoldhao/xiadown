import * as React from "react";

import { FileRelinkDialog } from "@/app/main/file-relink/FileRelinkDialog";
import { resolveUnknownErrorMessage } from "@/app/main/helpers";
import { getXiaText } from "@/features/xiadown/shared";
import { messageBus } from "@/shared/message";
import {
  selectLibraryDirectory,
  useApplyListenLocalRelinks,
  useListMissingListenLocalFiles,
  useScanMissingListenLocalFiles,
} from "@/shared/query/library";

function formatCountMessage(template: string, count: number) {
  return template.replace("{count}", String(count));
}

export function ListenLocalRelinkRepair(props: {
  open: boolean;
  text: ReturnType<typeof getXiaText>;
  clearingMissing: boolean;
  onOpenChange: (open: boolean) => void;
  onClearMissing: () => Promise<number>;
  onRefreshLocalTracks: () => Promise<void>;
}) {
  const missingFiles = useListMissingListenLocalFiles();
  const scanMissing = useScanMissingListenLocalFiles();
  const applyRelinks = useApplyListenLocalRelinks();
  const [selectedMatches, setSelectedMatches] = React.useState<Record<string, string>>({});
  const [clearConfirming, setClearConfirming] = React.useState(false);
  const openLoadStartedRef = React.useRef(false);

  const loadMissingFiles = React.useCallback(async () => {
    const result = await missingFiles.mutateAsync();
    setSelectedMatches({});
    setClearConfirming(false);
    scanMissing.reset();
    return result;
  }, [missingFiles, scanMissing]);

  React.useEffect(() => {
    if (!props.open) {
      openLoadStartedRef.current = false;
      setClearConfirming(false);
      return;
    }
    if (openLoadStartedRef.current) {
      return;
    }
    openLoadStartedRef.current = true;
    void loadMissingFiles().catch((error) => {
      messageBus.publishToast({
        intent: "danger",
        description: `${props.text.completed.fileMaintenanceFailed}: ${resolveUnknownErrorMessage(error, props.text.common.unknown)}`,
      });
    });
  }, [loadMissingFiles, props.open, props.text.common.unknown, props.text.completed.fileMaintenanceFailed]);

  const handleScanFolder = async () => {
    const currentMissing = missingFiles.data?.missing ?? [];
    const initialPath = currentMissing[0]?.oldPath ?? "";
    try {
      const selected = await selectLibraryDirectory(props.text.completed.relinkChooseFolderTitle, initialPath);
      if (!selected) {
        return;
      }
      setClearConfirming(false);
      const result = await scanMissing.mutateAsync({
        directory: selected,
        fileIds: currentMissing.map((file) => file.fileId),
      });
      const nextSelections: Record<string, string> = {};
      for (const match of result.matches) {
        if (!nextSelections[match.fileId]) {
          nextSelections[match.fileId] = match.newPath;
        }
      }
      setSelectedMatches(nextSelections);
      messageBus.publishToast({
        intent: result.matches.length > 0 ? "success" : "info",
        description:
          result.matches.length > 0
            ? formatCountMessage(props.text.completed.relinkScanFoundToast, result.matches.length)
            : props.text.completed.relinkScanNoneToast,
      });
    } catch (error) {
      messageBus.publishToast({
        intent: "danger",
        description: `${props.text.completed.relinkScanFailed}: ${resolveUnknownErrorMessage(error, props.text.common.unknown)}`,
      });
    }
  };

  const handleApplySelectedMatches = async () => {
    const matches = Object.entries(selectedMatches)
      .filter(([, path]) => path.trim())
      .map(([fileId, path]) => ({ fileId, path }));
    if (matches.length === 0) {
      return;
    }
    setClearConfirming(false);
    try {
      const result = await applyRelinks.mutateAsync({ matches });
      messageBus.publishToast({
        intent: "success",
        description: formatCountMessage(props.text.completed.relinkApplySuccessToast, result.relinked),
      });
      await props.onRefreshLocalTracks();
      await loadMissingFiles();
    } catch (error) {
      messageBus.publishToast({
        intent: "danger",
        description: `${props.text.completed.relinkApplyFailed}: ${resolveUnknownErrorMessage(error, props.text.common.unknown)}`,
      });
    }
  };

  const handleClearMissing = async () => {
    if (!clearConfirming) {
      setClearConfirming(true);
      return;
    }
    try {
      const removed = await props.onClearMissing();
      messageBus.publishToast({
        intent: removed > 0 ? "success" : "info",
        description:
          removed > 0
            ? formatCountMessage(props.text.listen.localClearMissingRemovedToast, removed)
            : props.text.listen.localClearMissingNoneToast,
      });
      await loadMissingFiles();
    } catch (error) {
      messageBus.publishToast({
        intent: "danger",
        description: `${props.text.listen.localClearMissingFailed}: ${resolveUnknownErrorMessage(error, props.text.common.unknown)}`,
      });
    } finally {
      setClearConfirming(false);
    }
  };

  return (
    <FileRelinkDialog
      open={props.open}
      text={props.text}
      loading={missingFiles.isPending}
      relinking={applyRelinks.isPending}
      scanning={scanMissing.isPending}
      clearing={props.clearingMissing}
      clearConfirming={clearConfirming}
      clearLabel={props.text.listen.localClearMissing}
      confirmClearLabel={props.text.listen.localConfirmClearMissing}
      missing={missingFiles.data?.missing ?? []}
      matches={scanMissing.data?.matches ?? []}
      selectedMatches={selectedMatches}
      onOpenChange={props.onOpenChange}
      onScanFolder={handleScanFolder}
      onApplyMatches={handleApplySelectedMatches}
      onClearMissing={handleClearMissing}
    />
  );
}
