import { create } from "zustand"
import { persist } from "zustand/middleware"

import type { TranscodePreset } from "@/shared/contracts/library"

export type {
  FFmpegSpeedPreset,
  TranscodePreset,
  TranscodePresetOutputType,
  TranscodeQualityMode,
  TranscodeScaleMode,
} from "@/shared/contracts/library"

type TranscodePresetState = {
  customPresets: TranscodePreset[]
  addPreset: (preset: TranscodePreset) => void
  updatePreset: (preset: TranscodePreset) => void
  removePreset: (id: string) => void
  setCustomPresets: (presets: TranscodePreset[]) => void
}

export const useTranscodePresetStore = create<TranscodePresetState>()(
  persist(
    (set) => ({
      customPresets: [],
      addPreset: (preset) =>
        set((state) => ({
          customPresets: [...state.customPresets, preset],
        })),
      updatePreset: (preset) =>
        set((state) => ({
          customPresets: state.customPresets.map((item) => (item.id === preset.id ? preset : item)),
        })),
      removePreset: (id) =>
        set((state) => ({
          customPresets: state.customPresets.filter((item) => item.id !== id),
        })),
      setCustomPresets: (presets) => set({ customPresets: presets }),
    }),
    {
      name: "transcode-presets",
    }
  )
)
