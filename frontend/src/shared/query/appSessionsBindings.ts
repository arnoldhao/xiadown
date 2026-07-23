type AppSessionsHandlerBindings = typeof import(
  "../../../bindings/xiadown/internal/presentation/wails/appsessionshandler"
);

let appSessionsHandlerBindings: Promise<AppSessionsHandlerBindings> | undefined;

export function loadAppSessionsHandlerBindings(): Promise<AppSessionsHandlerBindings> {
  appSessionsHandlerBindings ??= import(
    "../../../bindings/xiadown/internal/presentation/wails/appsessionshandler"
  );
  return appSessionsHandlerBindings;
}
