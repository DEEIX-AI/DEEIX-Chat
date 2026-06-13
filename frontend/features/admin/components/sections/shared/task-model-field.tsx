"use client";

import * as React from "react";
import { motion } from "motion/react";
import { useTranslations } from "next-intl";

import { Badge } from "@/components/ui/badge";
import { Field, FieldDescription, FieldLabel } from "@/components/ui/field";
import { ModelSelect, type ModelSelectOption } from "@/shared/components/model-select";

export type ModelOption = ModelSelectOption;

export function TaskModelField({
  id,
  label,
  description,
  value,
  fallbackValue,
  dirty,
  disabled,
  modelOptions,
  onChange,
}: {
  id: string;
  label: string;
  description: string;
  value: string;
  fallbackValue: string;
  dirty: boolean;
  disabled: boolean;
  modelOptions: ModelOption[];
  onChange: (value: string) => void;
}) {
  const t = useTranslations("common.states");
  const dirtyBadge = dirty ? <Badge variant="ghost" className="relative -mt-1.5 text-[8px] font-medium text-amber-800">{t("unsaved")}</Badge> : null;

  return (
    <motion.div layout transition={{ duration: 0.2, ease: [0.22, 1, 0.36, 1] }}>
      <Field>
        <div className="flex flex-col gap-2 md:flex-row md:items-start md:gap-4 xl:gap-6">
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2">
              <FieldLabel htmlFor={id}>{label}</FieldLabel>
              {dirtyBadge}
            </div>
            {description ? <FieldDescription className="text-[11px]">{description}</FieldDescription> : null}
          </div>

          <div className="w-full min-w-0 md:w-44 md:shrink-0 xl:w-52">
            <ModelSelect
              id={id}
              value={value}
              fallbackValue={fallbackValue}
              disabled={disabled}
              options={modelOptions}
              onChange={onChange}
            />
          </div>
        </div>
      </Field>
    </motion.div>
  );
}
