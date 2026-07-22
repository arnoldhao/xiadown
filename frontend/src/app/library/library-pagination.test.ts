import { describe, expect, test } from "bun:test";

import {
  clampLibraryPage,
  libraryPageCount,
  libraryPageRange,
  libraryPageTokens,
  normalizeLibraryPageSize,
  sliceLibraryPage,
} from "./library-pagination";

describe("Library pagination", () => {
  test("normalizes invalid sizes and clamps stale pages after totals shrink", () => {
    expect(normalizeLibraryPageSize(Number.NaN)).toBe(48);
    expect(normalizeLibraryPageSize(20.9)).toBe(20);
    expect(libraryPageCount(101, 48)).toBe(3);
    expect(clampLibraryPage(9, 25, 24)).toBe(2);
    expect(clampLibraryPage(0, 0, 24)).toBe(1);
  });

  test("reports an inclusive range and preserves an empty zero range", () => {
    expect(libraryPageRange(2, 24, 50)).toEqual({ start: 25, end: 48 });
    expect(libraryPageRange(99, 24, 50)).toEqual({ start: 49, end: 50 });
    expect(libraryPageRange(1, 24, 0)).toEqual({ start: 0, end: 0 });
  });

  test("creates direct page tokens with compact ellipses", () => {
    expect(libraryPageTokens(1, 5)).toEqual([1, 2, 3, 4, 5]);
    expect(libraryPageTokens(1, 20)).toEqual([1, 2, 3, 4, "ellipsis", 20]);
    expect(libraryPageTokens(10, 20)).toEqual([1, "ellipsis", 9, 10, 11, "ellipsis", 20]);
    expect(libraryPageTokens(20, 20)).toEqual([1, "ellipsis", 17, 18, 19, 20]);
  });

  test("slices only the requested local page", () => {
    const items = Array.from({ length: 53 }, (_, index) => index + 1);
    expect(sliceLibraryPage(items, 2, 24)).toEqual(items.slice(24, 48));
    expect(sliceLibraryPage(items, 99, 24)).toEqual(items.slice(48));
  });
});
