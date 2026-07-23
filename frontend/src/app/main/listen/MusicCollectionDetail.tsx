import {
  ArrowLeft,
  BookmarkCheck,
  BookmarkPlus,
  ChevronDown,
  ChevronUp,
  Download,
  ExternalLink,
  ListEnd,
  ListStart,
  Loader2,
  MoreHorizontal,
  MoreVertical,
  Music2,
  Play,
  Shuffle,
} from "lucide-react";
import * as React from "react";

import { cn } from "@/lib/utils";
import { Button } from "@/shared/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/shared/ui/dropdown-menu";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/shared/ui/tooltip";

import {
  buildListenImageCandidates,
  buildListenTrackThumbnailCandidates,
} from "@/app/main/listen/storage";
import type {
  ListenOnlineItem,
  ListenPageProps,
  ListenPlaylistItem,
} from "@/app/main/listen/types";

type CollectionDetailText = ListenPageProps["text"];

export type ListenMusicCollectionDetailProps = {
  httpBaseURL: string;
  collection: ListenPlaylistItem | null;
  title: string;
  typeLabel: string;
  author: string;
  description: string;
  headerMetadata: string;
  showFooter: boolean;
  isAlbum: boolean;
  items: ListenOnlineItem[];
  allItems: ListenOnlineItem[];
  selectedId: string;
  actionDisabled: boolean;
  playbackBusy: boolean;
  libraryBusy: boolean;
  saved: boolean;
  text: CollectionDetailText;
  onBack: () => void;
  onOpenAuthor?: () => void;
  onPlay: () => void;
  onShuffle: () => void;
  onToggleLibrary: () => void;
  onPlayNext: () => void;
  onAddToQueue: () => void;
  onSelectTrack: (item: ListenOnlineItem) => void;
  onDownloadTrack: (item: ListenOnlineItem) => void;
  onOpenTrack: (item: ListenOnlineItem) => void;
};

