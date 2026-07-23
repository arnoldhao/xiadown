export type InputModality = "keyboard" | "pointer" | "unknown";

const KEYBOARD_INTERACTION_KEYS = new Set([
  " ",
  "ArrowDown",
  "ArrowLeft",
  "ArrowRight",
  "ArrowUp",
  "End",
  "Enter",
  "Escape",
  "Home",
  "PageDown",
  "PageUp",
  "Tab",
]);

type KeyboardInteraction = Pick<
  KeyboardEvent,
  "altKey" | "ctrlKey" | "key" | "metaKey"
>;

export function isKeyboardInteraction(event: KeyboardInteraction) {
  return (
    !event.altKey &&
    !event.ctrlKey &&
    !event.metaKey &&
    KEYBOARD_INTERACTION_KEYS.has(event.key)
  );
}

/**
 * WebKit's :focus-visible heuristic can vary between OS/WebView releases.
 * Track the user's actual input modality so pointer focus never inherits a
 * browser-style halo while keyboard and assistive focus keep their fallback.
 */
export function installInputModalityTracking(documentNode: Document) {
  const root = documentNode.documentElement;
  const previousModality = root.dataset.inputModality;

  root.dataset.inputModality = "unknown" satisfies InputModality;

  const handlePointerDown = () => {
    root.dataset.inputModality = "pointer" satisfies InputModality;
  };
  const handleKeyDown = (event: KeyboardEvent) => {
    if (isKeyboardInteraction(event)) {
      root.dataset.inputModality = "keyboard" satisfies InputModality;
    }
  };

  documentNode.addEventListener("pointerdown", handlePointerDown, true);
  documentNode.addEventListener("keydown", handleKeyDown, true);

  return () => {
    documentNode.removeEventListener("pointerdown", handlePointerDown, true);
    documentNode.removeEventListener("keydown", handleKeyDown, true);
    if (previousModality === undefined) {
      delete root.dataset.inputModality;
      return;
    }
    root.dataset.inputModality = previousModality;
  };
}
