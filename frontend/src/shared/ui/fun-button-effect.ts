export type FunButtonEffect =
  | "water"
  | "fire"
  | "cloud"
  | "sun"
  | "mist"
  | "shadow";

export const FUN_BUTTON_EFFECTS: FunButtonEffect[] = [
  "water",
  "fire",
  "cloud",
  "sun",
  "mist",
  "shadow",
];

export function pickFunButtonEffect(): FunButtonEffect {
  const index = Math.floor(Math.random() * FUN_BUTTON_EFFECTS.length);
  return FUN_BUTTON_EFFECTS[index] ?? "water";
}
