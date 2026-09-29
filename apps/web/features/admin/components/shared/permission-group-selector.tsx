"use client";

import * as React from "react";
import { Check, ChevronDownIcon } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { cn } from "@/lib/utils";
import type { PermissionGroup } from "@/features/admin/api/permission-groups";

type PermissionGroupSelectorProps = {
  groups: PermissionGroup[];
  selectedIDs: number[];
  matchedIDs?: number[];
  disabled?: boolean;
  loading?: boolean;
  triggerPrefix?: string;
  placeholder: string;
  emptyLabel: string;
  autoBadgeLabel: string;
  onSelectedIDsChange: (ids: number[]) => void;
  // "inline" renders the trigger as plain text for use inside a sentence or status line.
  variant?: "outline" | "inline";
};

export function PermissionGroupSelector({
  groups,
  selectedIDs,
  matchedIDs = [],
  disabled,
  loading,
  triggerPrefix,
  placeholder,
  emptyLabel,
  autoBadgeLabel,
  onSelectedIDsChange,
  variant = "outline",
}: PermissionGroupSelectorProps) {
  const selectedSet = React.useMemo(() => new Set(selectedIDs), [selectedIDs]);
  const matchedSet = React.useMemo(() => new Set(matchedIDs), [matchedIDs]);
  const selectedGroups = React.useMemo(
    () => groups.filter((group) => selectedSet.has(group.id)),
    [groups, selectedSet],
  );
  const selectedLabel = selectedGroups.map((group) => group.name).join(", ");

  function toggleGroup(groupID: number) {
    const next = new Set(selectedIDs);
    if (next.has(groupID)) {
      next.delete(groupID);
    } else {
      next.add(groupID);
    }
    onSelectedIDsChange(Array.from(next).sort((a, b) => a - b));
  }

  const triggerLabel = loading
    ? placeholder
    : selectedLabel || placeholder;

  return (
    <div className="min-w-0">
      <Popover>
        <PopoverTrigger asChild>
          <Button
            type="button"
            variant={variant === "inline" ? "ghost" : "outline"}
            role="combobox"
            disabled={disabled || loading}
            className={cn(
              "justify-between gap-2 text-xs font-normal focus-visible:ring-[1px] focus-visible:ring-ring/40",
              variant === "inline"
                ? "h-6 w-auto max-w-full rounded-md px-1.5 py-0 hover:bg-muted/60 has-[>svg]:px-1.5"
                : "h-8 w-full border-input/40 bg-transparent px-3 py-1 hover:bg-transparent focus-visible:border-ring/60 has-[>svg]:px-3",
            )}
          >
            {triggerPrefix ? (
              <span className={cn("shrink-0", variant === "inline" ? "font-medium text-foreground" : "text-muted-foreground")}>
                {triggerPrefix}
              </span>
            ) : null}
            <span className={cn("min-w-0 flex-1 truncate text-left", selectedLabel ? "text-foreground/75" : "text-muted-foreground")}>
              {triggerLabel}
            </span>
            <ChevronDownIcon className="size-3 shrink-0 text-muted-foreground opacity-50" />
          </Button>
        </PopoverTrigger>
        <PopoverContent align="start" className="w-[320px] p-1">
          <div className="max-h-64 overflow-y-auto py-1">
            {groups.length === 0 ? (
              <div className="px-2 py-6 text-center text-xs text-muted-foreground">
                {emptyLabel}
              </div>
            ) : (
              groups.map((group) => {
                const selected = selectedSet.has(group.id);
                const autoMatched = matchedSet.has(group.id) && !selected;
                return (
                  <button
                    key={group.id}
                    type="button"
                    aria-pressed={selected}
                    className="relative flex w-full min-w-0 items-center rounded-sm py-1.5 pr-8 pl-2 text-left text-xs hover:bg-accent"
                    onClick={() => toggleGroup(group.id)}
                  >
                    <span className="min-w-0 flex-1 truncate">{group.name}</span>
                    {autoMatched ? (
                      <span className="ml-2 shrink-0 rounded border border-border px-1.5 text-[10px] leading-5 text-muted-foreground">
                        {autoBadgeLabel}
                      </span>
                    ) : null}
                    <Check
                      className={cn(
                        "absolute right-2 size-4 shrink-0 text-muted-foreground",
                        selected ? "opacity-100" : "opacity-0",
                      )}
                    />
                  </button>
                );
              })
            )}
          </div>
        </PopoverContent>
      </Popover>
    </div>
  );
}
