import { CheckCircle2 } from "lucide-react";
import * as React from "react";

import { getXiaText } from "@/features/xiadown/shared";
import { useDismissWhatsNew, useWhatsNew } from "@/shared/query/update";
import { Button } from "@/shared/ui/button";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/shared/ui/dialog";

const DialogMarkdown = React.lazy(() =>
  import("@/shared/markdown/dialog-markdown").then(({ DialogMarkdown: Component }) => ({
    default: Component,
  })),
);

export function WhatsNewFeatureDialog(props: {
  blocked: boolean;
  language?: string;
}) {
  const text = getXiaText(props.language);
  const whatsNewQuery = useWhatsNew();
  const dismissMutation = useDismissWhatsNew();
  const [open, setOpen] = React.useState(false);

  React.useEffect(() => {
    if (props.blocked) {
      setOpen(false);
      return;
    }
    if (whatsNewQuery.data?.version) {
      setOpen(true);
    }
  }, [props.blocked, whatsNewQuery.data?.version]);

  const handleClose = async () => {
    if (whatsNewQuery.data?.version) {
      await dismissMutation.mutateAsync(whatsNewQuery.data.version);
    }
    setOpen(false);
  };

  return (
    <Dialog open={open}>
      <DialogContent
        showCloseButton={false}
        className="grid max-h-[min(34rem,calc(100vh-2rem))] max-w-[min(92vw,40rem)] grid-rows-[minmax(0,1fr)] gap-0 overflow-hidden p-0"
        onEscapeKeyDown={(event) => event.preventDefault()}
        onPointerDownOutside={(event) => event.preventDefault()}
      >
        <div className="grid min-h-0 grid-rows-[auto_minmax(0,1fr)_auto] gap-5 overflow-hidden p-6 sm:p-7">
          <DialogHeader className="app-whats-new-header space-y-2">
            <div className="app-whats-new-eyebrow inline-flex w-fit items-center gap-2 px-3 py-1">
              <CheckCircle2 className="h-3.5 w-3.5" />
              {text.appName}
            </div>
            <DialogTitle className="app-whats-new-title">
              {text.whatsNew.title}{" "}
              {whatsNewQuery.data?.version
                ? `v${whatsNewQuery.data.version}`
                : ""}
            </DialogTitle>
          </DialogHeader>
          <div className="min-h-0 overflow-y-auto pr-1">
            <div className="app-whats-new-changelog p-4">
              {whatsNewQuery.data?.changelog?.trim() ? (
                <React.Suspense
                  fallback={<div className="app-whats-new-empty" aria-busy="true" />}
                >
                  <DialogMarkdown
                    content={whatsNewQuery.data.changelog}
                    className="max-h-none overflow-visible"
                  />
                </React.Suspense>
              ) : (
                <div className="app-whats-new-empty">
                  {text.whatsNew.empty}
                </div>
              )}
            </div>
          </div>
          <DialogFooter>
            <Button
              type="button"
              onClick={() => void handleClose()}
              disabled={dismissMutation.isPending}
            >
              {text.actions.close}
            </Button>
          </DialogFooter>
        </div>
      </DialogContent>
    </Dialog>
  );
}
