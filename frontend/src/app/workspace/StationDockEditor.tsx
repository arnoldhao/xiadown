import {
  ArrowDown,
  ArrowUp,
  LayoutGrid,
  Music2,
  Radar,
  Rss,
  Youtube,
} from "lucide-react";
import {
  type FormEvent,
  type ReactNode,
} from "react";

import {
  countVisibleStationDockItems,
  moveStationDockEditorItem,
  setStationDockEditorItemVisible,
  type StationDockEditorValue,
} from "@/app/workspace/station-dock-editor";
import { resolveAppStationCatalog } from "@/app/workspace/station-navigation";
import {
  APP_STATION_LIMIT,
  type AppStation,
} from "@/app/workspace/types";
import { cn } from "@/lib/utils";
import { Button } from "@/shared/ui/button";
import { DreamInlineSwitch } from "@/shared/ui/dream-inline-switch";
import {
  Sheet,
  SheetBody,
  SheetCloseButton,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetHeading,
  SheetTitle,
} from "@/shared/ui/sheet";

import "./station-dock-editor.css";

export interface StationDockEditorLabels {
  title: string;
  description: string;
  close: string;
  visible: string;
  moveUp: string;
  moveDown: string;
  actions: {
    cancel: string;
    save: string;
  };
}

export interface StationDockEditorProps {
  open: boolean;
  catalog: readonly AppStation[];
  value: StationDockEditorValue;
  labels: StationDockEditorLabels;
  disabled?: boolean;
  submitting?: boolean;
  className?: string;
  portalContainer?: HTMLElement | null;
  resolveStationLabel?: (station: AppStation) => string;
  renderStationIcon?: (station: AppStation) => ReactNode;
  onChange: (value: StationDockEditorValue) => void;
  onOpenChange: (open: boolean) => void;
  onSubmit: (value: StationDockEditorValue) => void;
}

/**
 * Window-modal Dock editor. This is intentionally separate from the legacy
 * single-station slide-over: station metadata is read-only here and the form
 * controls only Dock membership and order.
 */
export function StationDockEditor({
  open,
  catalog,
  value,
  labels,
  disabled = false,
  submitting = false,
  className,
  portalContainer,
  resolveStationLabel,
  renderStationIcon,
  onChange,
  onOpenChange,
  onSubmit,
}: StationDockEditorProps) {
  return (
    <Sheet onOpenChange={onOpenChange} open={open}>
      <SheetContent
        centered
        className={cn("app-station-dock-editor__sheet", className)}
        portalContainer={portalContainer}
        size="md"
      >
        <SheetHeader>
          <SheetHeading>
            <SheetTitle>{labels.title}</SheetTitle>
            <SheetDescription>{labels.description}</SheetDescription>
          </SheetHeading>
          <SheetCloseButton
            aria-label={labels.close}
            disabled={disabled || submitting}
          />
        </SheetHeader>
        <StationDockEditorForm
          catalog={catalog}
          disabled={disabled}
          labels={labels}
          onCancel={() => onOpenChange(false)}
          onChange={onChange}
          onSubmit={onSubmit}
          renderStationIcon={renderStationIcon}
          resolveStationLabel={resolveStationLabel}
          submitting={submitting}
          value={value}
        />
      </SheetContent>
    </Sheet>
  );
}

export interface StationDockEditorFormProps
  extends Omit<StationDockEditorProps, "open" | "onOpenChange" | "portalContainer"> {
  onCancel: () => void;
}

