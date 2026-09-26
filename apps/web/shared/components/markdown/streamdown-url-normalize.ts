const URL_PROTOCOL_RE = /^[A-Za-z][A-Za-z0-9+.-]*:/;
const BARE_DOMAIN_URL_RE = /^(?:www\.)?(?:[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?\.)+[A-Za-z]{2,63}(?::\d{2,5})?(?:[/?#][^\s]*)?$/;

type HASTNode = {
  type?: string;
  tagName?: string;
  properties?: Record<string, unknown>;
  children?: HASTNode[];
};

function normalizeBareURL(value: unknown): string | undefined {
  if (typeof value !== "string" || value === "") {
    return undefined;
  }

  if (value.startsWith("//")) {
    const withoutSlashes = value.slice(2);
    return BARE_DOMAIN_URL_RE.test(withoutSlashes) ? `https:${value}` : value;
  }

  if (
    URL_PROTOCOL_RE.test(value) ||
    value.startsWith("#") ||
    value.startsWith("/") ||
    value.startsWith("./") ||
    value.startsWith("../")
  ) {
    return value;
  }

  return BARE_DOMAIN_URL_RE.test(value) ? `https://${value}` : value;
}

function normalizeURLProperty(node: HASTNode, property: "href" | "src") {
  if (!node.properties) {
    return;
  }

  const normalized = normalizeBareURL(node.properties[property]);
  if (normalized) {
    node.properties[property] = normalized;
  }
}

function visitElementNodes(node: HASTNode) {
  if (node.type === "element") {
    if (node.tagName === "a") {
      normalizeURLProperty(node, "href");
    } else if (node.tagName === "img") {
      normalizeURLProperty(node, "src");
    }
  }

  for (const child of node.children ?? []) {
    visitElementNodes(child);
  }
}

export function normalizeBareURLRehypePlugin() {
  return (tree: HASTNode) => {
    visitElementNodes(tree);
  };
}
