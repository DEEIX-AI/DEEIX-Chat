"use client";

import { Spinner } from "@/components/ui/spinner";
import { cn } from "@/lib/utils";

type PreviewLoadingProps = {
  className?: string;
};

export function PreviewLoading({ className }: PreviewLoadingProps) {
  return (
    <div
      className={cn(
        "flex min-h-0 items-center justify-center text-muted-foreground",
        className,
      )}
    >
      <Spinner className="size-4" />
    </div>
  );
}
