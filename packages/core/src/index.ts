// @deeix/core — platform-agnostic client logic.
//
// Rules (enforced by tsconfig `lib: ["ES2022"]` and biome `noRestrictedImports`):
//   - No React, Next.js, React Native, Expo or Tauri imports.
//   - No `window`, `document`, `localStorage`, `fetch` globals. Hosts inject I/O.
//   - Everything here must run in plain Node.

export { SESSION_TERMINATING_ERROR_CODES, classifyAuthError } from "./auth/errors.ts";
export type { AuthErrorKind, AuthErrorShape } from "./auth/errors.ts";
export { createAuthClient } from "./auth/refresh.ts";
export type {
  AuthClient,
  AuthHost,
  RefreshLock,
  SessionCredentials,
  SessionStore,
  WithAuthRetryOptions,
} from "./auth/refresh.ts";
export {
  ALL_ENABLED,
  FEATURE_DISABLED_ERROR_CODE,
  FEATURE_NAMES,
  disabledFeatureOf,
  resolveCapabilities,
} from "./capabilities/capabilities.ts";
export type { CapabilityFlags, Feature } from "./capabilities/capabilities.ts";
export { normalizeApiBaseUrl, resolveApiBaseUrl } from "./server/url.ts";
export type { ResolveApiBaseUrlInput } from "./server/url.ts";
export {
  MAX_PROVIDER_IMPORT_ENTRIES,
  PROVIDER_IMPORT_PROTOCOLS,
  formatProviderImportLink,
  maskProviderApiKey,
  parseProviderImportLink,
} from "./providers/import-link.ts";
export type { ProviderImportLink, ProviderImportLinkInput, ProviderImportLinkResult, ProviderImportProtocol } from "./providers/import-link.ts";
// Interactive blocks (deeix-ui). Only what hosts use is exported; the engine's
// internals are imported from their modules by the tests.
export { BLOCK_LIMITS, compileBlock, mergeState, parseBlockSource, readAction, resolveBlock, stateDiff } from "./ui-block/block.ts";
export type { BlockIssue, BlockSource, ResolvedNode } from "./ui-block/block.ts";
export { compileExpression, expressionToLatex, formulaToLatex, valuesEqual } from "./ui-block/expression.ts";
export type { CompiledExpression, UIValue } from "./ui-block/expression.ts";
export { formatTemplateNumber } from "./ui-block/template.ts";
export {
  UI_BLOCK_FENCE_LANGUAGE,
  UI_BLOCK_FORMAT_VERSION,
  UI_WIDGET_FENCE_LANGUAGE,
  UI_WIDGET_MAX_BYTES,
  readMessageUIBlocks,
  replaceUIBlockFences,
  scanUIBlockFences,
  widgetKey,
  widgetTitle,
} from "./ui-block/fence.ts";
export { WHERE_LIST_PROP, expandPointProps, takeWhere } from "./ui-block/points.ts";
export { isUIContainer, isUIPrimitive } from "./ui-block/primitives.ts";
