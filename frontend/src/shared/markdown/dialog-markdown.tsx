import * as React from "react";
import ReactMarkdown, { type Components } from "react-markdown";
import remarkGfm from "remark-gfm";

import { cn } from "@/lib/utils";
import { normalizeMarkdown } from "@/shared/markdown/normalize";
import { isExternalHTTPURL, openExternalURL } from "@/shared/query/system";

const dialogMarkdownComponents: Components = {
  h1: ({ children }) => <h2>{children}</h2>,
  h2: ({ children }) => <h3>{children}</h3>,
  h3: ({ children }) => <h4>{children}</h4>,
  p: ({ children }) => <p>{children}</p>,
  ul: ({ children }) => (
    <ul className="app-dialog-markdown-list ml-4 space-y-1" data-kind="unordered">
      {children}
    </ul>
  ),
  ol: ({ children }) => (
    <ol className="app-dialog-markdown-list ml-4 space-y-1" data-kind="ordered">
      {children}
    </ol>
  ),
  li: ({ children }) => <li>{children}</li>,
  blockquote: ({ children }) => (
    <blockquote className="pl-3">{children}</blockquote>
  ),
  a: ({ href, children, ...props }) => {
    const externalURL = typeof href === "string" && isExternalHTTPURL(href) ? href.trim() : "";
    return (
      <a
        {...props}
        href={externalURL || undefined}
        aria-disabled={externalURL ? undefined : true}
        onClick={(event) => {
          event.preventDefault();
          if (!externalURL) {
            return;
          }
          void openExternalURL(externalURL).catch((error) => {
            console.warn("[DialogMarkdown] failed to open external link", error);
          });
        }}
      >
        {children}
      </a>
    );
  },
  code: ({ className, children, ...props }) => {
    const content = String(children ?? "").replace(/\n$/, "");
    if (!className) {
      return (
        <code className="app-dialog-markdown-code" data-block="false" {...props}>
          {content}
        </code>
      );
    }
    return (
      <code
        className="app-dialog-markdown-code block overflow-x-auto"
        data-block="true"
        {...props}
      >
        {content}
      </code>
    );
  },
  pre: ({ children }) => (
    <pre className="app-dialog-markdown-pre overflow-x-auto">{children}</pre>
  ),
  table: ({ children }) => (
    <div className="overflow-x-auto">
      <table className="app-dialog-markdown-table w-full">{children}</table>
    </div>
  ),
  thead: ({ children }) => <thead>{children}</thead>,
  th: ({ children }) => <th className="px-2 py-1">{children}</th>,
  td: ({ children }) => <td className="px-2 py-1 align-top">{children}</td>,
};

type DialogMarkdownProps = {
  content: string;
  className?: string;
};

export function DialogMarkdown({ content, className }: DialogMarkdownProps) {
  const normalizedContent = React.useMemo(() => normalizeMarkdown(content.trim()), [content]);

  return (
    <div className={cn("app-dialog-markdown max-h-80 overflow-auto space-y-2", className)}>
      <ReactMarkdown remarkPlugins={[remarkGfm]} components={dialogMarkdownComponents}>
        {normalizedContent}
      </ReactMarkdown>
    </div>
  );
}
