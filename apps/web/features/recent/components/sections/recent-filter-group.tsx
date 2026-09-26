"use client";

import { cn } from "@/lib/utils";

export function RecentFilterGroup<T extends string>({
  label,
  value,
  options,
  onChange,
}: {
  label: string;
  value: T;
  options: Array<{ value: T; label: string }>;
  onChange: (value: T) => void;
}) {
  return (
    <div
      className="inline-flex shrink-0 items-center gap-0.5 rounded-full bg-muted/40 p-0.5"
      role="group"
      aria-label={label}
    >
      {options.map((option) => {
        const active = option.value === value;
        return (
          <button
            key={option.value}
            type="button"
            className={cn(
              "max-w-32 truncate rounded-full px-2 py-1 text-[11px] font-medium transition-colors",
              active
                ? "bg-background text-foreground shadow-xs"
                : "text-foreground/60 hover:text-foreground",
            )}
            onClick={() => onChange(option.value)}
          >
            {option.label}
          </button>
        );
      })}
    </div>
  );
}
