import * as React from "react";
import { Events } from "@wailsio/runtime";
import { Loader2, Sparkles, WandSparkles } from "lucide-react";

import type {
  Settings,
  UpdateSettingsRequest,
} from "@/shared/contracts/settings";
import type { Sprite } from "@/shared/contracts/sprites";
import { getXiaText } from "@/features/xiadown/shared";
import {
  mergeSpritePreferences,
  resolveActiveSprite,
} from "@/features/sprites/shared";
import { LocalSpriteGalleryCard } from "@/features/sprites/card";
import { useHttpBaseURL } from "@/shared/query/runtime";
import { useHideSettingsWindow, useShowMainWindow } from "@/shared/query/settings";
import { useSprites } from "@/shared/query/sprites";
import { buildAssetPreviewURL } from "@/shared/utils/resourceHelpers";
import { Button } from "@/shared/ui/button";

type XiaText = ReturnType<typeof getXiaText>;

export function SpritesSection(props: {
  settings: Settings | null | undefined;
  text: XiaText;
  saveSettingsPatch: (patch: UpdateSettingsRequest) => Promise<void>;
}) {
  const { settings, text, saveSettingsPatch } = props;
  const { data: sprites = [], isLoading } = useSprites();
  const { data: httpBaseURL = "" } = useHttpBaseURL();
  const showMainWindow = useShowMainWindow();
  const hideSettingsWindow = useHideSettingsWindow();
  const readySprites = React.useMemo(
    () => sprites.filter((sprite) => sprite.status === "ready"),
    [sprites],
  );
  const activeSprite = React.useMemo(
    () => resolveActiveSprite(readySprites, settings),
    [readySprites, settings],
  );
  const activeSpriteId = activeSprite?.id ?? "";

  const handleActivateSprite = React.useCallback(
    async (sprite: Sprite) => {
      await saveSettingsPatch({
        appearanceConfig: mergeSpritePreferences(settings, {
          activeSpriteId: sprite.id,
        }),
      });
    },
    [saveSettingsPatch, settings],
  );

  const openSpriteStudio = React.useCallback(async () => {
    await showMainWindow.mutateAsync();
    void Events.Emit("sprites:studio:navigate", { action: "gallery" });
    void hideSettingsWindow.mutateAsync();
  }, [hideSettingsWindow, showMainWindow]);

  return (
    <div className="w-full min-w-0">
      <div className="mb-4 flex items-center justify-between gap-3">
        <div className="flex min-w-0 items-center gap-2 text-sm font-semibold text-foreground">
          <Sparkles className="h-4 w-4 shrink-0 text-primary" />
          <span className="truncate">{text.spriteStudio.gallery}</span>
        </div>
        <Button
          type="button"
          variant="outline"
          size="compact"
          onClick={() => void openSpriteStudio()}
          disabled={showMainWindow.isPending || hideSettingsWindow.isPending}
        >
          <WandSparkles className="h-4 w-4" />
          {text.spriteStudio.title}
        </Button>
      </div>

      {isLoading ? (
        <div className="flex h-32 items-center justify-center text-muted-foreground">
          <Loader2 className="h-5 w-5 animate-spin" />
        </div>
      ) : readySprites.length > 0 ? (
        <div className="flex flex-wrap gap-4">
          {readySprites.map((sprite) => (
            <LocalSpriteGalleryCard
              key={sprite.id}
              sprite={sprite}
              imageUrl={buildSpriteImageURL(httpBaseURL, sprite)}
              isDefault={activeSpriteId === sprite.id}
              onClick={() => void handleActivateSprite(sprite)}
            />
          ))}
        </div>
      ) : null}
    </div>
  );
}

function buildSpriteImageURL(httpBaseURL: string, sprite: Sprite | null) {
  if (!httpBaseURL || !sprite?.spritePath) {
    return "";
  }
  return buildAssetPreviewURL(httpBaseURL, sprite.spritePath, sprite.updatedAt);
}
