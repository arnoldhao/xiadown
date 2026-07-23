import {
  Archive,
  BookOpen,
  Braces,
  Captions,
  File,
  FileText,
  Image as ImageIcon,
  Music2,
  Type,
  Video,
  type LucideIcon,
} from "lucide-react";
import * as React from "react";

import { cn } from "@/lib/utils";

import { isLibraryDefaultArtworkURL } from "./LibraryArtwork";
import {
  LIBRARY_PAPER_GEOMETRY,
  LIBRARY_PAPER_NOTCH_POSITIONS,
  PLACEHOLDER_PAPER_HORIZONTAL_NOTCH_OFFSETS,
  TASK_FOLDER_PAPER_TRANSFORM,
} from "./library-paper-geometry";
import type { LibraryTaskPreviewItem } from "./types";

const MAX_GRID_PREVIEWS = 3;
const MAX_LIST_PREVIEWS = 1;
const MAX_COMPANION_OPEN_PREVIEWS = 2;
const SVG_FROST_COLOR_SPACE = "sRGB";
const SVG_FROST_SOURCE = "SourceGraphic";
const SVG_PREVIEW_ASPECT_RATIO = "xMidYMid slice";
const SVG_STAMP_ASPECT_RATIO = "xMidYMid meet";
const SVG_USER_SPACE = "userSpaceOnUse";
const SVG_FRONT_REGION_RESULT = "frontRegion";
const SVG_UNDER_COVER_RESULT = "underCover";
const SVG_CLEAR_REVEAL_RESULT = "clearReveal";

type CompanionStampPlacement = "inside" | "outside";

function taskPreviewImageURL(item?: LibraryTaskPreviewItem) {
  const previewURL = item?.previewURL?.trim() ?? "";
  return previewURL && !isLibraryDefaultArtworkURL(previewURL) ? previewURL : "";
}

function previewIcon(kind: string): LucideIcon {
  const normalized = kind.trim().toLocaleLowerCase();

  if (/video|movie|film|mp4|mkv|webm|mov/.test(normalized)) return Video;
  if (/audio|music|song|mp3|m4a|flac|wav|aac|opus/.test(normalized)) return Music2;
  if (/image|artwork|thumbnail|cover|photo|png|jpe?g|webp|gif|avif/.test(normalized)) {
    return ImageIcon;
  }
  if (/book|epub|mobi|azw/.test(normalized)) return BookOpen;
  if (/subtitle|caption|srt|vtt|ass/.test(normalized)) return Captions;
  if (/archive|zip|rar|7z|tar|gzip/.test(normalized)) return Archive;
  if (/font|woff|ttf|otf/.test(normalized)) return Type;
  if (/manifest|json|xml|yaml|api/.test(normalized)) return Braces;
  if (/document|text|pdf|doc|markdown|transcript/.test(normalized)) return FileText;
  return File;
}

function TaskFolderPreviewPage(props: { item: LibraryTaskPreviewItem }) {
  const [imageFailed, setImageFailed] = React.useState(false);
  const previewURL = taskPreviewImageURL(props.item);

  React.useEffect(() => setImageFailed(false), [previewURL]);

  const Icon = previewIcon(`${props.item.kind} ${props.item.label ?? ""}`);
  const showImage = Boolean(previewURL) && !imageFailed;

  return (
    <span
      className="app-library-task-folder__page"
      data-preview-kind={props.item.kind.trim().toLocaleLowerCase() || "file"}
    >
      {showImage ? (
        <img
          className="app-library-task-folder__page-image"
          src={previewURL}
          alt=""
          aria-hidden="true"
          draggable={false}
          loading="lazy"
          decoding="async"
          onError={() => setImageFailed(true)}
        />
      ) : (
        <span className="app-library-task-folder__page-fallback">
          <Icon aria-hidden="true" strokeWidth={1.45} />
        </span>
      )}
    </span>
  );
}

/**
 * Companion exposes each output as a complete postage stamp. This deliberately
 * uses the same paper geometry, four-edge notch recipe, and appearance hooks as
 * LibraryArtwork's semantic file stamp instead of approximating the edge in CSS.
 */
