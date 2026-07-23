import { Music2 } from "lucide-react";
import * as React from "react";

import { cn } from "@/lib/utils";
import {
  isListenDefaultCoverImageURL,
  LISTEN_DEFAULT_COVER_IMAGE_URL,
} from "@/shared/assets/default-cover";

export function isListenCoverImageReady(
  image: Pick<HTMLImageElement, "complete" | "naturalWidth"> | null,
) {
  return Boolean(image?.complete && image.naturalWidth > 0);
}

export function ListenDefaultCoverArtwork(props: {
  alt?: string;
  className?: string;
}) {
  const alt = props.alt?.trim() ?? "";
  return (
    <span
      data-listen-default-cover="music"
      role={alt ? "img" : undefined}
      aria-label={alt || undefined}
      aria-hidden={alt ? undefined : "true"}
      className={cn(
        "listen-default-cover-artwork",
        props.className,
      )}
    >
      <span
        aria-hidden="true"
        className="listen-default-cover-artwork__wash"
      />
      <span
        aria-hidden="true"
        className="listen-default-cover-artwork__orb"
      />
      <Music2
        aria-hidden="true"
        className="listen-default-cover-artwork__icon"
      />
    </span>
  );
}

export function ListenCoverArtwork(props: {
  alt?: string;
  candidates?: readonly string[];
  className?: string;
  imageClassName?: string;
  loading?: React.ImgHTMLAttributes<HTMLImageElement>["loading"];
  decoding?: React.ImgHTMLAttributes<HTMLImageElement>["decoding"];
  draggable?: boolean;
  changeSweep?: boolean;
  softenOnHover?: boolean;
  softenClassName?: string;
}) {
  const candidateKey = (props.candidates ?? []).join("\n");
  const candidates = React.useMemo(() => {
    const values = (props.candidates ?? [])
      .map((value) => value.trim())
      .filter(Boolean)
      .filter((value) => !isListenDefaultCoverImageURL(value));
    return Array.from(new Set([...values, LISTEN_DEFAULT_COVER_IMAGE_URL]));
  }, [candidateKey]);
  const [candidateSelection, setCandidateSelection] = React.useState({
    key: "",
    index: 0,
  });
  const candidateIndex =
    candidateSelection.key === candidateKey ? candidateSelection.index : 0;
  const [readySource, setReadySource] = React.useState("");
  const source =
    candidates[
      Math.min(candidateIndex, Math.max(0, candidates.length - 1))
    ] ?? LISTEN_DEFAULT_COVER_IMAGE_URL;
  const imageReady = !isListenDefaultCoverImageURL(source) && readySource === source;

  const handleImageRef = React.useCallback(
    (image: HTMLImageElement | null) => {
      // WKWebView can satisfy a newly assigned cached URL before React sees a
      // load event. Inspecting `complete` when the source node is committed
      // keeps an asynchronously enriched Now Playing cover from remaining on
      // the fallback until another UI action remounts the artwork.
      if (isListenCoverImageReady(image)) {
        setReadySource(source);
      }
    },
    [source],
  );

  return (
    <span
      className={cn("listen-cover-artwork", props.className)}
      data-listen-cover-artwork="true"
      data-artwork-ready={imageReady ? "true" : "false"}
    >
      {!imageReady ? (
        <ListenDefaultCoverArtwork
          alt={isListenDefaultCoverImageURL(source) ? props.alt : ""}
          className="listen-cover-artwork__fallback"
        />
      ) : null}
      {!isListenDefaultCoverImageURL(source) ? (
        <>
          <img
            ref={handleImageRef}
            key={source}
            src={source}
            alt={props.alt ?? ""}
            className={cn(
              "listen-cover-artwork__image",
              props.imageClassName,
            )}
            loading={props.loading ?? "eager"}
            decoding={props.decoding}
            draggable={props.draggable}
            onLoad={() => setReadySource(source)}
            onError={() => {
              setReadySource("");
              setCandidateSelection((current) => {
                const currentIndex =
                  current.key === candidateKey ? current.index : 0;
                return {
                  key: candidateKey,
                  index:
                    currentIndex + 1 < candidates.length
                      ? currentIndex + 1
                      : currentIndex,
                };
              });
            }}
          />
          {(props.softenOnHover || props.softenClassName) && imageReady ? (
            <img
              src={source}
              alt=""
              aria-hidden="true"
              className={cn(
                "listen-hover-soften-image",
                props.softenClassName,
              )}
            />
          ) : null}
        </>
      ) : null}
      {props.changeSweep && imageReady ? (
        <span
          key={`cover-sweep-${source}`}
          className="listen-cover-change-sweep"
          aria-hidden="true"
        />
      ) : null}
    </span>
  );
}
