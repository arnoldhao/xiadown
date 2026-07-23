export type ListenLocalPlaylistMutationContext = {
  routeId: string;
  playlistId: string;
};

export type ListenLocalPlaylistMutationRequest =
  ListenLocalPlaylistMutationContext & {
    generation: number;
  };

/**
 * Invalidates UI writes from mutations that complete after navigation or a
 * newer command. The backend result remains authoritative and will still be
 * picked up by the playlist reload triggered by the mutation client.
 */
export function createListenLocalPlaylistMutationGuard() {
  let generation = 0;

  return {
    begin(
      context: ListenLocalPlaylistMutationContext,
    ): ListenLocalPlaylistMutationRequest {
      generation += 1;
      return { ...context, generation };
    },
    invalidate() {
      generation += 1;
    },
    isCurrent(
      request: ListenLocalPlaylistMutationRequest,
      context: ListenLocalPlaylistMutationContext,
    ) {
      return (
        request.generation === generation &&
        request.routeId === context.routeId &&
        request.playlistId === context.playlistId
      );
    },
  };
}
