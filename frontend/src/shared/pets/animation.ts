export type PetAnimation =
  | "idle"
  | "running-right"
  | "running-left"
  | "waving"
  | "jumping"
  | "failed"
  | "waiting"
  | "running"
  | "review";

export interface PetAnimationDefinition {
  row: number;
  frames: number;
  durations: number[];
}

const repeat = (duration: number, count: number) => Array.from({ length: count }, () => duration);

export const PET_ANIMATION_DEFINITIONS: Record<PetAnimation, PetAnimationDefinition> = {
  idle: { row: 0, frames: 6, durations: [280, 110, 110, 140, 140, 320] },
  "running-right": { row: 1, frames: 8, durations: [...repeat(120, 7), 220] },
  "running-left": { row: 2, frames: 8, durations: [...repeat(120, 7), 220] },
  waving: { row: 3, frames: 4, durations: [140, 140, 140, 280] },
  jumping: { row: 4, frames: 5, durations: [140, 140, 140, 140, 280] },
  failed: { row: 5, frames: 8, durations: [...repeat(140, 7), 240] },
  waiting: { row: 6, frames: 6, durations: [150, 150, 150, 150, 150, 260] },
  running: { row: 7, frames: 6, durations: [120, 120, 120, 120, 120, 220] },
  review: { row: 8, frames: 6, durations: [150, 150, 150, 150, 150, 280] },
};

export const PET_ANIMATION_NAMES = Object.keys(PET_ANIMATION_DEFINITIONS) as PetAnimation[];
