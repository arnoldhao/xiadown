import { beforeEach, describe, expect, test } from "bun:test";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";

import { getXiaText } from "@/features/xiadown/shared";
import {
  SNIFF_WORKSPACE_START_TIMEOUT_MS,
  attachSniffWorkspaceStartSession,
  beginSniffWorkspaceStart,
  clearSniffWorkspaceStart,
  SniffWorkspaceKindSelect,
  SniffWorkspaceResourceSelect,
  SniffWorkspaceSourceSelect,
  useSniffWorkspaceFilterStore,
  useSniffWorkspaceStartStore,
} from "./workspace-filters";

describe("sniff workspace filters", () => {
  beforeEach(() => {
    useSniffWorkspaceFilterStore.getState().reset();
    useSniffWorkspaceStartStore.setState({ pending: null });
  });

  test("shares sidebar filters with the workspace page", () => {
    const state = useSniffWorkspaceFilterStore.getState();
    state.setQuery("playlist");
    state.setKind("manifest");
    state.setSource("network");
    state.setDownload("downloadable");

    expect(useSniffWorkspaceFilterStore.getState()).toMatchObject({
      query: "playlist",
      kind: "manifest",
      source: "network",
      download: "downloadable",
    });
  });

  test("keeps an immediate sniff handoff pending until its session is attached", () => {
    const filters = useSniffWorkspaceFilterStore.getState();
    filters.setQuery("old query");
    filters.setKind("video");
    filters.setSource("candidate");
    filters.setDownload("downloadable");
    const requestId = beginSniffWorkspaceStart();

    expect(useSniffWorkspaceStartStore.getState().pending).toMatchObject({
      requestId,
      sessionId: "",
    });
    expect(
      useSniffWorkspaceStartStore.getState().pending?.startedAt,
    ).toBeNumber();
    expect(SNIFF_WORKSPACE_START_TIMEOUT_MS).toBe(15_000);
    expect(useSniffWorkspaceFilterStore.getState()).toMatchObject({
      query: "",
      kind: "all",
      source: "all",
      download: "all",
    });

    attachSniffWorkspaceStartSession(requestId, "session-1");
    expect(useSniffWorkspaceStartStore.getState().pending?.sessionId).toBe(
      "session-1",
    );

    clearSniffWorkspaceStart(requestId);
    expect(useSniffWorkspaceStartStore.getState().pending).toBeNull();
  });

  test("reset restores the unfiltered resource view", () => {
    const state = useSniffWorkspaceFilterStore.getState();
    state.setQuery("video");
    state.setKind("video");
    state.setSource("candidate");
    state.setDownload("downloadable");
    useSniffWorkspaceFilterStore.getState().reset();

    expect(useSniffWorkspaceFilterStore.getState()).toMatchObject({
      query: "",
      kind: "all",
      source: "all",
      download: "all",
    });
  });

  test("keeps type as a select and renders source and download as radio segments", () => {
    const text = getXiaText("en");
    const kindMarkup = renderToStaticMarkup(
      createElement(SniffWorkspaceKindSelect, { text }),
    );
    const sourceMarkup = renderToStaticMarkup(
      createElement(SniffWorkspaceSourceSelect, { text }),
    );
    const resourceMarkup = renderToStaticMarkup(
      createElement(SniffWorkspaceResourceSelect, { text }),
    );

    expect(kindMarkup).toContain("<select");
    expect(sourceMarkup).not.toContain("<select");
    expect(sourceMarkup).toContain('role="radiogroup"');
    expect(sourceMarkup.match(/type="radio"/g)).toHaveLength(4);
    expect(resourceMarkup).not.toContain("<select");
    expect(resourceMarkup).toContain('role="radiogroup"');
    expect(resourceMarkup.match(/type="radio"/g)).toHaveLength(2);
  });
});
