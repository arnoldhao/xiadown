import * as React from "react";

import { cn } from "@/lib/utils";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/shared/ui/tooltip";

export type DreamSegmentSwitchItem<T extends string> = {
  value: T;
  label: string;
  icon?: React.ReactNode;
  tooltip?: string;
};

export function DreamSegmentSwitch<T extends string>(props: {
  value: T;
  items: readonly DreamSegmentSwitchItem<T>[];
  ariaLabel?: string;
  compact?: boolean;
  className?: string;
  onValueChange: (value: T) => void;
}) {
  const itemCount = Math.max(1, props.items.length);
  const resolvedActiveIndex = props.items.findIndex(
    (item) => item.value === props.value,
  );
  const activeIndex = resolvedActiveIndex >= 0 ? resolvedActiveIndex : 0;
  const activeSide = itemCount === 2 && activeIndex === 1 ? "right" : "left";
  const style = {
    "--app-dream-segment-count": itemCount,
    "--app-dream-segment-index": activeIndex,
  } as React.CSSProperties;

  return (
    <div
      role="tablist"
      aria-label={
        props.ariaLabel ?? props.items.map((item) => item.label).join(" / ")
      }
      data-side={activeSide}
      data-view={
        itemCount === 2 ? (activeSide === "right" ? "files" : "tasks") : undefined
      }
      data-count={itemCount}
      data-index={activeIndex}
      data-compact={props.compact ? "true" : "false"}
      className={cn("app-dream-segment-switch", props.className)}
      style={style}
    >
      <span className="app-dream-segment-switch-indicator" aria-hidden="true" />
      {props.items.map((item, index) => {
        const active = index === activeIndex;
        const button = (
          <button
            type="button"
            role="tab"
            aria-label={item.label}
            aria-selected={active}
            data-active={active ? "true" : "false"}
            className="app-dream-segment-switch-tab"
            onClick={() => props.onValueChange(item.value)}
          >
            {item.icon}
            <span
              className={cn(
                "app-dream-segment-switch-label",
                props.compact && "sr-only",
              )}
            >
              {item.label}
            </span>
          </button>
        );

        return (
          <Tooltip key={item.value}>
            <TooltipTrigger asChild openOnFocus={false}>
              {button}
            </TooltipTrigger>
            <TooltipContent side="bottom">{item.tooltip ?? item.label}</TooltipContent>
          </Tooltip>
        );
      })}
    </div>
  );
}
