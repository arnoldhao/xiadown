import * as React from "react";

import type { Sprite } from "@/shared/contracts/sprites";
import { cn } from "@/lib/utils";
import {
  SPRITE_GALLERY_CARD_SIZE_CLASS,
  resolveSpriteCardLighting,
} from "@/shared/styles/xiadown";
import { SpriteDisplay } from "@/shared/ui/sprite-player";

export type LocalSpriteGalleryCardProps = Omit<React.ButtonHTMLAttributes<HTMLButtonElement>, "children"> & {
  sprite: Sprite;
  imageUrl: string;
  isDefault: boolean;
};

export function LocalSpriteGalleryCard(props: LocalSpriteGalleryCardProps) {
  const {
    sprite,
    imageUrl,
    isDefault,
    className,
    type = "button",
    ...buttonProps
  } = props;
  const lighting = resolveSpriteCardLighting(sprite, isDefault);

  return (
    <button
      type={type}
      className={cn(
        "group relative isolate flex flex-col items-center overflow-hidden rounded-[22px] border px-3 pb-3 pt-3 text-center transition duration-200",
        SPRITE_GALLERY_CARD_SIZE_CLASS,
        lighting.cardClassName,
        className,
      )}
      {...buttonProps}
    >
      <div
        className={cn(
          "pointer-events-none absolute inset-0 z-0 rounded-[22px]",
          lighting.primaryGlowClassName,
        )}
      />
      <div
        className={cn(
          "pointer-events-none absolute inset-0 z-0 rounded-[22px]",
          lighting.directionalWashClassName,
        )}
      />
      <div
        className={cn(
          "pointer-events-none absolute inset-0 z-0 rounded-[22px]",
          lighting.rimGlowClassName,
        )}
      />
      {lighting.spotlightClassName ? (
        <div
          className={cn(
            "pointer-events-none absolute inset-0 rounded-[22px]",
            lighting.spotlightClassName,
          )}
        />
      ) : null}
      <div className="relative z-20 flex min-h-0 w-full flex-1 items-center justify-center">
        <SpriteDisplay
          sprite={sprite}
          imageUrl={imageUrl}
          staticImageUrl={sprite.coverImageDataUrl || undefined}
          alt={sprite.name}
          animate={false}
          glowClassName={lighting.spriteGlowClassName}
          glowStyle={lighting.spriteGlowStyle}
        />
      </div>
      <div className="relative z-30 mt-2 w-full truncate px-1 text-sm font-medium leading-5 text-foreground">
        {sprite.name}
      </div>
    </button>
  );
}
