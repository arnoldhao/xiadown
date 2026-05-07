import * as React from "react";
import { System } from "@wailsio/runtime";
import {
  AlertCircle,
  ArrowLeft,
  CheckCircle2,
  Download,
  Eye,
  ExternalLink,
  FileArchive,
  Globe2,
  HelpCircle,
  Loader2,
  PawPrint,
  Trash2,
  Upload,
} from "lucide-react";

import { WindowControls } from "@/components/layout/WindowControls";
import { LocalPetGalleryCard } from "@/features/pets/card";
import { mergePetPreferences, resolveActivePet } from "@/features/pets/shared";
import { getXiaText } from "@/features/xiadown/shared";
import { cn } from "@/lib/utils";
import type { Pet } from "@/shared/contracts/pets";
import { messageBus } from "@/shared/message";
import {
  useExportPet,
  useDeletePet,
  useFinishOnlinePetImportSession,
  useImportPet,
  useInspectPetSource,
  useOnlinePetImportSession,
  usePets,
  useStartOnlinePetImport,
} from "@/shared/query/pets";
import { useHttpBaseURL } from "@/shared/query/runtime";
import { useUpdateSettings } from "@/shared/query/settings";
import { Badge } from "@/shared/ui/badge";
import { Button } from "@/shared/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/shared/ui/dropdown-menu";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/shared/ui/dialog";
import { DreamSegmentSwitch } from "@/shared/ui/dream-segment-switch";
import { Select } from "@/shared/ui/select";
import { PetDisplay } from "@/shared/ui/pet-player";
import { openFileDialog } from "@/shared/utils/dialogHelpers";
import {
  buildAssetPreviewURL,
  getPathBaseName,
  stripPathExtension,
} from "@/shared/utils/resourceHelpers";
import {
  PET_ANIMATION_NAMES,
  type PetAnimation,
} from "@/shared/pets/animation";
import {
  PET_GALLERY_CONTEXT_MENU_CONTENT_CLASS_NAME,
  PET_GALLERY_CONTEXT_MENU_ICON_SLOT_CLASS_NAME,
  PET_GALLERY_CONTEXT_MENU_ITEM_CLASS_NAME,
} from "@/shared/styles/xiadown";
import type { Settings } from "@/shared/contracts/settings";
import type { OnlinePetImportSession, PetImportDraft } from "@/shared/contracts/pets";

export type PetsGalleryNavigation = {
  action: "gallery" | "detail";
  petId?: string;
  nonce: number;
};

type PetContextMenuTarget = {
  petId: string;
  x: number;
  y: number;
};

const PET_GALLERY_INITIAL_LIMIT = 48;
const PET_GALLERY_PAGE_SIZE = 24;

type XiaText = ReturnType<typeof getXiaText>;

