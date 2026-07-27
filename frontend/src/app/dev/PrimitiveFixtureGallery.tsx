import {
  CircleHelp,
  Grid2X2,
  List,
  MoreHorizontal,
  SlidersHorizontal,
  Sparkles,
} from "lucide-react";
import * as React from "react";

import { APP_BADGE_VARIANTS, Badge } from "@/shared/ui/badge";
import {
  APP_BUTTON_SHAPES,
  APP_BUTTON_SIZES,
  APP_BUTTON_TONES,
  APP_BUTTON_VARIANTS,
  Button,
} from "@/shared/ui/button";
import {
  APP_CARD_SECTION_SIZES,
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/shared/ui/card";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/shared/ui/dialog";
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
  DropdownMenuTrigger,
} from "@/shared/ui/dropdown-menu";
import { DreamInlineSwitch } from "@/shared/ui/dream-inline-switch";
import { DreamSegmentSwitch } from "@/shared/ui/dream-segment-switch";
import { GlassSurface } from "@/shared/ui/glass-surface";
import { FUN_BUTTON_EFFECTS } from "@/shared/ui/fun-button-effect";
import { APP_INPUT_SIZES, Input } from "@/shared/ui/input";
import {
  PET_DISPLAY_GLOW_VARIANTS,
  PetDisplay,
} from "@/shared/ui/pet-player";
import { Progress } from "@/shared/ui/progress";
import { SecondaryReveal } from "@/shared/ui/secondary-reveal";
import { Select } from "@/shared/ui/select";
import { Separator } from "@/shared/ui/separator";
import {
  Sheet,
  SheetBody,
  SheetClose,
  SheetCloseButton,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetHeading,
  SheetTitle,
  SheetTrigger,
  SHEET_SIDES,
  SHEET_SIZES,
  type SheetSide,
  type SheetSize,
} from "@/shared/ui/sheet";
import { DREAM_STATUS_TONES, StatusBadge } from "@/shared/ui/status-badge";
import {
  TOOLTIP_ALIGNS,
  TOOLTIP_SIDES,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
  type TooltipAlign,
  type TooltipSide,
} from "@/shared/ui/tooltip";
import {
  APP_USER_AVATAR_SHAPES,
  APP_USER_AVATAR_TONES,
  UserAvatar,
} from "@/shared/ui/user-avatar";

import {
  APPEARANCE_PRIMITIVE_FIXTURE_REGISTRY,
  type AppearancePrimitiveFixtureId,
} from "./appearance-fixture-registry";

const FIXTURE_AVATAR_PNG_BASE64 =
  "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=";

function Axis({ children, label }: React.PropsWithChildren<{ label: string }>) {
  return (
    <div className="appearance-lab__fixture-axis">
      <strong>{label}</strong>
      <div>{children}</div>
    </div>
  );
}

function ButtonsFixture() {
  return (
    <div className="appearance-lab__fixture-stack">
      <Axis label="Variant">
        {APP_BUTTON_VARIANTS.map((variant) => (
          <Button key={variant} variant={variant}>{variant}</Button>
        ))}
      </Axis>
      <Axis label="Tone">
        {APP_BUTTON_TONES.map((tone) => (
          <Button key={tone} tone={tone} variant="outline">{tone}</Button>
        ))}
      </Axis>
      <Axis label="Size">
        {APP_BUTTON_SIZES.map((size) => (
          <Button
            aria-label={size.includes("Icon") || size === "icon" ? `${size} button` : undefined}
            key={size}
            size={size}
            variant="secondary"
          >
            {size.includes("Icon") || size === "icon" ? <Sparkles /> : size}
          </Button>
        ))}
      </Axis>
      <Axis label="Shape and state">
        {APP_BUTTON_SHAPES.map((shape) => (
          <Button aria-label={`${shape} button`} key={shape} shape={shape} variant="outline">
            {shape === "circle" || shape === "square" ? <Sparkles /> : shape}
          </Button>
        ))}
        <Button disabled>disabled</Button>
      </Axis>
      <Axis label="Decorative effect">
        {FUN_BUTTON_EFFECTS.map((effect) => (
          <Button
            className="app-running-new-download-button"
            data-effect={effect}
            data-fixture-button-effect={effect}
            key={effect}
          >
            {effect}
          </Button>
        ))}
      </Axis>
    </div>
  );
}

