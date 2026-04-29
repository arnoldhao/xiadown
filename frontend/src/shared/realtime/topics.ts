export const REALTIME_TOPICS = {
  system: {
    hello: "system.hello",
  },
  library: {
    operation: "library.operation",
    file: "library.file",
    history: "library.history",
    workspace: "library.workspace",
    workspaceProject: "library.workspace_project",
  },
  dreamfm: {
    liveCatalog: "dreamfm.live.catalog",
  },
} as const

type ExtractValues<T> = T extends string ? T : { [K in keyof T]: ExtractValues<T[K]> }[keyof T]

export type RealtimeTopic = ExtractValues<typeof REALTIME_TOPICS> | (string & {})

export const DEFAULT_DEBUG_TOPICS: RealtimeTopic[] = [
  REALTIME_TOPICS.library.operation,
  REALTIME_TOPICS.library.file,
]
