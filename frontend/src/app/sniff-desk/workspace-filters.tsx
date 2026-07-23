import { Search } from "lucide-react";
import { useEffect, useRef } from "react";
import { create } from "zustand";

import { getXiaText } from "@/features/xiadown/shared";
import { useSettingsStore } from "@/shared/store/settings";
import { Input } from "@/shared/ui/input";
import { Select } from "@/shared/ui/select";

type XiaText = ReturnType<typeof getXiaText>;

export type SniffWorkspaceKindFilter =
  | "all"
  | "video"
  | "audio"
  | "live"
  | "manifest"
  | "image"
  | "subtitle"
  | "api"
  | "document"
  | "font"
  | "archive"
  | "other";
export type SniffWorkspaceSourceFilter =
  | "all"
  | "network"
  | "candidate"
  | "rejected";
export type SniffWorkspaceDownloadFilter = "all" | "downloadable";
const DEFAULT_KINDS: SniffWorkspaceKindFilter[] = [
  "all",
  "video",
  "audio",
  "live",
  "image",
  "subtitle",
];
const ADVANCED_KINDS: SniffWorkspaceKindFilter[] = [
  ...DEFAULT_KINDS,
  "manifest",
  "api",
  "other",
];
const ALL_KINDS: SniffWorkspaceKindFilter[] = [
  ...DEFAULT_KINDS,
  "manifest",
  "api",
  "document",
  "font",
  "archive",
  "other",
];

interface SniffWorkspaceFilterState {
  query: string;
  kind: SniffWorkspaceKindFilter;
  source: SniffWorkspaceSourceFilter;
  download: SniffWorkspaceDownloadFilter;
  setQuery: (query: string) => void;
  setKind: (kind: SniffWorkspaceKindFilter) => void;
  setSource: (source: SniffWorkspaceSourceFilter) => void;
  setDownload: (download: SniffWorkspaceDownloadFilter) => void;
  reset: () => void;
}

export const useSniffWorkspaceFilterStore = create<SniffWorkspaceFilterState>(
  (set) => ({
    query: "",
    kind: "all",
    source: "all",
    download: "all",
    setQuery: (query) => set({ query }),
    setKind: (kind) => set({ kind }),
    setSource: (source) => set({ source }),
    setDownload: (download) => set({ download }),
    reset: () =>
      set({ query: "", kind: "all", source: "all", download: "all" }),
  }),
);

type SniffWorkspaceStartHandoff = {
  requestId: string;
  sessionId: string;
  startedAt: number;
};

interface SniffWorkspaceStartState {
  pending: SniffWorkspaceStartHandoff | null;
}

export const useSniffWorkspaceStartStore = create<SniffWorkspaceStartState>(
  () => ({ pending: null }),
);

let sniffWorkspaceStartSequence = 0;
export const SNIFF_WORKSPACE_START_TIMEOUT_MS = 15_000;

export function beginSniffWorkspaceStart() {
  sniffWorkspaceStartSequence += 1;
  const requestId = `sniff-start-${sniffWorkspaceStartSequence}`;
  useSniffWorkspaceFilterStore.getState().reset();
  useSniffWorkspaceStartStore.setState({
    pending: { requestId, sessionId: "", startedAt: Date.now() },
  });
  return requestId;
}

export function attachSniffWorkspaceStartSession(
  requestId: string,
  sessionId: string,
) {
  useSniffWorkspaceStartStore.setState((state) =>
    state.pending?.requestId === requestId
      ? {
          pending: {
            ...state.pending,
            sessionId: sessionId.trim(),
          },
        }
      : state,
  );
}

export function clearSniffWorkspaceStart(requestId: string) {
  useSniffWorkspaceStartStore.setState((state) =>
    state.pending?.requestId === requestId ? { pending: null } : state,
  );
}

export function SniffWorkspaceSearchField(props: {
  text: XiaText;
  active?: boolean;
}) {
  const inputRef = useRef<HTMLInputElement | null>(null);
  const query = useSniffWorkspaceFilterStore((state) => state.query);
  const setQuery = useSniffWorkspaceFilterStore((state) => state.setQuery);
  useEffect(() => {
    if (props.active) {
      inputRef.current?.focus();
    }
  }, [props.active]);
  return (
    <label className="app-dream-search-control app-dream-control-shell flex h-9 w-full items-center gap-2 px-2.5">
      <Search className="app-sniff-desk-filter-icon h-3.5 w-3.5 shrink-0" />
      <Input
        ref={inputRef}
        value={query}
        onChange={(event) => setQuery(event.currentTarget.value)}
        placeholder={props.text.sniffDesk.searchPlaceholder}
        size="compact"
        className="app-sniff-desk-filter-input h-auto min-w-0 px-0"
      />
    </label>
  );
}

