import * as React from "react";
import { Browser } from "@wailsio/runtime";
import ReactMarkdown, { type Components } from "react-markdown";
import remarkGfm from "remark-gfm";

import { cn } from "@/lib/utils";
import { normalizeMarkdown } from "@/shared/markdown/normalize";

const dialogMarkdownComponents: Components = {
  h1: ({ children }) => <h2 className="text-base font-semibold text-foreground">{children}</h2>,
  h2: ({ children }) => <h3 className="text-sm font-semibold text-foreground">{children}</h3>,
  h3: ({ children }) => <h4 className="text-sm font-medium text-foreground">{children}</h4>,
  p: ({ children }) => <p className="text-sm leading-relaxed text-foreground">{children}</p>,
  ul: ({ children }) => <ul className="ml-4 list-disc space-y-1 text-sm text-foreground">{children}</ul>,
  ol: ({ children }) => <ol className="ml-4 list-decimal space-y-1 text-sm text-foreground">{children}</ol>,
  li: ({ children }) => <li className="leading-relaxed">{children}</li>,
  blockquote: ({ children }) => (
    <blockquote className="border-l-2 border-border pl-3 text-muted-foreground">{children}</blockquote>
  ),
  a: ({ href, children, ...props }) => (
    <a
      href={href}
      className="text-primary underline underline-offset-4"
      onClick={(event) => {
        if (!href) {
          return;
        }
        event.preventDefault();
        Browser.OpenURL(href);
      }}
      {...props}
    >
      {children}
    </a>
  ),
  code: ({ className, children, ...props }) => {
    const content = String(children ?? "").replace(/\n$/, "");
    if (!className) {
      return (
        <code className="rounded bg-muted px-1 py-0.5 font-mono text-[0.85em]" {...props}>
          {content}
        </code>
      );
    }
    return (
      <code className="block overflow-x-auto rounded bg-muted p-2 font-mono text-[0.85em]" {...props}>
        {content}
      </code>
    );
  },
  pre: ({ children }) => <pre className="overflow-x-auto rounded bg-muted p-2 text-xs">{children}</pre>,
  table: ({ children }) => (
    <div className="overflow-x-auto">
      <table className="w-full border-collapse text-left text-sm text-foreground">{children}</table>
    </div>
  ),
  thead: ({ children }) => <thead className="border-b border-border">{children}</thead>,
  th: ({ children }) => <th className="px-2 py-1 font-medium">{children}</th>,
  td: ({ children }) => <td className="px-2 py-1 align-top">{children}</td>,
};

type DialogMarkdownProps = {
  content: string;
  className?: string;
};

export function DialogMarkdown({ content, className }: DialogMarkdownProps) {
  const normalizedContent = React.useMemo(() => normalizeMarkdown(content.trim()), [content]);

  return (
    <div className={cn("max-h-80 overflow-auto space-y-2 text-sm text-foreground", className)}>
      <ReactMarkdown remarkPlugins={[remarkGfm]} components={dialogMarkdownComponents}>
        {normalizedContent}
      </ReactMarkdown>
    </div>
  );
}
