type MessageRecord = Record<string, unknown>;

function asRecord(value: unknown): MessageRecord | null {
  return value && typeof value === "object" && !Array.isArray(value) ? value as MessageRecord : null;
}

export function nativeToolMessageKey(toolKey: string): string {
  return toolKey.trim().replaceAll(".", "__");
}

export function localizedNativeToolText(
  messages: unknown,
  section: "nativeToolLabels" | "nativeToolDescriptions",
  toolKey: string,
): string {
  const root = asRecord(messages);
  const chat = asRecord(root?.chat);
  const values = asRecord(chat?.[section]);
  const value = values?.[nativeToolMessageKey(toolKey)];
  return typeof value === "string" ? value.trim() : "";
}
