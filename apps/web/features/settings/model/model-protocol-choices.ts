// Protocol choices for a single model of a user's own provider. A model runs on one protocol, or on
// the pair of protocols a media model offers (image generation + editing, xAI video + extension),
// the same combinations the server accepts for platform model bindings.

/** Protocols that only make sense together, offered as one choice. */
const PROTOCOL_PAIRS: ReadonlyArray<readonly [string, string]> = [
  ["openai_image_generations", "openai_image_edits"],
  ["xai_image", "xai_image_edits"],
  ["xai_video", "xai_video_extensions"],
];

export type ModelProtocolKind = "chat" | "image" | "video";

// Protocol names are technical terms (the upstream API endpoints); like the admin console they are
// shown in English and not translated.
const PROTOCOLS: Record<string, { vendor: string; name: string; kind: ModelProtocolKind }> = {
  openai_chat_completions: { vendor: "OpenAI", name: "Chat Completions", kind: "chat" },
  openai_responses: { vendor: "OpenAI", name: "Responses", kind: "chat" },
  openai_image_generations: { vendor: "OpenAI", name: "Images Generations", kind: "image" },
  openai_image_edits: { vendor: "OpenAI", name: "Images Edits", kind: "image" },
  anthropic_messages: { vendor: "Anthropic", name: "Messages", kind: "chat" },
  google_generate_content: { vendor: "Google", name: "Generate Content", kind: "chat" },
  google_image_generation: { vendor: "Google", name: "Image Generation", kind: "image" },
  gemini_interactions: { vendor: "Google", name: "Interactions", kind: "chat" },
  xai_responses: { vendor: "xAI", name: "Responses", kind: "chat" },
  xai_image: { vendor: "xAI", name: "Images Generations", kind: "image" },
  xai_image_edits: { vendor: "xAI", name: "Images Edits", kind: "image" },
  xai_video: { vendor: "xAI", name: "Video Generations", kind: "video" },
  xai_video_extensions: { vendor: "xAI", name: "Video Extensions", kind: "video" },
  openrouter_chat_completions: { vendor: "OpenRouter", name: "Chat Completions", kind: "chat" },
  openrouter_responses: { vendor: "OpenRouter", name: "Responses", kind: "chat" },
  openrouter_images: { vendor: "OpenRouter", name: "Images", kind: "image" },
};

const PAIR_NAMES: Record<string, string> = {
  "openai_image_generations+openai_image_edits": "Images Generations + Edits",
  "xai_image+xai_image_edits": "Images Generations + Edits",
  "xai_video+xai_video_extensions": "Video Generations + Extensions",
};

export type ModelProtocolChoice = {
  value: string;
  protocols: string[];
  /** Endpoint name without the vendor, e.g. "Chat Completions". */
  name: string;
  /** Empty for a protocol this client does not know. */
  vendor: string;
  kind: ModelProtocolKind;
};

/** Select value for a model's protocols; the server keeps their order, so it is stable. */
export function modelProtocolChoiceValue(protocols: readonly string[]): string {
  return protocols.join("+");
}

/** A protocol or protocol pair; an unknown protocol from a newer server is shown verbatim. */
export function describeModelProtocols(protocols: readonly string[]): ModelProtocolChoice {
  const value = modelProtocolChoiceValue(protocols);
  const first = PROTOCOLS[protocols[0] ?? ""];
  const name = PAIR_NAMES[value] ?? protocols.map((protocol) => PROTOCOLS[protocol]?.name ?? protocol).join(" + ");
  return { value, protocols: [...protocols], name, vendor: first?.vendor ?? "", kind: first?.kind ?? "chat" };
}

/** Full label, e.g. "Chat Completions (OpenAI)". */
export function modelProtocolLabel(protocols: readonly string[]): string {
  const { name, vendor } = describeModelProtocols(protocols);
  return vendor ? `${name} (${vendor})` : name;
}

/** Choices for the protocols the server offers, grouped by vendor; a pair comes right before its first protocol. */
export function modelProtocolChoiceGroups(available: readonly string[]): Array<{ vendor: string; choices: ModelProtocolChoice[] }> {
  const offered = new Set(available);
  const groups = new Map<string, ModelProtocolChoice[]>();
  const add = (protocols: readonly string[]) => {
    const choice = describeModelProtocols(protocols);
    groups.set(choice.vendor, [...(groups.get(choice.vendor) ?? []), choice]);
  };
  for (const protocol of available) {
    for (const pair of PROTOCOL_PAIRS) {
      if (pair[0] === protocol && offered.has(pair[1])) add(pair);
    }
    add([protocol]);
  }
  return [...groups].map(([vendor, choices]) => ({ vendor, choices }));
}

/**
 * Request entries for the chosen models. A server without per-model protocols (it offers none)
 * only takes model names.
 */
export function modelSelectionPayload(
  models: ReadonlyArray<{ name: string }>,
  selected: ReadonlyMap<string, readonly string[]>,
  serverProtocols: readonly string[],
): Array<string | { name: string; protocols: string[] }> {
  return models.flatMap((model) => {
    const protocols = selected.get(model.name);
    if (!protocols) return [];
    return [serverProtocols.length > 0 ? { name: model.name, protocols: [...protocols] } : model.name];
  });
}
