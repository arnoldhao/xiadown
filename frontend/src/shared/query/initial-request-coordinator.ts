export interface InitialRequestCoordinator<T> {
  request(): Promise<T>;
  preload(): Promise<void>;
  requestInitial(): Promise<T>;
}

/**
 * Coordinates an optional startup preload with the first real query without
 * letting a late preload create a duplicate request or a stale future result.
 */
export function createInitialRequestCoordinator<T>(
  fetchValue: () => Promise<T>,
): InitialRequestCoordinator<T> {
  let pendingRequest: Promise<T> | undefined;
  let startupPreload: Promise<T> | undefined;
  let initialRequested = false;

  const request = () => {
    if (pendingRequest) {
      return pendingRequest;
    }
    const current = fetchValue();
    pendingRequest = current;
    current.then(
      () => {
        if (pendingRequest === current) pendingRequest = undefined;
      },
      () => {
        if (pendingRequest === current) pendingRequest = undefined;
      },
    );
    return current;
  };

  const preload = async () => {
    if (initialRequested) {
      await pendingRequest;
      return;
    }
    startupPreload ??= request();
    await startupPreload;
  };

  const requestInitial = () => {
    initialRequested = true;
    const preloadRequest = startupPreload;
    startupPreload = undefined;
    return preloadRequest ?? request();
  };

  return { request, preload, requestInitial };
}
