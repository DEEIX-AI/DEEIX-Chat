"use client";

import * as React from "react";
import { PencilLine, SquareCheckBig } from "lucide-react";
import { useTranslations } from "next-intl";

import { Ellipsis } from "@/components/animate-ui/icons/ellipsis";
import { Trash2 } from "@/components/animate-ui/icons/trash-2";
import { resolveFileIcon } from "@/features/files/utils/file-display";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { CenteredEmptyState } from "@/components/ui/empty-state";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuItemIcon,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import type { FileObjectDTO } from "@/shared/api/file.types";

type SidebarListProps = {
  items: FileObjectDTO[];
  selectedFileID: string | null;
  selectedFileIDs: string[];
  loading: boolean;
  loadingMore: boolean;
  hasMore: boolean;
  syncing: boolean;
  renamingFileID: string | null;
  renameValue: string;
  onSelect: (fileID: string) => void;
  onToggleSelection: (fileID: string, checked: boolean) => void;
  onLoadMore: () => void;
  onRenameStart: (item: FileObjectDTO) => void;
  onRenameValueChange: (value: string) => void;
  onRenameCommit: (fileID: string, currentFileName: string) => void;
  onRenameCancel: () => void;
  onDeleteRequest: (item: FileObjectDTO) => void;
};

