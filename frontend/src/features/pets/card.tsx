import * as React from "react";

import type { Pet } from "@/shared/contracts/pets";
import { cn } from "@/lib/utils";
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
  const lightingVariant = pet.origin === "online"
    ? "online"
    : pet.scope === "imported"
      ? "imported"
      : "default";

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
      className={cn("app-pets-gallery-card", className)}
      data-default={isDefault ? "true" : undefined}
      data-lighting={lightingVariant}
      {...buttonProps}
    >
      <div className="app-pets-gallery-card-light app-pets-gallery-card-light--primary" />
      <div className="app-pets-gallery-card-light app-pets-gallery-card-light--directional" />
      <div className="app-pets-gallery-card-light app-pets-gallery-card-light--rim" />
      {isDefault ? (
        <div className="app-pets-gallery-card-light app-pets-gallery-card-light--spotlight" />
      ) : null}
      <div className="app-pets-gallery-card-pet">
        <PetDisplay
          pet={pet}
          imageUrl={imageUrl}
          alt={pet.displayName}
          animation="running"
          animate={inView}
          load={inView}
          glowVariant={isDefault ? "gallery-default" : undefined}
        />
      </div>
      <div className="app-pets-gallery-card-label">
        {pet.displayName}
      </div>
    </button>
  );
}
