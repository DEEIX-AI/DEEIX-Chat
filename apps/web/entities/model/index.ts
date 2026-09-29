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
export { parseProtocolsJSON } from "@/entities/model/lib/model-protocols";
export { localizedNativeToolText } from "@/entities/model/lib/native-tool-i18n";
export {
  nativeToolDefinitionVariantsFromConfig,
  nativeToolPayloadMatchesShape,
  nativeToolPayloadSignature,
} from "@/entities/model/lib/native-tool-payload";
export { parseKindsJSON, stringifyKinds } from "@/entities/model/model/llm-schema";
export type { ModelSelectOption } from "@/entities/model/types/model-select";
