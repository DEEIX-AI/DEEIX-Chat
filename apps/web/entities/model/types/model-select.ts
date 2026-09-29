import type { OptionSelectOption } from "@/shared/components/option-select";

// An option of ModelSelect: a generic select option plus the model's icon.
export type ModelSelectOption = OptionSelectOption & {
  iconUrl?: string | null;
};
