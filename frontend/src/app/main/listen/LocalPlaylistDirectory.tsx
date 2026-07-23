import { ListMusic, Loader2 } from "lucide-react";

import type { ListenLocalPlaylist } from "@/app/main/listen/local-playlists";

export interface ListenLocalPlaylistDirectoryProps {
  playlists: ListenLocalPlaylist[];
  loading: boolean;
  title: string;
  emptyLabel: string;
  loadingLabel: string;
  itemCountTemplate: string;
  onSelect: (playlistId: string) => void;
}

export function ListenLocalPlaylistDirectory(
  props: ListenLocalPlaylistDirectoryProps,
) {
  const showInitialLoading = props.loading && props.playlists.length === 0;

  return (
    <section aria-label={props.title} className="space-y-2.5">
      <div className="flex items-center gap-2">
        <h3 className="listen-local-playlist-directory__title min-w-0 flex-1 truncate">
          {props.title}
        </h3>
        {props.loading && !showInitialLoading ? (
          <Loader2
            aria-label={props.loadingLabel}
            className="listen-local-playlist-directory__loading-icon h-3.5 w-3.5 listen-loading-spinner"
          />
        ) : null}
      </div>

      {showInitialLoading ? (
        <div
          aria-live="polite"
          className="listen-local-playlist-directory__loading flex min-h-24 items-center justify-center gap-2"
          role="status"
        >
          <Loader2 className="h-4 w-4 listen-loading-spinner" />
          {props.loadingLabel}
        </div>
      ) : props.playlists.length > 0 ? (
        <div className="grid grid-cols-[repeat(auto-fill,minmax(14rem,1fr))] gap-2.5">
          {props.playlists.map((playlist) => {
            const itemCountLabel = formatItemCount(
              props.itemCountTemplate,
              playlist.itemCount,
            );
            return (
              <button
                aria-label={`${playlist.name}, ${itemCountLabel}`}
                className="listen-local-playlist-card group flex min-w-0 items-center gap-3 p-3 hover:-translate-y-0.5"
                data-playlist-id={playlist.id}
                key={playlist.id}
                onClick={() => props.onSelect(playlist.id)}
                type="button"
              >
                <span className="listen-local-playlist-card__icon grid h-10 w-10 shrink-0 place-items-center">
                  <ListMusic className="h-5 w-5" />
                </span>
                <span className="min-w-0 flex-1">
                  <span className="listen-local-playlist-card__title block truncate">
                    {playlist.name}
                  </span>
                  <span className="listen-local-playlist-card__meta mt-0.5 block truncate">
                    {itemCountLabel}
                  </span>
                </span>
              </button>
            );
          })}
        </div>
      ) : (
        <div className="listen-local-playlist-directory__empty flex min-h-24 flex-col items-center justify-center gap-2 px-5">
          <ListMusic className="h-6 w-6" />
          {props.emptyLabel}
        </div>
      )}
    </section>
  );
}

function formatItemCount(template: string, count: number) {
  return template.split("{count}").join(String(count));
}
