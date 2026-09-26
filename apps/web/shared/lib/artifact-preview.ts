export type ArtifactPreviewKind = "html" | "css" | "javascript" | "svg";

const HTML_LIKE_RE = /^\s*(?:<!doctype\s+html|<html\b|<head\b|<body\b|<(?:article|canvas|div|main|section|style|script)\b)/i;
const SVG_DOCTYPE_RE = /^<!doctype\s+svg(?:\s|\[|>)/i;
const SVG_ROOT_RE = /^<(?:[a-z_][\w.-]*:)?svg(?:\s|\/?>)/i;

function normalizeLanguage(language: string): string {
  return language.trim().toLowerCase();
}

function skipDocumentWhitespace(source: string, start: number): number {
  let index = start;
  while (index < source.length) {
    const character = source.charCodeAt(index);
    if (
      character !== 0x09 &&
      character !== 0x0a &&
      character !== 0x0d &&
      character !== 0x20 &&
      character !== 0xfeff
    ) {
      break;
    }
    index += 1;
  }
  return index;
}

function hasSVGDocumentRoot(code: string): boolean {
  let cursor = skipDocumentWhitespace(code, 0);

  while (cursor < code.length) {
    if (code.startsWith("<!--", cursor)) {
      const end = code.indexOf("-->", cursor + 4);
      if (end < 0) return false;
      cursor = skipDocumentWhitespace(code, end + 3);
      continue;
    }

    if (code.startsWith("<?", cursor)) {
      const end = code.indexOf("?>", cursor + 2);
      if (end < 0) return false;
      cursor = skipDocumentWhitespace(code, end + 2);
      continue;
    }

    if (SVG_DOCTYPE_RE.test(code.slice(cursor))) {
      let quote = "";
      let subsetDepth = 0;
      let end = -1;

      for (let index = cursor; index < code.length; index += 1) {
        const character = code[index];
        if (quote) {
          if (character === quote) quote = "";
          continue;
        }
        if (character === '"' || character === "'") {
          quote = character;
          continue;
        }
        if (character === "[") {
          subsetDepth += 1;
          continue;
        }
        if (character === "]" && subsetDepth > 0) {
          subsetDepth -= 1;
          continue;
        }
        if (character === ">" && subsetDepth === 0) {
          end = index;
          break;
        }
      }

      if (end < 0) return false;
      cursor = skipDocumentWhitespace(code, end + 1);
      continue;
    }

    break;
  }

  return SVG_ROOT_RE.test(code.slice(cursor));
}

// 未标注语言的围栏在渲染层会被归一化为纯文本语言,这些语言下按内容嗅探 HTML/SVG 预览能力。
const CONTENT_SNIFF_LANGUAGES = ["", "text", "txt", "plaintext", "plain", "markdown"];

export function resolveArtifactPreviewKind(language: string, code: string): ArtifactPreviewKind | null {
  const normalized = normalizeLanguage(language);
  if (["html", "htm", "xhtml"].includes(normalized)) return "html";
  if (["css", "scss", "sass", "less"].includes(normalized)) return "css";
  if (["js", "javascript", "mjs", "cjs"].includes(normalized)) return "javascript";
  if (["svg", "svg+xml", "image/svg+xml"].includes(normalized)) return "svg";
  if (
    [...CONTENT_SNIFF_LANGUAGES, "xml", "text/xml", "application/xml"].includes(normalized) &&
    hasSVGDocumentRoot(code)
  ) {
    return "svg";
  }
  if (CONTENT_SNIFF_LANGUAGES.includes(normalized) && HTML_LIKE_RE.test(code)) return "html";
  return null;
}
