type ModelIdentityInput = {
  code?: string | null;
  provider?: string | null;
  vendor?: string | null;
  icon?: string | null;
};

type VendorCatalogItem = {
  key: string;
  label: string;
  vendorIcon: string;
  modelIcon: string;
  aliases: readonly string[];
  patterns: readonly RegExp[];
};

type ResolvedModelIdentity = {
  vendorKey: string;
  vendorLabel: string;
  vendorIcon: string;
  modelIcon: string;
};

const GENERIC_VENDOR_KEYS = new Set(["openrouter", "copilot", "unknown"]);

const VENDOR_CATALOG: readonly VendorCatalogItem[] = [
  {
    key: "openai",
    label: "OpenAI",
    vendorIcon: "openai",
    modelIcon: "chatgpt",
    aliases: ["openai", "chatgpt"],
    patterns: [/\bchatgpt\b/i, /\bgpt(?:-[a-z0-9.]+)?\b/i, /\bo[134]\b/i, /\bcodex\b/i],
  },
  {
    key: "anthropic",
    label: "Anthropic",
    vendorIcon: "anthropic",
    modelIcon: "claude",
    aliases: ["anthropic", "claude"],
    patterns: [/\bclaude\b/i],
  },
  {
    key: "google",
    label: "Google",
    vendorIcon: "google",
    modelIcon: "gemini",
    aliases: ["google", "gemini", "gemma"],
    patterns: [/\bgemini\b/i, /\bgemma\b/i, /\bimagen\b/i, /\bveo\b/i, /\blearnlm\b/i],
  },
  {
    key: "meta",
    label: "Meta",
    vendorIcon: "meta",
    modelIcon: "meta",
    aliases: ["meta", "llama"],
    patterns: [/\bllama\b/i, /\bmeta[/-]/i],
  },
  {
    key: "microsoft",
    label: "Microsoft",
    vendorIcon: "microsoft",
    modelIcon: "microsoft",
    aliases: ["microsoft", "phi"],
    patterns: [/\bphi(?:-[a-z0-9.]+)?\b/i, /\bmicrosoft[/-]/i],
  },
  {
    key: "amazon",
    label: "Amazon",
    vendorIcon: "aws",
    modelIcon: "aws",
    aliases: ["amazon", "aws", "bedrock", "nova", "titan"],
    patterns: [/\bnova\b/i, /\btitan\b/i, /\bbedrock\b/i, /\bamazon[./-]/i, /\baws[./-]/i],
  },
  {
    key: "nvidia",
    label: "NVIDIA",
    vendorIcon: "nvidia",
    modelIcon: "nvidia",
    aliases: ["nvidia", "nemotron"],
    patterns: [/\bnemotron\b/i, /\bnvidia[/-]/i],
  },
  {
    key: "deepseek",
    label: "DeepSeek",
    vendorIcon: "deepseek",
    modelIcon: "deepseek",
    aliases: ["deepseek"],
    patterns: [/\bdeepseek\b/i],
  },
  {
    key: "moonshot",
    label: "MoonShot",
    vendorIcon: "moonshot",
    modelIcon: "kimi",
    aliases: ["moonshot", "kimi"],
    patterns: [/\bmoonshot\b/i, /\bkimi\b/i],
  },
  {
    key: "zhipu",
    label: "ZhiPu",
    vendorIcon: "zhipu",
    modelIcon: "chatglm",
    aliases: ["zhipu", "glm", "chatglm", "bigmodel"],
    patterns: [/\bglm(?:-[a-z0-9.]+)?\b/i, /\bcharglm\b/i, /\bcogview\b/i, /\bcogvideo\b/i],
  },
  {
    key: "minimax",
    label: "MiniMax",
    vendorIcon: "minimax",
    modelIcon: "minimax",
    aliases: ["minimax"],
    patterns: [/\bminimax\b/i, /\babab\b/i, /\bhailuo\b/i],
  },
  {
    key: "bytedance",
    label: "ByteDance",
    vendorIcon: "bytedance",
    modelIcon: "doubao",
    aliases: ["bytedance", "byte", "volcengine", "doubao", "seed"],
    patterns: [/\bdoubao\b/i, /\bseed\b/i, /\bbytedance\b/i, /\bvolcengine\b/i],
  },
  {
    key: "tencent",
    label: "Tencent",
    vendorIcon: "tencent",
    modelIcon: "hunyuan",
    aliases: ["hunyuan", "tencent"],
    patterns: [/\bhunyuan\b/i, /\btencent[/-]/i],
  },
  {
    key: "longcat",
    label: "LongCat",
    vendorIcon: "longcat",
    modelIcon: "longcat",
    aliases: ["longcat"],
    patterns: [/\blongcat\b/i],
  },
  {
    key: "mistral",
    label: "Mistral",
    vendorIcon: "mistral",
    modelIcon: "mistral",
    aliases: ["mistral"],
    patterns: [/\bmistral\b/i, /\bmixtral\b/i, /\bministral\b/i, /\bcodestral\b/i, /\bpixtral\b/i],
  },
  {
    key: "alibaba",
    label: "Alibaba",
    vendorIcon: "alibaba",
    modelIcon: "qwen",
    aliases: ["alibaba", "qwen", "qwq", "qvq", "tongyi", "wanx"],
    patterns: [/\bqwen/i, /\bqwq\b/i, /\btongyi\b/i, /\bwanx\b/i, /\bqvq\b/i],
  },
  {
    key: "xai",
    label: "xAI",
    vendorIcon: "xai",
    modelIcon: "grok",
    aliases: ["xai", "grok"],
    patterns: [/\bgrok\b/i],
  },
  {
    key: "xiaomi",
    label: "Xiaomi",
    vendorIcon: "xiaomimimo",
    modelIcon: "xiaomimimo",
    aliases: ["xiaomi", "mimo", "xiaomimimo", "xiaomi-mi-mo"],
    patterns: [/\bmimo\b/i, /\bxiaomi\b/i],
  },
  {
    key: "iflytek",
    label: "iFlytek",
    vendorIcon: "iflytekcloud",
    modelIcon: "spark",
    aliases: ["iflytek", "iflytekcloud", "spark"],
    patterns: [/\bspark\b/i, /\biflytek\b/i, /讯飞|星火/i],
  },
  {
    key: "stepfun",
    label: "StepFun",
    vendorIcon: "stepfun",
    modelIcon: "stepfun",
    aliases: ["stepfun", "step"],
    patterns: [/\bstep(?:-[a-z0-9.]+)?\b/i, /\bstepfun\b/i, /阶跃星辰/i],
  },
  {
    key: "baichuan",
    label: "Baichuan",
    vendorIcon: "baichuan",
    modelIcon: "baichuan",
    aliases: ["baichuan"],
    patterns: [/\bbaichuan/i, /百川/i],
  },
  {
    key: "baidu",
    label: "Baidu",
    vendorIcon: "baidu",
    modelIcon: "wenxin",
    aliases: ["baidu", "ernie", "wenxin"],
    patterns: [/\bernie\b/i, /\bwenxin\b/i, /\bbaidu[/-]/i, /文心|百度/i],
  },
  {
    key: "openrouter",
    label: "OpenRouter",
    vendorIcon: "openrouter",
    modelIcon: "openrouter",
    aliases: ["openrouter"],
    patterns: [/\bopenrouter\b/i],
  },
  {
    key: "copilot",
    label: "GitHub Copilot",
    vendorIcon: "copilot",
    modelIcon: "copilot",
    aliases: ["copilot", "github"],
    patterns: [/\bcopilot\b/i, /\bgithub\b/i],
  },
];

