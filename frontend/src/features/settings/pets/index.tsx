import * as React from "react";
import { Events } from "@wailsio/runtime";
import { PawPrint, Images } from "lucide-react";

import type {
  Settings,
  UpdateSettingsRequest,
} from "@/shared/contracts/settings";
import type { Pet } from "@/shared/contracts/pets";
import { getXiaText } from "@/features/xiadown/shared";
import { cn } from "@/lib/utils";
import {
  mergePetPreferences,
  resolveActivePet,
} from "@/features/pets/shared";
import { LocalPetGalleryCard } from "@/features/pets/card";
import { PET_GALLERY_CARD_SIZE_CLASS } from "@/shared/styles/xiadown";
import { useHttpBaseURL } from "@/shared/query/runtime";
import { useHideSettingsWindow, useShowMainWindow } from "@/shared/query/settings";
import { usePets } from "@/shared/query/pets";
import { buildAssetPreviewURL } from "@/shared/utils/resourceHelpers";
import { Button } from "@/shared/ui/button";

type XiaText = ReturnType<typeof getXiaText>;

const SETTINGS_PETS_PREVIEW_LIMIT = 24;

export function PetsSection(props: {
  settings: Settings | null | undefined;
  text: XiaText;
  saveSettingsPatch: (patch: UpdateSettingsRequest) => Promise<void>;
}) {
  const { settings, text, saveSettingsPatch } = props;
  const { data: pets = [], isLoading } = usePets();
  const { data: httpBaseURL = "" } = useHttpBaseURL();
  const showMainWindow = useShowMainWindow();
  const hideSettingsWindow = useHideSettingsWindow();
  const readyPets = React.useMemo(
    () => pets.filter((pet) => pet.status === "ready"),
    [pets],
  );
  const activePet = React.useMemo(
    () => resolveActivePet(readyPets, settings),
    [readyPets, settings],
  );
  const activePetId = activePet?.id ?? "";
  const visiblePets = React.useMemo(
    () => resolveVisibleSettingsPets(readyPets, activePetId),
    [activePetId, readyPets],
  );
  const hiddenPetCount = Math.max(readyPets.length - visiblePets.length, 0);

  const handleActivatePet = React.useCallback(
    async (pet: Pet) => {
      await saveSettingsPatch({
        appearanceConfig: mergePetPreferences(settings, {
          activePetId: pet.id,
        }),
      });
    },
    [saveSettingsPatch, settings],
  );

  const openPetsGallery = React.useCallback(async () => {
    await showMainWindow.mutateAsync();
    void Events.Emit("pets:gallery:navigate", { action: "gallery" });
    void hideSettingsWindow.mutateAsync();
  }, [hideSettingsWindow, showMainWindow]);

  return (
    <div className="w-full min-w-0">
      <div className="mb-4 flex items-center justify-between gap-3">
        <div className="flex min-w-0 items-center gap-2 text-sm font-semibold text-foreground">
          <PawPrint className="h-4 w-4 shrink-0 text-primary" />
          <span className="truncate">{text.petGallery.gallery}</span>
          {hiddenPetCount > 0 ? (
            <span className="shrink-0 text-xs font-medium text-muted-foreground">
              {visiblePets.length} / {readyPets.length}
            </span>
          ) : null}
        </div>
        <Button
          type="button"
          variant="outline"
          size="compact"
          onClick={() => void openPetsGallery()}
          disabled={showMainWindow.isPending || hideSettingsWindow.isPending}
        >
          <Images className="h-4 w-4" />
          {text.petGallery.title}
        </Button>
      </div>

      {isLoading ? (
        <div className="flex flex-wrap gap-4" aria-hidden="true">
          {Array.from({ length: 4 }).map((_, index) => (
            <PetGallerySkeletonCard key={index} />
          ))}
        </div>
      ) : readyPets.length > 0 ? (
        <div className="flex flex-wrap gap-4">
          {visiblePets.map((pet) => (
            <LocalPetGalleryCard
              key={pet.id}
              pet={pet}
              imageUrl={buildPetImageURL(httpBaseURL, pet)}
              isDefault={activePetId === pet.id}
              onClick={() => void handleActivatePet(pet)}
            />
          ))}
        </div>
      ) : null}
    </div>
  );
}

function resolveVisibleSettingsPets(pets: Pet[], activePetId: string) {
  if (pets.length <= SETTINGS_PETS_PREVIEW_LIMIT) {
    return pets;
  }

  if (!activePetId) {
    return pets.slice(0, SETTINGS_PETS_PREVIEW_LIMIT);
  }

  const activePet = pets.find((pet) => pet.id === activePetId);
  if (!activePet) {
    return pets.slice(0, SETTINGS_PETS_PREVIEW_LIMIT);
  }

  return [
    activePet,
    ...pets
      .filter((pet) => pet.id !== activePetId)
      .slice(0, SETTINGS_PETS_PREVIEW_LIMIT - 1),
  ];
}

function buildPetImageURL(httpBaseURL: string, pet: Pet | null) {
  if (!httpBaseURL || !pet?.spritesheetPath) {
    return "";
  }
  return buildAssetPreviewURL(httpBaseURL, pet.spritesheetPath, pet.updatedAt);
}

function PetGallerySkeletonCard() {
  return (
    <div
      className={cn(
        "app-pets-skeleton-card relative isolate flex flex-col items-center overflow-hidden px-3 pb-3 pt-3",
        PET_GALLERY_CARD_SIZE_CLASS,
      )}
    >
      <div className="app-pets-skeleton-glow absolute inset-0 animate-pulse" />
      <div className="relative z-10 flex min-h-0 w-full flex-1 items-center justify-center">
        <div className="app-pets-skeleton-avatar h-[4.75rem] w-[4.75rem] animate-pulse rounded-[18px]" />
      </div>
      <div className="app-pets-skeleton-label relative z-10 mt-2 h-3.5 w-16 animate-pulse rounded-full" />
    </div>
  );
}
