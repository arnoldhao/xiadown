import { System } from "@wailsio/runtime";
import { CheckCircle2, CheckSquare, ChevronRight, CircleSlash, ClipboardList, Clock3, Eye, Files, LayoutGrid, Loader2, PencilLine, Search, SlidersHorizontal, Trash2, X, XCircle } from "lucide-react";
import * as React from "react";

import { WindowControls } from "@/components/layout/WindowControls";
import { getXiaText } from "@/features/xiadown/shared";
import { cn } from "@/lib/utils";
import type { LibraryDTO, OperationListItemDTO } from "@/shared/contracts/library";
import type { Pet } from "@/shared/contracts/pets";
import { useDeleteFiles, useDeleteOperations, useRenameFile, useRenameOperation } from "@/shared/query/library";
import { Button } from "@/shared/ui/button";
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/shared/ui/dialog";
import { DropdownMenu, DropdownMenuCheckboxItem, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel, DropdownMenuSeparator, DropdownMenuTrigger } from "@/shared/ui/dropdown-menu";
import { Input } from "@/shared/ui/input";
import { Select } from "@/shared/ui/select";
import { Tooltip,TooltipContent,TooltipTrigger } from "@/shared/ui/tooltip";
import { PetDisplay } from "@/shared/ui/pet-player";
import { formatBytes } from "@/shared/utils/formatBytes";
import { buildAssetPreviewURL, extractExtensionFromPath, getPathBaseName, stripPathExtension } from "@/shared/utils/resourceHelpers";

import { CompletedFileDetailContent,CompletedFileDetailHeader,CompletedTaskDetailContent,CompletedTaskDetailHeader,SelectionCheckbox } from "@/app/main/completed/detail-components";
import { CompletedFileMaintenanceControls } from "@/app/main/completed/FileMaintenanceControls";
import { CompletedListViewSwitch } from "@/app/main/completed/ListTabButton";
import { COMPLETED_FILE_TYPE_ORDER,buildCompletedCoverLookup,canPreviewCompletedFile,firstCompletedText,formatCompletedTranscodedFromLabel,formatRelativeTime,isCompletedLibraryFileUnavailable,resolveCompletedDeleteDialogMessage,resolveCompletedDeleteDialogTitle,resolveCompletedFileCoverURL,resolveCompletedFileIcon,resolveCompletedFileType,resolveCompletedFileTypeLabel,resolveCompletedImagePreviewURL,resolveCompletedLibraryFileExplicitCoverURL,resolveCompletedOperationExplicitCoverURL,resolveCompletedPageLabel,resolveCompletedPerPageLabel,resolveCompletedPreviewGroupIcon,resolveCompletedPreviewGroupLabel,resolveCompletedPreviewKind,resolveCompletedSelectionSummary,resolveCompletedStatusLabel,resolveCompletedTaskCoverURL,resolveCompletedTaskFileTypeSummaries,resolveCompletedTotalLabel,resolveOperationUpdatedAt,resolveUnknownErrorMessage } from "@/app/main/helpers";
import { COMPLETED_FILE_PAGE_SIZE_OPTIONS,COMPLETED_TASK_PAGE_SIZE_OPTIONS,SIDEBAR_DROPDOWN_CONTENT_CLASS_NAME,SIDEBAR_DROPDOWN_ICON_SLOT_CLASS_NAME,SIDEBAR_DROPDOWN_ITEM_CLASS_NAME } from "@/app/main/main-constants";
import type { CompletedContextMenuTarget,CompletedDeleteConfirmation,CompletedFileEntry,CompletedFileType,CompletedTaskEntry,CompletedViewMode } from "@/app/main/types";

const COMPLETED_FILTER_MENU_CONTENT_CLASS = "app-completed-filter-menu w-64";
const COMPLETED_FILTER_MENU_ITEM_CLASS = "app-completed-filter-menu-item";
const COMPLETED_FILTER_MENU_CHECKBOX_CLASS =
  "app-completed-filter-menu-checkbox";
const COMPLETED_FILTER_MENU_ICON_CLASS =
  "app-completed-filter-menu-icon flex h-4 w-4 shrink-0 items-center justify-center";

type RenameDisplayNameTarget = {
  kind: "task" | "file";
  id: string;
  name: string;
};

