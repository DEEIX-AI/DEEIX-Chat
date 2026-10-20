"use client"

import { ChevronDownIcon } from "lucide-react"
import { Accordion as AccordionPrimitive } from "radix-ui"
import * as React from "react"

import { cn } from "@/lib/utils"

function Accordion({
  ...props
}: React.ComponentProps<typeof AccordionPrimitive.Root>) {
  return <AccordionPrimitive.Root data-slot="accordion" {...props} />
}

function AccordionItem({
  className,
  ...props
}: React.ComponentProps<typeof AccordionPrimitive.Item>) {
  return (
    <AccordionPrimitive.Item
      data-slot="accordion-item"
      className={cn("border-b last:border-b-0", className)}
      {...props}
    />
  )
}

function AccordionTrigger({
  className,
  children,
  iconPosition = "right",
  ...props
}: React.ComponentProps<typeof AccordionPrimitive.Trigger> & {
  iconPosition?: "left" | "right" | "adjacent" | "none"
}) {
  return (
    <AccordionPrimitive.Header className="flex">
      <AccordionPrimitive.Trigger
        data-slot="accordion-trigger"
        className={cn(
          "flex flex-1 items-start gap-4 rounded-md py-4 text-left text-sm font-medium transition-all outline-none hover:underline focus-visible:underline focus-visible:ring-0! disabled:pointer-events-none disabled:opacity-50 [&[data-state=open]_.accordion-trigger-icon]:rotate-180",
          iconPosition === "right" ? "justify-between" : "justify-start",
          className
        )}
        {...props}
      >
        {iconPosition === "left" ? (
          <ChevronDownIcon className="accordion-trigger-icon pointer-events-none size-4 shrink-0 translate-y-0.5 text-muted-foreground transition-transform duration-200" />
        ) : null}
        {iconPosition === "adjacent" ? (
          <span className="inline-flex min-w-0 items-center gap-1.5">
            {children}
            <ChevronDownIcon className="accordion-trigger-icon pointer-events-none size-4 shrink-0 translate-y-0.5 text-muted-foreground transition-transform duration-200" />
          </span>
        ) : (
          <>
            {children}
            {iconPosition === "right" ? (
              <ChevronDownIcon className="accordion-trigger-icon pointer-events-none size-4 shrink-0 translate-y-0.5 text-muted-foreground transition-transform duration-200" />
            ) : null}
          </>
        )}
      </AccordionPrimitive.Trigger>
    </AccordionPrimitive.Header>
  )
}

// The open and close animations run to this height at most. A panel taller than the
// viewport otherwise spends most of the animation below the fold, so the part in view
// rushes past in a few frames; capped, it moves at a readable pace and the jump to its full
// height when the animation ends happens out of sight.
const PANEL_ANIMATION_HEIGHT_STYLE = {
  "--radix-accordion-content-height": "min(var(--radix-collapsible-content-height), 100dvh)",
} as React.CSSProperties

type AccordionContentProps = React.ComponentProps<typeof AccordionPrimitive.Content> & {
  /** Classes for the animated panel itself, e.g. its duration and easing; `className` styles the inner padding box. */
  panelClassName?: string
}

function AccordionContent({
  className,
  panelClassName,
  style,
  children,
  ...props
}: AccordionContentProps) {
  return (
    <AccordionPrimitive.Content
      data-slot="accordion-content"
      className={cn(
        "overflow-hidden text-sm data-[state=closed]:animate-accordion-up data-[state=open]:animate-accordion-down",
        panelClassName,
      )}
      style={{ ...PANEL_ANIMATION_HEIGHT_STYLE, ...style }}
      {...props}
    >
      <div className={cn("pt-0 pb-4 px-0.5", className)}>{children}</div>
    </AccordionPrimitive.Content>
  )
}

// How long the pointer has to rest on a collapsed item before its panel is built ahead of
// the click, so sweeping across a long history does not build every trace it passes.
const PREWARM_HOVER_DELAY_MS = 80

