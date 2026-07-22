import {
  Archive,
  BookOpen,
  Braces,
  Captions,
  File,
  FileQuestion,
  FileText,
  Image as ImageIcon,
  ListTodo,
  Music2,
  Type,
  Video,
  type LucideIcon,
} from "lucide-react";
import * as React from "react";

import { cn } from "@/lib/utils";
import { COMPLETED_DEFAULT_COVER_IMAGE_URLS } from "@/shared/assets/default-cover";

import {
  LIBRARY_PAPER_GEOMETRY,
  LIBRARY_PAPER_NOTCH_POSITIONS,
  PLACEHOLDER_PAPER_HORIZONTAL_NOTCH_OFFSETS,
  PLACEHOLDER_PAPER_TRANSFORM,
} from "./library-paper-geometry";
import type {
  LibraryOtherGroup,
  LibraryWorkspaceItem,
} from "./types";

const DEFAULT_ARTWORK_URLS = new Set<string>(
  Object.values(COMPLETED_DEFAULT_COVER_IMAGE_URLS),
);
const SVG_USER_SPACE = "userSpaceOnUse";
const SVG_PLACEHOLDER_ASPECT_RATIO = "xMidYMid meet";

const CATEGORY_ICONS = {
  task: ListTodo,
  video: Video,
  audio: Music2,
  book: BookOpen,
  image: ImageIcon,
  other: File,
} satisfies Record<LibraryWorkspaceItem["category"], LucideIcon>;

const OTHER_GROUP_ICONS: Partial<Record<LibraryOtherGroup, LucideIcon>> = {
  document: FileText,
  font: Type,
  archive: Archive,
  subtitle: Captions,
  manifest: FileText,
  api: Braces,
  unknown: FileQuestion,
  "needs-review": FileQuestion,
  missing: FileQuestion,
};

export function LibraryOtherGroupIcon(props: {
  group: LibraryOtherGroup;
  className?: string;
}) {
  const Icon = OTHER_GROUP_ICONS[props.group] ?? CATEGORY_ICONS.other;
  return (
    <Icon
      className={props.className}
      aria-hidden="true"
      strokeWidth={1.5}
    />
  );
}

export function isLibraryDefaultArtworkURL(value?: string) {
  const normalized = value?.trim().split(/[?#]/, 1)[0] ?? "";
  return normalized.length > 0 && DEFAULT_ARTWORK_URLS.has(normalized);
}

function artworkKind(
  category: LibraryWorkspaceItem["category"],
  otherGroup?: LibraryOtherGroup,
) {
  return category === "other" && otherGroup ? otherGroup : category;
}

export interface LibraryArtworkProps {
  src?: string;
  fallbackSrc?: string;
  category: LibraryWorkspaceItem["category"];
  otherGroup?: LibraryOtherGroup;
  alt: string;
  className?: string;
}

/**
 * Theme-aware Library artwork boundary. Real artwork remains an image; old
 * bitmap defaults are represented by a tiny runtime SVG and CSS surface so
 * placeholders follow every theme without loading a 640px decorative asset.
 */
export function LibraryArtwork(props: LibraryArtworkProps) {
  const reactId = React.useId();
  const paperMaskId = `app-library-placeholder-paper-${reactId.replace(/[^a-zA-Z0-9_-]/g, "")}`;
  const [fallbackActive, setFallbackActive] = React.useState(false);
  React.useEffect(() => setFallbackActive(false), [props.src, props.fallbackSrc]);

  const primary = props.src?.trim() ?? "";
  const fallback = props.fallbackSrc?.trim() ?? "";
  const source = (!primary || fallbackActive) && fallback ? fallback : primary;
  const isPlaceholder = !source || isLibraryDefaultArtworkURL(source);
  const Icon = props.category === "other" && props.otherGroup
    ? OTHER_GROUP_ICONS[props.otherGroup] ?? CATEGORY_ICONS.other
    : CATEGORY_ICONS[props.category];
  const paper = LIBRARY_PAPER_GEOMETRY;

  if (isPlaceholder) {
    return (
      <span
        className={cn(
          "app-library-artwork app-library-artwork--placeholder",
          props.className,
        )}
        data-artwork-kind={artworkKind(props.category, props.otherGroup)}
        role={props.alt ? "img" : undefined}
        aria-label={props.alt || undefined}
        aria-hidden={props.alt ? undefined : "true"}
      >
        <svg
          className="app-library-artwork__placeholder-paper"
          viewBox="0 0 100 120"
          preserveAspectRatio={SVG_PLACEHOLDER_ASPECT_RATIO}
          aria-hidden="true"
          focusable="false"
        >
          <defs>
            <mask
              id={paperMaskId}
              maskUnits={SVG_USER_SPACE}
              maskContentUnits={SVG_USER_SPACE}
              x={paper.x - 2}
              y={paper.y - 2}
              width={paper.width + 4}
              height={paper.height + 4}
            >
              <rect
                x={paper.x}
                y={paper.y}
                width={paper.width}
                height={paper.height}
                rx={paper.radius}
                fill="white"
              />
              {LIBRARY_PAPER_NOTCH_POSITIONS.map((position) => (
                <React.Fragment key={`vertical-${position}`}>
                  <circle
                    cx={paper.x}
                    cy={paper.y + position}
                    r="1.25"
                    fill="black"
                  />
                  <circle
                    cx={paper.x + paper.width}
                    cy={paper.y + position}
                    r="1.25"
                    fill="black"
                  />
                </React.Fragment>
              ))}
              {PLACEHOLDER_PAPER_HORIZONTAL_NOTCH_OFFSETS.map((offset) => (
                <React.Fragment key={`horizontal-${offset}`}>
                  <circle
                    cx={paper.x + offset}
                    cy={paper.y}
                    r="1.25"
                    fill="black"
                  />
                  <circle
                    cx={paper.x + offset}
                    cy={paper.y + paper.height}
                    r="1.25"
                    fill="black"
                  />
                </React.Fragment>
              ))}
            </mask>
          </defs>
          <g
            transform={PLACEHOLDER_PAPER_TRANSFORM}
            mask={`url(#${paperMaskId})`}
          >
            <rect
              className="app-library-artwork__placeholder-paper-face"
              x={paper.x}
              y={paper.y}
              width={paper.width}
              height={paper.height}
              rx={paper.radius}
            />
            <rect
              className="app-library-artwork__placeholder-paper-border"
              x={paper.x}
              y={paper.y}
              width={paper.width}
              height={paper.height}
              rx={paper.radius}
            />
          </g>
        </svg>
        <Icon
          className="app-library-artwork__placeholder-icon"
          aria-hidden="true"
          strokeWidth={1.35}
        />
      </span>
    );
  }

  return (
    <img
      src={source}
      alt={props.alt}
      className={cn("app-library-artwork", props.className)}
      loading="lazy"
      decoding="async"
      onError={() => {
        if (!fallbackActive && fallback && fallback !== primary) {
          setFallbackActive(true);
        }
      }}
    />
  );
}
