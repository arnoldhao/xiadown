import { ArrowLeft } from "lucide-react";
import * as React from "react";

import type { Pet } from "@/shared/contracts/pets";
import type { PetAnimation } from "@/shared/pets/animation";
import { Button } from "@/shared/ui/button";
import { GlassSurface } from "@/shared/ui/glass-surface";
import { PetDisplay } from "@/shared/ui/pet-player";

export type ListenPrimaryStatusOverlayProps = {
  label: string;
  pet: Pet | null;
  petImageURL: string;
  kind: "loading" | "error";
  animation?: PetAnimation;
  backLabel?: string;
  onBack?: () => void;
  actionLabel?: string;
  onAction?: () => void;
};

export type ListenPrimaryLoadingOverlayProps = Omit<
  ListenPrimaryStatusOverlayProps,
  "kind" | "actionLabel" | "onAction"
>;

export type ListenPrimaryLoadingBoundaryProps = Omit<
  React.HTMLAttributes<HTMLDivElement>,
  "aria-busy" | "aria-hidden"
> & {
  loading: boolean;
  covered?: boolean;
};

/**
 * Keeps a loading page mounted for visual continuity while removing it from
 * keyboard navigation and the accessibility tree until the update completes.
 */
export function ListenPrimaryLoadingBoundary(
  props: ListenPrimaryLoadingBoundaryProps,
) {
  const { loading, covered = false, ...elementProps } = props;
  const inactive = loading || covered;
  const boundaryRef = React.useRef<HTMLDivElement>(null);
  const previousFocusRef = React.useRef<HTMLElement | null>(null);

  React.useEffect(() => {
    const boundary = boundaryRef.current;
    if (!boundary) {
      return;
    }

    if (inactive) {
      const activeElement = document.activeElement;
      if (
        activeElement instanceof HTMLElement &&
        boundary.contains(activeElement)
      ) {
        previousFocusRef.current = activeElement;
        activeElement.blur();
      }
      return;
    }

    const previousFocus = previousFocusRef.current;
    previousFocusRef.current = null;
    if (
      previousFocus?.isConnected &&
      boundary.contains(previousFocus) &&
      (!document.activeElement || document.activeElement === document.body)
    ) {
      previousFocus.focus({ preventScroll: true });
    }
  }, [inactive]);

  // React 18 treats `inert` as an unknown non-boolean attribute. The empty
  // string is the standards-compliant boolean form and avoids dropping it.
  const inertAttribute = inactive ? { inert: "" } : {};

  return (
    <div
      {...elementProps}
      {...inertAttribute}
      ref={boundaryRef}
      data-listen-primary-loading-boundary={
        loading ? "busy" : covered ? "covered" : "idle"
      }
      aria-busy={loading || undefined}
      aria-hidden={inactive || undefined}
    />
  );
}

/**
 * A content-level loading treatment: navigation and the floating transport stay
 * crisp while the current primary page remains visible, softened, and stable
 * behind a single centered progress indicator.
 */
export function ListenPrimaryStatusOverlay(
  props: ListenPrimaryStatusOverlayProps,
) {
  const error = props.kind === "error";
  const hasAction = Boolean(props.actionLabel && props.onAction);

  return (
    <div
      className="listen-primary-status-scrim"
      data-listen-primary-status={props.kind}
    >
      <GlassSurface
        aria-hidden="true"
        className="listen-primary-status-scrim__surface"
        surfaceRole="status"
        elevation="floating"
        shape="panel"
      />
      {props.backLabel && props.onBack ? (
        <Button
          type="button"
          variant="ghost"
          size="compactIcon"
          className="listen-primary-status-back wails-no-drag"
          data-listen-primary-loading-back="true"
          aria-label={props.backLabel}
          title={props.backLabel}
          onClick={props.onBack}
        >
          <ArrowLeft aria-hidden="true" className="h-4 w-4" />
        </Button>
      ) : null}
      <div
        className="listen-primary-status-content"
        role={error ? "alert" : "status"}
        aria-live={error ? "assertive" : "polite"}
        aria-label={props.label}
      >
        <PetDisplay
          pet={props.pet}
          imageUrl={props.petImageURL}
          animation={props.animation ?? "running"}
          alt=""
          size={56}
          className="listen-primary-status-pet h-14 w-14 shrink-0"
          glowClassName="listen-primary-status-pet-glow"
        />
        <span className="listen-primary-status-message">{props.label}</span>
        {hasAction ? (
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="listen-primary-status-action"
            onClick={props.onAction}
          >
            {props.actionLabel}
          </Button>
        ) : null}
      </div>
    </div>
  );
}

export function ListenPrimaryLoadingOverlay(
  props: ListenPrimaryLoadingOverlayProps,
) {
  return <ListenPrimaryStatusOverlay {...props} kind="loading" />;
}
