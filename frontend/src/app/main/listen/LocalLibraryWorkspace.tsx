import {
  ArrowLeft,
  ChevronDown,
  ChevronUp,
  Disc3,
  ListMusic,
  Loader2,
  MoreHorizontal,
  Pencil,
  Play,
  Plus,
  RefreshCw,
  Trash2,
  UserRound,
  Wrench,
} from "lucide-react";
import * as React from "react";
import { createPortal } from "react-dom";

import { cn } from "@/lib/utils";
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
import {
  DropdownMenu,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/shared/ui/dropdown-menu";
import { TooltipProvider } from "@/shared/ui/tooltip";
import {
  WorkspacePrimaryHeaderAction,
  WorkspacePrimaryHeaderActionGroup,
  WorkspacePrimaryHeaderMenuContent,
} from "@/shared/ui/workspace-primary-header-action";
import { useAppWorkspaceStore } from "@/app/workspace/store";
import { APP_WORKSPACE_IDS } from "@/app/workspace/types";

import {
  isListenLocalPlaylistRevisionConflict,
  useListenLocalPlaylists,
  type ListenLocalPlaylistDetail,
} from "@/app/main/listen/local-playlists";
import { ListenLocalPlaylistDirectory } from "@/app/main/listen/LocalPlaylistDirectory";
import { ListenLocalMetadataEditor } from "@/app/main/listen/LocalMetadataEditor";
import { formatListenLocalProbeWarning,ListenLocalProbeWarning } from "@/app/main/listen/LocalProbeWarning";
import { resolveListenLocalPlayableQueue } from "@/app/main/listen/local-format";
import {
  createListenLocalPlaylistMutationGuard,
  type ListenLocalPlaylistMutationRequest,
} from "@/app/main/listen/local-playlist-mutation";
import {
  buildListenLocalAlbumGroups,
  buildListenLocalArtistGroups,
  filterListenLocalWorkspaceTracks,
  moveListenLocalPlaylistTrack,
  parseListenLocalWorkspaceRoute,
  sortListenLocalSongs,
  sortListenLocalTracksByRecent,
  type ListenLocalCollectionGroup,
} from "@/app/main/listen/local-workspace";
import type {
  ListenLocalItem,
  ListenPageProps,
} from "@/app/main/listen/types";
import { ListenLocalArtwork } from "@/app/main/listen/ui";

type PlaylistNameDialogMode = "create" | "rename" | null;

export interface ListenLocalLibraryWorkspaceProps {
  routeId: string;
  tracks: ListenLocalItem[];
  tracksLoading: boolean;
  tracksRefreshing: boolean;
  tracksClearingMissing: boolean;
  query: string;
  selectedId: string;
  httpBaseURL: string;
  text: ListenPageProps["text"];
  hideHeader?: boolean;
  headerActionsTarget?: HTMLElement | null;
  onPlayTrack: (track: ListenLocalItem, queue: ListenLocalItem[]) => void;
  onRefreshTracks: () => void;
  onRepairMissingTracks: () => void;
}

export function ListenLocalLibraryWorkspace(
  props: ListenLocalLibraryWorkspaceProps,
) {
  const route = React.useMemo(
    () => parseListenLocalWorkspaceRoute(props.routeId),
    [props.routeId],
  );
  const localPlaylists = useListenLocalPlaylists(props.httpBaseURL);
  const navigateWorkspace = useAppWorkspaceStore((state) => state.navigate);
  const [selectedGroupId, setSelectedGroupId] = React.useState("");
  const [transientPlaylistId, setTransientPlaylistId] = React.useState("");
  const [playlistDetail, setPlaylistDetail] =
    React.useState<ListenLocalPlaylistDetail | null>(null);
  const [playlistDetailLoading, setPlaylistDetailLoading] = React.useState(false);
  const [operationError, setOperationError] = React.useState("");
  const [operationSuccess, setOperationSuccess] = React.useState("");
  const [nameDialogMode, setNameDialogMode] =
    React.useState<PlaylistNameDialogMode>(null);
  const [playlistNameDraft, setPlaylistNameDraft] = React.useState("");
  const [deleteDialogOpen, setDeleteDialogOpen] = React.useState(false);
  const [metadataTrack, setMetadataTrack] =
    React.useState<ListenLocalItem | null>(null);
  const [addTracksOpen, setAddTracksOpen] = React.useState(false);
  const [addTracksQuery, setAddTracksQuery] = React.useState("");
  const [selectedAddTrackIds, setSelectedAddTrackIds] = React.useState<
    Set<string>
  >(() => new Set());

  const routePlaylistId = route?.kind === "playlist" ? route.playlistId : "";
  const activePlaylistId = routePlaylistId || transientPlaylistId;
  const mutationGuardRef = React.useRef<ReturnType<
    typeof createListenLocalPlaylistMutationGuard
  > | null>(null);
  if (!mutationGuardRef.current) {
    mutationGuardRef.current = createListenLocalPlaylistMutationGuard();
  }
  const mutationContextRef = React.useRef({
    routeId: props.routeId,
    playlistId: activePlaylistId,
  });
  mutationContextRef.current = {
    routeId: props.routeId,
    playlistId: activePlaylistId,
  };
  const playlistOperationFailedLabel =
    props.text.listen.localPlaylistOperationFailed;
  const playlistRevisionConflictLabel =
    props.text.listen.localPlaylistRevisionConflict;

  React.useEffect(() => {
    setSelectedGroupId("");
    setTransientPlaylistId("");
    setPlaylistDetail(null);
    setOperationError("");
    setOperationSuccess("");
    setNameDialogMode(null);
    setDeleteDialogOpen(false);
    setMetadataTrack(null);
    setAddTracksOpen(false);
    setAddTracksQuery("");
    setSelectedAddTrackIds(new Set());
  }, [props.routeId]);

  React.useEffect(() => {
    mutationGuardRef.current?.invalidate();
  }, [activePlaylistId, props.routeId]);

  React.useEffect(
    () => () => mutationGuardRef.current?.invalidate(),
    [],
  );

  React.useEffect(() => {
    if (!activePlaylistId) {
      setPlaylistDetail(null);
      setPlaylistDetailLoading(false);
      return;
    }
    let cancelled = false;
    setPlaylistDetailLoading(true);
    setOperationError("");
    void localPlaylists
      .get(activePlaylistId)
      .then((detail) => {
        if (!cancelled) {
          setPlaylistDetail(detail);
        }
      })
      .catch((error) => {
        if (!cancelled) {
          setPlaylistDetail(null);
          setOperationError(
            resolvePlaylistError(error, playlistOperationFailedLabel),
          );
        }
      })
      .finally(() => {
        if (!cancelled) {
          setPlaylistDetailLoading(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [activePlaylistId, localPlaylists.get, playlistOperationFailedLabel]);

  const filteredTracks = React.useMemo(
    () => filterListenLocalWorkspaceTracks(props.tracks, props.query),
    [props.query, props.tracks],
  );
  const artistGroups = React.useMemo(
    () =>
      route?.kind === "artists"
        ? buildListenLocalArtistGroups(
            filteredTracks,
            props.text.listen.localUnknownArtist,
          )
        : [],
    [filteredTracks, props.text.listen.localUnknownArtist, route?.kind],
  );
  const albumGroups = React.useMemo(
    () =>
      route?.kind === "albums"
        ? buildListenLocalAlbumGroups(
            filteredTracks,
            props.text.listen.localUnknownAlbum,
          )
        : [],
    [filteredTracks, props.text.listen.localUnknownAlbum, route?.kind],
  );
  const selectedGroup =
    (route?.kind === "artists" ? artistGroups : albumGroups).find(
      (group) => group.id === selectedGroupId,
    ) ?? null;
  const playlistTracks = React.useMemo(
    () => playlistDetail?.items.map((item) => item.track) ?? [],
    [playlistDetail],
  );
  const visiblePlaylistItems = React.useMemo(
    () => {
      const visibleTrackIDs = new Set(
        filterListenLocalWorkspaceTracks(playlistTracks, props.query).map(
          (track) => track.id,
        ),
      );
      return (playlistDetail?.items ?? []).filter((item) =>
        visibleTrackIDs.has(item.track.id),
      );
    },
    [playlistDetail, playlistTracks, props.query],
  );
  const visiblePlaylistTracks = React.useMemo(
    () => visiblePlaylistItems.map((item) => item.track),
    [visiblePlaylistItems],
  );
  const emptySearchLanding =
    route?.kind === "search" && props.query.trim().length === 0;
  const visibleRouteTracks = React.useMemo(() => {
    if (emptySearchLanding) {
      return [];
    }
    if (route?.kind === "home" || route?.kind === "recently-added") {
      return sortListenLocalTracksByRecent(filteredTracks);
    }
    if (route?.kind === "search" || route?.kind === "songs") {
      return sortListenLocalSongs(filteredTracks);
    }
    return [];
  }, [emptySearchLanding, filteredTracks, route?.kind]);
  const currentQueue = activePlaylistId
    ? playlistTracks
    : selectedGroup?.tracks ?? visibleRouteTracks;
  const playableCurrentQueue = resolveListenLocalPlayableQueue(currentQueue);
  const playTrack = (track: ListenLocalItem, queue: ListenLocalItem[]) => {
    if (!track.playbackSupported) {
      return;
    }
    props.onPlayTrack(track, resolveListenLocalPlayableQueue(queue));
  };
  const pageTitle = resolveLocalWorkspaceTitle({
    routeKind: route?.kind,
    playlistName: playlistDetail?.playlist.name,
    selectedGroup,
    text: props.text,
  });
  const count = activePlaylistId
    ? playlistTracks.length
    : selectedGroup
      ? selectedGroup.tracks.length
      : route?.kind === "artists" || route?.kind === "albums"
        ? filteredTracks.length
        : visibleRouteTracks.length;

  const runOperation = React.useCallback(
    async <T,>(
      request: ListenLocalPlaylistMutationRequest,
      operation: () => Promise<T>,
    ) => {
      setOperationError("");
      setOperationSuccess("");
      try {
        return await operation();
      } catch (error) {
        if (isListenLocalPlaylistRevisionConflict(error)) {
          const refreshed = request.playlistId
            ? await localPlaylists.get(request.playlistId).catch(() => null)
            : null;
          await localPlaylists.reload().catch(() => undefined);
          if (
            mutationGuardRef.current?.isCurrent(
              request,
              mutationContextRef.current,
            )
          ) {
            if (refreshed) {
              setPlaylistDetail(refreshed);
            }
            setOperationError(playlistRevisionConflictLabel);
          }
          return null;
        }
        if (
          mutationGuardRef.current?.isCurrent(
            request,
            mutationContextRef.current,
          )
        ) {
          setOperationError(
            resolvePlaylistError(error, playlistOperationFailedLabel),
          );
        }
        return null;
      }
    },
    [
      localPlaylists.get,
      localPlaylists.reload,
      playlistOperationFailedLabel,
      playlistRevisionConflictLabel,
    ],
  );

  const beginPlaylistMutation = React.useCallback(
    () => mutationGuardRef.current!.begin(mutationContextRef.current),
    [],
  );
  const mutationIsCurrent = React.useCallback(
    (request: ListenLocalPlaylistMutationRequest) =>
      mutationGuardRef.current?.isCurrent(
        request,
        mutationContextRef.current,
      ) === true,
    [],
  );

  const submitPlaylistName = async () => {
    const name = playlistNameDraft.trim();
    if (!name || localPlaylists.mutating) {
      return;
    }
    if (nameDialogMode === "create") {
      const request = beginPlaylistMutation();
      const created = await runOperation(request, () =>
        localPlaylists.create(name),
      );
      if (!created || !mutationIsCurrent(request)) {
        return;
      }
      setTransientPlaylistId(created.id);
      setPlaylistDetail({ playlist: created, items: [] });
      setOperationSuccess(
        `${props.text.listen.localNewPlaylist}: ${created.name}`,
      );
      setNameDialogMode(null);
      navigateWorkspace(
        { routeId: `playlist:${created.id}` },
        APP_WORKSPACE_IDS.music,
      );
      return;
    }
    if (nameDialogMode === "rename" && activePlaylistId && playlistDetail) {
      const playlistId = activePlaylistId;
      const expectedRevision = playlistDetail.playlist.revision;
      const request = beginPlaylistMutation();
      const renamed = await runOperation(request, () =>
        localPlaylists.rename(playlistId, name, expectedRevision),
      );
      if (!renamed || !mutationIsCurrent(request)) {
        return;
      }
      setPlaylistDetail((current) =>
        current?.playlist.id === playlistId
          ? { ...current, playlist: renamed }
          : current,
      );
      setOperationSuccess(
        `${props.text.listen.localRenamePlaylist}: ${renamed.name}`,
      );
      setNameDialogMode(null);
    }
  };

  const deletePlaylist = async () => {
    if (!activePlaylistId || !playlistDetail || localPlaylists.mutating) {
      return;
    }
    const playlistId = activePlaylistId;
    const expectedRevision = playlistDetail.playlist.revision;
    const request = beginPlaylistMutation();
    const deletedPlaylistName = playlistDetail?.playlist.name ?? "";
    const removed = await runOperation(request, async () => {
      await localPlaylists.remove(playlistId, expectedRevision);
      return true;
    });
    if (!removed || !mutationIsCurrent(request)) {
      return;
    }
    setDeleteDialogOpen(false);
    setPlaylistDetail(null);
    setTransientPlaylistId("");
    setOperationSuccess(
      [props.text.listen.localDeletePlaylist, deletedPlaylistName]
        .filter(Boolean)
        .join(": "),
    );
    navigateWorkspace(
      { routeId: "songs" },
      APP_WORKSPACE_IDS.music,
    );
  };

  const addSelectedTracks = async () => {
    if (!activePlaylistId || !playlistDetail || selectedAddTrackIds.size === 0) {
      return;
    }
    const playlistId = activePlaylistId;
    const expectedRevision = playlistDetail.playlist.revision;
    const request = beginPlaylistMutation();
    const selectedTrackCount = selectedAddTrackIds.size;
    const updated = await runOperation(request, () =>
      localPlaylists.addTracks(
        playlistId,
        [...selectedAddTrackIds],
        expectedRevision,
      ),
    );
    if (!updated || !mutationIsCurrent(request)) {
      return;
    }
    setPlaylistDetail(updated);
    setOperationSuccess(
      `${props.text.listen.localAddTracks}: ${formatTemplate(
        props.text.listen.localSongCount,
        { count: selectedTrackCount },
      )}`,
    );
    setSelectedAddTrackIds(new Set());
    setAddTracksOpen(false);
  };

  const removePlaylistTrack = async (itemId: string) => {
    if (!activePlaylistId || !playlistDetail) {
      return;
    }
    const playlistId = activePlaylistId;
    const expectedRevision = playlistDetail.playlist.revision;
    const request = beginPlaylistMutation();
    const removedTrackTitle =
      playlistDetail.items.find((item) => item.id === itemId)?.track.title ??
      "";
    const updated = await runOperation(request, () =>
      localPlaylists.removeTrack(playlistId, itemId, expectedRevision),
    );
    if (updated && mutationIsCurrent(request)) {
      setPlaylistDetail(updated);
      setOperationSuccess(
        [props.text.listen.localRemoveTrack, removedTrackTitle]
          .filter(Boolean)
          .join(": "),
      );
    }
  };

  const movePlaylistTrack = async (itemId: string, direction: -1 | 1) => {
    if (!activePlaylistId || !playlistDetail) {
      return;
    }
    const playlistId = activePlaylistId;
    const expectedRevision = playlistDetail.playlist.revision;
    const currentItemIds = playlistDetail.items.map((item) => item.id);
    const nextItemIds = moveListenLocalPlaylistTrack(
      currentItemIds,
      itemId,
      direction,
    );
    if (nextItemIds.every((id, index) => id === currentItemIds[index])) {
      return;
    }
    const request = beginPlaylistMutation();
    const updated = await runOperation(request, () =>
      localPlaylists.reorder(playlistId, nextItemIds, expectedRevision),
    );
    if (updated && mutationIsCurrent(request)) {
      setPlaylistDetail(updated);
      setOperationSuccess(
        direction < 0
          ? props.text.listen.moveQueueItemUp
          : props.text.listen.moveQueueItemDown,
      );
    }
  };

  const availableTracks = React.useMemo(
    () => {
      if (!addTracksOpen) {
        return [];
      }
      return sortListenLocalSongs(
        filterListenLocalWorkspaceTracks(
          props.tracks,
          addTracksQuery,
        ),
      );
    },
    [addTracksOpen, addTracksQuery, props.tracks],
  );
  const contentLoading = props.tracksLoading || playlistDetailLoading;
  const showBack = Boolean(selectedGroup || transientPlaylistId);
  const canPlayAll = playableCurrentQueue.length > 0;
  const showInternalTitle = !props.hideHeader || showBack || Boolean(activePlaylistId);
  const openPlaylist = React.useCallback(
    (playlistId: string) => {
      navigateWorkspace(
        { routeId: `playlist:${playlistId}` },
        APP_WORKSPACE_IDS.music,
      );
    },
    [navigateWorkspace],
  );
  const closeInternalDetail = React.useCallback(() => {
    if (selectedGroup) {
      setSelectedGroupId("");
    } else {
      setTransientPlaylistId("");
    }
  }, [selectedGroup]);
  const openCreatePlaylistDialog = React.useCallback(() => {
    setOperationError("");
    setPlaylistNameDraft("");
    setNameDialogMode("create");
  }, []);
  const openAddTracksDialog = React.useCallback(() => {
    setOperationError("");
    setAddTracksQuery("");
    setSelectedAddTrackIds(new Set());
    setAddTracksOpen(true);
  }, []);
  const openRenamePlaylistDialog = React.useCallback(() => {
    if (!playlistDetail) {
      return;
    }
    setOperationError("");
    setPlaylistNameDraft(playlistDetail.playlist.name);
    setNameDialogMode("rename");
  }, [playlistDetail]);
  const openDeletePlaylistDialog = React.useCallback(() => {
    setOperationError("");
    setDeleteDialogOpen(true);
  }, []);

  const overflowActions =
    activePlaylistId && playlistDetail ? (
      <>
        <DropdownMenuItem onSelect={openAddTracksDialog}>
          <Plus className="h-3.5 w-3.5" />
          {props.text.listen.localAddTracks}
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={openRenamePlaylistDialog}>
          <Pencil className="h-3.5 w-3.5" />
          {props.text.listen.localRenamePlaylist}
        </DropdownMenuItem>
        <DropdownMenuItem
          className="listen-dropdown-item--destructive"
          onSelect={openDeletePlaylistDialog}
        >
          <Trash2 className="h-3.5 w-3.5" />
          {props.text.listen.localDeletePlaylist}
        </DropdownMenuItem>
      </>
    ) : route?.kind === "home" ? (
      <>
        <DropdownMenuItem
          disabled={props.tracksRefreshing}
          onSelect={props.onRefreshTracks}
        >
          <RefreshCw
            className={cn(
              "h-3.5 w-3.5",
              props.tracksRefreshing && "listen-loading-spinner",
            )}
          />
          {props.text.listen.localRefresh}
        </DropdownMenuItem>
        <DropdownMenuItem
          disabled={props.tracksClearingMissing}
          onSelect={props.onRepairMissingTracks}
        >
          {props.tracksClearingMissing ? (
            <Loader2 className="h-3.5 w-3.5 listen-loading-spinner" />
          ) : (
            <Wrench className="h-3.5 w-3.5" />
          )}
          {props.text.completed.relinkDialogTitle}
        </DropdownMenuItem>
      </>
    ) : null;
  const headerActions = !emptySearchLanding ? (
    <TooltipProvider delayDuration={0}>
      <>
        {showBack ? (
          <WorkspacePrimaryHeaderActionGroup label={props.text.actions.back}>
            <WorkspacePrimaryHeaderAction
              label={props.text.actions.back}
              onClick={closeInternalDetail}
            >
              <ArrowLeft className="h-4 w-4" />
            </WorkspacePrimaryHeaderAction>
          </WorkspacePrimaryHeaderActionGroup>
        ) : null}
        {canPlayAll || !activePlaylistId ? (
          <WorkspacePrimaryHeaderActionGroup label={pageTitle}>
            {canPlayAll ? (
              <WorkspacePrimaryHeaderAction
                label={props.text.listen.playAll}
                onClick={() =>
                  props.onPlayTrack(playableCurrentQueue[0], playableCurrentQueue)
                }
              >
                <Play className="h-4 w-4" />
              </WorkspacePrimaryHeaderAction>
            ) : null}
            {!activePlaylistId ? (
              <WorkspacePrimaryHeaderAction
                label={props.text.listen.localNewPlaylist}
                onClick={openCreatePlaylistDialog}
              >
                <Plus className="h-4 w-4" />
              </WorkspacePrimaryHeaderAction>
            ) : null}
          </WorkspacePrimaryHeaderActionGroup>
        ) : null}
        {overflowActions ? (
          <WorkspacePrimaryHeaderActionGroup label={props.text.listen.more}>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <WorkspacePrimaryHeaderAction label={props.text.listen.more}>
                  <MoreHorizontal className="h-4 w-4" />
                </WorkspacePrimaryHeaderAction>
              </DropdownMenuTrigger>
              <WorkspacePrimaryHeaderMenuContent>
                {overflowActions}
              </WorkspacePrimaryHeaderMenuContent>
            </DropdownMenu>
          </WorkspacePrimaryHeaderActionGroup>
        ) : null}
      </>
    </TooltipProvider>
  ) : null;

  return (
    <div className="listen-local-workspace min-w-0 space-y-4 px-1 pb-3">
      {props.headerActionsTarget && headerActions
        ? createPortal(headerActions, props.headerActionsTarget)
        : !props.hideHeader
          ? headerActions
          : null}
      {!emptySearchLanding && showInternalTitle ? (
        <div className="min-w-0">
          <h2 className="listen-local-workspace__title truncate">
            {pageTitle}
          </h2>
          <p className="listen-local-workspace__meta">
            {formatTemplate(props.text.listen.localSongCount, { count })}
          </p>
        </div>
      ) : null}

      <div
        aria-atomic="true"
        aria-live="polite"
        className="sr-only"
        role="status"
      >
        {operationSuccess}
      </div>

      {operationError &&
      nameDialogMode === null &&
      !deleteDialogOpen &&
      !addTracksOpen ? (
        <div
          aria-atomic="true"
          aria-live="assertive"
          className="listen-status-panel px-3 py-2"
          data-tone="error"
          role="alert"
        >
          {operationError}
        </div>
      ) : null}

      {emptySearchLanding ? null : contentLoading ? (
        <LocalWorkspaceLoading label={props.text.listen.localLoading} />
      ) : activePlaylistId ? (
        playlistDetail ? (
          <LocalTrackList
            busy={localPlaylists.mutating}
            onMove={
              props.query.trim()
                ? undefined
                : (_track, direction, itemId) =>
                    void movePlaylistTrack(itemId, direction)
            }
            onPlay={(track) => playTrack(track, playlistTracks)}
            onEdit={setMetadataTrack}
            onRemove={(_track, itemId) => void removePlaylistTrack(itemId)}
            rowIds={visiblePlaylistItems.map((item) => item.id)}
            selectedId={props.selectedId}
            text={props.text}
            tracks={visiblePlaylistTracks}
          />
        ) : (
          <LocalWorkspaceEmpty label={props.text.listen.playlistEmpty} />
        )
      ) : selectedGroup ? (
        <LocalTrackList
          onEdit={setMetadataTrack}
          onPlay={(track) => playTrack(track, selectedGroup.tracks)}
          selectedId={props.selectedId}
          text={props.text}
          tracks={selectedGroup.tracks}
        />
      ) : route?.kind === "artists" || route?.kind === "albums" ? (
        <LocalCollectionGrid
          emptyLabel={props.text.listen.searchEmpty}
          groups={route.kind === "artists" ? artistGroups : albumGroups}
          icon={route.kind === "artists" ? "artist" : "album"}
          onSelect={(group) => setSelectedGroupId(group.id)}
          text={props.text}
        />
      ) : route?.kind === "home" ? (
        <div className="space-y-6">
          <ListenLocalPlaylistDirectory
            emptyLabel={props.text.listen.localPlaylistsEmpty}
            itemCountTemplate={props.text.listen.localSongCount}
            loading={localPlaylists.loading}
            loadingLabel={props.text.listen.localLoading}
            onSelect={openPlaylist}
            playlists={localPlaylists.playlists}
            title={props.text.workspace.playlists}
          />
          <section
            aria-label={props.text.workspace.songs}
            className="space-y-2.5"
          >
            <h3 className="listen-local-workspace__section-title">
              {props.text.workspace.songs}
            </h3>
            {visibleRouteTracks.length > 0 ? (
              <LocalTrackList
                onEdit={setMetadataTrack}
                onPlay={(track) => playTrack(track, visibleRouteTracks)}
                selectedId={props.selectedId}
                text={props.text}
                tracks={visibleRouteTracks}
              />
            ) : (
              <LocalWorkspaceEmpty label={props.text.listen.searchEmpty} />
            )}
          </section>
        </div>
      ) : visibleRouteTracks.length > 0 ? (
        <LocalTrackList
          onEdit={setMetadataTrack}
          onPlay={(track) => playTrack(track, visibleRouteTracks)}
          selectedId={props.selectedId}
          text={props.text}
          tracks={visibleRouteTracks}
        />
      ) : (
        <LocalWorkspaceEmpty label={props.text.listen.searchEmpty} />
      )}

      <PlaylistNameDialog
        draft={playlistNameDraft}
        error={operationError}
        mode={nameDialogMode}
        mutating={localPlaylists.mutating}
        onDraftChange={setPlaylistNameDraft}
        onOpenChange={(open) => {
          if (!open && !localPlaylists.mutating) {
            setNameDialogMode(null);
          }
        }}
        onSubmit={() => void submitPlaylistName()}
        text={props.text}
      />
      <ListenLocalMetadataEditor
        httpBaseURL={props.httpBaseURL}
        onOpenChange={(open) => {
          if (!open) {
            setMetadataTrack(null);
          }
        }}
        onSaved={() => props.onRefreshTracks()}
        open={Boolean(metadataTrack)}
        text={props.text}
        track={metadataTrack}
      />
      <Dialog
        onOpenChange={(open) => {
          if (!localPlaylists.mutating) {
            setDeleteDialogOpen(open);
          }
        }}
        open={deleteDialogOpen}
      >
        <DialogContent className="max-w-sm">
          <DialogHeader>
            <DialogTitle>{props.text.listen.localDeletePlaylist}</DialogTitle>
            <DialogDescription>
              {formatTemplate(props.text.listen.localDeletePlaylistConfirm, {
                name: playlistDetail?.playlist.name ?? "",
              })}
            </DialogDescription>
          </DialogHeader>
          {operationError ? (
            <p
              aria-atomic="true"
              aria-live="assertive"
              className="listen-status-text listen-local-dialog-error"
              data-tone="error"
              role="alert"
            >
              {operationError}
            </p>
          ) : null}
          <DialogFooter>
            <Button
              disabled={localPlaylists.mutating}
              onClick={() => setDeleteDialogOpen(false)}
              type="button"
              variant="outline"
            >
              {props.text.actions.cancelDialog}
            </Button>
            <Button
              disabled={localPlaylists.mutating}
              onClick={() => void deletePlaylist()}
              type="button"
              variant="destructive"
            >
              {localPlaylists.mutating ? (
                <Loader2 className="h-4 w-4 listen-loading-spinner" />
              ) : (
                <Trash2 className="h-4 w-4" />
              )}
              {props.text.actions.deleteItem}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
      <AddPlaylistTracksDialog
        availableTracks={availableTracks}
        error={operationError}
        mutating={localPlaylists.mutating}
        onOpenChange={(open) => {
          if (!localPlaylists.mutating) {
            setAddTracksOpen(open);
          }
        }}
        onQueryChange={setAddTracksQuery}
        onSubmit={() => void addSelectedTracks()}
        onToggle={(trackId) =>
          setSelectedAddTrackIds((current) => {
            const next = new Set(current);
            if (next.has(trackId)) {
              next.delete(trackId);
            } else {
              next.add(trackId);
            }
            return next;
          })
        }
        open={addTracksOpen}
        query={addTracksQuery}
        selectedIds={selectedAddTrackIds}
        text={props.text}
      />
    </div>
  );
}

function LocalCollectionGrid(props: {
  groups: ListenLocalCollectionGroup[];
  icon: "artist" | "album";
  emptyLabel: string;
  text: ListenPageProps["text"];
  onSelect: (group: ListenLocalCollectionGroup) => void;
}) {
  if (props.groups.length === 0) {
    return <LocalWorkspaceEmpty label={props.emptyLabel} />;
  }
  return (
    <div className="grid grid-cols-[repeat(auto-fill,minmax(10rem,1fr))] gap-x-4 gap-y-6">
      {props.groups.map((group) => (
        <button
          className="listen-local-collection-card group min-w-0 hover:-translate-y-0.5"
          key={group.id}
          onClick={() => props.onSelect(group)}
          type="button"
        >
          <div className="listen-local-collection-card__artwork relative aspect-square overflow-hidden">
            {group.tracks[0] ? (
              <ListenLocalArtwork
                className="listen-local-collection-card__image h-full w-full"
                track={group.tracks[0]}
              />
            ) : props.icon === "artist" ? (
              <UserRound className="listen-local-collection-card__fallback absolute inset-0 m-auto h-8 w-8" />
            ) : (
              <Disc3 className="listen-local-collection-card__fallback absolute inset-0 m-auto h-8 w-8" />
            )}
            <span className="listen-local-collection-card__play absolute bottom-2 right-2 grid h-8 w-8 place-items-center">
              <Play className="listen-playback-icon--filled h-4 w-4" />
            </span>
          </div>
          <div className="mt-2.5 min-w-0 px-0.5">
            <div className="listen-local-collection-card__title truncate">
              {group.title}
            </div>
            <div className="listen-local-collection-card__meta mt-0.5 truncate">
              {group.subtitle ||
                formatTemplate(props.text.listen.localSongCount, {
                  count: group.tracks.length,
                })}
            </div>
          </div>
        </button>
      ))}
    </div>
  );
}

function LocalTrackList(props: {
  tracks: ListenLocalItem[];
  rowIds?: string[];
  selectedId: string;
  text: ListenPageProps["text"];
  busy?: boolean;
  onPlay: (track: ListenLocalItem) => void;
  onEdit?: (track: ListenLocalItem) => void;
  onMove?: (track: ListenLocalItem, direction: -1 | 1, rowId: string) => void;
  onRemove?: (track: ListenLocalItem, rowId: string) => void;
}) {
  if (props.tracks.length === 0) {
    return <LocalWorkspaceEmpty label={props.text.listen.playlistEmpty} />;
  }
  return (
    <div className="space-y-1" role="list">
      {props.tracks.map((track, index) => {
        const rowId = props.rowIds?.[index] || track.id;
        const selected = track.id === props.selectedId;
        const probeWarning = formatListenLocalProbeWarning(
          props.text.listen.localProbeFailed,
          track.probeError,
        );
        return (
          <div
            className="listen-local-track-row group flex min-w-0 items-center gap-1 px-1.5 py-1"
            data-selected={selected ? "true" : "false"}
            key={rowId}
            role="listitem"
          >
            <button
              aria-label={
                !track.playbackSupported
                  ? `${track.title}: ${props.text.listen.localPlaybackUnsupported}`
                  : probeWarning
                    ? `${track.title}: ${probeWarning}`
                    : track.title
              }
              className="listen-local-track-row__button flex min-w-0 flex-1 items-center gap-3 px-1.5 py-1"
              disabled={props.busy || !track.playbackSupported}
              onClick={() => props.onPlay(track)}
              title={
                !track.playbackSupported
                  ? props.text.listen.localPlaybackUnsupported
                  : probeWarning || undefined
              }
              type="button"
            >
              <ListenLocalArtwork className="listen-local-track-row__artwork h-10 w-10" track={track} />
              <span className="min-w-0 flex-1">
                <span className="listen-local-track-row__title block truncate">
                  {track.title}
                </span>
                {!track.playbackSupported ? (
                  <span className="listen-local-track__unsupported mt-0.5 block truncate">
                    {props.text.listen.localPlaybackUnsupported}
                    {track.format || track.audioCodec
                      ? ` · ${(track.format || track.audioCodec).toUpperCase()}`
                      : ""}
                  </span>
                ) : track.probeError ? (
                  <ListenLocalProbeWarning
                    className="listen-local-track-row__probe mt-0.5 flex"
                    error={track.probeError}
                    message={props.text.listen.localProbeFailed}
                  />
                ) : (
                  <span className="listen-local-track-row__meta mt-0.5 block truncate">
                    {[track.author, track.album].filter(Boolean).join(" · ") || track.path}
                  </span>
                )}
              </span>
              <span className="listen-local-track-row__duration shrink-0">
                {track.durationLabel}
              </span>
            </button>
            {props.onEdit || props.onMove || props.onRemove ? (
              <div className="listen-local-track-row__actions flex shrink-0 items-center gap-0.5">
                {props.onEdit ? (
                  <LocalTrackAction
                    disabled={props.busy || !track.metadataWritable}
                    label={
                      track.metadataWritable
                        ? props.text.listen.localMetadataEdit
                        : props.text.listen.localMetadataUnsupported
                    }
                    onClick={() => props.onEdit?.(track)}
                  >
                    <Pencil />
                  </LocalTrackAction>
                ) : null}
                {props.onMove ? (
                  <>
                    <LocalTrackAction
                      disabled={props.busy || index === 0}
                      label={props.text.listen.moveQueueItemUp}
                      onClick={() => props.onMove?.(track, -1, rowId)}
                    >
                      <ChevronUp />
                    </LocalTrackAction>
                    <LocalTrackAction
                      disabled={props.busy || index === props.tracks.length - 1}
                      label={props.text.listen.moveQueueItemDown}
                      onClick={() => props.onMove?.(track, 1, rowId)}
                    >
                      <ChevronDown />
                    </LocalTrackAction>
                  </>
                ) : null}
                {props.onRemove ? (
                  <LocalTrackAction
                    destructive
                    disabled={props.busy}
                    label={props.text.listen.localRemoveTrack}
                    onClick={() => props.onRemove?.(track, rowId)}
                  >
                    <Trash2 />
                  </LocalTrackAction>
                ) : null}
              </div>
            ) : null}
          </div>
        );
      })}
    </div>
  );
}

function LocalTrackAction(props: {
  label: string;
  disabled?: boolean;
  destructive?: boolean;
  children: React.ReactNode;
  onClick: () => void;
}) {
  return (
    <Button
      aria-label={props.label}
      className="listen-local-track-action h-7 w-7 [&>svg]:h-3.5 [&>svg]:w-3.5"
      disabled={props.disabled}
      shape="square"
      size="icon"
      tone={props.destructive ? "destructive" : "neutral"}
      variant="ghost"
      onClick={props.onClick}
      title={props.label}
      type="button"
    >
      {props.children}
    </Button>
  );
}

function PlaylistNameDialog(props: {
  mode: PlaylistNameDialogMode;
  draft: string;
  mutating: boolean;
  error: string;
  text: ListenPageProps["text"];
  onDraftChange: (value: string) => void;
  onOpenChange: (open: boolean) => void;
  onSubmit: () => void;
}) {
  const title =
    props.mode === "rename"
      ? props.text.listen.localRenamePlaylist
      : props.text.listen.localNewPlaylist;
  return (
    <Dialog onOpenChange={props.onOpenChange} open={props.mode !== null}>
      <DialogContent className="max-w-sm">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{props.text.listen.localPlaylistName}</DialogDescription>
        </DialogHeader>
        {props.error ? (
          <p
            aria-atomic="true"
            aria-live="assertive"
            className="listen-status-text listen-local-dialog-error"
            data-tone="error"
            role="alert"
          >
            {props.error}
          </p>
        ) : null}
        <form
          className="space-y-3"
          onSubmit={(event) => {
            event.preventDefault();
            props.onSubmit();
          }}
        >
          <Input
            aria-label={props.text.listen.localPlaylistName}
            autoFocus
            disabled={props.mutating}
            maxLength={120}
            onChange={(event) => props.onDraftChange(event.currentTarget.value)}
            placeholder={props.text.listen.localPlaylistNamePlaceholder}
            value={props.draft}
          />
          <DialogFooter>
            <Button
              disabled={props.mutating}
              onClick={() => props.onOpenChange(false)}
              type="button"
              variant="outline"
            >
              {props.text.actions.cancelDialog}
            </Button>
            <Button
              disabled={!props.draft.trim() || props.mutating}
              type="submit"
            >
              {props.mutating ? <Loader2 className="h-4 w-4 listen-loading-spinner" /> : null}
              {props.text.actions.save}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function AddPlaylistTracksDialog(props: {
  open: boolean;
  query: string;
  availableTracks: ListenLocalItem[];
  selectedIds: Set<string>;
  mutating: boolean;
  error: string;
  text: ListenPageProps["text"];
  onOpenChange: (open: boolean) => void;
  onQueryChange: (query: string) => void;
  onToggle: (trackId: string) => void;
  onSubmit: () => void;
}) {
  return (
    <Dialog onOpenChange={props.onOpenChange} open={props.open}>
      <DialogContent className="grid max-h-[min(38rem,calc(100vh-2rem))] max-w-lg grid-rows-[auto_auto_minmax(0,1fr)_auto]">
        <div className="space-y-2">
          <DialogHeader>
            <DialogTitle>{props.text.listen.localAddTracks}</DialogTitle>
            <DialogDescription>
              {formatTemplate(props.text.listen.localSongCount, {
                count: props.selectedIds.size,
              })}
            </DialogDescription>
          </DialogHeader>
          {props.error ? (
            <p
              aria-atomic="true"
              aria-live="assertive"
              className="listen-status-text listen-local-dialog-error"
              data-tone="error"
              role="alert"
            >
              {props.error}
            </p>
          ) : null}
        </div>
        <Input
          aria-label={props.text.listen.searchLocal}
          onChange={(event) => props.onQueryChange(event.currentTarget.value)}
          placeholder={props.text.listen.searchLocal}
          value={props.query}
        />
        <div className="listen-local-add-tracks-list min-h-0 space-y-1 overflow-y-auto p-1.5">
          {props.availableTracks.length > 0 ? (
            props.availableTracks.map((track) => (
              <label
                className="listen-local-add-track-row flex min-w-0 items-center gap-3 px-2 py-1.5"
                key={track.id}
              >
                <input
                  checked={props.selectedIds.has(track.id)}
                  disabled={props.mutating}
                  onChange={() => props.onToggle(track.id)}
                  type="checkbox"
                />
                <ListenLocalArtwork className="listen-local-add-track-row__artwork h-9 w-9" track={track} />
                <span className="min-w-0 flex-1">
                  <span className="listen-local-add-track-row__title block truncate">{track.title}</span>
                  <span className="listen-local-add-track-row__meta block truncate">
                    {track.author || track.album || track.path}
                  </span>
                </span>
              </label>
            ))
          ) : (
            <LocalWorkspaceEmpty label={props.text.listen.searchEmpty} />
          )}
        </div>
        <DialogFooter>
          <Button
            disabled={props.mutating}
            onClick={() => props.onOpenChange(false)}
            type="button"
            variant="outline"
          >
            {props.text.actions.cancelDialog}
          </Button>
          <Button
            disabled={props.selectedIds.size === 0 || props.mutating}
            onClick={props.onSubmit}
            type="button"
          >
            {props.mutating ? <Loader2 className="h-4 w-4 listen-loading-spinner" /> : null}
            {props.text.listen.localAddSelectedTracks}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function LocalWorkspaceLoading(props: { label: string }) {
  return (
    <div className="listen-local-workspace-loading flex min-h-40 flex-col items-center justify-center gap-2">
      <Loader2 className="h-5 w-5 listen-loading-spinner" />
      {props.label}
    </div>
  );
}

function LocalWorkspaceEmpty(props: { label: string }) {
  return (
    <div className="listen-local-workspace-empty flex min-h-40 flex-col items-center justify-center gap-2 px-5">
      <ListMusic className="h-7 w-7" />
      {props.label}
    </div>
  );
}

function resolveLocalWorkspaceTitle(options: {
  routeKind:
    | "home"
    | "search"
    | "recently-added"
    | "artists"
    | "albums"
    | "songs"
    | "playlist"
    | undefined;
  playlistName?: string;
  selectedGroup: ListenLocalCollectionGroup | null;
  text: ListenPageProps["text"];
}) {
  if (options.playlistName) {
    return options.playlistName;
  }
  if (options.selectedGroup) {
    return options.selectedGroup.title;
  }
  switch (options.routeKind) {
    case "home":
      return options.text.workspace.home;
    case "search":
      return options.text.workspace.search;
    case "recently-added":
      return options.text.listen.localRecentlyAdded;
    case "artists":
      return options.text.listen.searchArtists;
    case "albums":
      return options.text.listen.localAlbums;
    case "songs":
      return options.text.listen.searchSongs;
    case "playlist":
      return options.text.listen.searchPlaylists;
    default:
      return options.text.listen.linger;
  }
}

function resolvePlaylistError(error: unknown, fallback: string) {
  const message = error instanceof Error ? error.message.trim() : "";
  return message ? `${fallback} ${message}` : fallback;
}

function formatTemplate(
  template: string,
  values: Record<string, string | number>,
) {
  return Object.entries(values).reduce(
    (result, [key, value]) => result.split(`{${key}}`).join(String(value)),
    template,
  );
}