function FormsFixture() {
  return (
    <div className="appearance-lab__fixture-stack">
      <Axis label="Input sizes">
        {APP_INPUT_SIZES.map((size) => (
          <Input
            aria-label={`${size} input`}
            data-fixture-input-size={size}
            key={size}
            readOnly
            size={size}
            value={size}
          />
        ))}
      </Axis>
      <div className="appearance-lab__fixture-field-grid">
        <label><span>Placeholder</span><Input placeholder="Media URL" /></label>
        <label><span>Read only</span><Input value="Immutable value" readOnly /></label>
        <label><span>Invalid</span><Input aria-invalid value="Invalid value" readOnly /></label>
        <label><span>Disabled</span><Input disabled value="Unavailable" readOnly /></label>
        <label>
          <span>Select</span>
          <Select defaultValue="balanced"><option value="best">Best</option><option value="balanced">Balanced</option></Select>
        </label>
        <label><span>Disabled select</span><Select disabled><option>Unavailable</option></Select></label>
      </div>
      <Axis label="Progress and separators">
        <div className="appearance-lab__fixture-progress"><Progress aria-label="64 percent" value={64} /></div>
        <Separator />
        <span className="appearance-lab__fixture-vertical-separator"><Separator orientation="vertical" /></span>
      </Axis>
    </div>
  );
}

function ContentFixture() {
  return (
    <div className="appearance-lab__fixture-content-grid">
      {APP_CARD_SECTION_SIZES.map((size) => (
        <Card data-fixture-card-section-size={size} key={size}>
          <CardHeader size={size}>
            <CardTitle>{size} card sections</CardTitle>
            <CardDescription>Canonical shared anatomy.</CardDescription>
          </CardHeader>
          <CardContent size={size}>Content remains semantic and theme-aware.</CardContent>
          <CardFooter size={size}><Button size="compact">Action</Button></CardFooter>
        </Card>
      ))}
      <Axis label="Badge variants">
        {APP_BADGE_VARIANTS.map((variant) => (
          <Badge data-fixture-badge-variant={variant} key={variant} variant={variant}>
            {variant}
          </Badge>
        ))}
      </Axis>
      <Axis label="Status tones">
        {DREAM_STATUS_TONES.map((tone) => (
          <StatusBadge key={tone} marker tone={tone}>{tone}</StatusBadge>
        ))}
        <StatusBadge aria-label="Icon-only status" icon={<Sparkles />} iconOnly tone="success" />
      </Axis>
      <Axis label="Avatar tone and shape">
        {APP_USER_AVATAR_TONES.flatMap((tone) =>
          APP_USER_AVATAR_SHAPES.map((shape) => (
            <UserAvatar
              className="appearance-lab__fixture-avatar"
              data-fixture-user-avatar={`${tone}-${shape}`}
              key={`${tone}-${shape}`}
              profile={{
                displayName: `${tone} ${shape}`,
                username: `${tone}-${shape}`,
              }}
              shape={shape}
              tone={tone}
            />
          )),
        )}
        <UserAvatar
          className="appearance-lab__fixture-avatar"
          data-fixture-user-avatar-image="theme-wash"
          profile={{
            avatarBase64: FIXTURE_AVATAR_PNG_BASE64,
            avatarMime: "image/png",
            displayName: "Image avatar",
            username: "image-avatar",
          }}
          shape="circle"
          tone="theme"
        />
      </Axis>
      <Axis label="Pet display glow">
        {PET_DISPLAY_GLOW_VARIANTS.map((variant) => (
          <div data-fixture-pet-glow={variant} key={variant}>
            <PetDisplay
              alt={`${variant} pet glow`}
              fallbackSrc="/appicon.png"
              glowVariant={variant}
              imageUrl=""
              load={false}
              pet={null}
            />
          </div>
        ))}
      </Axis>
      <Axis label="Glass interaction and focus ring">
        <GlassSurface
          data-fixture-glass-interactive="true"
          interactive
          shape="control"
          surfaceRole="control"
          tabIndex={0}
        >
          Hover or focus
        </GlassSurface>
        <GlassSurface
          data-fixture-glass-focus-ring="true"
          focusRing
          shape="control"
          surfaceRole="control"
        >
          <Button size="compact" variant="ghost">Focus within</Button>
        </GlassSurface>
      </Axis>
    </div>
  );
}

