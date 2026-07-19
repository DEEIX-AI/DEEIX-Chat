"use client";

import { useTranslations } from "next-intl";

import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { APP_LOCALE_LABELS, APP_LOCALES, type AppLocale } from "@/i18n/config";
import { useAppLocale } from "@/i18n/app-i18n-provider";
import { cn } from "@/lib/utils";

export function LanguageSelect({
  className,
  triggerClassName,
}: {
  className?: string;
  triggerClassName?: string;
}) {
  const t = useTranslations("common.locale");
  const { locale, setLocale } = useAppLocale();

  return (
    <div className={cn("min-w-0", className)}>
      <Select
        value={locale}
        onValueChange={(value) => {
          void setLocale(value as AppLocale);
        }}
      >
        <SelectTrigger aria-label={t("label")} className={cn("h-8 w-[8.25rem]", triggerClassName)}>
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {APP_LOCALES.map((item) => (
            <SelectItem key={item} value={item}>
              {APP_LOCALE_LABELS[item]}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}
