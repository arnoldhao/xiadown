import {
  ArrowLeft,
  Loader2,
  Play,
  Shuffle,
  UserCheck,
  UserPlus,
  UserRound,
} from "lucide-react";
import * as React from "react";

import { Button } from "@/shared/ui/button";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/shared/ui/tooltip";

import { ListenArtistInfoDialog } from "@/app/main/listen/ArtistInfoDialog";
import { buildListenImageCandidates } from "@/app/main/listen/storage";

type ListenArtistActionBusy = "" | "mix" | "subscribe";

export type ListenMuseArtistHeroProps = {
  httpBaseURL: string;
  title: string;
  subtitle: string;
  description?: string;
  thumbnailUrl?: string;
  heroThumbnailUrl?: string;
  backLabel: string;
  infoLabel: string;
  biographyLabel: string;
  closeLabel: string;
  shuffleLabel: string;
  mixLabel: string;
  subscribeLabel: string;
  unsubscribeLabel: string;
  showActions: boolean;
  subscribed: boolean;
  actionBusy: ListenArtistActionBusy;
  shuffleDisabled: boolean;
  mixDisabled: boolean;
  subscribeDisabled: boolean;
  onBack: () => void;
  onShuffle: () => void;
  onMix: () => void;
  onToggleSubscription: () => void;
};

export function ListenMuseArtistHero(props: ListenMuseArtistHeroProps) {
  const subscriptionLabel = props.subscribed
    ? props.unsubscribeLabel
    : props.subscribeLabel;

  return (
    <section className="listen-muse-artist-hero" data-listen-artist-hero="true">
      <div className="listen-muse-artist-hero__body wails-drag">
        <ListenMuseArtistHeroArtwork
          httpBaseURL={props.httpBaseURL}
          title={props.title}
          thumbnailUrl={props.thumbnailUrl}
          heroThumbnailUrl={props.heroThumbnailUrl}
        />
        <span className="listen-muse-artist-hero__veil" aria-hidden="true" />

        <Button
          type="button"
          variant="glass"
          size="compactIcon"
          shape="circle"
          className="listen-muse-artist-hero__back wails-no-drag"
          aria-label={props.backLabel}
          title={props.backLabel}
          onClick={props.onBack}
        >
          <ArrowLeft className="h-4 w-4" />
        </Button>

        <div className="listen-muse-artist-hero__details">
          <div className="listen-muse-artist-hero__copy">
            <h1 className="listen-muse-artist-hero__title">{props.title}</h1>
            {props.subtitle ? (
              <p className="listen-muse-artist-hero__subtitle">
                {props.subtitle}
              </p>
            ) : null}
          </div>

          {props.showActions ? (
            <TooltipProvider delayDuration={0}>
              <div className="listen-muse-artist-hero__actions wails-no-drag">
                <ListenArtistInfoDialog
                  httpBaseURL={props.httpBaseURL}
                  title={props.title}
                  subtitle={props.subtitle}
                  description={props.description}
                  thumbnailUrl={props.thumbnailUrl}
                  heroThumbnailUrl={props.heroThumbnailUrl}
                  infoLabel={props.infoLabel}
                  biographyLabel={props.biographyLabel}
                  closeLabel={props.closeLabel}
                />
                <ListenMuseArtistHeroAction
                  action="shuffle"
                  size="large"
                  label={props.shuffleLabel}
                  disabled={props.shuffleDisabled}
                  onClick={props.onShuffle}
                >
                  <Shuffle aria-hidden="true" />
                </ListenMuseArtistHeroAction>
                <ListenMuseArtistHeroAction
                  action="mix"
                  size="large"
                  label={props.mixLabel}
                  busy={props.actionBusy === "mix"}
                  disabled={props.mixDisabled}
                  onClick={props.onMix}
                >
                  {props.actionBusy === "mix" ? (
                    <Loader2 aria-hidden="true" className="listen-loading-spinner" />
                  ) : (
                    <Play
                      aria-hidden="true"
                      className="listen-muse-artist-hero__play-icon"
                    />
                  )}
                </ListenMuseArtistHeroAction>
                <ListenMuseArtistHeroAction
                  action="subscribe"
                  size="small"
                  label={subscriptionLabel}
                  active={props.subscribed}
                  busy={props.actionBusy === "subscribe"}
                  disabled={props.subscribeDisabled}
                  onClick={props.onToggleSubscription}
                >
                  {props.actionBusy === "subscribe" ? (
                    <Loader2 aria-hidden="true" className="listen-loading-spinner" />
                  ) : props.subscribed ? (
                    <UserCheck aria-hidden="true" />
                  ) : (
                    <UserPlus aria-hidden="true" />
                  )}
                </ListenMuseArtistHeroAction>
              </div>
            </TooltipProvider>
          ) : null}
        </div>
      </div>
    </section>
  );
}

function ListenMuseArtistHeroAction(props: {
  action: "shuffle" | "mix" | "subscribe";
  size: "small" | "large";
  label: string;
  active?: boolean;
  busy?: boolean;
  disabled: boolean;
  children: React.ReactNode;
  onClick: () => void;
}) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          type="button"
          variant="glass"
          size="icon"
          shape="circle"
          className="listen-muse-artist-hero__action"
          aria-label={props.label}
          aria-busy={props.busy || undefined}
          aria-pressed={props.active}
          aria-disabled={props.disabled}
          data-active={props.active ? "true" : "false"}
          data-artist-action={props.action}
          data-disabled={props.disabled ? "true" : "false"}
          data-size={props.size}
          onClick={props.disabled ? undefined : props.onClick}
        >
          {props.children}
        </Button>
      </TooltipTrigger>
      <TooltipContent side="bottom" sideOffset={10}>
        {props.label}
      </TooltipContent>
    </Tooltip>
  );
}

function ListenMuseArtistHeroArtwork(props: {
  httpBaseURL: string;
  title: string;
  thumbnailUrl?: string;
  heroThumbnailUrl?: string;
}) {
  const candidates = React.useMemo(() => {
    const result: string[] = [];
    const seen = new Set<string>();
    for (const rawURL of [props.heroThumbnailUrl, props.thumbnailUrl]) {
      for (const candidate of buildListenImageCandidates(
        props.httpBaseURL,
        rawURL?.trim() ?? "",
      )) {
        if (!seen.has(candidate)) {
          seen.add(candidate);
          result.push(candidate);
        }
      }
    }
    return result;
  }, [props.heroThumbnailUrl, props.httpBaseURL, props.thumbnailUrl]);
  const candidateKey = candidates.join("\n");
  const [candidateIndex, setCandidateIndex] = React.useState(0);
  const [imageUnavailable, setImageUnavailable] = React.useState(false);
  const source = imageUnavailable ? "" : candidates[candidateIndex] ?? "";

  React.useEffect(() => {
    setCandidateIndex(0);
    setImageUnavailable(false);
  }, [candidateKey]);

  return (
    <div className="listen-muse-artist-hero__artwork" aria-label={props.title}>
      <span
        className="listen-muse-artist-hero__artwork-fallback"
        aria-hidden="true"
      >
        <UserRound />
      </span>
      {source ? (
        <img
          src={source}
          alt=""
          className="listen-muse-artist-hero__artwork-image"
          loading="eager"
          onError={() => {
            if (candidateIndex < candidates.length - 1) {
              setCandidateIndex((current) => current + 1);
              return;
            }
            setImageUnavailable(true);
          }}
        />
      ) : null}
    </div>
  );
}