export function ListenMusicCollectionDetail(
  props: ListenMusicCollectionDetailProps,
) {
  const saveLabel = props.saved
    ? props.text.listen.removePlaylist
    : props.text.listen.addToLibrary;
  const trackNumberByID = React.useMemo(
    () =>
      new Map(
        props.allItems.map((item, index) => [item.id, index + 1] as const),
      ),
    [props.allItems],
  );
  const collectionIdentity =
    props.collection?.playlistId.trim() ||
    `${props.typeLabel.trim()}:${props.title.trim()}`;

  return (
    <article
      className="listen-collection-detail"
      data-collection-kind={props.isAlbum ? "album" : "playlist"}
    >
      <div className="listen-collection-detail__toolbar wails-drag">
        <Button
          type="button"
          variant="ghost"
          size="compactIcon"
          className="listen-collection-detail__back wails-no-drag"
          aria-label={props.text.actions.back}
          onClick={props.onBack}
        >
          <ArrowLeft aria-hidden="true" />
        </Button>
      </div>

      <header className="listen-collection-detail__header">
        <ListenCollectionArtwork
          httpBaseURL={props.httpBaseURL}
          title={props.title}
          thumbnailUrl={props.collection?.thumbnailUrl}
        />
        <div className="listen-collection-detail__identity">
          <p className="listen-collection-detail__type">{props.typeLabel}</p>
          <h1 className="listen-collection-detail__title">{props.title}</h1>
          {props.author ? (
            props.onOpenAuthor ? (
              <Button
                type="button"
                variant="link"
                size="compact"
                className="listen-collection-detail__author"
                onClick={props.onOpenAuthor}
              >
                {props.author}
              </Button>
            ) : (
              <p className="listen-collection-detail__author">{props.author}</p>
            )
          ) : null}
          {props.headerMetadata ? (
            <p className="listen-collection-detail__metadata">
              {props.headerMetadata}
            </p>
          ) : null}
          <TooltipProvider delayDuration={0}>
            <div className="listen-collection-detail__actions">
              <ListenCollectionIconAction
                label={props.text.listen.shuffleAll}
                disabled={props.actionDisabled}
                onClick={props.onShuffle}
              >
                <Shuffle aria-hidden="true" />
              </ListenCollectionIconAction>
              <Button
                type="button"
                shape="capsule"
                className="listen-collection-detail__play"
                aria-label={props.text.actions.play}
                aria-busy={props.playbackBusy || undefined}
                disabled={props.actionDisabled}
                onClick={props.onPlay}
              >
                {props.playbackBusy ? (
                  <Loader2 aria-hidden="true" className="listen-loading-spinner" />
                ) : (
                  <Play aria-hidden="true" />
                )}
                <span>{props.text.actions.play}</span>
              </Button>
              <ListenCollectionIconAction
                label={saveLabel}
                active={props.saved}
                busy={props.libraryBusy}
                disabled={!props.collection || props.libraryBusy}
                onClick={props.onToggleLibrary}
              >
                {props.libraryBusy ? (
                  <Loader2 aria-hidden="true" className="listen-loading-spinner" />
                ) : props.saved ? (
                  <BookmarkCheck aria-hidden="true" />
                ) : (
                  <BookmarkPlus aria-hidden="true" />
                )}
              </ListenCollectionIconAction>
              <DropdownMenu>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <DropdownMenuTrigger asChild>
                      <Button
                        type="button"
                        variant="glass"
                        size="icon"
                        shape="circle"
                        className="listen-collection-detail__icon-action"
                        aria-label={props.text.listen.more}
                        disabled={props.actionDisabled}
                      >
                        <MoreHorizontal aria-hidden="true" />
                      </Button>
                    </DropdownMenuTrigger>
                  </TooltipTrigger>
                  <TooltipContent side="bottom">
                    {props.text.listen.more}
                  </TooltipContent>
                </Tooltip>
                <DropdownMenuContent
                  align="center"
                  side="bottom"
                  className="app-menu-content-fit"
                >
                  <DropdownMenuItem onSelect={props.onPlayNext}>
                    <ListStart aria-hidden="true" className="h-4 w-4" />
                    {props.text.listen.playNext}
                  </DropdownMenuItem>
                  <DropdownMenuItem onSelect={props.onAddToQueue}>
                    <ListEnd aria-hidden="true" className="h-4 w-4" />
                    {props.text.listen.addToQueue}
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            </div>
          </TooltipProvider>
        </div>
      </header>

      <ListenCollectionTrackList
        httpBaseURL={props.httpBaseURL}
        isAlbum={props.isAlbum}
        actionDisabled={props.actionDisabled}
        items={props.items}
        selectedId={props.selectedId}
        text={props.text}
        trackNumberByID={trackNumberByID}
        onDownloadTrack={props.onDownloadTrack}
        onOpenTrack={props.onOpenTrack}
        onSelectTrack={props.onSelectTrack}
      />

      {props.showFooter && props.description ? (
        <footer className="listen-collection-detail__footer">
          <ListenCollectionDescription
            key={collectionIdentity}
            description={props.description}
            expandLabel={props.text.listen.collectionDescriptionMore}
            collapseLabel={props.text.listen.collectionDescriptionLess}
          />
        </footer>
      ) : null}
    </article>
  );
}

function ListenCollectionDescription(props: {
  description: string;
  expandLabel: string;
  collapseLabel: string;
}) {
  const descriptionID = React.useId();
  const descriptionRef = React.useRef<HTMLParagraphElement>(null);
  const [expanded, setExpanded] = React.useState(false);
  const [canExpand, setCanExpand] = React.useState(false);

  React.useLayoutEffect(() => {
    setExpanded(false);
    setCanExpand(false);
  }, [props.description]);

  React.useLayoutEffect(() => {
    const description = descriptionRef.current;
    if (!description || expanded) {
      return;
    }

    const measureOverflow = () => {
      setCanExpand(description.scrollHeight > description.clientHeight + 1);
    };

    measureOverflow();

    if (typeof ResizeObserver === "undefined") {
      window.addEventListener("resize", measureOverflow);
      return () => window.removeEventListener("resize", measureOverflow);
    }

    const observer = new ResizeObserver(measureOverflow);
    observer.observe(description);
    return () => observer.disconnect();
  }, [expanded, props.description]);

  return (
    <div className="listen-collection-detail__description-wrap">
      <p
        ref={descriptionRef}
        id={descriptionID}
        className="listen-collection-detail__description"
        data-expanded={expanded ? "true" : "false"}
      >
        {props.description}
      </p>
      {canExpand ? (
        <Button
          type="button"
          variant="ghost"
          size="compact"
          className="listen-collection-detail__description-toggle"
          aria-controls={descriptionID}
          aria-expanded={expanded}
          onClick={() => setExpanded((current) => !current)}
        >
          {expanded ? (
            <ChevronUp aria-hidden="true" />
          ) : (
            <ChevronDown aria-hidden="true" />
          )}
          {expanded ? props.collapseLabel : props.expandLabel}
        </Button>
      ) : null}
    </div>
  );
}

