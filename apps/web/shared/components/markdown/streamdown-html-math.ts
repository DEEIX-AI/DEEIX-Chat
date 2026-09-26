const HTML_TOKEN_RE = /<!--[\s\S]*?-->|<![^>]*>|<\/?[A-Za-z][^>]*>|[^<]+/g;
const HTML_TAG_NAME_RE = /^<\/?\s*([A-Za-z][A-Za-z0-9-]*)/;
const RAW_HTML_MATH_SKIP_TAGS = new Set(["code", "pre", "script", "style", "textarea"]);

type HASTNode = {
  type?: string;
  value?: string;
  children?: HASTNode[];
};

function isEscapedCharacter(source: string, index: number): boolean {
  let slashCount = 0;
  for (let cursor = index - 1; cursor >= 0 && source[cursor] === "\\"; cursor -= 1) {
    slashCount += 1;
  }
  return slashCount % 2 === 1;
}

function getMathDelimiterLength(source: string, index: number): number {
  if (source[index] !== "$" || isEscapedCharacter(source, index) || source[index - 1] === "$") {
    return 0;
  }
  if (source[index + 1] === "$" && source[index + 2] !== "$") {
    return 2;
  }
  return source[index + 1] === "$" ? 0 : 1;
}

function escapeMathHTML(source: string): string {
  return source
    .replace(/&(?!(?:#\d+|#x[\da-f]+|[a-z][\w-]*);)/gi, "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;");
}

function renderMathInHTMLText(source: string): string {
  if (!source.includes("$")) {
    return source;
  }

  let output = "";
  let cursor = 0;
  let index = 0;

  while (index < source.length) {
    const delimiterLength = getMathDelimiterLength(source, index);
    if (!delimiterLength) {
      index += 1;
      continue;
    }

    let closingIndex = -1;
    for (let candidate = index + delimiterLength; candidate < source.length; candidate += 1) {
      if (getMathDelimiterLength(source, candidate) === delimiterLength) {
        closingIndex = candidate;
        break;
      }
    }

    if (closingIndex < 0) {
      break;
    }

    const math = source.slice(index + delimiterLength, closingIndex).trim();
    if (!math || (delimiterLength === 1 && math.includes("\n"))) {
      index = closingIndex + delimiterLength;
      continue;
    }

    output += source.slice(cursor, index);
    const code = `<code class="language-math">${escapeMathHTML(math)}</code>`;
    output += delimiterLength === 2 ? `<pre>${code}</pre>` : code;
    cursor = closingIndex + delimiterLength;
    index = cursor;
  }

  return cursor > 0 ? output + source.slice(cursor) : source;
}

function renderMathInRawHTML(source: string): string {
  if (!source.includes("$")) {
    return source;
  }

  const skippedTags: string[] = [];
  return source.replace(HTML_TOKEN_RE, (token) => {
    if (!token.startsWith("<")) {
      return skippedTags.length > 0 ? token : renderMathInHTMLText(token);
    }

    const tagName = HTML_TAG_NAME_RE.exec(token)?.[1]?.toLowerCase();
    if (!tagName || !RAW_HTML_MATH_SKIP_TAGS.has(tagName)) {
      return token;
    }

    if (/^<\//.test(token)) {
      const matchingIndex = skippedTags.lastIndexOf(tagName);
      if (matchingIndex >= 0) {
        skippedTags.splice(matchingIndex, 1);
      }
    } else if (!/\/\s*>$/.test(token)) {
      skippedTags.push(tagName);
    }
    return token;
  });
}

function visitRawHTMLNodes(node: HASTNode) {
  if (node.type === "raw" && node.value?.includes("$")) {
    node.value = renderMathInRawHTML(node.value);
  }

  for (const child of node.children ?? []) {
    visitRawHTMLNodes(child);
  }
}

export function renderRawHTMLMathRehypePlugin() {
  return (tree: HASTNode) => {
    visitRawHTMLNodes(tree);
  };
}
