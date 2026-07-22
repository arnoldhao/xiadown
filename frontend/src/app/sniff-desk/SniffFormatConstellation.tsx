import {
  resolveResourceKindIcon,
  type ResourceKindIconName,
} from "@/shared/ui/resource-kind-icon";

export type SniffFormatConstellationProps = {
  burstKey: number;
};

type SniffFormatConstellationItem = {
  kind: ResourceKindIconName;
  label: string;
};

const SNIFF_FORMAT_CONSTELLATION_ITEMS = [
  {
    kind: "video",
    label: "MP4",
  },
  {
    kind: "audio",
    label: "FLAC",
  },
  {
    kind: "image",
    label: "WEBP",
  },
  {
    kind: "manifest",
    label: "HLS",
  },
  {
    kind: "api",
    label: "JSON",
  },
  {
    kind: "document",
    label: "PDF",
  },
  {
    kind: "archive",
    label: "ZIP",
  },
  {
    kind: "subtitle",
    label: "SRT",
  },
  {
    kind: "font",
    label: "WOFF",
  },
  {
    kind: "other",
    label: "URL",
  },
] as const satisfies readonly SniffFormatConstellationItem[];

export function SniffFormatConstellation({
  burstKey,
}: SniffFormatConstellationProps) {
  return (
    <div className="app-sniff-desk-format-constellation" aria-hidden="true">
      <div className="app-sniff-desk-format-field" key={burstKey}>
        {SNIFF_FORMAT_CONSTELLATION_ITEMS.map((item) => {
          const Icon = resolveResourceKindIcon(item.kind);
          return (
            <span
              className="app-sniff-desk-format-item"
              data-tone={item.kind}
              key={item.kind}
            >
              <span className="app-sniff-desk-format-orientation">
                <span className="app-sniff-desk-format-glyph">
                  <Icon
                    className="app-sniff-desk-format-icon"
                    focusable="false"
                    strokeWidth={1.75}
                  />
                  <span className="app-sniff-desk-format-label">
                    {item.label}
                  </span>
                </span>
              </span>
            </span>
          );
        })}
      </div>
    </div>
  );
}
