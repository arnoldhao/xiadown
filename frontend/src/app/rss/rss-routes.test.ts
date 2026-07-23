import { describe, expect, test } from "bun:test";

import {
  createRSSCategoryRouteID,
  createRSSCollectionRouteID,
  createRSSSubscriptionRouteID,
  isMalformedRSSDynamicRouteID,
  parseRSSCategoryRouteID,
  parseRSSCollectionRouteID,
  parseRSSSubscriptionRouteID,
} from "./rss-routes";

describe("RSS workspace routes", () => {
  test("round-trips every organization identity through route-safe encoding", () => {
    const id = "group/\u4e2d\u6587?id=1";
    expect(parseRSSCategoryRouteID(createRSSCategoryRouteID(id))).toBe(id);
    expect(parseRSSCollectionRouteID(createRSSCollectionRouteID(id))).toBe(id);
    expect(parseRSSSubscriptionRouteID(createRSSSubscriptionRouteID(id))).toBe(id);
  });

  test("rejects missing, mismatched, or malformed route identities", () => {
    expect(parseRSSCategoryRouteID("category:")).toBeNull();
    expect(parseRSSCollectionRouteID("source:item")).toBeNull();
    expect(parseRSSSubscriptionRouteID("all")).toBeNull();
    expect(isMalformedRSSDynamicRouteID("category:")).toBeTrue();
    expect(isMalformedRSSDynamicRouteID("all")).toBeFalse();
  });
});
