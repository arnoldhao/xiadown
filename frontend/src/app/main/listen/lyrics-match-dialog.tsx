import {
  BadgeCheck,
  Check,
  CircleAlert,
  Clock3,
  FileText,
  Languages,
  Loader2,
  Music2,
  Search,
  Sparkles,
  Waves,
} from "lucide-react";
import * as React from "react";

import type { getXiaText } from "@/features/xiadown/shared";
import {
  callListenLyricsCandidate,
  callListenLyricsCandidates,
  type ListenLyricsCandidateTrack,
} from "@/app/main/listen/lyrics-api";
import type { ListenLyricsFocusStyle } from "@/app/main/listen/lyrics-focus-style";
import { ListenLyricsSurface } from "@/app/main/listen/lyrics";
import type { ListenLyricsRendererMode } from "@/app/main/listen/lyrics-renderers";
import {
  buildListenLyricsCandidateTrack,
  createListenLyricsPreviewCache,
  createListenLyricsRequestGate,
  formatListenLyricsCandidateDuration,
  initialListenLyricsSearchDraft,
  listenLyricsCandidateKey,
  listenLyricsCandidatePreviewKey,
  type ListenLyricsSearchDraft,
} from "@/app/main/listen/lyrics-workspace-state";
import type {
  ListenLyricsCandidate,
  ListenLyricsData,
  ListenLyricTimingQuality,
} from "@/app/main/listen/types";
import { Button } from "@/shared/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/shared/ui/dialog";
import { Input } from "@/shared/ui/input";

type LyricsText = ReturnType<typeof getXiaText>;

export type ListenLyricsMatchDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  text: LyricsText;
  track: ListenLyricsCandidateTrack;
  language?: string;
  synced?: boolean;
  renderer: ListenLyricsRendererMode;
  focusStyle?: ListenLyricsFocusStyle;
  currentTimeMs: number;
  offsetMs?: number;
  timelineRunning?: boolean;
  playbackRate?: number;
  romanized?: boolean;
  pinyin?: boolean;
  hasManualOverride: boolean;
  returnFocusRef?: React.RefObject<HTMLElement>;
  onConfirm: (
    candidate: ListenLyricsCandidate,
    lyrics: ListenLyricsData,
  ) => void | Promise<void>;
  onRestoreAutomatic: () => void | Promise<void>;
};

type PreviewState = {
  key: string;
  loading: boolean;
  lyrics: ListenLyricsData | null;
  error: boolean;
};

const EMPTY_PREVIEW: PreviewState = {
  key: "",
  loading: false,
  lyrics: null,
  error: false,
};

