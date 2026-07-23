import { describe, expect, test } from "bun:test";

const read = (relativePath: string) =>
  Bun.file(new URL(relativePath, import.meta.url)).text();

describe("comfortable product action sizing", () => {
  test("keeps one shared comfortable and choice vocabulary", async () => {
    const tokens = await read("../styles/dream/tokens.css");

    expect(tokens).toContain("--app-button-block-size-comfortable: 2.5rem;");
    expect(tokens).toContain("--app-button-block-size-choice: 2.75rem;");
  });

  test("maps every historically tall product action through Dream variables", async () => {
    const [
      workflows,
      settings,
      listen,
      running,
      sniff,
      pets,
      sessions,
      settingsApp,
      pageView,
      newTask,
    ] = await Promise.all([
      read("../styles/dream/workflows.css"),
      read("../styles/dream/settings.css"),
      read("../styles/dream/listen.css"),
      read("../../app/main/RunningPage.tsx"),
      read("../../app/sniff-desk/SniffDeskPage.tsx"),
      read("../../app/pets-gallery/PetsGalleryPage.tsx"),
      read("../../features/settings/app-sessions/index.tsx"),
      read("../../app/settings/SettingsApp.tsx"),
      read("../../app/main/listen/PageView.tsx"),
      read("../../app/main/NewTaskDialog.tsx"),
    ]);

    expect(workflows).toMatch(
      /\.app-running-new-download-button[^{}]*\{[^}]*--app-button-block-size:\s*var\(--app-button-block-size-comfortable\)/s,
    );
    expect(settings).toMatch(
      /\.app-settings-theme-pack-button\s*\{[^}]*--app-button-block-size:\s*var\(--app-button-block-size-choice\)/s,
    );
    expect(settings).toMatch(
      /\.app-settings-option-button\s*\{[^}]*--app-button-block-size:\s*var\(--app-button-block-size-comfortable\)/s,
    );
    expect(settings).toMatch(
      /\.app-sessions-account-action\s*\{[^}]*--app-button-block-size:\s*var\(--app-button-block-size-comfortable\)/s,
    );
    expect(listen).toMatch(
      /\.listen-muse-account-action\s*\{[^}]*--app-button-block-size:\s*var\(--app-button-block-size-comfortable\)/s,
    );
    expect(workflows).toMatch(
      /\.app-new-task-transcode-source-choice\s*\{[^}]*--app-button-block-size:\s*auto;[^}]*--app-button-padding:\s*var\(--app-space-4\);[^}]*min-height:\s*max\(6rem, var\(--app-button-block-size-choice\)\)/s,
    );

    expect(running).not.toContain("app-running-new-download-button h-10");
    expect(sniff).not.toContain("app-running-new-download-button h-10");
    expect(pets).not.toContain("app-running-new-download-button h-10");
    expect(sessions).not.toContain('className="h-10 min-w-0"');
    expect(settingsApp).not.toContain("app-settings-theme-pack-button app-motion-surface flex h-11");
    expect(settingsApp.match(/app-settings-option-button/g)?.length).toBeGreaterThanOrEqual(4);
    expect(pageView.match(/listen-muse-account-action/g)).toHaveLength(4);
    expect(pageView).not.toContain('className="h-10 w-auto min-w-44 px-5"');
    expect(newTask.match(/app-new-task-transcode-source-choice/g)).toHaveLength(2);
    expect(newTask).not.toContain('className="h-auto min-h-24 flex-col gap-2 py-4"');
  });

  test("restores wide prompt padding without resizing transport or tool buttons", async () => {
    const [listen, promptSource] = await Promise.all([
      read("../styles/dream/listen.css"),
      read("../../app/main/listen/ui.tsx"),
    ]);

    expect(listen).toMatch(
      /\.app-dream-button\.listen-connection-prompt__action[^{}]*\{[^}]*--app-button-padding:\s*0 var\(--app-space-5\)/s,
    );
    expect(promptSource).toContain('className="listen-connection-prompt__action"');
    expect(promptSource).not.toContain('className="listen-connection-prompt__action px-5"');
    expect(listen).not.toMatch(
      /\.listen-transport-icon-button[^{}]*--app-button-block-size-comfortable/s,
    );
  });
});
