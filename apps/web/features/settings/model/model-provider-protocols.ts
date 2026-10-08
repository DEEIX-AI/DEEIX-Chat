import { PROVIDER_IMPORT_PROTOCOLS, type ProviderImportProtocol } from "@deeix/core";

/** Translation key for each protocol the server offers for user-supplied keys. */
const PROTOCOL_LABEL_KEYS: Record<ProviderImportProtocol, `protocols.${ProviderImportProtocol}`> = {
  openai_chat_completions: "protocols.openai_chat_completions",
  openai_responses: "protocols.openai_responses",
  anthropic_messages: "protocols.anthropic_messages",
  google_generate_content: "protocols.google_generate_content",
  gemini_interactions: "protocols.gemini_interactions",
  xai_responses: "protocols.xai_responses",
  openrouter_chat_completions: "protocols.openrouter_chat_completions",
  openrouter_responses: "protocols.openrouter_responses",
};

function isKnownProtocol(value: string): value is ProviderImportProtocol {
  return PROVIDER_IMPORT_PROTOCOLS.some((protocol) => protocol === value);
}

/**
 * Label for a protocol. A newer server may offer one this client does not
 * know; it is shown verbatim instead of breaking the list.
 */
export function modelProviderProtocolLabel(
  protocol: string,
  translate: (key: `protocols.${ProviderImportProtocol}`) => string,
): string {
  return isKnownProtocol(protocol) ? translate(PROTOCOL_LABEL_KEYS[protocol]) : protocol;
}
