// Expression language for model-written formulas in interactive blocks.
//
// Whitelisted syntax, parsed to an AST and interpreted; never touches `eval` or
// `Function`. Values are JSON values. Grammar, loosest to tightest:
//
//   cond ? a : b      ||      &&      == !=      < <= > >=      + -      * / %
//   unary ! -         ^ (right-associative; -2^2 is -(2^2))      a.b  a[i]  f(x)
//
// Literals: numbers, 'strings' or "strings", true / false / null, [a, b].
// `+` concatenates when either side is a string. Member access reads own
// properties only and rejects __proto__ / constructor / prototype.
//
// Every evaluation draws from a step budget, so `range`, `sum` and nested
// formulas cannot run away. There is no assignment, loop or user function:
// imperative logic belongs in an Artifact.

export type UIValue = number | string | boolean | null | UIValue[] | { [key: string]: UIValue };

export class ExpressionError extends Error {
  override name = "ExpressionError";
}

const MAX_EXPRESSION_LENGTH = 512;
const MAX_RANGE_POINTS = 1000;
// Default step budget for one standalone evaluation. Block resolution shares one
// larger budget across all of its formulas instead.
const DEFAULT_STEPS = 100_000;

const FORBIDDEN_MEMBERS = new Set(["__proto__", "constructor", "prototype"]);
const KEYWORDS: Record<string, UIValue> = { true: true, false: false, null: null };
const CONSTANTS: Record<string, number> = { pi: Math.PI, e: Math.E };

type Token =
  | { kind: "number"; value: number }
  | { kind: "string"; value: string }
  | { kind: "ident"; value: string }
  | { kind: "op"; value: string }
  | { kind: "punct"; value: "(" | ")" | "[" | "]" | "," | "." | "?" | ":" }
  | { kind: "end" };

type Node =
  | { kind: "literal"; value: UIValue }
  | { kind: "array"; items: Node[] }
  | { kind: "var"; name: string }
  | { kind: "unary"; op: "-" | "!"; operand: Node }
  | { kind: "binary"; op: string; left: Node; right: Node }
  | { kind: "logical"; op: "&&" | "||"; left: Node; right: Node }
  | { kind: "conditional"; test: Node; then: Node; otherwise: Node }
  | { kind: "member"; object: Node; property: Node; computed: boolean }
  | { kind: "call"; name: string; args: Node[] };

const BINARY_PRECEDENCE: Record<string, number> = {
  "||": 1,
  "&&": 2,
  "==": 3,
  "!=": 3,
  "<": 4,
  "<=": 4,
  ">": 4,
  ">=": 4,
  "+": 5,
  "-": 5,
  "*": 6,
  "/": 6,
  "%": 6,
  "^": 8,
};
const POWER_PRECEDENCE = 8;

// Operators models write out of JavaScript habit are read as their equivalents.
const OPERATOR_ALIASES: Record<string, string> = { "===": "==", "!==": "!=" };
const OPERATORS = ["===", "!==", "==", "!=", "<=", ">=", "&&", "||", "<", ">", "+", "-", "*", "/", "%", "^", "!"];

function tokenize(source: string): Token[] {
  if (source.length > MAX_EXPRESSION_LENGTH) {
    throw new ExpressionError("expression too long");
  }
  const tokens: Token[] = [];
  let index = 0;
  while (index < source.length) {
    const char = source[index] ?? "";
    const previous = tokens[tokens.length - 1];
    if (/\s/.test(char)) {
      index += 1;
      continue;
    }
    // A dot after a value is member access; anywhere else it starts a number (.5).
    const dotIsMember = char === "." && previous !== undefined && isValueEnd(previous);
    if (/[0-9]/.test(char) || (char === "." && !dotIsMember && /[0-9]/.test(source[index + 1] ?? ""))) {
      const match = /^(\d+\.?\d*|\.\d+)(e[+-]?\d+)?/i.exec(source.slice(index));
      if (!match) {
        throw new ExpressionError(`bad number at ${index}`);
      }
      tokens.push({ kind: "number", value: Number(match[0]) });
      index += match[0].length;
      continue;
    }
    if (/[a-zA-Z_]/.test(char)) {
      const value = /^[a-zA-Z_][a-zA-Z0-9_]*/.exec(source.slice(index))?.[0] ?? char;
      tokens.push({ kind: "ident", value });
      index += value.length;
      continue;
    }
    if (char === "'" || char === '"') {
      const { value, end } = readString(source, index);
      tokens.push({ kind: "string", value });
      index = end;
      continue;
    }
    if ("()[],.?:".includes(char)) {
      tokens.push({ kind: "punct", value: char as "(" | ")" | "[" | "]" | "," | "." | "?" | ":" });
      index += 1;
      continue;
    }
    const operator = OPERATORS.find((candidate) => source.startsWith(candidate, index));
    if (operator) {
      tokens.push({ kind: "op", value: OPERATOR_ALIASES[operator] ?? operator });
      index += operator.length;
      continue;
    }
    throw new ExpressionError(`unexpected "${char}" at ${index}`);
  }
  tokens.push({ kind: "end" });
  return tokens;
}

