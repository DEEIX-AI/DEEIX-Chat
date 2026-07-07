const EXPORT_MANIFEST_TAIL_BYTES = 1024 * 1024;

export type ExportManifest = {
  _type?: string;
  complete?: boolean;
  exported?: number;
  failed?: number;
};

export async function readExportManifest(blob: Blob): Promise<ExportManifest | null> {
  if (blob.size <= 0) {
    return null;
  }
  const start = Math.max(0, blob.size - EXPORT_MANIFEST_TAIL_BYTES);
  const tail = await blob.slice(start).text();
  const lastLine = tail.trimEnd().split(/\r?\n/).at(-1);
  if (!lastLine) {
    return null;
  }
  try {
    const parsed = JSON.parse(lastLine) as unknown;
    if (!parsed || typeof parsed !== "object") {
      return null;
    }
    const manifest = parsed as ExportManifest;
    return manifest._type === "export_manifest" ? manifest : null;
  } catch {
    return null;
  }
}

export function downloadBlob(blob: Blob, fileName: string): void {
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = fileName;
  link.rel = "noopener";
  document.body.appendChild(link);
  try {
    link.click();
  } finally {
    link.remove();
    window.setTimeout(() => URL.revokeObjectURL(url), 0);
  }
}