// Keeps a panel's children mounted after they have been built once. Radix drops them on
// close and rebuilds them on the next open, which blocks the main thread for hundreds of
// milliseconds when the panel holds a long trace, exactly when the animation starts, and
// the collapse throws that work away. The first build is moved off the click as well: once
// the pointer rests on the item, or it takes focus or a press, the children are rendered
// into a hidden Activity at idle priority, so the open finds them ready. Nothing is built
// before that, so a collapsed panel in a long history costs a single empty element.
function AccordionContentKeepMounted({
  open,
  className,
  panelClassName,
  style,
  children,
  onAnimationEnd,
  onPrewarm,
  ref,
  ...props
}: AccordionContentProps & {
  /** The same open state the item was given; it must match the Accordion's value. */
  open: boolean
  /** Called once when the panel is built ahead of its first open, to start loading what it shows. */
  onPrewarm?: () => void
}) {
  // Derived during render so the children exist, and are shown, in the commit that opens
  // the panel: Radix measures the panel height right after it to animate from zero.
  const [built, setBuilt] = React.useState(open)
  const [revealed, setRevealed] = React.useState(open)
  const [collapsed, setCollapsed] = React.useState(!open)
  if (open && !built) {
    setBuilt(true)
  }
  if (open && !revealed) {
    setRevealed(true)
  }
  if (open && collapsed) {
    setCollapsed(false)
  }

  // Radix measures the panel in a layout effect when `open` changes and only writes the height
  // into --radix-collapsible-content-height on the render that follows, which its own presence
  // update provides. Force-mounted, that update never changes anything, so the height would
  // stay unset on the first open (the animation then runs to `auto` and jumps) and stale
  // after the content changes. This re-render, before paint, takes its place.
  const [, rerender] = React.useReducer((count: number) => count + 1, 0)
  const openRef = React.useRef(open)
  React.useLayoutEffect(() => {
    if (openRef.current !== open) {
      openRef.current = open
      rerender()
    }
  }, [open])

  const contentRef = React.useRef<HTMLDivElement | null>(null)
  const setContentRef = React.useCallback(
    (node: HTMLDivElement | null) => {
      contentRef.current = node
      if (typeof ref === "function") {
        ref(node)
      } else if (ref) {
        ref.current = node
      }
    },
    [ref],
  )

  const onPrewarmRef = React.useRef(onPrewarm)
  React.useEffect(() => {
    onPrewarmRef.current = onPrewarm
  })

  React.useEffect(() => {
    const item = contentRef.current?.parentElement
    if (built || !item) {
      return
    }
    let timer: ReturnType<typeof setTimeout> | undefined
    const prewarm = () => {
      clearTimeout(timer)
      onPrewarmRef.current?.()
      React.startTransition(() => setBuilt(true))
    }
    const schedule = () => {
      clearTimeout(timer)
      timer = setTimeout(prewarm, PREWARM_HOVER_DELAY_MS)
    }
    const cancel = () => clearTimeout(timer)
    item.addEventListener("pointerenter", schedule)
    item.addEventListener("pointerleave", cancel)
    item.addEventListener("pointerdown", prewarm)
    item.addEventListener("focusin", prewarm)
    return () => {
      clearTimeout(timer)
      item.removeEventListener("pointerenter", schedule)
      item.removeEventListener("pointerleave", cancel)
      item.removeEventListener("pointerdown", prewarm)
      item.removeEventListener("focusin", prewarm)
    }
  }, [built])

  // Always force-mounted, so Radix never hides the panel and it is hidden here instead, but
  // only once the closing animation has ended (the closed state holds height 0 until then
  // through `forwards`). Reduced motion shortens the animation rather than removing it, so
  // the event still fires.
  const handleAnimationEnd = (event: React.AnimationEvent<HTMLDivElement>) => {
    onAnimationEnd?.(event)
    if (event.target === event.currentTarget && event.currentTarget.dataset.state === "closed") {
      setCollapsed(true)
    }
  }

  return (
    <AccordionPrimitive.Content
      ref={setContentRef}
      forceMount
      data-slot="accordion-content"
      hidden={collapsed || undefined}
      inert={!open}
      className={cn(
        "overflow-hidden text-sm data-[state=closed]:animate-accordion-up data-[state=open]:animate-accordion-down data-[state=closed]:[animation-fill-mode:forwards]",
        panelClassName,
      )}
      style={{ ...PANEL_ANIMATION_HEIGHT_STYLE, ...style }}
      onAnimationEnd={handleAnimationEnd}
      {...props}
    >
      <div className={cn("pt-0 pb-4 px-0.5", className)}>
        {/* Hidden only until the first open, so the prewarmed tree renders at idle priority.
            After that it stays visible to React and is hidden by the panel alone: hiding the
            Activity again would tear down every effect inside and replay them all on the
            next open, right as the animation starts. */}
        {built ? <React.Activity mode={revealed ? "visible" : "hidden"}>{children}</React.Activity> : null}
      </div>
    </AccordionPrimitive.Content>
  )
}

export {
  Accordion,
  AccordionContent,
  AccordionContentKeepMounted,
  AccordionItem,
  AccordionTrigger,
}
