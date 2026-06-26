"use client"

import * as React from "react"
import { useTranslations } from "next-intl"

import { ArrowRight } from "@/components/animate-ui/icons/arrow-right"
import { MessageCircleMore } from "@/components/animate-ui/icons/message-circle-more"
import { Search } from "@/components/animate-ui/icons/search"
import { AnimatedText } from "@/components/ui/animated-text"
import { SpinnerLabel } from "@/components/ui/spinner"
import {
  CommandDialog,
  CommandEmpty,
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command"
import type { ConversationSearchResult } from "@/features/layouts/types/navigation"
import { formatUpdatedAtLabel } from "@/features/layouts/utils/navigation-search"

function NavigationSearchResultItem({
  item,
  onSelect,
}: {
  item: ConversationSearchResult
  onSelect: (href: string) => void
}) {
  const timeT = useTranslations("common.time")
  const [isHovered, setIsHovered] = React.useState(false)

  return (
    <CommandItem
      value={item.searchText}
      keywords={[item.publicID]}
      className="group/search-item flex h-9 items-center gap-2 rounded-lg text-xs select-none data-[selected=true]:bg-accent/60"
      onMouseEnter={() => setIsHovered(true)}
      onMouseLeave={() => setIsHovered(false)}
      onSelect={() => onSelect(item.href)}
    >
      <MessageCircleMore
        strokeWidth={1.2}
        animate={isHovered ? "default" : undefined}
        className="size-4 text-current"
      />
      <AnimatedText
        text={item.title}
        className="min-w-0 flex-1"
        textClassName="text-current"
      />
      <span className="relative flex h-4 min-w-[5.5rem] shrink-0 items-center justify-end">
        <span className="text-xs font-normal tabular-nums text-foreground/55 transition-opacity group-hover/search-item:opacity-0 group-data-[selected=true]/search-item:opacity-0">
          {formatUpdatedAtLabel(item.updatedAt, (key, values) => timeT(key, values))}
        </span>
        <ArrowRight
          size={12}
          strokeWidth={1.2}
          animate={isHovered ? "default" : undefined}
          className="absolute right-0 translate-x-[-2px] opacity-0 transition-all duration-150 group-hover/search-item:translate-x-0 group-hover/search-item:opacity-100 group-data-[selected=true]/search-item:translate-x-0 group-data-[selected=true]/search-item:opacity-100"
        />
      </span>
    </CommandItem>
  )
}

export function NavigationSearch({
  open,
  onOpenChange,
  query,
  onQueryChange,
  results,
  title,
  description,
  placeholder,
  loading,
  loadingText,
  emptyText,
  onSelect,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  query: string
  onQueryChange: (value: string) => void
  results: readonly ConversationSearchResult[]
  title: string
  description: string
  placeholder: string
  loading: boolean
  loadingText: string
  emptyText: string
  onSelect: (href: string) => void
}) {
  const [hasMounted, setHasMounted] = React.useState(false)

  React.useEffect(() => {
    setHasMounted(true)
  }, [])

  if (!hasMounted) {
    return null
  }

  return (
    <CommandDialog
      open={open}
      onOpenChange={onOpenChange}
      title={title}
      description={description}
      commandProps={{ shouldFilter: false }}
      className="w-[calc(100vw-1rem)] max-w-[calc(100vw-1rem)] overflow-hidden rounded-xl border border-border/60 bg-background p-0 sm:w-full sm:max-w-xl lg:max-w-2xl"
    >
      <CommandInput
        autoFocus
        value={query}
        onValueChange={onQueryChange}
        placeholder={placeholder}
        className="flex-1 bg-transparent text-sm text-foreground outline-none placeholder:text-foreground/45"
        wrapperClassName="gap-4 border-b border-border/60 px-4"
        icon={<Search size={18} strokeWidth={1.2} />}
      />

      <CommandList
        scrollContainerClassName="max-h-[280px] overflow-x-hidden overscroll-contain"
        className="px-2 py-2"
      >
        <div>
          <CommandEmpty className="flex min-h-32 items-center justify-center py-8 text-sm text-muted-foreground">
            {loading ? (
              <SpinnerLabel className="justify-center">
                {loadingText}
              </SpinnerLabel>
            ) : (
              emptyText
            )}
          </CommandEmpty>

          <div className="space-y-0.5">
            {results.map((item) => (
              <NavigationSearchResultItem
                key={item.publicID}
                item={item}
                onSelect={onSelect}
              />
            ))}
          </div>
        </div>
      </CommandList>
    </CommandDialog>
  )
}
