export type RenderSegment =
  | {
      type: "markdown";
      content: string;
    }
  | {
      type: "thinking";
      content: string;
      incomplete: boolean;
    };

export function normalizeContent(input: unknown): string {
  if (typeof input === "string") {
    return input;
  }

  if (typeof input === "number" || typeof input === "boolean" || typeof input === "bigint") {
    return String(input);
  }

  if (input == null) {
    return "";
  }

  if (Array.isArray(input)) {
    return input.map((item) => normalizeContent(item)).filter(Boolean).join("\n");
  }

  if (typeof input === "object") {
    const maybeRecord = input as Record<string, unknown>;
    const textValue = maybeRecord.content ?? maybeRecord.text ?? maybeRecord.message;
    if (typeof textValue === "string") {
      return textValue;
    }

    try {
      return JSON.stringify(input, null, 2);
    } catch {
      return "";
    }
  }

  return "";
}

export function normalizeMathDelimiters(source: string): string {
  if (!source || (!source.includes("\\(") && !source.includes("\\["))) {
    return source;
  }

  const fragments = source.split(/(```[\s\S]*?```|`[^`\n]*`)/g);

  return fragments
    .map((fragment) => {
      if (!fragment || fragment.startsWith("```") || fragment.startsWith("`")) {
        return fragment;
      }

      return fragment
        .replace(/\\\[\s*\n?([\s\S]*?)\n?\s*\\\]/g, (_, mathContent: string) => `$$\n${mathContent.trim()}\n$$`)
        .replace(/\\\(([\s\S]*?)\\\)/g, (_, mathContent: string) => `$${mathContent.trim()}$`);
    })
    .join("");
}

const LATEX_UNICODE_SYMBOLS: Array<[RegExp, string]> = [
  [/→/g, " \\to "],
  [/←/g, " \\leftarrow "],
  [/⇒/g, " \\Rightarrow "],
  [/⇐/g, " \\Leftarrow "],
  [/↔/g, " \\leftrightarrow "],
  [/⇔/g, " \\Leftrightarrow "],
];

const INLINE_CODE_OR_FENCE_RE = /(```[\s\S]*?```|~~~[\s\S]*?~~~|`[^`\n]*`)/g;
const THINKING_LIKE_HTML_TAG_RE = /<\/?\s*think[\w-]*\b[^>]*>/gi;

function escapeHtmlTag(value: string): string {
  return value
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");
}

function escapeThinkingLikeHtmlTags(source: string): string {
  if (!source || !/<\/?\s*think/i.test(source)) {
    return source;
  }

  return source
    .split(INLINE_CODE_OR_FENCE_RE)
    .map((fragment) => {
      if (!fragment || fragment.startsWith("```") || fragment.startsWith("~~~") || fragment.startsWith("`")) {
        return fragment;
      }

      return fragment.replace(THINKING_LIKE_HTML_TAG_RE, escapeHtmlTag);
    })
    .join("");
}

function normalizeLatexSymbols(mathContent: string): string {
  return LATEX_UNICODE_SYMBOLS.reduce(
    (normalizedContent, [pattern, replacement]) => normalizedContent.replace(pattern, replacement),
    mathContent,
  );
}

export function normalizeLatexUnicodeSymbols(source: string): string {
  if (!source || !/[→←⇒⇐↔⇔]/.test(source)) {
    return source;
  }

  const fragments = source.split(/(```[\s\S]*?```|`[^`\n]*`)/g);

  return fragments
    .map((fragment) => {
      if (!fragment || fragment.startsWith("```") || fragment.startsWith("`")) {
        return fragment;
      }

      return fragment.replace(/(\${2,})([\s\S]*?)(\1)/g, (match, openingDelimiter: string, mathContent: string, closingDelimiter: string) => {
        if (!mathContent) {
          return match;
        }

        return `${openingDelimiter}${normalizeLatexSymbols(mathContent)}${closingDelimiter}`;
      });
    })
    .join("");
}

export function normalizeMermaidBlocks(source: string): string {
  if (!source.includes("```mermaid")) {
    return source;
  }

  return source.replace(/```mermaid([\s\S]*?)```/gi, (block) =>
    block.replace(/<br\s*>/gi, "<br/>").replace(/<br\s*\/\s*>/gi, "<br/>"),
  );
}

export function parseStreamdownSegments(source: string): RenderSegment[] {
  if (!source) {
    return [];
  }

  const segments: RenderSegment[] = [];

  const thinkingBlock = parseLeadingThinkingBlock(source);
  if (!thinkingBlock) {
    if (source.trim()) {
      segments.push({
        type: "markdown",
        content: escapeThinkingLikeHtmlTags(source),
      });
    }
    return segments;
  }

  segments.push({
    type: "thinking",
    content: thinkingBlock.content,
    incomplete: false,
  });

  const tail = source.slice(thinkingBlock.end);
  if (tail.trim()) {
    segments.push({
      type: "markdown",
      content: escapeThinkingLikeHtmlTags(tail),
    });
  }

  return segments;
}

function parseLeadingThinkingBlock(source: string): { content: string; end: number } | null {
  const firstContentIndex = source.search(/\S/);
  if (firstContentIndex < 0) {
    return null;
  }

  const openingSource = source.slice(firstContentIndex);
  const openingMatch = /^<(think|thinking)\b[^>]*>/i.exec(openingSource);
  if (!openingMatch) {
    return null;
  }
  if (openingMatch[0].slice(0, -1).trimEnd().endsWith("/")) {
    return null;
  }

  const tagName = openingMatch[1].toLowerCase();
  const contentStart = firstContentIndex + openingMatch[0].length;
  const closingMatch = new RegExp(`</${tagName}\\s*>`, "i").exec(source.slice(contentStart));
  if (!closingMatch) {
    return null;
  }

  const closeStart = contentStart + closingMatch.index;
  const closeEnd = closeStart + closingMatch[0].length;
  return {
    content: source.slice(contentStart, closeStart),
    end: closeEnd,
  };
}
