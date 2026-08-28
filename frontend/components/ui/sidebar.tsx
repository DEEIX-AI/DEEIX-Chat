"use client"

import { cva, type VariantProps } from "class-variance-authority"
import { PanelLeftIcon } from "lucide-react"
import { useTranslations } from "next-intl"
import { Slot } from "radix-ui"
import * as React from "react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Separator } from "@/components/ui/separator"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import { Skeleton } from "@/components/ui/skeleton"
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import { cn } from "@/lib/utils"
import { useIsMobile } from "@/shared/hooks/use-mobile"
import {
  createSidebarRevealTransitionState,
  sidebarRevealTransitionReducer,
} from "./sidebar-hover-transition"

const SIDEBAR_STORAGE_KEY = "deeix.sidebar.open"

type SidebarState = "expanded" | "collapsed"

type SidebarActionsContextProps = {
  setOpen: (open: boolean | ((open: boolean) => boolean)) => void
  setOpenMobile: React.Dispatch<React.SetStateAction<boolean>>
  toggleSidebar: () => void
}

type SidebarHoverExpansionContextProps = {
  acquireLock: () => () => void
}

const SidebarOpenContext = React.createContext<boolean | null>(null)
const SidebarMobileOpenContext = React.createContext<boolean | null>(null)
const SidebarIsMobileContext = React.createContext<boolean | null>(null)
const SidebarActionsContext = React.createContext<SidebarActionsContextProps | null>(null)
const SidebarVisualStateContext = React.createContext<SidebarState | null>(null)
const SidebarHoverExpansionContext = React.createContext<SidebarHoverExpansionContextProps | null>(null)

function useRequiredSidebarContext<T>(context: React.Context<T | null>, hookName: string) {
  const value = React.useContext(context)
  if (value === null) {
    throw new Error(`${hookName} must be used within a SidebarProvider.`)
  }

  return value
}

function useSidebarOpen() {
  return useRequiredSidebarContext(SidebarOpenContext, "useSidebarOpen")
}

function useSidebarMobileOpen() {
  return useRequiredSidebarContext(SidebarMobileOpenContext, "useSidebarMobileOpen")
}

function useSidebarIsMobile() {
  return useRequiredSidebarContext(SidebarIsMobileContext, "useSidebarIsMobile")
}

function useSidebarActions() {
  return useRequiredSidebarContext(SidebarActionsContext, "useSidebarActions")
}

function useSidebarVisualState() {
  const state = React.useContext(SidebarVisualStateContext)
  if (!state) {
    throw new Error("useSidebarVisualState must be used within a Sidebar.")
  }

  return state
}

function useSidebarHoverExpansionLock(active: boolean) {
  const context = React.useContext(SidebarHoverExpansionContext)

  React.useLayoutEffect(() => {
    if (!active || !context) {
      return
    }
    return context.acquireLock()
  }, [active, context])
}

function shouldAutoCollapseSidebar() {
  return typeof window !== "undefined" && window.innerWidth < 1180
}

function shouldAutoRestoreSidebar() {
  return typeof window !== "undefined" && window.innerWidth >= 1360
}

function shouldReduceSidebarMotion() {
  return (
    typeof window !== "undefined" &&
    window.matchMedia("(prefers-reduced-motion: reduce)").matches
  )
}

function readSidebarInitialOpen(defaultOpen: boolean) {
  if (!defaultOpen) {
    return false
  }
  if (typeof window === "undefined") {
    return defaultOpen
  }

  try {
    const stored = window.localStorage.getItem(SIDEBAR_STORAGE_KEY)
    if (stored === "true") {
      return true
    }
    if (stored === "false") {
      return false
    }
  } catch {
    // Keep the default state when localStorage is unavailable.
  }

  return defaultOpen
}

function useSidebarAutoViewport() {
  const [autoViewport, setAutoViewport] = React.useState(() => ({
    shouldCollapse: shouldAutoCollapseSidebar(),
    shouldRestore: shouldAutoRestoreSidebar(),
  }))

  React.useEffect(() => {
    const collapseQuery = window.matchMedia("(max-width: 1179px)")
    const restoreQuery = window.matchMedia("(min-width: 1360px)")
    const sync = () => {
      setAutoViewport({
        shouldCollapse: collapseQuery.matches,
        shouldRestore: restoreQuery.matches,
      })
    }

    sync()
    collapseQuery.addEventListener("change", sync)
    restoreQuery.addEventListener("change", sync)
    return () => {
      collapseQuery.removeEventListener("change", sync)
      restoreQuery.removeEventListener("change", sync)
    }
  }, [])

  return autoViewport
}

