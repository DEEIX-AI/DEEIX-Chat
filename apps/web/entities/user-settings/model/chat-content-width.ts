export type ChatContentWidth = "compact" | "standard" | "wide" | "full";
export type ChatContentAlign = "center" | "left";

export type ChatContentWidthOption = {
  value: ChatContentWidth;
  width: number;
  className: string;
  previewScaleClassName: string;
};

export const DEFAULT_CHAT_CONTENT_WIDTH: ChatContentWidth = "compact";
export const DEFAULT_CHAT_CONTENT_ALIGN: ChatContentAlign = "center";

// The composer keeps one fixed width and position. The width/alignment setting reshapes the
// message column only, so changing it never moves the input box under the cursor.
export const CHAT_COMPOSER_WIDTH_CLASSNAME = "mx-auto w-full max-w-[900px]";

export const CHAT_CONTENT_WIDTH_OPTIONS: ChatContentWidthOption[] = [
  {
    value: "compact",
    width: 760,
    className: "max-w-[760px]",
    previewScaleClassName: "w-7/12",
  },
  {
    value: "standard",
    width: 900,
    className: "max-w-[900px]",
    previewScaleClassName: "w-9/12",
  },
  {
    value: "wide",
    width: 1080,
    className: "max-w-[1080px]",
    previewScaleClassName: "w-11/12",
  },
  {
    value: "full",
    width: 0,
    className: "max-w-none",
    previewScaleClassName: "w-full",
  },
];

export const CHAT_CONTENT_ALIGN_OPTIONS: Array<{ value: ChatContentAlign; className: string }> = [
  { value: "center", className: "mx-auto" },
  { value: "left", className: "mr-auto ml-0" },
];

export function isChatContentWidth(value: unknown): value is ChatContentWidth {
  return value === "compact" || value === "standard" || value === "wide" || value === "full";
}

export function isChatContentAlign(value: unknown): value is ChatContentAlign {
  return value === "center" || value === "left";
}

export function parseChatContentWidth(value: string | null | undefined): ChatContentWidth {
  return isChatContentWidth(value) ? value : DEFAULT_CHAT_CONTENT_WIDTH;
}

export function parseChatContentAlign(value: string | null | undefined): ChatContentAlign {
  return isChatContentAlign(value) ? value : DEFAULT_CHAT_CONTENT_ALIGN;
}

export function resolveChatContentWidthClassName(
  value: ChatContentWidth,
  align: ChatContentAlign = DEFAULT_CHAT_CONTENT_ALIGN,
): string {
  const widthClass =
    CHAT_CONTENT_WIDTH_OPTIONS.find((item) => item.value === value)?.className ??
    CHAT_CONTENT_WIDTH_OPTIONS[0].className;
  return `${widthClass} ${align === "left" ? "mr-auto ml-0" : "mx-auto"}`;
}

export function resolveChatContentAlignClassName(value: ChatContentAlign): string {
  return value === "left" ? "mr-auto ml-0" : "mx-auto";
}
