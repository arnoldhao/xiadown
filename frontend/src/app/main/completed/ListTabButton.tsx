import * as React from "react";

import { cn } from "@/lib/utils";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/shared/ui/tooltip";

export function CompletedListTabButton(props: {
  active: boolean;
  compact: boolean;
  label: string;
  children: React.ReactNode;
  onClick: () => void;
}) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button
          type="button"
          role="tab"
          aria-label={props.label}
          aria-selected={props.active}
          data-active={props.active ? "true" : "false"}
          data-compact={props.compact ? "true" : "false"}
          className={cn(
            "flex h-8 items-center justify-center overflow-hidden rounded-[10px] text-muted-foreground transition-[width,padding,background-color,color,box-shadow] duration-200 ease-out",
            "hover:bg-background hover:text-foreground focus-visible:outline-none",
            "data-[active=true]:bg-sidebar-accent data-[active=true]:text-sidebar-primary data-[active=true]:shadow-sm",
            props.compact ? "w-8 px-0" : "w-[4.75rem] px-1.5",
          )}
          onClick={props.onClick}
        >
          {props.children}
          <span
            className={cn(
              "block min-w-0 truncate text-xs font-medium transition-[margin,max-width,opacity,transform] duration-200 ease-out",
              props.compact
                ? "ml-0 max-w-0 -translate-x-1 opacity-0"
                : "ml-1.5 max-w-10 translate-x-0 opacity-100",
            )}
          >
            {props.label}
          </span>
        </button>
      </TooltipTrigger>
      <TooltipContent side="bottom">{props.label}</TooltipContent>
    </Tooltip>
  );
}
