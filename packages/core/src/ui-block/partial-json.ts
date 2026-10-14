// Lenient JSON reader for interactive blocks that are still being written.
//
// Models stream a block token by token and routinely deviate from strict JSON:
// trailing commas, // and /* */ comments, unescaped double quotes inside text
// (typical when Chinese prose uses ASCII quotes). This reader accepts all of
// that and any prefix of a document, and reports what was actually finished:
//
//   - `closed` holds every object and array whose closing bracket was read, so a
//     renderer can wait for a leaf node to finish while showing its container.
//   - `openTail` maps a container that was cut off to the key (or index) of its
//     last member when that member itself was cut off mid-value.
//
// A quote inside a string ends the string only when the next significant
// character is structural (`,` `}` `]` `:`) or the input ends there.

export type PartialJSONResult = {
  value: unknown;
  // False when the input ended before the top-level value was finished.
  complete: boolean;
  closed: WeakSet<object>;
  openTail: WeakMap<object, string | number>;
};

// Far deeper than any block allows (the block limit is 8 levels of nodes); a
// hard stop keeps hostile input from exhausting the call stack.
export const MAX_JSON_DEPTH = 64;

const STRUCTURAL_AFTER_STRING = new Set([",", "}", "]", ":"]);

class Reader {
  private index = 0;
  private depth = 0;
  readonly closed = new WeakSet<object>();
  readonly openTail = new WeakMap<object, string | number>();

  private readonly text: string;

  constructor(text: string) {
    this.text = text;
  }

  read(): PartialJSONResult {
    this.skip();
    if (this.atEnd()) {
      return { value: undefined, complete: false, closed: this.closed, openTail: this.openTail };
    }
    const { value, complete } = this.value();
    return { value, complete, closed: this.closed, openTail: this.openTail };
  }

  private atEnd(): boolean {
    return this.index >= this.text.length;
  }

  private peek(): string {
    return this.text[this.index] ?? "";
  }

  // Whitespace and comments.
  private skip(): void {
    while (!this.atEnd()) {
      const char = this.peek();
      if (/\s/.test(char)) {
        this.index += 1;
      } else if (char === "/" && this.text[this.index + 1] === "/") {
        const end = this.text.indexOf("\n", this.index);
        this.index = end === -1 ? this.text.length : end + 1;
      } else if (char === "/" && this.text[this.index + 1] === "*") {
        const end = this.text.indexOf("*/", this.index + 2);
        this.index = end === -1 ? this.text.length : end + 2;
      } else {
        return;
      }
    }
  }

  private value(): { value: unknown; complete: boolean } {
    const char = this.peek();
    if (char === "{" || char === "[") {
      if (this.depth >= MAX_JSON_DEPTH) {
        throw new SyntaxError(`nesting deeper than ${MAX_JSON_DEPTH} levels`);
      }
      this.depth += 1;
      try {
        return char === "{" ? this.object() : this.array();
      } finally {
        this.depth -= 1;
      }
    }
    if (char === '"' || char === "'") {
      return this.string();
    }
    return this.scalar();
  }

  private object(): { value: unknown; complete: boolean } {
    const result: Record<string, unknown> = {};
    this.index += 1;
    for (;;) {
      this.skip();
      if (this.atEnd()) {
        return { value: result, complete: false };
      }
      const char = this.peek();
      if (char === "}") {
        this.index += 1;
        this.closed.add(result);
        return { value: result, complete: true };
      }
      if (char === ",") {
        this.index += 1;
        continue;
      }
      const key = char === '"' || char === "'" ? this.string() : this.bareKey();
      if (!key.complete || typeof key.value !== "string") {
        // Cut off inside the key: the member does not exist yet.
        return { value: result, complete: false };
      }
      this.skip();
      if (this.peek() !== ":") {
        if (this.atEnd()) {
          return { value: result, complete: false };
        }
        throw new SyntaxError(`expected ":" at ${this.index}`);
      }
      this.index += 1;
      this.skip();
      if (this.atEnd()) {
        return { value: result, complete: false };
      }
      const member = this.value();
      // Own-property assignment; "__proto__" stays a plain key.
      Object.defineProperty(result, key.value, { value: member.value, enumerable: true, writable: true, configurable: true });
      if (!member.complete) {
        this.openTail.set(result, key.value);
        return { value: result, complete: false };
      }
    }
  }