export function PetsGalleryPage(props: {
  text: XiaText;
  settings: Settings | null;
  navigation: PetsGalleryNavigation | null;
}) {
  const { text, settings, navigation } = props;
  const isWindows = System.IsWindows();
  const petsQuery = usePets();
  const { data: httpBaseURL = "" } = useHttpBaseURL();
  const updateSettings = useUpdateSettings();
  const exportPet = useExportPet();
  const deletePet = useDeletePet();
  const [selectedPetId, setSelectedPetId] = React.useState("");
  const [guideOpen, setGuideOpen] = React.useState(false);
  const [importOpen, setImportOpen] = React.useState(false);
  const [contextMenuTarget, setContextMenuTarget] = React.useState<PetContextMenuTarget | null>(null);
  const [deleteConfirmPet, setDeleteConfirmPet] = React.useState<Pet | null>(null);
  const [deleteConfirmError, setDeleteConfirmError] = React.useState("");
  const [galleryLimit, setGalleryLimit] = React.useState(PET_GALLERY_INITIAL_LIMIT);
  const [animation, setAnimation] = React.useState<PetAnimation>("running");
  const pets = petsQuery.data ?? [];
  const readyPets = React.useMemo(() => pets.filter((pet) => pet.status === "ready"), [pets]);
  const activePet = React.useMemo(() => resolveActivePet(readyPets, settings), [readyPets, settings]);
  const galleryPets = React.useMemo(
    () => sortGalleryPets(readyPets, activePet?.id ?? ""),
    [activePet?.id, readyPets],
  );
  const visibleGalleryPets = React.useMemo(
    () => galleryPets.slice(0, galleryLimit),
    [galleryLimit, galleryPets],
  );
  const hasMoreGalleryPets = visibleGalleryPets.length < galleryPets.length;
  const selectedPet = React.useMemo(
    () => pets.find((pet) => pet.id === selectedPetId) ?? null,
    [pets, selectedPetId],
  );
  const contextMenuPet = React.useMemo(
    () => pets.find((pet) => pet.id === contextMenuTarget?.petId) ?? null,
    [contextMenuTarget?.petId, pets],
  );
  const mode = selectedPet ? "detail" : "gallery";

  React.useEffect(() => {
    if (!navigation) {
      return;
    }
    if (navigation.action === "detail" && navigation.petId) {
      setSelectedPetId(navigation.petId);
      return;
    }
    setSelectedPetId("");
  }, [navigation]);

  React.useEffect(() => {
    if (!selectedPetId) {
      return;
    }
    if (petsQuery.isFetched && !pets.some((pet) => pet.id === selectedPetId)) {
      setSelectedPetId("");
    }
  }, [pets, petsQuery.isFetched, selectedPetId]);

  React.useEffect(() => {
    setGalleryLimit((current) => Math.min(Math.max(current, PET_GALLERY_INITIAL_LIMIT), Math.max(galleryPets.length, PET_GALLERY_INITIAL_LIMIT)));
  }, [galleryPets.length]);

  const setActivePet = React.useCallback(
    async (pet: Pet) => {
      await updateSettings.mutateAsync({
        appearanceConfig: mergePetPreferences(settings, {
          activePetId: pet.id,
        }),
      });
    },
    [settings, updateSettings],
  );

  const handleExportPet = React.useCallback(
    async (pet: Pet) => {
      const selection = await openFileDialog({
        Title: text.petGallery.exportTitle,
        CanChooseDirectories: true,
        CanChooseFiles: false,
        CanCreateDirectories: true,
        AllowsOtherFiletypes: true,
        ButtonText: text.petGallery.exportAction,
      });
      const directoryPath = resolveDialogPath(selection);
      if (!directoryPath) {
        return;
      }
      try {
        await exportPet.mutateAsync({
          id: pet.id,
          outputPath: buildPetArchivePath(directoryPath, pet.displayName),
        });
        messageBus.publishToast({
          intent: "success",
          title: text.petGallery.exportSucceeded,
        });
      } catch (error) {
        messageBus.publishToast({
          intent: "danger",
          title: text.petGallery.exportTitle,
          description: resolvePetError(error, text),
        });
      }
    },
    [exportPet, text, text.petGallery],
  );

  const openPetContextMenu = React.useCallback((event: React.MouseEvent, pet: Pet) => {
    event.preventDefault();
    event.stopPropagation();
    setContextMenuTarget({
      petId: pet.id,
      x: event.clientX,
      y: event.clientY,
    });
  }, []);

  const handleViewContextMenuPet = React.useCallback(() => {
    if (!contextMenuPet) {
      return;
    }
    setSelectedPetId(contextMenuPet.id);
    setContextMenuTarget(null);
  }, [contextMenuPet]);

  const handleSetDefaultContextMenuPet = React.useCallback(() => {
    if (!contextMenuPet || contextMenuPet.id === activePet?.id) {
      return;
    }
    setContextMenuTarget(null);
    void setActivePet(contextMenuPet);
  }, [activePet?.id, contextMenuPet, setActivePet]);

  const handleDeleteContextMenuPet = React.useCallback(() => {
    if (!contextMenuPet || contextMenuPet.scope !== "imported") {
      return;
    }
    setDeleteConfirmError("");
    setDeleteConfirmPet(contextMenuPet);
    setContextMenuTarget(null);
  }, [contextMenuPet]);

  const executeDeletePet = React.useCallback(async () => {
    const pet = deleteConfirmPet;
    if (!pet || deletePet.isPending) {
      return;
    }
    setDeleteConfirmError("");
    try {
      await deletePet.mutateAsync({ id: pet.id });
      if (selectedPetId === pet.id) {
        setSelectedPetId("");
      }
      setDeleteConfirmPet(null);
      messageBus.publishToast({
        intent: "success",
        title: text.petGallery.deleteSucceeded,
      });
    } catch (error) {
      setDeleteConfirmError(resolvePetError(error, text));
    }
  }, [deleteConfirmPet, deletePet, selectedPetId, text]);

  const petContextMenu = (
    <DropdownMenu
      open={Boolean(contextMenuTarget)}
      onOpenChange={(open) => {
        if (!open) {
          setContextMenuTarget(null);
        }
      }}
    >
      {contextMenuTarget ? (
        <DropdownMenuTrigger asChild>
          <button
            type="button"
            aria-hidden="true"
            tabIndex={-1}
            className="fixed z-50 h-px w-px opacity-0 outline-none"
            style={{
              left: contextMenuTarget.x,
              top: contextMenuTarget.y,
            }}
          />
        </DropdownMenuTrigger>
      ) : null}
      <DropdownMenuContent
        side="bottom"
        align="start"
        sideOffset={2}
        className={PET_GALLERY_CONTEXT_MENU_CONTENT_CLASS_NAME}
      >
        <div className="p-1">
          <DropdownMenuItem
            className={PET_GALLERY_CONTEXT_MENU_ITEM_CLASS_NAME}
            disabled={!contextMenuPet || contextMenuPet.id === activePet?.id || updateSettings.isPending}
            onSelect={handleSetDefaultContextMenuPet}
          >
            <div className={PET_GALLERY_CONTEXT_MENU_ICON_SLOT_CLASS_NAME}>
              <PawPrint className="h-4 w-4" />
            </div>
            <span className="truncate font-medium">
              {contextMenuPet?.id === activePet?.id ? text.petGallery.activePet : text.petGallery.setActive}
            </span>
          </DropdownMenuItem>
          <DropdownMenuItem
            className={PET_GALLERY_CONTEXT_MENU_ITEM_CLASS_NAME}
            disabled={!contextMenuPet}
            onSelect={handleViewContextMenuPet}
          >
            <div className={PET_GALLERY_CONTEXT_MENU_ICON_SLOT_CLASS_NAME}>
              <Eye className="h-4 w-4" />
            </div>
            <span className="truncate font-medium">{text.actions.view}</span>
          </DropdownMenuItem>
          <DropdownMenuItem
            className={PET_GALLERY_CONTEXT_MENU_ITEM_CLASS_NAME}
            disabled={!contextMenuPet || contextMenuPet.scope !== "imported"}
            onSelect={handleDeleteContextMenuPet}
          >
            <div className={PET_GALLERY_CONTEXT_MENU_ICON_SLOT_CLASS_NAME}>
              <Trash2 className="h-4 w-4" />
            </div>
            <span className="truncate font-medium">{text.actions.deleteItem}</span>
          </DropdownMenuItem>
        </div>
      </DropdownMenuContent>
    </DropdownMenu>
  );

  const petDeleteDialog = (
    <Dialog
      open={Boolean(deleteConfirmPet)}
      onOpenChange={(open) => {
        if (deletePet.isPending) {
          return;
        }
        if (!open) {
          setDeleteConfirmPet(null);
          setDeleteConfirmError("");
        }
      }}
    >
      <DialogContent className="grid h-[min(14rem,calc(100vh-2rem))] w-[min(24rem,calc(100vw-2rem))] max-w-none grid-rows-[auto_minmax(0,1fr)_auto] gap-3 overflow-hidden">
        <DialogHeader className="min-w-0">
          <DialogTitle className="overflow-hidden break-words pr-6 text-left leading-[1.35] [display:-webkit-box] [-webkit-box-orient:vertical] [-webkit-line-clamp:2]">
            {text.petGallery.deleteTitle}
          </DialogTitle>
          <DialogDescription className="overflow-hidden break-words text-left leading-5 [display:-webkit-box] [-webkit-box-orient:vertical] [-webkit-line-clamp:3]">
            {deleteConfirmPet
              ? formatPetTemplate(text.petGallery.deleteMessage, {
                  name: deleteConfirmPet.displayName || deleteConfirmPet.id,
                })
              : ""}
          </DialogDescription>
        </DialogHeader>
        <div className="min-h-0 overflow-hidden">
          {deleteConfirmError ? (
            <div className="overflow-hidden break-words text-xs leading-5 text-destructive [display:-webkit-box] [-webkit-box-orient:vertical] [-webkit-line-clamp:2]">
              {deleteConfirmError}
            </div>
          ) : null}
        </div>
        <div className="app-dialog-footer flex flex-nowrap items-center justify-between gap-2">
          <DialogClose asChild>
            <Button type="button" variant="outline" disabled={deletePet.isPending}>
              {text.actions.cancelDialog}
            </Button>
          </DialogClose>
          <Button
            type="button"
            variant="destructive"
            disabled={!deleteConfirmPet || deletePet.isPending}
            onClick={() => void executeDeletePet()}
          >
            {deletePet.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
            {text.actions.deleteItem}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );

  return (
    <div className="app-main-page app-main-pets-page relative flex h-full min-w-0 flex-1 flex-col overflow-hidden bg-background">
      <div className="app-main-page-header wails-drag flex min-h-[3.75rem] items-center justify-between gap-4 border-b border-border/60 px-5 pb-3 pt-4">
        <div className="flex min-w-0 items-center gap-3">
          {mode === "detail" ? (
            <Button
              type="button"
              variant="ghost"
              size="icon"
              className="wails-no-drag h-8 w-8 rounded-full"
              onClick={() => setSelectedPetId("")}
              aria-label={text.actions.back}
            >
              <ArrowLeft className="h-4 w-4" />
            </Button>
          ) : null}
          <div className="flex min-w-0 items-center gap-2 text-sm font-semibold text-foreground">
            <PawPrint className="h-4 w-4 shrink-0 text-primary" />
            <span className="truncate">
              {mode === "detail" && selectedPet
                ? `${text.petGallery.title} / ${selectedPet.displayName}`
                : text.petGallery.title}
            </span>
          </div>
        </div>
        <div
          className={cn(
            "flex min-w-0 items-center justify-end gap-2",
            isWindows && "min-w-[var(--app-windows-caption-control-width)]",
          )}
        >
          {isWindows ? <WindowControls platform="windows" /> : null}
        </div>
      </div>

      {mode === "detail" && selectedPet ? (
        <PetDetailView
          text={text}
          pet={selectedPet}
          imageUrl={buildPetImageURL(httpBaseURL, selectedPet)}
          active={activePet?.id === selectedPet.id}
          animation={animation}
          onAnimationChange={setAnimation}
          onSetActive={() => void setActivePet(selectedPet)}
          onExport={() => void handleExportPet(selectedPet)}
          onDelete={() => {
            setDeleteConfirmError("");
            setDeleteConfirmPet(selectedPet);
          }}
          exporting={exportPet.isPending}
          deleting={deletePet.isPending && deleteConfirmPet?.id === selectedPet.id}
          canDelete={selectedPet.scope === "imported"}
        />
      ) : (
        <div className="app-pets-gallery-content min-h-0 flex-1 overflow-y-auto px-6 py-5">
          <div className="app-pets-gallery-toolbar mb-5 flex items-center justify-between gap-3">
            <div className="flex min-w-0 items-center gap-2 text-sm font-semibold text-foreground">
              <PawPrint className="h-4 w-4 shrink-0 text-primary" />
              <span className="truncate">{text.petGallery.localPets}</span>
            </div>
            <div className="flex shrink-0 items-center gap-2">
              <Button type="button" variant="outline" size="compact" onClick={() => setGuideOpen(true)}>
                <HelpCircle className="h-4 w-4" />
                {text.petGallery.generationGuide.action}
              </Button>
              <Button type="button" variant="default" size="compact" onClick={() => setImportOpen(true)}>
                <Upload className="h-4 w-4" />
                {text.petGallery.importAction}
              </Button>
            </div>
          </div>

          {petsQuery.isLoading ? (
            <div className="app-pets-loading flex h-56 items-center justify-center">
              <Loader2 className="h-5 w-5 animate-spin" />
            </div>
          ) : galleryPets.length > 0 ? (
            <>
              <div className="flex flex-wrap gap-4">
                {visibleGalleryPets.map((pet) => (
                  <LocalPetGalleryCard
                    key={pet.id}
                    pet={pet}
                    imageUrl={buildPetImageURL(httpBaseURL, pet)}
                    isDefault={activePet?.id === pet.id}
                    onClick={() => setSelectedPetId(pet.id)}
                    onContextMenu={(event) => openPetContextMenu(event, pet)}
                  />
                ))}
              </div>
              {hasMoreGalleryPets ? (
                <div className="mt-5 flex justify-center">
                  <Button
                    type="button"
                    variant="outline"
                    size="compact"
                    onClick={() =>
                      setGalleryLimit((current) =>
                        Math.min(current + PET_GALLERY_PAGE_SIZE, galleryPets.length),
                      )
                    }
                  >
                    {text.petGallery.showMore}
                  </Button>
                </div>
              ) : null}
            </>
          ) : (
            <div className="app-pets-empty-state flex h-56 items-center justify-center text-sm">
              {text.petGallery.empty}
            </div>
          )}
        </div>
      )}

      <GenerationGuideDialog
        text={text}
        open={guideOpen}
        onOpenChange={setGuideOpen}
      />
      <PetImportDialog
        text={text}
        open={importOpen}
        onOpenChange={setImportOpen}
      />
      {petContextMenu}
      {petDeleteDialog}
    </div>
  );
}

function PetDetailView(props: {
  text: XiaText;
  pet: Pet;
  imageUrl: string;
  active: boolean;
  animation: PetAnimation;
  onAnimationChange: (animation: PetAnimation) => void;
  onSetActive: () => void;
  onExport: () => void;
  onDelete: () => void;
  exporting: boolean;
  deleting: boolean;
  canDelete: boolean;
}) {
  const { text, pet, imageUrl, animation } = props;
  const sourceLabel = resolvePetSourceLabel(text, pet);

  return (
    <div className="app-pets-gallery-content min-h-0 flex-1 overflow-y-auto px-6 py-5">
      <div className="mx-auto flex max-w-6xl flex-col gap-5">
        <section className="app-pets-detail-grid grid gap-5">
          <div className="app-pets-detail-card flex min-h-[16.75rem] items-center justify-center p-5">
            <PetDisplay
              pet={pet}
              imageUrl={imageUrl}
              animation={animation}
              alt={pet.displayName}
              size={218}
              glowClassName="h-[18rem] w-[23rem] blur-2xl"
            />
          </div>
          <div className="app-pets-detail-card flex min-h-[16.75rem] flex-col p-4">
            <div className="shrink-0 text-base font-semibold text-foreground">{pet.displayName}</div>
            <div className="mt-3 min-h-0 flex-1 overflow-y-auto pr-1">
              {pet.description ? (
                <div className="text-sm leading-6 text-muted-foreground">{pet.description}</div>
              ) : null}
              <PetMetric label={text.petGallery.scopeLabel} value={sourceLabel} />
            </div>
            <div className="app-dream-button-group app-pets-detail-actions mt-4 grid shrink-0 grid-cols-3">
              <Button
                type="button"
                variant="ghost"
                className="app-pets-detail-action"
                data-active={props.active ? "true" : undefined}
                onClick={props.onSetActive}
                disabled={props.active}
              >
                <PawPrint className="h-4 w-4" />
                <span className="truncate">{props.active ? text.petGallery.activePet : text.petGallery.setActive}</span>
              </Button>
              <Button
                type="button"
                variant="ghost"
                className="app-pets-detail-action"
                onClick={props.onExport}
                disabled={props.exporting}
              >
                {props.exporting ? <Loader2 className="h-4 w-4 animate-spin" /> : <Download className="h-4 w-4" />}
                <span className="truncate">{text.petGallery.exportAction}</span>
              </Button>
              <Button
                type="button"
                variant="ghost"
                className="app-pets-detail-action text-destructive"
                onClick={props.onDelete}
                disabled={!props.canDelete || props.deleting}
              >
                {props.deleting ? <Loader2 className="h-4 w-4 animate-spin" /> : <Trash2 className="h-4 w-4" />}
                <span className="truncate">{text.actions.deleteItem}</span>
              </Button>
            </div>
          </div>
        </section>

        <PetAnimationPreviewGrid
          text={text}
          pet={pet}
          imageUrl={imageUrl}
          activeAnimation={animation}
          onAnimationChange={props.onAnimationChange}
        />
      </div>
    </div>
  );
}

function PetAnimationPreviewGrid(props: {
  text: XiaText;
  pet: Pet;
  imageUrl: string;
  activeAnimation: PetAnimation;
  onAnimationChange: (animation: PetAnimation) => void;
}) {
  return (
    <section className="app-pets-animation-grid flex flex-wrap gap-3">
      {PET_ANIMATION_NAMES.map((name) => (
        <button
          key={name}
          type="button"
          className={cn(
            "app-pets-animation-card flex h-36 w-32 flex-col items-center justify-center p-3 text-center",
            props.activeAnimation === name && "is-active",
          )}
          data-active={props.activeAnimation === name ? "true" : undefined}
          onClick={() => props.onAnimationChange(name)}
        >
          <PetDisplay
            pet={props.pet}
            imageUrl={props.imageUrl}
            animation={name}
            alt={props.text.petGallery.animations[name]}
            size={78}
            glowClassName="h-24 w-28 blur-lg opacity-65"
          />
          <span className="mt-2 w-full truncate text-xs font-medium text-foreground">
            {props.text.petGallery.animations[name]}
          </span>
        </button>
      ))}
    </section>
  );
}

function PetMetric(props: { label: string; value: string }) {
  return (
    <div className="app-pets-metric mt-3 flex min-w-0 items-center justify-between gap-3 text-xs">
      <div className="app-pets-metric-label shrink-0">{props.label}</div>
      <div className="app-pets-metric-value min-w-0 truncate font-medium">{props.value}</div>
    </div>
  );
}

function GenerationGuideDialog(props: {
  text: XiaText;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const guide = props.text.petGallery.generationGuide;
  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className="max-w-2xl">
        <DialogHeader className="text-left">
          <DialogTitle>{guide.title}</DialogTitle>
          <DialogDescription className="sr-only">{guide.title}</DialogDescription>
        </DialogHeader>
        <div className="max-h-[62vh] space-y-4 overflow-x-hidden overflow-y-auto pr-1">
          <section className="grid gap-3 sm:grid-cols-2">
            {guide.steps.map((step, index) => (
              <div key={step.title} className="app-pets-guide-step p-4">
                <div className="mb-3 flex items-center gap-2">
                  <span className="app-pets-guide-index flex h-7 w-7 shrink-0 items-center justify-center rounded-full text-xs font-semibold">
                    {index + 1}
                  </span>
                  <h3 className="min-w-0 truncate text-sm font-semibold text-foreground">{step.title}</h3>
                </div>
                <p className="text-sm leading-6 text-muted-foreground">{step.description}</p>
              </div>
            ))}
          </section>
          <section className="app-pets-guide-tips p-4">
            <h3 className="text-sm font-semibold text-foreground">{guide.greatPetTitle}</h3>
            <ul className="mt-3 space-y-2 text-sm leading-6 text-muted-foreground">
              {guide.greatPetTips.map((tip) => (
                <li key={tip} className="flex gap-2">
                  <span className="app-pets-guide-bullet mt-2 h-1.5 w-1.5 shrink-0 rounded-full" />
                  <span>{tip}</span>
                </li>
              ))}
            </ul>
          </section>
        </div>
        <DialogFooter>
          <DialogClose asChild>
            <Button type="button" variant="default">{props.text.actions.close}</Button>
          </DialogClose>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

type PetImportMode = "online" | "local";

const PET_IMPORT_SITE_OPTIONS = [
  {
    id: "codex-pets-net",
  },
  {
    id: "codexpet-xyz",
  },
] as const;

function PetImportDialog(props: {
  text: XiaText;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const { text } = props;
  const dialogText = text.petGallery.importDialog;
  const inspectPet = useInspectPetSource();
  const importPet = useImportPet();
  const startOnlineImport = useStartOnlinePetImport();
  const finishOnlineImport = useFinishOnlinePetImportSession();
  const [mode, setMode] = React.useState<PetImportMode>("online");
  const [siteId, setSiteId] = React.useState("codex-pets-net");
  const [sessionId, setSessionId] = React.useState("");
  const [sessionSnapshot, setSessionSnapshot] = React.useState<OnlinePetImportSession | null>(null);
  const [localPath, setLocalPath] = React.useState("");
  const [localDraft, setLocalDraft] = React.useState<PetImportDraft | null>(null);
  const [localError, setLocalError] = React.useState("");
  const [importedLocalPets, setImportedLocalPets] = React.useState<Pet[]>([]);

  const sessionQuery = useOnlinePetImportSession(
    { sessionId },
    props.open && sessionId.trim().length > 0 && !finishOnlineImport.isPending,
  );
  const onlineSession = sessionQuery.data ?? sessionSnapshot;
  const importedPets = React.useMemo(
    () => mergeImportedPets(importedLocalPets, onlineSession?.importedPets ?? []),
    [importedLocalPets, onlineSession?.importedPets],
  );
  const browserStatus = startOnlineImport.isPending
    ? "opening"
    : onlineSession?.browserStatus || (sessionId ? "open" : "not_open");
  const onlineError = sessionQuery.error
    ? resolvePetError(sessionQuery.error, text)
    : startOnlineImport.error
      ? resolvePetError(startOnlineImport.error, text)
      : resolvePetSessionError(onlineSession, text);

  React.useEffect(() => {
    if (sessionQuery.data) {
      setSessionSnapshot(sessionQuery.data);
    }
  }, [sessionQuery.data]);

  React.useEffect(() => {
    if (!props.open) {
      setMode("online");
      setSiteId("codex-pets-net");
      setSessionId("");
      setSessionSnapshot(null);
      setLocalPath("");
      setLocalDraft(null);
      setLocalError("");
      setImportedLocalPets([]);
    }
  }, [props.open]);

  const handleChooseLocalFile = React.useCallback(async () => {
    setLocalError("");
    const selection = await openFileDialog({
      Title: text.petGallery.importTitle,
      AllowsOtherFiletypes: false,
      CanChooseDirectories: false,
      CanChooseFiles: true,
      Filters: [{ DisplayName: text.petGallery.petPackageFilter, Pattern: "*.zip" }],
    });
    const path = resolveDialogPath(selection);
    if (!path) {
      return;
    }
    setLocalPath(path);
    setLocalDraft(null);
    try {
      const draft = await inspectPet.mutateAsync({ path });
      setLocalDraft(draft);
      if (draft.status !== "ready") {
        setLocalError(resolvePetValidationError(draft, text));
      }
    } catch (error) {
      setLocalError(resolvePetError(error, text));
    }
  }, [inspectPet, text, text.petGallery]);

  const handleImportLocalFile = React.useCallback(async () => {
    if (!localDraft || localDraft.status !== "ready") {
      return;
    }
    setLocalError("");
    try {
      const pet = await importPet.mutateAsync({ path: localDraft.path, origin: "local" });
      setImportedLocalPets((current) => mergeImportedPets(current, [pet]));
      setLocalPath("");
      setLocalDraft(null);
      messageBus.publishToast({
        intent: "success",
        title: text.petGallery.importSucceeded,
      });
    } catch (error) {
      setLocalError(resolvePetError(error, text));
    }
  }, [importPet, localDraft, text, text.petGallery.importSucceeded]);

  const handleBrowseOnline = React.useCallback(async () => {
    try {
      const session = await startOnlineImport.mutateAsync({ siteId });
      setSessionId(session.sessionId);
      setSessionSnapshot(session);
    } catch (error) {
      messageBus.publishToast({
        intent: "danger",
        title: text.petGallery.importFailedTitle,
        description: resolvePetError(error, text),
      });
    }
  }, [siteId, startOnlineImport, text, text.petGallery.importFailedTitle]);

  const handleCompleteImport = React.useCallback(async () => {
    if (startOnlineImport.isPending || importPet.isPending || inspectPet.isPending) {
      return;
    }
    const currentSessionId = sessionId.trim();
    if (currentSessionId) {
      try {
        await finishOnlineImport.mutateAsync({ sessionId: currentSessionId });
      } catch (error) {
        const details = parsePetError(error);
        if (details.code !== "pet_online_session_not_found") {
          messageBus.publishToast({
            intent: "danger",
            title: text.petGallery.importFailedTitle,
            description: resolvePetErrorDetails(details, text),
          });
          return;
        }
      }
    }
    props.onOpenChange(false);
  }, [
    finishOnlineImport,
    importPet.isPending,
    inspectPet.isPending,
    props,
    sessionId,
    startOnlineImport.isPending,
    text,
    text.petGallery.importFailedTitle,
  ]);

  const handleOpenChange = React.useCallback(
    (open: boolean) => {
      if (open) {
        props.onOpenChange(true);
        return;
      }
      void handleCompleteImport();
    },
    [handleCompleteImport, props],
  );

  const selectedSiteLabel = resolvePetImportSiteLabel(text, siteId);
  const localReady = localDraft?.status === "ready";
  const finishPending =
    finishOnlineImport.isPending ||
    startOnlineImport.isPending ||
    inspectPet.isPending ||
    importPet.isPending;

  return (
    <Dialog open={props.open} onOpenChange={handleOpenChange}>
      <DialogContent className="max-w-[min(92vw,34rem)] gap-4 overflow-hidden" showCloseButton={false}>
        <DialogHeader className="space-y-0 text-left">
          <DialogTitle className="sr-only">{text.petGallery.importTitle}</DialogTitle>
          <DialogDescription className="sr-only">{dialogText.description}</DialogDescription>
          <DreamSegmentSwitch
            value={mode}
            className="mr-auto"
            items={[
              {
                value: "online",
                label: dialogText.online,
                icon: <Globe2 className="h-3.5 w-3.5" />,
              },
              {
                value: "local",
                label: dialogText.local,
                icon: <FileArchive className="h-3.5 w-3.5" />,
              },
            ]}
            onValueChange={setMode}
          />
        </DialogHeader>

        <div className="max-h-[min(68vh,34rem)] space-y-4 overflow-x-hidden overflow-y-auto pr-1">
          {mode === "online" ? (
            <div className="app-pets-import-section space-y-3 p-4">
              <div className="text-xs leading-5 text-muted-foreground">{dialogText.onlineDescription}</div>
              <div className="flex gap-2">
                <Select
                  className="min-w-0 flex-1"
                  value={siteId}
                  onChange={(event) => setSiteId(event.target.value)}
                  disabled={Boolean(sessionId) || startOnlineImport.isPending}
                  aria-label={dialogText.onlineSite}
                >
                  {PET_IMPORT_SITE_OPTIONS.map((site) => (
                    <option key={site.id} value={site.id}>
                      {resolvePetImportSiteLabel(text, site.id)}
                    </option>
                  ))}
                </Select>
                <Button
                  type="button"
                  size="compact"
                  onClick={() => void handleBrowseOnline()}
                  disabled={Boolean(sessionId) || startOnlineImport.isPending}
                >
                  {startOnlineImport.isPending ? (
                    <Loader2 className="h-4 w-4 animate-spin" />
                  ) : (
                    <ExternalLink className="h-4 w-4" />
                  )}
                  {dialogText.browse}
                </Button>
              </div>
              <div className="app-pets-info-box grid gap-2 p-3 text-xs">
                <ImportInfoRow label={dialogText.onlineSite} value={selectedSiteLabel} />
                <ImportInfoRow
                  label={dialogText.browserStatus}
                  value={resolvePetImportBrowserStatusLabel(text, browserStatus)}
                />
              </div>
              {onlineError ? (
                <ImportStatusMessage intent="danger" icon={<AlertCircle className="h-4 w-4" />}>
                  {onlineError}
                </ImportStatusMessage>
              ) : null}
            </div>
          ) : null}

          {mode === "local" ? (
            !localPath ? (
              <div className="app-pets-import-section flex justify-center p-4">
                <Button
                  type="button"
                  size="compact"
                  onClick={() => void handleChooseLocalFile()}
                  disabled={inspectPet.isPending || importPet.isPending}
                >
                  {inspectPet.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <FileArchive className="h-4 w-4" />}
                  {text.actions.chooseFile}
                </Button>
              </div>
            ) : (
              <div className="app-pets-import-section space-y-3 p-4">
                <div className="app-pets-info-box grid gap-2 p-3 text-xs">
                  <ImportInfoRow label={dialogText.fileName} value={getPathBaseName(localPath)} />
                  <ImportInfoRow label={dialogText.path} value={localPath} />
                  {localDraft ? (
                    <>
                      <ImportInfoRow label={dialogText.petName} value={localDraft.displayName || "-"} />
                      <ImportInfoRow label={text.petGallery.sizeLabel} value={`${localDraft.imageWidth} x ${localDraft.imageHeight}`} />
                      <ImportInfoRow label={text.petGallery.gridLabel} value={`${localDraft.columns} x ${localDraft.rows}`} />
                    </>
                  ) : null}
                </div>
                {localDraft ? (
                  <ImportStatusMessage
                    intent={localReady ? "success" : "danger"}
                    icon={localReady ? <CheckCircle2 className="h-4 w-4" /> : <AlertCircle className="h-4 w-4" />}
                  >
                    {localReady ? dialogText.validationReady : resolvePetValidationError(localDraft, text)}
                  </ImportStatusMessage>
                ) : null}
                {localError && !localDraft ? (
                  <ImportStatusMessage intent="danger" icon={<AlertCircle className="h-4 w-4" />}>
                    {localError}
                  </ImportStatusMessage>
                ) : null}
                <div className="app-pets-import-actions flex justify-end gap-2">
                  <Button
                    type="button"
                    variant="outline"
                    size="compact"
                    onClick={() => void handleChooseLocalFile()}
                    disabled={inspectPet.isPending || importPet.isPending}
                  >
                    {text.actions.chooseFile}
                  </Button>
                  <Button
                    type="button"
                    size="compact"
                    onClick={() => void handleImportLocalFile()}
                    disabled={!localReady || importPet.isPending}
                  >
                    {importPet.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Upload className="h-4 w-4" />}
                    {dialogText.importSelected}
                  </Button>
                </div>
              </div>
            )
          ) : null}

          <div className="app-pets-import-section p-4">
            <div className="flex items-center justify-between gap-3">
              <div className="text-sm font-medium text-foreground">{dialogText.importedPets}</div>
              <Badge variant="outline">{formatImportedCount(dialogText.importedCount, importedPets.length)}</Badge>
            </div>
            {importedPets.length > 0 ? (
              <div className="mt-3 flex flex-wrap gap-2">
                {importedPets.map((pet) => (
                  <Badge key={pet.id} variant="secondary" className="max-w-full truncate">
                    {pet.displayName}
                  </Badge>
                ))}
              </div>
            ) : (
              <div className="mt-3 text-xs text-muted-foreground">{dialogText.importedEmpty}</div>
            )}
          </div>
        </div>

        <DialogFooter>
          <Button type="button" variant="default" onClick={() => void handleCompleteImport()} disabled={finishPending}>
            {finishPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <CheckCircle2 className="h-4 w-4" />}
            {dialogText.finish}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function ImportInfoRow(props: { label: string; value: string }) {
  return (
    <div className="app-pets-info-row flex min-w-0 items-center justify-between gap-3">
      <span className="shrink-0">{props.label}</span>
      <span className="min-w-0 truncate text-right font-medium">{props.value}</span>
    </div>
  );
}

function ImportStatusMessage(props: {
  intent: "success" | "danger";
  icon: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <div
      className="app-pets-status-message flex items-start gap-2 px-3 py-2 text-xs leading-5"
      data-intent={props.intent}
    >
      <span className="mt-0.5 shrink-0">{props.icon}</span>
      <span className="min-w-0">{props.children}</span>
    </div>
  );
}

function resolvePetImportSiteLabel(text: XiaText, siteId: string) {
  switch (siteId) {
    case "codexpet-xyz":
      return text.petGallery.importDialog.sites.codexpetXyz;
    case "codex-pets-net":
    default:
      return text.petGallery.importDialog.sites.codexPetsNet;
  }
}

function resolvePetImportBrowserStatusLabel(text: XiaText, status: string) {
  const labels = text.petGallery.importDialog.browserStatuses;
  switch (status) {
    case "opening":
      return labels.opening;
    case "open":
      return labels.open;
    case "browser_closed":
      return labels.browserClosed;
    case "completed":
      return labels.completed;
    case "failed":
      return labels.failed;
    case "not_open":
    default:
      return labels.notOpen;
  }
}

function formatImportedCount(template: string, count: number) {
  return formatPetTemplate(template, { count: String(count) });
}

function sortGalleryPets(pets: Pet[], activePetId: string) {
  const activeID = activePetId.trim();
  return [...pets].sort((left, right) => {
    const leftActive = Boolean(activeID && left.id === activeID);
    const rightActive = Boolean(activeID && right.id === activeID);
    if (leftActive !== rightActive) {
      return leftActive ? -1 : 1;
    }
    if (left.scope !== right.scope) {
      return left.scope === "builtin" ? -1 : 1;
    }
    return left.displayName.localeCompare(right.displayName);
  });
}

function formatPetTemplate(template: string, values: Record<string, string>) {
  return Object.entries(values).reduce(
    (next, [key, value]) => next.split(`{${key}}`).join(value),
    template,
  );
}

function mergeImportedPets(...groups: Pet[][]) {
  const byID = new Map<string, Pet>();
  for (const group of groups) {
    for (const pet of group) {
      if (pet.id) {
        byID.set(pet.id, pet);
      }
    }
  }
  return [...byID.values()];
}

function resolveDialogPath(selection: string | string[] | null | undefined) {
  if (Array.isArray(selection)) {
    return selection.find((item) => item.trim())?.trim() ?? "";
  }
  return typeof selection === "string" ? selection.trim() : "";
}

function buildPetImageURL(httpBaseURL: string, pet: Pet | null) {
  if (!httpBaseURL || !pet?.spritesheetPath) {
    return "";
  }
  return buildAssetPreviewURL(httpBaseURL, pet.spritesheetPath, pet.updatedAt);
}

function buildPetArchivePath(directory: string, name: string) {
  const baseName = sanitizeArchiveName(stripPathExtension(getPathBaseName(name)) || name || "pet");
  return `${directory.replace(/[\\/]+$/, "")}/${baseName}.zip`;
}

function resolvePetSourceLabel(text: XiaText, pet: Pet) {
  if (pet.scope === "builtin") {
    return text.petGallery.scopes.builtin;
  }
  const origin = pet.origin?.trim().toLowerCase() ?? "";
  if (!origin || origin === "direct" || origin === "local") {
    return text.petGallery.origins.localImport;
  }
  if (origin === "codexpet.xyz") {
    return text.petGallery.origins.codexpetXyz;
  }
  return text.petGallery.origins.codexPetsNet;
}

function sanitizeArchiveName(value: string) {
  return value.trim().replace(/[\\/:*?"<>|]+/g, "-").replace(/\s+/g, "-") || "pet";
}

function resolvePetError(error: unknown, text?: XiaText) {
  return resolvePetErrorDetails(parsePetError(error), text);
}

function resolvePetSessionError(session: OnlinePetImportSession | null | undefined, text: XiaText) {
  if (!session) {
    return "";
  }
  return resolvePetErrorDetails({ code: session.errorCode, message: session.error }, text);
}

function resolvePetValidationError(draft: PetImportDraft, text: XiaText) {
  return resolvePetErrorDetails(
    {
      code: draft.validationCode,
      message: draft.validationMessage || text.petGallery.importInvalid,
    },
    text,
  );
}

function resolvePetErrorDetails(details: PetErrorDetails, text?: XiaText) {
  const translated = translatePetErrorCode(details.code, text);
  if (translated) {
    return translated;
  }
  return details.message || details.code || "";
}

function translatePetErrorCode(code: string | undefined, text?: XiaText) {
  const trimmed = String(code ?? "").trim();
  if (!trimmed || !text) {
    return "";
  }
  const errors = text.petGallery.errors;
  switch (trimmed) {
    case "pet_package_path_required":
      return errors.packagePathRequired;
    case "pet_package_unsupported_type":
      return errors.packageUnsupportedType;
    case "pet_package_read_failed":
      return errors.packageReadFailed;
    case "pet_package_too_large":
      return errors.packageTooLarge;
    case "pet_package_open_failed":
      return errors.packageOpenFailed;
    case "pet_package_missing_manifest":
      return errors.missingManifest;
    case "pet_package_missing_spritesheet":
      return errors.missingSpritesheet;
    case "pet_package_contents_too_large":
      return errors.packageContentsTooLarge;
    case "pet_archive_file_open_failed":
      return errors.archiveFileOpenFailed;
    case "pet_archive_file_read_failed":
      return errors.archiveFileReadFailed;
    case "pet_manifest_decode_failed":
      return errors.manifestDecodeFailed;
    case "pet_spritesheet_decode_failed":
      return errors.spritesheetDecodeFailed;
    case "pet_spritesheet_size_invalid":
      return errors.spritesheetSizeInvalid;
    case "pet_online_download_canceled":
      return errors.onlineDownloadCanceled;
    case "pet_online_session_required":
      return errors.onlineSessionRequired;
    case "pet_online_session_not_found":
      return errors.onlineSessionNotFound;
    case "pet_online_unsupported_site":
      return errors.onlineUnsupportedSite;
    default:
      return "";
  }
}

type PetErrorDetails = {
  code?: string;
  message?: string;
};

function parsePetError(error: unknown): PetErrorDetails {
  const parsed = parsePetErrorFromUnknown(error);
  if (parsed.code || parsed.message) {
    return parsed;
  }
  if (error instanceof Error) {
    return { message: error.message };
  }
  return { message: String(error ?? "") };
}

function parsePetErrorFromUnknown(error: unknown): PetErrorDetails {
  if (error && typeof error === "object" && "message" in error) {
    const direct = (error as { message?: unknown }).message;
    if (typeof direct === "string") {
      return parsePetErrorFromString(direct);
    }
  }
  if (typeof error === "string") {
    return parsePetErrorFromString(error);
  }
  return {};
}

function parsePetErrorFromString(value: string): PetErrorDetails {
  const parsed = parsePetErrorJSON(value);
  if (!parsed) {
    return { message: value.trim() };
  }
  const code = stringFromPetErrorField(parsed.code) || stringFromPetErrorField(parsed.errorCode);
  const message = stringFromPetErrorField(parsed.message) || stringFromPetErrorField(parsed.error);
  if (message) {
    const nested = parsePetErrorFromString(message);
    return {
      code: code || nested.code,
      message: nested.code ? nested.message : message,
    };
  }
  return { code };
}

function parsePetErrorJSON(value: string): Record<string, unknown> | null {
  const trimmed = value.trim();
  if (!trimmed) {
    return null;
  }
  const candidates = [trimmed];
  const start = trimmed.indexOf("{");
  const end = trimmed.lastIndexOf("}");
  if (start >= 0 && end > start) {
    candidates.push(trimmed.slice(start, end + 1));
  }
  for (const candidate of candidates) {
    try {
      const parsed = JSON.parse(candidate) as unknown;
      if (parsed && typeof parsed === "object") {
        return parsed as Record<string, unknown>;
      }
    } catch {
      // Try the next JSON-looking candidate.
    }
  }
  return null;
}

function stringFromPetErrorField(value: unknown) {
  return typeof value === "string" ? value.trim() : "";
}