export function SniffWorkspaceKindSelect(props: { text: XiaText }) {
  const scope = useSettingsStore(
    (state) => state.settings?.resourceSniffScope ?? "default",
  );
  const value = useSniffWorkspaceFilterStore((state) => state.kind);
  const setValue = useSniffWorkspaceFilterStore((state) => state.setKind);
  const options = kindsForScope(scope);
  return (
    <Select
      aria-label={props.text.sniffDesk.kindFilter}
      className="h-9 w-full"
      value={value}
      onChange={(event) =>
        setValue(event.currentTarget.value as SniffWorkspaceKindFilter)
      }
    >
      {options.map((kind) => (
        <option key={kind} value={kind}>
          {kind === "all"
            ? props.text.sniffDesk.allKinds
            : kindLabel(props.text, kind)}
        </option>
      ))}
    </Select>
  );
}

export function SniffWorkspaceSourceSelect(props: { text: XiaText }) {
  const value = useSniffWorkspaceFilterStore((state) => state.source);
  const setValue = useSniffWorkspaceFilterStore((state) => state.setSource);
  const options: Array<{
    value: SniffWorkspaceSourceFilter;
    label: string;
  }> = (["all", "network", "candidate", "rejected"] as const).map(
    (source) => ({
      value: source,
      label:
        source === "all"
          ? props.text.sniffDesk.allSources
          : sourceLabel(props.text, source),
    }),
  );
  return (
    <SniffWorkspaceSegmentedFilter
      ariaLabel={props.text.sniffDesk.sourceFilter}
      name="sniff-workspace-source"
      value={value}
      options={options}
      onValueChange={setValue}
    />
  );
}

export function SniffWorkspaceResourceSelect(props: { text: XiaText }) {
  const value = useSniffWorkspaceFilterStore((state) => state.download);
  const setValue = useSniffWorkspaceFilterStore((state) => state.setDownload);
  return (
    <SniffWorkspaceSegmentedFilter
      ariaLabel={props.text.sniffDesk.downloadFilter}
      name="sniff-workspace-download"
      value={value}
      options={[
        { value: "all", label: props.text.sniffDesk.allDownloads },
        {
          value: "downloadable",
          label: props.text.sniffDesk.downloadableOnly,
        },
      ]}
      onValueChange={setValue}
    />
  );
}

function SniffWorkspaceSegmentedFilter<T extends string>(props: {
  ariaLabel: string;
  name: string;
  value: T;
  options: ReadonlyArray<{ value: T; label: string }>;
  onValueChange: (value: T) => void;
}) {
  return (
    <div
      aria-label={props.ariaLabel}
      className="app-sniff-workspace-segmented"
      role="radiogroup"
    >
      {props.options.map((option) => (
        <label
          className="app-sniff-workspace-segmented__option"
          key={option.value}
        >
          <input
            checked={props.value === option.value}
            className="app-sniff-workspace-segmented__input"
            name={props.name}
            onChange={() => props.onValueChange(option.value)}
            type="radio"
            value={option.value}
          />
          <span className="app-sniff-workspace-segmented__label">
            {option.label}
          </span>
        </label>
      ))}
    </div>
  );
}

function kindsForScope(scope: string) {
  switch (scope.trim().toLowerCase()) {
    case "advanced":
      return ADVANCED_KINDS;
    case "all":
      return ALL_KINDS;
    default:
      return DEFAULT_KINDS;
  }
}

function kindLabel(text: XiaText, kind: SniffWorkspaceKindFilter) {
  switch (kind) {
    case "video":
      return text.sniffDesk.kindVideo;
    case "audio":
      return text.sniffDesk.kindAudio;
    case "live":
      return text.sniffDesk.kindLive;
    case "manifest":
      return text.sniffDesk.kindManifest;
    case "image":
      return text.sniffDesk.kindImage;
    case "subtitle":
      return text.sniffDesk.kindSubtitle;
    case "api":
      return text.sniffDesk.kindApi;
    case "document":
      return text.sniffDesk.kindDocument;
    case "font":
      return text.sniffDesk.kindFont;
    case "archive":
      return text.sniffDesk.kindArchive;
    default:
      return text.sniffDesk.kindOther;
  }
}

function sourceLabel(text: XiaText, source: SniffWorkspaceSourceFilter) {
  switch (source) {
    case "network":
      return text.sniffDesk.sourceNetwork;
    case "candidate":
      return text.sniffDesk.sourceCandidate;
    case "rejected":
      return text.sniffDesk.sourceRejected;
    default:
      return text.sniffDesk.allSources;
  }
}
