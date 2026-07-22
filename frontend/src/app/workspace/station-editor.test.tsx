import { describe, expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";

import {
  applyStationEditorValue,
  stationToEditorValue,
  StationEditorForm,
  type StationEditorLabels,
  type StationEditorValue,
} from "./StationEditor";

const labels: StationEditorLabels = {
  title: "editor.title",
  description: "editor.description",
  close: "editor.close",
  fields: {
    name: "field.name",
    icon: "field.icon",
    order: "field.order",
    pinned: "field.pinned",
    defaultRoute: "field.default.route",
  },
  placeholders: {
    name: "placeholder.name",
    icon: "placeholder.icon",
    defaultRoute: "placeholder.route",
  },
  actions: {
    cancel: "action.cancel",
    save: "action.save",
  },
};

const value: StationEditorValue = {
  name: "Music",
  icon: "music",
  order: 2,
  pinned: true,
  defaultRoute: "home",
};

describe("station editor form", () => {
  test("uses the shared side Sheet without a local material recipe", async () => {
    const css = await Bun.file(
      new URL("./workspace-navigation.css", import.meta.url),
    ).text();
    const source = await Bun.file(
      new URL("./StationEditor.tsx", import.meta.url),
    ).text();

    expect(source).toContain('from "@/shared/ui/sheet"');
    expect(source).toContain("<SheetContent");
    expect(source).toContain("side={side}");
    expect(source).toContain('size="sm"');
    expect(css).not.toContain(".app-station-editor__sheet");
    expect(css).not.toContain(".app-station-editor__overlay");
  });

  test("maps editable values to the persisted station shape", () => {
    const station = {
      id: "music",
      workspaceId: "music",
      label: "Music",
      iconKey: "music",
      order: 0,
      enabled: true,
      defaultRouteId: "home",
    };
    const draft = stationToEditorValue(station);
    const updated = applyStationEditorValue(station, {
      ...draft,
      name: "  Radio  ",
      icon: " radio ",
      order: 3.8,
      pinned: false,
      defaultRoute: " radio ",
    });

    expect(draft).toEqual({
      name: "Music",
      icon: "music",
      order: 0,
      pinned: true,
      defaultRoute: "home",
    });
    expect(updated).toEqual({
      ...station,
      label: "Radio",
      iconKey: "radio",
      order: 3,
      pinned: false,
      defaultRouteId: "radio",
    });
  });

  test("renders all controlled station fields and supplied labels", () => {
    const markup = renderToStaticMarkup(
      <StationEditorForm
        iconOptions={[
          { value: "music", label: "icon.music" },
          { value: "radio", label: "icon.radio" },
        ]}
        labels={labels}
        onCancel={() => undefined}
        onChange={() => undefined}
        onSubmit={() => undefined}
        routeOptions={[
          { value: "home", label: "route.home" },
          { value: "radio", label: "route.radio" },
        ]}
        value={value}
      />,
    );

    expect(markup).toContain('aria-label="editor.title"');
    expect(markup).toContain('value="Music"');
    expect(markup).toContain('value="2"');
    expect(markup).toContain("field.pinned");
    expect(markup).toContain('checked=""');
    expect(markup).toContain('value="music" selected=""');
    expect(markup).toContain('value="home" selected=""');
    expect(markup).toContain(">action.cancel<");
    expect(markup).toContain(">action.save<");
  });

  test("disables save when required controlled values are empty", () => {
    const markup = renderToStaticMarkup(
      <StationEditorForm
        labels={labels}
        onCancel={() => undefined}
        onChange={() => undefined}
        onSubmit={() => undefined}
        value={{ ...value, name: "", defaultRoute: "" }}
      />,
    );

    expect(markup).toContain('disabled="" type="submit"');
  });
});