function ListenCollectionIconAction(props: {
  label: string;
  active?: boolean;
  busy?: boolean;
  disabled: boolean;
  children: React.ReactNode;
  onClick: () => void;
}) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          type="button"
          variant="glass"
          size="icon"
          shape="circle"
          className="listen-collection-detail__icon-action"
          aria-busy={props.busy || undefined}
          aria-label={props.label}
          aria-pressed={props.active}
          aria-disabled={props.disabled}
          data-active={props.active ? "true" : "false"}
          data-disabled={props.disabled ? "true" : "false"}
          onClick={props.disabled ? undefined : props.onClick}
        >
          {props.children}
        </Button>
      </TooltipTrigger>
      <TooltipContent side="bottom">{props.label}</TooltipContent>
    </Tooltip>
  );
}

function ListenCollectionTrackList(props: {
  httpBaseURL: string;
  isAlbum: boolean;
  actionDisabled: boolean;
  items: ListenOnlineItem[];
  selectedId: string;
  text: CollectionDetailText;
  trackNumberByID: Map<string, number>;
  onSelectTrack: (item: ListenOnlineItem) => void;
  onDownloadTrack: (item: ListenOnlineItem) => void;
  onOpenTrack: (item: ListenOnlineItem) => void;
}) {
  if (props.items.length === 0) {
    return null;
  }

  return (
    <section
      className="listen-collection-tracks"
      aria-label={props.text.workspace.songs}
      role="table"
    >
      {!props.isAlbum ? (
        <div
          className="listen-collection-tracks__columns"
          role="row"
          data-collection-track-columns="playlist"
        >
          <span role="columnheader">
            {props.text.listen.playlistColumnSong}
          </span>
          <span role="columnheader">
            {props.text.listen.playlistColumnArtist}
          </span>
          <span role="columnheader">
            {props.text.listen.playlistColumnTime}
          </span>
          <span aria-hidden="true" />
        </div>
      ) : null}
      <div role="rowgroup" className="listen-collection-tracks__body">
        {props.items.map((item, index) => {
          const trackNumber = props.trackNumberByID.get(item.id) ?? index + 1;
          return (
            <ListenCollectionTrackRow
              key={item.id}
              httpBaseURL={props.httpBaseURL}
              isAlbum={props.isAlbum}
              disabled={props.actionDisabled}
              item={item}
              selected={item.id === props.selectedId}
              text={props.text}
              trackNumber={trackNumber}
              onDownload={() => props.onDownloadTrack(item)}
              onOpen={() => props.onOpenTrack(item)}
              onSelect={() => props.onSelectTrack(item)}
            />
          );
        })}
      </div>
    </section>
  );
}

