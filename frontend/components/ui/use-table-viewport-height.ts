"use client"

import * as React from "react"

function setRefValue<T>(ref: React.Ref<T> | undefined, value: T | null) {
  if (typeof ref === "function") {
    ref(value)
    return
  }

  if (ref) {
    const mutableRef = ref as React.MutableRefObject<T | null>
    mutableRef.current = value
  }
}

export function useTableViewportHeight({
  disabled,
  externalRef,
}: {
  disabled: boolean
  externalRef?: React.Ref<HTMLDivElement>
}) {
  const viewportElementRef = React.useRef<HTMLDivElement | null>(null)
  const contentElementRef = React.useRef<HTMLDivElement | null>(null)
  const frameRef = React.useRef<number | null>(null)
  const [height, setHeight] = React.useState<number | null>(null)

  const viewportRef = React.useCallback(
    (node: HTMLDivElement | null) => {
      viewportElementRef.current = node
      setRefValue(externalRef, node)
    },
    [externalRef]
  )

  const measure = React.useCallback(() => {
    const viewportElement = viewportElementRef.current
    const contentElement = contentElementRef.current

    if (!viewportElement || !contentElement || disabled) {
      return
    }

    const maxHeight = Number.parseFloat(getComputedStyle(viewportElement).maxHeight)
    const contentHeight = contentElement.scrollHeight
    // Reserve non-overlay horizontal scrollbar space to avoid false vertical overflow.
    const horizontalScrollbarHeight = viewportElement.scrollWidth > viewportElement.clientWidth
      ? Math.max(0, viewportElement.offsetHeight - viewportElement.clientHeight)
      : 0
    const requiredHeight = contentHeight + horizontalScrollbarHeight
    const nextHeight = Math.ceil(
      Number.isFinite(maxHeight)
        ? Math.min(requiredHeight, maxHeight)
        : requiredHeight
    )

    setHeight((currentHeight) => currentHeight === nextHeight ? currentHeight : nextHeight)
  }, [disabled])

  const scheduleMeasure = React.useCallback(() => {
    if (frameRef.current !== null) {
      cancelAnimationFrame(frameRef.current)
    }

    frameRef.current = requestAnimationFrame(() => {
      frameRef.current = null
      measure()
    })
  }, [measure])

  React.useLayoutEffect(() => {
    if (disabled) {
      setHeight(null)
      return
    }

    const contentElement = contentElementRef.current

    if (!contentElement) {
      return
    }

    measure()

    const resizeObserver = new ResizeObserver(scheduleMeasure)
    resizeObserver.observe(contentElement)
    window.addEventListener("resize", scheduleMeasure)

    return () => {
      resizeObserver.disconnect()
      window.removeEventListener("resize", scheduleMeasure)

      if (frameRef.current !== null) {
        cancelAnimationFrame(frameRef.current)
        frameRef.current = null
      }
    }
  }, [disabled, measure, scheduleMeasure])

  return {
    contentRef: contentElementRef,
    heightStyle: height === null ? undefined : { height },
    viewportRef,
  }
}
