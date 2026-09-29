// Public entry of the skill entity; code outside entities/skill/ imports only this file.
// Unused re-exports are tree-shaken (package.json "sideEffects"), so UI and logic share one entry.
export {
  EMPTY_SKILL_FORM,
  SKILL_LIMITS,
  type SkillFormValue,
  skillFormFromDTO,
  skillFormIsWithinLimits,
  skillPayloadFromForm,
  skillPayloadIsComplete,
} from "@/entities/skill/model/skills";
