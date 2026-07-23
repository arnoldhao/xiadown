import { describe, expect, test } from "bun:test";

import {
  isWindowsTitlebarDoubleClickTarget,
  registerWindowsTitlebarDoubleClick,
} from "./windows-titlebar";

type FakeStyleElement = EventTarget & {
  draggableValue: string;
  interactive: boolean;
  inDragRegion: boolean;
  ownerDocument: FakeDocument;
  closest: (selector: string) => FakeStyleElement | null;
};

type DoubleClickListener = (event: MouseEvent) => void;

class FakeDocument {
  addCount = 0;
  removeCount = 0;
  listener: DoubleClickListener | null = null;
  defaultView = {
    getComputedStyle: (element: FakeStyleElement) => ({
      getPropertyValue: (property: string) =>
        property === "--wails-draggable" ? element.draggableValue : "",
    }),
  };

  addEventListener(type: string, listener: EventListenerOrEventListenerObject) {
    if (type !== "dblclick" || typeof listener !== "function") {
      return;
    }
    this.addCount += 1;
    this.listener = listener as DoubleClickListener;
  }

  removeEventListener(
    type: string,
    listener: EventListenerOrEventListenerObject,
  ) {
    if (type !== "dblclick" || listener !== this.listener) {
      return;
    }
    this.removeCount += 1;
    this.listener = null;
  }

  dispatchDoubleClick(event: MouseEvent) {
    this.listener?.(event);
  }
}

function fakeElement(
  ownerDocument: FakeDocument,
  options: Partial<
    Pick<
      FakeStyleElement,
      "draggableValue" | "interactive" | "inDragRegion"
    >
  > = {},
) {
  return {
    draggableValue: options.draggableValue ?? "drag",
    interactive: options.interactive ?? false,
    inDragRegion: options.inDragRegion ?? true,
    ownerDocument,
    closest(selector: string) {
      if (selector === ".wails-drag") {
        return this.inDragRegion ? this : null;
      }
      if (selector.includes(".wails-no-drag")) {
        return this.interactive ? this : null;
      }
      return null;
    },
  } as FakeStyleElement;
}

function fakeDoubleClick(
  target: EventTarget,
  options: { button?: number; defaultPrevented?: boolean } = {},
) {
  let preventDefaultCalls = 0;
  const event = {
    button: options.button ?? 0,
    defaultPrevented: options.defaultPrevented ?? false,
    target,
    preventDefault() {
      preventDefaultCalls += 1;
      this.defaultPrevented = true;
    },
  } as unknown as MouseEvent;
  return {
    event,
    preventDefaultCalls: () => preventDefaultCalls,
  };
}

describe("WindowControls", () => {
  test("renders Windows caption buttons inline with stable owner markers", async () => {
    const [source, contract, controls] = await Promise.all([
      Bun.file(new URL("./WindowControls.tsx", import.meta.url)).text(),
      Bun.file(
        new URL("../../shared/styles/dream/button-contract.css", import.meta.url),
      ).text(),
      Bun.file(
        new URL("../../shared/styles/dream/controls.css", import.meta.url),
      ).text(),
    ]);

    expect(source).toContain("app-window-controls wails-no-drag");
    expect(source).toContain("data-window-controls-owner={owner}");
    expect(source).toContain("data-window-controls-platform={platform}");
    expect(source).toContain("z-[var(--app-layer-window-controls)]");
    expect(source).toContain('owner = "primary"');
    expect(source).not.toContain("wails-no-drag fixed right-0 top-0");
    expect(source).not.toMatch(/bg-\[#|text-white|hover:!bg-black/);
    expect(source).not.toMatch(/(?:bg|border)-current|rounded-\[/);
    expect(source).toContain("app-window-control-glyph--restore");
    expect(controls).toContain(".app-window-control-glyph--close");
    expect(source).toContain("app-window-control--macos-close");
    expect(source).toContain("app-window-control--windows-close");
    expect(contract).toContain(".app-window-control--windows-close");
    expect(contract).toContain("var(--app-window-control-danger)");
  });

  test("accepts only computed draggable, non-interactive titlebar targets", () => {
    const targetDocument = new FakeDocument();

    expect(
      isWindowsTitlebarDoubleClickTarget(fakeElement(targetDocument)),
    ).toBe(true);
    expect(
      isWindowsTitlebarDoubleClickTarget(
        fakeElement(targetDocument, { interactive: true }),
      ),
    ).toBe(false);
    expect(
      isWindowsTitlebarDoubleClickTarget(
        fakeElement(targetDocument, { draggableValue: "no-drag" }),
      ),
    ).toBe(false);
    expect(
      isWindowsTitlebarDoubleClickTarget(
        fakeElement(targetDocument, { inDragRegion: false }),
      ),
    ).toBe(false);
  });

  test("deduplicates the document listener and unregisters it idempotently", () => {
    const targetDocument = new FakeDocument();
    const unregisterFirst = registerWindowsTitlebarDoubleClick(
      targetDocument as unknown as Document,
      () => undefined,
    );
    const unregisterSecond = registerWindowsTitlebarDoubleClick(
      targetDocument as unknown as Document,
      () => undefined,
    );

    expect(targetDocument.addCount).toBe(1);
    unregisterFirst();
    unregisterFirst();
    expect(targetDocument.removeCount).toBe(0);
    unregisterSecond();
    expect(targetDocument.removeCount).toBe(1);
  });

  test("toggles maximise only for an unhandled primary-button drag double click", () => {
    let toggleMaximiseCalls = 0;
    const targetDocument = new FakeDocument();
    const unregister = registerWindowsTitlebarDoubleClick(
      targetDocument as unknown as Document,
      () => {
        toggleMaximiseCalls += 1;
      },
    );

    const accepted = fakeDoubleClick(fakeElement(targetDocument));
    targetDocument.dispatchDoubleClick(accepted.event);
    expect(toggleMaximiseCalls).toBe(1);
    expect(accepted.preventDefaultCalls()).toBe(1);

    const interactive = fakeDoubleClick(
      fakeElement(targetDocument, { interactive: true }),
    );
    targetDocument.dispatchDoubleClick(interactive.event);
    targetDocument.dispatchDoubleClick(
      fakeDoubleClick(fakeElement(targetDocument), { button: 2 }).event,
    );
    targetDocument.dispatchDoubleClick(
      fakeDoubleClick(fakeElement(targetDocument), {
        defaultPrevented: true,
      }).event,
    );

    expect(toggleMaximiseCalls).toBe(1);
    expect(interactive.preventDefaultCalls()).toBe(0);
    unregister();
  });
});
