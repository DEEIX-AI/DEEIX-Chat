"use client";

import { AudioLines, FastForward, Film, Image, ImagePlus, type LucideIcon, Type } from "lucide-react";
import { useTranslations } from "next-intl";

import { Badge } from "@/components/ui/badge";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { MODEL_MODALITY_TONES } from "@/features/admin/components/shared/model-modality-tones";
import type { ModelModality } from "@/features/admin/model/model-input-modalities";
import { MODEL_KINDS } from "@/features/admin/utils/llm-display";
import { cn } from "@/lib/utils";
import { parseKindsJSON } from "@/entities/model";

type ModelKind = (typeof MODEL_KINDS)[number];

// Icons follow the chat composer's media modes; chat itself uses the text modality's icon.
const MODEL_KIND_ICONS: Record<ModelKind, LucideIcon> = {
  chat: Type,
  audio: AudioLines,
  image_gen: Image,
  image_edit: ImagePlus,
  video_gen: Film,
  video_extension: FastForward,
};

// The medium each kind works in, so it takes that modality's tint in the model sheet.
const MODEL_KIND_MEDIA: Record<ModelKind, ModelModality> = {
  chat: "text",
  audio: "audio",
  image_gen: "image",
  image_edit: "image",
  video_gen: "video",
  video_extension: "video",
};

const KNOWN_MODEL_KINDS: ReadonlySet<string> = new Set(MODEL_KINDS);

function isModelKind(kind: string): kind is ModelKind {
  return KNOWN_MODEL_KINDS.has(kind);
}

// A model's kinds as icons, each named by a tooltip and to screen readers. Kinds outside the known
// set keep their text badge.
export function ModelKindIcons({ kindsJson }: { kindsJson: string | null | undefined }) {
  const t = useTranslations("adminModels");
  const kinds = parseKindsJSON(kindsJson);
  if (kinds.length === 0) return <span className="text-muted-foreground">-</span>;
  return (
    <div className="flex h-7 min-w-0 flex-nowrap items-center gap-0.5 overflow-hidden">
      {kinds.map((kind) => {
        if (!isModelKind(kind)) {
          return (
            <Badge key={kind} variant="secondary">
              {kind}
            </Badge>
          );
        }
        const Icon = MODEL_KIND_ICONS[kind];
        const label = t(`kinds.${kind}`);
        return (
          <Tooltip key={kind}>
            <TooltipTrigger asChild>
              <span
                className={cn(
                  "inline-flex size-5 shrink-0 items-center justify-center rounded",
                  MODEL_MODALITY_TONES[MODEL_KIND_MEDIA[kind]],
                )}
              >
                <Icon className="size-3" strokeWidth={2} aria-hidden="true" />
                <span className="sr-only">{label}</span>
              </span>
            </TooltipTrigger>
            <TooltipContent side="top" className="text-xs">
              {label}
            </TooltipContent>
          </Tooltip>
        );
      })}
    </div>
  );
}
