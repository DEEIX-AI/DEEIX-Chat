"use client";

import * as React from "react";
import { useTranslations } from "next-intl";

import { OptionSelect } from "@/shared/components/model-select";
import { resolveTimeZoneOptions } from "@/shared/lib/time-zone";

type TimeZoneSelectProps = {
  id?: string;
  value: string;
  options?: string[];
  disabled?: boolean;
  fallbackValue?: string;
  placeholder?: string;
  triggerClassName?: string;
  valueClassName?: string;
  contentClassName?: string;
  portalContainer?: HTMLElement | ShadowRoot | null | React.RefObject<HTMLElement | ShadowRoot | null>;
  onChange: (value: string) => void;
};

export function TimeZoneSelect({
  id,
  value,
  options,
  disabled,
  fallbackValue = "Etc/UTC",
  placeholder,
  triggerClassName,
  valueClassName,
  contentClassName = "min-w-[320px]",
  portalContainer,
  onChange,
}: TimeZoneSelectProps) {
  const t = useTranslations("common.timeZoneSelect");
  const resolvedOptions = React.useMemo(() => options ?? resolveTimeZoneOptions(), [options]);
  const selectOptions = React.useMemo(
    () => resolvedOptions.map((timeZone) => ({ label: timeZone, value: timeZone })),
    [resolvedOptions],
  );

  return (
    <OptionSelect
      id={id}
      value={value}
      options={selectOptions}
      fallbackValue={fallbackValue}
      placeholder={placeholder ?? t("placeholder")}
      disabled={disabled}
      valueAlign="start"
      searchPlaceholder={t("searchPlaceholder")}
      emptyText={t("empty")}
      triggerClassName={triggerClassName}
      valueClassName={valueClassName}
      contentClassName={contentClassName}
      portalContainer={portalContainer}
      onChange={onChange}
    />
  );
}
