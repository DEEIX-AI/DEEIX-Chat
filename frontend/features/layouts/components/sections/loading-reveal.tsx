"use client"

import * as React from "react"

import { cn } from "@/lib/utils"

type LoadingRevealPhase = "loading" | "revealing" | "ready"

type LoadingRevealProps = {
  loading: boolean
  skeleton: React.ReactNode
  children: React.ReactNode
  className?: string
  skeletonClassName?: string
  contentClassName?: string
  durationMs?: number
}

export function LoadingReveal({
  loading,
  skeleton,
  children,
  className,
  skeletonClassName,
  contentClassName,
  durationMs = 220,
}: LoadingRevealProps) {
  const [phase, setPhase] = React.useState<LoadingRevealPhase>(() =>
    loading ? "loading" : "ready",
  )
  const [contentVisible, setContentVisible] = React.useState(() => !loading)

  React.useEffect(() => {
    if (loading) {
      setPhase("loading")
      setContentVisible(false)
      return
    }

    setPhase((current) => (current === "loading" ? "revealing" : "ready"))
  }, [loading])

  React.useEffect(() => {
    if (phase !== "revealing") {
      if (phase === "ready") {
        setContentVisible(true)
      }
      return
    }

    const frameID = window.requestAnimationFrame(() => {
      setContentVisible(true)
    })
    const timerID = window.setTimeout(() => {
      setPhase("ready")
    }, durationMs)

    return () => {
      window.cancelAnimationFrame(frameID)
      window.clearTimeout(timerID)
    }
  }, [durationMs, phase])

  const showSkeleton = phase !== "ready"
  const showContent = phase !== "loading"

  return (
    <div className={cn("relative min-h-0", className)}>
      {showContent ? (
        <div
          className={cn(
            "min-h-0 transition-opacity ease-[cubic-bezier(0.22,1,0.36,1)]",
            contentVisible ? "opacity-100" : "opacity-0",
            contentClassName,
          )}
          style={{ transitionDuration: `${durationMs}ms` }}
        >
          {children}
        </div>
      ) : null}

      {showSkeleton ? (
        <div
          aria-hidden="true"
          className={cn(
            "pointer-events-none transition-opacity ease-[cubic-bezier(0.22,1,0.36,1)]",
            showContent ? "absolute inset-0 z-10" : "relative",
            phase === "loading" ? "opacity-100" : "opacity-0",
            skeletonClassName,
          )}
          style={{ transitionDuration: `${durationMs}ms` }}
        >
          {skeleton}
        </div>
      ) : null}
    </div>
  )
}