function SidebarProvider({
  defaultOpen = true,
  open: openProp,
  onOpenChange: setOpenProp,
  className,
  style,
  children,
  ...props
}: React.ComponentProps<"div"> & {
  defaultOpen?: boolean
  open?: boolean
  onOpenChange?: (open: boolean) => void
}) {
  const isMobile = useIsMobile()
  const { shouldCollapse, shouldRestore } = useSidebarAutoViewport()
  const [openMobile, setOpenMobile] = React.useState(false)
  const autoCollapsedRef = React.useRef(readSidebarInitialOpen(defaultOpen) && shouldAutoCollapseSidebar())
  const wasAutoCollapseViewportRef = React.useRef(shouldAutoCollapseSidebar())
  const wasAutoRestoreViewportRef = React.useRef(shouldAutoRestoreSidebar())

  // This is the internal state of the sidebar.
  // We use openProp and setOpenProp for control from outside the component.
  const [_open, _setOpen] = React.useState(() => readSidebarInitialOpen(defaultOpen) && !shouldAutoCollapseSidebar())
  const open = openProp ?? _open
  const openRef = React.useRef(open)
  const isMobileRef = React.useRef(isMobile)
  React.useLayoutEffect(() => {
    openRef.current = open
    isMobileRef.current = isMobile
  }, [isMobile, open])
  const setOpen = React.useCallback(
    (value: boolean | ((value: boolean) => boolean)) => {
      const openState = typeof value === "function" ? value(openRef.current) : value
      openRef.current = openState
      autoCollapsedRef.current = false
      if (setOpenProp) {
        setOpenProp(openState)
      } else {
        _setOpen(openState)
      }

      // This sets the cookie to keep the sidebar state.
      document.cookie = `sidebar_state=${openState}; path=/; max-age=${60 * 60 * 24 * 7}`
      try {
        window.localStorage.setItem(SIDEBAR_STORAGE_KEY, openState ? "true" : "false")
      } catch {
        // Ignore storage failures; the current in-memory state still controls the UI.
      }
    },
    [setOpenProp]
  )

  React.useEffect(() => {
    const enteredAutoCollapseViewport = shouldCollapse && !wasAutoCollapseViewportRef.current
    const enteredAutoRestoreViewport = shouldRestore && !wasAutoRestoreViewportRef.current
    wasAutoCollapseViewportRef.current = shouldCollapse
    wasAutoRestoreViewportRef.current = shouldRestore

    if (enteredAutoCollapseViewport) {
      autoCollapsedRef.current = open
      if (!open) {
        return
      }

      openRef.current = false
      if (setOpenProp) {
        setOpenProp(false)
        return
      }

      _setOpen(false)
      return
    }

    if (!enteredAutoRestoreViewport || !autoCollapsedRef.current) {
      return
    }

    autoCollapsedRef.current = false
    openRef.current = true
    if (setOpenProp) {
      setOpenProp(true)
      return
    }

    _setOpen(true)
  }, [open, setOpenProp, shouldCollapse, shouldRestore])

  // Helper to toggle the sidebar.
  const toggleSidebar = React.useCallback(() => {
    return isMobileRef.current
      ? setOpenMobile((current) => !current)
      : setOpen((current) => !current)
  }, [setOpen])

  // Adds a keyboard shortcut to toggle the sidebar.
  React.useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (
        event.key === "b" &&
        (event.metaKey || event.ctrlKey)
      ) {
        event.preventDefault()
        toggleSidebar()
      }
    }

    window.addEventListener("keydown", handleKeyDown)
    return () => window.removeEventListener("keydown", handleKeyDown)
  }, [toggleSidebar])

  const actionsContextValue = React.useMemo<SidebarActionsContextProps>(
    () => ({
      setOpen,
      setOpenMobile,
      toggleSidebar,
    }),
    [setOpen, toggleSidebar]
  )

  return (
    <SidebarActionsContext.Provider value={actionsContextValue}>
      <SidebarIsMobileContext.Provider value={isMobile}>
        <SidebarOpenContext.Provider value={open}>
          <SidebarMobileOpenContext.Provider value={openMobile}>
            <TooltipProvider delayDuration={0}>
              <div
                data-slot="sidebar-wrapper"
                style={
                  {
                    "--sidebar-width": "17.96875rem",
                    "--sidebar-width-icon": "3rem",
                    ...style,
                  } as React.CSSProperties
                }
                className={cn(
                  "group/sidebar-wrapper flex min-h-svh w-full has-data-[variant=inset]:bg-sidebar",
                  className
                )}
                {...props}
              >
                {children}
              </div>
            </TooltipProvider>
          </SidebarMobileOpenContext.Provider>
        </SidebarOpenContext.Provider>
      </SidebarIsMobileContext.Provider>
    </SidebarActionsContext.Provider>
  )
}

