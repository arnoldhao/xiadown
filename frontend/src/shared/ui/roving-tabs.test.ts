import { describe, expect, test } from "bun:test";

import {
  resolveRovingTabDestination,
  resolveRovingTabStopIndex,
} from "./roving-tabs";

const items = [
  { value: "summary" as const },
  { value: "audit" as const, disabled: true },
  { value: "storage" as const },
  { value: "data" as const },
];

describe("roving tabs", () => {
  test("keeps exactly the selected enabled item in the tab order", () => {
    expect(resolveRovingTabStopIndex(items, "storage")).toBe(2);
    expect(resolveRovingTabStopIndex(items, "audit")).toBe(0);
  });

  test("wraps arrows, skips disabled tabs, and supports Home and End", () => {
    expect(resolveRovingTabDestination(items, 0, "ArrowLeft")).toBe(3);
    expect(resolveRovingTabDestination(items, 0, "ArrowRight")).toBe(2);
    expect(resolveRovingTabDestination(items, 3, "ArrowRight")).toBe(0);
    expect(resolveRovingTabDestination(items, 2, "Home")).toBe(0);
    expect(resolveRovingTabDestination(items, 0, "End")).toBe(3);
    expect(resolveRovingTabDestination(items, 0, "Enter")).toBeNull();
  });
});
