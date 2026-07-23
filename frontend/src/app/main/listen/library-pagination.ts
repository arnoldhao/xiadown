import { dedupePlaylistItems } from "@/app/main/listen/storage";
import type {
  ListenOnlineBrowseSource,
  ListenPlaylistItem,
} from "@/app/main/listen/types";

export function mergeListenLibraryPagePlaylists(
  source: ListenOnlineBrowseSource,
  current: ListenPlaylistItem[],
  incoming: ListenPlaylistItem[],
) {
  if (source !== "playlists" || incoming.length === 0) {
    return current;
  }
  return dedupePlaylistItems([...current, ...incoming]);
}

export function shouldAutoLoadListenLibraryPage(options: {
  normalizedQuery: string;
  continuation: string;
  likedMusicWorkspaceRoute: boolean;
  workspaceSearchRoute: boolean;
}) {
  return (
    !options.normalizedQuery &&
    Boolean(options.continuation) &&
    !options.likedMusicWorkspaceRoute &&
    !options.workspaceSearchRoute
  );
}
