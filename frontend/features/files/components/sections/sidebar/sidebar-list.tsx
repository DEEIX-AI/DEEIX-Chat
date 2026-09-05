"use client";

import { DatabaseZap, Ellipsis, PencilLine, SquareCheckBig, Trash2, Zap } from "lucide-react";
import { useTranslations } from "next-intl";
import * as React from "react";
import { AnimatedText } from "@/components/ui/animated-text";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuItemIcon,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { CenteredEmptyState } from "@/components/ui/empty-state";
import { Input } from "@/components/ui/input";
import { Spinner } from "@/components/ui/spinner";
import { cn } from "@/lib/utils";
import type { FileObjectDTO } from "@/shared/api/file.types";
import { useLoadMoreSentinel } from "@/shared/hooks/use-load-more-sentinel";
import { resolveFileIcon } from "@/shared/lib/file-display";
import { canManuallyVectorizeFile, isVectorIndexOutdated } from "@/shared/lib/file-processing";

type SidebarListProps = {
  items: FileObjectDTO[];
  emptyState: "all" | "filtered";
  selectedFileID: string | null;
  selectedFileIDs: string[];
  loading: boolean;
  loadingMore: boolean;
  hasMore: boolean;
  syncing: boolean;
  vectorizingFileIDs: string[];
  renamingFileID: string | null;
  renameValue: string;
  onSelect: (fileID: string) => void;
  onToggleSelection: (fileID: string, checked: boolean) => void;
  onLoadMore: () => void;
  onRenameStart: (item: FileObjectDTO) => void;
  onRenameValueChange: (value: string) => void;
  onRenameCommit: (fileID: string, currentFileName: string) => void;
  onRenameCancel: () => void;
  onVectorize: (fileID: string) => void;
  onDeleteRequest: (item: FileObjectDTO) => void;
};

