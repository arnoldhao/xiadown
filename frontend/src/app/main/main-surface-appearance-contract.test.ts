import { expect, test } from "bun:test";

const read = (relativePath: string) =>
  Bun.file(new URL(relativePath, import.meta.url)).text();

test("main workflow surfaces delegate static appearance to Dream selectors", async () => {
  const [newTask, running, dependency, mainApp, activity, messageHost] =
    await Promise.all([
      read("./NewTaskDialog.tsx"),
      read("./RunningPage.tsx"),
      read("./dependency-repair-card.tsx"),
      read("./MainApp.tsx"),
      read("./WorkspaceActivitySurfaces.tsx"),
      read("../../shared/message/MessageHost.tsx"),
    ]);

  expect(newTask).toContain("app-new-task-library-item");
  expect(newTask).toContain("app-new-task-row-label");
  expect(newTask).not.toMatch(/(?:bg|text|border)-(?:primary|muted|foreground|accent|border)(?:\/|\b)/);
  expect(running).toContain("app-running-summary-line");
  expect(running).not.toContain("bg-primary/[0.10]");
  expect(dependency).toContain("app-dependency-item-title");
  expect(dependency).not.toContain("tracking-[0.08em]");
  expect(mainApp).toContain('aria-current={active ? "page" : undefined}');
  expect(mainApp).not.toContain('active && "bg-accent/60"');
  expect(activity).toContain("app-workspace-status-context-anchor");
  expect(activity).not.toContain("opacity-0 outline-none");
  expect(messageHost).toContain("app-message-dialog-title");
  expect(messageHost).toContain("data-standalone={!title || undefined}");
});

test("Dream modules catalog the migrated main workflow selectors", async () => {
  const [workflows, components, activity, shell] = await Promise.all([
    read("../../shared/styles/dream/workflows.css"),
    read("../../shared/styles/dream/components.css"),
    read("../../shared/styles/dream/activity.css"),
    read("../../shared/styles/dream/shell.css"),
  ]);

  for (const selector of [
    ".app-new-task-library-item",
    ".app-running-summary-line",
    ".app-dependency-item-title",
  ]) {
    expect(workflows).toContain(selector);
  }
  expect(components).toContain(".app-message-dialog-title");
  expect(activity).toContain(".app-workspace-status-context-anchor");
  expect(shell).toContain('.app-sidebar-dropdown-item[aria-current="page"]');
});