function isValueEnd(token: Token): boolean {
  return (
    token.kind === "ident" ||
    token.kind === "number" ||
    token.kind === "string" ||
    (token.kind === "punct" && (token.value === ")" || token.value === "]"))
  );
}

function readString(source: string, start: number): { value: string; end: number } {
  const quote = source[start];
  let value = "";
  let index = start + 1;
  while (index < source.length) {
    const char = source[index] ?? "";
    if (char === "\\") {
      const next = source[index + 1] ?? "";
      value += next === "n" ? "\n" : next === "t" ? "\t" : next;
      index += 2;
      continue;
    }
    if (char === quote) {
      return { value, end: index + 1 };
    }
    value += char;
    index += 1;
  }
  throw new ExpressionError("unterminated string");
}

class Parser {
  private index = 0;
  private readonly tokens: Token[];

  constructor(tokens: Token[]) {
    this.tokens = tokens;
  }

  parse(): Node {
    const node = this.conditional();
    if (this.peek().kind !== "end") {
      throw new ExpressionError("unexpected trailing input");
    }
    return node;
  }

  private peek(): Token {
    return this.tokens[this.index] ?? { kind: "end" };
  }

  private next(): Token {
    const token = this.peek();
    this.index += 1;
    return token;
  }

  private isPunct(value: string): boolean {
    const token = this.peek();
    return token.kind === "punct" && token.value === value;
  }

  private expect(value: string): void {
    const token = this.next();
    if (token.kind !== "punct" || token.value !== value) {
      throw new ExpressionError(`expected ${value}`);
    }
  }

  private conditional(): Node {
    const test = this.binary(0);
    if (!this.isPunct("?")) {
      return test;
    }
    this.next();
    const then = this.conditional();
    this.expect(":");
    const otherwise = this.conditional();
    return { kind: "conditional", test, then, otherwise };
  }

  private binary(minPrecedence: number): Node {
    let left = this.unary();
    for (;;) {
      const token = this.peek();
      if (token.kind !== "op") {
        return left;
      }
      const precedence = BINARY_PRECEDENCE[token.value];
      if (precedence === undefined || precedence < minPrecedence) {
        return left;
      }
      this.next();
      const right = this.binary(token.value === "^" ? precedence : precedence + 1);
      left =
        token.value === "&&" || token.value === "||"
          ? { kind: "logical", op: token.value, left, right }
          : { kind: "binary", op: token.value, left, right };
    }
  }

  private unary(): Node {
    const token = this.peek();
    if (token.kind === "op" && token.value === "-") {
      this.next();
      // Unary minus binds looser than ^, so -2^2 is -(2^2) as in most math tools.
      return { kind: "unary", op: "-", operand: this.binary(POWER_PRECEDENCE) };
    }
    if (token.kind === "op" && token.value === "!") {
      this.next();
      return { kind: "unary", op: "!", operand: this.unary() };
    }
    if (token.kind === "op" && token.value === "+") {
      this.next();
      return this.unary();
    }
    return this.postfix(this.primary());
  }

  private postfix(node: Node): Node {
    let current = node;
    for (;;) {
      if (this.isPunct(".")) {
        this.next();
        const name = this.next();
        if (name.kind !== "ident") {
          throw new ExpressionError("expected property name");
        }
        current = { kind: "member", object: current, property: { kind: "literal", value: name.value }, computed: false };
      } else if (this.isPunct("[")) {
        this.next();
        const property = this.conditional();
        this.expect("]");
        current = { kind: "member", object: current, property, computed: true };
      } else {
        return current;
      }
    }
  }

