"use client"

import React, { useCallback, useEffect, useMemo, useState, type JSX } from "react"
import Image from "next/image"
import { ChevronLeft, ChevronRight } from "lucide-react"
import {
  AnimatePresence,
  motion,
  MotionProps,
  useAnimation,
  Variants,
} from "motion/react"

import { cn } from "@/lib/utils"
import { Button } from "@/components/ui/button"

interface Tip {
  text: string
  image: string
  url?: string
}

interface LoadingCarouselProps {
  tips?: Tip[]
  className?: string
  autoplayInterval?: number
  showNavigation?: boolean
  showIndicators?: boolean
  showProgress?: boolean
  aspectRatio?: "video" | "square" | "wide"
  textPosition?: "top" | "bottom"
  onTipChange?: (index: number) => void
  backgroundTips?: boolean
  backgroundGradient?: boolean
  shuffleTips?: boolean
  animateText?: boolean
  previousTipLabel?: string
  nextTipLabel?: string
}

const defaultTips: Tip[] = [
  {
    text: "Backend snippets. Shadcn style headless components.. but for your backend.",
    image: "/placeholders/cult-snips.png",
    url: "https://www.newcult.co/backend",
  },
  {
    text: "Create your first directory app today. AI batch scripts to process 100s of urls in seconds.",
    image: "/placeholders/cult-dir.png",
    url: "https://www.newcult.co/templates/cult-seo",
  },
  {
    text: "Cult landing page template. Framer motion, shadcn, and tailwind.",
    image: "/placeholders/cult-rune.png",
    url: "https://www.newcult.co/templates/cult-landing-page",
  },
  {
    text: "Vector embeddings, semantic search, and chat based vector retrieval on easy mode.",
    image: "/placeholders/cult-manifest.png",
    url: "https://www.newcult.co/templates/manifest",
  },
  {
    text: "SEO analysis app. Scraping, analysis, insights, and AI recommendations.",
    image: "/placeholders/cult-seo.png",
    url: "https://www.newcult.co/templates/cult-seo",
  },
]

function shuffleArray<T>(array: T[]): T[] {
  const shuffled = [...array]
  for (let i = shuffled.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1))
    ;[shuffled[i], shuffled[j]] = [shuffled[j], shuffled[i]]
  }
  return shuffled
}

const carouselVariants: Variants = {
  enter: (direction: number) => ({
    x: direction > 0 ? "100%" : "-100%",
    opacity: 0,
  }),
  center: {
    x: 0,
    opacity: 1,
  },
  exit: (direction: number) => ({
    x: direction < 0 ? "100%" : "-100%",
    opacity: 0,
  }),
}

const textVariants: Variants = {
  hidden: { opacity: 0, y: 20 },
  visible: { opacity: 1, y: 0, transition: { delay: 0.3, duration: 0.5 } },
}

const aspectRatioClasses = {
  video: "aspect-video",
  square: "aspect-square",
  wide: "aspect-[2/1]",
}

