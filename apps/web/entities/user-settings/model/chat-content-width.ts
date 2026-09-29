export type ChatContentWidth = "compact" | "standard" | "wide";

export type ChatContentWidthOption = {
  value: ChatContentWidth;
  width: number;
  className: string;
  previewScaleClassName: string;
};

export const DEFAULT_CHAT_CONTENT_WIDTH: ChatContentWidth = "compact";

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
];

export function isChatContentWidth(value: unknown): value is ChatContentWidth {
  return value === "compact" || value === "standard" || value === "wide";
}

export function parseChatContentWidth(value: string | null | undefined): ChatContentWidth {
  return isChatContentWidth(value) ? value : DEFAULT_CHAT_CONTENT_WIDTH;
}

export function resolveChatContentWidthClassName(value: ChatContentWidth): string {
  return (
    CHAT_CONTENT_WIDTH_OPTIONS.find((item) => item.value === value)?.className ??
    CHAT_CONTENT_WIDTH_OPTIONS[0].className
  );
}
