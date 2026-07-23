export type ListenArtistRequestKind = "shelf" | "action";

export type ListenArtistRequest = {
  kind: ListenArtistRequestKind;
  artistIdentity: string;
  generation: number;
  controller: AbortController;
};

export type ListenArtistRequestRegistry = {
  artistIdentity: string;
  generation: number;
  latestGeneration: Record<ListenArtistRequestKind, number>;
  active: Partial<Record<ListenArtistRequestKind, ListenArtistRequest>>;
};

export type ListenArtistShelfViewRequest = {
  artistIdentity: string;
  generation: number;
  shelfId: string;
};

export function createListenArtistIdentity(
  artist: Pick<{ id: string; name: string }, "id" | "name"> | null | undefined,
) {
  if (!artist) {
    return "";
  }
  return JSON.stringify([artist.id.trim(), artist.name.trim()]);
}

export function createListenArtistRequestRegistry(): ListenArtistRequestRegistry {
  return {
    artistIdentity: "",
    generation: 0,
    latestGeneration: { shelf: 0, action: 0 },
    active: {},
  };
}

export function invalidateListenArtistRequests(
  registry: ListenArtistRequestRegistry,
  artistIdentity: string,
) {
  registry.active.shelf?.controller.abort();
  registry.active.action?.controller.abort();
  registry.active = {};
  registry.artistIdentity = artistIdentity;
  const generation = registry.generation + 1;
  registry.generation = generation;
  registry.latestGeneration.shelf = generation;
  registry.latestGeneration.action = generation;
}

export function synchronizeListenArtistRequestIdentity(
  registry: ListenArtistRequestRegistry,
  artistIdentity: string,
) {
  if (registry.artistIdentity === artistIdentity) {
    return false;
  }
  invalidateListenArtistRequests(registry, artistIdentity);
  return true;
}

export function beginListenArtistRequest(
  registry: ListenArtistRequestRegistry,
  kind: ListenArtistRequestKind,
  artistIdentity: string,
): ListenArtistRequest {
  synchronizeListenArtistRequestIdentity(registry, artistIdentity);
  registry.active[kind]?.controller.abort();
  const request: ListenArtistRequest = {
    kind,
    artistIdentity,
    generation: registry.generation + 1,
    controller: new AbortController(),
  };
  registry.generation = request.generation;
  registry.latestGeneration[kind] = request.generation;
  registry.active[kind] = request;
  return request;
}

export function isListenArtistRequestCurrent(
  registry: ListenArtistRequestRegistry,
  request: ListenArtistRequest,
) {
  return (
    !request.controller.signal.aborted &&
    registry.artistIdentity === request.artistIdentity &&
    registry.latestGeneration[request.kind] === request.generation
  );
}

export function finishListenArtistRequest(
  registry: ListenArtistRequestRegistry,
  request: ListenArtistRequest,
) {
  if (registry.active[request.kind] === request) {
    delete registry.active[request.kind];
  }
}

export function isListenArtistShelfViewRequestCurrent(
  current: ListenArtistShelfViewRequest,
  request: ListenArtistShelfViewRequest,
  activeArtistIdentity: string,
) {
  return (
    activeArtistIdentity === request.artistIdentity &&
    current.artistIdentity === request.artistIdentity &&
    current.generation === request.generation &&
    current.shelfId === request.shelfId
  );
}
