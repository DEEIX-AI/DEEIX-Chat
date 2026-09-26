import type { AdminLLMModelDTO } from "@/features/admin/api/llm.types";
import type { ModelSelectOption } from "@/shared/components/model-select";
import {
  isRoutableChatPlatformModel,
  resolveModelOptionIconUrl,
  resolveModelOptionLabel,
} from "@/shared/lib/model-option-display";

export function buildTaskModelOptions({
  models,
  followLabel,
  followValue,
}: {
  models: AdminLLMModelDTO[];
  followLabel: string;
  followValue: string;
}): ModelSelectOption[] {
  const seen = new Set<string>();
  const options: ModelSelectOption[] = [{ label: followLabel, value: followValue, iconUrl: null }];

  for (const item of models) {
    if (!isRoutableChatPlatformModel(item)) continue;
    const platformModelName = item.platformModelName.trim();
    if (!platformModelName || seen.has(platformModelName)) continue;
    seen.add(platformModelName);
    options.push({
      label: resolveModelOptionLabel(platformModelName),
      value: platformModelName,
      iconUrl: resolveModelOptionIconUrl({
        platformModelName,
        vendor: item.vendor ?? "",
        icon: item.icon ?? "",
      }),
    });
  }

  return options;
}
