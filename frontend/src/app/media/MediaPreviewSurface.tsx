import * as React from "react";
import { FileText, ImageIcon, Loader2, Music, Radio, Video } from "lucide-react";

import { FlvPreview } from "@/app/media/FlvPreview";
import { VidstackPreview, type VidstackPreviewLabels, type VidstackPreviewProps } from "@/app/media/VidstackPreview";
import { cn } from "@/lib/utils";

export type MediaPreviewKind =
  | "audio"
  | "flv"
  | "image"
  | "live"
  | "subtitle"
  | "unsupported"
  | "video";

export type MediaPreviewSurfaceLabels = VidstackPreviewLabels & {
  loading?: string;
  unsupported?: string;
};

export type MediaPreviewSurfaceProps = {
  kind: MediaPreviewKind;
  labels: MediaPreviewSurfaceLabels;
  mediaUrl?: string;
  title?: string;
  className?: string;
  contentClassName?: string;
  imageAlt?: string;
  imageClassName?: string;
  loading?: boolean;
  loaded?: boolean;
  posterUrl?: string;
  durationMs?: number;
  persistKey?: string;
  persistProgress?: boolean;
  sourceType?: VidstackPreviewProps["sourceType"];
  streamType?: VidstackPreviewProps["streamType"];
  videoClassName?: string;
  audioPreview?: React.ReactNode;
  subtitlePreview?: React.ReactNode;
  onImageLoad?: () => void;
  onImageError?: () => void;
  onPresentationModeChange?: VidstackPreviewProps["onPresentationModeChange"];
};

function MediaPreviewPlaceholder(props: {
  kind: MediaPreviewKind;
  label: string;
  loading?: boolean;
}) {
  const Icon =
    props.kind === "live" || props.kind === "flv"
      ? Radio
      : props.kind === "video"
        ? Video
        : props.kind === "audio"
          ? Music
          : props.kind === "subtitle"
            ? FileText
            : ImageIcon;

  return (
    <div className="flex flex-col items-center gap-2 text-xs font-medium text-muted-foreground">
      {props.loading ? (
        <Loader2 className="h-5 w-5 animate-spin text-primary" />
      ) : (
        <Icon className="h-5 w-5" />
      )}
      <span>{props.label}</span>
    </div>
  );
}

export function MediaPreviewSurface(props: MediaPreviewSurfaceProps) {
  const mediaUrl = props.mediaUrl?.trim() ?? "";
  const title = props.title?.trim() || props.labels.noPreview;
  const isLive = props.kind === "live";
  const loaded =
    props.loaded ??
    Boolean(
      !props.loading &&
        ((props.kind === "image" && mediaUrl) ||
          ((props.kind === "video" ||
            props.kind === "live" ||
            props.kind === "flv") &&
            mediaUrl) ||
          (props.kind === "audio" && props.audioPreview) ||
          (props.kind === "subtitle" && props.subtitlePreview)),
    );

  let content: React.ReactNode;

  if (props.loading) {
    content = (
      <MediaPreviewPlaceholder
        kind={props.kind}
        label={props.labels.loading || props.labels.noPreview}
        loading
      />
    );
  } else if (props.kind === "flv" && mediaUrl) {
    content = (
      <div
        className={cn(
          "h-full min-h-0 w-full min-w-0",
          props.videoClassName,
        )}
      >
        <FlvPreview
          labels={props.labels}
          mediaUrl={mediaUrl}
          title={title}
          posterUrl={props.posterUrl}
          streamType={props.streamType === "live" ? "live" : "on-demand"}
          onPresentationModeChange={props.onPresentationModeChange}
        />
      </div>
    );
  } else if ((props.kind === "video" || props.kind === "live") && mediaUrl) {
    content = (
      <div
        className={cn(
          "h-full min-h-0 w-full min-w-0",
          props.videoClassName,
        )}
      >
        <VidstackPreview
          labels={props.labels}
          mediaUrl={mediaUrl}
          title={title}
          persistKey={props.persistKey}
          posterUrl={props.posterUrl}
          durationMs={props.durationMs}
          streamType={props.streamType ?? (isLive ? "live" : "on-demand")}
          sourceType={props.sourceType}
          persistProgress={props.persistProgress ?? (isLive ? false : undefined)}
          onPresentationModeChange={props.onPresentationModeChange}
        />
      </div>
    );
  } else if (props.kind === "image" && mediaUrl) {
    content = (
      <img
        src={mediaUrl}
        alt={props.imageAlt ?? title}
        className={cn(
          "app-media-preview-image app-completed-preview-image max-h-full w-full object-contain",
          props.imageClassName,
        )}
        draggable={false}
        onLoad={props.onImageLoad}
        onError={props.onImageError}
      />
    );
  } else if (props.kind === "audio" && props.audioPreview) {
    content = props.audioPreview;
  } else if (props.kind === "subtitle" && props.subtitlePreview) {
    content = props.subtitlePreview;
  } else {
    content = (
      <MediaPreviewPlaceholder
        kind={props.kind}
        label={props.labels.unsupported || props.labels.noPreview}
      />
    );
  }

  return (
    <div
      className={cn(
        "app-media-preview-surface flex min-h-[16rem] items-center justify-center overflow-hidden",
        props.className,
      )}
      data-kind={props.kind}
      data-loaded={loaded ? "true" : "false"}
    >
      <div
        className={cn(
          "app-media-preview-content flex h-full min-h-0 w-full min-w-0 items-center justify-center",
          props.contentClassName,
        )}
      >
        {content}
      </div>
    </div>
  );
}
