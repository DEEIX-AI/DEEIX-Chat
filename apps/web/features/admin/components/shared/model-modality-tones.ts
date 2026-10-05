import type { ModelModality } from "@/features/admin/model/model-input-modalities";

// One tint per medium, shared by the model sheet's modality chips and the model table's kind icons,
// so a medium reads in one colour across the admin UI.
export const MODEL_MODALITY_TONES: Record<ModelModality, string> = {
  text: "bg-sky-500/10 text-sky-700 dark:text-sky-300",
  image: "bg-emerald-500/10 text-emerald-700 dark:text-emerald-300",
  pdf: "bg-amber-500/12 text-amber-700 dark:text-amber-300",
  audio: "bg-violet-500/10 text-violet-700 dark:text-violet-300",
  video: "bg-rose-500/10 text-rose-700 dark:text-rose-300",
};
