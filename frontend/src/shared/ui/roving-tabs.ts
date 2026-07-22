import * as React from "react";

export type RovingTabKey = "ArrowLeft" | "ArrowRight" | "Home" | "End";

export interface RovingTabItem<T extends string> {
  value: T;
  disabled?: boolean;
}

export function resolveRovingTabStopIndex<T extends string>(
  items: readonly RovingTabItem<T>[],
  value: T,
): number {
  const selectedIndex = items.findIndex((item) => item.value === value);
  if (selectedIndex >= 0 && !items[selectedIndex]?.disabled) {
    return selectedIndex;
  }
  return items.findIndex((item) => !item.disabled);
}

export function resolveRovingTabDestination<T extends string>(
  items: readonly RovingTabItem<T>[],
  currentIndex: number,
  key: string,
): number | null {
  if (items.length === 0) return null;

  if (key === "Home") {
    const first = items.findIndex((item) => !item.disabled);
    return first >= 0 ? first : null;
  }
  if (key === "End") {
    for (let index = items.length - 1; index >= 0; index -= 1) {
      if (!items[index]?.disabled) return index;
    }
    return null;
  }
  if (key !== "ArrowLeft" && key !== "ArrowRight") return null;

  const direction = key === "ArrowRight" ? 1 : -1;
  for (let offset = 1; offset <= items.length; offset += 1) {
    const candidate = (
      currentIndex + direction * offset + items.length
    ) % items.length;
    if (!items[candidate]?.disabled) return candidate;
  }
  return null;
}

/**
 * Implements the WAI-ARIA horizontal tabs keyboard pattern with automatic
 * activation and a single tab stop.
 */
export function useRovingTabs<T extends string>(options: {
  items: readonly RovingTabItem<T>[];
  value: T;
  onValueChange: (value: T) => void;
}) {
  const tabRefs = React.useRef<Array<HTMLButtonElement | null>>([]);
  const focusableIndex = resolveRovingTabStopIndex(options.items, options.value);

  const onKeyDown = (
    event: React.KeyboardEvent<HTMLButtonElement>,
    currentIndex: number,
  ) => {
    const nextIndex = resolveRovingTabDestination(
      options.items,
      currentIndex,
      event.key,
    );
    if (nextIndex === null) return;

    const nextItem = options.items[nextIndex];
    if (!nextItem || nextItem.disabled) return;
    event.preventDefault();
    tabRefs.current[nextIndex]?.focus();
    options.onValueChange(nextItem.value);
  };

  return {
    focusableIndex,
    onKeyDown,
    setTabRef(index: number, node: HTMLButtonElement | null) {
      tabRefs.current[index] = node;
    },
  };
}
