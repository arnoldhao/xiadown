import { Search, X } from "lucide-react";
import * as React from "react";

import { cn } from "@/lib/utils";
import { Button } from "@/shared/ui/button";
import { Input } from "@/shared/ui/input";

export interface WorkspaceSearchControlProps
  extends Omit<
    React.ComponentPropsWithoutRef<"form">,
    "children" | "onChange" | "onSubmit"
  > {
  value: string;
  placeholder: string;
  clearLabel: string;
  submitLabel: React.ReactNode;
  inputRef?: React.Ref<HTMLInputElement>;
  onValueChange: (value: string) => void;
  onClear?: () => void;
  onSubmit?: (value: string) => void;
}

export function shouldSuppressWorkspaceSearchSubmit(event: {
  key: string;
  isComposing?: boolean;
  keyCode?: number;
}) {
  return (
    event.key === "Enter" &&
    (event.isComposing === true || event.keyCode === 229)
  );
}

/**
 * Canonical Search-page control used by every station. Page-specific code owns
 * the query semantics; this boundary owns the DOM order and Dream control role.
 */
export const WorkspaceSearchControl = React.forwardRef<
  HTMLFormElement,
  WorkspaceSearchControlProps
>(function WorkspaceSearchControl(
  {
    autoComplete = "off",
    autoFocus = true,
    className,
    clearLabel,
    inputRef,
    onClear,
    onSubmit,
    onValueChange,
    placeholder,
    submitLabel,
    value,
    ...props
  },
  ref,
) {
  const trimmedValue = value.trim();
  const composingRef = React.useRef(false);

  return (
    <form
      ref={ref}
      className={cn(
        "app-dream-workspace-search app-dream-search-control app-dream-control-shell app-station-search-content-search wails-no-drag",
        className,
      )}
      onSubmit={(event) => {
        event.preventDefault();
        if (trimmedValue) onSubmit?.(trimmedValue);
      }}
      {...props}
    >
      <Search aria-hidden="true" className="app-dream-workspace-search__icon" />
      <Input
        ref={inputRef}
        aria-label={placeholder}
        autoComplete={autoComplete}
        autoFocus={autoFocus}
        className="app-dream-workspace-search__input"
        onCompositionEnd={() => {
          composingRef.current = false;
        }}
        onCompositionStart={() => {
          composingRef.current = true;
        }}
        onChange={(event) => onValueChange(event.currentTarget.value)}
        onKeyDown={(event) => {
          if (
            shouldSuppressWorkspaceSearchSubmit({
              key: event.key,
              isComposing:
                composingRef.current || event.nativeEvent.isComposing,
              keyCode: event.keyCode,
            })
          ) {
            event.preventDefault();
          }
        }}
        placeholder={placeholder}
        type="search"
        value={value}
      />
      {value ? (
        <button
          aria-label={clearLabel}
          className="app-dream-workspace-search__clear"
          onClick={() => {
            onValueChange("");
            onClear?.();
          }}
          onMouseDown={(event) => event.preventDefault()}
          title={clearLabel}
          type="button"
        >
          <X aria-hidden="true" />
        </button>
      ) : null}
      <Button
        className="app-dream-workspace-search__submit"
        disabled={!trimmedValue}
        size="sm"
        type="submit"
      >
        {submitLabel}
      </Button>
    </form>
  );
});
