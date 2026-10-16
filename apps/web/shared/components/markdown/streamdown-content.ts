import { isRecord } from "@/shared/lib/type-guards";

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

type ParseStreamdownSegmentsOptions = {
  normalizeHTMLVisualFences?: boolean;
  parseThinking?: boolean;
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

  if (isRecord(input)) {
    const maybeRecord = input;
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
const HTML_VISUAL_MARKDOWN_FENCE_RE = /(^|\n)([ \t]{0,3})(```|~~~)[ \t]*(?:(?:markdown|md)[^\n]*)?\n([\s\S]*?)\n[ \t]*\3[ \t]*(?=\n|$)/gi;
const HTML_VISUAL_FRAGMENT_RE = /^\s*<(?:div|section|article|aside|main|details|table)\b[\s\S]*<\/(?:div|section|article|aside|main|details|table)>\s*$/i;
const HTML_VISUAL_STYLE_RE = /\sstyle\s*=\s*["'][^"']{8,}["']/i;
const HTML_TAG_RE = /<\/?[A-Za-z][^>\n]*>/g;
const HTML_BLOCK_ROOT_RE = /(^|\n)([ \t]{0,3})<(article|aside|details|div|main|section|table)\b/gi;
const HTML_STRUCTURE_TAG_RE = /<!--[\s\S]*?-->|<\/?([A-Za-z][A-Za-z0-9-]*)\b(?:[^>"']|"[^"]*"|'[^']*')*>/g;
const HTML_BLANK_LINE_RE = /(\r?\n)[ \t]*(?=\r?\n)/g;
const INLINE_DOLLAR_MATH_RE = /(^|[^\\$])\$([^$\n]{1,800})\$/g;
const DISPLAY_DOLLAR_MATH_RE = /(\${2,})([\s\S]*?)(\1)/g;
const MARKDOWN_DISPLAY_MATH_RE = /(?:^|\n)\s*\$\$[\s\S]+?\$\$|\\\[[\s\S]+?\\\]|\\begin\{[a-z*]+\}/i;
// A newline followed by a blank line, list marker, heading, quote, table row or fence starts
// another block, and inline math cannot continue into it.
const INLINE_MATH_BLOCK_BOUNDARY_RE = /\n[ \t]*(?:\n|[-+*][ \t]|\d{1,9}[.)][ \t]|#{1,6}[ \t]|>|\||```|~~~)/;
const BARE_URL_RE = /\b(?:https?:\/\/|www\.)[^\s<>"'`]+/gi;

function isMarkdownLiteralFragment(fragment: string): boolean {
  return fragment.startsWith("```") || fragment.startsWith("~~~") || fragment.startsWith("`");
}

function mapMarkdownTextFragments(source: string, transform: (fragment: string, offset: number) => string): string {
  let offset = 0;
  return source
    .split(MARKDOWN_LITERAL_FRAGMENT_RE)
    .map((fragment) => {
      const fragmentOffset = offset;
      offset += fragment.length;
      if (!fragment || isMarkdownLiteralFragment(fragment)) {
        return fragment;
      }
      return transform(fragment, fragmentOffset);
    })
    .join("");
}

const MARKDOWN_LIST_ITEM_RE = /^[ \t]{0,3}(?:[-+*]|\d{1,9}[.)])(?:[ \t]|$)/;

function leadingColumns(line: string): number {
  let columns = 0;
  for (const character of line) {
    if (character === " ") {
      columns += 1;
    } else if (character === "\t") {
      columns += 4 - (columns % 4);
    } else {
      break;
    }
  }
  return columns;
}

// Indented code blocks: lines indented four columns after a blank line, outside lists, where an
// indented line continues the list item instead.
function findIndentedCodeRanges(source: string): Array<readonly [number, number]> {
  const ranges: Array<readonly [number, number]> = [];
  let lineStart = 0;
  let previousBlank = true;
  let inList = false;
  let inCode = false;
  for (const line of source.split("\n")) {
    const lineEnd = lineStart + line.length;
    if (line.trim() === "") {
      previousBlank = true;
    } else {
      const indented = leadingColumns(line) >= 4;
      inCode = indented && (inCode || (previousBlank && !inList));
      if (inCode) {
        ranges.push([lineStart, lineEnd]);
      } else if (!indented) {
        inList = MARKDOWN_LIST_ITEM_RE.test(line) || (inList && !(previousBlank && leadingColumns(line) === 0));
      }
      previousBlank = false;
    }
    lineStart = lineEnd + 1;
  }
  return ranges;
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

function isUnescapedDollar(source: string, index: number): boolean {
  return source[index] === "$" && !isEscapedCharacter(source, index);
}

function dollarRunLength(source: string, index: number): number {
  let cursor = index;
  while (source[cursor] === "$") {
    cursor += 1;
  }
  return cursor - index;
}

function longestDollarRun(source: string): number {
  let longest = 0;
  for (let index = source.indexOf("$"); index >= 0; ) {
    const run = dollarRunLength(source, index);
    longest = Math.max(longest, run);
    index = source.indexOf("$", index + run);
  }
  return longest;
}

const HTML_BLOCK_TAG_RE = /<\/?\s*(?:div|p|section|article|aside|main|blockquote|ul|ol|li|table|thead|tbody|tr|th|td|h[1-6]|pre|hr|details|summary|nav|header|footer|figure|figcaption)\b/i;

// Pandoc's tex_math_dollars rules decide whether a dollar closes inline math: the closing dollar
// must not be followed by a digit, so prices such as "$1/月，$2" or "$3 ($1 + $2)" stay text.
// Pandoc also rejects whitespace just inside the delimiters; models write "$ x^2 $", so padded
// content is accepted when it contains a command or an operator. A bare letter is not enough,
// or shell text such as "$A $B" would pair.
function closesInlineMath(content: string, following: string | undefined): boolean {
  return (
    content.trim() !== "" &&
    !/\d/.test(following ?? "") &&
    !INLINE_MATH_BLOCK_BOUNDARY_RE.test(content) &&
    !HTML_BLOCK_TAG_RE.test(content) &&
    (!/^\s|\s$/.test(content) || /\\[A-Za-z]+|[\^_{}=<>+\-*/]/.test(content))
  );
}

// TeX ends inline math at the first unescaped dollar, so when that dollar cannot close the
// formula the opening dollar is plain text.
function findInlineMathClose(source: string, open: number): number {
  for (let cursor = open + 1; cursor < source.length; cursor += 1) {
    if (!isUnescapedDollar(source, cursor)) {
      continue;
    }
    const closes =
      dollarRunLength(source, cursor) === 1 && closesInlineMath(source.slice(open + 1, cursor), source[cursor + 1]);
    return closes ? cursor : -1;
  }
  return -1;
}

function findDisplayMathClose(source: string, from: number): number {
  for (let cursor = from; cursor < source.length; ) {
    if (!isUnescapedDollar(source, cursor)) {
      cursor += 1;
      continue;
    }
    const run = dollarRunLength(source, cursor);
    if (run === 2) {
      return cursor;
    }
    cursor += run;
  }
  return -1;
}

type DollarMathSpan = {
  start: number;
  end: number;
  delimiterLength: number;
};

// Every pass that reads dollar math pairs delimiters here, so escaping, normalization and
// plugin detection agree on what is a formula. Runs of three or more dollars are never math.
function findDollarMathSpans(source: string): DollarMathSpan[] {
  const spans: DollarMathSpan[] = [];
  for (let index = 0; index < source.length; ) {
    if (!isUnescapedDollar(source, index)) {
      index += 1;
      continue;
    }
    const run = dollarRunLength(source, index);
    let close = -1;
    if (run === 1) {
      close = findInlineMathClose(source, index);
    } else if (run === 2) {
      close = findDisplayMathClose(source, index + run);
    }
    if (close < 0) {
      index += run;
      continue;
    }
    spans.push({ start: index, end: close + run, delimiterLength: run });
    index = close + run;
  }
  return spans;
}

// Takes ranges in order without overlaps. Callers ask about increasing indexes, so one cursor
// walks the ranges once instead of searching them for every dollar.
function createRangeMembership(ranges: ReadonlyArray<readonly [number, number]>): (index: number) => boolean {
  let cursor = 0;
  return (index) => {
    while (cursor < ranges.length && ranges[cursor][1] <= index) {
      cursor += 1;
    }
    return cursor < ranges.length && ranges[cursor][0] <= index;
  };
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

  for (const span of findDollarMathSpans(source)) {
    const inline = span.delimiterLength === 1;
    const mathContent = source.slice(span.start + span.delimiterLength, span.end - span.delimiterLength);
    const shouldNormalize =
      (mathContent.includes("|") || (inline && mathContent.includes("\n"))) && looksLikeLatexMathContent(mathContent);
    let nextContent = shouldNormalize ? normalizeDollarMathContent(mathContent, inline) : mathContent;
    let delimiter = "$".repeat(span.delimiterLength);

    // remark-math reads inline math like a code span and closes it at the dollar of \$.
    // A longer dollar run keeps \$ inside the formula, the same way longer backtick runs
    // let code spans contain backticks, and KaTeX still renders \$ as a dollar sign.
    if (inline && mathContent.includes("\\$")) {
      delimiter = "$".repeat(Math.max(2, longestDollarRun(mathContent) + 1));
      nextContent = `${nextContent.startsWith("$") ? " " : ""}${nextContent}${nextContent.endsWith("$") ? " " : ""}`;
    }

    if (nextContent === mathContent && delimiter.length === span.delimiterLength) {
      continue;
    }
    normalizedSource += source.slice(consumedUntil, span.start);
    normalizedSource += delimiter + nextContent + delimiter;
    consumedUntil = span.end;
  }

  if (!consumedUntil) {
    return source;
  }

  return normalizedSource + source.slice(consumedUntil);
}

// Some models write inline math as \$x+1\$. Pairs of escaped dollars outside existing formulas
// become math under the same closing rules, so escaped prices such as \$2 + \$3 stay text.
function normalizeEscapedDollarDelimiters(source: string): string {
  const insideFormula = createRangeMembership(findDollarMathSpans(source).map((span) => [span.start, span.end] as const));
  const backslashes: number[] = [];
  for (let index = source.indexOf("\\$"); index >= 0; index = source.indexOf("\\$", index + 2)) {
    if (!isEscapedCharacter(source, index) && !insideFormula(index)) {
      backslashes.push(index);
    }
  }

  let normalizedSource = "";
  let consumedUntil = 0;
  for (let pair = 0; pair + 1 < backslashes.length; ) {
    const open = backslashes[pair];
    const close = backslashes[pair + 1];
    const mathContent = source.slice(open + 2, close);
    if (
      mathContent.includes("$") ||
      !looksLikeLatexMathContent(mathContent) ||
      !closesInlineMath(mathContent, source[close + 2])
    ) {
      pair += 1;
      continue;
    }
    normalizedSource += `${source.slice(consumedUntil, open)}$${mathContent.trim()}$`;
    consumedUntil = close + 2;
    pair += 2;
  }

  return consumedUntil ? normalizedSource + source.slice(consumedUntil) : source;
}

function normalizeLatexDelimitersInText(source: string): string {
  const normalizedSource = source
    .replace(/\\\[\s*\n?([\s\S]*?)\n?\s*\\\]/g, (_, mathContent: string) => `$$\n${mathContent.trim()}\n$$`)
    .replace(/\\\(([\s\S]*?)\\\)/g, (_, mathContent: string) => `$${mathContent.trim()}$`);
  return normalizedSource.includes("\\$") ? normalizeEscapedDollarDelimiters(normalizedSource) : normalizedSource;
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

// remark-math pairs any two single dollars in a paragraph, so a price such as "$1/月" next to
// "$2" renders as a formula. Escape every single dollar that does not delimit math by the rules
// above. Code is skipped, including indented code blocks, and so are bare URLs, where an entity
// would end the autolink. Double dollars are left alone so remend can still close display math
// while it streams.
export function escapeNonMathDollars(source: string): string {
  if (!source.includes("$")) {
    return source;
  }

  const insideIndentedCode = createRangeMembership(findIndentedCodeRanges(source));
  return mapMarkdownTextFragments(source, (fragment, offset) => {
    if (!fragment.includes("$")) {
      return fragment;
    }
    const insideFormula = createRangeMembership(findDollarMathSpans(fragment).map((span) => [span.start, span.end] as const));
    const insideURL = createRangeMembership(
      Array.from(fragment.matchAll(BARE_URL_RE), (match) => [match.index, match.index + match[0].length] as const),
    );

    let escapedFragment = "";
    let consumedUntil = 0;
    for (let index = fragment.indexOf("$"); index >= 0; ) {
      const run = dollarRunLength(fragment, index);
      if (
        run === 1 &&
        !isEscapedCharacter(fragment, index) &&
        !insideFormula(index) &&
        !insideURL(index) &&
        !insideIndentedCode(offset + index)
      ) {
        escapedFragment += `${fragment.slice(consumedUntil, index)}&#36;`;
        consumedUntil = index + 1;
      }
      index = fragment.indexOf("$", index + run);
    }
    return consumedUntil ? escapedFragment + fragment.slice(consumedUntil) : fragment;
  });
}

export function containsMarkdownMath(source: string): boolean {
  if (!source.includes("$") && !source.includes("\\")) {
    return false;
  }

  return MARKDOWN_DISPLAY_MATH_RE.test(source) || findDollarMathSpans(source).length > 0;
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

export function normalizeEscapedHTMLAttributeQuotes(source: string): string {
  if (!source.includes('\\"') && !source.includes("\\'")) {
    return source;
  }

  return mapMarkdownTextFragments(source, (fragment) =>
    fragment.replace(HTML_TAG_RE, (tag) => tag.replace(/\\"/g, '"').replace(/\\'/g, "'")),
  );
}

export function normalizeHTMLVisualMarkdownFences(source: string): string {
  if (!source.includes("```") && !source.includes("~~~")) {
    return source;
  }

  return source.replace(
    HTML_VISUAL_MARKDOWN_FENCE_RE,
    (match: string, prefix: string, _indent: string, _fence: string, code: string) => {
      const trimmedCode = code.trim();
      if (!HTML_VISUAL_FRAGMENT_RE.test(trimmedCode) || !HTML_VISUAL_STYLE_RE.test(trimmedCode)) {
        return match;
      }
      return `${prefix}${trimmedCode}`;
    },
  );
}

type CodeFenceEvent = {
  fenceStart: number;
  fenceEnd: number;
  marker: string;
  info: string;
};

type CodeFenceUpgrade = {
  openIdx: number;
  closeIdx: number;
  markerLength: number;
};

const CODE_FENCE_EVENT_RE = /(^|\n)([ \t]{0,3})(`{3,}|~{3,})([^\n]*)/g;

// A tagged fence (e.g. ```markdown) cannot contain another same-char tagged fence
// (e.g. ```text): CommonMark closes the outer block at the first bare fence, so
// the remainder of the document leaks out and renders as regular markdown. Models
// frequently emit this "nested fence" shape when asked to output a full markdown
// document containing example code blocks. When a tagged fence's intended body
// contains a same-char tagged inner fence and closes LIFO, upgrade the outer
// marker (and its closer) to a longer run so the nesting parses as authored.
export function normalizeNestedCodeFences(source: string): string {
  if (!source.includes("```") && !source.includes("~~~")) {
    return source;
  }

  CODE_FENCE_EVENT_RE.lastIndex = 0;
  const events: CodeFenceEvent[] = [];
  for (let match = CODE_FENCE_EVENT_RE.exec(source); match; match = CODE_FENCE_EVENT_RE.exec(source)) {
    events.push({
      fenceStart: match.index + match[1].length + match[2].length,
      fenceEnd: match.index + match[0].length,
      marker: match[3],
      info: match[4] ?? "",
    });
  }

  const upgrades: CodeFenceUpgrade[] = [];
  let index = 0;
  while (index < events.length) {
    const open = events[index];
    const openInfo = open.info.trim();
    if (openInfo === "" || openInfo.startsWith("`")) {
      index += 1;
      continue;
    }

    let depth = 0;
    let closer = -1;
    let innerTagged = false;
    let hasLongerInnerMarker = false;
    let maxInnerMarker = open.marker.length;
    for (let cursor = index + 1; cursor < events.length; cursor += 1) {
      const event = events[cursor];
      if (event.marker[0] !== open.marker[0] || event.marker.length < open.marker.length) {
        continue;
      }
      if (event.info.trim() === "") {
        if (depth === 0) {
          closer = cursor;
          break;
        }
        depth -= 1;
      } else {
        innerTagged = true;
        // A longer inner marker is already explicit syntax; leave this shape
        // untouched instead of guessing that the outer fence should grow.
        hasLongerInnerMarker ||= event.marker.length > open.marker.length;
        depth += 1;
        maxInnerMarker = Math.max(maxInnerMarker, event.marker.length);
      }
    }

    if (closer > 0 && innerTagged && !hasLongerInnerMarker) {
      upgrades.push({ openIdx: index, closeIdx: closer, markerLength: maxInnerMarker + 1 });
      index = closer + 1;
      continue;
    }
    index += 1;
  }

  if (upgrades.length === 0) {
    return source;
  }

  const parts: string[] = [];
  let cursor = 0;
  for (const upgrade of upgrades) {
    const open = events[upgrade.openIdx];
    const close = events[upgrade.closeIdx];
    const markerChar = open.marker[0];
    const upgradedMarker = markerChar.repeat(upgrade.markerLength);
    parts.push(source.slice(cursor, open.fenceStart));
    parts.push(upgradedMarker + open.info);
    cursor = open.fenceEnd;
    parts.push(source.slice(cursor, close.fenceStart));
    parts.push(upgradedMarker + close.info);
    cursor = close.fenceEnd;
  }
  parts.push(source.slice(cursor));
  return parts.join("");
}

function findHTMLBlockEnd(
  source: string,
  start: number,
  rootTagName: string,
  streaming: boolean,
): number | null {
  const tagPattern = new RegExp(HTML_STRUCTURE_TAG_RE.source, HTML_STRUCTURE_TAG_RE.flags);
  tagPattern.lastIndex = start;
  const openingTag = tagPattern.exec(source);
  if (
    !openingTag ||
    openingTag.index !== start ||
    openingTag[0].startsWith("</") ||
    openingTag[1]?.toLowerCase() !== rootTagName
  ) {
    return null;
  }

  let depth = /\/\s*>$/.test(openingTag[0]) ? 0 : 1;
  if (depth === 0) {
    return tagPattern.lastIndex;
  }

  for (let match = tagPattern.exec(source); match; match = tagPattern.exec(source)) {
    if (!match[1] || match[1].toLowerCase() !== rootTagName) {
      continue;
    }

    if (match[0].startsWith("</")) {
      depth -= 1;
      if (depth === 0) {
        return tagPattern.lastIndex;
      }
    } else if (!/\/\s*>$/.test(match[0])) {
      depth += 1;
    }
  }

  return streaming && depth > 0 ? source.length : null;
}

function isInsideOpenMarkdownFence(source: string, index: number): boolean {
  const fencePattern = /(^|\n)[ \t]{0,3}(`{3,}|~{3,})([^\n]*)/g;
  let opening: { character: string; length: number } | null = null;

  for (const match of source.slice(0, index).matchAll(fencePattern)) {
    const marker = match[2];
    if (!opening) {
      opening = { character: marker[0], length: marker.length };
      continue;
    }
    if (
      marker[0] === opening.character &&
      marker.length >= opening.length &&
      match[3].trim() === ""
    ) {
      opening = null;
    }
  }

  return opening != null;
}

export function normalizeHTMLBlockBlankLines(source: string, streaming = false): string {
  if (!source || !HTML_BLANK_LINE_RE.test(source)) {
    HTML_BLANK_LINE_RE.lastIndex = 0;
    return source;
  }
  HTML_BLANK_LINE_RE.lastIndex = 0;

  return mapMarkdownTextFragments(source, (fragment) => {
    const rootPattern = new RegExp(HTML_BLOCK_ROOT_RE.source, HTML_BLOCK_ROOT_RE.flags);
    const output: string[] = [];
    let cursor = 0;

    for (let match = rootPattern.exec(fragment); match; match = rootPattern.exec(fragment)) {
      const blockStart = match.index + match[1].length + match[2].length;
      if (blockStart < cursor || isInsideOpenMarkdownFence(fragment, blockStart)) {
        continue;
      }

      const blockEnd = findHTMLBlockEnd(fragment, blockStart, match[3].toLowerCase(), streaming);
      if (blockEnd == null) {
        continue;
      }

      output.push(fragment.slice(cursor, blockStart));
      output.push(
        fragment
          .slice(blockStart, blockEnd)
          .replace(HTML_BLANK_LINE_RE, "$1<!-- -->"),
      );
      cursor = blockEnd;
      rootPattern.lastIndex = blockEnd;
    }

    if (cursor === 0) {
      return fragment;
    }
    output.push(fragment.slice(cursor));
    return output.join("");
  });
}

export function parseStreamdownSegments(
  source: string,
  { normalizeHTMLVisualFences = true, parseThinking = true }: ParseStreamdownSegmentsOptions = {},
): RenderSegment[] {
  if (!source) {
    return [];
  }

  const normalizedSource = normalizeHTMLVisualFences ? normalizeHTMLVisualMarkdownFences(source) : source;
  const segments: RenderSegment[] = [];

  const thinkingBlock = parseThinking ? parseLeadingThinkingBlock(normalizedSource) : null;
  if (!thinkingBlock) {
    if (normalizedSource.trim()) {
      segments.push({
        type: "markdown",
        content: escapeThinkingLikeHtmlTags(normalizedSource),
      });
    }
    return segments;
  }

  segments.push({
    type: "thinking",
    content: thinkingBlock.content,
    incomplete: thinkingBlock.incomplete,
  });

  const tail = normalizedSource.slice(thinkingBlock.end);
  if (tail.trim()) {
    segments.push({
      type: "markdown",
      content: escapeThinkingLikeHtmlTags(tail),
    });
  }

  return segments;
}

function parseLeadingThinkingBlock(
  source: string,
): { content: string; end: number; incomplete: boolean } | null {
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
    return {
      content: source.slice(contentStart),
      end: source.length,
      incomplete: true,
    };
  }

  const closeStart = contentStart + closingMatch.index;
  const closeEnd = closeStart + closingMatch[0].length;
  return {
    content: source.slice(contentStart, closeStart),
    end: closeEnd,
    incomplete: false,
  };
}
