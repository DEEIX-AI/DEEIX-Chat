"use client";

import { ChevronDownIcon } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";

type ModerationCategorySelectorProps = {
  options: string[];
  value: string[];
  selectAllLabel: string;
  emptyLabel: string;
  selectedLabel: string;
  disabled?: boolean;
  disabledHint?: string;
  onChange: (next: string[]) => void;
};

export function ModerationCategorySelector({
  options,
  value,
  selectAllLabel,
  emptyLabel,
  selectedLabel,
  disabled,
  disabledHint,
  onChange,
}: ModerationCategorySelectorProps) {
  const selected = new Set(value);
  const selectedOptionCount = options.filter((category) => selected.has(category)).length;
  const allSelected = options.length > 0 && selectedOptionCount === options.length;

  function updateCategory(category: string, checked: boolean) {
    const next = new Set(selected);
    if (checked) {
      next.add(category);
    } else {
      next.delete(category);
    }
    onChange(Array.from(next).sort());
  }

  const trigger = (
    <DropdownMenuTrigger asChild>
      <Button
        type="button"
        variant="outline"
        size="sm"
        role="combobox"
        disabled={disabled || options.length === 0}
        className="h-8 w-full justify-between gap-2 border-input/40 bg-transparent px-3 py-1 text-xs font-normal shadow-none hover:bg-transparent focus-visible:border-ring/60 focus-visible:ring-[1px] focus-visible:ring-ring/40 has-[>svg]:px-3"
      >
        <span
          className={cn(
            "min-w-0 flex-1 truncate text-left md:text-right",
            selectedOptionCount > 0 ? "text-foreground/75" : "text-muted-foreground",
          )}
        >
          {selectedOptionCount > 0 ? selectedLabel : emptyLabel}
        </span>
        <ChevronDownIcon className="size-3 shrink-0 text-muted-foreground opacity-50" />
      </Button>
    </DropdownMenuTrigger>
  );

  return (
    <DropdownMenu>
      {disabled && disabledHint ? (
        <Tooltip>
          <TooltipTrigger asChild>
            <span className="block w-full cursor-not-allowed">{trigger}</span>
          </TooltipTrigger>
          <TooltipContent side="top" sideOffset={6}>
            {disabledHint}
          </TooltipContent>
        </Tooltip>
      ) : (
        trigger
      )}
      <DropdownMenuContent
        align="end"
        className="max-h-72 w-[var(--radix-dropdown-menu-trigger-width)]"
      >
        <DropdownMenuCheckboxItem
          checked={allSelected ? true : selectedOptionCount > 0 ? "indeterminate" : false}
          className="pr-8 pl-2 font-medium [&>span:first-child]:right-2 [&>span:first-child]:left-auto"
          onSelect={(event) => event.preventDefault()}
          onCheckedChange={(checked) =>
            onChange(checked === true ? Array.from(new Set(options)).sort() : [])
          }
        >
          {selectAllLabel}
        </DropdownMenuCheckboxItem>
        <DropdownMenuSeparator />
        {options.map((category) => (
          <DropdownMenuCheckboxItem
            key={category}
            checked={selected.has(category)}
            className="pr-8 pl-2 [&>span:first-child]:right-2 [&>span:first-child]:left-auto"
            onSelect={(event) => event.preventDefault()}
            onCheckedChange={(checked) => updateCategory(category, checked === true)}
          >
            <span className="truncate">{category}</span>
          </DropdownMenuCheckboxItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