  private primary(): Node {
    const token = this.next();
    switch (token.kind) {
      case "number":
      case "string":
        return { kind: "literal", value: token.value };
      case "ident": {
        if (this.isPunct("(")) {
          this.next();
          const args = this.list(")");
          if (!Object.hasOwn(FUNCTIONS, token.value)) {
            throw new ExpressionError(`unknown function ${token.value}`);
          }
          return { kind: "call", name: token.value, args };
        }
        if (Object.hasOwn(KEYWORDS, token.value)) {
          return { kind: "literal", value: KEYWORDS[token.value] ?? null };
        }
        return { kind: "var", name: token.value };
      }
      case "punct":
        if (token.value === "(") {
          const node = this.conditional();
          this.expect(")");
          return node;
        }
        if (token.value === "[") {
          return { kind: "array", items: this.list("]") };
        }
        throw new ExpressionError(`unexpected "${token.value}"`);
      default:
        throw new ExpressionError("unexpected end of expression");
    }
  }

  private list(close: ")" | "]"): Node[] {
    const items: Node[] = [];
    if (this.isPunct(close)) {
      this.next();
      return items;
    }
    items.push(this.conditional());
    while (this.isPunct(",")) {
      this.next();
      items.push(this.conditional());
    }
    this.expect(close);
    return items;
  }
}

// ---- Evaluation ----

export type ExpressionBudget = { steps: number };

export type ExpressionScope = {
  // Returns undefined for a name the scope does not define.
  lookup: (name: string) => UIValue | undefined;
  budget: ExpressionBudget;
};

export function createBudget(steps = DEFAULT_STEPS): ExpressionBudget {
  return { steps };
}

function spend(budget: ExpressionBudget, amount = 1): void {
  budget.steps -= amount;
  if (budget.steps < 0) {
    throw new ExpressionError("expression budget exceeded");
  }
}

function evaluateNode(node: Node, scope: ExpressionScope): UIValue {
  spend(scope.budget);
  switch (node.kind) {
    case "literal":
      return node.value;
    case "array":
      return node.items.map((item) => evaluateNode(item, scope));
    case "var": {
      const value = scope.lookup(node.name);
      if (value !== undefined) {
        return value;
      }
      const constant = CONSTANTS[node.name];
      if (constant !== undefined) {
        return constant;
      }
      throw new ExpressionError(`unknown variable ${node.name}`);
    }
    case "unary": {
      const operand = evaluateNode(node.operand, scope);
      return node.op === "!" ? !truthy(operand) : -toNumber(operand, "-");
    }
    case "logical": {
      const left = evaluateNode(node.left, scope);
      if (node.op === "&&") {
        return truthy(left) ? evaluateNode(node.right, scope) : left;
      }
      return truthy(left) ? left : evaluateNode(node.right, scope);
    }
    case "conditional":
      return truthy(evaluateNode(node.test, scope)) ? evaluateNode(node.then, scope) : evaluateNode(node.otherwise, scope);
    case "binary":
      return evaluateBinary(node.op, evaluateNode(node.left, scope), evaluateNode(node.right, scope));
    case "member":
      return readMember(evaluateNode(node.object, scope), evaluateNode(node.property, scope));
    case "call": {
      const fn = FUNCTIONS[node.name];
      if (!fn) {
        throw new ExpressionError(`unknown function ${node.name}`);
      }
      return fn(
        node.args.map((arg) => evaluateNode(arg, scope)),
        scope.budget,
      );
    }
  }
}

function evaluateBinary(op: string, left: UIValue, right: UIValue): UIValue {
  switch (op) {
    case "+":
      if (typeof left === "string" || typeof right === "string") {
        return stringify(left) + stringify(right);
      }
      return toNumber(left, op) + toNumber(right, op);
    case "-":
      return toNumber(left, op) - toNumber(right, op);
    case "*":
      return toNumber(left, op) * toNumber(right, op);
    case "/":
      return toNumber(left, op) / toNumber(right, op);
    case "%":
      return toNumber(left, op) % toNumber(right, op);
    case "^":
      return toNumber(left, op) ** toNumber(right, op);
    case "==":
      return valuesEqual(left, right);
    case "!=":
      return !valuesEqual(left, right);
    case "<":
    case "<=":
    case ">":
    case ">=":
      return compare(op, left, right);
    default:
      throw new ExpressionError(`unknown operator ${op}`);
  }
}

