import { LoaderCircle, Play, Youtube } from "lucide-react";

import { formatYouTubeViewCount } from "@/app/youtube/page-state";
import type { YouTubeWorkspaceVideo } from "@/app/youtube/types";
import { YouTubeImage } from "@/app/youtube/YouTubeImage";

export function YouTubeUpNextCompanion({
  queue,
  currentIndex,
  currentVideoId,
  openingVideoId,
  locale,
  title,
  emptyLabel,
  fallbackChannel,
  onOpenVideo,
}: {
  queue: YouTubeWorkspaceVideo[];
  currentIndex: number;
  currentVideoId: string;
  openingVideoId: string;
  locale: string;
  title: string;
  emptyLabel: string;
  fallbackChannel: string;
  onOpenVideo: (video: YouTubeWorkspaceVideo) => void;
}) {
  const resolvedIndex =
    currentIndex >= 0 && queue[currentIndex]?.videoId === currentVideoId
      ? currentIndex
      : queue.findIndex((item) => item.videoId === currentVideoId);
  const upcoming = queue.filter(
    (item, index) =>
      index > resolvedIndex &&
      item.videoId !== currentVideoId &&
      isPlayableYouTubeVideo(item),
  );

  return (
    <section
      className="youtube-up-next-companion"
      data-youtube-companion="up-next"
      aria-label={title}
    >
      <div
        className="youtube-up-next-companion-list"
        data-companion-scroll-owner="youtube-up-next"
      >
        {upcoming.length > 0 ? (
          upcoming.map((video, index) => (
            <button
              type="button"
              className="youtube-up-next-row"
              key={video.videoId}
              disabled={openingVideoId === video.videoId}
              onClick={() => onOpenVideo(video)}
            >
              <span className="youtube-up-next-row-index" aria-hidden="true">
                {index + 1}
              </span>
              <span className="youtube-up-next-row-thumbnail">
                <YouTubeImage
                  source={video.thumbnailUrl}
                  videoId={video.videoId}
                  alt=""
                  loading="lazy"
                  fallback={<Youtube />}
                />
                <span className="youtube-up-next-row-play" aria-hidden="true">
                  {openingVideoId === video.videoId ? (
                    <LoaderCircle className="app-motion-spin" />
                  ) : (
                    <Play />
                  )}
                </span>
                {video.durationLabel ? <em>{video.durationLabel}</em> : null}
              </span>
              <span className="youtube-up-next-row-details">
                <strong>{video.title}</strong>
                <small>{video.channel || fallbackChannel}</small>
                {video.viewCount ? (
                  <small>{formatYouTubeViewCount(video.viewCount, locale)}</small>
                ) : null}
              </span>
            </button>
          ))
        ) : (
          <div className="youtube-up-next-companion-empty">
            <span aria-hidden="true"><Youtube /></span>
            <p>{emptyLabel}</p>
          </div>
        )}
      </div>
    </section>
  );
}

function isPlayableYouTubeVideo(video: YouTubeWorkspaceVideo) {
  return (
    video.itemKind !== "playlist" &&
    /^[A-Za-z0-9_-]{11}$/.test(video.videoId.trim())
  );
}
