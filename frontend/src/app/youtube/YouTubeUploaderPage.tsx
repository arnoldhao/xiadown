import {
  ArrowLeft,
  Info,
  LoaderCircle,
  MoreHorizontal,
  Play,
  RefreshCw,
  X,
  Youtube,
} from "lucide-react";
import * as React from "react";

import {
  appendYouTubeUploaderPage,
  getYouTubeWorkspaceUploader,
  setYouTubeWorkspaceChannelSubscription,
} from "@/app/youtube/api";
import { ListenInfiniteScrollSentinel } from "@/app/main/listen/infinite-scroll-sentinel";
import { YouTubeImage } from "@/app/youtube/YouTubeImage";
import { YouTubeSubscriptionIconButton } from "@/app/youtube/YouTubeSubscriptionIconButton";
import { formatYouTubeViewCount } from "@/app/youtube/page-state";
import type {
  YouTubeUploaderPageData,
  YouTubeWorkspaceVideo,
} from "@/app/youtube/types";
import {
  YouTubeSubscriptionRequestGate,
  youtubeSubscriptionIdentity,
} from "@/app/youtube/subscription-request-gate";
import { Button } from "@/shared/ui/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogScrollArea,
  DialogTitle,
  DialogTrigger,
} from "@/shared/ui/dialog";

import "./youtube-uploader-page.css";

export interface YouTubeUploaderPageLabels {
  back: string;
  subscribe: string;
  unsubscribe: string;
  videos: string;
  loading: string;
  empty: string;
  error: string;
  retry: string;
  loadMore: string;
  fallbackChannel: string;
  more: string;
  description: string;
  close: string;
}

export interface YouTubeUploaderPageProps {
  channelId: string;
  locale: string;
  fallbackName?: string;
  fallbackAvatarUrl?: string;
  fallbackSubscribed?: boolean;
  labels: YouTubeUploaderPageLabels;
  onBack: () => void;
  onOpenVideo: (video: YouTubeWorkspaceVideo, queue: YouTubeWorkspaceVideo[]) => void;
  onSubscriptionChange?: (channelId: string, subscribed: boolean) => void;
  onError?: (error: unknown) => void;
}