function SidebarListItem({
  item,
  selected,
  checked,
  renaming,
  renameValue,
  onSelect,
  onToggleSelection,
  onRenameStart,
  onRenameValueChange,
  onRenameCommit,
  onRenameCancel,
  onDeleteRequest,
}: {
  item: FileObjectDTO;
  selected: boolean;
  checked: boolean;
  renaming: boolean;
  renameValue: string;
  onSelect: (fileID: string) => void;
  onToggleSelection: (fileID: string, checked: boolean) => void;
  onRenameStart: (item: FileObjectDTO) => void;
  onRenameValueChange: (value: string) => void;
  onRenameCommit: (fileID: string, currentFileName: string) => void;
  onRenameCancel: () => void;
  onDeleteRequest: (item: FileObjectDTO) => void;
}) {
  const t = useTranslations("files");
  const [hovered, setHovered] = React.useState(false);
  const fileIcon = resolveFileIcon(item);

  if (renaming) {
    return (
      <div className="flex h-8 items-center rounded-md bg-accent/75 px-1.5 text-xs text-foreground">
        {React.createElement(fileIcon, { className: "size-3 text-muted-foreground" })}
        <input
          autoFocus
          value={renameValue}
          className="h-6 min-w-0 flex-1 bg-transparent text-xs outline-none ml-2"
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
      role="button"
      tabIndex={0}
      className={cn(
        "group relative flex h-8 w-full max-w-full min-w-0 items-center gap-2 overflow-hidden rounded-md pl-1.5 pr-12 text-left transition-colors",
        selected ? "bg-accent text-accent-foreground" : "hover:bg-accent/65",
      )}
      onClick={() => onSelect(item.fileID)}
      onKeyDown={(event) => {
        if (event.key === "Enter" || event.key === " ") {
          event.preventDefault();
          onSelect(item.fileID);
        }
      }}
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
    >
      <Checkbox
        checked={checked}
        className="mr-0.5 size-3 shrink-0"
        aria-label={t("actions.selectFile")}
        onClick={(event) => event.stopPropagation()}
        onCheckedChange={(nextChecked) => onToggleSelection(item.fileID, nextChecked === true)}
      />
      {React.createElement(fileIcon, { className: "size-3 text-muted-foreground" })}

      <span className="min-w-0 flex-1 truncate text-xs" title={item.fileName}>{item.fileName}</span>
      {item.fileCategory !== "image" && item.embedStatus === "ready" ? (
        <span
          title={item.ragOptOut ? t("list.ragDisabled") : t("list.ragReady")}
          className={`shrink-0 text-[10px] ${item.ragOptOut ? "text-muted-foreground/40" : "text-emerald-500/70"}`}
        >
          ⚡
        </span>
      ) : null}

      <div
        className={cn(
          "absolute inset-y-0 right-1 flex items-center gap-0.5 transition-opacity duration-150",
          (hovered || selected) && "pointer-events-auto opacity-100",
          !(hovered || selected) && "pointer-events-none opacity-0",
        )}
      >
        <DropdownMenu modal={false}>
          <DropdownMenuTrigger asChild>
            <Button
              type="button"
              variant="ghost"
              size="icon"
              className="size-5 rounded-md p-1 text-muted-foreground hover:bg-accent hover:text-foreground"
              onClick={(event) => {
                event.preventDefault();
                event.stopPropagation();
              }}
              tabIndex={-1}
            >
              <Ellipsis className="size-3" strokeWidth={1} animate={hovered ? "pulse" : undefined} />
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
          </DropdownMenuContent>
        </DropdownMenu>

        <Button
          type="button"
          variant="ghost"
          size="icon"
          className="size-5 rounded-md p-1 text-muted-foreground hover:bg-accent hover:text-foreground"
          onClick={(event) => {
            event.preventDefault();
            event.stopPropagation();
            onDeleteRequest(item);
          }}
          tabIndex={-1}
        >
          <Trash2 className="size-3" strokeWidth={1} animate={hovered ? "default" : undefined} />
        </Button>
      </div>
    </div>
  );
}

export function SidebarList({
  items,
  selectedFileID,
  selectedFileIDs,
  loading,
  loadingMore,
  hasMore,
  syncing,
  renamingFileID,
  renameValue,
  onSelect,
  onToggleSelection,
  onLoadMore,
  onRenameStart,
  onRenameValueChange,
  onRenameCommit,
  onRenameCancel,
  onDeleteRequest,
}: SidebarListProps) {
  const t = useTranslations("files");
  const scrollAreaRef = React.useRef<HTMLDivElement | null>(null);
  const loadMoreRef = React.useRef<HTMLDivElement | null>(null);
  const selectedFileIDSet = React.useMemo(() => new Set(selectedFileIDs), [selectedFileIDs]);

  React.useEffect(() => {
    if (!hasMore || loading || loadingMore) {
      return;
    }

    const sentinel = loadMoreRef.current;
    const viewport = scrollAreaRef.current;
    if (!sentinel || !viewport) {
      return;
    }

    const observer = new IntersectionObserver(
      (entries) => {
        const entry = entries[0];
        if (entry?.isIntersecting) {
          onLoadMore();
        }
      },
      {
        root: viewport,
        rootMargin: "0px 0px 200px 0px",
        threshold: 0.01,
      },
    );

    observer.observe(sentinel);
    return () => observer.disconnect();
  }, [hasMore, loading, loadingMore, onLoadMore]);

  if (!loading && items.length === 0) {
    return (
      <CenteredEmptyState
        className="min-w-0 flex-1"
        title={t("empty")}
        description={t("emptyDescription")}
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
          {loading ? (
            Array.from({ length: 12 }).map((_, index) => (
              <div key={index} className="flex h-9 items-center gap-1.5 rounded-xl px-1.5">
                <Skeleton className="size-6.5 rounded-lg" />
                <Skeleton className="h-3 w-[60%] rounded-full" />
              </div>
            ))
          ) : items.length > 0 ? (
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
                  renaming={isRenaming}
                  renameValue={renameValue}
                  onSelect={onSelect}
                  onToggleSelection={onToggleSelection}
                  onRenameStart={onRenameStart}
                  onRenameValueChange={onRenameValueChange}
                  onRenameCommit={onRenameCommit}
                  onRenameCancel={onRenameCancel}
                  onDeleteRequest={onDeleteRequest}
                />
              );
            })
          ) : null}

          {!loading ? (
            <div ref={loadMoreRef} className="px-1.5 pt-2">
              <div className="flex h-9 w-full items-center justify-center text-center text-[11px] text-muted-foreground">
                {loadingMore ? t("list.loadingMore") : hasMore ? t("list.loadMore") : syncing ? t("list.syncing") : items.length > 0 ? t("list.allLoaded") : null}
              </div>
            </div>
          ) : null}
        </div>
      </div>
    </div>
  );
}
