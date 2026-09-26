function safeFileNamePart(value: string) {
  const normalized = value
    .trim()
    .replace(/[\\/:*?"<>|]+/g, "-")
    .replace(/\s+/g, "-")
    .replace(/-+/g, "-")
    .replace(/^-|-$/g, "");
  return normalized || "conversation";
}

function formatScreenshotTimestamp(date = new Date()) {
  const pad = (part: number) => String(part).padStart(2, "0");
  return [
    date.getFullYear(),
    pad(date.getMonth() + 1),
    pad(date.getDate()),
    "-",
    pad(date.getHours()),
    pad(date.getMinutes()),
    pad(date.getSeconds()),
  ].join("");
}

export function resolveConversationScreenshotFileName(title: string) {
  return `conversation-${safeFileNamePart(title)}-${formatScreenshotTimestamp()}.png`;
}

export function downloadPngBlob(blob: Blob, fileName: string) {
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = fileName;
  link.rel = "noopener";
  document.body.appendChild(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
}

export function isClipboardImageWriteSupported() {
  return (
    typeof navigator !== "undefined" &&
    typeof navigator.clipboard?.write === "function" &&
    typeof ClipboardItem !== "undefined"
  );
}

export async function copyPngBlobToClipboard(blob: Blob) {
  if (!isClipboardImageWriteSupported()) {
    throw new Error("Clipboard image write is not supported");
  }
  const pngBlob = blob.type === "image/png" ? blob : new Blob([blob], { type: "image/png" });
  await navigator.clipboard.write([new ClipboardItem({ "image/png": pngBlob })]);
}