export function YouTubeUploaderPage(props: YouTubeUploaderPageProps) {
  const channelId = props.channelId.trim();
  const [page, setPage] = React.useState<YouTubeUploaderPageData | null>(null);
  const [loadingMore, setLoadingMore] = React.useState(false);
  const [paginationFailed, setPaginationFailed] = React.useState(false);
  const [subscriptionBusy, setSubscriptionBusy] = React.useState(false);
  const [failed, setFailed] = React.useState(false);
  const [reloadToken, setReloadToken] = React.useState(0);
  const loadingMoreRef = React.useRef(false);
  const paginationRequestRef = React.useRef(0);
  const subscriptionGateRef = React.useRef(
    new YouTubeSubscriptionRequestGate(
      youtubeSubscriptionIdentity(channelId),
    ),
  );
  subscriptionGateRef.current.activate(youtubeSubscriptionIdentity(channelId));

  React.useEffect(() => () => {
    subscriptionGateRef.current.invalidate();
  }, []);

  React.useEffect(() => {
    let cancelled = false;
    setPage(null);
    setFailed(false);
    setPaginationFailed(false);
    setLoadingMore(false);
    setSubscriptionBusy(false);
    loadingMoreRef.current = false;
    paginationRequestRef.current += 1;
    void getYouTubeWorkspaceUploader(channelId, { locale: props.locale })
      .then((result) => {
        if (cancelled) return;
        setPage({
          ...result,
          name:
            result.name === result.channelId && props.fallbackName
              ? props.fallbackName
              : result.name,
          avatarUrl: result.avatarUrl || props.fallbackAvatarUrl,
          isSubscribed:
            result.isSubscribed ?? props.fallbackSubscribed ?? false,
        });
      })
      .catch((error) => {
        if (cancelled) return;
        setFailed(true);
        props.onError?.(error);
      });
    return () => {
      cancelled = true;
    };
  }, [
    channelId,
    props.fallbackAvatarUrl,
    props.fallbackName,
    props.fallbackSubscribed,
    props.locale,
    props.onError,
    reloadToken,
  ]);

  const loadMore = React.useCallback(() => {
    const continuation = page?.continuation?.trim() || "";
    if (!page || !continuation || loadingMoreRef.current) return;
    loadingMoreRef.current = true;
    const requestID = paginationRequestRef.current + 1;
    paginationRequestRef.current = requestID;
    setLoadingMore(true);
    setPaginationFailed(false);
    void getYouTubeWorkspaceUploader(channelId, {
      continuation,
      locale: props.locale,
    })
      .then((incoming) => {
        if (paginationRequestRef.current !== requestID) return;
        setPage((current) =>
          appendYouTubeUploaderPage(current, incoming, continuation),
        );
      })
      .catch((error) => {
        if (paginationRequestRef.current !== requestID) return;
        setPaginationFailed(true);
        props.onError?.(error);
      })
      .finally(() => {
        if (paginationRequestRef.current !== requestID) return;
        loadingMoreRef.current = false;
        setLoadingMore(false);
      });
  }, [channelId, page, props.locale, props.onError]);

  const toggleSubscription = React.useCallback(() => {
    if (!page || page.channelId.trim() !== channelId || subscriptionBusy) return;
    const identity = youtubeSubscriptionIdentity(page.channelId);
    const request = subscriptionGateRef.current.begin(identity);
    const subscribed = !page.isSubscribed;
    setSubscriptionBusy(true);
    setFailed(false);
    void setYouTubeWorkspaceChannelSubscription(page.channelId, subscribed)
      .then(() => {
        if (!subscriptionGateRef.current.isCurrent(request)) return;
        setPage((current) =>
          current?.channelId === page.channelId
            ? { ...current, isSubscribed: subscribed }
            : current,
        );
        props.onSubscriptionChange?.(page.channelId, subscribed);
      })
      .catch((error) => {
        if (!subscriptionGateRef.current.isCurrent(request)) return;
        setFailed(true);
        props.onError?.(error);
      })
      .finally(() => {
        if (subscriptionGateRef.current.isCurrent(request)) {
          setSubscriptionBusy(false);
        }
      });
  }, [channelId, page, props.onError, props.onSubscriptionChange, subscriptionBusy]);

  if (!page || page.channelId.trim() !== channelId) {
    return (
      <section
        className="youtube-uploader-page wails-drag"
        data-youtube-uploader={channelId}
      >
        <h1 className="sr-only">
          {props.fallbackName || props.labels.fallbackChannel}
        </h1>
        <UploaderBackButton label={props.labels.back} onBack={props.onBack} />
        <div className="youtube-uploader-page-state" role={failed ? "alert" : "status"}>
          {failed ? <RefreshCw aria-hidden="true" /> : <LoaderCircle className="app-motion-spin" aria-hidden="true" />}
          <p>{failed ? props.labels.error : props.labels.loading}</p>
          {failed ? (
            <Button
              type="button"
              variant="outline"
              className="wails-no-drag"
              onClick={() => setReloadToken((value) => value + 1)}
            >
              {props.labels.retry}
            </Button>
          ) : null}
        </div>
      </section>
    );
  }

  return (
    <YouTubeUploaderContent
      page={page}
      locale={props.locale}
      labels={props.labels}
      failed={failed}
      paginationFailed={paginationFailed}
      loadingMore={loadingMore}
      subscriptionBusy={subscriptionBusy}
      onBack={props.onBack}
      onOpenVideo={props.onOpenVideo}
      onToggleSubscription={toggleSubscription}
      onLoadMore={loadMore}
    />
  );
}

