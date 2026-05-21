import type { ReactNode } from "react";

import { cn } from "@/lib/utils";

type CenteredEmptyStateProps = {
  title: ReactNode;
  description?: ReactNode;
  className?: string;
};

export function CenteredEmptyState({ title, description, className }: CenteredEmptyStateProps) {
  return (
    <div className={cn("flex h-full min-h-0 items-center justify-center px-6", className)}>
      <div className="text-center">
        <p className="font-medium text-foreground">{title}</p>
        {description ? <p className="mt-1.5 text-xs text-muted-foreground">{description}</p> : null}
      </div>
    </div>
  );
}
