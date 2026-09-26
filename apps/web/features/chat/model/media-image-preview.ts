import type { StreamMessageEvent } from "@/shared/api/conversation.types";

const IMAGE_DATA_URL_PREFIX_RE = /^data:(image\/(?:png|jpe?g|webp|gif));base64,/i;
const MARKDOWN_DATA_IMAGE_RE = /^!\[([^\]]*)\]\((data:image\/(?:png|jpe?g|webp|gif);base64,([A-Za-z0-9+/=\s]+))\)/i;
const BASE64_PAYLOAD_RE = /^[A-Za-z0-9+/=]+$/;
const MIN_INLINE_IMAGE_BASE64_LENGTH = 64;
const RAW_IMAGE_BASE64_PREFIXES = [
  { prefix: "iVBORw0", mimeType: "image/png" },
  { prefix: "/9j/", mimeType: "image/jpeg" },
  { prefix: "R0lGOD", mimeType: "image/gif" },
  { prefix: "UklGR", mimeType: "image/webp" },
] as const;

export type LeadingImagePreview = {
  alt: string;
  complete: boolean;
  source: string;
  rest: string;
};

function escapeMarkdownImageAlt(value: string): string {
  return value.replaceAll("[", "").replaceAll("]", "").replaceAll("\n", " ").trim();
}

export function buildMediaImagePreviewMarkdown(
  event: Extract<StreamMessageEvent, { type: "media_image_delta" }>,
  fallbackAlt: string,
): string {
  const b64 = event.b64_json.trim();
  if (!b64) {
    return "";
  }
  const source = b64.startsWith("data:")
    ? b64
    : `data:${event.mime_type?.trim() || "image/png"};base64,${b64}`;
  const alt = escapeMarkdownImageAlt(event.revised_prompt?.trim() || fallbackAlt);
  return `![${alt}](${source})`;
}

export function resolveLeadingImagePreview(content: string): LeadingImagePreview | null {
  const text = content.trimStart();
  if (!text) {
    return null;
  }
  const markdownMatch = MARKDOWN_DATA_IMAGE_RE.exec(text);
  if (markdownMatch) {
    const payload = normalizeBase64Payload(markdownMatch[3]);
    if (!isLikelyImageBase64Payload(payload)) {
      return null;
    }
    return {
      alt: markdownMatch[1].trim(),
      complete: isCompleteImageBase64Payload(markdownImageMIMEType(markdownMatch[2]), payload),
      source: normalizeImageDataURL(markdownMatch[2]),
      rest: text.slice(markdownMatch[0].length).trimStart(),
    };
  }
  const dataURLMatch = IMAGE_DATA_URL_PREFIX_RE.exec(text);
  if (dataURLMatch) {
    const payload = readLeadingBase64Payload(text.slice(dataURLMatch[0].length));
    if (!payload) {
      return null;
    }
    const mimeType = dataURLMatch[1].toLowerCase();
    return {
      alt: "",
      complete: isCompleteImageBase64Payload(mimeType, payload.value),
      source: `data:${mimeType};base64,${payload.value}`,
      rest: payload.rest,
    };
  }

  const payload = readLeadingBase64Payload(text);
  if (!payload) {
    return null;
  }
  const match = RAW_IMAGE_BASE64_PREFIXES.find((item) => payload.value.startsWith(item.prefix));
  return match
    ? {
        alt: "",
        complete: isCompleteImageBase64Payload(match.mimeType, payload.value),
        source: `data:${match.mimeType};base64,${payload.value}`,
        rest: payload.rest,
      }
    : null;
}

function readLeadingBase64Payload(value: string): { value: string; rest: string } | null {
  const parts: string[] = [];
  let cursor = 0;

  while (cursor < value.length) {
    const lineBreakMatch = /\r?\n/.exec(value.slice(cursor));
    const lineEnd = lineBreakMatch ? cursor + lineBreakMatch.index : value.length;
    const line = value.slice(cursor, lineEnd).trim();
    if (!line) {
      cursor = lineBreakMatch ? lineEnd + lineBreakMatch[0].length : lineEnd;
      break;
    }
    if (!BASE64_PAYLOAD_RE.test(line)) {
      break;
    }
    parts.push(line);
    cursor = lineBreakMatch ? lineEnd + lineBreakMatch[0].length : lineEnd;
    if (!lineBreakMatch) {
      break;
    }
  }

  const payload = parts.join("");
  if (!isLikelyImageBase64Payload(payload)) {
    return null;
  }
  return {
    value: payload,
    rest: value.slice(cursor).trimStart(),
  };
}

