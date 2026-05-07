import * as React from "react";

import type { Pet } from "@/shared/contracts/pets";
import { cn } from "@/lib/utils";
import {
  PET_GALLERY_CARD_SIZE_CLASS,
  resolvePetCardLighting,
} from "@/shared/styles/xiadown";
import { PetDisplay } from "@/shared/ui/pet-player";

export type LocalPetGalleryCardProps = Omit<React.ButtonHTMLAttributes<HTMLButtonElement>, "children"> & {
  pet: Pet;
  imageUrl: string;
  isDefault: boolean;
};

export function LocalPetGalleryCard(props: LocalPetGalleryCardProps) {
  const {
    pet,
    imageUrl,
    isDefault,
    className,
    type = "button",
    ...buttonProps
  } = props;
  const buttonRef = React.useRef<HTMLButtonElement | null>(null);
  const [inView, setInView] = React.useState(false);
  const lighting = resolvePetCardLighting(pet, isDefault);

  React.useEffect(() => {
    const node = buttonRef.current;
    if (!node || typeof IntersectionObserver === "undefined") {
      setInView(true);
      return;
    }
    const observer = new IntersectionObserver(
      ([entry]) => setInView(Boolean(entry?.isIntersecting)),
      {
        rootMargin: "192px 0px",
        threshold: 0.01,
      },
    );
    observer.observe(node);
    return () => observer.disconnect();
  }, []);

  return (
    <button
      ref={buttonRef}
      type={type}
      className={cn(
        "app-pets-gallery-card group relative isolate flex flex-col items-center overflow-hidden px-3 pb-3 pt-3 text-center transition duration-200",
        PET_GALLERY_CARD_SIZE_CLASS,
        lighting.cardClassName,
        className,
      )}
      {...buttonProps}
    >
      <div
        className={cn(
          "pointer-events-none absolute inset-0 z-0 rounded-[inherit]",
          lighting.primaryGlowClassName,
        )}
      />
      <div
        className={cn(
          "pointer-events-none absolute inset-0 z-0 rounded-[inherit]",
          lighting.directionalWashClassName,
        )}
      />
      <div
        className={cn(
          "pointer-events-none absolute inset-0 z-0 rounded-[inherit]",
          lighting.rimGlowClassName,
        )}
      />
      {lighting.spotlightClassName ? (
        <div
          className={cn(
            "pointer-events-none absolute inset-0 rounded-[inherit]",
            lighting.spotlightClassName,
          )}
        />
      ) : null}
      <div className="relative z-20 flex min-h-0 w-full flex-1 items-center justify-center">
        <PetDisplay
          pet={pet}
          imageUrl={imageUrl}
          alt={pet.displayName}
          animation="running"
          animate={inView}
          glowClassName={lighting.petGlowClassName}
          glowStyle={lighting.petGlowStyle}
        />
      </div>
      <div className="relative z-30 mt-2 w-full truncate px-1 text-sm font-medium leading-5 text-foreground">
        {pet.displayName}
      </div>
    </button>
  );
}
