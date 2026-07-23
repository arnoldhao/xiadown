import * as React from "react";

import { cn } from "@/lib/utils";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/shared/ui/tooltip";
import { useRovingTabs } from "@/shared/ui/roving-tabs";

export type DreamSegmentSwitchItem<T extends string> = {
  value: T;
  label: string;
  icon?: React.ReactNode;
  tooltip?: string;
  disabled?: boolean;
  tabId?: string;
  panelId?: string;
};

export function DreamSegmentSwitch<T extends string>(props: {
  value: T;
  items: readonly DreamSegmentSwitchItem<T>[];
  ariaLabel?: string;
  compact?: boolean;
  tooltips?: boolean;
  className?: string;
  onValueChange: (value: T) => void;
}) {
  const itemCount = Math.max(1, props.items.length);
  const resolvedActiveIndex = props.items.findIndex(
    (item) => item.value === props.value,
  );
  const activeIndex = resolvedActiveIndex >= 0 ? resolvedActiveIndex : 0;
  const activeSide = itemCount === 2 && activeIndex === 1 ? "right" : "left";
  const tabs = useRovingTabs({
    items: props.items,
    value: props.value,
    onValueChange: props.onValueChange,
  });
  const style = {
    "--app-dream-segment-count": itemCount,
    "--app-dream-segment-index": activeIndex,
  } as React.CSSProperties;

  return (
    <div
      role="tablist"
      aria-orientation="horizontal"
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
            key={item.value}
            ref={(node) => {
              tabs.setTabRef(index, node);
            }}
            type="button"
            role="tab"
            id={item.tabId}
            aria-label={item.label}
            aria-controls={item.panelId}
            aria-selected={active}
            disabled={item.disabled}
            tabIndex={index === tabs.focusableIndex ? 0 : -1}
            data-active={active ? "true" : "false"}
            className="app-dream-segment-switch-tab"
            onClick={() => {
              if (!item.disabled) {
                props.onValueChange(item.value);
              }
            }}
            onKeyDown={(event) => tabs.onKeyDown(event, index)}
          >
            {item.icon}
            <span
              className={cn(
                "app-dream-segment-switch-label",
                props.compact && "app-visually-hidden",
              )}
            >
              {item.label}
            </span>
          </button>
        );

        if (props.tooltips === false) {
          return button;
        }

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
