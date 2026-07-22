import { Info, UserRound, X } from "lucide-react";
import * as React from "react";

import { Button } from "@/shared/ui/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogScrollArea,
  DialogTitle,
  DialogTrigger,
} from "@/shared/ui/dialog";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/shared/ui/tooltip";

import { buildListenImageCandidates } from "@/app/main/listen/storage";

export type ListenArtistInfoDialogProps = {
  httpBaseURL: string;
  title: string;
  subtitle: string;
  description?: string;
  thumbnailUrl?: string;
  heroThumbnailUrl?: string;
  infoLabel: string;
  biographyLabel: string;
  closeLabel: string;
};

export function ListenArtistInfoDialog(props: ListenArtistInfoDialogProps) {
  const biographyID = React.useId();
  const leadPhotoURL =
    props.heroThumbnailUrl?.trim() || props.thumbnailUrl?.trim() || "";

  return (
    <Dialog>
      <Tooltip>
        <TooltipTrigger asChild>
          <DialogTrigger asChild>
            <Button
              type="button"
              variant="glass"
              size="icon"
              shape="circle"
              className="listen-muse-artist-hero__action"
              aria-label={props.infoLabel}
              data-artist-action="info"
              data-size="small"
            >
              <Info aria-hidden="true" />
            </Button>
          </DialogTrigger>
        </TooltipTrigger>
        <TooltipContent side="bottom" sideOffset={10}>
          {props.infoLabel}
        </TooltipContent>
      </Tooltip>

      <DialogContent
        className="listen-artist-info-dialog"
        showCloseButton={false}
      >
        <div className="listen-artist-info-dialog__hero">
          <ListenArtistInfoImage
            httpBaseURL={props.httpBaseURL}
            rawURL={leadPhotoURL}
            alt=""
            className="listen-artist-info-dialog__hero-image"
          />
          <span
            className="listen-artist-info-dialog__hero-fallback"
            aria-hidden="true"
          >
            <UserRound />
          </span>
          <span
            className="listen-artist-info-dialog__hero-veil"
            aria-hidden="true"
          />
          <DialogClose asChild>
            <Button
              type="button"
              variant="glass"
              size="compactIcon"
              shape="circle"
              className="listen-artist-info-dialog__close"
              aria-label={props.closeLabel}
            >
              <X aria-hidden="true" />
            </Button>
          </DialogClose>
        </div>

        <DialogScrollArea className="listen-artist-info-dialog__scroll">
          <header className="listen-artist-info-dialog__identity">
            <DialogTitle className="listen-artist-info-dialog__title">
              {props.title}
            </DialogTitle>
            {props.subtitle ? (
              <DialogDescription className="listen-artist-info-dialog__subtitle">
                {props.subtitle}
              </DialogDescription>
            ) : (
              <DialogDescription className="sr-only">
                {props.infoLabel}
              </DialogDescription>
            )}
          </header>

          {props.description ? (
            <section
              className="listen-artist-info-dialog__section"
              aria-labelledby={biographyID}
            >
              <h2
                id={biographyID}
                className="listen-artist-info-dialog__heading"
              >
                {props.biographyLabel}
              </h2>
              <p className="listen-artist-info-dialog__biography">
                {props.description}
              </p>
            </section>
          ) : null}

        </DialogScrollArea>
      </DialogContent>
    </Dialog>
  );
}

function ListenArtistInfoImage(props: {
  httpBaseURL: string;
  rawURL: string;
  alt: string;
  className: string;
}) {
  const candidates = React.useMemo(
    () => buildListenImageCandidates(props.httpBaseURL, props.rawURL),
    [props.httpBaseURL, props.rawURL],
  );
  const candidateKey = candidates.join("\n");
  const [candidateIndex, setCandidateIndex] = React.useState(0);
  const [imageUnavailable, setImageUnavailable] = React.useState(false);
  const source = imageUnavailable ? "" : candidates[candidateIndex] ?? "";

  React.useEffect(() => {
    setCandidateIndex(0);
    setImageUnavailable(false);
  }, [candidateKey]);

  if (!source) {
    return null;
  }

  return (
    <img
      src={source}
      alt={props.alt}
      className={props.className}
      loading="lazy"
      onError={() => {
        if (candidateIndex < candidates.length - 1) {
          setCandidateIndex((current) => current + 1);
          return;
        }
        setImageUnavailable(true);
      }}
    />
  );
}
