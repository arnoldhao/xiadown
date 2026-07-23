import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import {
  createListenInfiniteScrollGate,
  LISTEN_INFINITE_SCROLL_ROOT_MARGIN,
  ListenInfiniteScrollSentinel,
} from "./infinite-scroll-sentinel";

describe("ListenInfiniteScrollSentinel", () => {
  test("does not acquire while hidden, disabled, loading, or missing a continuation", () => {
    const gate = createListenInfiniteScrollGate();

    expect(
      gate.tryAcquire({ enabled: true, loading: false, continuation: "next" }),
    ).toBe(false);

    gate.setVisible(true);
    expect(
      gate.tryAcquire({ enabled: false, loading: false, continuation: "next" }),
    ).toBe(false);
    expect(
      gate.tryAcquire({ enabled: true, loading: true, continuation: "next" }),
    ).toBe(false);
    expect(
      gate.tryAcquire({ enabled: true, loading: false, continuation: "  " }),
    ).toBe(false);
  });

  test("acquires a continuation only once", () => {
    const gate = createListenInfiniteScrollGate();
    gate.setVisible(true);

    expect(
      gate.tryAcquire({ enabled: true, loading: false, continuation: " next " }),
    ).toBe(true);
    expect(
      gate.tryAcquire({ enabled: true, loading: false, continuation: "next" }),
    ).toBe(false);

    gate.setVisible(false);
    gate.setVisible(true);
    expect(
      gate.tryAcquire({ enabled: true, loading: false, continuation: "next" }),
    ).toBe(false);
  });

  test("fills a short viewport when a new continuation arrives while still visible", () => {
    const gate = createListenInfiniteScrollGate();
    gate.setVisible(true);

    expect(
      gate.tryAcquire({ enabled: true, loading: false, continuation: "page-2" }),
    ).toBe(true);
    expect(
      gate.tryAcquire({ enabled: true, loading: false, continuation: "page-3" }),
    ).toBe(true);
  });

  test("can acquire when loading finishes during the same visible cycle", () => {
    const gate = createListenInfiniteScrollGate();
    gate.setVisible(true);

    expect(
      gate.tryAcquire({ enabled: true, loading: true, continuation: "page-2" }),
    ).toBe(false);
    expect(
      gate.tryAcquire({ enabled: true, loading: false, continuation: "page-2" }),
    ).toBe(true);
  });

  test("renders safely during SSR without IntersectionObserver", () => {
    const markup = renderToStaticMarkup(
      <ListenInfiniteScrollSentinel
        continuation="page-2"
        enabled
        loading={false}
        onLoadMore={() => undefined}
      />,
    );

    expect(markup).toContain('data-listen-infinite-scroll-sentinel="true"');
    expect(markup).toContain('aria-hidden="true"');
    expect(markup).toContain("h-px w-full pointer-events-none");
    expect(markup).not.toContain("style=");
    expect(LISTEN_INFINITE_SCROLL_ROOT_MARGIN).toBe("0px 0px 320px 0px");
  });
});