export function ListenLyricsMatchDialog(props: ListenLyricsMatchDialogProps) {
  const trackIdentity = [
    props.track.lyricsId,
    props.track.videoId,
    props.track.title,
    props.track.artist,
    props.track.channel,
    props.track.album,
    props.track.localPath,
    props.track.durationSeconds,
  ].join("\u0000");
  const stableTrack = React.useMemo(() => props.track, [trackIdentity]);
  const [draft, setDraft] = React.useState<ListenLyricsSearchDraft>(() =>
    initialListenLyricsSearchDraft(props.track),
  );
  const [submittedTrack, setSubmittedTrack] =
    React.useState<ListenLyricsCandidateTrack>(props.track);
  const [searchOwnerKey, setSearchOwnerKey] = React.useState(trackIdentity);
  const [candidates, setCandidates] = React.useState<ListenLyricsCandidate[]>(
    [],
  );
  const [selectedKey, setSelectedKey] = React.useState("");
  const [searching, setSearching] = React.useState(false);
  const [searchFinished, setSearchFinished] = React.useState(false);
  const [searchError, setSearchError] = React.useState(false);
  const [preview, setPreview] = React.useState<PreviewState>(EMPTY_PREVIEW);
  const [action, setAction] = React.useState<"confirm" | "restore" | "">("");
  const [actionError, setActionError] = React.useState(false);
  const searchGateRef = React.useRef(createListenLyricsRequestGate());
  const previewGateRef = React.useRef(createListenLyricsRequestGate());
  const previewCacheRef = React.useRef(createListenLyricsPreviewCache());
  const openRef = React.useRef(props.open);
  const selectedKeyRef = React.useRef(selectedKey);

  openRef.current = props.open;
  selectedKeyRef.current = selectedKey;

  React.useEffect(() => {
    return () => {
      openRef.current = false;
      searchGateRef.current.invalidate();
      previewGateRef.current.invalidate();
    };
  }, []);

  const loadPreview = React.useCallback(
    (
      candidate: ListenLyricsCandidate,
      searchTrack: ListenLyricsCandidateTrack,
    ) => {
      const candidateKey = listenLyricsCandidateKey(candidate);
      const previewKey = listenLyricsCandidatePreviewKey({
        track: searchTrack,
        candidate,
        language: props.language,
        synced: props.synced,
      });
      selectedKeyRef.current = candidateKey;
      setSelectedKey(candidateKey);
      setActionError(false);
      setPreview({
        key: previewKey,
        loading: true,
        lyrics: null,
        error: false,
      });
      const request = previewGateRef.current.begin();
      void previewCacheRef.current
        .load(previewKey, () =>
          callListenLyricsCandidate({
            track: searchTrack,
            candidate,
            language: props.language,
            synced: props.synced,
          }),
        )
        .then((lyrics) => {
          if (
            !openRef.current ||
            !previewGateRef.current.isCurrent(request) ||
            selectedKeyRef.current !== candidateKey
          ) {
            return;
          }
          setPreview({ key: previewKey, loading: false, lyrics, error: false });
        })
        .catch(() => {
          if (
            !openRef.current ||
            !previewGateRef.current.isCurrent(request) ||
            selectedKeyRef.current !== candidateKey
          ) {
            return;
          }
          setPreview({
            key: previewKey,
            loading: false,
            lyrics: null,
            error: true,
          });
        });
    }, [props.language, props.synced],
  );

  const runSearch = React.useCallback(
    (searchDraft: ListenLyricsSearchDraft) => {
      const searchTrack = buildListenLyricsCandidateTrack(
        stableTrack,
        searchDraft,
      );
      if (!searchTrack.title) {
        setSearchError(true);
        setSearchFinished(true);
        setCandidates([]);
        return;
      }
      const request = searchGateRef.current.begin();
      previewGateRef.current.invalidate();
      setSubmittedTrack(searchTrack);
      setSearchOwnerKey(trackIdentity);
      setSearching(true);
      setSearchFinished(false);
      setSearchError(false);
      setCandidates([]);
      setSelectedKey("");
      selectedKeyRef.current = "";
      setPreview(EMPTY_PREVIEW);
      setActionError(false);
      void callListenLyricsCandidates({
        track: searchTrack,
        language: props.language,
        synced: props.synced,
      })
        .then((nextCandidates) => {
          if (
            !openRef.current ||
            !searchGateRef.current.isCurrent(request)
          ) {
            return;
          }
          setCandidates(nextCandidates);
          setSearching(false);
          setSearchFinished(true);
          const preferred =
            nextCandidates.find((candidate) => candidate.accepted) ??
            nextCandidates[0];
          if (preferred) {
            loadPreview(preferred, searchTrack);
          }
        })
        .catch(() => {
          if (
            !openRef.current ||
            !searchGateRef.current.isCurrent(request)
          ) {
            return;
          }
          setSearching(false);
          setSearchFinished(true);
          setSearchError(true);
        });
    },
    [loadPreview, props.language, props.synced, stableTrack, trackIdentity],
  );

  React.useEffect(() => {
    previewCacheRef.current.clear();
    searchGateRef.current.invalidate();
    previewGateRef.current.invalidate();
    const nextDraft = initialListenLyricsSearchDraft(stableTrack);
    setDraft(nextDraft);
    setSubmittedTrack(stableTrack);
    setSearchOwnerKey(trackIdentity);
    setCandidates([]);
    setSelectedKey("");
    setPreview(EMPTY_PREVIEW);
    setSearchFinished(false);
    setSearchError(false);
    setActionError(false);
    if (props.open) {
      runSearch(nextDraft);
    }
  }, [props.open, runSearch, stableTrack, trackIdentity]);

  const handleOpenChange = (open: boolean) => {
    openRef.current = open;
    if (!open) {
      searchGateRef.current.invalidate();
      previewGateRef.current.invalidate();
    }
    props.onOpenChange(open);
  };

  const selectedCandidate = candidates.find(
    (candidate) => listenLyricsCandidateKey(candidate) === selectedKey,
  );
  const previewReady =
    searchOwnerKey === trackIdentity &&
    Boolean(selectedCandidate) &&
    Boolean(preview.lyrics) &&
    preview.lyrics?.kind !== "unavailable";
  const busy = Boolean(action);

  const handleConfirm = async () => {
    if (!selectedCandidate || !preview.lyrics || !previewReady || busy) {
      return;
    }
    setAction("confirm");
    setActionError(false);
    try {
      await props.onConfirm(selectedCandidate, preview.lyrics);
      if (openRef.current) {
        handleOpenChange(false);
      }
    } catch {
      if (openRef.current) {
        setActionError(true);
      }
    } finally {
      if (openRef.current) {
        setAction("");
      }
    }
  };

  const handleRestore = async () => {
    if (busy) {
      return;
    }
    setAction("restore");
    setActionError(false);
    try {
      await props.onRestoreAutomatic();
      if (openRef.current) {
        handleOpenChange(false);
      }
    } catch {
      if (openRef.current) {
        setActionError(true);
      }
    } finally {
      if (openRef.current) {
        setAction("");
      }
    }
  };

  return (
    <Dialog open={props.open} onOpenChange={handleOpenChange}>
      <DialogContent
        className="listen-lyrics-match-dialog max-w-[min(52rem,calc(100vw-2rem))]"
        onCloseAutoFocus={(event) => {
          const target = props.returnFocusRef?.current;
          if (!target?.isConnected) {
            return;
          }
          event.preventDefault();
          window.requestAnimationFrame(() => target.focus());
        }}
      >
        <DialogHeader className="listen-lyrics-match-dialog__header">
          <DialogTitle>{props.text.listen.lyricsMatchTitle}</DialogTitle>
          <DialogDescription>
            {props.text.listen.lyricsMatchDescription}
          </DialogDescription>
        </DialogHeader>

        <form
          className="listen-lyrics-match-dialog__search"
          onSubmit={(event) => {
            event.preventDefault();
            runSearch(draft);
          }}
        >
          <ListenLyricsSearchField
            label={props.text.listen.lyricsMatchSearchTitle}
            value={draft.title}
            onChange={(title) => setDraft((current) => ({ ...current, title }))}
            required
          />
          <ListenLyricsSearchField
            label={props.text.listen.lyricsMatchSearchArtist}
            value={draft.artist}
            onChange={(artist) =>
              setDraft((current) => ({ ...current, artist }))
            }
          />
          <ListenLyricsSearchField
            label={props.text.listen.lyricsMatchSearchAlbum}
            value={draft.album}
            onChange={(album) => setDraft((current) => ({ ...current, album }))}
          />
          <Button
            type="submit"
            size="compact"
            className="listen-lyrics-match-dialog__search-button"
            disabled={searching || !draft.title.trim()}
          >
            {searching ? (
              <Loader2 className="h-3.5 w-3.5 listen-loading-spinner" />
            ) : (
              <Search className="h-3.5 w-3.5" />
            )}
            {searching
              ? props.text.listen.lyricsMatchSearching
              : props.text.listen.lyricsMatchSearchAction}
          </Button>
        </form>

        <div className="listen-lyrics-match-dialog__workspace">
          <section
            className="listen-lyrics-match-dialog__candidates"
            aria-label={props.text.listen.lyricsMatchCandidates}
            aria-busy={searching}
          >
            <div className="listen-lyrics-match-dialog__section-heading">
              <div>
                <div>{props.text.listen.lyricsMatchCandidates}</div>
                <span>
                  {candidates.length > 0 ? String(candidates.length) : ""}
                </span>
              </div>
            </div>
            <div className="listen-lyrics-match-dialog__candidate-scroll">
              {searching ? (
                <ListenLyricsMatchStatus
                  icon={<Loader2 className="h-4 w-4 listen-loading-spinner" />}
                  label={props.text.listen.lyricsMatchSearching}
                />
              ) : searchError ? (
                <ListenLyricsMatchStatus
                  icon={<CircleAlert className="h-4 w-4" />}
                  label={props.text.listen.lyricsMatchSearchFailed}
                  tone="error"
                />
              ) : searchFinished && candidates.length === 0 ? (
                <ListenLyricsMatchStatus
                  icon={<Search className="h-4 w-4" />}
                  label={props.text.listen.lyricsMatchEmpty}
                />
              ) : (
                candidates.map((candidate) => (
                  <ListenLyricsCandidateCard
                    key={listenLyricsCandidateKey(candidate)}
                    candidate={candidate}
                    selected={
                      listenLyricsCandidateKey(candidate) === selectedKey
                    }
                    text={props.text}
                    onSelect={() => loadPreview(candidate, submittedTrack)}
                  />
                ))
              )}
            </div>
          </section>

          <section
            className="listen-lyrics-match-dialog__preview"
            aria-label={props.text.listen.lyricsMatchPreview}
            aria-busy={preview.loading}
          >
            <div className="listen-lyrics-match-dialog__section-heading">
              <div>{props.text.listen.lyricsMatchPreview}</div>
              {selectedCandidate ? (
                <span>{selectedCandidate.attribution ?? selectedCandidate.providerId}</span>
              ) : null}
            </div>
            <div className="listen-lyrics-match-dialog__preview-surface">
              {preview.loading ? (
                <ListenLyricsMatchStatus
                  icon={<Loader2 className="h-4 w-4 listen-loading-spinner" />}
                  label={props.text.listen.lyricsMatchPreviewLoading}
                />
              ) : preview.error ? (
                <ListenLyricsMatchStatus
                  icon={<CircleAlert className="h-4 w-4" />}
                  label={props.text.listen.lyricsMatchPreviewFailed}
                  tone="error"
                />
              ) : preview.lyrics ? (
                <ListenLyricsSurface
                  variant="player"
                  renderer={props.renderer}
                  focusStyle={props.focusStyle}
                  text={props.text}
                  lyrics={preview.lyrics}
                  currentTimeMs={props.currentTimeMs}
                  clockKey={`offset:${props.offsetMs ?? 0}`}
                  timelineRunning={props.timelineRunning}
                  playbackRate={props.playbackRate}
                  romanized={props.romanized}
                  pinyin={props.pinyin}
                />
              ) : (
                <ListenLyricsMatchStatus
                  icon={<Music2 className="h-4 w-4" />}
                  label={props.text.listen.lyricsMatchPreviewHint}
                />
              )}
            </div>
          </section>
        </div>

        {actionError ? (
          <div
            className="listen-lyrics-match-dialog__action-error"
            role="alert"
          >
            <CircleAlert className="h-3.5 w-3.5" />
            <span>{props.text.listen.lyricsMatchActionFailed}</span>
          </div>
        ) : null}

        <DialogFooter className="listen-lyrics-match-dialog__footer">
          {props.hasManualOverride ? (
            <Button
              type="button"
              variant="ghost"
              size="compact"
              className="mr-auto"
              disabled={busy}
              onClick={() => void handleRestore()}
            >
              {action === "restore" ? (
                <Loader2 className="h-3.5 w-3.5 listen-loading-spinner" />
              ) : (
                <Sparkles className="h-3.5 w-3.5" />
              )}
              {props.text.listen.lyricsMatchRestoreAutomatic}
            </Button>
          ) : null}
          <Button
            type="button"
            variant="ghost"
            size="compact"
            disabled={busy}
            onClick={() => handleOpenChange(false)}
          >
            {props.text.actions.cancel}
          </Button>
          <Button
            type="button"
            size="compact"
            disabled={!previewReady || busy}
            onClick={() => void handleConfirm()}
          >
            {action === "confirm" ? (
              <Loader2 className="h-3.5 w-3.5 listen-loading-spinner" />
            ) : (
              <Check className="h-3.5 w-3.5" />
            )}
            {props.text.listen.lyricsMatchUseVersion}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function ListenLyricsSearchField(props: {
  label: string;
  value: string;
  required?: boolean;
  onChange: (value: string) => void;
}) {
  return (
    <label className="listen-lyrics-match-dialog__field">
      <span>{props.label}</span>
      <Input
        value={props.value}
        required={props.required}
        autoComplete="off"
        onChange={(event) => props.onChange(event.target.value)}
      />
    </label>
  );
}

export function ListenLyricsCandidateCard(props: {
  candidate: ListenLyricsCandidate;
  selected: boolean;
  text: LyricsText;
  onSelect: () => void;
}) {
  const candidate = props.candidate;
  const duration = formatListenLyricsCandidateDuration(
    candidate.durationSeconds,
  );
  const capabilities = resolveListenLyricsCandidateCapabilities(
    candidate,
    props.text,
  );
  return (
    <button
      type="button"
      className="listen-lyrics-candidate-card"
      data-accepted={candidate.accepted}
      data-selected={props.selected}
      aria-pressed={props.selected}
      onClick={props.onSelect}
    >
      <span className="listen-lyrics-candidate-card__header">
        <span className="listen-lyrics-candidate-card__identity">
          <strong>{candidate.title || props.text.listen.lyricsMatchUntitled}</strong>
          <span>{candidate.artist || candidate.providerId}</span>
        </span>
        <span
          className="listen-lyrics-candidate-card__confidence"
          data-accepted={candidate.accepted}
          aria-label={`${props.text.listen.lyricsMatchConfidence} ${candidate.confidence}%`}
        >
          {candidate.accepted ? (
            <BadgeCheck className="h-3.5 w-3.5" />
          ) : (
            <CircleAlert className="h-3.5 w-3.5" />
          )}
          {candidate.confidence}%
        </span>
      </span>

      <span className="listen-lyrics-candidate-card__metadata">
        {candidate.album ? <span>{candidate.album}</span> : null}
        {duration ? (
          <span>
            <Clock3 className="h-3 w-3" />
            {duration}
          </span>
        ) : null}
        {typeof candidate.durationDiff === "number" ? (
          <span title={props.text.listen.lyricsMatchDurationDifference}>
            Δ {candidate.durationDiff.toFixed(1)} s
          </span>
        ) : null}
        <span>{candidate.attribution ?? candidate.providerId}</span>
      </span>

      <span className="listen-lyrics-candidate-card__capabilities">
        {capabilities.map((capability) => (
          <span key={capability.label} data-tone={capability.tone}>
            {capability.icon}
            {capability.label}
          </span>
        ))}
      </span>

      <span className="listen-lyrics-candidate-card__scores">
        <ListenLyricsCandidateScore
          label={props.text.listen.lyricsMatchTitleScore}
          value={candidate.titleScore}
        />
        <ListenLyricsCandidateScore
          label={props.text.listen.lyricsMatchArtistScore}
          value={candidate.artistScore}
        />
        <ListenLyricsCandidateScore
          label={props.text.listen.lyricsMatchAlbumScore}
          value={candidate.albumScore}
        />
        <ListenLyricsCandidateScore
          label={props.text.listen.lyricsMatchDurationScore}
          value={candidate.durationScore}
        />
      </span>

      {!candidate.accepted ? (
        <span className="listen-lyrics-candidate-card__rejection">
          <CircleAlert className="h-3 w-3" />
          {candidate.rejection
            ? resolveListenLyricsRejection(candidate.rejection, props.text)
            : props.text.listen.lyricsMatchRejected}
        </span>
      ) : (
        <span className="listen-lyrics-candidate-card__accepted">
          <Check className="h-3 w-3" />
          {props.text.listen.lyricsMatchAccepted}
        </span>
      )}
    </button>
  );
}

function ListenLyricsCandidateScore(props: { label: string; value: number }) {
  const value = Math.max(0, Math.min(100, Math.round(props.value)));
  return (
    <span className="listen-lyrics-candidate-score">
      <span>
        <span>{props.label}</span>
        <strong>{value}</strong>
      </span>
      <span
        role="progressbar"
        aria-label={props.label}
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={value}
      >
        <span style={{ width: `${value}%` }} />
      </span>
    </span>
  );
}

function ListenLyricsMatchStatus(props: {
  icon: React.ReactNode;
  label: string;
  tone?: "error";
}) {
  return (
    <div
      className="listen-lyrics-match-status"
      data-tone={props.tone}
      role={props.tone === "error" ? "alert" : "status"}
      aria-live="polite"
    >
      <span>{props.icon}</span>
      <div>{props.label}</div>
    </div>
  );
}

function resolveListenLyricsCandidateCapabilities(
  candidate: ListenLyricsCandidate,
  text: LyricsText,
) {
  const capabilities: Array<{
    label: string;
    icon: React.ReactNode;
    tone: "accent" | "neutral" | "warning";
  }> = [];
  if (candidate.instrumental) {
    capabilities.push({
      label: text.listen.lyricsMatchInstrumental,
      icon: <Waves className="h-3 w-3" />,
      tone: "warning",
    });
  }
  if (candidate.hasSynced) {
    capabilities.push({
      label: resolveListenLyricsTimingQuality(candidate.timingQuality, text),
      icon: <Waves className="h-3 w-3" />,
      tone: "accent",
    });
  }
  if (candidate.hasPlain) {
    capabilities.push({
      label: text.listen.lyricsMatchPlain,
      icon: <FileText className="h-3 w-3" />,
      tone: "neutral",
    });
  }
  if (!candidate.hasSynced && !candidate.hasPlain && !candidate.instrumental) {
    capabilities.push({
      label: resolveListenLyricsTimingQuality(candidate.timingQuality, text),
      icon: <Languages className="h-3 w-3" />,
      tone: "neutral",
    });
  }
  return capabilities;
}

function resolveListenLyricsTimingQuality(
  quality: ListenLyricTimingQuality | undefined,
  text: LyricsText,
) {
  switch (quality) {
    case "syllable":
      return text.listen.lyricsMatchSyllableSynced;
    case "word":
      return text.listen.lyricsMatchWordSynced;
    case "estimated":
      return text.listen.lyricsMatchEstimatedSynced;
    case "line":
      return text.listen.lyricsMatchLineSynced;
    default:
      return text.listen.lyricsMatchPlain;
  }
}

function resolveListenLyricsRejection(reason: string, text: LyricsText) {
  switch (reason.trim().toLowerCase()) {
    case "missing title":
      return text.listen.lyricsMatchRejectMissingTitle;
    case "incompatible title version":
      return text.listen.lyricsMatchRejectVersion;
    case "title mismatch":
      return text.listen.lyricsMatchRejectTitle;
    case "artist mismatch":
      return text.listen.lyricsMatchRejectArtist;
    case "duration mismatch":
      return text.listen.lyricsMatchRejectDuration;
    case "no identity metadata":
      return text.listen.lyricsMatchRejectIdentity;
    case "missing corroborating metadata":
      return text.listen.lyricsMatchRejectMetadata;
    case "low identity confidence":
      return text.listen.lyricsMatchRejectConfidence;
    case "instrumental record":
      return text.listen.lyricsMatchRejectInstrumental;
    case "plain lyrics unavailable":
      return text.listen.lyricsMatchRejectPlainUnavailable;
    case "lyrics unavailable":
      return text.listen.lyricsMatchRejectLyricsUnavailable;
    default:
      return reason;
  }
}
