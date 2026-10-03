// Public entry of the model entity; code outside entities/model/ imports only this file.
// Unused re-exports are tree-shaken (package.json "sideEffects"), so UI and logic share one entry.
export { ModelIcon } from "@/entities/model/components/model-icon";
export { ModelOptionIcon } from "@/entities/model/components/model-option-icon";
export { ModelSelect } from "@/entities/model/components/model-select";
export { resolveModelIconURL, resolveModelIdentity } from "@/entities/model/lib/model-identity";
export {
  isRoutableChatPlatformModel,
  resolveModelOptionIconUrl,
  resolveModelOptionLabel,
} from "@/entities/model/lib/model-option-display";
export {
  HARD_DENIED_MODEL_OPTION_PATHS,
  MODEL_OPTION_POLICY_PROTOCOLS,
  MODEL_OPTION_POLICY_PROTOCOL_LABELS,
  type ModelNativeToolConfig,
  type ModelOptionPolicy,
  type ModelOptionRuleMap,
  type NativeToolDefinition,
  isModelOptionPathFiltered,
  parseModelOptionRuleMap,
  resolveModelOptionPolicyProtocol,
  uniqueModelOptionPaths,
} from "@/entities/model/lib/model-option-policy";
export { resolveModelPresentationGroup } from "@/entities/model/lib/model-presentation";
export {
  isReasoningEffortLevel,
  type ModelReasoningCapability,
  parseModelReasoningCapability,
  type ReasoningEffortLevel,
} from "@/entities/model/lib/model-reasoning";
export {
  MODEL_CONTROL_ICON_NAMES,
  resolveModelControlIcon,
  resolveNativeToolIcon,
} from "@/entities/model/lib/model-control-icons";
export {
  MAX_PINNED_MODEL_CONTROLS,
  MODEL_CONTROL_REASONING_AUTO,
  MODEL_CONTROL_REASONING_ID,
  MODEL_CONTROL_TOGGLE_OFF,
  MODEL_CONTROL_TOGGLE_ON,
  type ModelControl,
  type ModelControlPlacement,
  type ModelControlType,
  type ModelControlValue,
  modelControlDefaultValue,
  modelControlNumberStep,
  normalizeModelControlValue,
  parseModelControls,
} from "@/entities/model/lib/model-controls";
export { parseProtocolsJSON } from "@/entities/model/lib/model-protocols";
export { localizedNativeToolText } from "@/entities/model/lib/native-tool-i18n";
export {
  nativeToolDefinitionVariantsFromConfig,
  nativeToolPayloadMatchesShape,
  nativeToolPayloadSignature,
} from "@/entities/model/lib/native-tool-payload";
export { parseKindsJSON, stringifyKinds } from "@/entities/model/model/llm-schema";
export type { ModelSelectOption } from "@/entities/model/types/model-select";
