// Inline HTML in Markdown is a single tag; block HTML (html-visual) spans whole lines.
const SINGLE_HTML_TAG_RE = /^<(\/?)([A-Za-z][A-Za-z0-9-]*)(?:\s[^<>]*)?(\/?)>$/;
// The sanitizer keeps <input> only as a disabled checkbox (for GFM task lists),
// so a raw <input type="range"> would turn into one.
const RAW_TAGS_REWRITTEN_BY_SANITIZER = new Set(["input"]);
// The parser reads what these wrap as plain text (or not at all), so even a closed
// pair would hide the Markdown inside it; they stay text either way.
const RAW_TEXT_TAGS = new Set([
  "iframe",
  "noembed",
  "noframes",
  "noscript",
  "plaintext",
  "script",
  "style",
  "template",
  "textarea",
  "title",
  "xmp",
]);

type HASTNode = {
  type?: string;
  value?: string;
  children?: HASTNode[];
};

type OpenTag = { name: string; node: HASTNode };

function escapeAsText(node: HASTNode) {
  node.type = "text";
}

// Pairs tags among the siblings of one parent: inline HTML cannot span paragraphs
// or table cells, so a tag closed elsewhere counts as unclosed here.
function escapeUnpairedRawTags(children: HASTNode[], allowedTags: ReadonlySet<string>) {
  const open: OpenTag[] = [];
  for (const child of children) {
    if (child.type !== "raw") {
      continue;
    }
    const match = SINGLE_HTML_TAG_RE.exec(child.value?.trim() ?? "");
    const name = match?.[2]?.toLowerCase();
    if (!match || !name || allowedTags.has(name)) {
      continue;
    }
    if (RAW_TEXT_TAGS.has(name) || match[3] === "/") {
      escapeAsText(child);
      continue;
    }
    if (match[1] !== "/") {
      open.push({ name, node: child });
      continue;
    }
    const opener = open.findLastIndex((tag) => tag.name === name);
    if (opener < 0) {
      escapeAsText(child);
      continue;
    }
    // The pair is left for the sanitizer, which drops both tags and keeps what they
    // wrap; anything opened inside it and never closed is still text.
    for (const tag of open.splice(opener).slice(1)) {
      escapeAsText(tag.node);
    }
  }
  for (const tag of open) {
    escapeAsText(tag.node);
  }
}

function visitRawHTMLNodes(node: HASTNode, allowedTags: ReadonlySet<string>) {
  const children = node.children ?? [];
  escapeUnpairedRawTags(children, allowedTags);
  for (const child of children) {
    visitRawHTMLNodes(child, allowedTags);
  }
}

// Shows a lone tag the sanitizer would not render ("use <button> here") as the
// text that was written. Dropping it loses the words, and an unclosed <button>
// or <select> still swallows the following paragraphs, nesting <p> inside <p>.
// A closed pair such as <font color="green">-0.3 kg</font> is markup, not prose:
// the sanitizer unwraps it to its content, so escaping it would print the tags.
export function escapeUnsupportedInlineHTMLRehypePlugin(tagNames: readonly string[]) {
  const allowedTags = new Set(tagNames.filter((tagName) => !RAW_TAGS_REWRITTEN_BY_SANITIZER.has(tagName)));
  return (tree: HASTNode) => {
    visitRawHTMLNodes(tree, allowedTags);
  };
}