function Sidebar({
  side = "left",
  variant = "sidebar",
  collapsible = "offcanvas",
  expandOnHover = false,
  className,
  children,
  onPointerEnter,
  onPointerLeave,
  onTransitionEnd,
  ...props
}: React.ComponentProps<"div"> & {
  side?: "left" | "right"
  variant?: "sidebar" | "floating" | "inset"
  collapsible?: "offcanvas" | "icon" | "none"
  expandOnHover?: boolean
}) {
  const open = useSidebarOpen()
  const openMobile = useSidebarMobileOpen()
  const isMobile = useSidebarIsMobile()
  const { setOpenMobile } = useSidebarActions()
  const [hoverExpanded, setHoverExpanded] = React.useState(false)
  const hoverCollapseTimerRef = React.useRef<number | null>(null)
  const hoverExpansionLockCountRef = React.useRef(0)
  const pointerInsideRef = React.useRef(false)
  const visualState = open || hoverExpanded ? "expanded" : "collapsed"
  const usesHoverReveal = expandOnHover && collapsible === "icon" && variant === "sidebar"
  // Keep the stable child layout separate from the animated shell target.
  const [revealTransition, dispatchRevealTransition] = React.useReducer(
    sidebarRevealTransitionReducer,
    visualState,
    createSidebarRevealTransitionState
  )
  const layoutState = revealTransition.layout
  const isResizing = revealTransition.resizing
  const transitionTargetRef = React.useRef<SidebarState>(visualState)
  const transitionTimerRef = React.useRef<number | null>(null)
  const t = useTranslations("common.navigation")

  const clearHoverCollapseTimer = React.useCallback(() => {
    if (hoverCollapseTimerRef.current === null) {
      return
    }
    window.clearTimeout(hoverCollapseTimerRef.current)
    hoverCollapseTimerRef.current = null
  }, [])

  const clearTransitionTimer = React.useCallback(() => {
    if (transitionTimerRef.current === null) {
      return
    }
    window.clearTimeout(transitionTimerRef.current)
    transitionTimerRef.current = null
  }, [])

  const settleTransition = React.useCallback(
    (state: SidebarState) => {
      clearTransitionTimer()
      dispatchRevealTransition({ type: "settle", target: state })
    },
    [clearTransitionTimer]
  )

  const syncTransition = React.useCallback(
    (state: SidebarState) => {
      clearTransitionTimer()
      dispatchRevealTransition({ type: "sync", target: state })
    },
    [clearTransitionTimer]
  )

  const scheduleHoverCollapse = React.useCallback(() => {
    clearHoverCollapseTimer()
    hoverCollapseTimerRef.current = window.setTimeout(() => {
      hoverCollapseTimerRef.current = null
      setHoverExpanded(false)
    }, 80)
  }, [clearHoverCollapseTimer])

  const acquireHoverExpansionLock = React.useCallback(() => {
    let released = false
    hoverExpansionLockCountRef.current += 1
    clearHoverCollapseTimer()

    return () => {
      if (released) {
        return
      }
      released = true
      hoverExpansionLockCountRef.current = Math.max(0, hoverExpansionLockCountRef.current - 1)
      if (hoverExpansionLockCountRef.current === 0 && !pointerInsideRef.current) {
        scheduleHoverCollapse()
      }
    }
  }, [clearHoverCollapseTimer, scheduleHoverCollapse])

  const hoverExpansionContextValue = React.useMemo<SidebarHoverExpansionContextProps>(
    () => ({ acquireLock: acquireHoverExpansionLock }),
    [acquireHoverExpansionLock]
  )

  React.useEffect(() => {
    if (isMobile || open || !expandOnHover || collapsible !== "icon") {
      clearHoverCollapseTimer()
      setHoverExpanded(false)
    }
  }, [clearHoverCollapseTimer, collapsible, expandOnHover, isMobile, open])

  React.useLayoutEffect(() => {
    clearTransitionTimer()

    if (!usesHoverReveal || shouldReduceSidebarMotion()) {
      transitionTargetRef.current = visualState
      syncTransition(visualState)
      return
    }

    if (transitionTargetRef.current === visualState) {
      return
    }

    transitionTargetRef.current = visualState
    dispatchRevealTransition({ type: "begin", target: visualState })

    transitionTimerRef.current = window.setTimeout(() => {
      settleTransition(visualState)
    }, 200 + 50)
  }, [clearTransitionTimer, settleTransition, syncTransition, usesHoverReveal, visualState])

  React.useEffect(
    () => () => {
      clearHoverCollapseTimer()
      clearTransitionTimer()
    },
    [clearHoverCollapseTimer, clearTransitionTimer]
  )

  const handlePointerEnter = React.useCallback(
    (event: React.PointerEvent<HTMLDivElement>) => {
      onPointerEnter?.(event)
      pointerInsideRef.current = true
      clearHoverCollapseTimer()
      if (
        expandOnHover &&
        collapsible === "icon" &&
        event.pointerType === "mouse" &&
        !open
      ) {
        setHoverExpanded(true)
      }
    },
    [clearHoverCollapseTimer, collapsible, expandOnHover, onPointerEnter, open]
  )

  const handlePointerLeave = React.useCallback(
    (event: React.PointerEvent<HTMLDivElement>) => {
      onPointerLeave?.(event)
      pointerInsideRef.current = false
      if (expandOnHover && collapsible === "icon" && event.pointerType === "mouse") {
        if (hoverExpansionLockCountRef.current > 0) {
          return
        }
        scheduleHoverCollapse()
      }
    },
    [collapsible, expandOnHover, onPointerLeave, scheduleHoverCollapse]
  )

  const handleContainerTransitionEnd = React.useCallback(
    (event: React.TransitionEvent<HTMLDivElement>) => {
      onTransitionEnd?.(event)
      if (
        !usesHoverReveal ||
        event.target !== event.currentTarget ||
        event.propertyName !== "width"
      ) {
        return
      }

      settleTransition(transitionTargetRef.current)
    },
    [onTransitionEnd, settleTransition, usesHoverReveal]
  )

  if (collapsible === "none") {
    return (
      <SidebarVisualStateContext.Provider value="expanded">
        <div
          data-slot="sidebar"
          className={cn(
            "flex h-full w-(--sidebar-width) flex-col bg-sidebar text-sidebar-foreground",
            className
          )}
          onPointerEnter={onPointerEnter}
          onPointerLeave={onPointerLeave}
          {...props}
        >
          {children}
        </div>
      </SidebarVisualStateContext.Provider>
    )
  }

  if (isMobile) {
    return (
      <SidebarVisualStateContext.Provider value="expanded">
        <Sheet open={openMobile} onOpenChange={setOpenMobile} {...props}>
          <SheetContent
            data-sidebar="sidebar"
            data-slot="sidebar"
            data-mobile="true"
            className="w-(--sidebar-width) bg-sidebar p-0 text-sidebar-foreground [&>button]:hidden"
            style={
              {
                "--sidebar-width": "17.96875rem",
              } as React.CSSProperties
            }
            side={side}
            onPointerEnter={onPointerEnter}
            onPointerLeave={onPointerLeave}
          >
            <SheetHeader className="sr-only">
              <SheetTitle>{t("sidebarTitle")}</SheetTitle>
              <SheetDescription>{t("mobileSidebarDescription")}</SheetDescription>
            </SheetHeader>
            <div className="flex h-full w-full flex-col">{children}</div>
          </SheetContent>
        </Sheet>
      </SidebarVisualStateContext.Provider>
    )
  }

  return (
    <div
      className="group peer hidden text-sidebar-foreground md:block"
      data-state={visualState}
      data-resizing={isResizing ? "true" : "false"}
      data-hover-expanded={hoverExpanded && !open ? "true" : "false"}
      data-collapsible={layoutState === "collapsed" ? collapsible : ""}
      data-variant={variant}
      data-side={side}
      data-slot="sidebar"
    >
      {/* This is what handles the sidebar gap on desktop */}
      <div
        data-slot="sidebar-gap"
        className={cn(
          "relative w-(--sidebar-width) bg-transparent transition-[width] duration-200 ease-linear",
          "group-data-[collapsible=offcanvas]:w-0",
          "group-data-[side=right]:rotate-180",
          collapsible === "icon" && !open &&
            (variant === "floating" || variant === "inset"
              ? "w-[calc(var(--sidebar-width-icon)+(--spacing(4)))]"
              : "w-(--sidebar-width-icon)")
        )}
      />
      <div
        data-slot="sidebar-container"
        className={cn(
          "fixed inset-y-0 z-10 hidden h-svh w-(--sidebar-width) md:flex",
          usesHoverReveal
            ? "overflow-hidden transition-[width,box-shadow] duration-200 ease-linear motion-reduce:transition-none group-data-[state=collapsed]:w-(--sidebar-width-icon)"
            : "transition-[left,right,width] duration-200 ease-linear",
          side === "left"
            ? "left-0 group-data-[collapsible=offcanvas]:left-[calc(var(--sidebar-width)*-1)]"
            : "right-0 group-data-[collapsible=offcanvas]:right-[calc(var(--sidebar-width)*-1)]",
          // Adjust the padding for floating and inset variants.
          variant === "floating" || variant === "inset"
            ? "p-2 group-data-[collapsible=icon]:w-[calc(var(--sidebar-width-icon)+(--spacing(4))+2px)]"
            : cn(
                "group-data-[side=left]:border-r-[0.5px] group-data-[side=right]:border-l-[0.5px]",
                !usesHoverReveal && "group-data-[collapsible=icon]:w-(--sidebar-width-icon)"
              ),
          "group-data-[hover-expanded=true]:z-40 group-data-[resizing=true]:z-40",
          side === "left"
            ? "group-data-[hover-expanded=true]:shadow-[8px_0_28px_-10px_rgb(0_0_0_/_0.16),1px_0_0_rgb(0_0_0_/_0.05)] dark:group-data-[hover-expanded=true]:shadow-[10px_0_32px_-10px_rgb(0_0_0_/_0.44),1px_0_0_rgb(255_255_255_/_0.06)]"
            : "group-data-[hover-expanded=true]:shadow-[-8px_0_28px_-10px_rgb(0_0_0_/_0.16),-1px_0_0_rgb(0_0_0_/_0.05)] dark:group-data-[hover-expanded=true]:shadow-[-10px_0_32px_-10px_rgb(0_0_0_/_0.44),-1px_0_0_rgb(255_255_255_/_0.06)]",
          className
        )}
        onPointerEnter={handlePointerEnter}
        onPointerLeave={handlePointerLeave}
        onTransitionEnd={handleContainerTransitionEnd}
        {...props}
      >
        <div
          data-sidebar="sidebar"
          data-slot="sidebar-inner"
          className={cn(
            "flex h-full shrink-0 flex-col bg-sidebar group-data-[variant=floating]:rounded-lg group-data-[variant=floating]:border group-data-[variant=floating]:border-sidebar-border group-data-[variant=floating]:shadow-sm",
            usesHoverReveal
              ? cn(
                  "whitespace-nowrap",
                  layoutState === "collapsed"
                    ? "w-(--sidebar-width-icon)"
                    : "w-(--sidebar-width)"
                )
              : "w-full"
          )}
        >
          <SidebarHoverExpansionContext.Provider value={hoverExpansionContextValue}>
            <SidebarVisualStateContext.Provider value={layoutState}>
              {children}
            </SidebarVisualStateContext.Provider>
          </SidebarHoverExpansionContext.Provider>
        </div>
      </div>
    </div>
  )
}

