"use client"

import * as React from "react"

import { cn } from "@/lib/utils"

type HeightTransitionProps = {
  children: React.ReactNode
  className?: string
  contentClassName?: string
}

// Nested transitions must not each ease the same change: the inner one would
// finish first and the outer would lag behind it, clipping the content.
// Only the outermost instance animates; inner ones render as plain boxes.
const NestedContext = React.createContext(false)

// Animates height changes of its content: the outer box tracks the measured
// height of the inner box so swapping children (tabs, filters, expand/collapse)
// slides instead of jumping. Height is unset until first measurement, so SSR
// and the first paint render at natural size. Children must not carry vertical
// margins: they collapse through the wrapper and make the measurement short.
// Put spacing on `className` / `contentClassName` (e.g. gap-*) instead.
function HeightTransition({ children, className, contentClassName }: HeightTransitionProps) {
  const nested = React.useContext(NestedContext)
  if (nested) {
    return (
      <div className={cn("min-h-0", className)}>
        <div className={contentClassName}>{children}</div>
      </div>
    )
  }
  return (
    <NestedContext.Provider value={true}>
      <AnimatedHeight className={className} contentClassName={contentClassName}>
        {children}
      </AnimatedHeight>
    </NestedContext.Provider>
  )
}

function AnimatedHeight({ children, className, contentClassName }: HeightTransitionProps) {
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
      className={cn("relative min-h-0 overflow-hidden transition-[height] duration-200 ease-out motion-reduce:transition-none", className)}
      style={height === null ? undefined : { height }}
    >
      <div ref={contentRef} className={contentClassName}>
        {children}
      </div>
    </div>
  )
}

export { HeightTransition }