function SidebarListItem({
  item,
  selected,
  checked,
  vectorizing,
  vectorizationBusy,
  renaming,
  renameValue,
  onSelect,
  onToggleSelection,
  onRenameStart,
  onRenameValueChange,
  onRenameCommit,
  onRenameCancel,
  onVectorize,
  onDeleteRequest,
}: {
  item: FileObjectDTO;
  selected: boolean;
  checked: boolean;
  vectorizing: boolean;
  vectorizationBusy: boolean;
  renaming: boolean;
  renameValue: string;
  onSelect: (fileID: string) => void;
  onToggleSelection: (fileID: string, checked: boolean) => void;
  onRenameStart: (item: FileObjectDTO) => void;
  onRenameValueChange: (value: string) => void;
  onRenameCommit: (fileID: string, currentFileName: string) => void;
  onRenameCancel: () => void;
  onVectorize: (fileID: string) => void;
  onDeleteRequest: (item: FileObjectDTO) => void;
}) {
  const t = useTranslations("files");
  const fileIcon = resolveFileIcon(item);
  const showsRetrievalStatus = item.fileCategory !== "image" && item.embedStatus === "ready";
  const vectorizable = canManuallyVectorizeFile(item);
  const [actionsMenuOpen, setActionsMenuOpen] = React.useState(false);

  if (renaming) {
    return (
      <div className="flex h-8 items-center rounded-md bg-accent/75 px-1.5 text-xs text-foreground">
        {React.createElement(fileIcon, { className: "size-3 text-muted-foreground" })}
        <Input
          autoFocus
          value={renameValue}
          aria-label={t("actions.rename")}
          className="ml-2 h-6 min-w-0 flex-1 border-0 bg-transparent px-0 text-xs shadow-none focus-visible:ring-0"
          onChange={(event) => onRenameValueChange(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === "Enter") {
              event.preventDefault();
              onRenameCommit(item.fileID, item.fileName);
            } else if (event.key === "Escape") {
              event.preventDefault();
              onRenameCancel();
            }
          }}
          onBlur={() => onRenameCommit(item.fileID, item.fileName)}
        />
      </div>
    );
  }

  return (
    <div
      className="group relative h-8 w-full max-w-full min-w-0 overflow-hidden rounded-md"
      data-animated-text-scroll-trigger
    >
      <Checkbox
        checked={checked}
        className="absolute left-1.5 top-1/2 z-20 size-3 -translate-y-1/2"
        aria-label={t("actions.selectFile")}
        onClick={(event) => event.stopPropagation()}
        onCheckedChange={(nextChecked) => onToggleSelection(item.fileID, nextChecked === true)}
      />
      <Button
        type="button"
        variant="ghost"
        className={cn(
          "h-8 w-full max-w-full justify-start gap-2 overflow-hidden rounded-md py-0 pl-7 text-left text-xs font-normal shadow-none",
          actionsMenuOpen
            ? showsRetrievalStatus ? "pr-18" : "pr-12"
            : showsRetrievalStatus ? "pr-7 group-hover:pr-18" : "pr-2 group-hover:pr-12",
          selected ? "bg-accent text-accent-foreground hover:bg-accent" : "text-foreground hover:bg-accent/65 hover:text-foreground",
        )}
        onClick={() => onSelect(item.fileID)}
      >
        <span className="flex size-3 shrink-0 items-center justify-center">
          {React.createElement(fileIcon, { className: "size-3 text-muted-foreground" })}
        </span>

        <AnimatedText
          text={item.fileName}
          className="min-w-0 flex-1 text-xs"
          textClassName="text-current"
          scrollOverflow
        />
      </Button>

      <div className="pointer-events-none absolute inset-y-0 right-1 z-20 flex items-center gap-0.5">
        {showsRetrievalStatus ? (
          <span
            title={item.ragOptOut ? t("list.ragDisabled") : t("list.ragReady")}
            className={cn(
              "flex w-5 shrink-0 items-center justify-center",
              item.ragOptOut ? "text-muted-foreground/40" : "text-primary/70",
            )}
          >
            <Zap aria-hidden className="size-3" strokeWidth={1.5} />
          </span>
        ) : null}

        <div
          className={cn(
            "flex items-center gap-0.5 overflow-hidden transition-[max-width,opacity] duration-150",
            actionsMenuOpen
              ? "pointer-events-auto max-w-11 opacity-100"
              : "pointer-events-none max-w-0 opacity-0 group-hover:pointer-events-auto group-hover:max-w-11 group-hover:opacity-100",
          )}
        >
          <DropdownMenu modal={false} open={actionsMenuOpen} onOpenChange={setActionsMenuOpen}>
            <DropdownMenuTrigger asChild>
              <Button
                type="button"
                variant="ghost"
                size="icon"
                className="size-5 shrink-0 rounded-md p-1 text-muted-foreground hover:bg-accent hover:text-foreground"
                aria-label={t("actions.moreActions")}
                onClick={(event) => {
                  event.preventDefault();
                  event.stopPropagation();
                }}
                tabIndex={-1}
              >
                <Ellipsis className="size-3" strokeWidth={1} />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-32">
              <DropdownMenuItem
                onSelect={(event) => {
                  event.preventDefault();
                  onToggleSelection(item.fileID, !checked);
                }}
              >
                <DropdownMenuItemIcon icon={SquareCheckBig} />
                {checked ? t("actions.cancelSelect") : t("actions.select")}
              </DropdownMenuItem>
              <DropdownMenuItem
                onSelect={(event) => {
                  event.preventDefault();
                  onRenameStart(item);
                }}
              >
                <DropdownMenuItemIcon icon={PencilLine} />
                {t("actions.rename")}
              </DropdownMenuItem>
              {vectorizable ? (
                <DropdownMenuItem
                  disabled={vectorizationBusy}
                  onSelect={() => onVectorize(item.fileID)}
                >
                  {vectorizing ? (
                    <Spinner className="size-4 shrink-0 text-muted-foreground" />
                  ) : (
                    <DropdownMenuItemIcon icon={DatabaseZap} />
                  )}
                  {t(isVectorIndexOutdated(item) ? "actions.updateIndex" : "actions.vectorize")}
                </DropdownMenuItem>
              ) : null}
            </DropdownMenuContent>
          </DropdownMenu>

          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="size-5 shrink-0 rounded-md p-1 text-muted-foreground hover:bg-accent hover:text-foreground"
            aria-label={t("actions.delete")}
            onClick={(event) => {
              event.preventDefault();
              event.stopPropagation();
              onDeleteRequest(item);
            }}
            tabIndex={-1}
          >
            <Trash2 className="size-3" strokeWidth={1} />
          </Button>
        </div>
      </div>
    </div>
  );
}

