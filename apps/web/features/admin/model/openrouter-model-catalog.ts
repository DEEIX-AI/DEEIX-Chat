import type { AdminOfficialPricingCatalogItemDTO } from "@/features/admin/api/billing.types";

const VENDOR_PREFIXES: Record<string, string[]> = {
  anthropic: ["anthropic"],
  google: ["google"],
  meta: ["meta-llama"],
  microsoft: ["microsoft"],
  amazon: ["amazon"],
  nvidia: ["nvidia"],
  deepseek: ["deepseek"],
  moonshot: ["moonshotai"],
  zhipu: ["thudm", "z-ai"],
  minimax: ["minimax"],
  bytedance: ["bytedance"],
  tencent: ["tencent"],
  openai: ["openai"],
  xai: ["x-ai"],
};

function normalize(value: string | null | undefined): string {
  return value?.normalize("NFKC").trim().toLowerCase() ?? "";
}

function identifierSignature(value: string | null | undefined): string {
  const tokens = normalize(value).match(/[\p{L}]+|[\p{N}]+/gu) ?? [];
  const parts: Array<{ kind: "letters" | "numbers"; value: string }> = [];
  for (const token of tokens) {
    const kind = /^\p{N}+$/u.test(token) ? "numbers" : "letters";
    const previous = parts.at(-1);
    if (kind === "letters" && previous?.kind === "letters") {
      previous.value += token;
    } else {
      parts.push({ kind, value: token });
    }
  }
  return parts.map((part) => part.value).join("|");
}

function modelSegment(value: string | null | undefined): string {
  const segments = normalize(value).split("/").filter(Boolean);
  return segments.at(-1) ?? "";
}

function providerSegment(value: string | null | undefined): string {
  const segments = normalize(value).split("/").filter(Boolean);
  return segments.length > 1 ? segments[0] : "";
}

function catalogKeys(item: AdminOfficialPricingCatalogItemDTO): Set<string> {
  return new Set([
    identifierSignature(item.id),
    identifierSignature(modelSegment(item.id)),
    identifierSignature(item.canonicalSlug),
    identifierSignature(modelSegment(item.canonicalSlug)),
    identifierSignature(item.name),
  ].filter(Boolean));
}

function matchOpenRouterModelCatalogItem(
  items: AdminOfficialPricingCatalogItemDTO[],
  platformModelName: string,
  vendor: string,
): AdminOfficialPricingCatalogItemDTO | null {
  const model = normalize(platformModelName);
  if (!model) {
    return null;
  }

  const fullModelKey = identifierSignature(model);
  const exactMatches = items.filter((item) => catalogKeys(item).has(fullModelKey));
  if (exactMatches.length === 1) {
    return exactMatches[0];
  }

  const leafModelKey = identifierSignature(modelSegment(model));
  const matches = exactMatches.length > 1 ? exactMatches : items.filter((item) => {
    const keys = catalogKeys(item);
    return keys.has(leafModelKey);
  });
  if (matches.length <= 1) {
    return matches[0] ?? null;
  }

  // 同名模型可能由多个组织发布，优先使用已配置厂商消歧；仍有歧义时不猜测。
  const prefixes = new Set([
    identifierSignature(providerSegment(model)),
    ...(VENDOR_PREFIXES[normalize(vendor)] ?? []).map(identifierSignature),
  ].filter(Boolean));
  const vendorMatches = matches.filter((item) =>
    prefixes.has(identifierSignature(providerSegment(item.id) || providerSegment(item.canonicalSlug))),
  );
  return vendorMatches.length === 1 ? vendorMatches[0] : null;
}

export function resolveAutomaticModelContextWindow(
  items: AdminOfficialPricingCatalogItemDTO[],
  platformModelName: string,
  vendor: string,
): number | null {
  const contextLength = matchOpenRouterModelCatalogItem(items, platformModelName, vendor)?.contextLength;
  return typeof contextLength === "number" &&
    Number.isSafeInteger(contextLength) &&
    contextLength >= 4_096 &&
    contextLength <= 16_000_000
    ? contextLength
    : null;
}
