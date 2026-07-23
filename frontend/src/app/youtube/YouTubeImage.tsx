import * as React from "react";

export function buildYouTubeImageCandidates(
  source?: string,
  videoId?: string,
) {
  const candidates: string[] = [];
  const add = (value?: string) => {
    const trimmed = value?.trim() || "";
    if (!trimmed) {
      return;
    }
    const normalized = trimmed.startsWith("//") ? `https:${trimmed}` : trimmed;
    if (!candidates.includes(normalized)) {
      candidates.push(normalized);
    }
  };

  add(source);
  const normalizedVideoID = videoId?.trim() || "";
  if (/^[A-Za-z0-9_-]{11}$/.test(normalizedVideoID)) {
    add(`https://i.ytimg.com/vi_webp/${normalizedVideoID}/hqdefault.webp`);
    add(`https://i.ytimg.com/vi/${normalizedVideoID}/hqdefault.jpg`);
    add(`https://i.ytimg.com/vi/${normalizedVideoID}/mqdefault.jpg`);
    add(`https://i.ytimg.com/vi/${normalizedVideoID}/default.jpg`);
  }
  return candidates;
}

export function nextYouTubeImageCandidate(
  currentIndex: number,
  candidateCount: number,
) {
  const nextIndex = currentIndex + 1;
  return nextIndex < candidateCount ? nextIndex : null;
}

export function YouTubeImage({
  source,
  videoId,
  fallback = null,
  onLoad,
  ...props
}: Omit<React.ImgHTMLAttributes<HTMLImageElement>, "src"> & {
  source?: string;
  videoId?: string;
  fallback?: React.ReactNode;
}) {
  const candidates = React.useMemo(
    () => buildYouTubeImageCandidates(source, videoId),
    [source, videoId],
  );
  const candidateKey = candidates.join("\n");
  const [candidateIndex, setCandidateIndex] = React.useState(0);
  const [unavailable, setUnavailable] = React.useState(candidates.length === 0);

  React.useEffect(() => {
    setCandidateIndex(0);
    setUnavailable(candidates.length === 0);
  }, [candidateKey, candidates.length]);

  const current = unavailable ? "" : candidates[candidateIndex] || "";
  if (!current) {
    return <>{fallback}</>;
  }

  return (
    <img
      {...props}
      src={current}
      data-youtube-image-candidate={candidateIndex}
      onLoad={onLoad}
      onError={(event) => {
        props.onError?.(event);
        const nextIndex = nextYouTubeImageCandidate(
          candidateIndex,
          candidates.length,
        );
        if (nextIndex !== null) {
          setCandidateIndex(nextIndex);
          return;
        }
        setUnavailable(true);
      }}
    />
  );
}
