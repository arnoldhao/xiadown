import { Captions } from "lucide-react";

import type { ListenLyricsKind } from "@/app/main/listen/types";

export function resolveListenLyricsIcon(_kind?: ListenLyricsKind | null) {
  return Captions;
}
