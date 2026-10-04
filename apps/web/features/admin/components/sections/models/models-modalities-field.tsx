"use client";

import type * as React from "react";
import { AudioLines, FileText, Image as ImageIcon, MoveRight, Type, Video } from "lucide-react";
import { useTranslations } from "next-intl";

import { DropdownMenuLabel, DropdownMenuSeparator } from "@/components/ui/dropdown-menu";
import { cn } from "@/lib/utils";
import type { AdminLLMModelCatalogResolution } from "@/features/admin/api/llm-types";
import {
  ModelCapabilityMenuItem,
  ModelCapabilityRow,
} from "@/features/admin/components/sections/models/models-capability-row";
import {
  CONFIGURABLE_INPUT_MODALITIES,
  type ModelModality,
  normalizeModalities,
} from "@/features/admin/model/model-input-modalities";

const MODALITY_ICONS: Record<ModelModality, React.ComponentType<{ className?: string; strokeWidth?: number }>> = {
  text: Type,
  image: ImageIcon,
  pdf: FileText,
  audio: AudioLines,
  video: Video,
};

const MODALITY_TONES: Record<ModelModality, string> = {
  text: "bg-sky-500/10 text-sky-700 dark:text-sky-300",
  image: "bg-emerald-500/10 text-emerald-700 dark:text-emerald-300",
  pdf: "bg-amber-500/12 text-amber-700 dark:text-amber-300",
  audio: "bg-violet-500/10 text-violet-700 dark:text-violet-300",
  video: "bg-rose-500/10 text-rose-700 dark:text-rose-300",
};

type ModelModalitiesFieldProps = {
  /** Explicit `inputModalities` override from the capabilities JSON; null follows the catalog. */
  override: ModelModality[] | null;
  resolution: AdminLLMModelCatalogResolution | null;
  disabled?: boolean;
  onChange: (modalities: ModelModality[] | null) => boolean;
};

/**
 * Input → output modalities of the model, as resolved by the server: an explicit `inputModalities`
 * declaration wins, otherwise the models.dev catalog. Output modalities come from the catalog only.
 */
export function ModelModalitiesField({ override, resolution, disabled = false, onChange }: ModelModalitiesFieldProps) {
  const t = useTranslations("adminModels");
  const match = resolution?.matched ? resolution : null;
  const catalogInputs = normalizeModalities(match?.inputModalities);
  const outputs = normalizeModalities(match?.outputModalities);
  const inputs = override ?? catalogInputs;
  const names: Record<ModelModality, string> = {
    text: t("sheet.modalities.names.text"),
    image: t("sheet.modalities.names.image"),
    pdf: t("sheet.modalities.names.pdf"),
    audio: t("sheet.modalities.names.audio"),
    video: t("sheet.modalities.names.video"),
  };

  function toggle(modality: ModelModality) {
    const base = override ?? (catalogInputs.length > 0 ? catalogInputs : ["text" as const]);
    const next = base.includes(modality) ? base.filter((value) => value !== modality) : [...base, modality];
    onChange(normalizeModalities([...next, "text"]));
  }

  return (
    <ModelCapabilityRow
      label={t("sheet.modalities.label")}
      description={t("sheet.modalities.description")}
      editLabel={t("sheet.modalities.edit")}
      custom={override !== null}
      disabled={disabled}
      menu={
        <>
          <ModelCapabilityMenuItem role="menuitemradio" checked={override === null} onSelect={() => onChange(null)}>
            <span>{t("sheet.capabilityMode.auto")}</span>
          </ModelCapabilityMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuLabel className="text-[11px] font-normal text-muted-foreground">
            {t("sheet.modalities.customTitle")}
          </DropdownMenuLabel>
          <ModelCapabilityMenuItem checked disabled>
            <Type strokeWidth={1.8} aria-hidden="true" />
            <span>{names.text}</span>
          </ModelCapabilityMenuItem>
          {CONFIGURABLE_INPUT_MODALITIES.map((modality) => {
            const Icon = MODALITY_ICONS[modality];
            return (
              <ModelCapabilityMenuItem
                key={modality}
                checked={inputs.includes(modality)}
                keepOpen
                onSelect={() => toggle(modality)}
              >
                <Icon strokeWidth={1.8} aria-hidden="true" />
                <span>{names[modality]}</span>
              </ModelCapabilityMenuItem>
            );
          })}
        </>
      }
    >
      {inputs.length > 0 ? (
        <>
          <ModalityChips modalities={inputs} names={names} />
          {outputs.length > 0 ? (
            <>
              <MoveRight className="mx-0.5 size-3 shrink-0 text-muted-foreground" aria-hidden="true" />
              <ModalityChips modalities={outputs} names={names} />
            </>
          ) : null}
        </>
      ) : (
        <span className="truncate text-xs text-muted-foreground">{t("sheet.modalities.unknown")}</span>
      )}
    </ModelCapabilityRow>
  );
}

function ModalityChips({ modalities, names }: { modalities: ModelModality[]; names: Record<ModelModality, string> }) {
  return (
    <>
      {modalities.map((modality) => {
        const Icon = MODALITY_ICONS[modality];
        return (
          <span
            key={modality}
            className={cn("inline-flex size-5 shrink-0 items-center justify-center rounded", MODALITY_TONES[modality])}
            title={names[modality]}
          >
            <Icon className="size-3" strokeWidth={2} aria-hidden="true" />
            <span className="sr-only">{names[modality]}</span>
          </span>
        );
      })}
    </>
  );
}
