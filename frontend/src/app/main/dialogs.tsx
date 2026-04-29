import {
CheckCircle2,
} from "lucide-react";
import * as React from "react";

import {
getXiaText
} from "@/features/xiadown/shared";
import { DialogMarkdown } from "@/shared/markdown/dialog-markdown";
import {
useDismissWhatsNew,
useWhatsNew
} from "@/shared/query/update";
import { Button } from "@/shared/ui/button";
import {
Dialog,
DialogContent,
DialogFooter,
DialogHeader,
DialogTitle
} from "@/shared/ui/dialog";

export function WhatsNewFeatureDialog(props: { blocked: boolean; language?: string }) {
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
        className="max-w-[min(92vw,40rem)] border-0 bg-transparent p-0 shadow-none"
        onEscapeKeyDown={(event) => event.preventDefault()}
        onPointerDownOutside={(event) => event.preventDefault()}
      >
        <div className="overflow-hidden rounded-[26px] border border-white/45 bg-[linear-gradient(155deg,rgba(255,255,255,0.97),rgba(245,247,252,0.94)_50%,rgba(240,244,251,0.96)_100%)] shadow-[0_36px_100px_-48px_rgba(15,23,42,0.55)] dark:border-white/10 dark:bg-[linear-gradient(155deg,rgba(15,23,42,0.98),rgba(2,6,23,0.96)_50%,rgba(15,23,42,0.98)_100%)]">
          <div className="space-y-5 p-6 sm:p-7">
            <DialogHeader className="space-y-2 text-left">
              <div className="inline-flex w-fit items-center gap-2 rounded-full border border-white/60 bg-white/70 px-3 py-1 text-[11px] font-semibold uppercase tracking-[0.22em] text-slate-600 shadow-sm dark:border-white/10 dark:bg-white/5 dark:text-slate-300">
                <CheckCircle2 className="h-3.5 w-3.5" />
                {text.appName}
              </div>
              <DialogTitle className="text-2xl text-slate-950 dark:text-white">
                {text.whatsNew.title}{" "}
                {whatsNewQuery.data?.version
                  ? `v${whatsNewQuery.data.version}`
                  : ""}
              </DialogTitle>
            </DialogHeader>
            <div className="max-h-[22rem] overflow-y-auto rounded-2xl border border-border/60 bg-card/80 p-4">
              {whatsNewQuery.data?.changelog?.trim() ? (
                <DialogMarkdown
                  content={whatsNewQuery.data.changelog}
                  className="max-h-none overflow-visible"
                />
              ) : (
                <div className="text-sm text-muted-foreground">
                  {text.whatsNew.empty}
                </div>
              )}
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
        </div>
      </DialogContent>
    </Dialog>
  );
}
