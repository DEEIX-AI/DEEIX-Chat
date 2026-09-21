import type { HTMLVisualThemeSnapshot } from "@/shared/lib/html-visual-theme";

// Shared primitives for every model-generated document rendered in a sandboxed
// iframe (chat artifacts, custom ui-blocks). Keep the CSP and permission list
// in one place so the two surfaces cannot drift apart.

const SCRIPT_CLOSE_RE = /<\/script/gi;
const STYLE_CLOSE_RE = /<\/style/gi;

export const SANDBOX_CSP = [
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
  "img-src data: blob:",
  "media-src data: blob:",
  "font-src data:",
  "style-src 'unsafe-inline'",
  "script-src 'unsafe-inline'",
].join("; ");

export const SANDBOX_IFRAME_PERMISSIONS = [
  "accelerometer 'none'",
  "autoplay 'none'",
  "camera 'none'",
  "clipboard-read 'none'",
  "clipboard-write 'none'",
  "encrypted-media 'none'",
  "fullscreen 'none'",
  "geolocation 'none'",
  "gyroscope 'none'",
  "microphone 'none'",
  "midi 'none'",
  "payment 'none'",
  "serial 'none'",
  "usb 'none'",
  "bluetooth 'none'",
].join("; ");

export function escapeHTML(value: string): string {
  return value
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;");
}

export function escapeScriptContent(value: string): string {
  return value.replace(SCRIPT_CLOSE_RE, "<\\/script");
}

export function escapeStyleContent(value: string): string {
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

export function sandboxDocumentHead(title: string, theme: HTMLVisualThemeSnapshot): string {
  return [
    `<meta charset="utf-8">`,
    `<meta name="viewport" content="width=device-width, initial-scale=1">`,
    `<meta http-equiv="Content-Security-Policy" content="${SANDBOX_CSP}">`,
    `<title>${escapeHTML(title)}</title>`,
    artifactThemeStyle(theme),
    artifactPreviewResetStyle(),
    artifactRuntimeScript(),
  ].join("");
}

