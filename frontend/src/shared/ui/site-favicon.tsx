import { Globe2 } from "lucide-react";
import * as React from "react";

export type SiteFaviconProps = Omit<
  React.ImgHTMLAttributes<HTMLImageElement>,
  "onError" | "src"
> & {
  source?: string | null;
  fallback?: React.ReactNode;
};

/**
 * Renders a resolved site favicon and falls back when the source is empty or
 * cannot be decoded. A failed source is remembered without preventing a later
 * source from being tried.
 */
export function SiteFavicon({
  source,
  fallback,
  alt = "",
  ...imageProps
}: SiteFaviconProps) {
  const normalizedSource = source?.trim() ?? "";
  const [failedSource, setFailedSource] = React.useState("");

  if (!normalizedSource || failedSource === normalizedSource) {
    return fallback ?? <Globe2 aria-hidden="true" />;
  }

  return (
    <img
      {...imageProps}
      alt={alt}
      src={normalizedSource}
      onError={() => setFailedSource(normalizedSource)}
    />
  );
}