const COMPLETED_RENAME_DISPLAY_NAME_MAX_LENGTH = 160;
const COMPLETED_RENAME_INVALID_NAME_PATTERN = /[<>:"/\\|?*\u0000-\u001f\u007f]/;
const COMPLETED_RENAME_RESERVED_NAMES = new Set([
  "CON",
  "PRN",
  "AUX",
  "NUL",
  "COM1",
  "COM2",
  "COM3",
  "COM4",
  "COM5",
  "COM6",
  "COM7",
  "COM8",
  "COM9",
  "LPT1",
  "LPT2",
  "LPT3",
  "LPT4",
  "LPT5",
  "LPT6",
  "LPT7",
  "LPT8",
  "LPT9",
]);

function validateCompletedRenameDisplayName(
  name: string,
  text: ReturnType<typeof getXiaText>,
) {
  const trimmed = name.trim();
  if (!trimmed) {
    return text.completed.renameNameRequired;
  }
  if ([...trimmed].length > COMPLETED_RENAME_DISPLAY_NAME_MAX_LENGTH) {
    return text.completed.renameNameTooLong;
  }
  if (
    COMPLETED_RENAME_INVALID_NAME_PATTERN.test(trimmed) ||
    trimmed.replace(/\./g, "") === "" ||
    trimmed.endsWith(".")
  ) {
    return text.completed.renameNameInvalid;
  }
  const reservedCandidate = trimmed.split(".")[0]?.toUpperCase() ?? "";
  if (COMPLETED_RENAME_RESERVED_NAMES.has(reservedCandidate)) {
    return text.completed.renameNameInvalid;
  }
  return "";
}

function resolveCompletedLibrarySourceFileLabel(
  file: LibraryDTO["files"][number],
) {
  const localPath = file.storage.localPath?.trim() ?? "";
  return (
    file.displayName?.trim() ||
    file.displayLabel?.trim() ||
    file.fileName?.trim() ||
    file.name?.trim() ||
    getPathBaseName(localPath) ||
    file.id
  );
}

function resolveCompletedTranscodeSourceFile(
  file: LibraryDTO["files"][number],
  filesById: Map<string, LibraryDTO["files"][number]>,
) {
  const kind = (file.kind ?? "").trim().toLowerCase();
  const originKind = (file.origin.kind ?? "").trim().toLowerCase();
  if (kind !== "transcode" && originKind !== "transcode") {
    return null;
  }
  const rootFileId = file.lineage.rootFileId?.trim() ?? "";
  if (!rootFileId || rootFileId === file.id) {
    return null;
  }
  return filesById.get(rootFileId) ?? null;
}

function resolveCompletedTaskTranscodeSource(
  operation: OperationListItemDTO,
  library: LibraryDTO | null,
  files: CompletedFileEntry[],
) {
  if ((operation.kind ?? "").trim().toLowerCase() !== "transcode") {
    return { id: "", name: "" };
  }
  const libraryFilesById = new Map(
    (library?.files ?? []).map((file) => [file.id, file]),
  );
  const sourceIds = [
    operation.request?.fileId,
    operation.request?.rootFileId,
  ]
    .map((value) => value?.trim() ?? "")
    .filter(Boolean);
  for (const sourceId of sourceIds) {
    const sourceFile = libraryFilesById.get(sourceId);
    if (sourceFile) {
      return {
        id: sourceFile.id,
        name: resolveCompletedLibrarySourceFileLabel(sourceFile),
      };
    }
  }
  const outputSource = files.find((file) => file.sourceFileName.trim());
  if (outputSource) {
    return {
      id: outputSource.sourceFileId,
      name: outputSource.sourceFileName,
    };
  }
  const inputPath = operation.request?.inputPath?.trim() ?? "";
  if (inputPath) {
    return { id: "", name: getPathBaseName(inputPath) || inputPath };
  }
  return { id: "", name: "" };
}

function isCompletedFloatingLayerTarget(target: EventTarget | null) {
  if (!(target instanceof Node)) {
    return false;
  }
  const element =
    target instanceof Element ? target : target.parentElement;
  return Boolean(
    element?.closest(
      ".app-dialog-content, .app-dialog-overlay, .app-dialog-close, [role='dialog']",
    ),
  );
}

function isCompletedListItemTarget(target: EventTarget | null) {
  if (!(target instanceof Node)) {
    return false;
  }
  const element =
    target instanceof Element ? target : target.parentElement;
  return Boolean(
    element?.closest(".app-completed-task-card, .app-completed-file-card"),
  );
}

function isCompletedDetailOrFloatingLayerTarget(
  target: EventTarget | null,
  detailPane: HTMLElement | null,
) {
  if (!(target instanceof Node)) {
    return false;
  }
  return Boolean(
    detailPane?.contains(target) || isCompletedFloatingLayerTarget(target),
  );
}

function resolveCompletedTaskStatusIcon(status?: string) {
  switch ((status ?? "").trim().toLowerCase()) {
    case "succeeded":
      return CheckCircle2;
    case "failed":
      return XCircle;
    case "canceled":
      return CircleSlash;
    default:
      return Clock3;
  }
}

function resolveCompletedTaskStatusIconTone(status?: string) {
  switch ((status ?? "").trim().toLowerCase()) {
    case "succeeded":
      return "app-completed-status-icon-success";
    case "failed":
      return "app-completed-status-icon-danger";
    case "canceled":
      return "app-completed-status-icon-warning";
    default:
      return "app-completed-status-icon-muted";
  }
}

function buildCompletedTaskFileSummaryItems(
  entry: CompletedTaskEntry,
  text: ReturnType<typeof getXiaText>,
) {
  return entry.fileTypeSummaries.map((item) => ({
    count: item.count,
    icon: resolveCompletedPreviewGroupIcon(item.type),
    key: item.type,
    label: resolveCompletedPreviewGroupLabel(item.type, text),
  }));
}

export function CompletedPage(props: {
  text: ReturnType<typeof getXiaText>;
  libraries: LibraryDTO[];
  terminalOperations: OperationListItemDTO[];
  httpBaseURL: string;
  pet: Pet | null;
  petImageURL: string;
  onTranscodeFile?: (file: CompletedFileEntry) => void;
}) {
  const isWindows = System.IsWindows();
  const deleteOperations = useDeleteOperations();
  const deleteFiles = useDeleteFiles();
  const renameOperation = useRenameOperation();
  const renameFile = useRenameFile();
  const [viewMode, setViewMode] = React.useState<CompletedViewMode>("tasks");
  const [query, setQuery] = React.useState("");
  const [searchFocused, setSearchFocused] = React.useState(false);
  const [selectedTaskId, setSelectedTaskId] = React.useState("");
  const [selectedFileId, setSelectedFileId] = React.useState("");
  const [selectedPreviewFileId, setSelectedPreviewFileId] = React.useState("");
  const [taskSelectionMode, setTaskSelectionMode] = React.useState(false);
  const [fileSelectionMode, setFileSelectionMode] = React.useState(false);
  const [contextMenuTarget, setContextMenuTarget] =
    React.useState<CompletedContextMenuTarget | null>(null);
  const [deleteConfirmTarget, setDeleteConfirmTarget] =
    React.useState<CompletedDeleteConfirmation | null>(null);
  const [deleteConfirmError, setDeleteConfirmError] = React.useState("");
  const [renameTarget, setRenameTarget] =
    React.useState<RenameDisplayNameTarget | null>(null);
  const [renameDraft, setRenameDraft] = React.useState("");
  const [renameError, setRenameError] = React.useState("");
  const [selectedTaskIds, setSelectedTaskIds] = React.useState<string[]>([]);
  const [selectedFileIds, setSelectedFileIds] = React.useState<string[]>([]);
  const [previewPresentationModeActive, setPreviewPresentationModeActive] =
    React.useState(false);
  const [taskStatusFilters, setTaskStatusFilters] = React.useState<string[]>(
    [],
  );
  const [fileTypeFilters, setFileTypeFilters] = React.useState<
    CompletedFileType[]
  >([]);
  const [taskPage, setTaskPage] = React.useState(1);
  const [filePage, setFilePage] = React.useState(1);
  const [taskPageSize, setTaskPageSize] = React.useState<number>(
    COMPLETED_TASK_PAGE_SIZE_OPTIONS[0],
  );
  const [filePageSize, setFilePageSize] = React.useState<number>(
    COMPLETED_FILE_PAGE_SIZE_OPTIONS[0],
  );
  const searchInputRef = React.useRef<HTMLInputElement | null>(null);
  const renameInputRef = React.useRef<HTMLInputElement | null>(null);
  const detailPaneRef = React.useRef<HTMLElement | null>(null);
  const searchHasText = query.length > 0;
  const trimmedQuery = query.trim().toLowerCase();

  const librariesById = React.useMemo(
    () => new Map(props.libraries.map((library) => [library.id, library])),
    [props.libraries],
  );

  const operationUpdatedAtById = React.useMemo(() => {
    const map = new Map<string, string>();
    props.terminalOperations.forEach((operation) => {
      map.set(operation.operationId, resolveOperationUpdatedAt(operation));
    });
    return map;
  }, [props.terminalOperations]);

  const operationNameById = React.useMemo(() => {
    const map = new Map<string, string>();
    props.terminalOperations.forEach((operation) => {
      map.set(operation.operationId, operation.name.trim());
    });
    return map;
  }, [props.terminalOperations]);

  const realFiles = React.useMemo<CompletedFileEntry[]>(
    () =>
      props.libraries.flatMap((library) => {
        const coverLookup = buildCompletedCoverLookup(
          props.httpBaseURL,
          library,
        );
        const libraryFilesById = new Map(
          library.files.map((file) => [file.id, file]),
        );
        return library.files
          .filter((file) => !file.state.deleted)
          .map((file) => {
            const localPath = file.storage.localPath?.trim() ?? "";
            const label = resolveCompletedLibrarySourceFileLabel(file);
            const title =
              firstCompletedText(
                stripPathExtension(label),
                file.metadata.title,
              ) || label;
            const latestOperationId = file.latestOperationId?.trim() ?? "";
            const originOperationId = file.origin.operationId?.trim() ?? "";
            const rootFileId = file.lineage.rootFileId?.trim() ?? "";
            const sourceFile = resolveCompletedTranscodeSourceFile(
              file,
              libraryFilesById,
            );
            const sourceFileName = sourceFile
              ? resolveCompletedLibrarySourceFileLabel(sourceFile)
              : "";
            const operationUpdatedAt =
              operationUpdatedAtById.get(latestOperationId) ||
              operationUpdatedAtById.get(originOperationId);
            const operationName =
              firstCompletedText(
                operationNameById.get(originOperationId),
                operationNameById.get(latestOperationId),
              ) || "";
            const explicitCoverURL = resolveCompletedLibraryFileExplicitCoverURL(
              props.httpBaseURL,
              library,
              file,
              coverLookup,
            );
            const fileUnavailable = isCompletedLibraryFileUnavailable(file);
            const entry: CompletedFileEntry = {
              id: file.id,
              libraryId: library.id,
              libraryName: library.name || library.id,
              operationName,
              operationId: originOperationId || latestOperationId,
              latestOperationId,
              originOperationId,
              rootFileId,
              sourceFileId: sourceFile?.id ?? "",
              sourceFileName,
              name: label,
              title,
              author: firstCompletedText(file.metadata.author),
              path: localPath,
              kind: file.kind,
              format: (
                file.media?.format ||
                extractExtensionFromPath(localPath) ||
                file.kind ||
                "file"
              )
                .toString()
                .toUpperCase(),
              sizeBytes: file.media?.sizeBytes ?? 0,
              updatedAt:
                operationUpdatedAt ||
                file.origin.import?.importedAt ||
                file.createdAt ||
                file.updatedAt ||
                library.updatedAt ||
                "",
              previewURL: !fileUnavailable && localPath
                ? buildAssetPreviewURL(props.httpBaseURL, localPath)
                : "",
              coverURL: "",
              canDelete: true,
              media: file.media ?? null,
            };
            entry.coverURL = resolveCompletedFileCoverURL(
              entry,
              explicitCoverURL,
            );
            return entry;
          });
      }),
    [operationNameById, operationUpdatedAtById, props.httpBaseURL, props.libraries],
  );

  const allFiles = React.useMemo<CompletedFileEntry[]>(() => {
    const map = new Map<string, CompletedFileEntry>();
    realFiles.forEach((file) => {
      map.set(file.id, file);
    });
    props.terminalOperations.forEach((operation) => {
      const library = librariesById.get(operation.libraryId) ?? null;
      const operationCoverURL = resolveCompletedOperationExplicitCoverURL(
        props.httpBaseURL,
        operation,
        library,
      );
      (operation.outputFiles ?? []).forEach((output, index) => {
        if (output.deleted || map.has(output.fileId)) {
          return;
        }
        const label = `${operation.name} ${index + 1}`;
        const format = (output.format || output.kind || "file")
          .toString()
          .toUpperCase();
        const entry: CompletedFileEntry = {
          id: output.fileId,
          libraryId: operation.libraryId,
          libraryName:
            operation.libraryName || library?.name || operation.libraryId,
          operationName: operation.name,
          operationId: operation.operationId,
          latestOperationId: operation.operationId,
          originOperationId: operation.operationId,
          rootFileId: "",
          sourceFileId: "",
          sourceFileName: "",
          name: label,
          title: firstCompletedText(
            stripPathExtension(label),
            operation.name,
          ),
          author: firstCompletedText(operation.uploader, operation.libraryName),
          path: "",
          kind: output.kind,
          format,
          sizeBytes: output.sizeBytes ?? 0,
          updatedAt: resolveOperationUpdatedAt(operation),
          previewURL: "",
          coverURL: "",
          canDelete: false,
          media: null,
        };
        entry.coverURL = resolveCompletedFileCoverURL(
          entry,
          operationCoverURL,
        );
        map.set(output.fileId, entry);
      });
    });

    return [...map.values()].sort((left, right) => {
      const parsedLeft = Date.parse(left.updatedAt || "");
      const parsedRight = Date.parse(right.updatedAt || "");
      const leftTime = Number.isFinite(parsedLeft) ? parsedLeft : 0;
      const rightTime = Number.isFinite(parsedRight) ? parsedRight : 0;
      return rightTime - leftTime;
    });
  }, [librariesById, props.terminalOperations, realFiles]);

  const filesById = React.useMemo(
    () => new Map(allFiles.map((file) => [file.id, file])),
    [allFiles],
  );

  const realFilesByOperationId = React.useMemo(() => {
    const map = new Map<string, CompletedFileEntry[]>();
    realFiles.forEach((file) => {
      if (!file.originOperationId) {
        return;
      }
      const current = map.get(file.originOperationId) ?? [];
      current.push(file);
      map.set(file.originOperationId, current);
    });
    return map;
  }, [realFiles]);

  const taskEntries = React.useMemo<CompletedTaskEntry[]>(
    () =>
      [...props.terminalOperations]
        .map((operation) => {
          const library = librariesById.get(operation.libraryId) ?? null;
          const operationCoverURL = resolveCompletedOperationExplicitCoverURL(
            props.httpBaseURL,
            operation,
            library,
          );
          const filesMap = new Map<string, CompletedFileEntry>();

          (realFilesByOperationId.get(operation.operationId) ?? []).forEach(
            (file) => {
              filesMap.set(file.id, file);
            },
          );

          (operation.outputFiles ?? []).forEach((output, index) => {
            if (output.deleted) {
              return;
            }
            const existing = filesById.get(output.fileId);
            if (existing) {
              filesMap.set(existing.id, existing);
              return;
            }
            const label = `${operation.name} ${index + 1}`;
            const entry: CompletedFileEntry = {
              id: output.fileId,
              libraryId: operation.libraryId,
              libraryName:
                operation.libraryName || library?.name || operation.libraryId,
              operationName: operation.name,
              operationId: operation.operationId,
              latestOperationId: operation.operationId,
              originOperationId: operation.operationId,
              rootFileId: "",
              sourceFileId: "",
              sourceFileName: "",
              name: label,
              title: firstCompletedText(
                stripPathExtension(label),
                operation.name,
              ),
              author: firstCompletedText(
                operation.uploader,
                operation.libraryName,
              ),
              path: "",
              kind: output.kind,
              format: (output.format || output.kind || "file")
                .toString()
                .toUpperCase(),
              sizeBytes: output.sizeBytes ?? 0,
              updatedAt: resolveOperationUpdatedAt(operation),
              previewURL: "",
              coverURL: "",
              canDelete: false,
              media: null,
            };
            entry.coverURL = resolveCompletedFileCoverURL(
              entry,
              operationCoverURL,
            );
            filesMap.set(output.fileId, entry);
          });

          const files = [...filesMap.values()];
          const transcodeSource = resolveCompletedTaskTranscodeSource(
            operation,
            library,
            files,
          );
          const fileTypeSummaries = resolveCompletedTaskFileTypeSummaries(files);

          return {
            operation,
            library,
            coverURL: resolveCompletedTaskCoverURL(files, operationCoverURL),
            files,
            sourceFileId: transcodeSource.id,
            sourceFileName: transcodeSource.name,
            fileTypeSummaries,
            updatedAt: resolveOperationUpdatedAt(operation),
          };
        })
        .sort((left, right) => {
          const parsedLeft = Date.parse(left.updatedAt || "");
          const parsedRight = Date.parse(right.updatedAt || "");
          const leftTime = Number.isFinite(parsedLeft) ? parsedLeft : 0;
          const rightTime = Number.isFinite(parsedRight) ? parsedRight : 0;
          return rightTime - leftTime;
        }),
    [
      filesById,
      librariesById,
      props.httpBaseURL,
      props.terminalOperations,
      realFilesByOperationId,
    ],
  );

  const taskStatusOptions = React.useMemo(() => {
    const order = ["succeeded", "failed", "canceled"];
    const seen = new Set<string>();
    const statuses = taskEntries
      .map((entry) => (entry.operation.status ?? "").trim().toLowerCase())
      .filter(Boolean)
      .filter((status) => {
        if (seen.has(status)) {
          return false;
        }
        seen.add(status);
        return true;
      });

    return statuses.sort((left, right) => {
      const leftIndex = order.indexOf(left);
      const rightIndex = order.indexOf(right);
      if (leftIndex === -1 && rightIndex === -1) {
        return left.localeCompare(right);
      }
      if (leftIndex === -1) {
        return 1;
      }
      if (rightIndex === -1) {
        return -1;
      }
      return leftIndex - rightIndex;
    });
  }, [taskEntries]);

  const fileTypeOptions = React.useMemo<CompletedFileType[]>(() => {
    const seen = new Set<string>();
    const types = allFiles
      .map((file) => resolveCompletedFileType(file))
      .filter((type) => {
        if (seen.has(type)) {
          return false;
        }
        seen.add(type);
        return true;
      });
    return types.sort(
      (left, right) =>
        COMPLETED_FILE_TYPE_ORDER.indexOf(left) -
        COMPLETED_FILE_TYPE_ORDER.indexOf(right),
    );
  }, [allFiles]);

  const filteredTasks = React.useMemo(
    () =>
      taskEntries.filter((entry) => {
        const status = (entry.operation.status ?? "").trim().toLowerCase();
        if (
          taskStatusFilters.length > 0 &&
          !taskStatusFilters.includes(status)
        ) {
          return false;
        }
        if (!trimmedQuery) {
          return true;
        }
        return [
          entry.operation.name,
          entry.operation.libraryName || entry.library?.name || "",
          entry.sourceFileName,
          entry.operation.domain || "",
          resolveCompletedStatusLabel(props.text, entry.operation.status),
        ]
          .join(" ")
          .toLowerCase()
          .includes(trimmedQuery);
      }),
    [props.text, taskEntries, taskStatusFilters, trimmedQuery],
  );

  const filteredFiles = React.useMemo(
    () =>
      allFiles.filter((file) => {
        const fileType = resolveCompletedFileType(file);
        if (fileTypeFilters.length > 0 && !fileTypeFilters.includes(fileType)) {
          return false;
        }
        if (!trimmedQuery) {
          return true;
        }
        return [file.name, file.operationName, file.sourceFileName, file.libraryName, file.kind, file.format, file.path]
          .join(" ")
          .toLowerCase()
          .includes(trimmedQuery);
      }),
    [allFiles, fileTypeFilters, trimmedQuery],
  );

  const taskPageCount = Math.max(
    1,
    Math.ceil(filteredTasks.length / taskPageSize),
  );
  const filePageCount = Math.max(
    1,
    Math.ceil(filteredFiles.length / filePageSize),
  );
  const pagedTasks = filteredTasks.slice(
    (taskPage - 1) * taskPageSize,
    taskPage * taskPageSize,
  );
  const pagedFiles = filteredFiles.slice(
    (filePage - 1) * filePageSize,
    filePage * filePageSize,
  );
  const currentPage = viewMode === "tasks" ? taskPage : filePage;
  const currentPageCount = viewMode === "tasks" ? taskPageCount : filePageCount;
  const currentPageSize = viewMode === "tasks" ? taskPageSize : filePageSize;
  const currentPageSizeOptions =
    viewMode === "tasks"
      ? COMPLETED_TASK_PAGE_SIZE_OPTIONS
      : COMPLETED_FILE_PAGE_SIZE_OPTIONS;
  const currentTotalCount =
    viewMode === "tasks" ? filteredTasks.length : filteredFiles.length;

  React.useEffect(() => {
    setTaskPage(1);
    setFilePage(1);
  }, [
    filePageSize,
    fileTypeFilters,
    query,
    taskPageSize,
    taskStatusFilters,
    viewMode,
  ]);

  React.useEffect(() => {
    if (!renameTarget) {
      return;
    }
    const frame = window.requestAnimationFrame(() => {
      renameInputRef.current?.focus();
      renameInputRef.current?.select();
    });
    return () => window.cancelAnimationFrame(frame);
  }, [renameTarget]);

  React.useEffect(() => {
    setTaskPage((current) => Math.min(current, taskPageCount));
  }, [taskPageCount]);

  React.useEffect(() => {
    setFilePage((current) => Math.min(current, filePageCount));
  }, [filePageCount]);

  React.useEffect(() => {
    if (viewMode !== "tasks") {
      return;
    }
    if (
      selectedTaskId &&
      !pagedTasks.some(
        (entry) => entry.operation.operationId === selectedTaskId,
      )
    ) {
      setSelectedTaskId("");
    }
  }, [pagedTasks, selectedTaskId, viewMode]);

  React.useEffect(() => {
    if (viewMode !== "files") {
      return;
    }
    if (selectedFileId && !pagedFiles.some((file) => file.id === selectedFileId)) {
      setSelectedFileId("");
    }
  }, [pagedFiles, selectedFileId, viewMode]);

  React.useEffect(() => {
    setSelectedTaskIds((current) =>
      current.filter((operationId) =>
        taskEntries.some(
          (entry) => entry.operation.operationId === operationId,
        ),
      ),
    );
  }, [taskEntries]);

  React.useEffect(() => {
    setSelectedFileIds((current) =>
      current.filter((fileId) => filesById.has(fileId)),
    );
  }, [filesById]);

  React.useEffect(() => {
    setTaskStatusFilters((current) =>
      current.filter((status) => taskStatusOptions.includes(status)),
    );
  }, [taskStatusOptions]);

  React.useEffect(() => {
    setFileTypeFilters((current) =>
      current.filter((type) => fileTypeOptions.includes(type)),
    );
  }, [fileTypeOptions]);

  const selectedTask =
    taskEntries.find(
      (entry) => entry.operation.operationId === selectedTaskId,
    ) ?? null;
  const selectedFile =
    allFiles.find((file) => file.id === selectedFileId) ?? null;
  const contextMenuTask =
    contextMenuTarget?.kind === "task"
      ? (taskEntries.find(
          (entry) => entry.operation.operationId === contextMenuTarget.id,
        ) ?? null)
      : null;
  const contextMenuFile =
    contextMenuTarget?.kind === "file"
      ? (filesById.get(contextMenuTarget.id) ?? null)
      : null;

  React.useEffect(() => {
    if (!selectedTask || selectedTask.files.length === 0) {
      setSelectedPreviewFileId("");
      return;
    }
    if (!selectedTask.files.some((file) => file.id === selectedPreviewFileId)) {
      setSelectedPreviewFileId(
        selectedTask.files.find((file) => canPreviewCompletedFile(file))?.id ??
          selectedTask.files[0].id,
      );
    }
  }, [selectedPreviewFileId, selectedTask]);

  const selectedTaskIdsSet = React.useMemo(
    () => new Set(selectedTaskIds),
    [selectedTaskIds],
  );
  const selectedBatchTasks = React.useMemo(
    () =>
      taskEntries.filter((entry) =>
        selectedTaskIdsSet.has(entry.operation.operationId),
      ),
    [selectedTaskIdsSet, taskEntries],
  );
  const selectedFileIdsSet = React.useMemo(
    () => new Set(selectedFileIds),
    [selectedFileIds],
  );
  const selectedBatchFiles = React.useMemo(
    () => allFiles.filter((file) => selectedFileIdsSet.has(file.id)),
    [allFiles, selectedFileIdsSet],
  );
  const currentSelectionTargetIds = React.useMemo(
    () =>
      viewMode === "tasks"
        ? filteredTasks.map((entry) => entry.operation.operationId)
        : filteredFiles.map((file) => file.id),
    [filteredFiles, filteredTasks, viewMode],
  );
  const selectionMode =
    viewMode === "tasks" ? taskSelectionMode : fileSelectionMode;
  const selectionCount =
    viewMode === "tasks" ? selectedTaskIds.length : selectedFileIds.length;
  const allCurrentSelectionTargetsSelected =
    currentSelectionTargetIds.length > 0 &&
    currentSelectionTargetIds.every((id) =>
      viewMode === "tasks"
        ? selectedTaskIdsSet.has(id)
        : selectedFileIdsSet.has(id),
    );
  const searchInputExpanded = !selectionMode && (searchFocused || searchHasText);
  const tabsCompact = searchHasText;
  const canDeleteSelectedTasks = selectedBatchTasks.length > 0;
  const canDeleteSelectedFiles =
    selectedBatchFiles.length > 0 &&
    selectedBatchFiles.every((file) => file.canDelete);
  const canDeleteCurrentSelection =
    viewMode === "tasks" ? canDeleteSelectedTasks : canDeleteSelectedFiles;
  const contentTitle =
    viewMode === "tasks"
      ? selectedTask?.operation.name || props.text.completed.taskDetail
      : selectedFile?.name || props.text.completed.fileDetail;
  const ContentHeaderIcon =
    viewMode === "tasks"
      ? ClipboardList
      : selectedFile
        ? resolveCompletedFileIcon(selectedFile)
        : Files;
  const searchPlaceholder =
    viewMode === "tasks"
      ? props.text.completed.searchTasks
      : props.text.completed.searchFiles;
  const activeFilterCount =
    viewMode === "tasks" ? taskStatusFilters.length : fileTypeFilters.length;
  const canDeleteContextMenuTarget =
    contextMenuTarget?.kind === "task"
      ? Boolean(contextMenuTask) && !deleteOperations.isPending
      : contextMenuTarget?.kind === "file"
        ? Boolean(contextMenuFile?.canDelete) && !deleteFiles.isPending
        : false;
  const renamePending = renameOperation.isPending || renameFile.isPending;
  const canRenameContextMenuTarget =
    contextMenuTarget?.kind === "task"
      ? Boolean(contextMenuTask) && !renamePending
      : contextMenuTarget?.kind === "file"
        ? Boolean(contextMenuFile?.canDelete) && !renamePending
        : false;
  const isDeleteConfirmPending =
    deleteConfirmTarget?.kind === "tasks"
      ? deleteOperations.isPending
      : deleteConfirmTarget?.kind === "files"
        ? deleteFiles.isPending
        : false;
  const trimmedRenameDraft = renameDraft.trim();
  const canSaveRename =
    Boolean(renameTarget) &&
    trimmedRenameDraft.length > 0 &&
    trimmedRenameDraft !== (renameTarget?.name.trim() ?? "") &&
    !renamePending;

  const toggleFileSelection = (fileId: string) => {
    setSelectedFileIds((current) =>
      current.includes(fileId)
        ? current.filter((item) => item !== fileId)
        : [...current, fileId],
    );
  };

  const toggleTaskSelection = (operationId: string) => {
    setSelectedTaskIds((current) =>
      current.includes(operationId)
        ? current.filter((item) => item !== operationId)
        : [...current, operationId],
    );
  };

  const exitSelectionMode = () => {
    if (viewMode === "tasks") {
      setTaskSelectionMode(false);
      setSelectedTaskIds([]);
      return;
    }
    setFileSelectionMode(false);
    setSelectedFileIds([]);
  };

  const enterSelectionMode = () => {
    setSearchFocused(false);
    if (viewMode === "tasks") {
      setTaskSelectionMode(true);
      setSelectedTaskIds([]);
      return;
    }
    setFileSelectionMode(true);
    setSelectedFileIds([]);
  };

  const toggleSelectAllCurrentSelection = () => {
    if (currentSelectionTargetIds.length === 0) {
      return;
    }
    if (viewMode === "tasks") {
      setSelectedTaskIds(
        allCurrentSelectionTargetsSelected
          ? []
          : [...currentSelectionTargetIds],
      );
      return;
    }
    setSelectedFileIds(
      allCurrentSelectionTargetsSelected
        ? []
        : [...currentSelectionTargetIds],
    );
  };

  const openTaskContextMenu = (
    event: React.MouseEvent,
    task: CompletedTaskEntry,
  ) => {
    event.preventDefault();
    event.stopPropagation();
    setContextMenuTarget({
      kind: "task",
      id: task.operation.operationId,
      x: event.clientX,
      y: event.clientY,
    });
  };

  const openFileContextMenu = (
    event: React.MouseEvent,
    file: CompletedFileEntry,
  ) => {
    event.preventDefault();
    event.stopPropagation();
    setContextMenuTarget({
      kind: "file",
      id: file.id,
      x: event.clientX,
      y: event.clientY,
    });
  };

  const handleViewContextMenuTarget = () => {
    if (!contextMenuTarget) {
      return;
    }
    if (contextMenuTarget.kind === "task" && contextMenuTask) {
      setViewMode("tasks");
      setTaskSelectionMode(false);
      setSelectedTaskIds([]);
      setSelectedTaskId(contextMenuTask.operation.operationId);
    } else if (contextMenuTarget.kind === "file" && contextMenuFile) {
      setViewMode("files");
      setFileSelectionMode(false);
      setSelectedFileIds([]);
      setSelectedFileId(contextMenuFile.id);
    }
    setContextMenuTarget(null);
  };

  const openRenameTaskDialog = (task: CompletedTaskEntry) => {
    const name = task.operation.name.trim() || task.operation.operationId;
    setContextMenuTarget(null);
    setRenameError("");
    setRenameDraft(name);
    setRenameTarget({
      kind: "task",
      id: task.operation.operationId,
      name,
    });
  };

  const openRenameFileDialog = (file: CompletedFileEntry) => {
    const name = file.name.trim() || file.id;
    setContextMenuTarget(null);
    setRenameError("");
    setRenameDraft(name);
    setRenameTarget({
      kind: "file",
      id: file.id,
      name,
    });
  };

  const handleRenameContextMenuTarget = () => {
    if (!canRenameContextMenuTarget) {
      return;
    }
    if (contextMenuTarget?.kind === "task" && contextMenuTask) {
      openRenameTaskDialog(contextMenuTask);
    } else if (contextMenuTarget?.kind === "file" && contextMenuFile) {
      openRenameFileDialog(contextMenuFile);
    }
  };

  const closeRenameDialog = () => {
    if (renamePending) {
      return;
    }
    setRenameTarget(null);
    setRenameDraft("");
    setRenameError("");
  };

  const executeRename = async () => {
    const target = renameTarget;
    const name = trimmedRenameDraft;
    if (!target || renamePending) {
      return;
    }
    const validationError = validateCompletedRenameDisplayName(name, props.text);
    if (validationError) {
      setRenameError(validationError);
      return;
    }
    if (name === target.name.trim()) {
      return;
    }
    setRenameError("");
    try {
      if (target.kind === "task") {
        await renameOperation.mutateAsync({
          operationId: target.id,
          name,
        });
      } else {
        await renameFile.mutateAsync({
          fileId: target.id,
          name,
        });
      }
      setRenameTarget(null);
      setRenameDraft("");
    } catch (error) {
      setRenameError(
        resolveUnknownErrorMessage(error, props.text.common.unknown),
      );
    }
  };

  const handleDeleteContextMenuTarget = () => {
    const target = contextMenuTarget;
    if (!target || !canDeleteContextMenuTarget) {
      return;
    }
    setContextMenuTarget(null);
    setDeleteConfirmError("");
    if (target.kind === "task" && contextMenuTask) {
      setDeleteConfirmTarget({
        kind: "tasks",
        ids: [target.id],
        label: contextMenuTask.operation.name,
        count: 1,
      });
      return;
    }
    if (target.kind === "file" && contextMenuFile) {
      setDeleteConfirmTarget({
        kind: "files",
        ids: [target.id],
        label: contextMenuFile.name,
        count: 1,
      });
    }
  };

  const executeDeleteConfirmation = async () => {
    const target = deleteConfirmTarget;
    if (!target || isDeleteConfirmPending) {
      return;
    }
    setDeleteConfirmError("");
    try {
      if (target.kind === "tasks") {
        await deleteOperations.mutateAsync({
          operationIds: target.ids,
          cascadeFiles: true,
        });
        setSelectedTaskIds((current) =>
          current.filter((operationId) => !target.ids.includes(operationId)),
        );
        setTaskSelectionMode(false);
        setDeleteConfirmTarget(null);
        return;
      }

      await deleteFiles.mutateAsync({
        fileIds: target.ids,
        deleteFiles: true,
      });
      setSelectedFileIds((current) =>
        current.filter((fileId) => !target.ids.includes(fileId)),
      );
      setFileSelectionMode(false);
      setDeleteConfirmTarget(null);
    } catch (error) {
      setDeleteConfirmError(
        resolveUnknownErrorMessage(error, props.text.common.unknown),
      );
    }
  };

  const handleDeleteSelectedTasks = () => {
    if (!canDeleteSelectedTasks) {
      return;
    }
    setDeleteConfirmError("");
    setDeleteConfirmTarget({
      kind: "tasks",
      ids: selectedBatchTasks.map((entry) => entry.operation.operationId),
      label: selectedBatchTasks[0]?.operation.name ?? "",
      count: selectedBatchTasks.length,
    });
  };

  const handleDeleteSelectedFiles = () => {
    if (!canDeleteSelectedFiles) {
      return;
    }
    setDeleteConfirmError("");
    setDeleteConfirmTarget({
      kind: "files",
      ids: selectedBatchFiles.map((file) => file.id),
      label: selectedBatchFiles[0]?.name ?? "",
      count: selectedBatchFiles.length,
    });
  };

  const handleDeleteCurrentSelection = () => {
    if (viewMode === "tasks") {
      handleDeleteSelectedTasks();
      return;
    }
    handleDeleteSelectedFiles();
  };

  const renderItemContextMenu = () => {
    const viewDisabled =
      contextMenuTarget?.kind === "task"
        ? !contextMenuTask
        : contextMenuTarget?.kind === "file"
          ? !contextMenuFile
          : true;

    return (
      <DropdownMenu
        open={Boolean(contextMenuTarget)}
        onOpenChange={(open) => {
          if (!open) {
            setContextMenuTarget(null);
          }
        }}
      >
        {contextMenuTarget ? (
          <DropdownMenuTrigger asChild>
            <button
              type="button"
              aria-hidden="true"
              tabIndex={-1}
              className="fixed z-50 h-px w-px opacity-0 outline-none"
              style={{
                left: contextMenuTarget.x,
                top: contextMenuTarget.y,
              }}
            />
          </DropdownMenuTrigger>
        ) : null}
        <DropdownMenuContent
          side="bottom"
          align="start"
          sideOffset={2}
          className={SIDEBAR_DROPDOWN_CONTENT_CLASS_NAME}
        >
          <div className="grid">
            <DropdownMenuItem
              className={SIDEBAR_DROPDOWN_ITEM_CLASS_NAME}
              disabled={viewDisabled}
              onSelect={handleViewContextMenuTarget}
            >
              <div className={SIDEBAR_DROPDOWN_ICON_SLOT_CLASS_NAME}>
                <Eye className="h-4 w-4 text-muted-foreground" />
              </div>
              <span className="truncate font-medium text-muted-foreground">
                {props.text.actions.view}
              </span>
            </DropdownMenuItem>
            <DropdownMenuItem
              className={SIDEBAR_DROPDOWN_ITEM_CLASS_NAME}
              disabled={!canRenameContextMenuTarget}
              onSelect={handleRenameContextMenuTarget}
            >
              <div className={SIDEBAR_DROPDOWN_ICON_SLOT_CLASS_NAME}>
                <PencilLine className="h-4 w-4 text-muted-foreground" />
              </div>
              <span className="truncate font-medium text-muted-foreground">
                {contextMenuTarget?.kind === "file"
                  ? props.text.completed.renameFile
                  : props.text.completed.renameTask}
              </span>
            </DropdownMenuItem>
            <DropdownMenuItem
              className={SIDEBAR_DROPDOWN_ITEM_CLASS_NAME}
              disabled={!canDeleteContextMenuTarget}
              onSelect={() => void handleDeleteContextMenuTarget()}
            >
              <div className={SIDEBAR_DROPDOWN_ICON_SLOT_CLASS_NAME}>
                <Trash2 className="h-4 w-4 text-muted-foreground" />
              </div>
              <span className="truncate font-medium text-muted-foreground">
                {props.text.actions.deleteItem}
              </span>
            </DropdownMenuItem>
          </div>
        </DropdownMenuContent>
      </DropdownMenu>
    );
  };

  const renderDeleteConfirmationDialog = () => (
    <Dialog
      open={Boolean(deleteConfirmTarget)}
      onOpenChange={(open) => {
        if (isDeleteConfirmPending) {
          return;
        }
        if (!open) {
          setDeleteConfirmTarget(null);
          setDeleteConfirmError("");
        }
      }}
    >
      <DialogContent className="grid max-h-[calc(100vh-2rem)] w-[min(24rem,calc(100vw-2rem))] max-w-none grid-rows-[auto_minmax(0,1fr)_auto] gap-3 overflow-hidden">
        <DialogHeader className="min-w-0">
          <DialogTitle
            className="overflow-hidden break-words pr-6 text-left leading-[1.35] [display:-webkit-box] [-webkit-box-orient:vertical] [-webkit-line-clamp:2]"
          >
            {deleteConfirmTarget
              ? resolveCompletedDeleteDialogTitle(
                  deleteConfirmTarget,
                  props.text,
                )
              : props.text.actions.deleteItem}
          </DialogTitle>
          <DialogDescription
            className="overflow-hidden break-words text-left leading-5 [display:-webkit-box] [-webkit-box-orient:vertical] [-webkit-line-clamp:3]"
          >
            {deleteConfirmTarget
              ? resolveCompletedDeleteDialogMessage(
                  deleteConfirmTarget,
                  props.text,
                )
              : ""}
          </DialogDescription>
        </DialogHeader>

        <div className="min-h-0 overflow-hidden">
          {deleteConfirmError ? (
            <div className="app-dream-status-message overflow-hidden break-words px-3 py-2 text-xs leading-5 [display:-webkit-box] [-webkit-box-orient:vertical] [-webkit-line-clamp:2]" data-intent="danger">
              {deleteConfirmError}
            </div>
          ) : null}
        </div>

        <div className="app-dialog-footer flex flex-nowrap items-center justify-between gap-2">
          <DialogClose asChild>
            <Button
              variant="outline"
              disabled={isDeleteConfirmPending}
            >
              {props.text.actions.cancelDialog}
            </Button>
          </DialogClose>
          <Button
            variant="destructive"
            disabled={!deleteConfirmTarget || isDeleteConfirmPending}
            onClick={() => void executeDeleteConfirmation()}
          >
            {isDeleteConfirmPending ? (
              <Loader2 className="h-4 w-4 animate-spin" />
            ) : null}
            {props.text.actions.deleteItem}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );

  const renderRenameDialog = () => {
    const isFileRename = renameTarget?.kind === "file";
    return (
      <Dialog
        open={Boolean(renameTarget)}
        onOpenChange={(open) => {
          if (!open) {
            closeRenameDialog();
          }
        }}
      >
        <DialogContent className="grid max-h-[calc(100vh-2rem)] w-[min(26rem,calc(100vw-2rem))] max-w-none grid-rows-[auto_auto_auto] gap-3 overflow-hidden">
          <DialogHeader className="min-w-0">
            <DialogTitle className="truncate pr-6 text-left">
              {isFileRename
                ? props.text.completed.renameFileTitle
                : props.text.completed.renameTaskTitle}
            </DialogTitle>
          </DialogHeader>

          <div className="grid min-w-0 gap-2">
            <label
              htmlFor="completed-rename-name"
              className="text-xs font-medium text-muted-foreground"
            >
              {isFileRename
                ? props.text.completed.renameFileNameLabel
                : props.text.completed.renameTaskNameLabel}
            </label>
            <Input
              id="completed-rename-name"
              ref={renameInputRef}
              value={renameDraft}
              placeholder={
                isFileRename
                  ? props.text.completed.renameFileNamePlaceholder
                  : props.text.completed.renameTaskNamePlaceholder
              }
              disabled={renamePending}
              onChange={(event) => {
                setRenameDraft(event.target.value);
                if (renameError) {
                  setRenameError("");
                }
              }}
              onKeyDown={(event) => {
                if (event.key === "Enter") {
                  event.preventDefault();
                  void executeRename();
                } else if (event.key === "Escape") {
                  event.preventDefault();
                  closeRenameDialog();
                }
              }}
            />
            {renameError ? (
              <div
                className="app-dream-status-message overflow-hidden break-words px-3 py-2 text-xs leading-5 [display:-webkit-box] [-webkit-box-orient:vertical] [-webkit-line-clamp:2]"
                data-intent="danger"
              >
                {renameError}
              </div>
            ) : null}
          </div>

          <div className="app-dialog-footer flex flex-nowrap items-center justify-between gap-2">
            <DialogClose asChild>
              <Button variant="outline" disabled={renamePending}>
                {props.text.actions.cancelDialog}
              </Button>
            </DialogClose>
            <Button disabled={!canSaveRename} onClick={() => void executeRename()}>
              {renamePending ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
              {props.text.actions.save}
            </Button>
          </div>
        </DialogContent>
      </Dialog>
    );
  };

  const renderFilterDropdownContent = () => {
    const filterLabel =
      viewMode === "tasks"
        ? props.text.completed.taskStatus
        : props.text.completed.fileType;
    const options = viewMode === "tasks" ? taskStatusOptions : fileTypeOptions;

    return (
      <DropdownMenuContent
        align="center"
        className={COMPLETED_FILTER_MENU_CONTENT_CLASS}
      >
        <DropdownMenuLabel>{filterLabel}</DropdownMenuLabel>
        <DropdownMenuItem
          className={COMPLETED_FILTER_MENU_ITEM_CLASS}
          onSelect={() => {
            if (viewMode === "tasks") {
              setTaskStatusFilters([]);
              return;
            }
            setFileTypeFilters([]);
          }}
        >
          <div className={COMPLETED_FILTER_MENU_ICON_CLASS}>
            <X className="h-4 w-4" />
          </div>
          <span className="app-completed-filter-menu-label truncate font-medium">
            {props.text.actions.clear}
          </span>
        </DropdownMenuItem>
        {options.length > 0 ? <DropdownMenuSeparator /> : null}
        {viewMode === "tasks"
          ? taskStatusOptions.map((status) => (
              <DropdownMenuCheckboxItem
                key={status}
                checked={taskStatusFilters.includes(status)}
                onSelect={(event) => event.preventDefault()}
                onCheckedChange={(checked) => {
                  setTaskStatusFilters((current) =>
                    checked
                      ? [...current, status]
                      : current.filter((item) => item !== status),
                  );
                }}
                className={COMPLETED_FILTER_MENU_CHECKBOX_CLASS}
              >
                {resolveCompletedStatusLabel(props.text, status)}
              </DropdownMenuCheckboxItem>
            ))
          : fileTypeOptions.map((type) => (
              <DropdownMenuCheckboxItem
                key={type}
                checked={fileTypeFilters.includes(type)}
                onSelect={(event) => event.preventDefault()}
                onCheckedChange={(checked) => {
                  setFileTypeFilters((current) =>
                    checked
                      ? [...current, type]
                      : current.filter((item) => item !== type),
                  );
                }}
                className={COMPLETED_FILTER_MENU_CHECKBOX_CLASS}
              >
                {resolveCompletedFileTypeLabel(type, props.text)}
              </DropdownMenuCheckboxItem>
            ))}
      </DropdownMenuContent>
    );
  };

  const activateSearchInput = React.useCallback(() => {
    if (selectionMode) {
      return;
    }
    setSearchFocused(true);
    window.requestAnimationFrame(() => {
      searchInputRef.current?.focus();
    });
  }, [selectionMode]);

  const handleSearchChange = React.useCallback(
    (event: React.ChangeEvent<HTMLInputElement>) => {
      setQuery(event.target.value);
      setSearchFocused(true);
    },
    [],
  );

  const handleSearchBlur = React.useCallback(() => {
    if (!query.length) {
      setSearchFocused(false);
    }
  }, [query.length]);

  const clearSearch = () => {
    setQuery("");
    setSearchFocused(false);
  };

  const clearCurrentListSelection = React.useCallback(() => {
    if (viewMode === "tasks") {
      if (taskSelectionMode) {
        setSelectedTaskIds([]);
        return;
      }
      setSelectedTaskId("");
      return;
    }
    if (fileSelectionMode) {
      setSelectedFileIds([]);
      return;
    }
    setSelectedFileId("");
  }, [fileSelectionMode, taskSelectionMode, viewMode]);

  const handleBlankListMouseDown = React.useCallback(
    (event: React.MouseEvent<HTMLDivElement>) => {
      if (event.target !== event.currentTarget) {
        return;
      }
      clearCurrentListSelection();
    },
    [clearCurrentListSelection],
  );

  const handleCompletedPageMouseDownCapture = React.useCallback(
    (event: React.MouseEvent<HTMLDivElement>) => {
      if (event.button !== 0 || selectionMode) {
        return;
      }
      const hasOpenDetail =
        (viewMode === "tasks" && Boolean(selectedTask)) ||
        (viewMode === "files" && Boolean(selectedFile));
      if (!hasOpenDetail) {
        return;
      }
      const target = event.target;
      if (!(target instanceof Node)) {
        return;
      }
      if (previewPresentationModeActive) {
        if (
          !isCompletedDetailOrFloatingLayerTarget(
            target,
            detailPaneRef.current,
          )
        ) {
          event.preventDefault();
          event.stopPropagation();
        }
        return;
      }
      if (isCompletedFloatingLayerTarget(target)) {
        return;
      }
      if (isCompletedListItemTarget(target)) {
        return;
      }
      if (detailPaneRef.current?.contains(target)) {
        return;
      }
      if (viewMode === "tasks") {
        setSelectedTaskId("");
        return;
      }
      setSelectedFileId("");
    },
    [
      previewPresentationModeActive,
      selectedFile,
      selectedTask,
      selectionMode,
      viewMode,
    ],
  );

  const handleCompletedPageClickCapture = React.useCallback(
    (event: React.MouseEvent<HTMLDivElement>) => {
      if (!previewPresentationModeActive || selectionMode) {
        return;
      }
      if (
        isCompletedDetailOrFloatingLayerTarget(
          event.target,
          detailPaneRef.current,
        )
      ) {
        return;
      }
      event.preventDefault();
      event.stopPropagation();
    },
    [previewPresentationModeActive, selectionMode],
  );

  const renderToolbarActionGroup = () => {
    return (
      <div className="app-dream-button-group app-completed-toolbar-actions inline-flex h-9 shrink-0 items-center p-0.5">
        <DropdownMenu>
          <Tooltip>
            <TooltipTrigger asChild>
              <DropdownMenuTrigger asChild>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  className={cn(
                    "app-completed-toolbar-button relative h-8 w-8 p-0",
                    activeFilterCount > 0
                      ? "bg-accent text-accent-foreground"
                      : "",
                  )}
                  aria-label={props.text.completed.searchFilter}
                >
                  <SlidersHorizontal className="h-4 w-4" />
                  {activeFilterCount > 0 ? (
                    <span className="absolute top-1 right-1 h-1.5 w-1.5 rounded-full bg-primary" />
                  ) : null}
                </Button>
              </DropdownMenuTrigger>
            </TooltipTrigger>
            <TooltipContent side="bottom">
              {props.text.completed.searchFilter}
            </TooltipContent>
          </Tooltip>
          {renderFilterDropdownContent()}
        </DropdownMenu>

        {!selectionMode ? (
          <>
            <CompletedFileMaintenanceControls
              text={props.text}
              httpBaseURL={props.httpBaseURL}
            />
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  className="app-completed-toolbar-button h-8 w-8 p-0"
                  aria-label={props.text.completed.selectFiles}
                  onClick={enterSelectionMode}
                >
                  <CheckCircle2 className="h-4 w-4" />
                </Button>
              </TooltipTrigger>
              <TooltipContent side="bottom">
                {props.text.completed.selectFiles}
              </TooltipContent>
            </Tooltip>
          </>
        ) : (
          <>
            <div className="app-completed-selection-summary h-8 px-3">
              <CheckCircle2 className="h-3.5 w-3.5" />
              {resolveCompletedSelectionSummary(selectionCount, props.text)}
            </div>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="app-completed-selection-action text-destructive"
              disabled={
                !canDeleteCurrentSelection ||
                deleteFiles.isPending ||
                deleteOperations.isPending
              }
              onClick={() => void handleDeleteCurrentSelection()}
            >
              <Trash2 className="h-3.5 w-3.5" />
              {props.text.completed.deleteFiles}
            </Button>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="app-completed-selection-action"
              disabled={currentSelectionTargetIds.length === 0}
              onClick={toggleSelectAllCurrentSelection}
            >
              {allCurrentSelectionTargetsSelected ? (
                <CircleSlash className="h-3.5 w-3.5" />
              ) : (
                <CheckSquare className="h-3.5 w-3.5" />
              )}
              {allCurrentSelectionTargetsSelected
                ? props.text.completed.clearSelection
                : props.text.completed.selectAll}
            </Button>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="app-completed-selection-action"
              onClick={exitSelectionMode}
            >
              <X className="h-3.5 w-3.5" />
              {props.text.actions.cancelDialog}
            </Button>
          </>
        )}
      </div>
    );
  };

  const renderEmptyListState = (label: string) => (
    <div
      className="flex h-full min-h-[16rem] items-center justify-center px-6 text-center"
      onMouseDown={handleBlankListMouseDown}
    >
      <div className="flex min-w-0 flex-col items-center justify-center gap-3">
        <PetDisplay
          pet={props.pet}
          imageUrl={props.petImageURL}
          animation="review"
          alt={props.text.appName}
          size={96}
          load={Boolean(props.petImageURL)}
          className="shrink-0"
          glowClassName="opacity-60"
        />
        <div className="max-w-[18rem] text-sm font-medium leading-5 text-muted-foreground">
          {label}
        </div>
      </div>
    </div>
  );

  const renderSelectedDetailPanel = () => {
    if (selectionMode) {
      return null;
    }

    const detailContent =
      viewMode === "tasks" && selectedTask ? (
        <CompletedTaskDetailContent
          text={props.text}
          appName={props.text.appName}
          task={selectedTask}
          selectedPreviewFileId={selectedPreviewFileId}
          onTranscodeFile={props.onTranscodeFile}
          onPreviewPresentationModeChange={setPreviewPresentationModeActive}
          pet={props.pet}
          petImageURL={props.petImageURL}
        />
      ) : viewMode === "files" && selectedFile ? (
        <CompletedFileDetailContent
          text={props.text}
          appName={props.text.appName}
          file={selectedFile}
          onTranscodeFile={props.onTranscodeFile}
          onPreviewPresentationModeChange={setPreviewPresentationModeActive}
        />
      ) : null;

    if (!detailContent) {
      return null;
    }

    const detailCoverURL =
      viewMode === "tasks" && selectedTask
        ? selectedTask.coverURL
        : viewMode === "files" && selectedFile
          ? resolveCompletedPreviewKind(selectedFile) === "image"
            ? resolveCompletedImagePreviewURL(selectedFile)
            : selectedFile.coverURL
          : "";
    return (
      <aside
        ref={detailPaneRef}
        data-preview-presentation={
          previewPresentationModeActive ? "true" : undefined
        }
        className={cn(
          "app-main-detail-pane app-completed-inline-detail my-3 flex w-[25rem] shrink-0 flex-col overflow-hidden xl:w-[27rem]",
          previewPresentationModeActive && "z-[300] overflow-visible",
        )}
      >
        {viewMode === "tasks" && selectedTask ? (
          <CompletedTaskDetailHeader
            text={props.text}
            task={selectedTask}
            coverURL={detailCoverURL}
            title={contentTitle}
            fallbackIcon={<ContentHeaderIcon className="h-5 w-5" />}
            selectedPreviewFileId={selectedPreviewFileId}
            onSelectedPreviewFileIdChange={setSelectedPreviewFileId}
            onRenameTask={openRenameTaskDialog}
            renameTaskDisabled={renamePending}
          />
        ) : viewMode === "files" && selectedFile ? (
          <CompletedFileDetailHeader
            text={props.text}
            file={selectedFile}
            coverURL={detailCoverURL}
            title={contentTitle}
            fallbackIcon={<ContentHeaderIcon className="h-5 w-5" />}
            onRenameFile={selectedFile.canDelete ? openRenameFileDialog : undefined}
            renameFileDisabled={renamePending}
          />
        ) : null}
        <div className="min-h-0 flex-1 overflow-hidden">{detailContent}</div>
      </aside>
    );
  };

  return (
    <div
      className="app-main-page app-main-completed-page flex min-h-0 flex-1 flex-col overflow-hidden bg-background"
      onMouseDownCapture={handleCompletedPageMouseDownCapture}
      onClickCapture={handleCompletedPageClickCapture}
    >
      <div
        className={cn(
          "app-completed-page-toolbar wails-drag flex min-w-0 shrink-0 items-center justify-between gap-3 px-5",
          isWindows
            ? "min-h-[var(--app-page-top-drag-height)] pb-3 pt-4"
            : "py-4",
        )}
      >
        <div className="flex min-w-0 flex-1 items-center gap-2">
          <div className="wails-no-drag shrink-0">
            <CompletedListViewSwitch
              value={viewMode}
              compact={tabsCompact}
              tasksLabel={props.text.views.tasks}
              filesLabel={props.text.views.files}
              tasksIcon={<LayoutGrid className="h-4 w-4" />}
              filesIcon={<Files className="h-4 w-4" />}
              onValueChange={setViewMode}
            />
          </div>
          {renderToolbarActionGroup()}
          <div
            className={cn(
              "app-dream-search-control app-dream-control-shell app-completed-search-control wails-no-drag h-9 transition-[width,box-shadow,border-color] duration-200 ease-out",
              searchInputExpanded
                ? "w-[18rem] min-w-[10rem] px-3"
                : "w-9 min-w-9 shrink-0 grow-0 cursor-text justify-center px-0",
            )}
            onMouseDown={(event) => {
              if (searchInputExpanded || selectionMode) {
                return;
              }
              event.preventDefault();
              activateSearchInput();
            }}
            onKeyDown={(event) => {
              if (searchInputExpanded || selectionMode) {
                return;
              }
              if (event.key === "Enter" || event.key === " ") {
                event.preventDefault();
                activateSearchInput();
              }
            }}
            role={searchInputExpanded || selectionMode ? undefined : "button"}
            tabIndex={searchInputExpanded || selectionMode ? undefined : 0}
          >
            <Search className="h-4 w-4 shrink-0 text-muted-foreground" />
            {searchInputExpanded ? (
              <>
                <Input
                  ref={searchInputRef}
                  value={query}
                  onChange={handleSearchChange}
                  onFocus={() => setSearchFocused(true)}
                  onBlur={handleSearchBlur}
                  placeholder={searchPlaceholder}
                  size="compact"
                  className="app-control-input-compact h-auto min-w-0 flex-1 rounded-none border-0 bg-transparent px-0 shadow-none"
                />
                <span
                  className={cn(
                    "block shrink-0 overflow-hidden transition-[width,opacity,transform] duration-200 ease-out",
                    searchHasText
                      ? "w-5 translate-x-0 opacity-100"
                      : "w-0 -translate-x-1 opacity-0",
                  )}
                >
                  <button
                    type="button"
                    aria-label={props.text.actions.clear}
                    title={props.text.actions.clear}
                    disabled={!searchHasText}
                    tabIndex={searchHasText ? 0 : -1}
                    className="app-completed-search-clear flex h-5 w-5 items-center justify-center transition focus-visible:outline-none disabled:pointer-events-none"
                    onMouseDown={(event) => event.preventDefault()}
                    onClick={clearSearch}
                  >
                    <X className="h-3.5 w-3.5" />
                  </button>
                </span>
              </>
            ) : null}
          </div>
        </div>
        <div
          className={cn(
            "shrink-0 justify-end",
            isWindows
              ? "flex min-w-[var(--app-windows-caption-control-width)]"
              : "hidden",
          )}
        >
          {isWindows ? <WindowControls platform="windows" /> : null}
        </div>
      </div>

      <div className="app-main-page-content app-main-completed-content flex min-h-0 flex-1 gap-3 overflow-hidden pr-3">
        <div
          className="app-main-list-content min-h-0 flex-1 overflow-y-auto px-5 py-5"
          onMouseDown={handleBlankListMouseDown}
        >
          {viewMode === "tasks" ? (
            pagedTasks.length === 0 ? (
              renderEmptyListState(props.text.completed.emptyTasks)
            ) : (
              <div
                className="app-completed-task-grid grid gap-x-4 gap-y-5"
                style={{
                  gridTemplateColumns:
                    "repeat(auto-fill, minmax(min(100%, 7.75rem), 1fr))",
                }}
                onMouseDown={handleBlankListMouseDown}
              >
                {pagedTasks.map((entry) => {
                  const isChecked = selectedTaskIdsSet.has(
                    entry.operation.operationId,
                  );
                  const isActive =
                    !taskSelectionMode &&
                    selectedTaskId === entry.operation.operationId;
                  const statusLabel = resolveCompletedStatusLabel(
                    props.text,
                    entry.operation.status,
                  );
                  const StatusIcon = resolveCompletedTaskStatusIcon(
                    entry.operation.status,
                  );
                  const updatedAtLabel = entry.updatedAt
                    ? formatRelativeTime(entry.updatedAt)
                    : "";
                  const statusTimeLabel = updatedAtLabel
                    ? `${statusLabel} · ${updatedAtLabel}`
                    : statusLabel;
                  const fileSummaryItems = buildCompletedTaskFileSummaryItems(
                    entry,
                    props.text,
                  ).filter((item) => item.count > 0);

                  return (
                    <button
                      key={entry.operation.operationId}
                      type="button"
                      onClick={() => {
                        if (taskSelectionMode) {
                          toggleTaskSelection(entry.operation.operationId);
                          return;
                        }
                        setSelectedTaskId(entry.operation.operationId);
                      }}
                      onContextMenu={(event) =>
                        openTaskContextMenu(event, entry)
                      }
                      data-active={isActive ? "true" : "false"}
                      data-selected={
                        taskSelectionMode && isChecked ? "true" : "false"
                      }
                      className="app-completed-task-card group relative overflow-hidden text-left transition"
                    >
                      <div className="app-completed-task-card-cover relative aspect-square overflow-hidden">
                        <img
                          src={entry.coverURL}
                          alt={entry.operation.name}
                          className="app-completed-task-card-image h-full w-full object-cover"
                          loading="lazy"
                          decoding="async"
                          draggable={false}
                        />
                        <img
                          src={entry.coverURL}
                          alt=""
                          aria-hidden="true"
                          className="app-completed-task-card-glow pointer-events-none absolute inset-0 h-full w-full scale-[1.08] object-cover"
                          loading="lazy"
                          decoding="async"
                          draggable={false}
                        />
                        <div className="app-completed-task-card-cover-overlay absolute inset-0 transition" />
                        <div className="app-completed-task-card-sheen absolute inset-y-[-20%] left-[-48%] w-[38%] rotate-[12deg]" />
                        {taskSelectionMode ? (
                          <SelectionCheckbox
                            checked={isChecked}
                            className="absolute top-1 left-1"
                          />
                        ) : null}
                        <span
                          className="app-completed-task-card-status-time absolute bottom-1.5 left-1.5 inline-flex max-w-[calc(100%-0.75rem)] items-center gap-1 truncate px-2 py-1 text-2xs font-medium"
                          title={statusTimeLabel}
                          aria-label={statusTimeLabel}
                        >
                          <StatusIcon
                            className={cn(
                              "h-3 w-3 shrink-0",
                              resolveCompletedTaskStatusIconTone(
                                entry.operation.status,
                              ),
                            )}
                            aria-hidden="true"
                          />
                          {updatedAtLabel ? (
                            <span className="min-w-0 truncate">
                              {updatedAtLabel}
                            </span>
                          ) : null}
                        </span>
                      </div>
                      <div className="app-completed-task-card-body relative grid gap-1 px-0.5 pt-1.5">
                        <div
                          className="app-completed-task-card-title truncate text-xs font-semibold leading-4 transition-colors"
                          title={entry.operation.name}
                        >
                          {entry.operation.name}
                        </div>
                        {fileSummaryItems.length > 0 ? (
                          <div className="app-completed-task-card-meta flex min-w-0 items-center gap-1 overflow-hidden text-2xs font-medium leading-4">
                            {fileSummaryItems.map((item) => {
                              const Icon = item.icon;
                              return (
                                <span
                                  key={item.key}
                                  className="inline-flex min-w-0 shrink-0 items-center gap-0.5"
                                  title={`${item.label} ${item.count}`}
                                  aria-label={`${item.label} ${item.count}`}
                                >
                                  <Icon className="h-3 w-3 shrink-0" />
                                  <span className="tabular-nums">
                                    {item.count}
                                  </span>
                                </span>
                              );
                            })}
                          </div>
                        ) : (
                          <div
                            className="app-completed-task-card-meta flex min-w-0 items-center overflow-hidden text-2xs font-medium leading-4"
                            aria-hidden="true"
                          />
                        )}
                      </div>
                    </button>
                  );
                })}
              </div>
            )
          ) : pagedFiles.length === 0 ? (
            renderEmptyListState(props.text.completed.emptyFiles)
          ) : (
            <div
              className="app-completed-file-list space-y-2"
              onMouseDown={handleBlankListMouseDown}
            >
              {pagedFiles.map((file) => {
                const previewKind = resolveCompletedPreviewKind(file);
                const fileThumbnailURL =
                  previewKind === "image"
                    ? resolveCompletedImagePreviewURL(file)
                    : file.coverURL;
                const hasThumbnail = Boolean(fileThumbnailURL);
                const isChecked = selectedFileIdsSet.has(file.id);
                const isActive =
                  !fileSelectionMode && selectedFileId === file.id;
                const FileIcon = resolveCompletedFileIcon(file);
                const fileTaskLabel =
                  file.operationName ||
                  formatCompletedTranscodedFromLabel(
                    props.text,
                    file.sourceFileName,
                  ) ||
                  file.libraryName;

                return (
                  <button
                    key={file.id}
                    type="button"
                    onClick={() => {
                      if (fileSelectionMode) {
                        toggleFileSelection(file.id);
                        return;
                      }
                      setSelectedFileId(file.id);
                    }}
                    onContextMenu={(event) => openFileContextMenu(event, file)}
                    data-active={isActive ? "true" : "false"}
                    data-preview-kind={previewKind}
                    data-selected={
                      fileSelectionMode && isChecked ? "true" : "false"
                    }
                    className="app-completed-file-card group relative flex w-full items-center gap-3 overflow-hidden px-3 py-2.5 text-left transition"
                  >
                    <span className="app-completed-file-thumb relative flex h-11 w-11 shrink-0 items-center justify-center overflow-hidden rounded-lg">
                      {hasThumbnail ? (
                        <img
                          src={fileThumbnailURL}
                          alt=""
                          aria-hidden="true"
                          className="h-full w-full object-cover"
                          loading="lazy"
                          decoding="async"
                          draggable={false}
                        />
                      ) : null}
                      <span
                        className={cn(
                          "app-completed-file-icon flex h-7 w-7 shrink-0 items-center justify-center rounded-md",
                          hasThumbnail
                            ? "app-completed-file-icon-floating absolute bottom-1 right-1 !h-5 !w-5"
                            : "relative",
                        )}
                      >
                        <FileIcon className="h-3.5 w-3.5" />
                      </span>
                      {fileSelectionMode ? (
                        <SelectionCheckbox
                          checked={isChecked}
                          className="absolute left-1 top-1"
                        />
                      ) : null}
                    </span>
                    <div className="min-w-0 flex-1">
                      <div className="app-completed-file-title truncate text-sm font-semibold leading-5 transition-colors">
                        {file.name}
                      </div>
                      <div className="app-completed-file-subtitle mt-0.5 truncate text-xs leading-4">
                        {fileTaskLabel}
                      </div>
                    </div>
                    <div className="app-completed-file-meta flex shrink-0 items-center gap-2 text-xs">
                      <span className="app-completed-file-format rounded-md px-1.5 py-0.5 font-medium">
                        {file.format}
                      </span>
                      <span className="app-completed-file-size min-w-[3.75rem] text-right tabular-nums">
                        {file.sizeBytes > 0
                          ? formatBytes(file.sizeBytes)
                          : "--"}
                      </span>
                    </div>
                  </button>
                );
              })}
            </div>
          )}
        </div>
        {renderSelectedDetailPanel()}
      </div>

      <div className="app-main-page-footer flex items-center justify-between gap-3 px-5 py-3">
          <div className="text-xs text-muted-foreground">
            {resolveCompletedTotalLabel(
              currentTotalCount,
              viewMode,
              props.text,
            )}
          </div>
          <div className="ml-auto flex items-center gap-2">
            <Select
              value={String(currentPageSize)}
              onChange={(event) => {
                const nextPageSize = Number(event.target.value);
                if (!Number.isFinite(nextPageSize) || nextPageSize <= 0) {
                  return;
                }
                if (viewMode === "tasks") {
                  setTaskPageSize(nextPageSize);
                  setTaskPage(1);
                  return;
                }
                setFilePageSize(nextPageSize);
                setFilePage(1);
              }}
              aria-label={props.text.completed.perPage}
              className="app-completed-page-size-select h-8 min-w-[7.5rem] px-2.5 text-xs"
            >
              {currentPageSizeOptions.map((option) => (
                <option key={option} value={option}>
                  {resolveCompletedPerPageLabel(option, props.text)}
                </option>
              ))}
            </Select>
            <div className="min-w-[4.75rem] text-center text-xs text-muted-foreground">
              {resolveCompletedPageLabel(
                currentPage,
                currentPageCount,
                props.text,
              )}
            </div>
            <div className="app-dream-button-group app-completed-footer-pager inline-flex h-[var(--app-control-height-compact)] shrink-0 overflow-hidden">
              <Button
                type="button"
                variant="ghost"
                size="icon"
                className="app-completed-footer-pager-button !h-full !w-[var(--app-control-height-compact)] rounded-none"
                aria-label={props.text.completed.previousPage}
                title={props.text.completed.previousPage}
                disabled={currentPage <= 1}
                onClick={() => {
                  if (viewMode === "tasks") {
                    setTaskPage((page) => Math.max(1, page - 1));
                    return;
                  }
                  setFilePage((page) => Math.max(1, page - 1));
                }}
              >
                <ChevronRight className="h-4 w-4 rotate-180" />
              </Button>
              <Button
                type="button"
                variant="ghost"
                size="icon"
                className="app-completed-footer-pager-button !h-full !w-[var(--app-control-height-compact)] rounded-none"
                aria-label={props.text.completed.nextPage}
                title={props.text.completed.nextPage}
                disabled={currentPage >= currentPageCount}
                onClick={() => {
                  if (viewMode === "tasks") {
                    setTaskPage((page) => Math.min(taskPageCount, page + 1));
                    return;
                  }
                  setFilePage((page) => Math.min(filePageCount, page + 1));
                }}
              >
                <ChevronRight className="h-4 w-4" />
              </Button>
            </div>
          </div>
      </div>

      {renderItemContextMenu()}
      {renderRenameDialog()}
      {renderDeleteConfirmationDialog()}
    </div>
  );
}