function TogglesFixture() {
  const [checked, setChecked] = React.useState(false);
  const [view, setView] = React.useState<"grid" | "list">("grid");
  const [mode, setMode] = React.useState<"music" | "video" | "data">("music");
  return (
    <div className="appearance-lab__fixture-stack">
      <Axis label="Inline switch">
        <DreamInlineSwitch ariaLabel="Interactive switch" checked={checked} onCheckedChange={setChecked} />
        <DreamInlineSwitch ariaLabel="Checked switch" checked onCheckedChange={() => undefined} />
        <DreamInlineSwitch ariaLabel="Disabled switch" checked={false} disabled onCheckedChange={() => undefined} />
      </Axis>
      <Axis label="Two segments">
        <DreamSegmentSwitch
          ariaLabel="View"
          items={[{ value: "grid", label: "Grid", icon: <Grid2X2 /> }, { value: "list", label: "List", icon: <List /> }]}
          onValueChange={setView}
          tooltips={false}
          value={view}
        />
      </Axis>
      <Axis label="Three segments and disabled state">
        <DreamSegmentSwitch
          ariaLabel="Mode"
          items={[
            { value: "music", label: "Music" },
            { value: "video", label: "Video" },
            { value: "data", label: "Data", disabled: true },
          ]}
          onValueChange={setMode}
          tooltips={false}
          value={mode}
        />
      </Axis>
      <Axis label="Compact segments">
        <DreamSegmentSwitch
          ariaLabel="Compact view"
          compact
          items={[{ value: "grid", label: "Grid", icon: <Grid2X2 /> }, { value: "list", label: "List", icon: <List /> }]}
          onValueChange={setView}
          tooltips={false}
          value={view}
        />
      </Axis>
    </div>
  );
}

function MenusFixture() {
  const [downloadsVisible, setDownloadsVisible] = React.useState(true);
  const [notificationsVisible, setNotificationsVisible] = React.useState(false);
  const [density, setDensity] = React.useState("comfortable");

  return (
    <div className="appearance-lab__fixture-stack">
      <Axis label="Open the production menu to inspect every item branch">
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              data-fixture-dropdown-menu="true"
              variant="outline"
            >
              <MoreHorizontal />
              Open state menu
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="start">
            <DropdownMenuLabel>Visibility</DropdownMenuLabel>
            <DropdownMenuCheckboxItem
              checked={downloadsVisible}
              data-fixture-menu-branch="checkbox-checked"
              onCheckedChange={(value) => setDownloadsVisible(value === true)}
            >
              Downloads
            </DropdownMenuCheckboxItem>
            <DropdownMenuCheckboxItem
              checked={notificationsVisible}
              data-fixture-menu-branch="checkbox-unchecked"
              onCheckedChange={(value) => setNotificationsVisible(value === true)}
            >
              Notifications
            </DropdownMenuCheckboxItem>
            <DropdownMenuSeparator />
            <DropdownMenuLabel>Density</DropdownMenuLabel>
            <DropdownMenuRadioGroup onValueChange={setDensity} value={density}>
              <DropdownMenuRadioItem
                data-fixture-menu-branch="radio-selected"
                value="comfortable"
              >
                Comfortable
              </DropdownMenuRadioItem>
              <DropdownMenuRadioItem
                data-fixture-menu-branch="radio-unselected"
                value="compact"
              >
                Compact
              </DropdownMenuRadioItem>
            </DropdownMenuRadioGroup>
            <DropdownMenuSeparator />
            <DropdownMenuItem data-fixture-menu-branch="shortcut">
              Command palette
              <DropdownMenuShortcut>⌘ K</DropdownMenuShortcut>
            </DropdownMenuItem>
            <DropdownMenuItem data-fixture-menu-branch="disabled" disabled>
              Unavailable action
            </DropdownMenuItem>
            <DropdownMenuItem
              data-fixture-menu-branch="destructive"
              tone="destructive"
            >
              Remove item
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </Axis>
    </div>
  );
}

