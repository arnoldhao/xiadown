import * as React from "react";

import { cn } from "@/lib/utils";
import type { BrowserSourceProfile } from "@/shared/contracts/browserSources";

const CUSTOM_PROFILE_TONES = [
  "accent",
  "busy",
  "success",
  "warning",
] as const;

function profileHash(value: string) {
  let result = 0;
  for (let index = 0; index < value.length; index += 1) {
    result = ((result << 5) - result + value.charCodeAt(index)) | 0;
  }
  return Math.abs(result);
}

export function managedProfileInitials(profile: BrowserSourceProfile) {
  const source = profile.label?.trim() || "XiaDown";
  const segments = source.split(/\s+/).filter(Boolean);
  if (segments.length > 1) {
    return segments
      .slice(0, 2)
      .map((segment) => segment[0] ?? "")
      .join("")
      .toUpperCase();
  }
  return source.slice(0, 2).toUpperCase();
}

export function managedProfileDisplayLabel(
  profile: BrowserSourceProfile,
  profiles: BrowserSourceProfile[],
  defaultLabel: string,
) {
  const rawLabel = profile.label?.trim() || profile.id;
  if (profile.isDefault || profile.virtual) {
    return `${rawLabel} · ${defaultLabel}`;
  }
  const peers = profiles
    .filter(
      (candidate) =>
        !candidate.isDefault &&
        !candidate.virtual &&
        (candidate.label?.trim() || candidate.id).localeCompare(rawLabel, undefined, {
          sensitivity: "accent",
        }) === 0,
    )
    .sort((left, right) => left.id.localeCompare(right.id));
  if (peers.length < 2) {
    return rawLabel;
  }
  const ordinal = peers.findIndex((candidate) => candidate.id === profile.id) + 1;
  return ordinal > 0 ? `${rawLabel} (${ordinal})` : rawLabel;
}

export function ManagedProfileAvatar(props: {
  profile: BrowserSourceProfile;
  badge?: React.ReactNode;
  className?: string;
}) {
  const isDefault = props.profile.isDefault || props.profile.virtual;
  const tone = CUSTOM_PROFILE_TONES[
    profileHash(props.profile.id || props.profile.browserId || "xiadown") %
      CUSTOM_PROFILE_TONES.length
  ];
  return (
    <span
      aria-hidden="true"
      className={cn(
        "app-managed-profile-avatar relative flex h-10 w-10 shrink-0 items-center justify-center overflow-visible",
        props.className,
      )}
      data-tone={isDefault ? "default" : tone}
    >
      {isDefault ? (
        <img
          alt=""
          className="app-managed-profile-avatar__image h-8 w-8 object-cover"
          src="/appicon.png"
        />
      ) : (
        <span className="app-managed-profile-initials">
          {managedProfileInitials(props.profile)}
        </span>
      )}
      {props.badge ? (
        <span className="app-managed-profile-avatar__badge absolute -bottom-1 -right-1 flex h-5 w-5 items-center justify-center">
          {props.badge}
        </span>
      ) : null}
    </span>
  );
}
