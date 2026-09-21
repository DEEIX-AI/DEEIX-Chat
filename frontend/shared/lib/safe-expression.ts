// Small arithmetic expression evaluator for model-supplied formulas.
// Parses to an AST and evaluates against a variable map; never touches eval
// or Function. Grammar: numbers, identifiers, + - * / % ^, unary minus,
// parentheses, and a fixed function whitelist.

type Token =
  | { kind: "number"; value: number }
  | { kind: "ident"; value: string }
  | { kind: "op"; value: string }
  | { kind: "lparen" }
  | { kind: "rparen" }
  | { kind: "comma" }
  | { kind: "end" };

type Node =
  | { kind: "number"; value: number }
  | { kind: "var"; name: string }
  | { kind: "unary"; op: "-"; operand: Node }
  | { kind: "binary"; op: string; left: Node; right: Node }
  | { kind: "call"; name: string; args: Node[] };

const FUNCTIONS: Record<string, (...args: number[]) => number> = {
  abs: Math.abs,
  sqrt: Math.sqrt,
  pow: Math.pow,
  min: Math.min,
  max: Math.max,
  round: (value, digits = 0) => {
    const factor = 10 ** Math.trunc(digits);
    return Math.round(value * factor) / factor;
  },
  floor: Math.floor,
  ceil: Math.ceil,
  log: Math.log,
  log10: Math.log10,
  exp: Math.exp,
  sin: Math.sin,
  cos: Math.cos,
  tan: Math.tan,
  asin: Math.asin,
  acos: Math.acos,
  atan: Math.atan,
  sinh: Math.sinh,
  cosh: Math.cosh,
  tanh: Math.tanh,
  ln: Math.log,
  log2: Math.log2,
  cbrt: Math.cbrt,
  sign: Math.sign,
};

const CONSTANTS: Record<string, number> = { pi: Math.PI, e: Math.E };

const PRECEDENCE: Record<string, number> = { "+": 1, "-": 1, "*": 2, "/": 2, "%": 2, "^": 3 };
const RIGHT_ASSOCIATIVE = new Set(["^"]);
const MAX_LENGTH = 512;

export class ExpressionError extends Error {}

function tokenize(source: string): Token[] {
  if (source.length > MAX_LENGTH) {
    throw new ExpressionError("expression too long");
  }
  const tokens: Token[] = [];
  let index = 0;
  while (index < source.length) {
    const char = source[index] ?? "";
    if (/\s/.test(char)) {
      index += 1;
    } else if (/[0-9.]/.test(char)) {
      const match = /^(\d+\.?\d*|\.\d+)(e[+-]?\d+)?/i.exec(source.slice(index));
      if (!match) {
        throw new ExpressionError(`bad number at ${index}`);
      }
      tokens.push({ kind: "number", value: Number(match[0]) });
      index += match[0].length;
    } else if (/[a-zA-Z_]/.test(char)) {
      const match = /^[a-zA-Z_][a-zA-Z0-9_]*/.exec(source.slice(index));
      const value = match?.[0] ?? char;
      tokens.push({ kind: "ident", value });
      index += value.length;
    } else if ("+-*/%^".includes(char)) {
      tokens.push({ kind: "op", value: char });
      index += 1;
    } else if (char === "(") {
      tokens.push({ kind: "lparen" });
      index += 1;
    } else if (char === ")") {
      tokens.push({ kind: "rparen" });
      index += 1;
    } else if (char === ",") {
      tokens.push({ kind: "comma" });
      index += 1;
    } else {
      throw new ExpressionError(`unexpected "${char}" at ${index}`);
    }
  }
  tokens.push({ kind: "end" });
  return tokens;
}

class Parser {
  private index = 0;
  constructor(private readonly tokens: Token[]) {}

