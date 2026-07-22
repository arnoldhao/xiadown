import {
  forwardRef,
  type ButtonHTMLAttributes,
  type HTMLAttributes,
  type ReactNode,
} from "react";

import { cn } from "@/lib/utils";

export type AccountDockProps = HTMLAttributes<HTMLElement>;

export interface AccountDockProfileProps
  extends Omit<ButtonHTMLAttributes<HTMLButtonElement>, "name"> {
  avatar: ReactNode;
  name: ReactNode;
  username: ReactNode;
  statusIndicator?: ReactNode;
  disclosure: ReactNode;
}

/**
 * A single account/workspace menu trigger for wide sidebars. The disclosure is
 * deliberately part of the same button as the avatar and identity so the
 * whole visual row has one predictable target.
 */
export const AccountDockProfile = forwardRef<
  HTMLButtonElement,
  AccountDockProfileProps
>(function AccountDockProfile(
  {
    avatar,
    name,
    username,
    statusIndicator,
    disclosure,
    className,
    type = "button",
    ...props
  },
  ref,
) {
  return (
    <button
      ref={ref}
      type={type}
      {...props}
      className={cn(
        "app-workspace-account-profile wails-no-drag",
        className,
      )}
    >
      <span className="app-workspace-account-profile__avatar">
        <span className="app-workspace-account-profile__avatar-media">
          {avatar}
        </span>
      </span>
      <span className="app-workspace-account-profile__identity">
        <span className="app-workspace-account-profile__name">{name}</span>
        <span className="app-workspace-account-profile__username">
          {username}
        </span>
      </span>
      <span
        aria-hidden="true"
        className="app-workspace-account-profile__disclosure"
      >
        {statusIndicator}
        {disclosure}
      </span>
    </button>
  );
});

export function AccountDock({
  children,
  className,
  "aria-label": ariaLabel = "Account",
  ...props
}: AccountDockProps) {
  if (children == null) {
    return null;
  }

  return (
    <section
      {...props}
      aria-label={ariaLabel}
      className={cn("app-workspace-account-dock", className)}
    >
      {children}
    </section>
  );
}