export function SidebarList({
  items,
  emptyState,
  selectedFileID,
  selectedFileIDs,
  loading,
  loadingMore,
  hasMore,
  syncing,
  vectorizingFileIDs,
  renamingFileID,
  renameValue,
  onSelect,
  onToggleSelection,
  onLoadMore,
  onRenameStart,
  onRenameValueChange,
  onRenameCommit,
  onRenameCancel,
  onVectorize,
  onDeleteRequest,
}: SidebarListProps) {
  const t = useTranslations("files");
  const scrollAreaRef = React.useRef<HTMLDivElement | null>(null);
  const selectedFileIDSet = React.useMemo(() => new Set(selectedFileIDs), [selectedFileIDs]);
  const vectorizingFileIDSet = React.useMemo(() => new Set(vectorizingFileIDs), [vectorizingFileIDs]);

  const loadMoreRef = useLoadMoreSentinel<HTMLDivElement>({
    enabled: hasMore && !loading && !loadingMore,
    rootMargin: "0px 0px 200px 0px",
    rootRef: scrollAreaRef,
    onLoadMore,
  });

  if (loading) {
    return (
      <div className="flex min-h-0 min-w-0 flex-1 items-center justify-center pr-2 text-muted-foreground">
        <Spinner className="size-4" />
      </div>
    );
  }

  if (items.length === 0) {
    return (
      <CenteredEmptyState
        className="min-w-0 flex-1"
        title={t(emptyState === "filtered" ? "searchEmpty" : "empty")}
        description={t(emptyState === "filtered" ? "searchEmptyDescription" : "emptyDescription")}
      />
    );
  }

  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
      <div
        ref={scrollAreaRef}
        className="min-h-0 min-w-0 flex-1 overflow-y-auto overflow-x-hidden pr-2"
      >
        <div className="w-full max-w-full min-w-0 space-y-1 px-1.5 py-2.5 pb-4">
          {items.length > 0 ? (
            items.map((item) => {
              const isSelected = item.fileID === selectedFileID;
              const isChecked = selectedFileIDSet.has(item.fileID);
              const isRenaming = renamingFileID === item.fileID;

              return (
                <SidebarListItem
                  key={item.fileID}
                  item={item}
                  selected={isSelected}
                  checked={isChecked}
                  vectorizing={vectorizingFileIDSet.has(item.fileID)}
                  vectorizationBusy={vectorizingFileIDs.length > 0}
                  renaming={isRenaming}
                  renameValue={renameValue}
                  onSelect={onSelect}
                  onToggleSelection={onToggleSelection}
                  onRenameStart={onRenameStart}
                  onRenameValueChange={onRenameValueChange}
                  onRenameCommit={onRenameCommit}
                  onRenameCancel={onRenameCancel}
                  onVectorize={onVectorize}
                  onDeleteRequest={onDeleteRequest}
                />
              );
            })
          ) : null}

          {!loading ? (
            <div ref={loadMoreRef} className="px-1.5 pt-2">
              <div className="flex h-9 w-full items-center justify-center text-center text-[11px] text-muted-foreground">
                {loadingMore ? t("list.loadingMore") : syncing ? t("list.syncing") : hasMore ? t("list.loadMore") : items.length > 0 ? t("list.allLoaded") : null}
              </div>
            </div>
          ) : null}
        </div>
      </div>
    </div>
  );
}
