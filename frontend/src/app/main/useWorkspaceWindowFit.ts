import { Events, Window } from "@wailsio/runtime";
import * as React from "react";

import {
  COMPANION_PANEL_WIDTH,
  type CompanionPresentation,
} from "@/app/workspace";
import {
  resolveCompanionWindowPlan,
  WORKSPACE_WINDOW_EDGE_GAP,
} from "@/app/main/workspace-window-fit";

export function useWorkspaceWindowFit(companionOpen: boolean) {
  const [presentation, setPresentation] =
    React.useState<CompanionPresentation>("docked");
  const presentationRef = React.useRef(presentation);
  const openRef = React.useRef(companionOpen);
  const requestRef = React.useRef(0);
  const reportedMinimumWidthRef = React.useRef(0);

  // AppShell reports its minimum from a child effect. Keep these refs in sync
  // during render so the first open cannot observe the previous render's
  // closed state while React is flushing passive effects child-first.
  openRef.current = companionOpen;
  presentationRef.current = presentation;

  React.useEffect(() => {
    if (!companionOpen) {
      requestRef.current += 1;
      presentationRef.current = "docked";
      setPresentation("docked");
    }
  }, [companionOpen]);

  const fitMinimumWidth = React.useCallback((reportedMinimumWidth: number) => {
    reportedMinimumWidthRef.current = reportedMinimumWidth;
    if (!openRef.current) {
      return;
    }
    const request = ++requestRef.current;
    const requiredDockedWidth =
      reportedMinimumWidth +
      (presentationRef.current === "overlay" ? COMPANION_PANEL_WIDTH : 0);

    void Promise.all([
      Window.IsFullscreen(),
      Window.IsMaximised(),
      Window.Size(),
      Window.Position(),
      Window.GetScreen().catch(() => null),
    ])
      .then(async ([fullscreen, maximized, size, position, screen]) => {
        if (request !== requestRef.current || !openRef.current) {
          return;
        }
        const workAreaWidth = Number(
          screen?.WorkArea?.Width ?? window.screen.availWidth,
        );
        const plan = resolveCompanionWindowPlan({
          open: true,
          currentWidth: size.width,
          requiredDockedWidth,
          workAreaWidth,
          fullscreen,
          maximized,
        });
        presentationRef.current = plan.presentation;
        setPresentation(plan.presentation);
        if (!plan.targetWidth || plan.presentation !== "docked") {
          return;
        }
        const workAreaX = Number(screen?.WorkArea?.X ?? 0);
        const workAreaRight = workAreaX + workAreaWidth;
        const targetX = Math.max(
          workAreaX + WORKSPACE_WINDOW_EDGE_GAP / 2,
          Math.min(
            position.x,
            workAreaRight - plan.targetWidth - WORKSPACE_WINDOW_EDGE_GAP / 2,
          ),
        );
        await Window.SetSize(plan.targetWidth, size.height);
        if (Math.abs(targetX - position.x) >= 1) {
          await Window.SetPosition(Math.round(targetX), position.y);
        }
      })
      .catch(() => {
        if (request !== requestRef.current || !openRef.current) {
          return;
        }
        const plan = resolveCompanionWindowPlan({
          open: true,
          currentWidth: window.innerWidth,
          requiredDockedWidth,
          workAreaWidth: window.screen.availWidth,
          fullscreen: false,
          maximized: false,
        });
        presentationRef.current = plan.presentation;
        setPresentation(plan.presentation);
      });
  }, []);

  React.useEffect(() => {
    if (!companionOpen) {
      return;
    }
    let timer = 0;
    const scheduleFit = () => {
      window.clearTimeout(timer);
      timer = window.setTimeout(() => {
        const minimumWidth = reportedMinimumWidthRef.current;
        if (minimumWidth > 0 && openRef.current) {
          fitMinimumWidth(minimumWidth);
        }
      }, 100);
    };
    // Re-evaluate after the opening render and whenever Wails reports a native
    // presentation transition, the viewport resizes, or the display changes.
    scheduleFit();
    const removeNativeListeners = [
      Events.On(Events.Types.Common.WindowFullscreen, scheduleFit),
      Events.On(Events.Types.Common.WindowUnFullscreen, scheduleFit),
      Events.On(Events.Types.Common.WindowMaximise, scheduleFit),
      Events.On(Events.Types.Common.WindowUnMaximise, scheduleFit),
      Events.On(Events.Types.Common.WindowRestore, scheduleFit),
    ];
    window.addEventListener("resize", scheduleFit);
    window.screen.orientation?.addEventListener?.("change", scheduleFit);
    return () => {
      window.clearTimeout(timer);
      removeNativeListeners.forEach((removeListener) => removeListener());
      window.removeEventListener("resize", scheduleFit);
      window.screen.orientation?.removeEventListener?.("change", scheduleFit);
    };
  }, [companionOpen, fitMinimumWidth]);

  return { companionPresentation: presentation, fitMinimumWidth };
}