export function StationDockEditorForm({
  catalog,
  value,
  labels,
  disabled = false,
  submitting = false,
  className,
  resolveStationLabel = (station) => station.label,
  renderStationIcon = renderDefaultStationIcon,
  onChange,
  onCancel,
  onSubmit,
}: StationDockEditorFormProps) {
  const controlsDisabled = disabled || submitting;
  const resolvedCatalog = resolveAppStationCatalog(catalog).filter(
    (station) => station.enabled,
  );
  const stationById = new Map(
    resolvedCatalog.map((station) => [station.id, station]),
  );
  const valueIds = new Set(value.items.map((item) => item.stationId));
  const orderedItems = [
    ...value.items,
    ...resolvedCatalog
      .filter((station) => !valueIds.has(station.id))
      .map((station) => ({
        stationId: station.id,
        visible: station.pinned !== false,
      })),
  ].filter((item) => stationById.has(item.stationId));
  const visibleItems = orderedItems.filter((item) => item.visible);
  const visibleCount = countVisibleStationDockItems({ items: orderedItems });

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!controlsDisabled) {
      onSubmit(value);
    }
  };

  return (
    <form
      aria-label={labels.title}
      className={cn("app-station-dock-editor__form", className)}
      onSubmit={handleSubmit}
    >
      <SheetBody className="app-station-dock-editor__list">
        {orderedItems.map((item) => {
          const station = stationById.get(item.stationId);
          if (!station) {
            return null;
          }
          const stationLabel = resolveStationLabel(station);
          const visibleIndex = visibleItems.findIndex(
            (candidate) => candidate.stationId === item.stationId,
          );
          const cannotShow = !item.visible && visibleCount >= APP_STATION_LIMIT;
          return (
            <div
              className="app-station-dock-editor__item"
              data-visible={item.visible ? "true" : "false"}
              key={station.id}
            >
              <span
                aria-hidden="true"
                className="app-station-dock-editor__station-icon"
              >
                {renderStationIcon(station)}
              </span>
              <span className="app-station-dock-editor__station-name">
                {stationLabel}
              </span>
              <DreamInlineSwitch
                ariaLabel={`${labels.visible}: ${stationLabel}`}
                checked={item.visible}
                className="app-station-dock-editor__visibility"
                disabled={controlsDisabled || cannotShow}
                onCheckedChange={(checked) =>
                  onChange(
                    setStationDockEditorItemVisible(
                      { items: orderedItems },
                      item.stationId,
                      checked,
                    ),
                  )
                }
              />
              <div className="app-station-dock-editor__order-actions">
                <Button
                  aria-label={`${labels.moveUp}: ${stationLabel}`}
                  className="app-station-dock-editor__order-action"
                  disabled={
                    controlsDisabled || !item.visible || visibleIndex <= 0
                  }
                  onClick={() =>
                    onChange(
                      moveStationDockEditorItem(
                        { items: orderedItems },
                        item.stationId,
                        -1,
                      ),
                    )
                  }
                  shape="square"
                  size="compactIcon"
                  title={labels.moveUp}
                  type="button"
                  variant="ghost"
                >
                  <ArrowUp aria-hidden="true" />
                </Button>
                <Button
                  aria-label={`${labels.moveDown}: ${stationLabel}`}
                  className="app-station-dock-editor__order-action"
                  disabled={
                    controlsDisabled ||
                    !item.visible ||
                    visibleIndex < 0 ||
                    visibleIndex >= visibleItems.length - 1
                  }
                  onClick={() =>
                    onChange(
                      moveStationDockEditorItem(
                        { items: orderedItems },
                        item.stationId,
                        1,
                      ),
                    )
                  }
                  shape="square"
                  size="compactIcon"
                  title={labels.moveDown}
                  type="button"
                  variant="ghost"
                >
                  <ArrowDown aria-hidden="true" />
                </Button>
              </div>
            </div>
          );
        })}
      </SheetBody>
      <SheetFooter>
        <Button
          disabled={controlsDisabled}
          onClick={onCancel}
          type="button"
          variant="outline"
        >
          {labels.actions.cancel}
        </Button>
        <Button disabled={controlsDisabled} type="submit">
          {labels.actions.save}
        </Button>
      </SheetFooter>
    </form>
  );
}

function renderDefaultStationIcon(station: AppStation) {
  switch (station.iconKey) {
    case "music":
      return <Music2 />;
    case "sniff":
      return <Radar />;
    case "youtube":
      return <Youtube />;
    case "rss":
      return <Rss />;
    default:
      return <LayoutGrid />;
  }
}
