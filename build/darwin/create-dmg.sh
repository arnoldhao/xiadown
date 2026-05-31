#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -lt 2 ] || [ "$#" -gt 3 ]; then
  echo "Usage: $0 <XiaDown.app> <output.dmg> [volume-name]" >&2
  exit 64
fi

APP_BUNDLE="$1"
OUTPUT_DMG="$2"
VOL_NAME="${3:-XiaDown}"

if [ "$(uname -s)" != "Darwin" ]; then
  echo "DMG creation requires macOS because it uses hdiutil and Finder." >&2
  exit 69
fi

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BACKGROUND_IMAGE="$ROOT_DIR/build/darwin/dmg-background.png"
VOLUME_ICON="$ROOT_DIR/build/darwin/dmg-volume.icns"

if [ ! -d "$APP_BUNDLE" ]; then
  echo "App bundle not found: $APP_BUNDLE" >&2
  exit 66
fi

if [ ! -f "$BACKGROUND_IMAGE" ]; then
  echo "DMG background not found: $BACKGROUND_IMAGE" >&2
  exit 66
fi

if [ ! -f "$VOLUME_ICON" ]; then
  echo "DMG volume icon not found: $VOLUME_ICON" >&2
  exit 66
fi

APP_BUNDLE="$(cd "$(dirname "$APP_BUNDLE")" && pwd)/$(basename "$APP_BUNDLE")"
mkdir -p "$(dirname "$OUTPUT_DMG")"
OUTPUT_DMG="$(cd "$(dirname "$OUTPUT_DMG")" && pwd)/$(basename "$OUTPUT_DMG")"

WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/xiadown-dmg.XXXXXX")"
RW_DMG="$WORK_DIR/xiadown-rw.dmg"
ATTACH_LOG="$WORK_DIR/attach.log"
DEVICE=""

cleanup() {
  if [ -n "$DEVICE" ]; then
    hdiutil detach "$DEVICE" >/dev/null 2>&1 || hdiutil detach -force "$DEVICE" >/dev/null 2>&1 || true
  fi
  rm -rf "$WORK_DIR"
}
trap cleanup EXIT

APP_SIZE_MB="$(du -sm "$APP_BUNDLE" | awk '{print $1}')"
IMAGE_SIZE_MB="$((APP_SIZE_MB + 96))"

rm -f "$OUTPUT_DMG"
hdiutil create \
  -volname "$VOL_NAME" \
  -size "${IMAGE_SIZE_MB}m" \
  -fs HFS+ \
  -type UDIF \
  -ov \
  "$RW_DMG" >/dev/null

hdiutil attach \
  -readwrite \
  -noverify \
  -noautoopen \
  "$RW_DMG" >"$ATTACH_LOG"

DEVICE="$(awk '/Apple_HFS/ {print $1; exit}' "$ATTACH_LOG")"
MOUNT_DIR="$(awk '/Apple_HFS/ {for (i = 1; i <= NF; i++) if ($i ~ /^\/Volumes\//) {print substr($0, index($0, $i)); exit}}' "$ATTACH_LOG")"

if [ -z "$DEVICE" ] || [ -z "$MOUNT_DIR" ] || [ ! -d "$MOUNT_DIR" ]; then
  echo "Unable to attach writable DMG." >&2
  cat "$ATTACH_LOG" >&2
  exit 70
fi

APP_NAME="$(basename "$APP_BUNDLE")"
ditto "$APP_BUNDLE" "$MOUNT_DIR/$APP_NAME"
ln -s /Applications "$MOUNT_DIR/Applications"
mkdir -p "$MOUNT_DIR/.background"
cp "$BACKGROUND_IMAGE" "$MOUNT_DIR/.background/background.png"

SetFile -a V "$MOUNT_DIR/.background"

osascript <<APPLESCRIPT
tell application "Finder"
  tell disk "$VOL_NAME"
    open
    set current view of container window to icon view
    set toolbar visible of container window to false
    set statusbar visible of container window to false
    set the bounds of container window to {120, 120, 800, 540}
    set viewOptions to icon view options of container window
    set arrangement of viewOptions to not arranged
    set icon size of viewOptions to 112
    set background picture of viewOptions to file ".background:background.png"
    set position of item "$APP_NAME" of container window to {156, 213}
    set position of item "Applications" of container window to {525, 220}
    update without registering applications
    delay 1
    close
  end tell
end tell
APPLESCRIPT

cp "$VOLUME_ICON" "$MOUNT_DIR/.VolumeIcon.icns"
SetFile -a C "$MOUNT_DIR"
SetFile -a V "$MOUNT_DIR/.VolumeIcon.icns"

sync
for attempt in 1 2 3 4 5; do
  if hdiutil detach "$DEVICE" >/dev/null 2>&1; then
    DEVICE=""
    break
  fi
  sleep "$attempt"
done

if [ -n "$DEVICE" ]; then
  hdiutil detach -force "$DEVICE" >/dev/null
  DEVICE=""
fi

hdiutil convert \
  "$RW_DMG" \
  -format UDZO \
  -imagekey zlib-level=9 \
  -ov \
  -o "$OUTPUT_DMG" >/dev/null
