import * as React from "react";

import { cn } from "@/lib/utils";

export type DreamInlineSwitchProps = Omit<
  React.ButtonHTMLAttributes<HTMLButtonElement>,
  "aria-checked" | "aria-label" | "children" | "onChange" | "role"
> & {
  ariaLabel: string;
  checked: boolean;
  onCheckedChange: (checked: boolean) => void;
};

export type DreamInlineSwitchVisualProps = Omit<
  React.HTMLAttributes<HTMLSpanElement>,
  "aria-hidden" | "children"
> & {
  checked: boolean;
};

function DreamInlineSwitchKnob() {
  return <span className="app-dream-inline-switch-knob" aria-hidden="true" />;
}

export const DreamInlineSwitchVisual = React.forwardRef<
  HTMLSpanElement,
  DreamInlineSwitchVisualProps
>(function DreamInlineSwitchVisual({ checked, className, ...spanProps }, ref) {
  return (
    <span
      {...spanProps}
      ref={ref}
      aria-hidden="true"
      className={cn("app-dream-inline-switch", className)}
      data-state={checked ? "checked" : "unchecked"}
    >
      <DreamInlineSwitchKnob />
    </span>
  );
});

export const DreamInlineSwitch = React.forwardRef<
  HTMLButtonElement,
  DreamInlineSwitchProps
>(function DreamInlineSwitch(
  {
    ariaLabel,
    checked,
    className,
    disabled,
    onCheckedChange,
    onClick,
    ...buttonProps
  },
  ref,
) {
  return (
    <button
      {...buttonProps}
      ref={ref}
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={ariaLabel}
      className={cn("app-dream-inline-switch", className)}
      data-state={checked ? "checked" : "unchecked"}
      disabled={disabled}
      onClick={(event) => {
        onClick?.(event);
        if (!event.defaultPrevented && disabled !== true) {
          onCheckedChange(!checked);
        }
      }}
    >
      <DreamInlineSwitchKnob />
    </button>
  );
});
