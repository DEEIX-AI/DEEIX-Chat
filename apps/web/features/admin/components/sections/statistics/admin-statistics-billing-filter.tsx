"use client";

import { Check, CircleDollarSign } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import type { AdminUsageStatisticsBillingScope } from "@/features/admin/api";
import { cn } from "@/lib/utils";

type BillingFilterOption = {
  value: AdminUsageStatisticsBillingScope;
  label: string;
};

export function AdminStatisticsBillingFilter({
  value,
  options,
  label,
  triggerClassName,
  disabled = false,
  onChange,
}: {
  value: AdminUsageStatisticsBillingScope;
  options: BillingFilterOption[];
  label: string;
  triggerClassName?: string;
  disabled?: boolean;
  onChange: (value: AdminUsageStatisticsBillingScope) => void;
}) {
  const selectedLabel = options.find((option) => option.value === value)?.label ?? "";

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          type="button"
          variant="ghost"
          disabled={disabled}
          className={cn(
            "h-8 w-full min-w-0 justify-start gap-2 rounded-md px-2 text-xs font-normal shadow-none data-[state=open]:bg-accent/50",
            triggerClassName,
          )}
        >
          <CircleDollarSign className="size-3.5 shrink-0 text-muted-foreground" />
          <span>{label}</span>
          <span className="ml-auto max-w-32 truncate text-[11px] text-muted-foreground">{selectedLabel}</span>
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent side="right" align="start" sideOffset={8} className="w-40 rounded-md p-1">
        {options.map((option) => (
          <DropdownMenuItem key={option.value} className="h-8 py-0" onSelect={() => onChange(option.value)}>
            <span className="min-w-0 flex-1 truncate">{option.label}</span>
            {value === option.value ? <Check className="size-3.5 text-foreground" /> : null}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
