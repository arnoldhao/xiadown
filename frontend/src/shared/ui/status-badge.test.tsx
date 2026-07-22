import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";
import { CircleCheck } from "lucide-react";

import { DREAM_STATUS_TONES, StatusBadge } from "./status-badge";

describe("StatusBadge", () => {
  test("publishes the complete semantic tone vocabulary", () => {
    expect(DREAM_STATUS_TONES).toEqual([
      "neutral",
      "accent",
      "busy",
      "success",
      "warning",
      "danger",
      "muted",
    ]);
  });

  test("publishes one semantic tone contract with an optional icon", () => {
    const markup = renderToStaticMarkup(
      <StatusBadge tone="success" icon={<CircleCheck />}>Ready</StatusBadge>,
    );

    expect(markup).toContain('data-app-status-badge="true"');
    expect(markup).toContain('data-tone="success"');
    expect(markup).toContain("app-dream-status-badge__icon");
    expect(markup).toContain("Ready");
  });

  test("can use the shared compact marker without inventing feature CSS", () => {
    const markup = renderToStaticMarkup(
      <StatusBadge tone="warning" marker>Needs review</StatusBadge>,
    );
    expect(markup).toContain("app-dream-status-badge__marker");
  });

  test("owns the compact icon-only state for dense lists", () => {
    const markup = renderToStaticMarkup(
      <StatusBadge
        aria-label="Connected"
        icon={<CircleCheck />}
        iconOnly
        tone="success"
      />,
    );

    expect(markup).toContain('aria-label="Connected"');
    expect(markup).toContain('data-icon-only="true"');
    expect(markup).not.toContain("app-dream-status-badge__label");
  });
});