function ListenCollectionTrackRow(props: {
  httpBaseURL: string;
  isAlbum: boolean;
  disabled: boolean;
  item: ListenOnlineItem;
  selected: boolean;
  text: CollectionDetailText;
  trackNumber: number;
  onSelect: () => void;
  onDownload: () => void;
  onOpen: () => void;
}) {
  const artist = resolveCollectionTrackArtist(props.item);
  const album = resolveCollectionTrackAlbum(props.item, artist);

  return (
    <div
      role="row"
      className={cn(
        "listen-collection-track-row",
        props.selected && "is-selected",
      )}
      data-track-layout={props.isAlbum ? "album" : "playlist"}
      data-disabled={props.disabled ? "true" : "false"}
      data-selected={props.selected ? "true" : "false"}
    >
      <button
        type="button"
        className="listen-collection-track-row__hit-area"
        aria-label={`${props.text.actions.play}: ${props.item.title}`}
        disabled={props.disabled}
        onClick={props.onSelect}
      />
      {props.isAlbum ? (
        <>
          <span
            role="cell"
            className="listen-collection-track-row__number"
          >
            {props.trackNumber}
          </span>
          <span role="cell" className="listen-collection-track-row__title">
            {props.item.title}
          </span>
        </>
      ) : (
        <>
          <span role="cell" className="listen-collection-track-row__song">
            <ListenCollectionTrackArtwork
              httpBaseURL={props.httpBaseURL}
              item={props.item}
            />
            <span className="listen-collection-track-row__song-text">
              <span className="listen-collection-track-row__title">
                {props.item.title}
              </span>
              {album ? (
                <span className="listen-collection-track-row__album">
                  {album}
                </span>
              ) : null}
            </span>
          </span>
          <span role="cell" className="listen-collection-track-row__artist">
            {artist}
          </span>
        </>
      )}
      <span role="cell" className="listen-collection-track-row__time">
        {props.item.durationLabel}
      </span>
      <div role="cell" className="listen-collection-track-row__menu">
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              type="button"
              variant="ghost"
              size="compactIcon"
              aria-label={props.text.listen.more}
              className="listen-collection-track-row__more"
            >
              <MoreVertical aria-hidden="true" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent
            align="end"
            side="bottom"
            className="app-menu-content-fit"
          >
            <DropdownMenuItem
              disabled={props.disabled}
              onSelect={props.onSelect}
            >
              <Play aria-hidden="true" className="h-4 w-4" />
              {props.text.actions.play}
            </DropdownMenuItem>
            <DropdownMenuItem onSelect={props.onDownload}>
              <Download aria-hidden="true" className="h-4 w-4" />
              {props.text.actions.download}
            </DropdownMenuItem>
            <DropdownMenuItem onSelect={props.onOpen}>
              <ExternalLink aria-hidden="true" className="h-4 w-4" />
              {props.text.listen.openPage}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </div>
  );
}

function ListenCollectionArtwork(props: {
  httpBaseURL: string;
  title: string;
  thumbnailUrl?: string;
}) {
  const candidates = React.useMemo(
    () =>
      buildListenImageCandidates(
        props.httpBaseURL,
        props.thumbnailUrl?.trim() ?? "",
      ),
    [props.httpBaseURL, props.thumbnailUrl],
  );
  const candidateKey = candidates.join("\n");
  const [candidateIndex, setCandidateIndex] = React.useState(0);
  const [imageUnavailable, setImageUnavailable] = React.useState(false);
  const source = imageUnavailable ? "" : candidates[candidateIndex] ?? "";

  React.useEffect(() => {
    setCandidateIndex(0);
    setImageUnavailable(false);
  }, [candidateKey]);

  return (
    <div className="listen-collection-detail__artwork" aria-label={props.title}>
      {source ? (
        <img
          src={source}
          alt=""
          loading="eager"
          onError={() => {
            if (candidateIndex < candidates.length - 1) {
              setCandidateIndex((current) => current + 1);
              return;
            }
            setImageUnavailable(true);
          }}
        />
      ) : (
        <Music2 aria-hidden="true" />
      )}
    </div>
  );
}

function ListenCollectionTrackArtwork(props: {
  httpBaseURL: string;
  item: ListenOnlineItem;
}) {
  const candidates = React.useMemo(
    () => buildListenTrackThumbnailCandidates(props.httpBaseURL, props.item),
    [props.httpBaseURL, props.item],
  );
  const candidateKey = candidates.join("\n");
  const [candidateIndex, setCandidateIndex] = React.useState(0);
  const [imageUnavailable, setImageUnavailable] = React.useState(false);
  const source = imageUnavailable ? "" : candidates[candidateIndex] ?? "";

  React.useEffect(() => {
    setCandidateIndex(0);
    setImageUnavailable(false);
  }, [candidateKey]);

  return (
    <span className="listen-collection-track-row__artwork" aria-hidden="true">
      {source ? (
        <img
          src={source}
          alt=""
          loading="lazy"
          onError={() => {
            if (candidateIndex < candidates.length - 1) {
              setCandidateIndex((current) => current + 1);
              return;
            }
            setImageUnavailable(true);
          }}
        />
      ) : (
        <Music2 />
      )}
    </span>
  );
}

function resolveCollectionTrackArtist(item: ListenOnlineItem) {
  const artists = (item.artists ?? [])
    .map((artist) => artist.name.trim())
    .filter(Boolean);
  return Array.from(new Set(artists)).join(", ") || item.channel.trim();
}

function resolveCollectionTrackAlbum(item: ListenOnlineItem, artist: string) {
  const album = item.description.trim();
  return !album || album === artist || album === item.channel.trim() ? "" : album;
}