export const KNOWN_VENDOR_OPTIONS = VENDOR_CATALOG.map((item) => ({
  value: item.key,
  label: item.label,
}));

function detectModelFamilyIcon(value: string): string {
  if (!value) {
    return "";
  }

  if (/\bclaude\b/i.test(value)) return "claude";
  if (/\bgemma\b/i.test(value)) return "gemma";
  if (/\bgemini\b/i.test(value) || /\bimagen\b/i.test(value) || /\bveo\b/i.test(value)) return "gemini";
  if (/\bllama\b/i.test(value)) return "meta";
  if (/\bphi(?:-[a-z0-9.]+)?\b/i.test(value)) return "microsoft";
  if (/\bnova\b/i.test(value)) return "nova";
  if (/\btitan\b/i.test(value) || /\bbedrock\b/i.test(value)) return "bedrock";
  if (/\bnemotron\b/i.test(value)) return "nvidia";
  if (/\bgrok\b/i.test(value)) return "grok";
  if (/\bglm\b/i.test(value) || /\bchatglm\b/i.test(value)) return "chatglm";
  if (/\bcogview\b/i.test(value)) return "cogview";
  if (/\bkimi\b/i.test(value)) return "kimi";
  if (/\bqwen/i.test(value) || /\bqwq\b/i.test(value) || /\bqvq\b/i.test(value)) return "qwen";
  if (/\bdeepseek\b/i.test(value)) return "deepseek";
  if (/\bdoubao\b/i.test(value) || /\bseed\b/i.test(value)) return "doubao";
  if (/\bhunyuan\b/i.test(value)) return "hunyuan";
  if (/\blongcat\b/i.test(value)) return "longcat";
  if (/\bmistral\b/i.test(value) || /\bmixtral\b/i.test(value) || /\bcodestral\b/i.test(value) || /\bpixtral\b/i.test(value)) return "mistral";
  if (/\bmimo\b/i.test(value)) return "xiaomimimo";
  if (/\bspark\b/i.test(value)) return "spark";
  if (/\bstep(?:-[a-z0-9.]+)?\b/i.test(value)) return "stepfun";
  if (/\bbaichuan/i.test(value)) return "baichuan";
  if (/\bernie\b/i.test(value) || /\bwenxin\b/i.test(value)) return "wenxin";
  if (/\bdall-e\b/i.test(value)) return "dalle";
  if (/\bsora\b/i.test(value)) return "sora";
  if (/\bgpt\b/i.test(value) || /\bchatgpt\b/i.test(value) || /\bo[134]\b/i.test(value) || /\bcodex\b/i.test(value)) return "chatgpt";
  return "";
}

