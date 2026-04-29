export type SpriteAnimation =
  | "greeting"
  | "snoring"
  | "upset"
  | "celebrate"
  | "seeking"
  | "embarrassed"
  | "working"
  | "listeningToMusic";

export const SPRITE_ROW_BY_ANIMATION: Record<SpriteAnimation, number> = {
  greeting: 0,
  snoring: 1,
  upset: 2,
  celebrate: 3,
  seeking: 4,
  embarrassed: 5,
  working: 6,
  listeningToMusic: 7,
};
