import type { BrowserSourceProfile } from "@/shared/contracts/browserSources";

const PROFILE_CHANNEL_SEPARATOR = " · ";

function splitProfileChannel(label: string) {
  const separatorIndex = label.lastIndexOf(PROFILE_CHANNEL_SEPARATOR);
  if (separatorIndex < 0) {
    return { base: label, channel: "" };
  }
  return {
    base: label.slice(0, separatorIndex).trim(),
    channel: label.slice(separatorIndex + PROFILE_CHANNEL_SEPARATOR.length).trim(),
  };
}

function isBrowserGeneratedDefaultLabel(
  profile: BrowserSourceProfile,
  browserId: string,
  label: string,
) {
  if (!profile.isDefault) {
    return false;
  }
  const normalizedLabel = label.trim().toLocaleLowerCase("en-US");
  if (normalizedLabel === "default") {
    return true;
  }
  return browserId.trim().toLocaleLowerCase("en-US") === "chrome" &&
    normalizedLabel === "your chrome";
}

/**
 * Localizes browser-generated profile aliases without translating names the
 * user chose in the browser. Chromium persists aliases such as `Default` and
 * `Your Chrome` in Local State even when XiaDown uses another display locale.
 */
export function browserProfileDisplayLabel(
  profile: BrowserSourceProfile,
  browserId: string,
  defaultLabel: string,
  otherProfilesLabel: string,
) {
  const rawLabel = profile.label?.trim() ?? "";
  if (!profile.available && rawLabel === "Other Profiles") {
    return otherProfilesLabel;
  }
  if (!rawLabel) {
    return profile.isDefault ? defaultLabel : profile.id;
  }

  const { base, channel } = splitProfileChannel(rawLabel);
  if (!isBrowserGeneratedDefaultLabel(profile, browserId, base)) {
    return rawLabel;
  }
  return channel
    ? `${defaultLabel}${PROFILE_CHANNEL_SEPARATOR}${channel}`
    : defaultLabel;
}