function normalizeValue(value: string | null | undefined): string {
  return value?.trim().toLowerCase() ?? "";
}

function findVendorByExactValue(value: string): VendorCatalogItem | null {
  if (!value) {
    return null;
  }

  for (const item of VENDOR_CATALOG) {
    if (item.aliases.some((alias) => value === alias || value.startsWith(`${alias}.`) || value.startsWith(`${alias}-`))) {
      return item;
    }
  }

  return null;
}

function findVendorByText(value: string): VendorCatalogItem | null {
  if (!value) {
    return null;
  }

  for (const item of VENDOR_CATALOG) {
    if (item.patterns.some((pattern) => pattern.test(value))) {
      return item;
    }
  }

  return null;
}

function chooseResolvedVendor(input: ModelIdentityInput): VendorCatalogItem | null {
  const normalizedCode = normalizeValue(input.code);
  const normalizedIcon = normalizeValue(input.icon);
  const normalizedVendor = normalizeValue(input.vendor);
  const normalizedProvider = normalizeValue(input.provider);

  const vendorFromName = findVendorByText(normalizedCode);

  if (vendorFromName) {
    return vendorFromName;
  }

  const vendorFromIcon = findVendorByExactValue(normalizedIcon) ?? findVendorByText(normalizedIcon);
  if (vendorFromIcon && !GENERIC_VENDOR_KEYS.has(vendorFromIcon.key)) {
    return vendorFromIcon;
  }

  const vendorFromDeclared =
    findVendorByExactValue(normalizedVendor) ??
    findVendorByText(normalizedVendor) ??
    findVendorByExactValue(normalizedProvider) ??
    findVendorByText(normalizedProvider);

  if (vendorFromDeclared) {
    return vendorFromDeclared;
  }

  return vendorFromIcon;
}

export function resolveModelIdentity(input: ModelIdentityInput): ResolvedModelIdentity {
  const resolvedVendor = chooseResolvedVendor(input);
  const rawIcon = input.icon?.trim() ?? "";
  const rawVendor = input.vendor?.trim() ?? "";
  const normalizedCode = normalizeValue(input.code);
  const detectedModelIcon = detectModelFamilyIcon(normalizedCode);

  if (!resolvedVendor) {
    return {
      vendorKey: "unknown",
      vendorLabel: "Unknown",
      vendorIcon: "",
      modelIcon: rawIcon || detectedModelIcon,
    };
  }

  const iconVendor = findVendorByExactValue(normalizeValue(input.icon)) ?? findVendorByText(normalizeValue(input.icon));
  const shouldOverrideGenericIcon = iconVendor != null && GENERIC_VENDOR_KEYS.has(iconVendor.key) && !GENERIC_VENDOR_KEYS.has(resolvedVendor.key);

  return {
    vendorKey: resolvedVendor.key,
    vendorLabel: resolvedVendor.label,
    vendorIcon: resolvedVendor.vendorIcon,
    modelIcon: rawIcon && !shouldOverrideGenericIcon ? rawIcon : detectedModelIcon || resolvedVendor.modelIcon,
  };
}

export function resolveVendorLabel(vendor: string): string {
  return resolveModelIdentity({ vendor }).vendorLabel;
}

export function resolveLobeHubIconURL(icon: string): string | null {
  const normalized = icon.trim();
  if (!normalized) {
    return null;
  }
  return `/vendor/lobehub-icons/${normalized}.svg`;
}
