import type * as React from "react";

import { cn } from "@/lib/utils";

export function UIBlockFrame({ children, className }: { children?: React.ReactNode; className?: string }) {
  return (
    <div
      className={cn(
        "w-full min-w-0 overflow-hidden rounded-xl border-[0.5px] border-border bg-card text-card-foreground leading-normal",
        "[&_:is(a,button,input,summary):focus]:outline-none",
        "[&_:is(a,button,input,summary):focus-visible]:border-ring [&_:is(a,button,input,summary):focus-visible]:text-foreground",
        "[&_a:focus-visible]:bg-accent/45",
        className,
      )}
      data-ui-block=""
    >
      {children}
    </div>
  );
}