  parse(): Node {
    const node = this.expression(0);
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

  private expression(minPrecedence: number): Node {
    let left = this.unary();
    for (;;) {
      const token = this.peek();
      if (token.kind !== "op" || token.value === "" || (PRECEDENCE[token.value] ?? 0) < minPrecedence) {
        return left;
      }
      const precedence = PRECEDENCE[token.value] ?? 0;
      if (precedence < minPrecedence) {
        return left;
      }
      this.next();
      const right = this.expression(RIGHT_ASSOCIATIVE.has(token.value) ? precedence : precedence + 1);
      left = { kind: "binary", op: token.value, left, right };
    }
  }

  private unary(): Node {
    const token = this.peek();
    if (token.kind === "op" && token.value === "-") {
      this.next();
      // Unary minus binds looser than ^, so -2^2 is -(2^2) as in most math tools.
      return { kind: "unary", op: "-", operand: this.expression(PRECEDENCE["^"] ?? 3) };
    }
    if (token.kind === "op" && token.value === "+") {
      this.next();
      return this.unary();
    }
    return this.primary();
  }

  private primary(): Node {
    const token = this.next();
    switch (token.kind) {
      case "number":
        return { kind: "number", value: token.value };
      case "ident": {
        if (this.peek().kind === "lparen") {
          this.next();
          const args: Node[] = [];
          if (this.peek().kind !== "rparen") {
            args.push(this.expression(0));
            while (this.peek().kind === "comma") {
              this.next();
              args.push(this.expression(0));
            }
          }
          if (this.next().kind !== "rparen") {
            throw new ExpressionError("expected )");
          }
          if (!(token.value in FUNCTIONS)) {
            throw new ExpressionError(`unknown function ${token.value}`);
          }
          return { kind: "call", name: token.value, args };
        }
        return { kind: "var", name: token.value };
      }
      case "lparen": {
        const node = this.expression(0);
        if (this.next().kind !== "rparen") {
          throw new ExpressionError("expected )");
        }
        return node;
      }
      default:
        throw new ExpressionError("unexpected token");
    }
  }
}

function evaluate(node: Node, variables: Readonly<Record<string, number>>): number {
  switch (node.kind) {
    case "number":
      return node.value;
    case "var": {
      const value = variables[node.name] ?? CONSTANTS[node.name];
      if (value === undefined) {
        throw new ExpressionError(`unknown variable ${node.name}`);
      }
      return value;
    }
    case "unary":
      return -evaluate(node.operand, variables);
    case "binary": {
      const left = evaluate(node.left, variables);
      const right = evaluate(node.right, variables);
      switch (node.op) {
        case "+":
          return left + right;
        case "-":
          return left - right;
        case "*":
          return left * right;
        case "/":
          return left / right;
        case "%":
          return left % right;
        case "^":
          return left ** right;
        default:
          throw new ExpressionError(`unknown operator ${node.op}`);
      }
    }
    case "call": {
      const fn = FUNCTIONS[node.name];
      if (!fn) {
        throw new ExpressionError(`unknown function ${node.name}`);
      }
      return fn(...node.args.map((arg) => evaluate(arg, variables)));
    }
  }
}

export type CompiledExpression = {
  evaluate: (variables: Readonly<Record<string, number>>) => number;
  variables: readonly string[];
};

export function compileExpression(source: string): CompiledExpression {
  const ast = new Parser(tokenize(source)).parse();
  const names = new Set<string>();
  const collect = (node: Node) => {
    switch (node.kind) {
      case "var":
        if (!(node.name in CONSTANTS)) {
          names.add(node.name);
        }
        break;
      case "unary":
        collect(node.operand);
        break;
      case "binary":
        collect(node.left);
        collect(node.right);
        break;
      case "call":
        node.args.forEach(collect);
        break;
      default:
        break;
    }
  };
  collect(ast);
  return {
    evaluate: (variables) => evaluate(ast, variables),
    variables: Array.from(names),
  };
}

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
  floor: "\\lfloor",
  ceil: "\\lceil",
};
const LATEX_CONSTANTS: Record<string, string> = { pi: "\\pi", e: "e" };

function nodePrecedence(node: Node): number {
  switch (node.kind) {
    case "binary":
      return PRECEDENCE[node.op] ?? 0;
    case "unary":
      return 1.5;
    default:
      return 10;
  }
}

function wrap(node: Node, minPrecedence: number): string {
  const latex = nodeToLatex(node);
  return nodePrecedence(node) < minPrecedence ? `\\left(${latex}\\right)` : latex;
}

function nodeToLatex(node: Node): string {
  switch (node.kind) {
    case "number":
      return String(node.value);
    case "var":
      return LATEX_CONSTANTS[node.name] ?? (node.name.length > 1 ? `\\mathit{${node.name}}` : node.name);
    case "unary":
      return `-${wrap(node.operand, 2)}`;
    case "binary": {
      const precedence = PRECEDENCE[node.op] ?? 0;
      switch (node.op) {
        case "/":
          return `\\frac{${nodeToLatex(node.left)}}{${nodeToLatex(node.right)}}`;
        case "^":
          return `{${wrap(node.left, 10)}}^{${nodeToLatex(node.right)}}`;
        case "*": {
          // Juxtapose when the right factor cannot be misread as part of a number.
          const right = wrap(node.right, precedence);
          const glue = node.right.kind === "number" || node.right.kind === "unary" ? " \\cdot " : " ";
          return `${wrap(node.left, precedence)}${glue}${right}`;
        }
        case "%":
          return `${wrap(node.left, precedence)} \\bmod ${wrap(node.right, precedence + 1)}`;
        default:
          return `${wrap(node.left, precedence)} ${node.op} ${wrap(node.right, precedence + 1)}`;
      }
    }
    case "call": {
      const args = node.args.map(nodeToLatex);
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
          return `{${wrap(node.args[0] ?? { kind: "number", value: 0 }, 10)}}^{${args[1] ?? ""}}`;
        default: {
          const name = LATEX_FUNCTIONS[node.name] ?? `\\operatorname{${node.name}}`;
          return `${name}\\left(${args.join(", ")}\\right)`;
        }
      }
    }
  }
}

// LaTeX for display next to an editable expression. Throws ExpressionError on
// invalid input like compileExpression does.
export function expressionToLatex(source: string): string {
  return nodeToLatex(new Parser(tokenize(source)).parse());
}
