"use client";

import * as React from "react";
import { Archive, ArrowDown, ArrowUp, Folder, Maximize2, Minimize2 } from "lucide-react";
import { useLocale, useTranslations } from "next-intl";

import { ArrowRight } from "@/components/animate-ui/icons/arrow-right";
import { MessageCircleMore } from "@/components/animate-ui/icons/message-circle-more";
import { Search } from "@/components/animate-ui/icons/search";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { DialogCollapsible } from "@/components/ui/dialog";
import { Kbd, KbdGroup } from "@/components/ui/kbd";
import {
  CommandDialog,
  CommandEmpty,
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command";
import { SpinnerLabel } from "@/components/ui/spinner";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import {
  formatUpdatedAtLabel,
  groupConversationSearchResultsByDate,
} from "@/features/layouts/model/navigation-search";
import type { ConversationSearchResult } from "@/features/layouts/types/navigation";
import type { ConversationPreviewMessageDTO } from "@/shared/api/conversation.types";
import { StreamdownRender } from "@/shared/components/markdown/streamdown-render";
import { useLoadMoreSentinel } from "@/shared/hooks/use-load-more-sentinel";
import { useIsMobile } from "@/shared/hooks/use-mobile";
import { usePointerInteraction } from "@/shared/hooks/use-pointer-interaction";
import { useStoredBoolean } from "@/shared/hooks/use-stored-boolean";
import { cn } from "@/lib/utils";

const SEARCH_PREVIEW_PANE_STORAGE_KEY = "deeix.navigation-search.preview.open";

function NavigationSearchResultItem({
  item,
  showPreviewPane,
  onPreview,
  onSelect,
}: {
  item: ConversationSearchResult;
  showPreviewPane: boolean;
  onPreview: (publicID: string) => void;
  onSelect: (href: string) => void;
}) {
  const timeT = useTranslations("common.time");
  const navigationT = useTranslations("common.navigation");

  return (
    <CommandItem
      value={item.publicID}
      keywords={[item.publicID]}
      className="group/search-item h-9 items-center gap-2.5 rounded-md px-2 py-1.5 text-xs select-none data-[selected=true]:bg-accent/60"
      onFocus={showPreviewPane ? () => onPreview(item.publicID) : undefined}
      onPointerMove={showPreviewPane ? () => onPreview(item.publicID) : undefined}
      onSelect={() => onSelect(item.href)}
    >
      <MessageCircleMore
        aria-hidden
        strokeWidth={1.2}
        className="size-4 shrink-0 text-foreground/55"
      />
      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 items-center gap-1.5">
          <span className="truncate font-medium text-foreground">{item.title}</span>
          {item.projectName ? (
            <Badge
              variant="secondary"
              className="hidden h-4 min-w-0 max-w-28 gap-1 rounded-sm bg-muted/65 px-1.5 font-normal text-foreground/50 sm:inline-flex [&>svg]:!size-2.5"
            >
              <Folder aria-hidden strokeWidth={1.4} />
              <span className="truncate">{item.projectName}</span>
            </Badge>
          ) : null}
          {item.status === "archived" ? (
            <span className="inline-flex shrink-0 items-center gap-1 text-[11px] text-foreground/45">
              <Archive aria-hidden className="!size-3" strokeWidth={1.4} />
              <span className="sr-only">{navigationT("archived")}</span>
            </span>
          ) : null}
        </div>
      </div>
      <span className="relative flex h-4 min-w-[5.5rem] shrink-0 items-center justify-end">
        <span className="text-[11px] font-normal tabular-nums text-foreground/45 transition-opacity group-hover/search-item:opacity-0 group-data-[selected=true]/search-item:opacity-0">
          {formatUpdatedAtLabel(item.updatedAt, (key, values) => timeT(key, values))}
        </span>
        <ArrowRight
          aria-hidden
          size={12}
          strokeWidth={1.2}
          className="absolute right-0 translate-x-[-2px] opacity-0 transition-[transform,opacity] duration-150 group-hover/search-item:translate-x-0 group-hover/search-item:opacity-100 group-data-[selected=true]/search-item:translate-x-0 group-data-[selected=true]/search-item:opacity-100"
        />
      </span>
    </CommandItem>
  );
}

function NavigationSearchPreview({
  conversationID,
  messages,
  loading,
  loadFailed,
  onRetry,
}: {
  conversationID: string;
  messages: readonly ConversationPreviewMessageDTO[];
  loading: boolean;
  loadFailed: boolean;
  onRetry: () => void;
}) {
  const navigationT = useTranslations("common.navigation");
  const actionsT = useTranslations("common.actions");
  const scrollRef = React.useRef<HTMLDivElement>(null);
  const visibleMessages = React.useMemo(
    () => messages.filter((message) => Boolean(message.content.trim() || message.errorMessage.trim())),
    [messages],
  );

  React.useEffect(() => {
    if (!conversationID || loading || visibleMessages.length === 0) {
      return;
    }
    const animationFrame = window.requestAnimationFrame(() => {
      const element = scrollRef.current;
      if (element) {
        element.scrollTop = element.scrollHeight;
      }
    });
    return () => window.cancelAnimationFrame(animationFrame);
  }, [conversationID, loading, visibleMessages]);

  if (!conversationID) {
    return (
      <div className="flex min-h-0 flex-1 items-center justify-center px-8 text-center text-xs text-muted-foreground">
        {navigationT("searchPreviewHint")}
      </div>
    );
  }

  if (loading) {
    return (
      <div className="flex min-h-0 flex-1 items-center justify-center px-8 text-xs text-muted-foreground">
        <SpinnerLabel>{navigationT("searchPreviewLoading")}</SpinnerLabel>
      </div>
    );
  }

  if (loadFailed) {
    return (
      <div className="flex min-h-0 flex-1 flex-col items-center justify-center gap-2 px-8 text-xs text-muted-foreground">
        <span>{navigationT("searchPreviewLoadFailed")}</span>
        <Button type="button" variant="ghost" size="xs" onClick={onRetry}>
          {actionsT("retry")}
        </Button>
      </div>
    );
  }

  if (visibleMessages.length === 0) {
    return (
      <div className="flex min-h-0 flex-1 items-center justify-center px-8 text-center text-xs text-muted-foreground">
        {navigationT("searchPreviewEmpty")}
      </div>
    );
  }

  return (
    <div
      ref={scrollRef}
      className="min-h-0 flex-1 scroll-fade-y scroll-fade-12 overflow-y-auto overscroll-contain px-5 py-6"
    >
      <div className="flex min-h-full flex-col gap-5">
        {visibleMessages.map((message) => {
          const content = message.content.trim() || message.errorMessage.trim();
          const isUser = message.role === "user";
          return (
            <div
              key={message.publicID}
              className={cn("min-w-0", isUser && "ml-auto max-w-[88%]")}
            >
              <div
                className={cn(
                  "min-w-0 text-sm",
                  isUser && "rounded-lg bg-muted/65 px-3 py-2",
                )}
              >
                <span className="sr-only">
                  {navigationT(isUser ? "searchRoleUser" : "searchRoleAssistant")}: {" "}
                </span>
                <StreamdownRender content={content} streaming={false} className="text-sm leading-6" />
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
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
  loadingMore = false,
  loadFailed = false,
  loadMoreFailed = false,
  hasMore = false,
  loadingText,
  loadingMoreText = "",
  loadFailedText = "",
  loadMoreFailedText = "",
  emptyText,
  showPreviewPane = false,
  previewConversationID = "",
  previewMessages = [],
  previewLoading = false,
  previewLoadFailed = false,
  onLoadMore = () => undefined,
  onRetry = () => undefined,
  onPreviewChange = () => undefined,
  onPreviewRetry = () => undefined,
  onSelect,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  query: string;
  onQueryChange: (value: string) => void;
  results: readonly ConversationSearchResult[];
  title: string;
  description: string;
  placeholder: string;
  loading: boolean;
  loadingMore?: boolean;
  loadFailed?: boolean;
  loadMoreFailed?: boolean;
  hasMore?: boolean;
  loadingText: string;
  loadingMoreText?: string;
  loadFailedText?: string;
  loadMoreFailedText?: string;
  emptyText: string;
  showPreviewPane?: boolean;
  previewConversationID?: string;
  previewMessages?: readonly ConversationPreviewMessageDTO[];
  previewLoading?: boolean;
  previewLoadFailed?: boolean;
  onLoadMore?: () => void | Promise<void>;
  onRetry?: () => void;
  onPreviewChange?: (publicID: string) => void;
  onPreviewRetry?: () => void;
  onSelect: (href: string) => void;
}) {
  const [hasMounted, setHasMounted] = React.useState(false);
  const locale = useLocale();
  const navigationT = useTranslations("common.navigation");
  const actionsT = useTranslations("common.actions");
  const isMobile = useIsMobile();
  const { hasHoverInput } = usePointerInteraction();
  const [previewPaneOpen, setPreviewPaneOpen] = useStoredBoolean(
    SEARCH_PREVIEW_PANE_STORAGE_KEY,
    true,
  );
  const previewPaneAvailable = showPreviewPane && !isMobile && hasHoverInput;
  const previewPaneEnabled = previewPaneAvailable && previewPaneOpen;
  const scrollRootRef = React.useRef<HTMLDivElement>(null);
  const [previewPublicID, setPreviewPublicID] = React.useState("");
  const resultGroups = React.useMemo(
    () => groupConversationSearchResultsByDate(results, {
      locale,
      todayLabel: navigationT("today"),
    }),
    [locale, navigationT, results],
  );
  const loadMoreRef = useLoadMoreSentinel<HTMLDivElement>({
    enabled: open && hasMore && !loading && !loadingMore && !loadMoreFailed,
    rootMargin: "120px",
    rootRef: scrollRootRef,
    onLoadMore,
  });

  React.useEffect(() => {
    setHasMounted(true);
  }, []);

  React.useEffect(() => {
    if (!open) {
      setPreviewPublicID("");
      return;
    }
    if (results.length === 0) {
      setPreviewPublicID("");
      return;
    }
    setPreviewPublicID((current) => (
      results.some((item) => item.publicID === current) ? current : ""
    ));
  }, [open, results]);

  const handlePreview = React.useCallback((publicID: string) => {
    setPreviewPublicID((current) => (current === publicID ? current : publicID));
  }, []);

  React.useEffect(() => {
    onPreviewChange(open && previewPaneEnabled ? previewPublicID : "");
  }, [onPreviewChange, open, previewPaneEnabled, previewPublicID]);

  if (!hasMounted) {
    return null;
  }

  return (
    <CommandDialog
      open={open}
      onOpenChange={onOpenChange}
      title={title}
      description={description}
      commandProps={{
        shouldFilter: false,
        loop: true,
        value: previewPublicID,
        onValueChange: setPreviewPublicID,
      }}
      className={cn(
        "h-auto w-[calc(100vw-1rem)] max-w-[calc(100vw-1rem)] overflow-hidden rounded-lg border border-border/60 bg-background p-0 transition-[max-width] duration-200 ease-out sm:w-full",
        previewPaneEnabled
          ? "md:max-w-5xl"
          : previewPaneAvailable
            ? "sm:max-w-xl lg:max-w-2xl"
            : "sm:max-w-xl lg:max-w-2xl",
      )}
    >
      <CommandInput
        autoFocus
        maxLength={200}
        value={query}
        onValueChange={onQueryChange}
        placeholder={placeholder}
        className="flex-1 bg-transparent text-sm text-foreground outline-none placeholder:text-foreground/45"
        wrapperClassName="gap-4 border-b border-border/60 px-4"
        icon={<Search aria-hidden size={18} strokeWidth={1.2} />}
      />

      <div
        className={cn(
          "min-h-0 overflow-hidden",
          previewPaneAvailable &&
            "md:grid md:h-[280px] md:transition-[height,grid-template-columns] md:duration-200 md:ease-out",
          previewPaneEnabled
            ? "md:h-[min(70svh,40rem)] md:grid-cols-[minmax(18rem,0.9fr)_minmax(0,1.35fr)]"
            : previewPaneAvailable && "md:grid-cols-[minmax(0,1fr)_0fr]",
        )}
      >
        <CommandList
          scrollContainerRef={scrollRootRef}
          scrollContainerClassName={cn(
            "min-h-0 max-h-[280px] scroll-fade-y scroll-fade-12 overflow-x-hidden overscroll-contain",
            previewPaneAvailable && "md:h-full",
            previewPaneEnabled && "md:max-h-none md:border-r md:border-border/60",
          )}
          className={cn(
            "px-2 py-2",
            previewPaneAvailable && "md:h-full md:[&_[cmdk-list-sizer]]:h-full",
          )}
        >
          <CommandEmpty
            className={cn(
              "flex min-h-36 items-center justify-center py-8 text-xs text-muted-foreground",
              previewPaneAvailable && "md:h-full md:min-h-0",
            )}
          >
            {loading ? (
              <SpinnerLabel className="justify-center">{loadingText}</SpinnerLabel>
            ) : loadFailed ? (
              <div className="flex flex-col items-center gap-2">
                <span>{loadFailedText}</span>
                <Button type="button" variant="ghost" size="xs" onClick={onRetry}>
                  {actionsT("retry")}
                </Button>
              </div>
            ) : (
              emptyText
            )}
          </CommandEmpty>

          <div className="space-y-3">
            {resultGroups.map((group) => (
              <div key={group.key} className="space-y-0.5">
                <div className="px-2 pb-0.5 text-[11px] font-medium text-foreground/45">
                  {group.label}
                </div>
                {group.items.map((item) => (
                  <NavigationSearchResultItem
                    key={item.publicID}
                    item={item}
                    showPreviewPane={previewPaneEnabled}
                    onPreview={handlePreview}
                    onSelect={onSelect}
                  />
                ))}
              </div>
            ))}
          </div>

          {loadingMore ? (
            <SpinnerLabel className="justify-center py-3 text-xs text-muted-foreground">
              {loadingMoreText}
            </SpinnerLabel>
          ) : null}
          {loadMoreFailed ? (
            <div className="flex items-center justify-center gap-2 py-3 text-xs text-muted-foreground">
              <span>{loadMoreFailedText}</span>
              <Button type="button" variant="ghost" size="xs" onClick={() => void onLoadMore()}>
                {actionsT("retry")}
              </Button>
            </div>
          ) : null}
          {hasMore ? <div ref={loadMoreRef} aria-hidden className="h-px" /> : null}
        </CommandList>

        {previewPaneAvailable ? (
          <DialogCollapsible open={previewPaneEnabled} className="hidden min-h-0 min-w-0 md:grid">
            <div className="flex h-full min-h-0">
              <NavigationSearchPreview
                conversationID={previewConversationID}
                messages={previewMessages}
                loading={previewLoading}
                loadFailed={previewLoadFailed}
                onRetry={onPreviewRetry}
              />
            </div>
          </DialogCollapsible>
        ) : null}
      </div>

      {previewPaneAvailable ? (
        <div className="hidden h-9 shrink-0 items-center justify-between gap-4 border-t border-border/60 px-4 text-[11px] text-muted-foreground sm:flex">
          <div className="flex items-center gap-4">
            <span className="inline-flex items-center gap-1.5">
              <KbdGroup>
                <Kbd className="h-4 min-w-4 px-0.5">
                  <ArrowUp aria-hidden className="size-2.5" strokeWidth={1.5} />
                </Kbd>
                <Kbd className="h-4 min-w-4 px-0.5">
                  <ArrowDown aria-hidden className="size-2.5" strokeWidth={1.5} />
                </Kbd>
              </KbdGroup>
              {navigationT("searchShortcutNavigate")}
            </span>
            <span className="inline-flex items-center gap-1.5">
              <Kbd className="h-4 min-w-0 px-1 text-[10px]">Enter</Kbd>
              {navigationT("searchShortcutOpen")}
            </span>
          </div>
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                type="button"
                variant="ghost"
                size="icon-sm"
                className="-mr-1 text-muted-foreground shadow-none"
                aria-label={navigationT(
                  previewPaneEnabled ? "searchCompactView" : "searchPreviewView",
                )}
                onClick={() => setPreviewPaneOpen((current) => !current)}
              >
                {previewPaneEnabled ? (
                  <Minimize2 aria-hidden className="size-3.5" strokeWidth={1.5} />
                ) : (
                  <Maximize2 aria-hidden className="size-3.5" strokeWidth={1.5} />
                )}
              </Button>
            </TooltipTrigger>
            <TooltipContent side="top" sideOffset={6}>
              {navigationT(previewPaneEnabled ? "searchCompactView" : "searchPreviewView")}
            </TooltipContent>
          </Tooltip>
        </div>
      ) : null}
    </CommandDialog>
  );
}
