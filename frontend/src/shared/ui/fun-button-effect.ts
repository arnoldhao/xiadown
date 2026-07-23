export const FUN_BUTTON_EFFECTS = [
  "aurora",
  "prism",
  "comet",
  "plasma",
  "magnetic",
  "ripple",
  "stardust",
  "orbit",
  "shards",
  "hologram",
] as const;

export type FunButtonEffect = (typeof FUN_BUTTON_EFFECTS)[number];

export function pickFunButtonEffect(): FunButtonEffect {
  const index = Math.floor(Math.random() * FUN_BUTTON_EFFECTS.length);
  return FUN_BUTTON_EFFECTS[index] ?? "aurora";
}