function isLikelyImageBase64Payload(value: string): boolean {
  return value.length >= MIN_INLINE_IMAGE_BASE64_LENGTH && BASE64_PAYLOAD_RE.test(value);
}

function normalizeImageDataURL(value: string): string {
  const commaIndex = value.indexOf(",");
  if (commaIndex < 0) {
    return "";
  }
  const prefix = value.slice(0, commaIndex + 1);
  const payload = normalizeBase64Payload(value.slice(commaIndex + 1));
  return `${prefix}${payload}`;
}

function normalizeBase64Payload(value: string): string {
  return value.replace(/\s/g, "");
}

function markdownImageMIMEType(value: string): string {
  const match = IMAGE_DATA_URL_PREFIX_RE.exec(value);
  return match?.[1].toLowerCase() || "";
}

function isCompleteImageBase64Payload(mimeType: string, payload: string): boolean {
  const normalized = normalizeBase64Payload(payload);
  if (!isLikelyImageBase64Payload(normalized) || base64DecodedByteLength(normalized) <= 0) {
    return false;
  }
  switch (mimeType.toLowerCase()) {
    case "image/png":
      return base64PayloadEndsWithBytes(normalized, [0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82]);
    case "image/jpeg":
    case "image/jpg":
      return base64PayloadEndsWithBytes(normalized, [0xff, 0xd9]);
    case "image/gif":
      return base64PayloadEndsWithBytes(normalized, [0x3b]);
    case "image/webp":
      return isCompleteWebPBase64Payload(normalized);
    default:
      return false;
  }
}

function base64PayloadEndsWithBytes(value: string, suffix: number[]): boolean {
  const tail = decodeBase64Tail(value, Math.max(24, suffix.length + 8));
  return tail ? bytesEndWith(tail, suffix) : false;
}

function decodeBase64Tail(value: string, minDecodedBytes: number): Uint8Array | null {
  const normalized = normalizeBase64Payload(value);
  if (!isLikelyImageBase64Payload(normalized) || normalized.length % 4 === 1) {
    return null;
  }
  const charCount = Math.ceil((minDecodedBytes + 2) / 3) * 4;
  let start = Math.max(0, normalized.length - charCount);
  start -= start % 4;
  return decodeBase64Segment(normalized.slice(start));
}

function decodeBase64Segment(value: string): Uint8Array | null {
  if (!value || value.length % 4 === 1 || typeof globalThis.atob !== "function") {
    return null;
  }
  const padded = value.padEnd(value.length + ((4 - (value.length % 4)) % 4), "=");
  try {
    const binary = globalThis.atob(padded);
    const bytes = new Uint8Array(binary.length);
    for (let index = 0; index < binary.length; index += 1) {
      bytes[index] = binary.charCodeAt(index);
    }
    return bytes;
  } catch {
    return null;
  }
}

function base64DecodedByteLength(value: string): number {
  const normalized = normalizeBase64Payload(value);
  if (!normalized || normalized.length % 4 === 1) {
    return -1;
  }
  const padding = normalized.endsWith("==") ? 2 : normalized.endsWith("=") ? 1 : 0;
  const completeGroups = Math.floor(normalized.length / 4);
  const remainder = normalized.length % 4;
  const remainderBytes = remainder === 2 ? 1 : remainder === 3 ? 2 : 0;
  return completeGroups * 3 + remainderBytes - padding;
}

function bytesEndWith(bytes: Uint8Array, suffix: number[]): boolean {
  if (bytes.length < suffix.length) {
    return false;
  }
  const offset = bytes.length - suffix.length;
  return suffix.every((item, index) => bytes[offset + index] === item);
}

function isCompleteWebPBase64Payload(value: string): boolean {
  const decodedLength = base64DecodedByteLength(value);
  if (decodedLength < 12) {
    return false;
  }
  const bytes = decodeBase64Segment(normalizeBase64Payload(value).slice(0, 16));
  if (!bytes || bytes.length < 12) {
    return false;
  }
  const riff = bytes[0] === 0x52 && bytes[1] === 0x49 && bytes[2] === 0x46 && bytes[3] === 0x46;
  const webp = bytes[8] === 0x57 && bytes[9] === 0x45 && bytes[10] === 0x42 && bytes[11] === 0x50;
  if (!riff || !webp) {
    return false;
  }
  const declaredSize = (bytes[4] | (bytes[5] << 8) | (bytes[6] << 16) | (bytes[7] << 24)) >>> 0;
  return declaredSize + 8 === decodedLength;
}