export function LoadingCarousel({
  onTipChange,
  className,
  tips = defaultTips,
  showProgress = true,
  aspectRatio = "video",
  showNavigation = false,
  showIndicators = true,
  backgroundTips = false,
  textPosition = "bottom",
  autoplayInterval = 4500,
  backgroundGradient = false,
  shuffleTips = false,
  animateText = true,
  previousTipLabel,
  nextTipLabel,
}: LoadingCarouselProps) {
  const [current, setCurrent] = useState(0)
  const [direction, setDirection] = useState(1)
  const displayTips = useMemo(() => (shuffleTips ? shuffleArray(tips) : tips), [shuffleTips, tips])
  const totalTips = displayTips.length

  const selectTip = useCallback((index: number) => {
    if (totalTips <= 0) return
    setDirection(index > current ? 1 : -1)
    const nextIndex = (index + totalTips) % totalTips
    setCurrent(nextIndex)
    onTipChange?.(nextIndex)
  }, [current, onTipChange, totalTips])

  useEffect(() => {
    if (totalTips <= 1) return
    const timer = window.setInterval(() => selectTip(current + 1), autoplayInterval)
    return () => window.clearInterval(timer)
  }, [autoplayInterval, current, selectTip, totalTips])

  const activeTip = displayTips[current]

  return (
    <motion.div
      initial={{ opacity: 0, y: 20 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.8, ease: "easeOut" }}
      className={cn(
        "w-full max-w-6xl mx-auto rounded-lg bg-muted shadow-[0px_1px_1px_0px_rgba(0,0,0,0.05),0px_1px_1px_0px_rgba(255,252,240,0.5)_inset,0px_0px_0px_1px_hsla(0,0%,100%,0.1)_inset,0px_0px_1px_0px_rgba(28,27,26,0.5)]",
        className
      )}
    >
      <div className="w-full overflow-hidden rounded-lg">
        <div className="relative w-full">
          <AnimatePresence initial={false} custom={direction} mode="wait">
            {activeTip ? (
              <motion.div
                key={`${activeTip.image}-${current}`}
                variants={carouselVariants}
                initial="enter"
                animate="center"
                exit="exit"
                custom={direction}
                transition={{ duration: 0.45, ease: "easeInOut" }}
                data-slot="loading-carousel-frame"
                className={cn("relative w-full overflow-hidden", aspectRatioClasses[aspectRatio])}
              >
                <Image
                  src={activeTip.image}
                  alt={`Visual representation for tip: ${activeTip.text}`}
                  fill
                  className="bg-muted/15 object-contain p-3"
                  priority
                />
                {backgroundGradient && (
                  <div className="absolute inset-0 bg-gradient-to-t from-black/86 via-black/10 to-transparent" />
                )}

                {backgroundTips ? (
                  <motion.div
                    variants={textVariants}
                    initial="hidden"
                    animate="visible"
                    className={cn(
                      "absolute left-0 right-0 p-4 sm:p-5",
                      textPosition === "top" ? "top-0" : "bottom-0"
                    )}
                  >
                    {activeTip.url ? (
                      <a href={activeTip.url} target="_blank" rel="noopener noreferrer">
                        <p className="text-left text-base font-semibold leading-snug tracking-normal text-white">
                          {activeTip.text}
                        </p>
                      </a>
                    ) : (
                      <p className="text-left text-base font-semibold leading-snug tracking-normal text-white">
                        {activeTip.text}
                      </p>
                    )}
                  </motion.div>
                ) : null}
              </motion.div>
            ) : null}
          </AnimatePresence>
          {showNavigation && totalTips > 1 ? (
            <>
              <Button
                type="button"
                variant="secondary"
                size="icon"
                className="absolute left-2 top-1/2 size-7 -translate-y-1/2 rounded-full shadow-none"
                onClick={() => selectTip(current - 1)}
                aria-label={previousTipLabel}
              >
                <ChevronLeft className="size-3.5" />
              </Button>
              <Button
                type="button"
                variant="secondary"
                size="icon"
                className="absolute right-2 top-1/2 size-7 -translate-y-1/2 rounded-full shadow-none"
                onClick={() => selectTip(current + 1)}
                aria-label={nextTipLabel}
              >
                <ChevronRight className="size-3.5" />
              </Button>
            </>
          ) : null}
        </div>
        <div
          className={cn("bg-muted/35 p-3")}
        >
          <div
            className={cn(
              "flex flex-col items-start gap-3",
              showIndicators && !backgroundTips
                ? ""
                : ""
            )}
          >
            {showIndicators && (
              <div className="flex w-full gap-1.5 overflow-x-auto">
                {displayTips.map((_, index) => (
                  <button
                    key={index}
                    className="h-1 min-w-0 flex-1 overflow-hidden rounded-full bg-border/80"
                    onClick={() => selectTip(index)}
                    aria-label={`Go to tip ${index + 1}`}
                  >
                    {index === current ? (
                      <motion.span
                        key={`progress-${current}`}
                        className="block h-full origin-left rounded-full bg-foreground/75"
                        initial={{ scaleX: showProgress ? 0 : 1 }}
                        animate={{ scaleX: 1 }}
                        transition={{ duration: showProgress ? autoplayInterval / 1000 : 0, ease: "linear" }}
                      />
                    ) : null}
                  </button>
                ))}
              </div>
            )}
            <div className="flex min-h-14 items-start gap-2 text-foreground">
              {backgroundTips ? (
                <span className="text-sm font-medium">
                  Tip {current + 1}/{displayTips.length}
                </span>
              ) : (
                <div className="flex flex-col">
                  {activeTip?.url ? (
                    <a
                      href={activeTip.url}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="text-xs font-medium leading-5 tracking-normal text-foreground"
                    >
                      {animateText ? (
                        <TextScramble
                          key={activeTip.text}
                          duration={1.2}
                          characterSet=". "
                        >
                          {activeTip.text}
                        </TextScramble>
                      ) : (
                        activeTip.text
                      )}
                    </a>
                  ) : (
                    <span className="text-xs font-medium leading-5 tracking-normal text-foreground">
                      {animateText && activeTip ? (
                        <TextScramble
                          key={activeTip.text}
                          duration={1.2}
                          characterSet=". "
                        >
                          {activeTip.text}
                        </TextScramble>
                      ) : (
                        activeTip?.text
                      )}
                    </span>
                  )}
                </div>
              )}
              {backgroundTips && <ChevronRight className="mt-0.5 size-4 shrink-0" />}
            </div>
          </div>
        </div>
      </div>
    </motion.div>
  )
}

