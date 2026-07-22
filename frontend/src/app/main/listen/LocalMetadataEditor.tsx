import { Check, CircleAlert, FileAudio, Loader2, Sparkles } from "lucide-react";
import * as React from "react";

import {
  applyListenLocalMetadataCandidate,
  isListenLocalMetadataCommittedIndexStale,
  listenLocalMetadataDraft,
  parseListenLocalMetadataNumber,
  searchListenLocalMetadataCandidate,
  updateListenLocalTrackMetadata,
  type ListenLocalMetadataDraft,
} from "@/app/main/listen/local-metadata";
import type {
  ListenLocalItem,
  ListenLyricsCandidate,
  ListenPageProps,
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

export type ListenLocalMetadataEditorProps = {
  open: boolean;
  track: ListenLocalItem | null;
  httpBaseURL: string;
  text: ListenPageProps["text"];
  onOpenChange: (open: boolean) => void;
  onSaved: (track: ListenLocalItem) => void | Promise<void>;
};

export function ListenLocalMetadataEditor(
  props: ListenLocalMetadataEditorProps,
) {
  const emptyDraft = React.useMemo<ListenLocalMetadataDraft>(
    () => ({
      title: "",
      author: "",
      album: "",
      albumArtist: "",
      genre: "",
      trackNumber: 0,
      discNumber: 0,
      year: 0,
    }),
    [],
  );
  const [draft, setDraft] =
    React.useState<ListenLocalMetadataDraft>(emptyDraft);
  const [suggestion, setSuggestion] =
    React.useState<ListenLyricsCandidate | null>(null);
  const [matching, setMatching] = React.useState(false);
  const [saving, setSaving] = React.useState(false);
  const [committedIndexStale, setCommittedIndexStale] = React.useState(false);
  const [message, setMessage] = React.useState<
    | ""
    | "no-suggestion"
    | "applied"
    | "save-failed"
    | "match-failed"
    | "committed-index-stale"
  >("");
  const [errorDetail, setErrorDetail] = React.useState("");
  const requestRef = React.useRef(0);
  const trackIdentity = props.track
    ? [
        props.track.id,
        props.track.title,
        props.track.author,
        props.track.album,
        props.track.albumArtist,
        props.track.genre,
        props.track.trackNumber,
        props.track.discNumber,
        props.track.year,
      ].join("\u0000")
    : "";

  React.useEffect(() => {
    requestRef.current += 1;
    setDraft(props.track ? listenLocalMetadataDraft(props.track) : emptyDraft);
    setSuggestion(null);
    setMatching(false);
    setSaving(false);
    setCommittedIndexStale(false);
    setMessage("");
    setErrorDetail("");
  }, [emptyDraft, props.open, trackIdentity]);

  const updateText = React.useCallback(
    (
      field: keyof Pick<
        ListenLocalMetadataDraft,
        "title" | "author" | "album" | "albumArtist" | "genre"
      >,
      value: string,
    ) => {
      setDraft((current) => ({ ...current, [field]: value }));
      setSuggestion(null);
      setMessage("");
      setErrorDetail("");
    },
    [],
  );

  const updateNumber = React.useCallback(
    (field: "trackNumber" | "discNumber" | "year", value: string) => {
      setDraft((current) => ({
        ...current,
        [field]: parseListenLocalMetadataNumber(value),
      }));
      setMessage("");
      setErrorDetail("");
    },
    [],
  );

  const searchSuggestion = async () => {
    if (!props.track || matching || saving || committedIndexStale) {
      return;
    }
    const request = ++requestRef.current;
    setMatching(true);
    setSuggestion(null);
    setMessage("");
    setErrorDetail("");
    try {
      const candidate = await searchListenLocalMetadataCandidate({
        track: props.track,
        draft,
        language: props.text.locale,
      });
      if (request !== requestRef.current) {
        return;
      }
      setSuggestion(candidate);
      setMessage(candidate ? "" : "no-suggestion");
    } catch {
      if (request === requestRef.current) {
        setMessage("match-failed");
      }
    } finally {
      if (request === requestRef.current) {
        setMatching(false);
      }
    }
  };

  const applySuggestion = () => {
    if (!suggestion || committedIndexStale) {
      return;
    }
    setDraft((current) =>
      applyListenLocalMetadataCandidate(current, suggestion),
    );
    setSuggestion(null);
    setMessage("applied");
  };

  const save = async () => {
    const targetTrack = props.track;
    if (
      !targetTrack ||
      !draft.title.trim() ||
      matching ||
      saving ||
      committedIndexStale
    ) {
      return;
    }
    const request = ++requestRef.current;
    setSaving(true);
    setMessage("");
    setErrorDetail("");
    try {
      const updated = await updateListenLocalTrackMetadata({
        httpBaseURL: props.httpBaseURL,
        track: targetTrack,
        draft,
      });
      if (request !== requestRef.current) {
        return;
      }
      await props.onSaved(updated);
      props.onOpenChange(false);
    } catch (error) {
      if (request === requestRef.current) {
        if (isListenLocalMetadataCommittedIndexStale(error)) {
          setCommittedIndexStale(true);
          setSaving(false);
          setErrorDetail("");
          setMessage("committed-index-stale");
          try {
            await props.onSaved(targetTrack);
          } catch {
            // The file is already committed. Keep this terminal state even if
            // the best-effort index refresh also fails.
          }
        } else {
          setErrorDetail(error instanceof Error ? error.message.trim() : "");
          setMessage("save-failed");
        }
      }
    } finally {
      if (request === requestRef.current) {
        setSaving(false);
      }
    }
  };

  const track = props.track;
  const disabled = !track?.metadataWritable || saving || committedIndexStale;
  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className="max-w-xl">
        <DialogHeader>
          <DialogTitle>{props.text.listen.localMetadataEdit}</DialogTitle>
          <DialogDescription>
            {props.text.listen.localMetadataEditDescription}
          </DialogDescription>
        </DialogHeader>

        {track ? (
          <div className="listen-metadata-file-summary flex min-w-0 items-center gap-2 px-3 py-2">
            <FileAudio className="h-4 w-4 shrink-0" />
            <span className="min-w-0 flex-1 truncate">{track.path}</span>
            <span className="listen-metadata-file-format shrink-0">
              {track.format || track.audioCodec}
            </span>
          </div>
        ) : null}

        {!track?.metadataWritable ? (
          <MetadataMessage tone="error">
            {props.text.listen.localMetadataUnsupported}
          </MetadataMessage>
        ) : null}

        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <MetadataField
            className="sm:col-span-2"
            label={props.text.listen.localMetadataTitle}
          >
            <Input
              autoFocus
              disabled={disabled}
              maxLength={512}
              onChange={(event) => updateText("title", event.target.value)}
              value={draft.title}
            />
          </MetadataField>
          <MetadataField label={props.text.listen.localMetadataArtist}>
            <Input
              disabled={disabled}
              maxLength={512}
              onChange={(event) => updateText("author", event.target.value)}
              value={draft.author}
            />
          </MetadataField>
          <MetadataField label={props.text.listen.localMetadataAlbum}>
            <Input
              disabled={disabled}
              maxLength={512}
              onChange={(event) => updateText("album", event.target.value)}
              value={draft.album}
            />
          </MetadataField>
          <MetadataField label={props.text.listen.localMetadataAlbumArtist}>
            <Input
              disabled={disabled}
              maxLength={512}
              onChange={(event) =>
                updateText("albumArtist", event.target.value)
              }
              value={draft.albumArtist}
            />
          </MetadataField>
          <MetadataField label={props.text.listen.localMetadataGenre}>
            <Input
              disabled={disabled}
              maxLength={512}
              onChange={(event) => updateText("genre", event.target.value)}
              value={draft.genre}
            />
          </MetadataField>
          <div className="grid grid-cols-3 gap-2 sm:col-span-2">
            <MetadataField label={props.text.listen.localMetadataTrackNumber}>
              <Input
                disabled={disabled}
                inputMode="numeric"
                min={0}
                onChange={(event) =>
                  updateNumber("trackNumber", event.target.value)
                }
                type="number"
                value={draft.trackNumber || ""}
              />
            </MetadataField>
            <MetadataField label={props.text.listen.localMetadataDiscNumber}>
              <Input
                disabled={disabled}
                inputMode="numeric"
                min={0}
                onChange={(event) =>
                  updateNumber("discNumber", event.target.value)
                }
                type="number"
                value={draft.discNumber || ""}
              />
            </MetadataField>
            <MetadataField label={props.text.listen.localMetadataYear}>
              <Input
                disabled={disabled}
                inputMode="numeric"
                min={0}
                onChange={(event) => updateNumber("year", event.target.value)}
                type="number"
                value={draft.year || ""}
              />
            </MetadataField>
          </div>
        </div>

        {suggestion ? (
          <div className="listen-metadata-suggestion p-3">
            <div className="flex items-start justify-between gap-3">
              <div className="min-w-0">
                <div className="listen-metadata-suggestion__title">
                  {props.text.listen.localMetadataSuggestion}
                </div>
                <div className="listen-metadata-suggestion__summary mt-1 truncate">
                  {[suggestion.title, suggestion.artist, suggestion.album]
                    .filter(Boolean)
                    .join(" · ")}
                </div>
                <div className="listen-metadata-suggestion__attribution mt-1">
                  {suggestion.attribution || suggestion.providerId} ·{" "}
                  {suggestion.confidence}%
                </div>
              </div>
              <Button
                disabled={saving || committedIndexStale}
                onClick={applySuggestion}
                size="compact"
                type="button"
                variant="outline"
              >
                <Check className="h-3.5 w-3.5" />
                {props.text.listen.localMetadataApplySuggestion}
              </Button>
            </div>
          </div>
        ) : null}

        {message ? (
          <MetadataMessage tone={message === "applied" ? "success" : "error"}>
            {message === "applied"
              ? props.text.listen.localMetadataSuggestionApplied
              : message === "no-suggestion"
                ? props.text.listen.localMetadataNoSuggestion
                : message === "match-failed"
                  ? props.text.listen.localMetadataAutoFillFailed
                  : message === "committed-index-stale"
                    ? props.text.listen.localMetadataCommittedIndexStale
                    : errorDetail || props.text.listen.localMetadataSaveFailed}
          </MetadataMessage>
        ) : null}

        <DialogFooter className="gap-2 sm:justify-between">
          <Button
            disabled={disabled || matching || !draft.title.trim()}
            onClick={() => void searchSuggestion()}
            type="button"
            variant="outline"
          >
            {matching ? (
              <Loader2 className="h-4 w-4 listen-loading-spinner" />
            ) : (
              <Sparkles className="h-4 w-4" />
            )}
            {matching
              ? props.text.listen.localMetadataAutoFilling
              : props.text.listen.localMetadataAutoFill}
          </Button>
          <div className="flex gap-2">
            <Button
              disabled={saving}
              onClick={() => props.onOpenChange(false)}
              type="button"
              variant="outline"
            >
              {props.text.actions.cancel}
            </Button>
            <Button
              disabled={disabled || matching || !draft.title.trim()}
              onClick={() => void save()}
              type="button"
            >
              {saving ? <Loader2 className="h-4 w-4 listen-loading-spinner" /> : null}
              {props.text.actions.save}
            </Button>
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function MetadataField(props: {
  label: string;
  className?: string;
  children: React.ReactNode;
}) {
  return (
    <label className={props.className}>
      <span className="listen-metadata-field__label mb-1 block">
        {props.label}
      </span>
      {props.children}
    </label>
  );
}

function MetadataMessage(props: {
  tone: "success" | "error";
  children: React.ReactNode;
}) {
  return (
    <div
      aria-live={props.tone === "error" ? "assertive" : "polite"}
      className="listen-status-panel flex items-center gap-2 px-3 py-2"
      data-tone={props.tone}
      role={props.tone === "error" ? "alert" : "status"}
    >
      {props.tone === "error" ? (
        <CircleAlert className="h-4 w-4 shrink-0" />
      ) : (
        <Check className="h-4 w-4 shrink-0" />
      )}
      <span>{props.children}</span>
    </div>
  );
}
