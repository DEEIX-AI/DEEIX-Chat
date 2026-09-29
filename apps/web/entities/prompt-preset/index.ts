// Public entry of the prompt-preset entity; code outside entities/prompt-preset/ imports only this file.
// Unused re-exports are tree-shaken (package.json "sideEffects"), so UI and logic share one entry.
export { PROMPT_PRESET_LIMITS, normalizePromptPresetName } from "@/entities/prompt-preset/model/prompt-presets";