function SidebarTrigger({
  className,
  onClick,
  ...props
}: React.ComponentProps<typeof Button>) {
  const { toggleSidebar } = useSidebarActions()
  const t = useTranslations("common.navigation")

  return (
    <Button
      data-sidebar="trigger"
      data-slot="sidebar-trigger"
      variant="ghost"
      size="icon"
      className={cn("size-7", className)}
      onClick={(event) => {
        onClick?.(event)
        toggleSidebar()
      }}
      {...props}
    >
      <PanelLeftIcon />
      <span className="sr-only">{t("toggleSidebar")}</span>
    </Button>
  )
}

function SidebarRail({ className, ...props }: React.ComponentProps<"button">) {
  const { toggleSidebar } = useSidebarActions()
  const t = useTranslations("common.navigation")

  return (
    <button
      data-sidebar="rail"
      data-slot="sidebar-rail"
      aria-label={t("toggleSidebar")}
      tabIndex={-1}
      onClick={toggleSidebar}
      title={t("toggleSidebar")}
      className={cn(
        "absolute inset-y-0 z-20 hidden w-4 -translate-x-1/2 transition-all ease-linear group-data-[side=left]:-right-4 group-data-[side=right]:left-0 after:absolute after:inset-y-0 after:left-1/2 after:w-[2px] hover:after:bg-sidebar-border sm:flex",
        "in-data-[side=left]:cursor-w-resize in-data-[side=right]:cursor-e-resize",
        "[[data-side=left][data-state=collapsed]_&]:cursor-e-resize [[data-side=right][data-state=collapsed]_&]:cursor-w-resize",
        "group-data-[collapsible=offcanvas]:translate-x-0 group-data-[collapsible=offcanvas]:after:left-full hover:group-data-[collapsible=offcanvas]:bg-sidebar",
        "[[data-side=left][data-collapsible=offcanvas]_&]:-right-2",
        "[[data-side=right][data-collapsible=offcanvas]_&]:-left-2",
        className
      )}
      {...props}
    />
  )
}

