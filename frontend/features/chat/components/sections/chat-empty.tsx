"use client";

import * as React from "react";

type ChatEmptyStateProps = {
  greetingTitle: string;
  children?: React.ReactNode;
};

export function ChatEmptyState({ greetingTitle, children }: ChatEmptyStateProps) {
  return (
    <div className="flex h-full min-h-0 flex-col items-center justify-center px-3 py-12 text-center md:px-6 md:py-20">
      <h1 className="text-balance text-[22px] font-medium leading-[1.12] tracking-[-0.005em] text-foreground [font-family:var(--font-economist)] md:text-[32px]">
        {greetingTitle}
      </h1>
      {children ? <div className="mt-7 w-full max-w-[800px] md:mt-8">{children}</div> : null}
    </div>
  );
}
