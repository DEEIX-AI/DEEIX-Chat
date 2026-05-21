import { Loader2Icon } from "lucide-react"

import { cn } from "@/lib/utils"
import * as React from "react"

function Spinner({ className, label, ...props }: React.ComponentProps<"svg"> & { label?: string }) {
  const accessibleLabel = label ?? props["aria-label"]

  return (
    <Loader2Icon
      role={accessibleLabel ? "status" : undefined}
      aria-label={accessibleLabel}
      aria-hidden={accessibleLabel ? undefined : true}
      className={cn("size-4 animate-spin", className)}
      {...props}
    />
  )
}

function SpinnerLabel({
  className,
  spinnerClassName,
  children,
  ...props
}: React.ComponentProps<"span"> & {
  spinnerClassName?: string
}) {
  return (
    <span
      className={cn("inline-flex items-center gap-1.5 whitespace-nowrap", className)}
      {...props}
    >
      <Spinner className={cn("size-3.5", spinnerClassName)} />
      <span>{children}</span>
    </span>
  )
}

export { Spinner, SpinnerLabel }
