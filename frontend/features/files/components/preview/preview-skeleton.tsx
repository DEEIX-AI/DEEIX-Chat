"use client";

import { cn } from "@/lib/utils";
import { Skeleton } from "@/components/ui/skeleton";

type PreviewStageSkeletonProps = {
  className?: string;
};

const LINE_WIDTHS = [
  "28%",
  "92%",
  "88%",
  "90%",
  "84%",
  "87%",
  "79%",
  "91%",
  "83%",
  "89%",
  "77%",
  "90%",
  "82%",
  "88%",
  "75%",
  "87%",
  "80%",
  "69%",
];

export function PreviewStageSkeleton({
  className,
}: PreviewStageSkeletonProps) {
  return (
    <div className={cn("flex min-h-full w-full flex-col px-8 py-6 md:px-12 md:py-8", className)}>
      <div className="w-full max-w-[1460px] space-y-2.5">
        {LINE_WIDTHS.map((width, index) => (
          <Skeleton
            key={index}
            className="h-3.5 rounded-md bg-muted/50 md:h-4"
            style={{ width }}
          />
        ))}
      </div>
    </div>
  );
}
