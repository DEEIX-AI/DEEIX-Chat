// Inline HTML in Markdown is a single tag; block HTML (html-visual) spans whole lines.
const SINGLE_HTML_TAG_RE = /^<\/?([A-Za-z][A-Za-z0-9-]*)(?:\s[^<>]*)?\/?>$/;
// The sanitizer keeps <input> only as a disabled checkbox (for GFM task lists),
// so a raw <input type="range"> would turn into one.
const RAW_TAGS_REWRITTEN_BY_SANITIZER = new Set(["input"]);

type HASTNode = {
  type?: string;
  value?: string;
  children?: HASTNode[];
};

function visitRawHTMLNodes(node: HASTNode, allowedTags: ReadonlySet<string>) {
  for (const child of node.children ?? []) {
    if (child.type !== "raw") {
      visitRawHTMLNodes(child, allowedTags);
      continue;
    }
    const tagName = SINGLE_HTML_TAG_RE.exec(child.value?.trim() ?? "")?.[1]?.toLowerCase();
    if (tagName && !allowedTags.has(tagName)) {
      child.type = "text";
    }
  }
}

// Shows a lone tag the sanitizer would not render ("use <button> here") as the
// text that was written. Dropping it loses the words, and an unclosed <button>
// or <select> still swallows the following paragraphs, nesting <p> inside <p>.
export function escapeUnsupportedInlineHTMLRehypePlugin(tagNames: readonly string[]) {
  const allowedTags = new Set(tagNames.filter((tagName) => !RAW_TAGS_REWRITTEN_BY_SANITIZER.has(tagName)));
  return (tree: HASTNode) => {
    visitRawHTMLNodes(tree, allowedTags);
  };
}