function TaskFolderPreviewStamp(props: {
  item?: LibraryTaskPreviewItem;
  placement: CompanionStampPlacement;
}) {
  const reactId = React.useId();
  const safeId = reactId.replace(/[^a-zA-Z0-9_-]/g, "");
  const stampMaskId = `app-task-folder-stamp-${safeId}`;
  const [imageFailed, setImageFailed] = React.useState(false);
  const previewURL = taskPreviewImageURL(props.item);
  const kind = props.item?.kind ?? "file";
  const label = props.item?.label ?? "";
  const Icon = previewIcon(`${kind} ${label}`);
  const showImage = Boolean(previewURL) && !imageFailed;
  const paper = LIBRARY_PAPER_GEOMETRY;
  const paperX = 2;
  const paperY = 2;
  const imageX = paperX + paper.inset;
  const imageY = paperY + paper.inset;
  const imageWidth = paper.width - paper.inset * 2;
  const imageHeight = paper.height - paper.inset * 2;

  React.useEffect(() => setImageFailed(false), [previewURL]);

  // The outside slot is reserved for a decodable real preview. If loading
  // fails, removing the stamp is less misleading than replacing the focal
  // artwork with a generic type page.
  if (props.placement === "outside" && (!previewURL || imageFailed)) return null;

  return (
    <span
      className={cn(
        "app-library-task-folder__page",
        `app-library-task-folder__page--${props.placement}`,
        props.item ? undefined : "app-library-task-folder__page--empty",
      )}
      data-placement={props.placement}
      data-preview-kind={kind.trim().toLocaleLowerCase() || "file"}
      data-preview-source={showImage ? "asset" : "fallback"}
      data-stamp-recipe="library-paper"
    >
      <svg
        className="app-library-task-folder__page-stamp"
        viewBox={`0 0 ${paper.width + 4} ${paper.height + 4}`}
        preserveAspectRatio={SVG_STAMP_ASPECT_RATIO}
        aria-hidden="true"
        focusable="false"
      >
        <defs>
          <mask
            id={stampMaskId}
            maskUnits={SVG_USER_SPACE}
            maskContentUnits={SVG_USER_SPACE}
            x="0"
            y="0"
            width={paper.width + 4}
            height={paper.height + 4}
          >
            <rect
              x={paperX}
              y={paperY}
              width={paper.width}
              height={paper.height}
              rx={paper.radius}
              fill="white"
            />
            {LIBRARY_PAPER_NOTCH_POSITIONS.map((position) => (
              <React.Fragment key={`vertical-${position}`}>
                <circle
                  cx={paperX}
                  cy={paperY + position}
                  r="1.25"
                  fill="black"
                />
                <circle
                  cx={paperX + paper.width}
                  cy={paperY + position}
                  r="1.25"
                  fill="black"
                />
              </React.Fragment>
            ))}
            {PLACEHOLDER_PAPER_HORIZONTAL_NOTCH_OFFSETS.map((offset) => (
              <React.Fragment key={`horizontal-${offset}`}>
                <circle
                  cx={paperX + offset}
                  cy={paperY}
                  r="1.25"
                  fill="black"
                />
                <circle
                  cx={paperX + offset}
                  cy={paperY + paper.height}
                  r="1.25"
                  fill="black"
                />
              </React.Fragment>
            ))}
          </mask>
        </defs>
        <g mask={`url(#${stampMaskId})`}>
          <rect
            className="app-library-artwork__placeholder-paper-face app-library-task-folder__stamp-face"
            x={paperX}
            y={paperY}
            width={paper.width}
            height={paper.height}
            rx={paper.radius}
          />
          {showImage ? (
            <image
              className="app-library-task-folder__page-image"
              href={previewURL}
              x={imageX}
              y={imageY}
              width={imageWidth}
              height={imageHeight}
              preserveAspectRatio={SVG_PREVIEW_ASPECT_RATIO}
              onError={() => setImageFailed(true)}
            />
          ) : (
            <Icon
              className="app-library-task-folder__stamp-fallback-icon"
              x={paperX + paper.width * 0.34}
              y={paperY + paper.height * 0.39}
              width={paper.width * 0.32}
              height={paper.height * 0.22}
              aria-hidden="true"
              strokeWidth={1.45}
            />
          )}
          <rect
            className="app-library-artwork__placeholder-paper-border app-library-task-folder__stamp-border"
            x={paperX}
            y={paperY}
            width={paper.width}
            height={paper.height}
            rx={paper.radius}
          />
        </g>
      </svg>
    </span>
  );
}

