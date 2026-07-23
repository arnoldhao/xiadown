import { CalendarDays, Eye, ThumbsUp, X, Youtube } from "lucide-react";
import * as React from "react";

import {
  formatYouTubePublishedLabel,
  formatYouTubeViewCount,
} from "@/app/youtube/page-state";
import type { YouTubeVideoDetails } from "@/app/youtube/types";
import { YouTubeImage } from "@/app/youtube/YouTubeImage";
import { Button } from "@/shared/ui/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogScrollArea,
  DialogTitle,
} from "@/shared/ui/dialog";

export interface YouTubeVideoInfoLabels {
  title: string;
  description: string;
  descriptionUnavailable: string;
  published: string;
  views: string;
  likes: string;
  close: string;
}

export function YouTubeVideoInfoDialog({
  open,
  onOpenChange,
  details,
  fallbackTitle,
  fallbackChannel,
  fallbackThumbnail,
  locale,
  labels,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  details: YouTubeVideoDetails | null;
  fallbackTitle: string;
  fallbackChannel: string;
  fallbackThumbnail?: string;
  locale: string;
  labels: YouTubeVideoInfoLabels;
}) {
  const title = details?.title || fallbackTitle;
  const channel = details?.channel || fallbackChannel;
  const thumbnail = details?.thumbnailUrl || fallbackThumbnail || "";
  const published = formatYouTubePublishedLabel(
    details?.publishedLabel || details?.publishedDate || "",
    locale,
  );
  const description = details?.description?.trim() || "";
  const [thumbnailLoaded, setThumbnailLoaded] = React.useState(false);

  React.useEffect(() => {
    setThumbnailLoaded(false);
  }, [thumbnail]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        className="youtube-video-info-dialog"
        showCloseButton={false}
      >
        <div className="youtube-video-info-dialog-hero">
          <span className="youtube-video-info-dialog-hero-fallback" aria-hidden="true">
            <Youtube />
          </span>
          <YouTubeImage
            source={thumbnail}
            videoId={details?.videoId}
            alt=""
            draggable={false}
            data-loaded={thumbnailLoaded ? "true" : "false"}
            onLoad={() => setThumbnailLoaded(true)}
            onError={() => setThumbnailLoaded(false)}
          />
          <span className="youtube-video-info-dialog-hero-veil" aria-hidden="true" />
          <DialogClose asChild>
            <Button
              type="button"
              variant="glass"
              size="compactIcon"
              shape="circle"
              className="youtube-video-info-dialog-close"
              aria-label={labels.close}
              title={labels.close}
            >
              <X aria-hidden="true" />
            </Button>
          </DialogClose>
        </div>
        <header className="youtube-video-info-dialog-header">
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription className="youtube-video-info-dialog-channel">
            <Youtube aria-hidden="true" />
            <span>{channel}</span>
          </DialogDescription>
        </header>
        <DialogScrollArea className="youtube-video-info-dialog-scroll">
          <dl className="youtube-video-info-dialog-stats">
            {published ? (
              <div>
                <dt><CalendarDays aria-hidden="true" />{labels.published}</dt>
                <dd>{published}</dd>
              </div>
            ) : null}
            {details?.viewCount ? (
              <div>
                <dt><Eye aria-hidden="true" />{labels.views}</dt>
                <dd>{formatYouTubeViewCount(details.viewCount, locale)}</dd>
              </div>
            ) : null}
            {details?.likeCount ? (
              <div>
                <dt><ThumbsUp aria-hidden="true" />{labels.likes}</dt>
                <dd>{formatYouTubeViewCount(details.likeCount, locale)}</dd>
              </div>
            ) : null}
          </dl>
          <section className="youtube-video-info-dialog-description">
            <h3>{labels.description}</h3>
            <p>{description || labels.descriptionUnavailable}</p>
          </section>
        </DialogScrollArea>
      </DialogContent>
    </Dialog>
  );
}
