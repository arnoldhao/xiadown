import { describe, expect, mock, test } from "bun:test";
import * as React from "react";
import { renderToStaticMarkup } from "react-dom/server";

import {
  DreamSegmentSwitch,
  type DreamSegmentSwitchItem,
} from "./dream-segment-switch";

type TestValue = "preview" | "info" | "versions" | "activity";

type CapturedButton = React.ReactElement<{
  "aria-selected": boolean;
  disabled?: boolean;
  onKeyDown?: (event: React.KeyboardEvent<HTMLButtonElement>) => void;
  tabIndex?: number;
}>;

const defaultItems: readonly DreamSegmentSwitchItem<TestValue>[] = [
  { value: "preview", label: "Preview" },
  { value: "info", label: "Info" },
  { value: "versions", label: "Versions" },
  { value: "activity", label: "Activity" },
];

type CaptureOptions = {
  value: TestValue;
  items?: readonly DreamSegmentSwitchItem<TestValue>[];
  tooltips?: boolean;
  onValueChange: (value: TestValue) => void;
};

function captureSwitch(options: CaptureOptions) {
  let captured: React.ReactElement | undefined;

  function Capture() {
    // Calling the component while React's server renderer owns the hook
    // dispatcher preserves the real keyboard handler closures without needing
    // a browser-like DOM dependency just to dispatch a KeyboardEvent.
    captured = DreamSegmentSwitch<TestValue>({
      value: options.value,
      items: options.items ?? defaultItems,
      tooltips: options.tooltips,
      onValueChange: options.onValueChange,
    });
    return null;
  }

  renderToStaticMarkup(<Capture />);
  if (!captured) throw new Error("DreamSegmentSwitch was not captured");

  return captured;
}

function captureButtons(options: CaptureOptions) {
  const captured = captureSwitch(options);

  return React.Children.toArray(captured.props.children)
    .slice(1)
    .map((tooltip) => {
      if (!React.isValidElement(tooltip)) {
        throw new Error("Expected a Tooltip element");
      }
      const trigger = React.Children.toArray(tooltip.props.children)[0];
      if (!React.isValidElement(trigger) || !React.isValidElement(trigger.props.children)) {
        throw new Error("Expected a TooltipTrigger with a button child");
      }
      return trigger.props.children as CapturedButton;
    });
}

function attachFocusSpies(buttons: CapturedButton[]) {
  return buttons.map((button) => {
    const focus = mock(() => undefined);
    const ref = (button as React.ReactElement & {
      ref?: (node: HTMLButtonElement | null) => void;
    }).ref;
    ref?.({ focus } as unknown as HTMLButtonElement);
    return focus;
  });
}

function pressKey(button: CapturedButton, key: string) {
  const preventDefault = mock(() => undefined);
  button.props.onKeyDown?.({
    key,
    preventDefault,
  } as unknown as React.KeyboardEvent<HTMLButtonElement>);
  return preventDefault;
}

describe("DreamSegmentSwitch keyboard navigation", () => {
  test("can render plain tabs without tooltip wrappers", () => {
    const captured = captureSwitch({
      value: "preview",
      tooltips: false,
      onValueChange: () => undefined,
    });
    const segments = React.Children.toArray(captured.props.children).slice(1);

    expect(segments).toHaveLength(defaultItems.length);
    expect(
      segments.every(
        (segment) => React.isValidElement(segment) && segment.type === "button",
      ),
    ).toBe(true);
  });

  test("uses a roving tab stop for the selected segment", () => {
    const buttons = captureButtons({
      value: "versions",
      onValueChange: () => undefined,
    });

    expect(buttons.map((button) => button.props.tabIndex)).toEqual([-1, -1, 0, -1]);
    expect(buttons.map((button) => button.props["aria-selected"])).toEqual([
      false,
      false,
      true,
      false,
    ]);
  });

  test("wraps ArrowLeft and ArrowRight while focusing and selecting the target", () => {
    const leftChanges: TestValue[] = [];
    const leftButtons = captureButtons({
      value: "preview",
      onValueChange: (value) => leftChanges.push(value),
    });
    const leftFocus = attachFocusSpies(leftButtons);
    const leftPrevented = pressKey(leftButtons[0]!, "ArrowLeft");

    expect(leftPrevented).toHaveBeenCalledTimes(1);
    expect(leftFocus[3]).toHaveBeenCalledTimes(1);
    expect(leftChanges).toEqual(["activity"]);

    const rightChanges: TestValue[] = [];
    const rightButtons = captureButtons({
      value: "activity",
      onValueChange: (value) => rightChanges.push(value),
    });
    const rightFocus = attachFocusSpies(rightButtons);
    const rightPrevented = pressKey(rightButtons[3]!, "ArrowRight");

    expect(rightPrevented).toHaveBeenCalledTimes(1);
    expect(rightFocus[0]).toHaveBeenCalledTimes(1);
    expect(rightChanges).toEqual(["preview"]);
  });

  test("moves Home and End to the first and last enabled segments", () => {
    const homeChanges: TestValue[] = [];
    const homeButtons = captureButtons({
      value: "versions",
      onValueChange: (value) => homeChanges.push(value),
    });
    const homeFocus = attachFocusSpies(homeButtons);
    pressKey(homeButtons[2]!, "Home");

    expect(homeFocus[0]).toHaveBeenCalledTimes(1);
    expect(homeChanges).toEqual(["preview"]);

    const endChanges: TestValue[] = [];
    const endButtons = captureButtons({
      value: "info",
      onValueChange: (value) => endChanges.push(value),
    });
    const endFocus = attachFocusSpies(endButtons);
    pressKey(endButtons[1]!, "End");

    expect(endFocus[3]).toHaveBeenCalledTimes(1);
    expect(endChanges).toEqual(["activity"]);
  });

  test("skips disabled segments during arrow and boundary navigation", () => {
    const items: readonly DreamSegmentSwitchItem<TestValue>[] = [
      { value: "preview", label: "Preview", disabled: true },
      { value: "info", label: "Info" },
      { value: "versions", label: "Versions", disabled: true },
      { value: "activity", label: "Activity" },
    ];
    const changes: TestValue[] = [];
    const buttons = captureButtons({
      value: "info",
      items,
      onValueChange: (value) => changes.push(value),
    });
    const focus = attachFocusSpies(buttons);

    pressKey(buttons[1]!, "ArrowRight");
    pressKey(buttons[1]!, "ArrowLeft");
    pressKey(buttons[1]!, "Home");
    pressKey(buttons[1]!, "End");

    expect(focus[0]).not.toHaveBeenCalled();
    expect(focus[2]).not.toHaveBeenCalled();
    expect(focus[1]).toHaveBeenCalledTimes(1);
    expect(focus[3]).toHaveBeenCalledTimes(3);
    expect(changes).toEqual(["activity", "activity", "info", "activity"]);
  });
});
