"use client";

import { ChevronDown, ShieldCheck, ShieldOff } from "lucide-react";
import { useTranslations } from "next-intl";

import { Button } from "@/components/ui/button";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Switch } from "@/components/ui/switch";
import { cn } from "@/lib/utils";

type AdminCircuitBreakerControlProps = {
  available: boolean;
  enabled: boolean;
  loading: boolean;
  saving: boolean;
  onEnabledChange: (checked: boolean) => void;
};

export function AdminCircuitBreakerControl({
  available,
  enabled,
  loading,
  saving,
  onEnabledChange,
}: AdminCircuitBreakerControlProps) {
  const t = useTranslations("adminModels.circuitBreaker");

  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button
          type="button"
          size="xs"
          variant="ghost"
          className={cn(
            "h-6 rounded-full border-0 px-2 text-[11px] font-normal shadow-none ring-0 transition-colors focus-visible:border-transparent focus-visible:ring-0",
            enabled
              ? "bg-primary text-primary-foreground hover:bg-primary/90 hover:text-primary-foreground"
              : "bg-muted/60 text-muted-foreground hover:bg-muted hover:text-foreground",
          )}
          disabled={loading || !available}
          aria-label={enabled ? t("enabledStatus") : t("disabledStatus")}
        >
          {enabled ? <ShieldCheck className="size-3 stroke-1" /> : <ShieldOff className="size-3 stroke-1" />}
          {t("label")}
          <ChevronDown className="size-3 stroke-1" />
        </Button>
      </PopoverTrigger>
      <PopoverContent align="end" className="w-72 p-3">
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0 space-y-1">
            <p className="text-xs font-medium text-foreground">{t("label")}</p>
            <p className="text-[11px] leading-4 text-muted-foreground">{t("description")}</p>
          </div>
          <Switch
            size="sm"
            className="shrink-0"
            checked={enabled}
            disabled={saving || !available}
            aria-label={t("label")}
            onCheckedChange={onEnabledChange}
          />
        </div>
      </PopoverContent>
    </Popover>
  );
}