function compare(op: string, left: UIValue, right: UIValue): boolean {
  if (typeof left === "string" && typeof right === "string") {
    return op === "<" ? left < right : op === "<=" ? left <= right : op === ">" ? left > right : left >= right;
  }
  const a = toNumber(left, op);
  const b = toNumber(right, op);
  return op === "<" ? a < b : op === "<=" ? a <= b : op === ">" ? a > b : a >= b;
}

function readMember(object: UIValue, property: UIValue): UIValue {
  if (typeof property === "string" && FORBIDDEN_MEMBERS.has(property)) {
    throw new ExpressionError(`property ${property} is not allowed`);
  }
  if (Array.isArray(object)) {
    if (property === "length") {
      return object.length;
    }
    if (typeof property === "number" && Number.isInteger(property) && property >= 0 && property < object.length) {
      return object[property] ?? null;
    }
    throw new ExpressionError("index out of range");
  }
  if (typeof object === "string") {
    if (property === "length") {
      return object.length;
    }
    throw new ExpressionError("strings only have length");
  }
  if (object !== null && typeof object === "object" && typeof property === "string") {
    if (!Object.hasOwn(object, property)) {
      throw new ExpressionError(`unknown property ${property}`);
    }
    return object[property] ?? null;
  }
  throw new ExpressionError("cannot read a property here");
}

export function truthy(value: UIValue): boolean {
  if (value === null || value === false || value === 0 || value === "") {
    return false;
  }
  return !(typeof value === "number" && Number.isNaN(value));
}

function toNumber(value: UIValue, context: string): number {
  if (typeof value === "number") {
    return value;
  }
  throw new ExpressionError(`${context} expects numbers`);
}

function stringify(value: UIValue): string {
  if (typeof value === "string") {
    return value;
  }
  if (typeof value === "number" || typeof value === "boolean") {
    return String(value);
  }
  if (value === null) {
    return "";
  }
  return JSON.stringify(value);
}

export function valuesEqual(left: UIValue, right: UIValue): boolean {
  if (left === right) {
    return true;
  }
  if (Array.isArray(left) && Array.isArray(right)) {
    return left.length === right.length && left.every((item, index) => valuesEqual(item, right[index] ?? null));
  }
  if (isPlainValueObject(left) && isPlainValueObject(right)) {
    const keys = Object.keys(left);
    return keys.length === Object.keys(right).length && keys.every((key) => Object.hasOwn(right, key) && valuesEqual(left[key] ?? null, right[key] ?? null));
  }
  return false;
}

