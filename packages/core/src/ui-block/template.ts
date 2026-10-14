// Templates: strings with `{expression}` holes, evaluated against block state.
//
//   "{round(monthly)}"         one hole and nothing else: the raw value (number, list…)
//   "共 {years} 年"             mixed text: a string; numbers print with grouping and
//                               at most two decimals
//   "{{literal}}"              doubled braces are literal braces
//
// A template that references a point variable (`x`, `i`, `row`) the block does
// not define is not evaluated here: it is handed to the component, which
// evaluates it once per point (chart series, table filters).

import { type Expression, ExpressionError, type ExpressionScope, parseExpression, type UIValue } from "./expression.ts";

export const POINT_VARIABLES: readonly string[] = ["x", "i", "row"];

type Segment = { kind: "text"; text: string } | { kind: "expression"; expression: Expression };

export type Template = {
  readonly source: string;
  readonly variables: readonly string[];
  // True when the whole string is a single hole: the result keeps its type.
  readonly raw: boolean;
  // With `onError`, a failing hole in mixed text prints "—" and the rest of the
  // text stays; without it (and always for a single hole) the error is thrown.
  evaluate: (scope: ExpressionScope, onError?: (error: unknown) => void) => UIValue;
};

// A template the component evaluates per point, with the block's values bound.
export type PointTemplate = {
  readonly kind: "point-template";
  readonly source: string;
  evaluate: (point: Readonly<Record<string, UIValue>>) => UIValue;
};

const NUMBER_FORMAT = new Intl.NumberFormat("en-US", { maximumFractionDigits: 2 });

export function formatTemplateNumber(value: number): string {
  return Number.isFinite(value) ? NUMBER_FORMAT.format(value) : "—";
}


// Returns null for plain text (no holes). Throws ExpressionError when a hole
// does not parse or a brace is left open.
export function compileTemplate(source: string): Template | null {
  const segments = splitTemplate(source);
  if (!segments.some((segment) => segment.kind === "expression")) {
    return null;
  }
  const variables = new Set<string>();
  for (const segment of segments) {
    if (segment.kind === "expression") {
      for (const name of segment.expression.variables) {
        variables.add(name);
      }
    }
  }
  const raw = segments.length === 1;
  return {
    source,
    variables: Array.from(variables),
    raw,
    evaluate: (scope, onError) => {
      if (raw) {
        const [only] = segments;
        return only?.kind === "expression" ? only.expression.evaluate(scope) : null;
      }
      let text = "";
      for (const segment of segments) {
        if (segment.kind === "text") {
          text += segment.text;
          continue;
        }
        if (!onError) {
          text += templateText(segment.expression.evaluate(scope));
          continue;
        }
        try {
          text += templateText(segment.expression.evaluate(scope));
        } catch (error) {
          onError(error);
          text += "—";
        }
      }
      return text;
    },
  };
}

function templateText(value: UIValue): string {
  if (typeof value === "number") {
    return formatTemplateNumber(value);
  }
  if (typeof value === "string") {
    return value;
  }
  if (value === null) {
    return "";
  }
  if (typeof value === "boolean") {
    return String(value);
  }
  return JSON.stringify(value);
}

function splitTemplate(source: string): Segment[] {
  const segments: Segment[] = [];
  let text = "";
  let index = 0;
  while (index < source.length) {
    const char = source[index];
    if ((char === "{" || char === "}") && source[index + 1] === char) {
      text += char;
      index += 2;
      continue;
    }
    if (char === "}") {
      throw new ExpressionError("unmatched }");
    }
    if (char !== "{") {
      text += char;
      index += 1;
      continue;
    }
    const end = findHoleEnd(source, index + 1);
    if (end === -1) {
      throw new ExpressionError("unclosed {");
    }
    if (text) {
      segments.push({ kind: "text", text });
      text = "";
    }
    segments.push({ kind: "expression", expression: parseExpression(source.slice(index + 1, end)) });
    index = end + 1;
  }
  if (text) {
    segments.push({ kind: "text", text });
  }
  return segments;
}

// The closing brace of a hole, skipping braces inside quoted strings.
function findHoleEnd(source: string, start: number): number {
  let quote = "";
  for (let index = start; index < source.length; index += 1) {
    const char = source[index];
    if (quote) {
      if (char === "\\") {
        index += 1;
      } else if (char === quote) {
        quote = "";
      }
      continue;
    }
    if (char === "'" || char === '"') {
      quote = char;
    } else if (char === "}") {
      return index;
    } else if (char === "{") {
      return -1;
    }
  }
  return -1;
}

// `computed` entries are expressions without braces; models sometimes wrap
// them anyway (`"{a * 2}"`), which is unwrapped instead of failing.
export function unwrapExpressionSource(source: string): string {
  const trimmed = source.trim();
  if (trimmed.startsWith("{") && trimmed.endsWith("}") && !trimmed.startsWith("{{")) {
    return trimmed.slice(1, -1);
  }
  return trimmed;
}