// Credit -> https://motion-primitives.com/docs/text-scramble
// https://x.com/Ibelick
type TextScrambleProps = {
  children: string
  duration?: number
  speed?: number
  characterSet?: string
  as?: React.ElementType
  className?: string
  trigger?: boolean
  onScrambleComplete?: () => void
} & MotionProps

const defaultChars =
  "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

function TextScramble({
  children,
  duration = 0.8,
  speed = 0.04,
  characterSet = defaultChars,
  className,
  as: Component = "p",
  trigger = true,
  onScrambleComplete,
  ...props
}: TextScrambleProps) {
  const MotionComponent = motion.create(
    Component as keyof JSX.IntrinsicElements
  )
  const [displayText, setDisplayText] = useState(children)
  const isAnimatingRef = React.useRef(false)
  const intervalRef = React.useRef<ReturnType<typeof setInterval> | null>(null)
  const text = children

  const scramble = useCallback(() => {
    if (isAnimatingRef.current) return
    isAnimatingRef.current = true

    const steps = duration / speed
    let step = 0

    if (intervalRef.current) {
      clearInterval(intervalRef.current)
    }

    intervalRef.current = setInterval(() => {
      let scrambled = ""
      const progress = step / steps

      for (let i = 0; i < text.length; i++) {
        if (text[i] === " ") {
          scrambled += " "
          continue
        }

        if (progress * text.length > i) {
          scrambled += text[i]
        } else {
          scrambled +=
            characterSet[Math.floor(Math.random() * characterSet.length)]
        }
      }

      setDisplayText(scrambled)
      step++

      if (step > steps) {
        if (intervalRef.current) {
          clearInterval(intervalRef.current)
          intervalRef.current = null
        }
        setDisplayText(text)
        isAnimatingRef.current = false
        onScrambleComplete?.()
      }
    }, speed * 1000)
  }, [characterSet, duration, onScrambleComplete, speed, text])

  useEffect(() => {
    if (!trigger) return

    scramble()
    return () => {
      if (intervalRef.current) {
        clearInterval(intervalRef.current)
        intervalRef.current = null
      }
      isAnimatingRef.current = false
    }
  }, [scramble, trigger])

  return (
    <MotionComponent className={className} {...props}>
      {displayText}
    </MotionComponent>
  )
}

export default LoadingCarousel