export interface TaskFolderArtworkProps {
  items: readonly LibraryTaskPreviewItem[];
  /** Full output count when `items` is already a bounded preview projection. */
  totalCount?: number;
  view?: "grid" | "list";
  presentation?: "closed" | "companion-open";
  className?: string;
}

/**
 * A decorative, static folder preview for Task cards. Opening the card remains
 * the responsibility of the surrounding Library item button and Companion.
 */
export function TaskFolderArtwork({
  items,
  totalCount,
  view = "grid",
  presentation = "closed",
  className,
}: TaskFolderArtworkProps) {
  const reactId = React.useId();
  const safeId = reactId.replace(/[^a-zA-Z0-9_-]/g, "");
  const frostFilterId = `app-task-folder-frost-${safeId}`;
  const sheetMaskId = `app-task-folder-sheet-${safeId}`;
  const paper = LIBRARY_PAPER_GEOMETRY;
  const imageX = paper.x + paper.inset;
  const imageY = paper.y + paper.inset;
  const imageWidth = paper.width - paper.inset * 2;
  const imageHeight = paper.height - paper.inset * 2;
  const previewLimit = presentation === "companion-open"
    ? MAX_COMPANION_OPEN_PREVIEWS
    : view === "list"
      ? MAX_LIST_PREVIEWS
      : MAX_GRID_PREVIEWS;
  const previewItems = items
    .map((item, index) => ({ item, index }))
    .sort((left, right) => {
      const leftHasImage = Boolean(taskPreviewImageURL(left.item));
      const rightHasImage = Boolean(taskPreviewImageURL(right.item));
      if (leftHasImage !== rightHasImage) return leftHasImage ? -1 : 1;
      return left.index - right.index;
    })
    .slice(0, previewLimit)
    .map(({ item }) => item);
  const resolvedTotal = Math.max(items.length, totalCount ?? 0);
  const unifiedPreviewItem = previewItems.find((item) => taskPreviewImageURL(item));
  const frostedPreviewURL = taskPreviewImageURL(unifiedPreviewItem);
  const [frostedPreviewFailed, setFrostedPreviewFailed] = React.useState(false);
  const showUnifiedPreview = Boolean(frostedPreviewURL) && !frostedPreviewFailed;
  const remainingPreviewItems = showUnifiedPreview
    ? previewItems.filter((item) => item.id !== unifiedPreviewItem?.id)
    : previewItems;

  React.useEffect(() => setFrostedPreviewFailed(false), [frostedPreviewURL]);

  if (presentation === "companion-open") {
    // The complete, outside stamp is the visual focal point, so it gets the
    // first real preview. A type/default page stays partially tucked inside.
    const outsideItem = previewItems.find((item) => taskPreviewImageURL(item));
    const insideItem = previewItems.find((item) => item.id !== outsideItem?.id);

    return (
      <span
        className={cn("app-library-task-folder", className)}
        data-task-folder-artwork="true"
        data-presentation="companion-open"
        data-view={view}
        data-preview-count={previewItems.length}
        data-total-count={resolvedTotal}
        aria-hidden="true"
      >
        <span className="app-library-task-folder__back" />
        <span className="app-library-task-folder__contents">
          <TaskFolderPreviewStamp
            key={insideItem?.id ?? "empty-inside"}
            item={insideItem}
            placement="inside"
          />
          {outsideItem ? (
            <TaskFolderPreviewStamp
              key={outsideItem.id}
              item={outsideItem}
              placement="outside"
            />
          ) : null}
        </span>
        <span className="app-library-task-folder__front">
          <span className="app-library-task-folder__front-cover" />
          <span className="app-library-task-folder__front-film" />
        </span>
      </span>
    );
  }

  return (
    <span
      className={cn("app-library-task-folder", className)}
      data-task-folder-artwork="true"
      data-presentation="closed"
      data-view={view}
      data-preview-count={previewItems.length}
      data-total-count={resolvedTotal}
      aria-hidden="true"
    >
      <span className="app-library-task-folder__back" />

      <span className="app-library-task-folder__contents">
        {previewItems.length > 0 ? (
          remainingPreviewItems.map((item) => (
            <TaskFolderPreviewPage key={item.id} item={item} />
          ))
        ) : (
          <span
            className="app-library-task-folder__page app-library-task-folder__page--empty"
            data-preview-kind="file"
          >
            <span className="app-library-task-folder__page-fallback">
              <File aria-hidden="true" strokeWidth={1.45} />
            </span>
          </span>
        )}
      </span>

      <span className="app-library-task-folder__front">
        {showUnifiedPreview ? (
          <svg
            className="app-library-task-folder__unified-preview"
            viewBox="0 0 100 120"
            preserveAspectRatio="none"
            aria-hidden="true"
            focusable="false"
          >
            <defs>
              <filter
                id={frostFilterId}
                filterUnits={SVG_USER_SPACE}
                primitiveUnits={SVG_USER_SPACE}
                x="-10"
                y="-10"
                width="120"
                height="140"
                colorInterpolationFilters={SVG_FROST_COLOR_SPACE}
              >
                <feGaussianBlur
                  in={SVG_FROST_SOURCE}
                  stdDeviation="8.5"
                  result="softened"
                />
                <feColorMatrix
                  in="softened"
                  type="saturate"
                  values="0.25"
                  result="frosted"
                />
                <feFlood
                  x="0"
                  y="0"
                  width="94.5"
                  height="120"
                  floodColor="white"
                  result={SVG_FRONT_REGION_RESULT}
                />
                <feComposite
                  in="frosted"
                  in2={SVG_FRONT_REGION_RESULT}
                  operator="in"
                  result={SVG_UNDER_COVER_RESULT}
                />
                <feComposite
                  in={SVG_FROST_SOURCE}
                  in2={SVG_FRONT_REGION_RESULT}
                  operator="out"
                  result={SVG_CLEAR_REVEAL_RESULT}
                />
                <feMerge>
                  <feMergeNode in={SVG_UNDER_COVER_RESULT} />
                  <feMergeNode in={SVG_CLEAR_REVEAL_RESULT} />
                </feMerge>
              </filter>
              <mask
                id={sheetMaskId}
                maskUnits={SVG_USER_SPACE}
                maskContentUnits={SVG_USER_SPACE}
                x={paper.x - 1}
                y={paper.y - 1}
                width={paper.width + 3}
                height={paper.height + 2}
              >
                <rect
                  x={paper.x}
                  y={paper.y}
                  width={paper.width}
                  height={paper.height}
                  rx={paper.radius}
                  fill="white"
                />
                {LIBRARY_PAPER_NOTCH_POSITIONS.map(
                  (position) => (
                    <circle
                      key={position}
                      cx="0"
                      cy={position}
                      r="1.25"
                      fill="black"
                    />
                  ),
                )}
              </mask>
            </defs>
            <g filter={`url(#${frostFilterId})`}>
              <g
                className="app-library-task-folder__preview-sheet"
                transform={TASK_FOLDER_PAPER_TRANSFORM}
                mask={`url(#${sheetMaskId})`}
              >
                <rect
                  className="app-library-task-folder__preview-paper"
                  x={paper.x}
                  y={paper.y}
                  width={paper.width}
                  height={paper.height}
                  rx={paper.radius}
                />
                <image
                  className="app-library-task-folder__preview-image"
                  href={frostedPreviewURL}
                  x={imageX}
                  y={imageY}
                  width={imageWidth}
                  height={imageHeight}
                  preserveAspectRatio={SVG_PREVIEW_ASPECT_RATIO}
                  onError={() => setFrostedPreviewFailed(true)}
                />
                <rect
                  className="app-library-task-folder__preview-border"
                  x={paper.x}
                  y={paper.y}
                  width={paper.width}
                  height={paper.height}
                  rx={paper.radius}
                />
              </g>
            </g>
          </svg>
        ) : null}
        <span className="app-library-task-folder__front-cover" />
        <span className="app-library-task-folder__front-film" />
      </span>
    </span>
  );
}
