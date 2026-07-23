# RSS cache strategy

This document defines the cache contract for the built-in `RSS` station. The
goal is instant return navigation and offline readability without hiding a
successful refresh or weakening the remote-resource security boundary.

## Source of truth

SQLite is the durable source of truth for subscriptions, entries, hydrated
bodies, reading state, discovery metadata, refresh validators, and sync
cursors. React Query is a bounded in-process projection of SQLite; it is not a
second database. Remote feeds are fetched only by the RSS application service,
never as a consequence of mounting a page component.

The UI follows stale-while-revalidate:

1. Render the matching in-memory projection immediately when it exists.
2. Otherwise read SQLite through the Wails handler.
3. Refresh a remote feed only on its background schedule, explicit refresh,
   initial subscription hydration, or history backfill.
4. Merge the durable result and invalidate only projections affected by the
   resulting revision.

Query payloads are intentionally not persisted to browser storage. Desktop
resource URLs contain a process-scoped loopback token and become invalid after
restart, while SQLite already provides the authoritative persistent cache.

## Cache layers and budgets

| Data | Fresh for | Retained unused | Storage | Revalidation |
| --- | ---: | ---: | --- | --- |
| Entry collections | 10 minutes | 60 minutes | React Query + SQLite | revision/snapshot or explicit refresh |
| Hydrated entry detail | 30 minutes | 2 hours | React Query + SQLite | entry revision |
| Subscriptions | 30 minutes | 2 hours | React Query + SQLite | subscription revision |
| Local/remote search | 10 minutes | 60 minutes | React Query; local results in SQLite | normalized search key |
| Discovery catalog | 24 hours | 48 hours | React Query + SQLite | stale-while-refresh or explicit refresh |
| Successful feed preview | 4 minutes | 16 entries | bounded session cache | canonical URL + requested view type |
| Failed feed preview | 60 seconds | 16 entries | bounded session cache | retry or expiry |
| RSS image/favicon bytes | content 24h; icon 7d | 256 items / 64 MiB memory; 256 MiB disk | memory LRU + private disk cache | source identity and internal ETag |
| Failed image/favicon | 5 minutes | 5 minutes | memory negative cache | retry or expiry |
| Loaded image UI state | session | bounded LRU | renderer memory | projected resource URL |
| View scroll/selection state | session | 64 views | renderer memory | route + subscription + presentation + filters |

A fresh renderer projection does not refetch merely because the window
focused, the network reconnected, or a route remounted. Collections,
subscriptions, and details revalidate after precise invalidation; active
collection polling is a low-frequency SQLite snapshot reconciliation mechanism
and runs only while the RSS station is active. Remote search and Discovery have
no poller, so they revalidate on a later mount only after their 10-minute and
24-hour TTLs respectively. None of these renderer reads fetches a remote feed.

## Keys and invalidation

Every key includes all result-shaping input after normalization: workspace,
subscription, kind, search text, unread/starred filters, sort, and pagination
origin. Equivalent inputs share a key; different inputs cannot share data.

Mutations use optimistic cache updates and precise reconciliation:

| Mutation | Patch immediately | Invalidate/reset |
| --- | --- | --- |
| Read/unread | matching entry in cached lists and detail | unread-only projections and subscription unread totals |
| Star/unstar | matching entry in cached lists and detail | starred-only projections |
| Reading/video progress | matching entry detail and loaded rows | no collection-wide invalidation |
| Rename/pause/view type | subscription row | only projections affected by that subscription |
| Add/remove subscription | subscription list | aggregate projections and the changed subscription |
| Feed refresh | none until durable commit | changed subscription, aggregate projections, changed entry details |
| History backfill | preserve unrelated pages | aggregate pages and only source subscriptions reported by the result |
| Mark all read | optimistic rows where practical | read-derived projections after the durable batch settles |

Offset-based infinite pages are reset only when insertion/removal can change
their offsets. Returning to an unchanged view keeps its loaded pages, selected
item, image state, and scroll position. Manual refresh bypasses freshness TTLs;
normal navigation never does.

## Remote resources

An entity is resolved before every resource lookup so changing an entry slot
cannot expose bytes cached for the old source. The byte-cache identity includes
the canonical source URL, safe referrer origin, resource role, and variant.
Only a payload that passed the existing MIME, decoded-dimension, animation,
size, redirect, and public-network checks may enter positive cache.

Concurrent misses for one identity are coalesced. Failed fetches receive a
short negative cache to stop broken feeds from creating request storms. A
validated stale image may be served if revalidation fails; an unsafe or
unvalidated payload may not.

The tokenized Desktop surface may return a private cacheable response with an
internal ETag. The paired-device resource API and all streamed media remain
`private, no-store`. Client validators are checked only against XiaDown's
internal cache identity and are never forwarded to a newly resolved upstream
URL.

Desktop entry and subscription resource URLs include only the durable entity
revision as a cache-busting value. They never contain the upstream URL. A
content revision therefore gets a distinct Chromium/session-cache identity,
while repeated mounts of the same revision can use browser cache without a
loopback round trip. Exact versioned Desktop routes use
`private, max-age=3600, immutable`; unversioned or malformed revision queries
and Discovery resources must revalidate. Paired-device resources, streamed
media, and all error responses remain `private, no-store`.

Validated Desktop image bytes persist under
`<UserCacheDir>/xiadown/rss/resources/v1`. Files and their parent directory are
private, writes use a temporary file plus `fsync` and atomic rename, and a
bounded asynchronous cleanup removes expired and least-recently-used records.
The versioned directory is disposable cache data, not a second source of truth.
Disk metadata contains only the hashed cache key, role, local validator,
deadlines, type, and size—never an upstream URL or referrer. Every disk hit is
treated as untrusted and repeats MIME, decoded-image, dimension, animation,
role, and ETag validation before promotion to memory.

## Feed and paired-device behavior

Remote feed refresh keeps using persisted `ETag`/`Last-Modified` provenance and
bounded conditional requests. A `304` updates refresh metadata without
rewriting entries. Refresh failure never removes the last successful SQLite
snapshot.

Paired clients consume the versioned snapshot/change contract and keep their
own cache and outbox implementation in the client repository. Desktop resource
and media responses remain `private, no-store`; Desktop cache eviction never
deletes synchronized RSS history or changes a paired client's cursor.

## Verification gates

- Navigate A → B → A inside the freshness window: A's query function is
  not called again and its scroll position is restored.
- Read or star one entry: unrelated collection and subscription queries remain
  fresh; filtered totals become correct.
- Backfill one subscription: another subscription's loaded infinite pages are
  unchanged.
- Mount the same projected image concurrently: one upstream request occurs;
  later mounts use cached validated bytes and do not flash a broken image.
- Change the resource behind an entity slot: the old cached bytes and validator
  cannot be reused.
- Restart Desktop: SQLite content is immediately readable; expiring loopback
  URLs are regenerated rather than restored from browser storage.
- Paired resource and media responses remain `no-store`, including error paths.
