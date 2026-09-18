// 替代 turndown 默认的全量转义：只转义会改变 CommonMark / GFM 结构或格式的位置，使粘贴结果与页面显示一致。

// 与 turndown 一致，只在文本节点起始处判断块级标记。
const BLOCK_START_ESCAPES: ReadonlyArray<readonly [RegExp, string]> = [
  [/^-/, "\\-"],
  [/^\+ /, "\\+ "],
  [/^([*_])(?=\s|$|[*_ \t]*$)/, "\\$1"],
  [/^(=+)/, "\\$1"],
  [/^(#{1,6}) /, "\\$1 "],
  [/^~~~/, "\\~~~"],
  [/^>/, "\\>"],
  [/^(\d+)([.)]) /, "$1\\$2 "],
];

const ASCII_PUNCTUATION = /[!-/:-@[-`{-~]/;
const UNICODE_PUNCTUATION_OR_SYMBOL = /[\p{P}\p{S}]/u;
const DELIMITER_CHARS = new Set(["*", "_", "~"]);

type DelimiterRun = {
  char: string;
  start: number;
  end: number;
  canOpen: boolean;
  canClose: boolean;
};

function isWhitespace(char: string): boolean {
  return char === "" || /\s/u.test(char);
}

function isPunctuation(char: string): boolean {
  return char !== "" && (ASCII_PUNCTUATION.test(char) || UNICODE_PUNCTUATION_OR_SYMBOL.test(char));
}

// 按 CommonMark 6.2 的 left-/right-flanking 规则判断定界符串能否开启或关闭强调。
function classifyDelimiterRun(text: string, char: string, start: number, end: number): DelimiterRun {
  const before = start > 0 ? text[start - 1] : "";
  const after = end < text.length ? text[end] : "";
  const beforeWhitespace = isWhitespace(before);
  const afterWhitespace = isWhitespace(after);
  const beforePunctuation = isPunctuation(before);
  const afterPunctuation = isPunctuation(after);

  const leftFlanking = !afterWhitespace && (!afterPunctuation || beforeWhitespace || beforePunctuation);
  const rightFlanking = !beforeWhitespace && (!beforePunctuation || afterWhitespace || afterPunctuation);

  if (char === "_") {
    return {
      char,
      start,
      end,
      canOpen: leftFlanking && (!rightFlanking || beforePunctuation),
      canClose: rightFlanking && (!leftFlanking || afterPunctuation),
    };
  }
  return { char, start, end, canOpen: leftFlanking, canClose: rightFlanking };
}

function collectDelimiterRuns(text: string): DelimiterRun[] {
  const runs: DelimiterRun[] = [];
  let index = 0;
  while (index < text.length) {
    const char = text[index];
    if (!DELIMITER_CHARS.has(char)) {
      index += 1;
      continue;
    }
    let end = index + 1;
    while (end < text.length && text[end] === char) {
      end += 1;
    }
    runs.push(classifyDelimiterRun(text, char, index, end));
    index = end;
  }
  return runs;
}

// 仅当存在可配对的开启 / 关闭定界符时才转义。
function resolveEscapedDelimiterPositions(text: string): Set<number> {
  const runs = collectDelimiterRuns(text);
  const positions = new Set<number>();
  runs.forEach((run, runIndex) => {
    const hasLaterCloser = run.canOpen && runs.some((other, otherIndex) => otherIndex > runIndex && other.char === run.char && other.canClose);
    const hasEarlierOpener = run.canClose && runs.some((other, otherIndex) => otherIndex < runIndex && other.char === run.char && other.canOpen);
    if (!hasLaterCloser && !hasEarlierOpener) {
      return;
    }
    for (let position = run.start; position < run.end; position += 1) {
      positions.add(position);
    }
  });
  return positions;
}

function escapeInlineMarkdown(text: string): string {
  const delimiterPositions = resolveEscapedDelimiterPositions(text);
  let result = "";
  for (let index = 0; index < text.length; index += 1) {
    const char = text[index];
    const next = index + 1 < text.length ? text[index + 1] : "";
    if (char === "\\" && ASCII_PUNCTUATION.test(next)) {
      result += "\\\\";
      continue;
    }
    if (char === "`") {
      result += "\\`";
      continue;
    }
    if (char === "]" && (next === "(" || next === "[" || next === ":")) {
      result += "\\]";
      continue;
    }
    if (char === "[" && next === "^") {
      result += "\\[";
      continue;
    }
    if (delimiterPositions.has(index)) {
      result += `\\${char}`;
      continue;
    }
    result += char;
  }
  return result;
}

export function escapeStructuralMarkdown(text: string): string {
  const inlineEscaped = escapeInlineMarkdown(text);
  return BLOCK_START_ESCAPES.reduce((accumulator, [pattern, replacement]) => accumulator.replace(pattern, replacement), inlineEscaped);
}
