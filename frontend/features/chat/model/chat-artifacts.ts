import type { ChatAreaMessage } from "@/features/chat/types/messages";
import {
  resolveArtifactPreviewKind,
  type ArtifactPreviewKind,
} from "@/shared/lib/artifact-preview";
import { getBrandingSnapshot } from "@/shared/config/branding";
import type { HTMLVisualThemeSnapshot } from "@/shared/lib/html-visual-theme";

export type { ArtifactPreviewKind } from "@/shared/lib/artifact-preview";

export type ChatArtifact = {
  id: string;
  messageID: string;
  messageKey: string;
  runID?: string;
  blockIndex: number;
  kind: ArtifactPreviewKind;
  language: string;
  code: string;
  complete: boolean;
  streaming: boolean;
  updatedAt?: string;
};

export type OpenCodeArtifactInput = {
  code: string;
  language: string;
  kind: ArtifactPreviewKind;
};

const SCRIPT_CLOSE_RE = /<\/script/gi;
const STYLE_CLOSE_RE = /<\/style/gi;
const FENCE_OPEN_RE = /^[ \t]*(`{3,}|~{3,})([^\n]*)$/;
const DOCTYPE_RE = /<!doctype\s+html[^>]*>/i;
const HTML_OPEN_RE = /<html\b[^>]*>/i;
const HTML_CLOSE_RE = /<\/html\s*>/i;
const HEAD_BLOCK_RE = /<head\b[^>]*>([\s\S]*?)<\/head\s*>/i;
const BODY_BLOCK_RE = /<body\b[^>]*>([\s\S]*?)<\/body\s*>/i;
const ARTIFACT_CSP = [
  "default-src 'none'",
  "base-uri 'none'",
  "form-action 'none'",
  "object-src 'none'",
  "frame-src 'none'",
  "child-src 'none'",
  "worker-src 'none'",
  "connect-src 'none'",
  "manifest-src 'none'",
  "prefetch-src 'none'",
  "navigate-to 'none'",
  "img-src data: blob:",
  "media-src data: blob:",
  "font-src data:",
  "style-src 'unsafe-inline'",
  "script-src 'unsafe-inline'",
].join("; ");

function parseFenceLanguage(info: string): string {
  const raw = info.trim().split(/\s+/)[0] ?? "";
  return raw.replace(/^\{?\.?/, "").replace(/\}?$/, "");
}

function artifactStableMessageID(
  message: Pick<ChatAreaMessage, "publicID" | "runID">,
): string {
  return message.runID?.trim() || message.publicID;
}

function isFenceClose(line: string, marker: string): boolean {
  const escaped = marker[0] === "`" ? "`" : "~";
  const re = new RegExp(`^[ \\t]*${escaped}{${marker.length},}[ \\t]*$`);
  return re.test(line);
}

function escapeHTML(value: string): string {
  return value
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;");
}

function escapeScriptContent(value: string): string {
  return value.replace(SCRIPT_CLOSE_RE, "<\\/script");
}

function escapeStyleContent(value: string): string {
  return value.replace(STYLE_CLOSE_RE, "<\\/style");
}

function artifactRuntimeScript(): string {
  return `<script>
(() => {
  const formatError = (value) => {
    if (!value) return "Unknown preview error";
    if (value && value.stack) return String(value.stack);
    if (value && value.message) return String(value.message);
    return String(value);
  };
  const showError = (value) => {
    const message = formatError(value);
    const node = document.createElement("pre");
    node.textContent = message;
    node.style.cssText = "margin:16px;padding:12px;border:1px solid var(--destructive);border-radius:var(--radius);background:color-mix(in oklch,var(--destructive) 12%,var(--background));color:var(--destructive);font:12px/1.5 var(--font-mono);white-space:pre-wrap;";
    document.body.appendChild(node);
  };
  window.addEventListener("error", (event) => showError(event.error || event.message));
  window.addEventListener("unhandledrejection", (event) => showError(event.reason));
})();
</script>`;
}

function artifactPreviewResetStyle(): string {
  return `<style data-deeix-artifact-reset>
html,
body {
  min-height: 100%;
  width: 100%;
  margin: 0;
}

body {
  overflow: auto;
}

*,
*::before,
*::after {
  box-sizing: border-box;
}
</style>`;
}

function artifactThemeStyle(theme: HTMLVisualThemeSnapshot): string {
  const declarations = theme.variables.map(([name, value]) => `${name}:${value}`).join(";");
  return `<style data-deeix-artifact-theme>
:root { color-scheme: ${theme.colorScheme}; ${escapeStyleContent(declarations)} }
html, body { color: var(--foreground); background: var(--background); }
</style>`;
}

function previewHead(title: string, theme: HTMLVisualThemeSnapshot): string {
  return [
    `<meta charset="utf-8">`,
    `<meta name="viewport" content="width=device-width, initial-scale=1">`,
    `<meta http-equiv="Content-Security-Policy" content="${ARTIFACT_CSP}">`,
    `<title>${escapeHTML(title)}</title>`,
    artifactThemeStyle(theme),
    artifactPreviewResetStyle(),
    artifactRuntimeScript(),
  ].join("");
}

function htmlPreviewDocument(code: string, theme: HTMLVisualThemeSnapshot): string {
  const safeHead = previewHead("Artifact Preview", theme);
  const userHead = HEAD_BLOCK_RE.exec(code)?.[1]?.trim() ?? "";
  const bodyMatch = BODY_BLOCK_RE.exec(code);
  const body = bodyMatch
    ? bodyMatch[1]
    : code
        .replace(DOCTYPE_RE, "")
        .replace(HTML_OPEN_RE, "")
        .replace(HTML_CLOSE_RE, "")
        .replace(HEAD_BLOCK_RE, "")
        .trim();

  return `<!doctype html><html><head>${safeHead}${userHead}</head><body>${body}</body></html>`;
}

function cssPreviewDocument(code: string, theme: HTMLVisualThemeSnapshot): string {
  const branding = getBrandingSnapshot();
  return `<!doctype html>
<html>
<head>
${previewHead("CSS Preview", theme)}
<style>${escapeStyleContent(code)}</style>
</head>
<body>
  <main class="artifact-preview">
    <section class="preview-panel">
      <p class="eyebrow">${escapeHTML(branding.shortName)} Artifact</p>
      <h1>Preview Surface</h1>
      <p>Generated CSS is applied to this isolated document.</p>
      <div class="preview-row">
        <button type="button">Primary action</button>
        <button type="button" class="secondary">Secondary</button>
      </div>
      <div class="preview-grid">
        <article><strong>Card</strong><span>Sample content</span></article>
        <article><strong>Metric</strong><span>128</span></article>
      </div>
    </section>
  </main>
</body>
</html>`;
}

function javascriptPreviewDocument(code: string, theme: HTMLVisualThemeSnapshot): string {
  return `<!doctype html>
<html>
<head>
${previewHead("JavaScript Preview", theme)}
<style>
body { margin: 0; font: 14px/1.5 var(--font-sans); color: var(--foreground); background: var(--background); }
#root { min-height: 100vh; padding: 20px; box-sizing: border-box; }
.artifact-console { position: fixed; inset-inline: 12px; bottom: 12px; max-height: 32vh; overflow: auto; border: 1px solid var(--border); border-radius: var(--radius); background: var(--muted); color: var(--muted-foreground); padding: 10px; font: 12px/1.5 var(--font-mono); white-space: pre-wrap; }
</style>
</head>
<body>
<div id="root"></div>
<pre id="console" class="artifact-console" hidden></pre>
<script>
(() => {
  const consoleNode = document.getElementById("console");
  const write = (level, values) => {
    consoleNode.hidden = false;
    consoleNode.textContent += "[" + level + "] " + values.map((item) => {
      try { return typeof item === "string" ? item : JSON.stringify(item); }
      catch { return String(item); }
    }).join(" ") + "\\n";
  };
  for (const level of ["log", "info", "warn", "error"]) {
    const original = console[level].bind(console);
    console[level] = (...values) => {
      write(level, values);
      original(...values);
    };
  }
})();
</script>
<script>${escapeScriptContent(code)}</script>
</body>
</html>`;
}

export function buildArtifactPreviewDocument(
  kind: ArtifactPreviewKind,
  code: string,
  theme: HTMLVisualThemeSnapshot,
): string {
  if (kind === "css") return cssPreviewDocument(code, theme);
  if (kind === "javascript") return javascriptPreviewDocument(code, theme);
  return htmlPreviewDocument(code, theme);
}

export function resolveArtifactDownloadName(kind: ArtifactPreviewKind): string {
  if (kind === "css") return "artifact-css-preview.html";
  if (kind === "javascript") return "artifact-js-preview.html";
  return "artifact-preview.html";
}

export function downloadArtifactHTML(fileName: string, value: string): void {
  const blob = new Blob([value], { type: "text/html;charset=utf-8" });
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = fileName;
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  URL.revokeObjectURL(url);
}

export function extractArtifactsFromContent(
  message: Pick<ChatAreaMessage, "content" | "isStreaming" | "key" | "publicID" | "runID" | "updatedAt">,
): ChatArtifact[] {
  const content = message.content;
  const artifacts: ChatArtifact[] = [];
  const lines = content.split(/\r?\n/);
  const stableMessageID = artifactStableMessageID(message);
  const runID = message.runID?.trim() || undefined;
  let openMarker = "";
  let language = "";
  let codeLines: string[] = [];
  let blockIndex = 0;

  const pushArtifact = (code: string, complete: boolean) => {
    const kind = resolveArtifactPreviewKind(language, code);
    if (!kind || !code.trim()) {
      return;
    }
    artifacts.push({
      id: `${stableMessageID}:artifact:${blockIndex}`,
      messageID: message.publicID,
      messageKey: message.key,
      runID,
      blockIndex,
      kind,
      language,
      code,
      complete,
      streaming: Boolean(message.isStreaming),
      updatedAt: message.updatedAt,
    });
    blockIndex += 1;
  };

  for (const line of lines) {
    if (!openMarker) {
      const openMatch = line.match(FENCE_OPEN_RE);
      if (!openMatch) {
        continue;
      }
      openMarker = openMatch[1] ?? "";
      language = parseFenceLanguage(openMatch[2] ?? "");
      codeLines = [];
      continue;
    }

    if (isFenceClose(line, openMarker)) {
      pushArtifact(codeLines.join("\n"), true);
      openMarker = "";
      language = "";
      codeLines = [];
      continue;
    }

    codeLines.push(line);
  }

  if (openMarker && message.isStreaming) {
    pushArtifact(codeLines.join("\n"), false);
  }

  if (artifacts.length === 0) {
    const kind = resolveArtifactPreviewKind("", content);
    if (kind && content.trim()) {
      artifacts.push({
        id: `${stableMessageID}:artifact:0`,
        messageID: message.publicID,
        messageKey: message.key,
        runID,
        blockIndex: 0,
        kind,
        language: kind,
        code: content,
        complete: !message.isStreaming,
        streaming: Boolean(message.isStreaming),
        updatedAt: message.updatedAt,
      });
    }
  }

  return artifacts;
}

export function extractArtifactsFromMessages(messages: ChatAreaMessage[]): ChatArtifact[] {
  return messages.flatMap((message) => (message.role === "assistant" ? extractArtifactsFromContent(message) : []));
}