function SidebarInset({ className, ...props }: React.ComponentProps<"main">) {
  return (
    <main
      data-slot="sidebar-inset"
      className={cn(
        "relative flex min-h-0 w-full flex-1 flex-col overflow-hidden bg-background",
        "md:peer-data-[variant=inset]:m-2 md:peer-data-[variant=inset]:ml-0 md:peer-data-[variant=inset]:rounded-xl md:peer-data-[variant=inset]:shadow-sm md:peer-data-[variant=inset]:peer-data-[state=collapsed]:ml-2",
        className
      )}
      {...props}
    />
  )
}

function SidebarInput({
  className,
  ...props
}: React.ComponentProps<typeof Input>) {
  return (
    <Input
      data-slot="sidebar-input"
      data-sidebar="input"
      className={cn("h-8 w-full bg-background shadow-none", className)}
      {...props}
    />
  )
}

function SidebarHeader({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="sidebar-header"
      data-sidebar="header"
      className={cn("flex flex-col gap-2 p-2", className)}
      {...props}
    />
  )
}

function SidebarFooter({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="sidebar-footer"
      data-sidebar="footer"
      className={cn("flex flex-col gap-2 p-2", className)}
      {...props}
    />
  )
}

function SidebarTransitionContent({
  className,
  asChild = false,
  ...props
}: React.ComponentProps<"div"> & { asChild?: boolean }) {
  const Comp = asChild ? Slot.Root : "div"

  return (
    <Comp
      data-slot="sidebar-transition-content"
      data-sidebar="transition-content"
      className={cn(
        "group-data-[resizing=true]:pointer-events-none group-data-[resizing=true]:invisible",
        className
      )}
      {...props}
    />
  )
}

