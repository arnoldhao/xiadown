import { describe, expect, test } from "bun:test";

const featureSources = [
  "../../app/media/FlvPreview.tsx",
  "../../app/main/NewTaskDialog.tsx",
  "../../app/main/completed/detail-components.tsx",
  "../../app/main/listen/ui.tsx",
  "../../app/main/listen/HushLiveList.tsx",
] as const;

async function read(relativePath: string) {
  return Bun.file(new URL(relativePath, import.meta.url)).text();
}

describe("Dream feature visual recipes", () => {
  test("keeps material filters out of feature Tailwind utilities", async () => {
    const sources = await Promise.all(featureSources.map(read));

    for (const source of sources) {
      expect(source).not.toContain("backdrop-blur");
    }
  });

  test("routes FLV, batch-table and selection visuals through Dream classes", async () => {
    const [flv, newTask, completed, completedCSS, workflowsCSS, buttonCSS] =
      await Promise.all([
        read("../../app/media/FlvPreview.tsx"),
        read("../../app/main/NewTaskDialog.tsx"),
        read("../../app/main/completed/detail-components.tsx"),
        read("./dream/completed.css"),
        read("./dream/workflows.css"),
        read("./dream/button-contract.css"),
      ]);

    expect(flv).toContain("app-flv-preview-fullscreen-action");
    expect(flv).toContain("app-flv-preview-loading-overlay");
    expect(flv).not.toContain("!bg-black");
    expect(completedCSS).toContain(".app-flv-preview-loading-overlay");
    expect(buttonCSS).toContain(".app-flv-preview-fullscreen-action");

    expect(newTask).toContain("app-new-task-batch-table-head");
    expect(workflowsCSS).toContain(".app-new-task-batch-table-head");

    expect(completed).toContain("app-completed-selection-checkbox");
    expect(completed).not.toContain("backdrop-blur-sm");
    expect(workflowsCSS).toContain(".app-completed-selection-checkbox");
  });

  test("shares one semantic Listen scroll and Lofi-card recipe", async () => {
    const [online, lofi, workflowsCSS, motionCSS] = await Promise.all([
      read("../../app/main/listen/ui.tsx"),
      read("../../app/main/listen/HushLiveList.tsx"),
      read("./dream/workflows.css"),
      read("./dream/motion.css"),
    ]);

    for (const source of [online, lofi]) {
      expect(source).toContain("listen-horizontal-scroll-fade");
      expect(source).toContain("app-listen-horizontal-scroll-control");
      expect(source).toContain("data-side={props.side}");
      expect(source).not.toContain("from-[hsl(var(--sidebar-background)");
    }

    expect(lofi).toContain('imageClassName="listen-hush-card-image"');
    expect(lofi).toContain("app-listen-hush-card-action-group");
    expect(workflowsCSS).toContain(
      '.listen-horizontal-scroll-fade[data-side="left"]',
    );
    expect(workflowsCSS).toContain(".listen-hush-card-artwork");
    expect(motionCSS).toContain(".listen-hush-card-image");
    expect(motionCSS).toContain(".app-listen-hush-card-action-reveal");
  });

  test("keeps workflow typography and compact chrome discoverable in Dream", async () => {
    const [
      running,
      newTask,
      activity,
      tray,
      main,
      preview,
      workflowsCSS,
      activityCSS,
      anatomyCSS,
      workspaceCSS,
      completedCSS,
    ] = await Promise.all([
      read("../../app/main/RunningPage.tsx"),
      read("../../app/main/NewTaskDialog.tsx"),
      read("../../app/main/WorkspaceActivitySurfaces.tsx"),
      read("../../app/main/TrayMiniPlayerApp.tsx"),
      read("../../app/main/MainApp.tsx"),
      read("../../app/media/VidstackPreview.tsx"),
      read("./dream/workflows.css"),
      read("./dream/activity.css"),
      read("./dream/anatomy.css"),
      read("./dream/workspace.css"),
      read("./dream/completed.css"),
    ]);

    expect(running).toContain("app-running-meta-cell--source");
    expect(running).not.toMatch(/app-running-meta-cell[^"\n]*text-\[/);
    expect(workflowsCSS).toContain(".app-running-meta-cell--created");
    expect(workflowsCSS).toContain(".app-running-operation-name");

    expect(newTask).toContain('className="app-new-task-batch-table-head-row"');
    expect(newTask).not.toMatch(/app-new-task-batch-table-head-row[^"\n]*text-\[/);
    expect(workflowsCSS).toContain(".app-new-task-batch-table-head-row th");

    expect(activity).toContain("app-workspace-sniff-session-tooltip__address");
    expect(activityCSS).toContain(".app-workspace-sniff-session-tooltip__address");
    expect(tray).toContain('className="tray-mini-player-root"');
    expect(anatomyCSS).toContain(".tray-mini-player-root");

    expect(main).toContain('className="app-workspace-session-avatar"');
    expect(main).not.toContain('fallbackClassName="text-[10px]');
    expect(workspaceCSS).toContain(".app-workspace-session-avatar");

    expect(preview).toContain("app-completed-preview-time--current");
    expect(preview).not.toMatch(/app-completed-preview-time[^"\n]*text-\[/);
    expect(completedCSS).toContain(".app-completed-preview-time--duration");
  });
});
