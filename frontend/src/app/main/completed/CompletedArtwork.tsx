import {
  isLibraryDefaultArtworkURL,
  LibraryArtwork,
} from "@/app/library/LibraryArtwork";
import type {
  LibraryItemCategory,
  LibraryOtherGroup,
} from "@/app/library/types";
import { resolveCompletedFileType } from "@/app/main/helpers";
import type {
  CompletedFileEntry,
  CompletedFileType,
  CompletedTaskEntry,
} from "@/app/main/types";
import { COMPLETED_DEFAULT_COVER_IMAGE_URLS } from "@/shared/assets/default-cover";

import "@/app/library/library.css";

type CompletedArtworkKind = {
  category: LibraryItemCategory;
  otherGroup?: LibraryOtherGroup;
};

const COMPLETED_FILE_ARTWORK_KINDS: Record<
  CompletedFileType,
  CompletedArtworkKind
> = {
  video: { category: "video" },
  audio: { category: "audio" },
  image: { category: "image" },
  document: { category: "other", otherGroup: "document" },
  font: { category: "other", otherGroup: "font" },
  archive: { category: "other", otherGroup: "archive" },
  subtitle: { category: "other", otherGroup: "subtitle" },
  manifest: { category: "other", otherGroup: "manifest" },
  api: { category: "other", otherGroup: "api" },
  other: { category: "other", otherGroup: "unknown" },
};

const COMPLETED_FILE_FALLBACKS: Record<CompletedFileType, string> = {
  video: COMPLETED_DEFAULT_COVER_IMAGE_URLS.video,
  audio: COMPLETED_DEFAULT_COVER_IMAGE_URLS.audio,
  image: COMPLETED_DEFAULT_COVER_IMAGE_URLS.image,
  document: COMPLETED_DEFAULT_COVER_IMAGE_URLS.document,
  font: COMPLETED_DEFAULT_COVER_IMAGE_URLS.font,
  archive: COMPLETED_DEFAULT_COVER_IMAGE_URLS.archive,
  subtitle: COMPLETED_DEFAULT_COVER_IMAGE_URLS.subtitle,
  manifest: COMPLETED_DEFAULT_COVER_IMAGE_URLS.manifest,
  api: COMPLETED_DEFAULT_COVER_IMAGE_URLS.api,
  other: COMPLETED_DEFAULT_COVER_IMAGE_URLS.other,
};

export function resolveCompletedArtworkImageURL(
  ...candidates: Array<string | null | undefined>
) {
  return (
    candidates
      .map((candidate) => candidate?.trim() ?? "")
      .find(
        (candidate) =>
          candidate.length > 0 && !isLibraryDefaultArtworkURL(candidate),
      ) ?? ""
  );
}

export function CompletedFileArtwork(props: {
  file: Pick<
    CompletedFileEntry,
    "kind" | "path" | "format" | "media" | "coverURL"
  >;
  src?: string;
  alt: string;
  className?: string;
}) {
  const type = resolveCompletedFileType(props.file);
  const kind = COMPLETED_FILE_ARTWORK_KINDS[type];
  return (
    <LibraryArtwork
      src={props.src ?? props.file.coverURL}
      fallbackSrc={COMPLETED_FILE_FALLBACKS[type]}
      category={kind.category}
      otherGroup={kind.otherGroup}
      alt={props.alt}
      className={props.className}
    />
  );
}

export function CompletedTaskArtwork(props: {
  task: Pick<CompletedTaskEntry, "coverURL">;
  src?: string;
  alt: string;
  className?: string;
}) {
  return (
    <LibraryArtwork
      src={props.src ?? props.task.coverURL}
      fallbackSrc={COMPLETED_DEFAULT_COVER_IMAGE_URLS.other}
      category="task"
      alt={props.alt}
      className={props.className}
    />
  );
}