function SidebarSeparator({
  className,
  ...props
}: React.ComponentProps<typeof Separator>) {
  return (
    <Separator
      data-slot="sidebar-separator"
      data-sidebar="separator"
      className={cn("mx-2 w-auto bg-sidebar-border", className)}
      {...props}
    />
  )
}

function SidebarContent({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="sidebar-content"
      data-sidebar="content"
      className={cn(
        "flex min-h-0 flex-1 flex-col gap-2 overflow-auto group-data-[collapsible=icon]:overflow-hidden",
        className
      )}
      {...props}
    />
  )
}

function SidebarGroup({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="sidebar-group"
      data-sidebar="group"
      className={cn("relative flex w-full min-w-0 flex-col p-2", className)}
      {...props}
    />
  )
}

function SidebarGroupLabel({
  className,
  asChild = false,
  ...props
}: React.ComponentProps<"div"> & { asChild?: boolean }) {
  const Comp = asChild ? Slot.Root : "div"

  return (
    <Comp
      data-slot="sidebar-group-label"
      data-sidebar="group-label"
      className={cn(
        "flex h-8 shrink-0 items-center rounded-md px-2 text-xs font-medium text-sidebar-foreground/70 outline-hidden transition-[margin,opacity] duration-200 ease-linear focus-visible:bg-sidebar-accent focus-visible:text-sidebar-accent-foreground focus-visible:ring-0! [&>svg]:size-4 [&>svg]:shrink-0",
        "group-data-[collapsible=icon]:-mt-8 group-data-[collapsible=icon]:opacity-0",
        className
      )}
      {...props}
    />
  )
}

function SidebarGroupAction({
  className,
  asChild = false,
  ...props
}: React.ComponentProps<"button"> & { asChild?: boolean }) {
  const Comp = asChild ? Slot.Root : "button"

  return (
    <Comp
      data-slot="sidebar-group-action"
      data-sidebar="group-action"
      className={cn(
        "absolute top-3.5 right-3 flex aspect-square w-5 items-center justify-center rounded-md p-0 text-sidebar-foreground outline-hidden transition-transform hover:bg-sidebar-accent hover:text-sidebar-accent-foreground focus-visible:bg-sidebar-accent focus-visible:text-sidebar-accent-foreground focus-visible:ring-0! [&>svg]:size-4 [&>svg]:shrink-0",
        // Increases the hit area of the button on mobile.
        "after:absolute after:-inset-2 md:after:hidden",
        "group-data-[collapsible=icon]:hidden",
        className
      )}
      {...props}
    />
  )
}

function SidebarGroupContent({
  className,
  ...props
}: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="sidebar-group-content"
      data-sidebar="group-content"
      className={cn("w-full text-sm", className)}
      {...props}
    />
  )
}

