const WINDOWS_TITLEBAR_INTERACTIVE_SELECTOR = [
  ".wails-no-drag",
  "button",
  "a[href]",
  "input",
  "select",
  "textarea",
  "summary",
  "label",
  "[contenteditable]:not([contenteditable='false'])",
  "[draggable='true']",
  "[role='button']",
  "[role='checkbox']",
  "[role='combobox']",
  "[role='link']",
  "[role='menuitem']",
  "[role='option']",
  "[role='radio']",
  "[role='slider']",
  "[role='spinbutton']",
  "[role='switch']",
  "[role='tab']",
  "[tabindex]:not([tabindex='-1'])",
].join(",");

function eventTargetElement(target: EventTarget | null): Element | null {
  if (!target) {
    return null;
  }
  if (typeof (target as Element).closest === "function") {
    return target as Element;
  }
  return (target as Node).parentElement ?? null;
}

export function isWindowsTitlebarDoubleClickTarget(
  target: EventTarget | null,
) {
  const element = eventTargetElement(target);
  if (!element || !element.closest(".wails-drag")) {
    return false;
  }
  if (element.closest(WINDOWS_TITLEBAR_INTERACTIVE_SELECTOR)) {
    return false;
  }

  const view =
    element.ownerDocument?.defaultView ??
    (typeof window === "undefined" ? null : window);
  return (
    view
      ?.getComputedStyle(element)
      .getPropertyValue("--wails-draggable")
      .trim() === "drag"
  );
}

type WindowsTitlebarRegistration = {
  count: number;
  listener: (event: MouseEvent) => void;
};

const windowsTitlebarDoubleClickRegistrations = new WeakMap<
  Document,
  WindowsTitlebarRegistration
>();

export function registerWindowsTitlebarDoubleClick(
  targetDocument: Document,
  toggleMaximise: () => void,
) {
  let registration =
    windowsTitlebarDoubleClickRegistrations.get(targetDocument);
  if (!registration) {
    const listener = (event: MouseEvent) => {
      if (
        event.defaultPrevented ||
        event.button !== 0 ||
        !isWindowsTitlebarDoubleClickTarget(event.target)
      ) {
        return;
      }
      event.preventDefault();
      toggleMaximise();
    };
    registration = { count: 0, listener };
    windowsTitlebarDoubleClickRegistrations.set(targetDocument, registration);
    targetDocument.addEventListener("dblclick", listener);
  }
  registration.count += 1;

  let active = true;
  return () => {
    if (!active) {
      return;
    }
    active = false;

    const current = windowsTitlebarDoubleClickRegistrations.get(targetDocument);
    if (!current) {
      return;
    }
    current.count = Math.max(0, current.count - 1);
    if (current.count > 0) {
      return;
    }
    windowsTitlebarDoubleClickRegistrations.delete(targetDocument);
    targetDocument.removeEventListener("dblclick", current.listener);
  };
}