  private array(): { value: unknown; complete: boolean } {
    const result: unknown[] = [];
    this.index += 1;
    for (;;) {
      this.skip();
      if (this.atEnd()) {
        return { value: result, complete: false };
      }
      const char = this.peek();
      if (char === "]") {
        this.index += 1;
        this.closed.add(result);
        return { value: result, complete: true };
      }
      if (char === ",") {
        this.index += 1;
        continue;
      }
      const item = this.value();
      result.push(item.value);
      if (!item.complete) {
        this.openTail.set(result, result.length - 1);
        return { value: result, complete: false };
      }
    }
  }

  // Unquoted keys are tolerated; they end at the colon.
  private bareKey(): { value: unknown; complete: boolean } {
    const match = /^[A-Za-z_$][\w$-]*/.exec(this.text.slice(this.index));
    if (!match) {
      throw new SyntaxError(`unexpected "${this.peek()}" at ${this.index}`);
    }
    this.index += match[0].length;
    return { value: match[0], complete: !this.atEnd() };
  }

  private string(): { value: unknown; complete: boolean } {
    const quote = this.peek();
    let value = "";
    this.index += 1;
    while (!this.atEnd()) {
      const char = this.peek();
      if (char === "\\") {
        const escaped = this.escape();
        if (escaped === null) {
          return { value, complete: false };
        }
        value += escaped;
        continue;
      }
      if (char === quote) {
        if (this.quoteEndsString()) {
          this.index += 1;
          return { value, complete: true };
        }
        value += char;
        this.index += 1;
        continue;
      }
      value += char;
      this.index += 1;
    }
    return { value, complete: false };
  }

  private quoteEndsString(): boolean {
    let look = this.index + 1;
    while (look < this.text.length && /\s/.test(this.text[look] ?? "")) {
      look += 1;
    }
    const next = this.text[look];
    return next === undefined || STRUCTURAL_AFTER_STRING.has(next);
  }

  // Returns null when the input ends inside the escape sequence.
  private escape(): string | null {
    const kind = this.text[this.index + 1];
    if (kind === undefined) {
      this.index = this.text.length;
      return null;
    }
    const simple: Record<string, string> = { n: "\n", t: "\t", r: "\r", b: "\b", f: "\f", "/": "/", "\\": "\\", '"': '"', "'": "'" };
    if (kind === "u") {
      const hex = this.text.slice(this.index + 2, this.index + 6);
      if (hex.length < 4) {
        this.index = this.text.length;
        return null;
      }
      this.index += 6;
      return /^[0-9a-fA-F]{4}$/.test(hex) ? String.fromCharCode(Number.parseInt(hex, 16)) : "";
    }
    this.index += 2;
    return simple[kind] ?? kind;
  }

  private scalar(): { value: unknown; complete: boolean } {
    const match = /^(?:-?(?:\d+\.?\d*|\.\d+)(?:[eE][+-]?\d*)?|true|false|null|-)/.exec(this.text.slice(this.index));
    if (!match) {
      // A literal still being typed ("tr", "nul").
      const rest = this.text.slice(this.index);
      if (["true", "false", "null"].some((word) => word.startsWith(rest))) {
        this.index = this.text.length;
        return { value: undefined, complete: false };
      }
      throw new SyntaxError(`unexpected "${this.peek()}" at ${this.index}`);
    }
    const token = match[0];
    this.index += token.length;
    // A number at the very end may still grow.
    const complete = !this.atEnd();
    if (token === "true") {
      return { value: true, complete: true };
    }
    if (token === "false") {
      return { value: false, complete: true };
    }
    if (token === "null") {
      return { value: null, complete: true };
    }
    const number = Number(token);
    if (!Number.isFinite(number)) {
      return { value: undefined, complete: false };
    }
    return { value: number, complete };
  }
}

// Parses `text`, tolerating the deviations above and any truncation. Throws
// SyntaxError only for input no continuation could fix (a stray character).
export function parsePartialJSON(text: string): PartialJSONResult {
  return new Reader(text).read();
}