function SidebarMenu({ className, ...props }: React.ComponentProps<"ul">) {
  return (
    <ul
      data-slot="sidebar-menu"
      data-sidebar="menu"
      className={cn("flex w-full min-w-0 flex-col gap-1", className)}
      {...props}
    />
  )
}

function SidebarMenuItem({ className, ...props }: React.ComponentProps<"li">) {
  return (
    <li
      data-slot="sidebar-menu-item"
      data-sidebar="menu-item"
      className={cn("group/menu-item relative", className)}
      {...props}
    />
  )
}

const sidebarMenuButtonVariants = cva(
  "peer/menu-button flex w-full items-center gap-2 overflow-hidden rounded-md p-2 text-left text-sm outline-hidden transition-[width,height,padding] group-has-data-[sidebar=menu-action]/menu-item:pr-8 group-data-[collapsible=icon]:size-8! group-data-[collapsible=icon]:p-2! hover:bg-sidebar-accent hover:text-sidebar-accent-foreground focus-visible:bg-sidebar-accent focus-visible:text-sidebar-accent-foreground focus-visible:ring-0! active:bg-sidebar-accent active:text-sidebar-accent-foreground disabled:pointer-events-none disabled:opacity-50 aria-disabled:pointer-events-none aria-disabled:opacity-50 data-[active=true]:bg-sidebar-accent data-[active=true]:font-medium data-[active=true]:text-sidebar-accent-foreground data-[state=open]:hover:bg-sidebar-accent data-[state=open]:hover:text-sidebar-accent-foreground [&>span:last-child]:truncate [&>svg]:size-4 [&>svg]:shrink-0",
  {
    variants: {
      variant: {
        default: "hover:bg-sidebar-accent hover:text-sidebar-accent-foreground",
        outline:
          "border border-sidebar-border bg-background hover:border-sidebar-accent hover:bg-sidebar-accent hover:text-sidebar-accent-foreground",
      },
      size: {
        default: "h-8 text-sm",
        sm: "h-7 text-xs",
        lg: "h-12 text-sm group-data-[collapsible=icon]:p-0!",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "default",
    },
  }
)

function SidebarMenuButton({
  asChild = false,
  isActive = false,
  variant = "default",
  size = "default",
  tooltip,
  className,
  ...props
}: React.ComponentProps<"button"> & {
  asChild?: boolean
  isActive?: boolean
  tooltip?: string | React.ComponentProps<typeof TooltipContent>
} & VariantProps<typeof sidebarMenuButtonVariants>) {
  const Comp = asChild ? Slot.Root : "button"
  const isMobile = useSidebarIsMobile()
  const state = useSidebarVisualState()

  const button = (
    <Comp
      data-slot="sidebar-menu-button"
      data-sidebar="menu-button"
      data-size={size}
      data-active={isActive}
      className={cn(sidebarMenuButtonVariants({ variant, size }), className)}
      {...props}
    />
  )

  if (!tooltip) {
    return button
  }

  if (typeof tooltip === "string") {
    tooltip = {
      children: tooltip,
    }
  }

  return (
    <Tooltip>
      <TooltipTrigger asChild>{button}</TooltipTrigger>
      <TooltipContent
        side="right"
        align="center"
        hidden={state !== "collapsed" || isMobile}
        {...tooltip}
      />
    </Tooltip>
  )
}

function SidebarMenuAction({
  className,
  asChild = false,
  showOnHover = false,
  ...props
}: React.ComponentProps<"button"> & {
  asChild?: boolean
  showOnHover?: boolean
}) {
  const Comp = asChild ? Slot.Root : "button"

  return (
    <Comp
      data-slot="sidebar-menu-action"
      data-sidebar="menu-action"
      className={cn(
        "absolute top-1.5 right-1 flex aspect-square w-5 items-center justify-center rounded-md p-0 text-sidebar-foreground outline-hidden transition-transform peer-hover/menu-button:text-sidebar-accent-foreground hover:bg-sidebar-accent hover:text-sidebar-accent-foreground focus-visible:bg-sidebar-accent focus-visible:text-sidebar-accent-foreground focus-visible:ring-0! [&>svg]:size-4 [&>svg]:shrink-0",
        // Increases the hit area of the button on mobile.
        "after:absolute after:-inset-2 md:after:hidden",
        "peer-data-[size=sm]/menu-button:top-1",
        "peer-data-[size=default]/menu-button:top-1.5",
        "peer-data-[size=lg]/menu-button:top-2.5",
        "group-data-[collapsible=icon]:hidden",
        showOnHover &&
          "group-focus-within/menu-item:opacity-100 group-hover/menu-item:opacity-100 peer-data-[active=true]/menu-button:text-sidebar-accent-foreground data-[state=open]:opacity-100 md:opacity-0",
        className
      )}
      {...props}
    />
  )
}

function SidebarMenuBadge({
  className,
  ...props
}: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="sidebar-menu-badge"
      data-sidebar="menu-badge"
      className={cn(
        "pointer-events-none absolute right-1 flex h-5 min-w-5 items-center justify-center rounded-md px-1 text-xs font-medium text-sidebar-foreground tabular-nums select-none",
        "peer-hover/menu-button:text-sidebar-accent-foreground peer-data-[active=true]/menu-button:text-sidebar-accent-foreground",
        "peer-data-[size=sm]/menu-button:top-1",
        "peer-data-[size=default]/menu-button:top-1.5",
        "peer-data-[size=lg]/menu-button:top-2.5",
        "group-data-[collapsible=icon]:hidden",
        className
      )}
      {...props}
    />
  )
}

