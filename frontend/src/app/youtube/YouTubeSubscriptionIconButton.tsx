import { LoaderCircle, UserCheck, UserPlus } from "lucide-react";

import { cn } from "@/lib/utils";
import { Button } from "@/shared/ui/button";

export function YouTubeSubscriptionIconButton(props: {
  subscribed: boolean;
  busy?: boolean;
  label: string;
  className?: string;
  disabled?: boolean;
  onClick?: () => void;
}) {
  const busy = props.busy === true;

  return (
    <Button
      type="button"
      variant="ghost"
      tone={props.subscribed ? "accent" : "neutral"}
      size="compactIcon"
      shape="circle"
      className={cn(
        "youtube-subscription-icon-button wails-no-drag",
        props.className,
      )}
      aria-label={props.label}
      title={props.label}
      aria-pressed={props.subscribed}
      aria-busy={busy || undefined}
      disabled={props.disabled || busy}
      onClick={props.onClick}
    >
      {busy ? (
        <LoaderCircle className="app-motion-spin" aria-hidden="true" />
      ) : props.subscribed ? (
        <UserCheck aria-hidden="true" />
      ) : (
        <UserPlus aria-hidden="true" />
      )}
    </Button>
  );
}
