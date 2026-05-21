"use client";

import * as React from "react";
import { Archive, ChevronDown, PencilLine, Share2, Star, StarOff, Trash } from "lucide-react";
import { useTranslations } from "next-intl";

import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItemIcon,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { SpinnerLabel } from "@/components/ui/spinner";
import { AnimatedText } from "@/components/ui/animated-text";
import { cn } from "@/lib/utils";

type ChatLabelProps = {
  title: string;
  starred?: boolean;
  className?: string;
  onToggleStar?: () => void | Promise<void>;
  onRename?: (title: string) => void | Promise<void>;
  onAddToProject?: () => void | Promise<void>;
  onShare?: () => void;
  shareActive?: boolean;
  onDelete?: () => void | Promise<void>;
};

export function ChatLabel({
  title,
  starred = false,
  className,
  onToggleStar,
  onRename,
  onAddToProject,
  onShare,
  shareActive = false,
  onDelete,
}: ChatLabelProps) {
  const t = useTranslations("chat.labelMenu");
  const common = useTranslations("common.actions");
  const [menuOpen, setMenuOpen] = React.useState(false);
  const [renameDialogOpen, setRenameDialogOpen] = React.useState(false);
  const [renameValue, setRenameValue] = React.useState(title);
  const [renaming, setRenaming] = React.useState(false);

  React.useEffect(() => {
    if (renameDialogOpen) {
      setRenameValue(title);
    }
  }, [renameDialogOpen, title]);

  const commitRename = React.useCallback(async () => {
    const nextTitle = renameValue.trim();
    if (!onRename || !nextTitle || nextTitle === title || renaming) {
      setRenameDialogOpen(false);
      return;
    }
    setRenaming(true);
    try {
      await onRename(nextTitle);
      setRenameDialogOpen(false);
    } finally {
      setRenaming(false);
    }
  }, [onRename, renameValue, title, renaming]);

  return (
    <div className={cn("inline-flex max-w-full items-center", className)}>
      <DropdownMenu
        modal={false}
        open={menuOpen}
        onOpenChange={setMenuOpen}
      >
        <DropdownMenuTrigger asChild>
          <button
            id="chat-label-actions-trigger"
            type="button"
            aria-label={t("actions")}
            className={cn(
              "group inline-flex h-7 max-w-full items-center gap-0.5 rounded-lg text-left transition-colors",
              "hover:bg-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/50",
            )}
          >
            <span className="inline-flex min-w-0 items-center px-2">
              <AnimatedText
                text={title}
                className="max-w-full"
                textClassName="text-sm font-medium leading-none text-foreground"
              />
            </span>
            <span className="inline-flex h-7 items-center px-1 text-muted-foreground transition-colors group-hover:text-foreground">
              <ChevronDown className="size-4 stroke-[1.8]" />
            </span>
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent
          side="bottom"
          align="end"
          sideOffset={8}
          className="w-44"
        >
          <DropdownMenuItem
            disabled={!onToggleStar}
            onSelect={(event) => {
              event.preventDefault();
              if (!onToggleStar) {
                return;
              }
              void onToggleStar();
            }}
          >
            <DropdownMenuItemIcon icon={starred ? StarOff : Star} />
            {starred ? t("unstar") : t("star")}
          </DropdownMenuItem>
          <DropdownMenuItem
            disabled={!onRename}
            onSelect={(event) => {
              event.preventDefault();
              if (!onRename) {
                return;
              }
              setMenuOpen(false);
              requestAnimationFrame(() => {
                setRenameDialogOpen(true);
              });
            }}
          >
            <DropdownMenuItemIcon icon={PencilLine} />
            {t("rename")}
          </DropdownMenuItem>
          <DropdownMenuItem
            disabled={!onShare}
            onSelect={(event) => {
              event.preventDefault();
              if (!onShare) {
                return;
              }
              setMenuOpen(false);
              onShare();
            }}
          >
            <DropdownMenuItemIcon icon={Share2} />
            {shareActive ? t("manageShare") : t("share")}
          </DropdownMenuItem>
          <DropdownMenuItem
            disabled={!onAddToProject}
            onSelect={(event) => {
              event.preventDefault();
              if (!onAddToProject) {
                return;
              }
              void onAddToProject();
            }}
          >
            <DropdownMenuItemIcon icon={Archive} />
            {t("archive")}
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem
            variant="destructive"
            disabled={!onDelete}
            onSelect={(event) => {
              event.preventDefault();
              if (!onDelete) {
                return;
              }
              void onDelete();
            }}
          >
            <DropdownMenuItemIcon icon={Trash} className="text-current" />
            {t("delete")}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      <Dialog open={renameDialogOpen} onOpenChange={setRenameDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("rename")}</DialogTitle>
            <DialogDescription>{t("renameDescription")}</DialogDescription>
          </DialogHeader>
          <form
            onSubmit={(event) => {
              event.preventDefault();
              void commitRename();
            }}
            className="space-y-4"
          >
            <Input
              autoFocus
              value={renameValue}
              onChange={(event) => setRenameValue(event.target.value)}
              placeholder={t("renamePlaceholder")}
            />
            <DialogFooter>
              <Button
                type="button"
                variant="ghost"
                onClick={() => setRenameDialogOpen(false)}
                disabled={renaming}
              >
                {common("cancel")}
              </Button>
              <Button type="submit" disabled={renaming}>
                {renaming ? <SpinnerLabel>{common("saving")}</SpinnerLabel> : common("save")}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </div>
  );
}
