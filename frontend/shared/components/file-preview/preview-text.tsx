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
    <div className="overflow-hidden rounded-[22px] border border-border/40 bg-background/80">
      <div>
        <div className="min-w-full px-4 py-4">
          <table className="w-full border-collapse text-[12.5px] leading-6">
            <tbody>
              {lines.map((line, index) => (
                <tr key={index} className="align-top">
                  <td className="w-10 select-none pr-4 text-right text-[11px] text-muted-foreground/75">
                    {index + 1}
                  </td>
                  <td className="whitespace-pre-wrap break-words font-mono text-foreground">
                    {line || " "}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
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

      {kind === "code" ? <CodeBlock content={content} /> : null}

      {kind === "text" ? (
        <div className="bg-background/80">
          <div className="px-5 py-5">
            <pre className="whitespace-pre-wrap break-words text-xs leading-6 text-foreground">
              {content}
            </pre>
          </div>
        </div>
      ) : null}
    </div>
  );
}
