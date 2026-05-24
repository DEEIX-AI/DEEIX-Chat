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

const MARKDOWN_LITERAL_FRAGMENT_RE = /(```[\s\S]*?```|~~~[\s\S]*?~~~|`[^`\n]*`)/g;
const INLINE_DOLLAR_MATH_RE = /(^|[^\\$])\$([^$\n]{1,800})\$/g;
const ESCAPED_INLINE_DOLLAR_MATH_RE = /\\\$([^$\n]{1,400})\\\$/g;
const DISPLAY_DOLLAR_MATH_RE = /(\${2,})([\s\S]*?)(\1)/g;

function isMarkdownLiteralFragment(fragment: string): boolean {
  return fragment.startsWith("```") || fragment.startsWith("~~~") || fragment.startsWith("`");
}

function mapMarkdownTextFragments(source: string, transform: (fragment: string) => string): string {
  return source
    .split(MARKDOWN_LITERAL_FRAGMENT_RE)
    .map((fragment) => {
      if (!fragment || isMarkdownLiteralFragment(fragment)) {
        return fragment;
      }
      return transform(fragment);
    })
    .join("");
}

function looksLikeLatexMathContent(value: string): boolean {
  const trimmedValue = value.trim();
  if (!trimmedValue || /^\d+(?:[.,]\d+)?$/.test(trimmedValue)) {
    return false;
  }

  return (
    /\\[A-Za-z]+/.test(trimmedValue) ||
    /[\^_{}=<>+\-*/]/.test(trimmedValue) ||
    (trimmedValue.includes("|") && /[A-Za-z\\Α-ω]|[\^_{}=<>+\-*/]/.test(trimmedValue)) ||
    /^[A-Za-z]$/.test(trimmedValue) ||
    /[Α-ω]/.test(trimmedValue)
  );
}

function normalizeLatexPipes(mathContent: string): string {
  return mathContent.replace(/(^|[^\\])\|/g, "$1\\vert{}");
}

function isEscapedCharacter(source: string, index: number): boolean {
  let slashCount = 0;
  for (let cursor = index - 1; cursor >= 0 && source[cursor] === "\\"; cursor -= 1) {
    slashCount += 1;
  }
  return slashCount % 2 === 1;
}

function getDollarMathDelimiterLength(source: string, index: number): number {
  if (source[index] !== "$" || isEscapedCharacter(source, index) || source[index - 1] === "$") {
    return 0;
  }

  if (source[index + 1] === "$" && source[index + 2] !== "$") {
    return 2;
  }

  return source[index + 1] === "$" ? 0 : 1;
}

function normalizeDollarMathContent(mathContent: string, inline: boolean): string {
  const normalizedContent = inline ? mathContent.replace(/\s*\n\s*/g, " ") : mathContent;
  return normalizeLatexPipes(normalizedContent);
}

function normalizeDollarMathSegments(source: string): string {
  if (!source.includes("$")) {
    return source;
  }

  let normalizedSource = "";
  let consumedUntil = 0;

  for (let index = 0; index < source.length; index += 1) {
    const delimiterLength = getDollarMathDelimiterLength(source, index);
    if (!delimiterLength) {
      continue;
    }

    const openingDelimiterIndex = index;
    let closingDelimiterIndex = -1;
    for (let cursor = openingDelimiterIndex + delimiterLength; cursor < source.length; cursor += 1) {
      if (getDollarMathDelimiterLength(source, cursor) === delimiterLength) {
        closingDelimiterIndex = cursor;
        break;
      }
    }

    if (closingDelimiterIndex < 0) {
      break;
    }

    const mathContent = source.slice(openingDelimiterIndex + delimiterLength, closingDelimiterIndex);
    const inline = delimiterLength === 1;
    const shouldNormalize =
      (mathContent.includes("|") || (inline && mathContent.includes("\n"))) &&
      looksLikeLatexMathContent(mathContent);

    if (shouldNormalize) {
      normalizedSource += source.slice(consumedUntil, openingDelimiterIndex + delimiterLength);
      normalizedSource += normalizeDollarMathContent(mathContent, inline);
      normalizedSource += source.slice(closingDelimiterIndex, closingDelimiterIndex + delimiterLength);
      consumedUntil = closingDelimiterIndex + delimiterLength;
    }

    index = closingDelimiterIndex + delimiterLength - 1;
  }

  if (!consumedUntil) {
    return source;
  }

  return normalizedSource + source.slice(consumedUntil);
}

function normalizeLatexDelimitersInText(source: string): string {
  return source
    .replace(/\\\[\s*\n?([\s\S]*?)\n?\s*\\\]/g, (_, mathContent: string) => `$$\n${mathContent.trim()}\n$$`)
    .replace(/\\\(([\s\S]*?)\\\)/g, (_, mathContent: string) => `$${mathContent.trim()}$`)
    .replace(ESCAPED_INLINE_DOLLAR_MATH_RE, (match: string, mathContent: string) => {
      const trimmedMathContent = mathContent.trim();
      return looksLikeLatexMathContent(trimmedMathContent) ? `$${trimmedMathContent}$` : match;
    });
}

export function normalizeMathDelimiters(source: string): string {
  if (!source) {
    return source;
  }

  const shouldNormalizeDelimiters = source.includes("\\(") || source.includes("\\[") || source.includes("\\$");
  const hasDollarMath = source.includes("$");
  if (!shouldNormalizeDelimiters && !hasDollarMath) {
    return source;
  }

  return mapMarkdownTextFragments(source, (fragment) => {
    const normalizedFragment = shouldNormalizeDelimiters ? normalizeLatexDelimitersInText(fragment) : fragment;
    return normalizedFragment.includes("$") ? normalizeDollarMathSegments(normalizedFragment) : normalizedFragment;
  });
}

const LATEX_UNICODE_SYMBOLS: Array<[RegExp, string]> = [
  [/→/g, " \\to "],
  [/←/g, " \\leftarrow "],
  [/⇒/g, " \\Rightarrow "],
  [/⇐/g, " \\Leftarrow "],
  [/↔/g, " \\leftrightarrow "],
  [/⇔/g, " \\Leftrightarrow "],
];

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

  return mapMarkdownTextFragments(source, (fragment) => fragment.replace(THINKING_LIKE_HTML_TAG_RE, escapeHtmlTag));
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

  return mapMarkdownTextFragments(source, (fragment) =>
    fragment
      .replace(DISPLAY_DOLLAR_MATH_RE, (match, openingDelimiter: string, mathContent: string, closingDelimiter: string) => {
        if (!mathContent) {
          return match;
        }

        return `${openingDelimiter}${normalizeLatexSymbols(mathContent)}${closingDelimiter}`;
      })
      .replace(INLINE_DOLLAR_MATH_RE, (match: string, prefix: string, mathContent: string) => {
        if (!mathContent) {
          return match;
        }

        return `${prefix}$${normalizeLatexSymbols(mathContent)}$`;
      }),
  );
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