export function YouTubeUploaderContent({
  page,
  locale,
  labels,
  failed = false,
  paginationFailed = false,
  loadingMore = false,
  subscriptionBusy = false,
  onBack,
  onOpenVideo,
  onToggleSubscription,
  onLoadMore,
}: {
  page: YouTubeUploaderPageData;
  locale: string;
  labels: YouTubeUploaderPageLabels;
  failed?: boolean;
  paginationFailed?: boolean;
  loadingMore?: boolean;
  subscriptionBusy?: boolean;
  onBack: () => void;
  onOpenVideo: (video: YouTubeWorkspaceVideo, queue: YouTubeWorkspaceVideo[]) => void;
  onToggleSubscription: () => void;
  onLoadMore: () => void;
}) {
  const subscriberLabel =
    page.subscriberLabel ||
    (typeof page.subscriberCount === "number"
      ? formatYouTubeViewCount(page.subscriberCount, locale)
      : "");
  const metadata = [page.handle, subscriberLabel, page.videoCountLabel].filter(
    (value): value is string => Boolean(value),
  );
  const subscribeLabel = page.isSubscribed ? labels.unsubscribe : labels.subscribe;

  return (
    <section className="youtube-uploader-page" data-youtube-uploader={page.channelId}>
      <header className="youtube-uploader-hero wails-drag">
        <div className="youtube-uploader-banner" aria-hidden="true">
          <YouTubeImage
            source={page.bannerUrl}
            alt=""
            draggable={false}
            fallback={<span className="youtube-uploader-banner-fallback"><Youtube /></span>}
          />
        </div>
        <UploaderBackButton label={labels.back} onBack={onBack} />
        <div className="youtube-uploader-identity">
          <div className="youtube-uploader-avatar" aria-label={page.name}>
            <YouTubeImage
              source={page.avatarUrl}
              alt=""
              draggable={false}
              fallback={<span><Youtube /></span>}
            />
          </div>
          <div className="youtube-uploader-name-row">
            <h1>{page.name}</h1>
            <YouTubeSubscriptionIconButton
              subscribed={page.isSubscribed === true}
              busy={subscriptionBusy}
              label={subscribeLabel}
              className="youtube-uploader-subscribe"
              onClick={onToggleSubscription}
            />
          </div>
          {metadata.length > 0 ? (
            <p className="youtube-uploader-metadata">
              {metadata.map((item) => <span key={item}>{item}</span>)}
            </p>
          ) : null}
          {page.description ? (
            <p className="youtube-uploader-description" title={page.description}>
              {page.description}
            </p>
          ) : null}
          <YouTubeUploaderInfoDialog
            page={page}
            metadata={metadata}
            labels={labels}
          />
        </div>
      </header>

      <div className="youtube-uploader-videos">
        <h2>{labels.videos}</h2>
        {page.videos.length > 0 ? (
          <div className="youtube-uploader-video-grid">
            {page.videos.map((video) => (
              <UploaderVideoCard
                key={video.videoId}
                video={video}
                locale={locale}
                fallbackChannel={page.name || labels.fallbackChannel}
                onOpen={() => onOpenVideo(video, page.videos)}
              />
            ))}
          </div>
        ) : (
          <div className="youtube-uploader-empty">
            <Youtube aria-hidden="true" />
            <p>{labels.empty}</p>
          </div>
        )}
        {failed ? (
          <p className="youtube-uploader-inline-error" role="alert">
            {labels.error}
          </p>
        ) : null}
        {page.continuation && !paginationFailed ? (
          <ListenInfiniteScrollSentinel
            key={page.channelId}
            continuation={page.continuation}
            enabled={!loadingMore}
            loading={loadingMore}
            onLoadMore={onLoadMore}
            className="youtube-uploader-infinite-sentinel"
          />
        ) : null}
        {loadingMore ? (
          <div
            className="youtube-uploader-pagination-state"
            role="status"
            aria-label={labels.loading}
          >
            <LoaderCircle className="app-motion-spin" aria-hidden="true" />
            <span>{labels.loading}</span>
          </div>
        ) : paginationFailed ? (
          <div className="youtube-uploader-pagination-state youtube-uploader-pagination-error">
            <p role="alert">{labels.error}</p>
            <Button type="button" variant="ghost" size="compact" onClick={onLoadMore}>
              <RefreshCw aria-hidden="true" />
              {labels.retry}
            </Button>
          </div>
        ) : null}
      </div>
    </section>
  );
}