function SidebarMenuSkeleton({
  className,
  showIcon = false,
  textWidth = "68%",
  ...props
}: React.ComponentProps<"div"> & {
  showIcon?: boolean
  textWidth?: string
}) {
  return (
    <div
      data-slot="sidebar-menu-skeleton"
      data-sidebar="menu-skeleton"
      className={cn("flex h-8 items-center gap-2 rounded-md px-2", className)}
      {...props}
    >
      {showIcon && (
        <Skeleton
          className="size-4 rounded-md"
          data-sidebar="menu-skeleton-icon"
        />
      )}
      <Skeleton
        className="h-4 max-w-(--skeleton-width) flex-1"
        data-sidebar="menu-skeleton-text"
        style={
          {
            "--skeleton-width": textWidth,
          } as React.CSSProperties
        }
      />
    </div>
  )
}

function SidebarMenuSub({ className, ...props }: React.ComponentProps<"ul">) {
  return (
    <ul
      data-slot="sidebar-menu-sub"
      data-sidebar="menu-sub"
      className={cn(
        "mx-3.5 flex min-w-0 translate-x-px flex-col gap-1 border-l border-sidebar-border px-2.5 py-0.5",
        "group-data-[collapsible=icon]:hidden",
        className
      )}
      {...props}
    />
  )
}

function SidebarMenuSubItem({
  className,
  ...props
}: React.ComponentProps<"li">) {
  return (
    <li
      data-slot="sidebar-menu-sub-item"
      data-sidebar="menu-sub-item"
      className={cn("group/menu-sub-item relative", className)}
      {...props}
    />
  )
}

function SidebarMenuSubButton({
  asChild = false,
  size = "md",
  isActive = false,
  className,
  ...props
}: React.ComponentProps<"a"> & {
  asChild?: boolean
  size?: "sm" | "md"
  isActive?: boolean
}) {
  const Comp = asChild ? Slot.Root : "a"

  return (
    <Comp
      data-slot="sidebar-menu-sub-button"
      data-sidebar="menu-sub-button"
      data-size={size}
      data-active={isActive}
      className={cn(
        "flex h-7 min-w-0 -translate-x-px items-center gap-2 overflow-hidden rounded-md px-2 text-sidebar-foreground outline-hidden hover:bg-sidebar-accent hover:text-sidebar-accent-foreground focus-visible:bg-sidebar-accent focus-visible:text-sidebar-accent-foreground focus-visible:ring-0! active:bg-sidebar-accent active:text-sidebar-accent-foreground disabled:pointer-events-none disabled:opacity-50 aria-disabled:pointer-events-none aria-disabled:opacity-50 [&>span:last-child]:truncate [&>svg]:size-4 [&>svg]:shrink-0 [&>svg]:text-sidebar-accent-foreground",
        "data-[active=true]:bg-sidebar-accent data-[active=true]:text-sidebar-accent-foreground",
        size === "sm" && "text-xs",
        size === "md" && "text-sm",
        "group-data-[collapsible=icon]:hidden",
        className
      )}
      {...props}
    />
  )
}

export {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupAction,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarInput,
  SidebarInset,
  SidebarMenu,
  SidebarMenuAction,
  SidebarMenuBadge,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarMenuSkeleton,
  SidebarMenuSub,
  SidebarMenuSubButton,
  SidebarMenuSubItem,
  SidebarProvider,
  SidebarRail,
  SidebarSeparator,
  SidebarTransitionContent,
  SidebarTrigger,
  useSidebarActions,
  useSidebarHoverExpansionLock,
  useSidebarIsMobile,
  useSidebarMobileOpen,
  useSidebarOpen,
  useSidebarVisualState,
}