function DialogSpecimen() {
  return (
    <Dialog>
      <DialogTrigger asChild><Button variant="outline">Open dialog</Button></DialogTrigger>
      <DialogContent>
        <DialogHeader><DialogTitle>Real Dream dialog</DialogTitle><DialogDescription>Portal, overlay, focus trap and motion use the shared primitive.</DialogDescription></DialogHeader>
        <Input aria-label="Dialog value" defaultValue="https://example.com/media" />
        <DialogFooter><DialogClose asChild><Button variant="ghost">Cancel</Button></DialogClose><Button>Confirm</Button></DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function SheetSpecimen({
  centered = false,
  side = "right",
  size,
  windowChromeSafeArea = false,
}: {
  centered?: boolean;
  side?: SheetSide;
  size: SheetSize;
  windowChromeSafeArea?: boolean;
}) {
  const placement = centered ? `${size} centered sheet` : `${side} ${size} sheet`;
  const label = windowChromeSafeArea
    ? `${placement} · window chrome safe`
    : placement;
  return (
    <Sheet>
      <SheetTrigger asChild>
        <Button
          data-fixture-sheet-centered={centered ? "true" : "false"}
          data-fixture-sheet-side={centered ? undefined : side}
          data-fixture-sheet-size={size}
          data-fixture-sheet-window-chrome-safe-area={
            windowChromeSafeArea ? "true" : undefined
          }
          variant="outline"
        >
          {label}
        </Button>
      </SheetTrigger>
      <SheetContent
        centered={centered}
        side={side}
        size={size}
        windowChromeSafeArea={windowChromeSafeArea}
      >
        <SheetHeader><SheetHeading><SheetTitle>{label}</SheetTitle><SheetDescription>Real modal slide-over presentation.</SheetDescription></SheetHeading><SheetCloseButton aria-label="Close sheet" /></SheetHeader>
        <SheetBody><Card><CardContent size="compact">Sheet content uses canonical surfaces.</CardContent></Card></SheetBody>
        <SheetFooter><SheetClose asChild><Button>Done</Button></SheetClose></SheetFooter>
      </SheetContent>
    </Sheet>
  );
}

function TooltipSpecimen({
  align,
  multiline = false,
  side,
}: {
  align: TooltipAlign;
  multiline?: boolean;
  side: TooltipSide;
}) {
  const label = multiline ? "multiline" : `${side} ${align}`;
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          aria-label={`${label} tooltip sample`}
          data-fixture-tooltip={label.replace(" ", "-")}
          size="compact"
          variant="outline"
        >
          {label}
        </Button>
      </TooltipTrigger>
      <TooltipContent align={align} multiline={multiline} side={side}>
        {multiline
          ? "A deliberately longer tooltip that verifies wrapped explanatory copy."
          : `Dream tooltip · ${side} · ${align}`}
      </TooltipContent>
    </Tooltip>
  );
}

function OverlaysFixture() {
  return (
    <div className="appearance-lab__fixture-stack">
      <Axis label="Open or hover each real overlay">
        <Tooltip><TooltipTrigger asChild><Button aria-label="Tooltip sample" shape="square" variant="outline"><CircleHelp /></Button></TooltipTrigger><TooltipContent>Dream tooltip</TooltipContent></Tooltip>
        <DialogSpecimen />
        <SecondaryReveal
          ariaLabel="Secondary reveal sample"
          content={<div className="appearance-lab__fixture-reveal-content"><strong>Secondary reveal</strong><span>Hover, focus, click pin and Escape all use the shared behavior.</span></div>}
        >
          {({ anchorProps, triggerProps }) => (
            <div {...anchorProps} className="appearance-lab__fixture-reveal-anchor">
              <Button {...triggerProps} variant="outline"><SlidersHorizontal />Reveal</Button>
            </div>
          )}
        </SecondaryReveal>
        <Button aria-label="More overlay samples" shape="square" variant="ghost"><MoreHorizontal /></Button>
      </Axis>
      <Axis label="Tooltip side and alignment">
        {TOOLTIP_SIDES.flatMap((side) =>
          TOOLTIP_ALIGNS.map((align) => (
            <TooltipSpecimen align={align} key={`${side}-${align}`} side={side} />
          )),
        )}
        <TooltipSpecimen align="center" multiline side="top" />
      </Axis>
      <Axis label="Sheet sides">
        {SHEET_SIDES.map((side) => <SheetSpecimen key={side} side={side} size="sm" />)}
      </Axis>
      <Axis label="Sheet sizes and centered presentation">
        {SHEET_SIZES.map((size) => <SheetSpecimen key={size} size={size} />)}
        <SheetSpecimen centered size="md" />
        <SheetSpecimen centered size="lg" windowChromeSafeArea />
      </Axis>
    </div>
  );
}

export const APPEARANCE_FIXTURE_RENDERERS = {
  buttons: ButtonsFixture,
  forms: FormsFixture,
  content: ContentFixture,
  toggles: TogglesFixture,
  menus: MenusFixture,
  overlays: OverlaysFixture,
} satisfies Record<AppearancePrimitiveFixtureId, React.ComponentType>;

export function PrimitiveFixtureGallery() {
  return (
    <section aria-labelledby="appearance-lab-primitives-title" className="appearance-lab__section" data-appearance-fixture="primitive-gallery">
      <div className="appearance-lab__section-heading">
        <span>08</span>
        <div><h2 id="appearance-lab-primitives-title">Primitive contract gallery</h2><p>Fixtures are generated from a machine-readable registry and render production primitives.</p></div>
      </div>
      <div className="appearance-lab__fixture-gallery">
        {APPEARANCE_PRIMITIVE_FIXTURE_REGISTRY.map((fixture) => {
          const Fixture = APPEARANCE_FIXTURE_RENDERERS[fixture.id];
          return (
            <GlassSurface
              asChild
              elevation="embedded"
              key={fixture.id}
              shape="card"
              surfaceRole="card"
            >
              <article className="appearance-lab__fixture-card" data-appearance-fixture={fixture.id}>
                <header><div><h3>{fixture.title}</h3><p>{fixture.description}</p></div><code>{fixture.contracts.length} contracts</code></header>
                <Fixture />
              </article>
            </GlassSurface>
          );
        })}
      </div>
    </section>
  );
}