function YouTubeUploaderInfoDialog({
  page,
  metadata,
  labels,
}: {
  page: YouTubeUploaderPageData;
  metadata: string[];
  labels: YouTubeUploaderPageLabels;
}) {
  const descriptionID = React.useId();

  return (
    <Dialog>
      <DialogTrigger asChild>
        <Button
          type="button"
          variant="ghost"
          size="compact"
          shape="capsule"
          className="youtube-uploader-more wails-no-drag"
        >
          <MoreHorizontal aria-hidden="true" />
          <span>{labels.more}</span>
        </Button>
      </DialogTrigger>

      <DialogContent
        className="youtube-uploader-info-dialog"
        showCloseButton={false}
      >
        <div className="youtube-uploader-info-dialog-hero">
          <span
            className="youtube-uploader-info-dialog-hero-fallback"
            aria-hidden="true"
          >
            <Youtube />
          </span>
          <YouTubeImage
            source={page.bannerUrl || page.avatarUrl}
            alt=""
            draggable={false}
          />
          <span
            className="youtube-uploader-info-dialog-hero-veil"
            aria-hidden="true"
          />
          <DialogClose asChild>
            <Button
              type="button"
              variant="glass"
              size="compactIcon"
              shape="circle"
              className="youtube-uploader-info-dialog-close"
              aria-label={labels.close}
              title={labels.close}
            >
              <X aria-hidden="true" />
            </Button>
          </DialogClose>
        </div>

        <DialogScrollArea className="youtube-uploader-info-dialog-scroll">
          <div className="youtube-uploader-info-dialog-avatar" aria-hidden="true">
            <YouTubeImage
              source={page.avatarUrl}
              alt=""
              draggable={false}
              fallback={<span><Youtube /></span>}
            />
          </div>
          <header className="youtube-uploader-info-dialog-identity">
            <DialogTitle>{page.name}</DialogTitle>
            {metadata.length > 0 ? (
              <DialogDescription>{metadata.join(" · ")}</DialogDescription>
            ) : (
              <DialogDescription className="sr-only">
                {labels.more}
              </DialogDescription>
            )}
          </header>
          {page.description ? (
            <section
              className="youtube-uploader-info-dialog-description"
              aria-labelledby={descriptionID}
            >
              <h2 id={descriptionID}>
                <Info aria-hidden="true" />
                {labels.description}
              </h2>
              <p>{page.description}</p>
            </section>
          ) : null}
        </DialogScrollArea>
      </DialogContent>
    </Dialog>
  );
}

function UploaderBackButton({ label, onBack }: { label: string; onBack: () => void }) {
  return (
    <Button type="button" variant="glass" size="compactIcon" shape="circle" className="youtube-uploader-back wails-no-drag" aria-label={label} title={label} onClick={onBack}>
      <ArrowLeft aria-hidden="true" />
    </Button>
  );
}

function UploaderVideoCard({
  video,
  locale,
  fallbackChannel,
  onOpen,
}: {
  video: YouTubeWorkspaceVideo;
  locale: string;
  fallbackChannel: string;
  onOpen: () => void;
}) {
  const details = [
    video.channel || fallbackChannel,
    typeof video.viewCount === "number"
      ? formatYouTubeViewCount(video.viewCount, locale)
      : "",
    video.publishedLabel || "",
  ].filter(Boolean);
  return (
    <button type="button" className="youtube-uploader-video-card" onClick={onOpen}>
      <span className="youtube-uploader-video-thumbnail">
        <YouTubeImage
          source={video.thumbnailUrl}
          videoId={video.videoId}
          alt=""
          loading="lazy"
          draggable={false}
          fallback={<span><Youtube /></span>}
        />
        <span className="youtube-uploader-video-play" aria-hidden="true"><Play /></span>
        {video.durationLabel ? <em>{video.durationLabel}</em> : null}
      </span>
      <span className="youtube-uploader-video-details">
        <strong>{video.title}</strong>
        {details.length > 0 ? <small>{details.join(" · ")}</small> : null}
      </span>
    </button>
  );
}
