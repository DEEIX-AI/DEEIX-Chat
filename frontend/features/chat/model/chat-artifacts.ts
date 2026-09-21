import type { ChatAreaMessage } from "@/features/chat/types/messages";
import { getBrandingSnapshot } from "@/shared/config/branding";
import {
  type ArtifactPreviewKind,
  resolveArtifactPreviewKind,
} from "@/shared/lib/artifact-preview";
import type { HTMLVisualThemeSnapshot } from "@/shared/lib/html-visual-theme";
import { escapeHTML, escapeScriptContent, escapeStyleContent, sandboxDocumentHead } from "@/shared/lib/sandbox-document";

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

const FENCE_OPEN_RE = /^[ \t]*(`{3,}|~{3,})([^\n]*)$/;
const DOCTYPE_RE = /<!doctype\s+html[^>]*>/i;
const HTML_OPEN_RE = /<html\b[^>]*>/i;
const HTML_CLOSE_RE = /<\/html\s*>/i;
const HEAD_BLOCK_RE = /<head\b[^>]*>([\s\S]*?)<\/head\s*>/i;
const BODY_BLOCK_RE = /<body\b[^>]*>([\s\S]*?)<\/body\s*>/i;

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

function htmlPreviewDocument(code: string, theme: HTMLVisualThemeSnapshot): string {
  const safeHead = sandboxDocumentHead("Artifact Preview", theme);
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
${sandboxDocumentHead("CSS Preview", theme)}
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
${sandboxDocumentHead("JavaScript Preview", theme)}
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
  kind: Exclude<ArtifactPreviewKind, "svg">,
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
  if (kind === "svg") return "artifact.svg";
  return "artifact-preview.html";
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
