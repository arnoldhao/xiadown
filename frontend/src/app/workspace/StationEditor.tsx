import {
  useId,
  type FormEvent,
  type FormHTMLAttributes,
  type ReactNode,
} from "react";

import type { AppStation } from "@/app/workspace/types";
import { cn } from "@/lib/utils";
import { Button } from "@/shared/ui/button";
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
import { Input } from "@/shared/ui/input";
import { Select } from "@/shared/ui/select";

import "./workspace-navigation.css";

export interface StationEditorValue {
  name: string;
  icon: string;
  order: number;
  pinned: boolean;
  defaultRoute: string;
}

export interface StationEditorOption {
  value: string;
  label: string;
  disabled?: boolean;
}

export function stationToEditorValue(
  station: AppStation,
  fallbackDefaultRoute = "",
): StationEditorValue {
  return {
    name: station.label,
    icon: station.iconKey ?? "",
    order: station.order,
    pinned: station.pinned !== false,
    defaultRoute: station.defaultRouteId ?? fallbackDefaultRoute,
  };
}

export function applyStationEditorValue(
  station: AppStation,
  value: StationEditorValue,
): AppStation {
  const order = Number.isFinite(value.order)
    ? Math.max(0, Math.trunc(value.order))
    : station.order;
  return {
    ...station,
    label: value.name.trim(),
    iconKey: value.icon.trim() || undefined,
    order,
    pinned: value.pinned,
    defaultRouteId: value.defaultRoute.trim() || undefined,
  };
}

export interface StationEditorLabels {
  title: string;
  description: string;
  close: string;
  fields: {
    name: string;
    icon: string;
    order: string;
    pinned: string;
    defaultRoute: string;
  };
  placeholders?: {
    name?: string;
    icon?: string;
    defaultRoute?: string;
  };
  actions: {
    cancel: string;
    save: string;
  };
}

export interface StationEditorProps {
  open: boolean;
  value: StationEditorValue;
  labels: StationEditorLabels;
  iconOptions?: readonly StationEditorOption[];
  routeOptions?: readonly StationEditorOption[];
  disabled?: boolean;
  submitting?: boolean;
  side?: "left" | "right";
  className?: string;
  portalContainer?: HTMLElement | null;
  renderIconPreview?: (icon: string) => ReactNode;
  onChange: (value: StationEditorValue) => void;
  onOpenChange: (open: boolean) => void;
  onSubmit: (value: StationEditorValue) => void;
}

/**
 * Controlled sheet presentation. Context-menu ownership intentionally remains
 * with the station trigger so the same editor can also be opened elsewhere.
 */
export function StationEditor({
  open,
  value,
  labels,
  iconOptions = [],
  routeOptions = [],
  disabled = false,
  submitting = false,
  side = "right",
  className,
  portalContainer,
  renderIconPreview,
  onChange,
  onOpenChange,
  onSubmit,
}: StationEditorProps) {
  return (
    <Sheet onOpenChange={onOpenChange} open={open}>
      <SheetContent
        className={cn("app-station-editor__sheet", className)}
        portalContainer={portalContainer}
        side={side}
        size="sm"
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
        <StationEditorForm
          disabled={disabled}
          iconOptions={iconOptions}
          labels={labels}
          onCancel={() => onOpenChange(false)}
          onChange={onChange}
          onSubmit={onSubmit}
          renderIconPreview={renderIconPreview}
          routeOptions={routeOptions}
          submitting={submitting}
          value={value}
        />
      </SheetContent>
    </Sheet>
  );
}

export interface StationEditorFormProps
  extends Omit<
    FormHTMLAttributes<HTMLFormElement>,
    "onChange" | "onSubmit"
  > {
  value: StationEditorValue;
  labels: StationEditorLabels;
  iconOptions?: readonly StationEditorOption[];
  routeOptions?: readonly StationEditorOption[];
  disabled?: boolean;
  submitting?: boolean;
  renderIconPreview?: (icon: string) => ReactNode;
  onChange: (value: StationEditorValue) => void;
  onCancel: () => void;
  onSubmit: (value: StationEditorValue) => void;
}

