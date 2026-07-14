"use client";

import * as React from "react";

import { StreamdownRender } from "@/shared/components/markdown/streamdown-render";

type PreviewTextProps = {
  kind: "markdown" | "code" | "text";
  content: string;
  className?: string;
};

function CodeBlock({ content }: { content: string }) {
  const lines = React.useMemo(() => content.split("\n"), [content]);

  return (
    <div className="overflow-hidden rounded-md border border-border/40 bg-background/80">
      <div className="min-w-full px-4 py-4 font-mono text-[12.5px] leading-6">
        {lines.map((line, index) => (
          <div key={index} className="flex min-w-0 items-start">
            <span className="w-10 shrink-0 select-none pr-4 text-right text-[11px] text-muted-foreground/75">
              {index + 1}
            </span>
            <span className="min-w-0 flex-1 whitespace-pre-wrap break-words text-foreground">
              {line || " "}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}

export function PreviewText({
  kind,
  content,
  className,
}: PreviewTextProps) {
  return (
    <div className={className ?? "min-h-[320px]"}>
      {kind === "markdown" ? (
        <div className="bg-background/80">
          <div className="px-5 py-5">
            <StreamdownRender content={content} className="text-sm" />
          </div>
        </div>
      ) : null}

      {kind === "code" || kind === "text" ? <CodeBlock content={content} /> : null}
    </div>
  );
}
