"use client"

import * as React from "react"
import { Dialog as DialogPrimitive } from "radix-ui"

import { cn } from "@/lib/utils"

function Dialog({
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Root>) {
  return <DialogPrimitive.Root data-slot="dialog" {...props} />
}

function DialogTrigger({
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Trigger>) {
  return <DialogPrimitive.Trigger data-slot="dialog-trigger" {...props} />
}

function DialogPortal({
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Portal>) {
  return <DialogPrimitive.Portal data-slot="dialog-portal" {...props} />
}

function DialogClose({
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Close>) {
  return <DialogPrimitive.Close data-slot="dialog-close" {...props} />
}

function DialogOverlay({
  className,
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Overlay>) {
  return (
    <DialogPrimitive.Overlay
      data-slot="dialog-overlay"
      className={cn(
        "fixed inset-0 z-50 bg-black/50 data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:animate-in data-[state=open]:fade-in-0",
        className
      )}
      {...props}
    />
  )
}

const DialogContent = React.forwardRef<
  React.ElementRef<typeof DialogPrimitive.Content>,
  React.ComponentProps<typeof DialogPrimitive.Content>
>(function DialogContent(
  {
    className,
    children,
    onInteractOutside,
    ...props
  },
  ref
) {
  return (
    <DialogPortal data-slot="dialog-portal">
      <DialogOverlay />
      <DialogPrimitive.Content
        ref={ref}
        data-slot="dialog-content"
        className={cn(
          "fixed top-[50%] left-[50%] z-50 grid max-h-[calc(100svh-2rem)] w-full max-w-[calc(100%-2rem)] translate-x-[-50%] translate-y-[-50%] gap-4 overflow-y-auto rounded-lg border border-border/60 bg-background/96 p-5 shadow-xl outline-none backdrop-blur duration-200 data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=closed]:zoom-out-95 data-[state=open]:animate-in data-[state=open]:fade-in-0 data-[state=open]:zoom-in-95 sm:max-w-[560px]",
          className
        )}
        onInteractOutside={(event) => {
          const target = event.target as HTMLElement | null
          if (
            target?.closest('[data-slot="combobox-content"]') ||
            target?.closest('[data-slot="combobox-item"]') ||
            target?.closest('[data-slot="select-content"]') ||
            target?.closest('[data-slot="select-item"]') ||
            target?.closest('[data-slot="context-menu-content"]') ||
            target?.closest('[data-slot="context-menu-item"]')
          ) {
            event.preventDefault()
            return
          }

          onInteractOutside?.(event)
        }}
        {...props}
      >
        {children}
      </DialogPrimitive.Content>
    </DialogPortal>
  )
})

type DialogHeightTransitionProps = {
  children: React.ReactNode
  contentClassName?: string
}

function DialogHeightTransition({ children, contentClassName }: DialogHeightTransitionProps) {
  const contentRef = React.useRef<HTMLDivElement>(null)
  const [height, setHeight] = React.useState<number | null>(null)

  const measure = React.useCallback(() => {
    const nextHeight = contentRef.current?.offsetHeight
    if (!nextHeight) return
    setHeight((current) => current === nextHeight ? current : nextHeight)
  }, [])

  React.useLayoutEffect(() => {
    measure()
    if (typeof ResizeObserver === "undefined" || !contentRef.current) return
    const observer = new ResizeObserver(measure)
    observer.observe(contentRef.current)
    return () => observer.disconnect()
  }, [measure])

  return (
    <div
      className="relative min-h-0 overflow-hidden transition-[height] duration-200 ease-out motion-reduce:transition-none"
      style={height === null ? undefined : { height }}
    >
      <div
        ref={contentRef}
        className={cn("flex max-h-[min(82vh,560px)] min-h-0 flex-col overflow-hidden", contentClassName)}
      >
        {children}
      </div>
    </div>
  )
}

function DialogHeader({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="dialog-header"
      className={cn("flex flex-col gap-1.5 text-left", className)}
      {...props}
    />
  )
}

function DialogFooter({
  className,
  children,
  ...props
}: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="dialog-footer"
      className={cn(
        "flex flex-row justify-end gap-2 [&_[data-slot=button][data-size=default]]:h-7",
        className
      )}
      {...props}
    >
      {children}
    </div>
  )
}

function DialogTitle({
  className,
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Title>) {
  return (
    <DialogPrimitive.Title
      data-slot="dialog-title"
      className={cn("text-sm leading-5 font-semibold", className)}
      {...props}
    />
  )
}

function DialogDescription({
  className,
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Description>) {
  return (
    <DialogPrimitive.Description
      data-slot="dialog-description"
      className={cn("text-xs leading-5 text-muted-foreground", className)}
      {...props}
    />
  )
}

function DialogCollapsible({
  open,
  className,
  children,
  ...props
}: React.ComponentProps<"div"> & {
  open: boolean
}) {
  return (
    <div
      {...props}
      data-slot="dialog-collapsible"
      aria-hidden={!open}
      inert={!open}
      className={cn(
        "grid transition-[grid-template-rows,opacity] duration-200 ease-out",
        open ? "grid-rows-[1fr] opacity-100" : "grid-rows-[0fr] opacity-0",
        className
      )}
    >
      <div className="min-h-0 overflow-hidden">{children}</div>
    </div>
  )
}

export {
  Dialog,
  DialogCollapsible,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeightTransition,
  DialogHeader,
  DialogOverlay,
  DialogPortal,
  DialogTitle,
  DialogTrigger,
}