export function StationEditorForm({
  value,
  labels,
  iconOptions = [],
  routeOptions = [],
  disabled = false,
  submitting = false,
  renderIconPreview,
  onChange,
  onCancel,
  onSubmit,
  className,
  ...props
}: StationEditorFormProps) {
  const fieldId = useId();
  const nameId = `${fieldId}-name`;
  const iconId = `${fieldId}-icon`;
  const orderId = `${fieldId}-order`;
  const pinnedId = `${fieldId}-pinned`;
  const defaultRouteId = `${fieldId}-default-route`;
  const controlsDisabled = disabled || submitting;
  const orderIsValid = Number.isFinite(value.order) && value.order >= 0;
  const canSubmit =
    !controlsDisabled &&
    orderIsValid &&
    value.name.trim().length > 0 &&
    value.defaultRoute.trim().length > 0;

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (canSubmit) {
      onSubmit(value);
    }
  };

  return (
    <form
      {...props}
      aria-label={labels.title}
      className={cn("app-station-editor__form", className)}
      onSubmit={handleSubmit}
    >
      <SheetBody className="app-station-editor__fields">
        <StationEditorField htmlFor={nameId} label={labels.fields.name}>
          <Input
            autoComplete="off"
            disabled={controlsDisabled}
            id={nameId}
            onChange={(event) =>
              onChange({ ...value, name: event.currentTarget.value })
            }
            placeholder={labels.placeholders?.name}
            required
            value={value.name}
          />
        </StationEditorField>

        <StationEditorField htmlFor={iconId} label={labels.fields.icon}>
          <div className="app-station-editor__icon-control">
            {renderIconPreview ? (
              <span className="app-station-editor__icon-preview">
                {renderIconPreview(value.icon)}
              </span>
            ) : null}
            <StationEditorChoice
              disabled={controlsDisabled}
              id={iconId}
              onChange={(icon) => onChange({ ...value, icon })}
              options={iconOptions}
              placeholder={labels.placeholders?.icon}
              value={value.icon}
            />
          </div>
        </StationEditorField>

        <div className="app-station-editor__row">
          <StationEditorField htmlFor={orderId} label={labels.fields.order}>
            <Input
              disabled={controlsDisabled}
              id={orderId}
              min={0}
              onChange={(event) => {
                const nextOrder = Number(event.currentTarget.value);
                onChange({
                  ...value,
                  order: Number.isFinite(nextOrder)
                    ? Math.max(0, Math.trunc(nextOrder))
                    : value.order,
                });
              }}
              step={1}
              type="number"
              value={Number.isFinite(value.order) ? value.order : 0}
            />
          </StationEditorField>
          <label className="app-station-editor__pinned" htmlFor={pinnedId}>
            <input
              checked={value.pinned}
              disabled={controlsDisabled}
              id={pinnedId}
              onChange={(event) =>
                onChange({ ...value, pinned: event.currentTarget.checked })
              }
              type="checkbox"
            />
            <span>{labels.fields.pinned}</span>
          </label>
        </div>

        <StationEditorField
          htmlFor={defaultRouteId}
          label={labels.fields.defaultRoute}
        >
          <StationEditorChoice
            disabled={controlsDisabled}
            id={defaultRouteId}
            onChange={(defaultRoute) => onChange({ ...value, defaultRoute })}
            options={routeOptions}
            placeholder={labels.placeholders?.defaultRoute}
            required
            value={value.defaultRoute}
          />
        </StationEditorField>
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
        <Button disabled={!canSubmit} type="submit">
          {labels.actions.save}
        </Button>
      </SheetFooter>
    </form>
  );
}

interface StationEditorFieldProps {
  htmlFor: string;
  label: string;
  children: ReactNode;
}

function StationEditorField({
  htmlFor,
  label,
  children,
}: StationEditorFieldProps) {
  return (
    <label className="app-station-editor__field" htmlFor={htmlFor}>
      <span className="app-station-editor__field-label">{label}</span>
      {children}
    </label>
  );
}

interface StationEditorChoiceProps {
  id: string;
  value: string;
  options: readonly StationEditorOption[];
  placeholder?: string;
  disabled: boolean;
  required?: boolean;
  onChange: (value: string) => void;
}

function StationEditorChoice({
  id,
  value,
  options,
  placeholder,
  disabled,
  required,
  onChange,
}: StationEditorChoiceProps) {
  if (options.length === 0) {
    return (
      <Input
        disabled={disabled}
        id={id}
        onChange={(event) => onChange(event.currentTarget.value)}
        placeholder={placeholder}
        required={required}
        value={value}
      />
    );
  }

  return (
    <Select
      className="app-station-editor__select"
      disabled={disabled}
      id={id}
      onChange={(event) => onChange(event.currentTarget.value)}
      required={required}
      value={value}
    >
      {placeholder ? (
        <option disabled value="">
          {placeholder}
        </option>
      ) : null}
      {options.map((option) => (
        <option disabled={option.disabled} key={option.value} value={option.value}>
          {option.label}
        </option>
      ))}
    </Select>
  );
}
