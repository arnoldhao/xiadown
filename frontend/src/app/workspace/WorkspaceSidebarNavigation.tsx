import type { HTMLAttributes, ReactNode } from "react";

import type {
  WorkspaceSidebarNavigationItem,
  WorkspaceSidebarSection,
  WorkspaceSidebarChromeSlots,
} from "@/app/workspace/navigation-types";
import type { WorkspaceRouteId } from "@/app/workspace/types";
import { WorkspaceSidebar } from "@/app/workspace/WorkspaceSidebar";
import { cn } from "@/lib/utils";

import "./workspace-navigation.css";

export interface WorkspaceSidebarNavigationProps
  extends Omit<HTMLAttributes<HTMLElement>, "onNavigate">,
    WorkspaceSidebarChromeSlots {
  ariaLabel: string;
  activeRouteId?: WorkspaceRouteId | null;
  bottomOrder?: "activity-first" | "control-first";
  glass?: boolean;
  sections: readonly WorkspaceSidebarSection[];
  onNavigate: (routeId: WorkspaceRouteId) => void;
}

export function WorkspaceSidebarNavigation({
  ariaLabel,
  activeRouteId,
  sections,
  onNavigate,
  header,
  controlPanel,
  activity,
  account,
  workspaceSwitcher,
  newAction,
  bottomOrder = "activity-first",
  className,
  ...props
}: WorkspaceSidebarNavigationProps) {
  const hasControlPanel = controlPanel != null;
  const hasActivity = activity != null;
  const hasUpperBottomContent = hasControlPanel || hasActivity;
  const hasAccount = account != null;
  const hasWorkspaceSwitcher = workspaceSwitcher != null;
  const hasNewAction = newAction != null;
  const activityRegion = hasActivity ? (
    <div className="app-workspace-wide-sidebar__activity">{activity}</div>
  ) : null;
  const controlPanelRegion = hasControlPanel ? (
    <div className="app-workspace-wide-sidebar__control-panel">
      {controlPanel}
    </div>
  ) : null;
  const bottom = hasControlPanel || hasActivity || hasAccount ? (
    <div
      className="app-workspace-wide-sidebar__bottom"
      data-has-upper-content={hasUpperBottomContent ? "true" : "false"}
    >
      {bottomOrder === "control-first" ? controlPanelRegion : activityRegion}
      {bottomOrder === "control-first" ? activityRegion : controlPanelRegion}
      {hasAccount ? (
        <div className="app-workspace-wide-sidebar__account">
          {account}
        </div>
      ) : null}
    </div>
  ) : undefined;
  const footer = hasWorkspaceSwitcher || hasNewAction ? (
    <WorkspaceSidebarFooter
      newAction={newAction}
      workspaceSwitcher={workspaceSwitcher}
    />
  ) : undefined;

  return (
    <WorkspaceSidebar
      {...props}
      aria-label={ariaLabel}
      bottom={bottom}
      className={cn("app-workspace-wide-sidebar", className)}
      footer={footer}
      header={header}
    >
      <div className="app-workspace-wide-sidebar__sections">
        {sections.map((section) => (
          <WorkspaceNavigationSection
            activeRouteId={activeRouteId}
            key={section.id}
            onNavigate={onNavigate}
            section={section}
          />
        ))}
      </div>
    </WorkspaceSidebar>
  );
}

export interface WorkspaceSidebarFooterProps {
  workspaceSwitcher?: ReactNode;
  newAction?: ReactNode;
}

export function WorkspaceSidebarFooter({
  workspaceSwitcher,
  newAction,
}: WorkspaceSidebarFooterProps) {
  return (
    <div className="app-workspace-wide-sidebar__footer">
      {workspaceSwitcher != null ? (
        <div className="app-workspace-wide-sidebar__switcher">
          {workspaceSwitcher}
        </div>
      ) : null}
      {newAction != null ? (
        <div className="app-workspace-wide-sidebar__new">{newAction}</div>
      ) : null}
    </div>
  );
}

interface WorkspaceNavigationSectionProps {
  activeRouteId?: WorkspaceRouteId | null;
  onNavigate: (routeId: WorkspaceRouteId) => void;
  section: WorkspaceSidebarSection;
}

function WorkspaceNavigationSection({
  activeRouteId,
  onNavigate,
  section,
}: WorkspaceNavigationSectionProps) {
  const hasItems = Boolean(section.items?.length);
  const hasContent = section.content != null;
  if (!hasItems && !hasContent && !section.label) {
    return null;
  }

  return (
    <section
      aria-label={section.label}
      className="app-workspace-nav-section"
      data-section={section.id}
    >
      {section.label || section.action ? (
        <div className="app-workspace-nav-section__header">
          {section.label ? section.labelRouteId ? (
            <button
              aria-current={activeRouteId === section.labelRouteId ? "page" : undefined}
              className="app-workspace-nav-section__label app-workspace-nav-section__label-button"
              data-active={activeRouteId === section.labelRouteId}
              data-route-id={section.labelRouteId}
              onClick={() => onNavigate(section.labelRouteId!)}
              title={section.label}
              type="button"
            >
              {section.labelIcon ? (
                <span aria-hidden="true">{section.labelIcon}</span>
              ) : null}
              <span>{section.label}</span>
            </button>
          ) : (
            <h2 className="app-workspace-nav-section__label">{section.label}</h2>
          ) : <span />}
          {section.action ? (
            <div className="app-workspace-nav-section__action">
              {section.action}
            </div>
          ) : null}
        </div>
      ) : null}
      {hasItems ? (
        <ul className="app-workspace-nav-list">
          {section.items?.map((item) => (
            <WorkspaceNavigationItem
              active={activeRouteId === item.routeId}
              item={item}
              key={item.routeId}
              onNavigate={onNavigate}
            />
          ))}
        </ul>
      ) : null}
      {hasContent ? (
        <div className="app-workspace-nav-section__content">
          {section.content}
        </div>
      ) : null}
    </section>
  );
}

interface WorkspaceNavigationItemProps {
  active: boolean;
  item: WorkspaceSidebarNavigationItem;
  onNavigate: (routeId: WorkspaceRouteId) => void;
}

function WorkspaceNavigationItem({
  active,
  item,
  onNavigate,
}: WorkspaceNavigationItemProps) {
  return (
    <li className="app-workspace-nav-list__item">
      <button
        aria-current={active ? "page" : undefined}
        aria-label={item.ariaLabel ?? item.label}
        className="app-workspace-nav-button"
        data-active={active}
        data-route-id={item.routeId}
        disabled={item.disabled}
        onClick={() => onNavigate(item.routeId)}
        onContextMenu={item.onContextMenu}
        onKeyDown={item.onKeyDown}
        title={item.tooltip}
        type="button"
      >
        {item.icon ? (
          <span aria-hidden="true" className="app-workspace-nav-button__icon">
            {item.icon}
          </span>
        ) : null}
        <span className="app-workspace-nav-button__label">{item.label}</span>
        {item.badge ? (
          <span className="app-workspace-nav-button__badge">{item.badge}</span>
        ) : null}
      </button>
    </li>
  );
}