function isPlainValueObject(value: UIValue): value is { [key: string]: UIValue } {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

// ---- Functions ----

type Fn = (args: UIValue[], budget: ExpressionBudget) => UIValue;

function numbers(name: string, args: UIValue[], min: number, max = min): number[] {
  if (args.length < min || args.length > max) {
    throw new ExpressionError(`${name} takes ${min === max ? min : `${min}-${max}`} arguments`);
  }
  return args.map((arg) => toNumber(arg, name));
}

function math(name: string, fn: (value: number) => number): Fn {
  return (args) => fn(numbers(name, args, 1)[0] ?? Number.NaN);
}

function list(name: string, value: UIValue | undefined, budget: ExpressionBudget): number[] {
  if (!Array.isArray(value)) {
    throw new ExpressionError(`${name} expects a list`);
  }
  spend(budget, value.length);
  return value.map((item) => toNumber(item, name));
}

// min / max take either numbers or a single list.
function extreme(name: string, pick: (values: number[]) => number): Fn {
  return (args, budget) => {
    const values = args.length === 1 && Array.isArray(args[0]) ? list(name, args[0], budget) : numbers(name, args, 1, 64);
    if (values.length === 0) {
      throw new ExpressionError(`${name} of an empty list`);
    }
    return pick(values);
  };
}

const FUNCTIONS: Record<string, Fn> = {
  abs: math("abs", Math.abs),
  sqrt: math("sqrt", Math.sqrt),
  cbrt: math("cbrt", Math.cbrt),
  floor: math("floor", Math.floor),
  ceil: math("ceil", Math.ceil),
  sign: math("sign", Math.sign),
  log: math("log", Math.log),
  ln: math("ln", Math.log),
  log10: math("log10", Math.log10),
  log2: math("log2", Math.log2),
  exp: math("exp", Math.exp),
  sin: math("sin", Math.sin),
  cos: math("cos", Math.cos),
  tan: math("tan", Math.tan),
  asin: math("asin", Math.asin),
  acos: math("acos", Math.acos),
  atan: math("atan", Math.atan),
  sinh: math("sinh", Math.sinh),
  cosh: math("cosh", Math.cosh),
  tanh: math("tanh", Math.tanh),
  pow: (args) => {
    const [base = Number.NaN, exponent = Number.NaN] = numbers("pow", args, 2);
    return base ** exponent;
  },
  min: extreme("min", (values) => Math.min(...values)),
  max: extreme("max", (values) => Math.max(...values)),
  round: (args) => {
    const [value = Number.NaN, digits = 0] = numbers("round", args, 1, 2);
    const factor = 10 ** Math.trunc(digits);
    return Math.round(value * factor) / factor;
  },
  fixed: (args) => {
    const [value = Number.NaN, digits = 0] = numbers("fixed", args, 1, 2);
    return Number.isFinite(value) ? value.toFixed(Math.min(10, Math.max(0, Math.trunc(digits)))) : "—";
  },
  clamp: (args) => {
    const [value = Number.NaN, low = Number.NaN, high = Number.NaN] = numbers("clamp", args, 3);
    return Math.min(high, Math.max(low, value));
  },
  // range(start, end, step = ±1), both ends included: range(1, years) is every year.
  range: (args, budget) => {
    const [start = 0, end = 0, given] = numbers("range", args, 2, 3);
    const step = given ?? (end >= start ? 1 : -1);
    if (!Number.isFinite(start) || !Number.isFinite(end) || !Number.isFinite(step) || step === 0 || (end - start) / step < 0) {
      throw new ExpressionError("range needs a step that moves from start to end");
    }
    const count = Math.floor((end - start) / step + 1e-9) + 1;
    if (count > MAX_RANGE_POINTS) {
      throw new ExpressionError(`range is limited to ${MAX_RANGE_POINTS} points`);
    }
    spend(budget, count);
    return Array.from({ length: count }, (_, index) => roundFloat(start + index * step));
  },
  sum: (args, budget) => list("sum", args[0], budget).reduce((total, value) => total + value, 0),
  avg: (args, budget) => {
    const values = list("avg", args[0], budget);
    if (values.length === 0) {
      throw new ExpressionError("avg of an empty list");
    }
    return values.reduce((total, value) => total + value, 0) / values.length;
  },
  len: (args) => {
    const [value] = args;
    if (Array.isArray(value) || typeof value === "string") {
      return value.length;
    }
    throw new ExpressionError("len expects a list or text");
  },
  contains: (args, budget) => {
    const [container, needle = null] = args;
    if (typeof container === "string") {
      return container.includes(stringify(needle));
    }
    if (Array.isArray(container)) {
      spend(budget, container.length);
      return container.some((item) => valuesEqual(item, needle));
    }
    throw new ExpressionError("contains expects a list or text");
  },
};

// Steps like 0.1 accumulate binary noise (0.30000000000000004); trim it so
// range values print and compare the way they were written.
function roundFloat(value: number): number {
  return Math.round(value * 1e10) / 1e10;
}

export const EXPRESSION_FUNCTIONS: readonly string[] = Object.keys(FUNCTIONS);

// ---- Compilation ----

export type Expression = {
  readonly source: string;
  // Names the expression reads from its scope (constants and functions excluded).
  readonly variables: readonly string[];
  evaluate: (scope: ExpressionScope) => UIValue;
};

export function parseExpression(source: string): Expression {
  const ast = new Parser(tokenize(source)).parse();
  const names = new Set<string>();
  collectVariables(ast, names);
  return {
    source,
    variables: Array.from(names),
    evaluate: (scope) => evaluateNode(ast, scope),
  };
}

function collectVariables(node: Node, names: Set<string>): void {
  switch (node.kind) {
    case "var":
      if (!Object.hasOwn(CONSTANTS, node.name)) {
        names.add(node.name);
      }
      return;
    case "array":
      for (const item of node.items) {
        collectVariables(item, names);
      }
      return;
    case "unary":
      collectVariables(node.operand, names);
      return;
    case "binary":
    case "logical":
      collectVariables(node.left, names);
      collectVariables(node.right, names);
      return;
    case "conditional":
      collectVariables(node.test, names);
      collectVariables(node.then, names);
      collectVariables(node.otherwise, names);
      return;
    case "member":
      collectVariables(node.object, names);
      if (node.computed) {
        collectVariables(node.property, names);
      }
      return;
    case "call":
      for (const arg of node.args) {
        collectVariables(arg, names);
      }
      return;
    default:
      return;
  }
}

export function scopeFromRecord(values: Readonly<Record<string, UIValue>>, budget = createBudget()): ExpressionScope {
  return {
    lookup: (name) => (Object.hasOwn(values, name) ? values[name] : undefined),
    budget,
  };
}

// ---- Numeric formulas (calculator, function-plot) ----

export type CompiledExpression = {
  evaluate: (variables: Readonly<Record<string, number>>) => number;
  variables: readonly string[];
};

// A formula whose result must be a number. Errors: "unknown function <name>" at
// compile time, "unknown variable <name>" at evaluation; callers surface both.
export function compileExpression(source: string): CompiledExpression {
  const expression = parseExpression(source);
  // A comparison, logic or text at the top can never be a number: say so now
  // rather than failing at every point.
  const root = new Parser(tokenize(source)).parse();
  if (!mayBeNumber(root)) {
    throw new ExpressionError("expected a numeric formula");
  }
  return {
    variables: expression.variables,
    evaluate: (variables) => {
      const value = expression.evaluate(scopeFromRecord(variables));
      if (typeof value !== "number") {
        throw new ExpressionError("expected a number");
      }
      return value;
    },
  };
}

const NON_NUMERIC_OPERATORS = new Set(["==", "!=", "<", "<=", ">", ">="]);

function mayBeNumber(node: Node): boolean {
  switch (node.kind) {
    case "literal":
      return typeof node.value === "number";
    case "array":
      return false;
    case "unary":
      return node.op === "-";
    case "binary":
      return !NON_NUMERIC_OPERATORS.has(node.op);
    case "logical":
      return false;
    case "conditional":
      return mayBeNumber(node.then) || mayBeNumber(node.otherwise);
    default:
      return true;
  }
}

// ---- LaTeX (function-plot display) ----

const LATEX_FUNCTIONS: Record<string, string> = {
  sin: "\\sin",
  cos: "\\cos",
  tan: "\\tan",
  asin: "\\arcsin",
  acos: "\\arccos",
  atan: "\\arctan",
  sinh: "\\sinh",
  cosh: "\\cosh",
  tanh: "\\tanh",
  ln: "\\ln",
  log: "\\ln",
  log10: "\\log_{10}",
  log2: "\\log_{2}",
  exp: "\\exp",
  min: "\\min",
  max: "\\max",
};
const LATEX_CONSTANTS: Record<string, string> = { pi: "\\pi", e: "e" };

// Variable names that read as Greek letters in formulas (theta, omega_0…).
const GREEK = new Set([
  "alpha", "beta", "gamma", "delta", "epsilon", "zeta", "eta", "theta", "iota", "kappa", "lambda", "mu", "nu", "xi",
  "rho", "sigma", "tau", "upsilon", "phi", "chi", "psi", "omega", "Gamma", "Delta", "Theta", "Lambda", "Xi", "Sigma",
  "Phi", "Psi", "Omega",
]);

// `F_g` → F_{g}, `v_0` → v_{0}, `theta` → \theta, `mass` → \mathit{mass}.
// Names are identifiers, so the result never contains model-written LaTeX.
function variableLatex(name: string): string {
  const [base = name, ...rest] = name.split("_");
  const head = GREEK.has(base) ? `\\${base}` : base.length > 1 ? `\\mathit{${base}}` : base;
  const sub = rest.filter(Boolean).join(",");
  if (!sub) {
    return head;
  }
  return `${head}_{${GREEK.has(sub) ? `\\${sub}` : sub.length > 1 ? `\\mathrm{${sub}}` : sub}}`;
}

// A number as written in a worked formula: at most four decimals, thousands
// grouped, no exponent for everyday magnitudes.
function formatLatexNumber(value: number): string {
  if (!Number.isFinite(value)) {
    return "\\text{—}";
  }
  const abs = Math.abs(value);
  if (abs !== 0 && (abs >= 1e15 || abs < 1e-4)) {
    const [mantissa = "0", exponent = "0"] = value.toExponential(3).split("e");
    return `${Number(mantissa)} \\times 10^{${Number(exponent)}}`;
  }
  const fixed = Number(value.toFixed(4)).toString();
  const [whole = "0", fraction] = fixed.split(".");
  const grouped = whole.replace(/\B(?=(\d{3})+(?!\d))/g, "{,}");
  return fraction ? `${grouped}.${fraction}` : grouped;
}

function latexPrecedence(node: Node): number {
  if (node.kind === "binary") {
    return BINARY_PRECEDENCE[node.op] ?? 0;
  }
  if (node.kind === "unary") {
    return 5.5;
  }
  return 10;
}

// Values substituted for variables in a worked formula; absent for the symbolic form.
type LatexValues = Readonly<Record<string, number>> | undefined;

function wrapLatex(node: Node, minPrecedence: number, values?: LatexValues): string {
  const latex = nodeToLatex(node, values);
  return latexPrecedence(node) < minPrecedence ? `\\left(${latex}\\right)` : latex;
}

function nodeToLatex(node: Node, values?: LatexValues): string {
  const wrap = (child: Node, minPrecedence: number) => wrapLatex(child, minPrecedence, values);
  const latex = (child: Node) => nodeToLatex(child, values);
  switch (node.kind) {
    case "literal":
      if (typeof node.value !== "number") {
        throw new ExpressionError("only numeric formulas render as math");
      }
      return formatLatexNumber(node.value);
    case "var":
      if (values && Object.hasOwn(values, node.name)) {
        return `(${formatLatexNumber(values[node.name] ?? Number.NaN)})`;
      }
      return LATEX_CONSTANTS[node.name] ?? variableLatex(node.name);
    case "unary":
      if (node.op !== "-") {
        throw new ExpressionError("only numeric formulas render as math");
      }
      return `-${wrap(node.operand, 6)}`;
    case "binary": {
      const precedence = BINARY_PRECEDENCE[node.op] ?? 0;
      switch (node.op) {
        case "/":
          return `\\frac{${latex(node.left)}}{${latex(node.right)}}`;
        case "^":
          return `{${wrap(node.left, 10)}}^{${latex(node.right)}}`;
        case "*": {
          // Juxtapose when the right factor cannot be misread as part of a number.
          const right = wrap(node.right, precedence);
          const substituted = node.right.kind === "var" && values !== undefined && Object.hasOwn(values, node.right.name);
          const glue = (node.right.kind === "literal" || node.right.kind === "unary") && !substituted ? " \\cdot " : " ";
          return `${wrap(node.left, precedence)}${glue}${right}`;
        }
        case "%":
          return `${wrap(node.left, precedence)} \\bmod ${wrap(node.right, precedence + 1)}`;
        case "+":
        case "-":
          return `${wrap(node.left, precedence)} ${node.op} ${wrap(node.right, precedence + 1)}`;
        default:
          throw new ExpressionError("only numeric formulas render as math");
      }
    }
    case "call": {
      const args = node.args.map(latex);
      switch (node.name) {
        case "sqrt":
          return `\\sqrt{${args[0] ?? ""}}`;
        case "cbrt":
          return `\\sqrt[3]{${args[0] ?? ""}}`;
        case "abs":
          return `\\left|${args[0] ?? ""}\\right|`;
        case "floor":
          return `\\lfloor ${args[0] ?? ""} \\rfloor`;
        case "ceil":
          return `\\lceil ${args[0] ?? ""} \\rceil`;
        case "pow":
          return `{${wrap(node.args[0] ?? { kind: "literal", value: 0 }, 10)}}^{${args[1] ?? ""}}`;
        default: {
          const name = LATEX_FUNCTIONS[node.name] ?? `\\operatorname{${node.name}}`;
          return `${name}\\left(${args.join(", ")}\\right)`;
        }
      }
    }
    default:
      throw new ExpressionError("only numeric formulas render as math");
  }
}

// LaTeX for display next to an editable formula. Throws ExpressionError when the
// formula is invalid or not purely numeric.
export function expressionToLatex(source: string): string {
  return nodeToLatex(new Parser(tokenize(source)).parse());
}

// ---- Worked formulas (formula primitive) ----

export type FormulaLatex = {
  // name = expression, e.g. F_{g} = m g
  symbolic: string;
  // The line under it: = (9) (9.8) = 88.2\,\mathrm{N}. Absent when a value is
  // missing or not a number.
  worked?: string;
  result?: number;
};

// LaTeX for a formula shown with its current values: `name = expr`, then the
// line `= (values) = result unit`. Built from the parsed formula only, with the
// unit in \text and LaTeX specials escaped, so no model-written LaTeX reaches
// the renderer.
// `with` gives the value substituted for a name when it differs from the block's
// (a slider in 10^24 kg, a formula in kg): a number, or a formula of the block's
// values written with or without braces, as in `computed`.
export function formulaToLatex(
  source: string,
  options: { name?: string; unit?: string; values?: Readonly<Record<string, UIValue>>; with?: unknown; precision?: number } = {},
): FormulaLatex {
  const ast = new Parser(tokenize(source)).parse();
  if (!mayBeNumber(ast)) {
    throw new ExpressionError("expected a numeric formula");
  }
  const name = options.name && /^[A-Za-z_][A-Za-z0-9_]{0,31}$/.test(options.name) ? `${variableLatex(options.name)} = ` : "";
  const symbolic = `${name}${nodeToLatex(ast)}`;
  const names = new Set<string>();
  collectVariables(ast, names);
  const numbers: Record<string, number> = {};
  const overrides = options.with !== null && typeof options.with === "object" && !Array.isArray(options.with) ? options.with : {};
  for (const variable of names) {
    const value = Object.hasOwn(overrides, variable) ? overrideValue(Reflect.get(overrides, variable), options.values ?? {}) : options.values?.[variable];
    if (typeof value !== "number" || !Number.isFinite(value)) {
      return { symbolic };
    }
    Object.defineProperty(numbers, variable, { value, enumerable: true });
  }
  let result: number;
  try {
    const value = parseExpression(source).evaluate(scopeFromRecord(numbers));
    if (typeof value !== "number" || !Number.isFinite(value)) {
      return { symbolic };
    }
    result = value;
  } catch {
    return { symbolic };
  }
  const digits = Math.min(6, Math.max(0, Math.round(options.precision ?? 2)));
  const rounded = Number(result.toFixed(digits));
  const unit = options.unit?.trim() ? `\\,${unitLatex(options.unit.trim().slice(0, 24))}` : "";
  // The worked line continues the formula above it instead of repeating it.
  const steps = names.size > 0 ? `= ${nodeToLatex(ast, numbers)} ` : "";
  return { symbolic, worked: `${steps}= ${formatLatexNumber(rounded)}${unit}`, result };
}

function overrideValue(value: unknown, values: Readonly<Record<string, UIValue>>): UIValue | undefined {
  if (typeof value === "number") {
    return value;
  }
  if (typeof value !== "string") {
    return undefined;
  }
  const trimmed = value.trim();
  const source = trimmed.startsWith("{") && trimmed.endsWith("}") ? trimmed.slice(1, -1) : trimmed;
  try {
    return parseExpression(source).evaluate(scopeFromRecord(values));
  } catch {
    return undefined;
  }
}

// Plain unit strings (`N`, `m/s^2`, `kg*m/s^-1`) read as upright math; anything
// else (`元/月`, `万元`) is shown as text.
function unitLatex(unit: string): string {
  if (/^[A-Za-z]+(\^-?\d)?([*/][A-Za-z]+(\^-?\d)?)*$/.test(unit)) {
    return unit.replace(/([A-Za-z]+)(?:\^(-?\d))?/g, (_, word: string, power?: string) => `\\mathrm{${word}}${power ? `^{${power}}` : ""}`).replace(/\*/g, "\\cdot ");
  }
  return `\\text{${escapeLatexText(unit)}}`;
}

function escapeLatexText(text: string): string {
  return text.replace(/[\\{}$&#^_%~]/g, (char) => {
    switch (char) {
      case "\\":
        return "\\textbackslash{}";
      case "^":
        return "\\textasciicircum{}";
      case "~":
        return "\\textasciitilde{}";
      default:
        return `\\${char}`;
    }
  });
}
