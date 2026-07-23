import {
  FileArchive,
  FileBraces,
  FileCode,
  FileText,
  FileType,
  FileVideo,
  ImageIcon,
  Languages,
  Link2,
  Music2,
  type LucideIcon,
} from "lucide-react";

export type ResourceKindIconName =
  | "video"
  | "audio"
  | "subtitle"
  | "image"
  | "manifest"
  | "api"
  | "document"
  | "font"
  | "archive"
  | "other";

const RESOURCE_KIND_ICONS = {
  video: FileVideo,
  audio: Music2,
  subtitle: Languages,
  image: ImageIcon,
  manifest: FileCode,
  api: FileBraces,
  document: FileText,
  font: FileType,
  archive: FileArchive,
  other: Link2,
} satisfies Record<ResourceKindIconName, LucideIcon>;

export function resolveResourceKindIcon(kind?: string | null): LucideIcon {
  const normalizedKind = (kind ?? "").trim().toLowerCase();
  if (normalizedKind in RESOURCE_KIND_ICONS) {
    return RESOURCE_KIND_ICONS[normalizedKind as ResourceKindIconName];
  }
  return RESOURCE_KIND_ICONS.other;
}
